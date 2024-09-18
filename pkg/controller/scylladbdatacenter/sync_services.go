package scylladbdatacenter

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"time"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/helpers/slices"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/resourceapply"
	"github.com/scylladb/scylla-operator/pkg/scyllafeatures"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
)

var serviceOrdinalRegex = regexp.MustCompile("^.*-([0-9]+)$")

func (sdcc *Controller) makeServices(sdc *scyllav1alpha1.ScyllaDBDatacenter, oldServices map[string]*corev1.Service, jobs map[string]*batchv1.Job) ([]*corev1.Service, error) {
	identityService, err := IdentityService(sdc)
	if err != nil {
		return nil, fmt.Errorf("can't create identity service: %w", err)
	}

	services := []*corev1.Service{
		identityService,
	}

	for _, rack := range sdc.Spec.Racks {
		stsName := naming.StatefulSetNameForRack(rack, sdc)
		rackNodes, err := controllerhelpers.GetRackNodeCount(sdc, rack.Name)
		if err != nil {
			return nil, fmt.Errorf("can't get rack %q node count of ScyllaDBDatacenter %q: %w", rack.Name, naming.ObjRef(sdc), err)
		}

		for ord := int32(0); ord < *rackNodes; ord++ {
			svcName := fmt.Sprintf("%s-%d", stsName, ord)
			oldSvc := oldServices[svcName]
			svc, err := MemberService(sdc, rack.Name, svcName, oldSvc, jobs)
			if err != nil {
				return nil, fmt.Errorf("can't create member service for %d'th node: %w", ord, err)
			}
			services = append(services, svc)
		}
	}

	return services, nil
}

