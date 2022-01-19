// Copyright (C) 2021 ScyllaDB

package nodeconfigpod

import (
	"context"
	"encoding/json"
	"fmt"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/internalapi"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/resourceapply"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
)

func (ncpc *Controller) makeConfigMap(ctx context.Context, pod *corev1.Pod) (*corev1.ConfigMap, error) {
	if len(pod.Spec.NodeName) == 0 {
		return nil, nil
	}

	node, err := ncpc.nodeLister.Get(pod.Spec.NodeName)
	if err != nil {
		return nil, fmt.Errorf("can't get node: %w", err)
	}

	allNodeConfigs, err := ncpc.nodeConfigLister.List(labels.Everything())
	if err != nil {
		return nil, fmt.Errorf("can't list nodeconfigs: %w", err)
	}

	// Filter nodeconfig selecting this node.
	var ncs []*scyllav1alpha1.NodeConfig
	for _, nc := range allNodeConfigs {
		isSelectingNode, err := controllerhelpers.IsNodeConfigSelectingNode(nc, node)
		if err != nil {
			return nil, fmt.Errorf(
				"can't check if nodecondig %q is selecting node %q: %w",
				naming.ObjRef(nc),
				naming.ObjRef(node),
				err,
			)
		}
		if isSelectingNode {
			ncs = append(ncs, nc)
		}
	}

	klog.V(4).InfoS("Pod matched by NodeConfigs", "Pod", klog.KObj(pod), "MatchingNodeConfigs", len(ncs))

	containerID, err := controllerhelpers.GetScyllaContainerID(pod)
	if err != nil {
		return nil, fmt.Errorf("can't get container id: %w", err)
	}

	src := &internalapi.SidecarRuntimeConfig{
		ContainerID:         containerID,
		BlockingNodeConfigs: nil,
	}

	if len(ncs) != 0 {
		for _, nc := range ncs {
			src.MatchingNodeConfigs = append(src.MatchingNodeConfigs, nc.Name)

			if nc.Spec.DisableOptimizations {
				continue
			}

			if controllerhelpers.IsNodeTunedForContainer(nc, node.Name, containerID) {
				continue
			}

			src.BlockingNodeConfigs = append(src.BlockingNodeConfigs, nc.Name)
		}
	}

	srcData, err := json.Marshal(src)
	if err != nil {
		return nil, fmt.Errorf("can't marshal scylla runtime config: %w", err)
	}

	data := map[string]string{
		naming.ScyllaRuntimeConfigKey: string(srcData),
	}

	return makeConfigMap(pod, data), nil
}

func (ncpc *Controller) pruneConfigMaps(ctx context.Context, required *corev1.ConfigMap, configMaps map[string]*corev1.ConfigMap) error {
	var errs []error

	for _, cm := range configMaps {
		if cm.DeletionTimestamp != nil {
			continue
		}

		if cm.Name == required.Name {
			continue
		}

		propagationPolicy := metav1.DeletePropagationBackground
		err := ncpc.kubeClient.CoreV1().ConfigMaps(cm.Namespace).Delete(ctx, cm.Name, metav1.DeleteOptions{
			Preconditions: &metav1.Preconditions{
				UID: &cm.UID,
			},
			PropagationPolicy: &propagationPolicy,
		})
		if err != nil {
			errs = append(errs, err)
			continue
		}
	}

	return utilerrors.NewAggregate(errs)
}

func (ncpc *Controller) syncConfigMaps(
	ctx context.Context,
	pod *corev1.Pod,
	configMaps map[string]*corev1.ConfigMap,
) error {
	required, err := ncpc.makeConfigMap(ctx, pod)
	if err != nil {
		return fmt.Errorf("can't make configmap for pod %q: %w", naming.ObjRef(pod), err)
	}

	// Delete any excessive ConfigMaps.
	// Delete has to be the first action to avoid getting stuck on quota.
	err = ncpc.pruneConfigMaps(ctx, required, configMaps)
	if err != nil {
		return fmt.Errorf("can't delete configmap(s): %w", err)
	}

	if required != nil {
		_, _, err := resourceapply.ApplyConfigMap(ctx, ncpc.kubeClient.CoreV1(), ncpc.configMapLister, ncpc.eventRecorder, required)
		if err != nil {
			return fmt.Errorf("can't apply configmap: %w", err)
		}
	}

	return nil
}
