// Copyright (C) 2025 ScyllaDB

package scylladbdatacenter

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/resourceapply"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (sdcc *Controller) syncClusterStatusConfigMap(
	ctx context.Context,
	sdc *scyllav1alpha1.ScyllaDBDatacenter,
	services map[string]*corev1.Service,
) ([]metav1.Condition, error) {
	var progressingConditions []metav1.Condition
	var err error

	var clusterStatus *controllerhelpers.ClusterStatus
	if clusterStatusOverrideConfigMapRef, ok := sdc.Annotations[naming.ScyllaDBClusterStatusOverrideConfigMapRefAnnotation]; ok {
		// TODO: move to func
		overrideCM, err := sdcc.configMapLister.ConfigMaps(sdc.Namespace).Get(clusterStatusOverrideConfigMapRef)
		if err != nil {
			if !apierrors.IsNotFound(err) {
				return progressingConditions, fmt.Errorf("can't get configmap %q referenced in annotation %q of ScyllaDBDatacenter %q: %w", naming.ManualRef(sdc.Namespace, clusterStatusOverrideConfigMapRef), naming.ScyllaDBClusterStatusOverrideConfigMapRefAnnotation, naming.ObjRef(sdc), err)
			}

			progressingConditions = append(progressingConditions, metav1.Condition{
				Type:    clusterStatusControllerProgressingCondition,
				Status:  metav1.ConditionTrue,
				Reason:  "WaitingForClusterStatusOverrideConfigMap",
				Message: fmt.Sprintf("Waiting for configmap %q referenced in annotation %q of ScyllaDBDatacenter %q to be created", naming.ManualRef(sdc.Namespace, clusterStatusOverrideConfigMapRef), naming.ScyllaDBClusterStatusOverrideConfigMapRefAnnotation, naming.ObjRef(sdc)),
			})
			return progressingConditions, nil
		}

		var overrideClusterStatus controllerhelpers.ClusterStatus
		data, ok := overrideCM.Data[naming.ScyllaDBClusterStatusKey]
		if !ok {
			return progressingConditions, fmt.Errorf("missing %s key", naming.ScyllaDBClusterStatusKey)
		}

		err = json.Unmarshal([]byte(data), &overrideClusterStatus)
		if err != nil {
			return progressingConditions, fmt.Errorf("can't unmarshal cluster status: %w", err)
		}

		clusterStatus = &overrideClusterStatus
	} else {
		clusterStatus, err = getClusterStatus(sdc, services)
		if err != nil {
			return progressingConditions, fmt.Errorf("can't get node cluster statuses: %w", err)
		}
	}

	requiredConfigMap, err := MakeClusterStatusConfigMap(sdc, clusterStatus)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't make cluster status configmap: %w", err)
	}

	_, changed, err := resourceapply.ApplyConfigMap(ctx, sdcc.kubeClient.CoreV1(), sdcc.configMapLister, sdcc.eventRecorder, requiredConfigMap, resourceapply.ApplyOptions{})
	if changed {
		controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, clusterStatusControllerProgressingCondition, requiredConfigMap, "apply", sdc.Generation)
	}
	if err != nil {
		return progressingConditions, fmt.Errorf("can't apply configmap %q: %w", naming.ObjRef(requiredConfigMap), err)
	}

	return progressingConditions, nil
}

func getClusterStatus(sdc *scyllav1alpha1.ScyllaDBDatacenter, services map[string]*corev1.Service) (*controllerhelpers.ClusterStatus, error) {
	clusterStatus := &controllerhelpers.ClusterStatus{
		ReplacingHostIDs:    []string{},
		NodeClusterStatuses: []controllerhelpers.NodeClusterStatus{},
	}

	for _, rack := range sdc.Spec.Racks {
		stsName := naming.StatefulSetNameForRack(rack, sdc)
		rackNodes, err := controllerhelpers.GetRackNodeCount(sdc, rack.Name)
		if err != nil {
			return nil, fmt.Errorf("can't get rack %q node count of ScyllaDBDatacenter %q: %w", rack.Name, naming.ObjRef(sdc), err)
		}

		for ord := int32(0); ord < *rackNodes; ord++ {
			svcName := fmt.Sprintf("%s-%d", stsName, ord)

			svc, ok := services[svcName]
			if !ok {
				// FIXME
				// The service might not have been created yet.
				continue
			}

			if replacingHostIDAnnotation, ok := svc.Annotations[naming.ReplacingNodeHostIDLabel]; ok {
				clusterStatus.ReplacingHostIDs = append(clusterStatus.ReplacingHostIDs, replacingHostIDAnnotation)
			}

			if nodeClusterStatusAnnotation, ok := svc.Annotations[naming.NodeClusterStatusAnnotation]; ok {
				var nodeClusterStatus controllerhelpers.NodeClusterStatus
				err = json.NewDecoder(strings.NewReader(nodeClusterStatusAnnotation)).Decode(&nodeClusterStatus)
				if err != nil {
					return nil, fmt.Errorf("can't decode annotation %q of service %q for ScyllaDBDatacenter %q: %w", naming.NodeClusterStatusAnnotation, naming.ManualRef(sdc.Namespace, svcName), naming.ObjRef(sdc), err)
				}

				clusterStatus.NodeClusterStatuses = append(clusterStatus.NodeClusterStatuses, nodeClusterStatus)
			}
		}
	}

	return clusterStatus, nil
}