func (sdcc *Controller) pruneServices(
	ctx context.Context,
	sdc *scyllav1alpha1.ScyllaDBDatacenter,
	requiredServices []*corev1.Service,
	services map[string]*corev1.Service,
	statefulSets map[string]*appsv1.StatefulSet,
) ([]metav1.Condition, error) {
	var errs []error
	var progressingConditions []metav1.Condition
	for _, svc := range services {
		if svc.DeletionTimestamp != nil {
			continue
		}

		isRequired := false
		for _, req := range requiredServices {
			if svc.Name == req.Name {
				isRequired = true
			}
		}
		if isRequired {
			continue
		}

		// Do not delete services for scale down.
		rackName, ok := svc.Labels[naming.RackNameLabel]
		if !ok {
			errs = append(errs, fmt.Errorf("service %s/%s is missing %q label", svc.Namespace, svc.Name, naming.RackNameLabel))
			continue
		}
		rackSpec, _, ok := slices.Find(sdc.Spec.Racks, func(spec scyllav1alpha1.RackSpec) bool {
			return spec.Name == rackName
		})
		if !ok {
			errs = append(errs, fmt.Errorf("can't find rack spec for service %q having %q rack label", naming.ObjRef(svc), rackName))
			continue
		}

		stsName := naming.StatefulSetNameForRack(rackSpec, sdc)
		sts, ok := statefulSets[stsName]
		if !ok {
			errs = append(errs, fmt.Errorf("statefulset %s/%s is missing", sdc.Namespace, stsName))
			continue
		}
		// TODO: Label services with the ordinal instead of parsing.
		// TODO: Move it to a function and unit test it.
		svcOrdinalStrings := serviceOrdinalRegex.FindStringSubmatch(svc.Name)
		if len(svcOrdinalStrings) != 2 {
			errs = append(errs, fmt.Errorf("can't parse ordinal from service %s/%s", svc.Namespace, svc.Name))
			continue
		}
		svcOrdinal, err := strconv.Atoi(svcOrdinalStrings[1])
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if int32(svcOrdinal) < *sts.Spec.Replicas {
			progressingConditions = append(progressingConditions, metav1.Condition{
				Type:               serviceControllerProgressingCondition,
				Status:             metav1.ConditionTrue,
				Reason:             "WaitingForAnotherService",
				Message:            fmt.Sprintf("Service %q is waiting for another service to be decommissioned first.", naming.ObjRef(svc)),
				ObservedGeneration: sdc.Generation,
			})
			continue
		}

		// Do not delete services that weren't properly decommissioned.
		if svc.Labels[naming.DecommissionedLabel] != naming.LabelValueTrue {
			klog.Warningf("Refusing to cleanup service %s/%s whose member wasn't decommissioned.", svc.Namespace, svc.Name)
			progressingConditions = append(progressingConditions, metav1.Condition{
				Type:               serviceControllerProgressingCondition,
				Status:             metav1.ConditionTrue,
				Reason:             "WaitingForServiceDecommission",
				Message:            fmt.Sprintf("Waiting for service %q to be fully decommissioned", naming.ObjRef(svc)),
				ObservedGeneration: sdc.Generation,
			})
			continue
		}

		// Because we also delete the PVC, we need to recheck the service state with a live call.
		// We can't delete the PVC after the service because the deletion wouldn't be retried.
		{
			freshSvc, err := sdcc.kubeClient.CoreV1().Services(svc.Namespace).Get(ctx, svc.Name, metav1.GetOptions{})
			if err != nil {
				errs = append(errs, err)
				continue
			}

			if freshSvc.UID != svc.UID {
				klog.V(2).InfoS("Stale caches, won't delete the pvc because the service UIDs don't match.", "Service", klog.KObj(svc))
				progressingConditions = append(progressingConditions, metav1.Condition{
					Type:               serviceControllerProgressingCondition,
					Status:             metav1.ConditionTrue,
					Reason:             "WaitingForCaches",
					Message:            fmt.Sprintf("Service %q is outdated because live UID is different. Waiting on service caches to catch up.", naming.ObjRef(svc)),
					ObservedGeneration: sdc.Generation,
				})
				continue
			}

			if freshSvc.Labels[naming.DecommissionedLabel] != naming.LabelValueTrue {
				klog.V(2).InfoS("Stale caches, won't delete the pvc because the service is no longer decommissioned.", "Service", klog.KObj(svc))
				progressingConditions = append(progressingConditions, metav1.Condition{
					Type:               serviceControllerProgressingCondition,
					Status:             metav1.ConditionTrue,
					Reason:             "WaitingForCaches",
					Message:            fmt.Sprintf("Service %q is outdated because the service is no longed decommisioned. Waiting on service caches to catch up.", naming.ObjRef(svc)),
					ObservedGeneration: sdc.Generation,
				})
				continue
			}
		}

		backgroundPropagationPolicy := metav1.DeletePropagationBackground

		controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, serviceControllerProgressingCondition, &corev1.PersistentVolumeClaim{}, "delete", sdc.Generation)
		pvcName := naming.PVCNameForService(svc.Name)
		err = sdcc.kubeClient.CoreV1().PersistentVolumeClaims(svc.Namespace).Delete(ctx, pvcName, metav1.DeleteOptions{
			PropagationPolicy: &backgroundPropagationPolicy,
		})
		if err != nil && !apierrors.IsNotFound(err) {
			errs = append(errs, err)
			continue
		}

		controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, serviceControllerProgressingCondition, svc, "delete", sdc.Generation)
		err = sdcc.kubeClient.CoreV1().Services(svc.Namespace).Delete(ctx, svc.Name, metav1.DeleteOptions{
			Preconditions: &metav1.Preconditions{
				UID:             &svc.UID,
				ResourceVersion: &svc.ResourceVersion,
			},
			PropagationPolicy: &backgroundPropagationPolicy,
		})
		if err != nil && !apierrors.IsNotFound(err) {
			errs = append(errs, err)
			continue
		}
	}
	return progressingConditions, utilerrors.NewAggregate(errs)
}

