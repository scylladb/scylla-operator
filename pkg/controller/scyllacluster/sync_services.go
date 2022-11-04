package scyllacluster

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"time"

	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/resourceapply"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
)

var serviceOrdinalRegex = regexp.MustCompile("^.*-([0-9]+)$")

func (scc *Controller) makeServices(sc *scyllav1.ScyllaCluster, oldServices map[string]*corev1.Service) []*corev1.Service {
	services := []*corev1.Service{
		IdentityService(sc),
	}

	for _, rack := range sc.Spec.Datacenter.Racks {
		stsName := naming.StatefulSetNameForRack(rack, sc)

		for ord := int32(0); ord < rack.Members; ord++ {
			svcName := fmt.Sprintf("%s-%d", stsName, ord)
			oldSvc := oldServices[svcName]
			services = append(services, MemberService(sc, rack.Name, svcName, oldSvc))
		}
	}

	return services
}

func (scc *Controller) pruneServices(
	ctx context.Context,
	sc *scyllav1.ScyllaCluster,
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
		stsName := fmt.Sprintf("%s-%s-%s", sc.Name, sc.Spec.Datacenter.Name, rackName)
		sts, ok := statefulSets[stsName]
		if !ok {
			errs = append(errs, fmt.Errorf("statefulset %s/%s is missing", sc.Namespace, stsName))
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
				ObservedGeneration: sc.Generation,
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
				ObservedGeneration: sc.Generation,
			})
			continue
		}

		// Because we also delete the PVC, we need to recheck the service state with a live call.
		// We can't delete the PVC after the service because the deletion wouldn't be retried.
		{
			freshSvc, err := scc.kubeClient.CoreV1().Services(svc.Namespace).Get(ctx, svc.Name, metav1.GetOptions{})
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
					ObservedGeneration: sc.Generation,
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
					ObservedGeneration: sc.Generation,
				})
				continue
			}
		}

		backgroundPropagationPolicy := metav1.DeletePropagationBackground

		controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, serviceControllerProgressingCondition, &corev1.PersistentVolumeClaim{}, "delete", sc.Generation)
		pvcName := naming.PVCNameForService(svc.Name)
		err = scc.kubeClient.CoreV1().PersistentVolumeClaims(svc.Namespace).Delete(ctx, pvcName, metav1.DeleteOptions{
			PropagationPolicy: &backgroundPropagationPolicy,
		})
		if err != nil && !apierrors.IsNotFound(err) {
			errs = append(errs, err)
			continue
		}

		controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, serviceControllerProgressingCondition, svc, "delete", sc.Generation)
		err = scc.kubeClient.CoreV1().Services(svc.Namespace).Delete(ctx, svc.Name, metav1.DeleteOptions{
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

func (scc *Controller) syncServices(
	ctx context.Context,
	sc *scyllav1.ScyllaCluster,
	status *scyllav1.ScyllaClusterStatus,
	services map[string]*corev1.Service,
	statefulSets map[string]*appsv1.StatefulSet,
) ([]metav1.Condition, error) {
	var err error

	requiredServices := scc.makeServices(sc, services)

	// Delete any excessive Services.
	// Delete has to be the fist action to avoid getting stuck on quota.
	progressingConditions, err := scc.pruneServices(ctx, sc, requiredServices, services, statefulSets)
	if err != nil {
		return nil, fmt.Errorf("can't delete Service(s): %w", err)
	}

	// We need to first propagate ReplaceAddressFirstBoot from status for the new service.
	for _, svc := range requiredServices {
		_, changed, err := resourceapply.ApplyService(ctx, scc.kubeClient.CoreV1(), scc.serviceLister, scc.eventRecorder, svc, false)
		if changed {
			controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, serviceControllerProgressingCondition, svc, "apply", sc.Generation)
		}
		if err != nil {
			return progressingConditions, err
		}
	}

	// Replace members.
	for _, svc := range services {
		replaceAddr, hasReplaceAnnotation := svc.Annotations[naming.ReplaceAnnotation]
		// Maintain backwards compatibility
		replaceAddrLabel, hasReplaceLabel := svc.Labels[naming.ReplaceAnnotation]
		if !hasReplaceLabel && !hasReplaceAnnotation {
			continue
		}
		if hasReplaceLabel && !hasReplaceAnnotation {
			replaceAddr = replaceAddrLabel
		}

		rackName, ok := svc.Labels[naming.RackNameLabel]
		if !ok {
			return progressingConditions, fmt.Errorf("service %s/%s is missing %q label", svc.Namespace, svc.Name, naming.RackNameLabel)
		}

		rackStatus := status.Racks[rackName]
		controllerhelpers.SetRackCondition(&rackStatus, scyllav1.RackConditionTypeMemberReplacing)
		status.Racks[rackName] = rackStatus

		if len(replaceAddr) == 0 {
			// Member needs to be replaced.

			// TODO: Can't we just use the same service and keep it?
			//       The pod has identity so there will never be 2 at the same time.

			// Save replace address in RackStatus
			if len(status.Racks[rackName].ReplaceAddressFirstBoot[svc.Name]) == 0 {
				status.Racks[rackName].ReplaceAddressFirstBoot[svc.Name] = svc.Spec.ClusterIP
				klog.V(2).InfoS("Adding member address to replace address list", "Member", svc.Name, "IP", svc.Spec.ClusterIP, "ReplaceAddresses", status.Racks[rackName].ReplaceAddressFirstBoot)

				// Make sure the address is stored before proceeding further.
				progressingConditions = append(progressingConditions, metav1.Condition{
					Type:               serviceControllerProgressingCondition,
					Status:             metav1.ConditionTrue,
					Reason:             "WaitingToPickUpReplaceAddress",
					Message:            fmt.Sprintf("Service %q is waiting to pick up stored replace address.", naming.ObjRef(svc)),
					ObservedGeneration: sc.Generation,
				})
				return progressingConditions, nil
			}

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
				"ScyllaCluster", klog.KObj(sc),
				"Service", klog.KObj(svc),
				"PVC", klog.KObj(pvcMeta),
			)
			controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, serviceControllerProgressingCondition, pvcMeta, "delete", sc.Generation)
			err = scc.kubeClient.CoreV1().PersistentVolumeClaims(pvcMeta.Namespace).Delete(ctx, pvcMeta.Name, metav1.DeleteOptions{
				PropagationPolicy: &backgroundPropagationPolicy,
			})
			if err != nil {
				if apierrors.IsNotFound(err) {
					klog.V(4).InfoS("PVC not found", "PVC", klog.KObj(pvcMeta))
				} else {
					resourceapply.ReportDeleteEvent(scc.eventRecorder, pvcMeta, err)
					return progressingConditions, err
				}
			}
			resourceapply.ReportDeleteEvent(scc.eventRecorder, pvcMeta, nil)

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
				ObservedGeneration: sc.Generation,
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
				"ScyllaCluster", klog.KObj(sc),
				"Service", klog.KObj(svc),
				"Pod", klog.KObj(podMeta),
			)
			controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, serviceControllerProgressingCondition, podMeta, "delete", sc.Generation)
			// TODO: Revert back to eviction when it's fixed in kubernetes 1.19.z (#732)
			//       (https://github.com/kubernetes/kubernetes/issues/103970)
			err = scc.kubeClient.CoreV1().Pods(podMeta.Namespace).Delete(ctx, podMeta.Name, metav1.DeleteOptions{
				PropagationPolicy: &backgroundPropagationPolicy,
			})
			if err != nil {
				if apierrors.IsNotFound(err) {
					klog.V(4).InfoS("Pod not found", "Pod", klog.ObjectRef{Namespace: svc.Namespace, Name: svc.Name})
				} else {
					resourceapply.ReportDeleteEvent(scc.eventRecorder, podMeta, err)
					return progressingConditions, err
				}
			}
			resourceapply.ReportDeleteEvent(scc.eventRecorder, podMeta, nil)

			// Delete the member Service.
			klog.V(2).InfoS("Deleting the member service to replace member",
				"ScyllaCluster", klog.KObj(sc),
				"Service", klog.KObj(svc),
			)
			controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, serviceControllerProgressingCondition, svc, "delete", sc.Generation)
			err = scc.kubeClient.CoreV1().Services(svc.Namespace).Delete(ctx, svc.Name, metav1.DeleteOptions{
				PropagationPolicy: &backgroundPropagationPolicy,
				Preconditions: &metav1.Preconditions{
					UID: &svc.UID,
					// Delete only if its the version we see in cache. (Has the same replace label state.)
					ResourceVersion: &svc.ResourceVersion,
				},
			})
			if err != nil {
				if apierrors.IsNotFound(err) {
					klog.V(4).InfoS("Pod not found", "Pod", klog.ObjectRef{Namespace: svc.Namespace, Name: svc.Name})
				} else {
					resourceapply.ReportDeleteEvent(scc.eventRecorder, svc, err)
					return progressingConditions, err
				}
			} else {
				resourceapply.ReportDeleteEvent(scc.eventRecorder, svc, nil)

				// FIXME: The pod could have been already up and read the old ClusterIP - make sure it will restart.
				//        We can't delete the pod here as it wouldn't retry failures.

				// We have deleted the service. Wait for re-queue.
				progressingConditions = append(progressingConditions, metav1.Condition{
					Type:               serviceControllerProgressingCondition,
					Status:             metav1.ConditionTrue,
					Reason:             "WaitingToObserveDeletedService",
					Message:            fmt.Sprintf("Waiting to observe service %q as deleted to proceeed with other services.", naming.ObjRef(svc)),
					ObservedGeneration: sc.Generation,
				})
				return progressingConditions, nil
			}
		} else {
			// Member is being replaced. Wait for readiness and clear the replace label.

			pod, err := scc.podLister.Pods(svc.Namespace).Get(svc.Name)
			if err != nil {
				if !apierrors.IsNotFound(err) {
					return progressingConditions, err
				}

				klog.V(2).InfoS("Pod has not been recreated by the StatefulSet controller yet",
					"ScyllaCluster", klog.KObj(sc),
					"Pod", klog.KObj(pod),
				)
				progressingConditions = append(progressingConditions, metav1.Condition{
					Type:               serviceControllerProgressingCondition,
					Status:             metav1.ConditionTrue,
					Reason:             "WaitingForPodRecreation",
					Message:            fmt.Sprintf("Service %q is waiting for StatefulSet controller to recreate pod %q.", naming.ObjRef(svc), naming.ObjRef(pod)),
					ObservedGeneration: sc.Generation,
				})
			} else {
				sc := scc.resolveScyllaClusterControllerThroughStatefulSet(pod)
				if sc == nil {
					return progressingConditions, fmt.Errorf("pod %q is not owned by us anymore", naming.ObjRef(pod))
				}

				// We could still see an old pod in the caches - verify with a live call.
				podReady, pod, err := controllerhelpers.IsPodReadyWithPositiveLiveCheck(ctx, scc.kubeClient.CoreV1(), pod)
				if err != nil {
					return progressingConditions, err
				}
				if podReady {
					scc.eventRecorder.Eventf(svc, corev1.EventTypeNormal, "FinishedReplacingNode", "New pod %s/%s is ready.", pod.Namespace, pod.Name)
					svcCopy := svc.DeepCopy()
					if hasReplaceAnnotation {
						delete(svcCopy.Annotations, naming.ReplaceAnnotation)
					}
					if hasReplaceLabel {
						delete(svcCopy.Labels, naming.ReplaceAnnotation)
					}
					controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, serviceControllerProgressingCondition, svcCopy, "update", sc.Generation)
					_, err := scc.kubeClient.CoreV1().Services(svcCopy.Namespace).Update(ctx, svcCopy, metav1.UpdateOptions{})
					resourceapply.ReportUpdateEvent(scc.eventRecorder, svc, err)
					if err != nil {
						return progressingConditions, err
					}

					delete(status.Racks[rackName].ReplaceAddressFirstBoot, svc.Name)
				} else {
					klog.V(2).InfoS("Pod isn't ready yet",
						"ScyllaCluster", klog.KObj(sc),
						"Pod", klog.KObj(pod),
					)
					progressingConditions = append(progressingConditions, metav1.Condition{
						Type:               serviceControllerProgressingCondition,
						Status:             metav1.ConditionTrue,
						Reason:             "WaitingForPodReadiness",
						Message:            fmt.Sprintf("Service %q is waiting for Pod %q to become ready.", naming.ObjRef(svc), naming.ObjRef(pod)),
						ObservedGeneration: sc.Generation,
					})
				}
			}
		}
	}

	return progressingConditions, nil
}
