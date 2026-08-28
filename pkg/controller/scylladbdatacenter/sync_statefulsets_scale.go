package scylladbdatacenter

import (
	"context"
	"fmt"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/naming"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

// scaleStatefulSets scales the first StatefulSet whose replicas differ from the required ones. Racks are scaled one at
// a time, and a scale-down goes one node at a time: the leaving node is first asked to decommission through its member
// Service and the StatefulSet is only scaled once the node reports it's decommissioned. Nothing is scaled while a
// decommission is in progress.
func (sdcc *Controller) scaleStatefulSets(ctx context.Context, sc *statefulSetSyncContext) (stepResult, error) {
	sdc := sc.sdc
	for _, req := range sc.requiredStatefulSets {
		sts := sc.existingStatefulSets[req.Name]
		rackServices := servicesForRack(sc.services, sts.Labels[naming.RackNameLabel])

		// Wait if any decommissioning is in progress.
		for _, svc := range rackServices {
			if svc.Labels[naming.DecommissionedLabel] == naming.LabelValueFalse {
				klog.V(4).InfoS("Waiting for service to be decommissioned")
				return blockWith(newStatefulSetProgressingCondition(
					sdc,
					reasonWaitingForRackServiceDecommission,
					fmt.Sprintf("Waiting for rack service %q to decommission.", naming.ObjRef(svc)),
				)), nil
			}
		}

		requiredReplicas := *req.Spec.Replicas
		currentReplicas := *sts.Spec.Replicas
		if requiredReplicas == currentReplicas {
			continue
		}

		var progressingConditions []metav1.Condition
		var err error
		if requiredReplicas < currentReplicas {
			progressingConditions, err = sdcc.scaleStatefulSetDown(ctx, sdc, sts, rackServices)
		} else {
			progressingConditions, err = sdcc.scaleStatefulSet(ctx, sdc, sts, requiredReplicas)
		}
		return blockWith(progressingConditions...), err
	}

	return proceed(), nil
}

// scaleStatefulSetDown removes the last node of the StatefulSet: it asks the node to decommission and, once the node
// is decommissioned, scales the StatefulSet down by one.
func (sdcc *Controller) scaleStatefulSetDown(
	ctx context.Context,
	sdc *scyllav1alpha1.ScyllaDBDatacenter,
	sts *appsv1.StatefulSet,
	rackServices map[string]*corev1.Service,
) ([]metav1.Condition, error) {
	// Make sure we always scale down by 1 member.
	targetReplicas := *sts.Spec.Replicas - 1

	lastSvcName := fmt.Sprintf("%s-%d", sts.Name, targetReplicas)
	lastSvc, ok := rackServices[lastSvcName]
	if !ok {
		klog.V(4).InfoS("Missing service", "ScyllaDBDatacenter", klog.KObj(sdc), "ServiceName", lastSvcName)
		// Services are managed in the other loop.
		// When informers see the new service, will get re-queued.
		return []metav1.Condition{
			newStatefulSetProgressingCondition(
				sdc,
				reasonWaitingForMissingService,
				fmt.Sprintf("Statusfulset %q is waiting for service %q to be created", naming.ObjRef(sts), lastSvcName),
			),
		}, nil
	}

	if len(lastSvc.Labels[naming.DecommissionedLabel]) == 0 {
		return sdcc.requestNodeDecommission(ctx, sdc, lastSvc)
	}

	return sdcc.scaleStatefulSet(ctx, sdc, sts, targetReplicas)
}

// requestNodeDecommission records the intent to decommission the node on its member Service. The node's sidecar picks
// it up, decommissions the node and flips the label once done.
// TODO: Move this into syncServices so it reconciles properly. This is edge triggered and nothing will reconcile the
// label if something goes wrong or the flow changes.
func (sdcc *Controller) requestNodeDecommission(ctx context.Context, sdc *scyllav1alpha1.ScyllaDBDatacenter, svc *corev1.Service) ([]metav1.Condition, error) {
	var progressingConditions []metav1.Condition

	svcCopy := svc.DeepCopy()
	svcCopy.Labels[naming.DecommissionedLabel] = naming.LabelValueFalse
	controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, statefulSetControllerProgressingCondition, svcCopy, "update", sdc.Generation)
	_, err := sdcc.kubeClient.CoreV1().Services(svcCopy.Namespace).Update(ctx, svcCopy, metav1.UpdateOptions{})
	if err != nil {
		return progressingConditions, err
	}

	return progressingConditions, nil
}

// scaleStatefulSet sets the replicas of the StatefulSet through its scale subresource.
func (sdcc *Controller) scaleStatefulSet(ctx context.Context, sdc *scyllav1alpha1.ScyllaDBDatacenter, sts *appsv1.StatefulSet, replicas int32) ([]metav1.Condition, error) {
	var progressingConditions []metav1.Condition

	scale := &autoscalingv1.Scale{
		ObjectMeta: metav1.ObjectMeta{
			Name:            sts.Name,
			Namespace:       sts.Namespace,
			ResourceVersion: sts.ResourceVersion,
		},
		Spec: autoscalingv1.ScaleSpec{
			Replicas: replicas,
		},
	}

	klog.V(2).InfoS("Scaling StatefulSet", "ScyllaDBDatacenter", klog.KObj(sdc), "StatefulSet", klog.KObj(sts), "CurrentReplicas", *sts.Spec.Replicas, "UpdatedReplicas", scale.Spec.Replicas)
	controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, statefulSetControllerProgressingCondition, scale, "updateScale", sdc.Generation)
	_, err := sdcc.kubeClient.AppsV1().StatefulSets(sts.Namespace).UpdateScale(ctx, sts.Name, scale, metav1.UpdateOptions{})
	if err != nil {
		return progressingConditions, fmt.Errorf("can't update scale: %w", err)
	}

	return progressingConditions, nil
}

// servicesForRack returns the member Services of the named rack.
func servicesForRack(services map[string]*corev1.Service, rackName string) map[string]*corev1.Service {
	rackServices := map[string]*corev1.Service{}
	for _, svc := range services {
		svcRackName, ok := svc.Labels[naming.RackNameLabel]
		if ok && svcRackName == rackName {
			rackServices[svc.Name] = svc
		}
	}
	return rackServices
}