func (sdcc *Controller) syncServices(
	ctx context.Context,
	sdc *scyllav1alpha1.ScyllaDBDatacenter,
	status *scyllav1alpha1.ScyllaDBDatacenterStatus,
	services map[string]*corev1.Service,
	statefulSets map[string]*appsv1.StatefulSet,
	jobs map[string]*batchv1.Job,
) ([]metav1.Condition, error) {
	requiredServices, err := sdcc.makeServices(sdc, services, jobs)
	if err != nil {
		return nil, fmt.Errorf("can't make services: %w", err)
	}

	// Delete any excessive Services.
	// Delete has to be the fist action to avoid getting stuck on quota.
	progressingConditions, err := sdcc.pruneServices(ctx, sdc, requiredServices, services, statefulSets)
	if err != nil {
		return nil, fmt.Errorf("can't delete Service(s): %w", err)
	}

	// We need to first propagate ReplaceAddressFirstBoot from status for the new service.
	for _, svc := range requiredServices {
		_, changed, err := resourceapply.ApplyService(ctx, sdcc.kubeClient.CoreV1(), sdcc.serviceLister, sdcc.eventRecorder, svc, resourceapply.ApplyOptions{})
		if changed {
			controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, serviceControllerProgressingCondition, svc, "apply", sdc.Generation)
		}
		if err != nil {
			return progressingConditions, err
		}
	}

	// Replace members.
	for _, svc := range services {
		_, ok := svc.Labels[naming.ReplaceLabel]
		if !ok {
			continue
		}

		klog.V(4).InfoS("Node is labelled with replacement label and will be replaced", "ScyllaDBDatacenter", klog.KObj(sdc), "Service", klog.KObj(svc))

		supportsReplaceUsingHostID, err := scyllafeatures.Supports(sdc, scyllafeatures.ReplacingNodeUsingHostID)
		if err != nil {
			return progressingConditions, fmt.Errorf("can't determine if ScyllaCluster %q supports replacing using hostID: %w", naming.ObjRef(sdc), err)
		}
		if !supportsReplaceUsingHostID {
			return progressingConditions, fmt.Errorf("can't replace node %q, ScyllaDB version of %q ScyllaCluster doesn't support HostID based replace procedure", naming.ObjRef(svc), naming.ObjRef(sdc))
		}

		klog.V(4).InfoS("Replacing node using HostID", "ScyllaDBDatacenter", klog.KObj(sdc), "Service", klog.KObj(svc))
		pcs, err := sdcc.replaceNodeUsingHostID(ctx, sdc, svc)
		if err != nil {
			return progressingConditions, fmt.Errorf("can't replace node using Host ID: %w", err)
		}

		progressingConditions = append(progressingConditions, pcs...)
	}

	return progressingConditions, nil
}

func (sdcc *Controller) replaceNodeUsingHostID(ctx context.Context, sdc *scyllav1alpha1.ScyllaDBDatacenter, svc *corev1.Service) ([]metav1.Condition, error) {
	var progressingConditions []metav1.Condition

	nodeHostID, ok := svc.Annotations[naming.HostIDAnnotation]
	if !ok {
		progressingConditions = append(progressingConditions, metav1.Condition{
			Type:               serviceControllerProgressingCondition,
			Status:             metav1.ConditionTrue,
			Reason:             "WaitingForHostIDAnnotationBeforeReplacement",
			Message:            fmt.Sprintf("Waiting to observe node Host ID annotation on service %q before node is replaced", naming.ObjRef(svc)),
			ObservedGeneration: sdc.Generation,
		})
		return progressingConditions, nil
	}

	replacingNodeHostID, ok := svc.Labels[naming.ReplacingNodeHostIDLabel]
	if !ok {
		pcs, err := sdcc.initializeReplaceNodeUsingHostID(ctx, sdc, svc, nodeHostID)
		if err != nil {
			return progressingConditions, fmt.Errorf("can't initialize replace using HostID: %w", err)
		}
		progressingConditions = append(progressingConditions, pcs...)
	} else {
		pcs, err := sdcc.finishOngoingReplaceNodeUsingHostID(ctx, sdc, svc, nodeHostID, replacingNodeHostID)
		if err != nil {
			return progressingConditions, fmt.Errorf("can't finish replace using HostID: %w", err)
		}
		progressingConditions = append(progressingConditions, pcs...)
	}

	return progressingConditions, nil
}

