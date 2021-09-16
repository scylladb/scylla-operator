package scyllanodeconfig

import (
	"context"
	"fmt"
	"strings"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controller/scyllanodeconfig/resource"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/resourceapply"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
)

func (scc *Controller) makeConfigMaps(ctx context.Context, scyllaNodeConfig *scyllav1alpha1.ScyllaNodeConfig, scyllaPods []*corev1.Pod, jobs map[string]*batchv1.Job, configMaps map[string]*corev1.ConfigMap) ([]*corev1.ConfigMap, error) {
	var cms []*corev1.ConfigMap

	optimizedNodes := map[string]*scyllav1alpha1.ScyllaNodeConfig{}

	sncs, err := scc.scyllaNodeConfigLister.List(labels.Everything())
	if err != nil {
		return nil, fmt.Errorf("list scyllanodeconfigs: %w", err)
	}

	nodes, err := scc.nodeLister.List(labels.Everything())
	if err != nil {
		return nil, fmt.Errorf("list nodes: %w", err)
	}

	for _, snc := range sncs {
		ds, err := scc.daemonSetLister.DaemonSets(naming.ScyllaOperatorNodeTuningNamespace).Get(snc.Name)
		if err != nil {
			return nil, fmt.Errorf("get daemonset '%s/%s': %w", naming.ScyllaOperatorNodeTuningNamespace, snc.Name, err)
		}

		for _, node := range nodes {
			if nodeIsTargetedByDaemonSet(node, ds) {
				optimizedNodes[node.Name] = snc
			}
		}
	}

	var errs []error
	for _, scyllaPod := range scyllaPods {
		var data []byte
		nodeName := scyllaPod.Spec.NodeName
		if nodeName == "" {
			klog.V(4).InfoS("Scylla Pod not scheduled yet", "Pod", klog.KObj(scyllaPod))
			continue
		}

		owner, isOptimized := optimizedNodes[nodeName]
		// Create empty ConfigMap for Pods running on non optimized Nodes.
		if !isOptimized {
			klog.V(4).InfoS("Scylla Pod running on non optimized Node", "Pod", klog.KObj(scyllaPod), "Node", klog.KObj(scyllaPod))

			defaultSnc, err := scc.scyllaNodeConfigLister.Get(resource.DefaultScyllaNodeConfig().Name)
			if err != nil {
				return nil, fmt.Errorf("get default ScyllaNodeConfig: %w", err)
			}
			cms = append(cms, resource.PerftuneConfigMap(defaultSnc, scyllaPod, data))
			continue
		}

		// Skip if Pod should be optimized, but not by us.
		if owner.Name != scyllaNodeConfig.Name {
			klog.V(4).InfoS("Scylla Pod not optimized by us", "Pod", klog.KObj(scyllaPod), "ScyllaNodeConfig", klog.KObj(scyllaNodeConfig))
			continue
		}

		// Create empty ConfigMap when optimizations are disabled or Pod is not subject for optimizations
		if scyllaNodeConfig.Spec.DisableOptimizations || scyllaPod.Status.QOSClass != corev1.PodQOSGuaranteed {
			klog.V(4).InfoS("Scylla Pod not subject for optimizations", "Pod", klog.KObj(scyllaPod))
			cms = append(cms, resource.PerftuneConfigMap(scyllaNodeConfig, scyllaPod, data))
			continue
		}

		// Wait until optimizing Job completes.
		job, perftuneJobCreated := jobs[naming.PerftuneJobName(nodeName)]
		if !perftuneJobCreated || job.Status.CompletionTime == nil {
			klog.V(4).Infof("Job for %s Pod not completed, will retry in a bit", scyllaPod.Name)
			if cm, ok := configMaps[naming.PerftuneResultName(string(scyllaPod.UID))]; ok {
				cms = append(cms, cm)
			}
			continue
		}

		klog.V(4).InfoS("Scylla Pod is optimized", "Pod", klog.KObj(scyllaPod))

		var perftuneContainer corev1.Container
		for _, c := range job.Spec.Template.Spec.Containers {
			if c.Name == naming.PerftuneContainerName {
				perftuneContainer = c
				break
			}
		}

		command := make([]string, 0, len(perftuneContainer.Command)+len(perftuneContainer.Args))
		command = append(command, perftuneContainer.Command...)
		command = append(command, perftuneContainer.Args...)

		cms = append(cms, resource.PerftuneConfigMap(scyllaNodeConfig, scyllaPod, []byte(strings.Join(command, " "))))
	}

	return cms, utilerrors.NewAggregate(errs)
}

func (scc *Controller) pruneConfigMaps(
	ctx context.Context,
	requiredConfigMaps []*corev1.ConfigMap,
	configMaps map[string]*corev1.ConfigMap,
) error {
	var errs []error
	for _, cm := range configMaps {
		if cm.DeletionTimestamp != nil {
			continue
		}

		isRequired := false
		for _, req := range requiredConfigMaps {
			if cm.Name == req.Name {
				isRequired = true
				break
			}
		}
		if isRequired {
			continue
		}

		klog.InfoS("Removing stale ConfigMap", "ConfigMap", klog.KObj(cm))
		propagationPolicy := metav1.DeletePropagationBackground
		err := scc.kubeClient.CoreV1().ConfigMaps(cm.Namespace).Delete(ctx, cm.Name, metav1.DeleteOptions{
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

func (scc *Controller) syncConfigMaps(
	ctx context.Context,
	snc *scyllav1alpha1.ScyllaNodeConfig,
	scyllaPods []*corev1.Pod,
	configMaps map[string]*corev1.ConfigMap,
	jobs map[string]*batchv1.Job,
) error {
	requiredConfigMaps, err := scc.makeConfigMaps(ctx, snc, scyllaPods, jobs, configMaps)
	if err != nil {
		return err
	}

	// Delete any excessive ConfigMaps.
	// Delete has to be the first action to avoid getting stuck on quota.
	err = scc.pruneConfigMaps(ctx, requiredConfigMaps, configMaps)
	if err != nil {
		return fmt.Errorf("can't delete ConfigMaps(s): %w", err)
	}

	var errs []error
	for _, required := range requiredConfigMaps {
		_, _, err := resourceapply.ApplyConfigMap(ctx, scc.kubeClient.CoreV1(), scc.configMapLister, scc.eventRecorder, required)
		if err != nil {
			errs = append(errs, fmt.Errorf("can't apply configmap update: %w", err))
			continue
		}
	}

	return utilerrors.NewAggregate(errs)
}