func (sdcc *Controller) initializeReplaceNodeUsingHostID(ctx context.Context, sdc *scyllav1alpha1.ScyllaDBDatacenter, svc *corev1.Service, nodeHostID string) ([]metav1.Condition, error) {
	klog.V(2).InfoS("Initiating node replacement", "ScyllaDBDatacenter", klog.KObj(sdc), "Service", klog.KObj(svc), "HostID", nodeHostID)

	var progressingConditions []metav1.Condition

	pcs, err := sdcc.removePodAndAssociatedPVC(ctx, sdc, svc)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't remove ScyllaCluster %q Pod %q and associated PVC: %w", naming.ObjRef(sdc), naming.ObjRef(svc), err)
	}
	progressingConditions = append(progressingConditions, pcs...)

	// Update the member Service.
	klog.V(2).InfoS("Updating member service with Host ID of a node it is replacing",
		"ScyllaDBDatacenter", klog.KObj(sdc),
		"Service", klog.KObj(svc),
		"HostID", svc.Annotations[naming.HostIDAnnotation],
	)
	svcCopy := svc.DeepCopy()
	svcCopy.Labels[naming.ReplacingNodeHostIDLabel] = nodeHostID

	controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, serviceControllerProgressingCondition, svcCopy, "update", sdc.Generation)
	_, err = sdcc.kubeClient.CoreV1().Services(svcCopy.Namespace).Update(ctx, svcCopy, metav1.UpdateOptions{})
	resourceapply.ReportUpdateEvent(sdcc.eventRecorder, svc, err)
	if err != nil {
		return progressingConditions, err
	}

	// We have updated the service. Wait for re-queue.
	progressingConditions = append(progressingConditions, metav1.Condition{
		Type:               serviceControllerProgressingCondition,
		Status:             metav1.ConditionTrue,
		Reason:             "WaitingToObserveUpdatedService",
		Message:            fmt.Sprintf("Waiting to observe updated service %q to proceeed with other services.", naming.ObjRef(svc)),
		ObservedGeneration: sdc.Generation,
	})
	return progressingConditions, nil
}

func (sdcc *Controller) finishOngoingReplaceNodeUsingHostID(ctx context.Context, sdc *scyllav1alpha1.ScyllaDBDatacenter, svc *corev1.Service, nodeHostID, replacingNodeHostID string) ([]metav1.Condition, error) {
	klog.V(4).InfoS("Node is being replaced, awaiting node to become ready", "ScyllaDBDatacenter", klog.KObj(sdc), "Service", klog.KObj(svc))

	var progressingConditions []metav1.Condition

	pod, err := sdcc.podLister.Pods(svc.Namespace).Get(svc.Name)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return progressingConditions, err
		}

		klog.V(2).InfoS("Pod has not been recreated by the StatefulSet controller yet",
			"ScyllaDBDatacenter", klog.KObj(sdc),
			"Pod", klog.KObj(svc),
		)
		progressingConditions = append(progressingConditions, metav1.Condition{
			Type:               serviceControllerProgressingCondition,
			Status:             metav1.ConditionTrue,
			Reason:             "WaitingForPodRecreation",
			Message:            fmt.Sprintf("Service %q is waiting for StatefulSet controller to recreate pod.", naming.ObjRef(svc)),
			ObservedGeneration: sdc.Generation,
		})

		return progressingConditions, nil
	}
	if nodeHostID == replacingNodeHostID {
		klog.V(2).InfoS("Node is still replacing, Host ID annotation wasn't updated",
			"ScyllaDBDatacenter", klog.KObj(sdc),
			"Service", klog.KObj(svc),
			"NodeHostID", nodeHostID,
			"ReplacingNodeHostID", replacingNodeHostID,
		)
		progressingConditions = append(progressingConditions, metav1.Condition{
			Type:               serviceControllerProgressingCondition,
			Status:             metav1.ConditionTrue,
			Reason:             "WaitingForUpdatedHostIDAnnotation",
			Message:            fmt.Sprintf("Service %q is waiting updated Host ID annotation", naming.ObjRef(svc)),
			ObservedGeneration: sdc.Generation,
		})
		return progressingConditions, nil
	}

	sdc = sdcc.resolveScyllaDBDatacenterControllerThroughStatefulSet(pod)
	if sdc == nil {
		return progressingConditions, fmt.Errorf("pod %q is not owned by us anymore", naming.ObjRef(pod))
	}

	// We could still see an old pod in the caches - verify with a live call.
	podReady, pod, err := controllerhelpers.IsPodReadyWithPositiveLiveCheck(ctx, sdcc.kubeClient.CoreV1(), pod)
	if err != nil {
		return progressingConditions, err
	}
	if !podReady {
		klog.V(2).InfoS("Pod isn't ready yet",
			"ScyllaDBDatacenter", klog.KObj(sdc),
			"Pod", klog.KObj(pod),
		)
		progressingConditions = append(progressingConditions, metav1.Condition{
			Type:               serviceControllerProgressingCondition,
			Status:             metav1.ConditionTrue,
			Reason:             "WaitingForPodReadiness",
			Message:            fmt.Sprintf("Service %q is waiting for Pod %q to become ready.", naming.ObjRef(svc), naming.ObjRef(pod)),
			ObservedGeneration: sdc.Generation,
		})
		return progressingConditions, nil
	}

	sdcc.eventRecorder.Eventf(svc, corev1.EventTypeNormal, "FinishedReplacingNode", "New pod %s/%s is ready.", pod.Namespace, pod.Name)
	svcCopy := svc.DeepCopy()
	delete(svcCopy.Labels, naming.ReplaceLabel)
	delete(svcCopy.Labels, naming.ReplacingNodeHostIDLabel)
	controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, serviceControllerProgressingCondition, svcCopy, "update", sdc.Generation)
	_, err = sdcc.kubeClient.CoreV1().Services(svcCopy.Namespace).Update(ctx, svcCopy, metav1.UpdateOptions{})
	resourceapply.ReportUpdateEvent(sdcc.eventRecorder, svc, err)
	if err != nil {
		return progressingConditions, err
	}

	return progressingConditions, nil
}

func (sdcc *Controller) removePodAndAssociatedPVC(ctx context.Context, sdc *scyllav1alpha1.ScyllaDBDatacenter, svc *corev1.Service) ([]metav1.Condition, error) {
	var progressingConditions []metav1.Condition
	backgroundPropagationPolicy := metav1.DeletePropagationBackground

	// First, Delete the PVC if it exists.
	// The PVC has finalizer protection so it will wait for the pod to be deleted.
	// We can't do it the other way around or the pod could be recreated before we delete the PVC.
	pvcMeta := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: svc.Namespace,
			Name:      naming.PVCNameForPod(svc.Name),
		},
	}
	klog.V(2).InfoS("Deleting the PVC to replace member",
		"ScyllaDBDatacenter", klog.KObj(sdc),
		"Service", klog.KObj(svc),
		"PVC", klog.KObj(pvcMeta),
	)
	controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, serviceControllerProgressingCondition, pvcMeta, "delete", sdc.Generation)
	err := sdcc.kubeClient.CoreV1().PersistentVolumeClaims(pvcMeta.Namespace).Delete(ctx, pvcMeta.Name, metav1.DeleteOptions{
		PropagationPolicy: &backgroundPropagationPolicy,
	})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			resourceapply.ReportDeleteEvent(sdcc.eventRecorder, pvcMeta, err)
			return progressingConditions, err
		}
		klog.V(4).InfoS("PVC not found", "PVC", klog.KObj(pvcMeta))
	}
	resourceapply.ReportDeleteEvent(sdcc.eventRecorder, pvcMeta, nil)

	// Hack: Sleeps are terrible thing to in sync logic but this compensates for a broken
	// StatefulSet controller in Kubernetes. StatefulSet doesn't reconcile PVCs and decides
	// it's presence only from its cache, and never recreates it if it's missing, only when it
	// creates the pod. This gives StatefulSet controller a chance to see the PVC was deleted
	// before deleting the pod.
	progressingConditions = append(progressingConditions, metav1.Condition{
		Type:               serviceControllerProgressingCondition,
		Status:             metav1.ConditionTrue,
		Reason:             "WaitingForStatefulSetToObservePVCDeletion",
		Message:            "Waiting for StatefulSet controller to observe PVC deletion.",
		ObservedGeneration: sdc.Generation,
	})
	time.Sleep(10 * time.Second)

	// Evict the Pod if it exists.
	podMeta := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: svc.Namespace,
			Name:      svc.Name,
		},
	}
	klog.V(2).InfoS("Deleting the Pod to replace member",
		"ScyllaDBDatacenter", klog.KObj(sdc),
		"Service", klog.KObj(svc),
		"Pod", klog.KObj(podMeta),
	)
	controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, serviceControllerProgressingCondition, podMeta, "delete", sdc.Generation)
	err = sdcc.kubeClient.CoreV1().Pods(podMeta.Namespace).EvictV1(ctx, &policyv1.Eviction{
		ObjectMeta: metav1.ObjectMeta{
			Name: podMeta.Name,
		},
		DeleteOptions: &metav1.DeleteOptions{
			PropagationPolicy: &backgroundPropagationPolicy,
		},
	})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			resourceapply.ReportDeleteEvent(sdcc.eventRecorder, podMeta, err)
			return progressingConditions, err
		}
		klog.V(4).InfoS("Pod not found", "Pod", klog.ObjectRef{Namespace: svc.Namespace, Name: svc.Name})
	}
	resourceapply.ReportDeleteEvent(sdcc.eventRecorder, podMeta, nil)

	return progressingConditions, nil
}
