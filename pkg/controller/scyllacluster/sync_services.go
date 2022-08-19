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
) error {
	var errs []error
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
			continue
		}

		// Do not delete services that weren't properly decommissioned.
		if svc.Labels[naming.DecommissionedLabel] != naming.LabelValueTrue {
			klog.Warningf("Refusing to cleanup service %s/%s whose member wasn't decommissioned.", svc.Namespace, svc.Name)
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
				continue
			}

			if freshSvc.Labels[naming.DecommissionedLabel] != naming.LabelValueTrue {
				klog.V(2).InfoS("Stale caches, won't delete the pvc because the service is no longer decommissioned.", "Service", klog.KObj(svc))
				continue
			}
		}

		backgroundPropagationPolicy := metav1.DeletePropagationBackground

		pvcName := naming.PVCNameForService(svc.Name)
		err = scc.kubeClient.CoreV1().PersistentVolumeClaims(svc.Namespace).Delete(ctx, pvcName, metav1.DeleteOptions{
			PropagationPolicy: &backgroundPropagationPolicy,
		})
		if err != nil && !apierrors.IsNotFound(err) {
			errs = append(errs, err)
			continue
		}

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
	return utilerrors.NewAggregate(errs)
}

func (scc *Controller) syncServices(
	ctx context.Context,
	sc *scyllav1.ScyllaCluster,
	status *scyllav1.ScyllaClusterStatus,
	services map[string]*corev1.Service,
	statefulSets map[string]*appsv1.StatefulSet,
) (*scyllav1.ScyllaClusterStatus, error) {
	var err error

	requiredServices := scc.makeServices(sc, services)

	// Delete any excessive Services.
	// Delete has to be the fist action to avoid getting stuck on quota.
	err = scc.pruneServices(ctx, sc, requiredServices, services, statefulSets)
	if err != nil {
		return status, fmt.Errorf("can't delete Service(s): %w", err)
	}

	// We need to first propagate ReplaceAddressFirstBoot from status for the new service.
	for _, svc := range requiredServices {
		_, _, err = resourceapply.ApplyService(ctx, scc.kubeClient.CoreV1(), scc.serviceLister, scc.eventRecorder, svc, false)
		if err != nil {
			return status, err
		}
	}

	// Replace members.
	for _, svc := range services {
		replaceAddr, ok := svc.Labels[naming.ReplaceLabel]
		if !ok {
			continue
		}

		rackName, ok := svc.Labels[naming.RackNameLabel]
		if !ok {
			return status, fmt.Errorf("service %s/%s is missing %q label", svc.Namespace, svc.Name, naming.RackNameLabel)
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
				return status, nil
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
			err = scc.kubeClient.CoreV1().PersistentVolumeClaims(pvcMeta.Namespace).Delete(ctx, pvcMeta.Name, metav1.DeleteOptions{
				PropagationPolicy: &backgroundPropagationPolicy,
			})
			if err != nil {
				if apierrors.IsNotFound(err) {
					klog.V(4).InfoS("PVC not found", "PVC", klog.KObj(pvcMeta))
				} else {
					resourceapply.ReportDeleteEvent(scc.eventRecorder, pvcMeta, err)
					return status, err
				}
			}
			resourceapply.ReportDeleteEvent(scc.eventRecorder, pvcMeta, nil)

			// Hack: Sleeps are terrible thing to in sync logic but this compensates for a broken
			// StatefulSet controller in Kubernetes. StatefulSet doesn't reconcile PVCs and decides
			// it's presence only from its cache, and never recreates it if it's missing, only when it
			// creates the pod. This gives StatefulSet controller a chance to see the PVC was deleted
			// before deleting the pod.
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
					return status, err
				}
			}
			resourceapply.ReportDeleteEvent(scc.eventRecorder, podMeta, nil)

			// Delete the member Service.
			klog.V(2).InfoS("Deleting the member service to replace member",
				"ScyllaCluster", klog.KObj(sc),
				"Service", klog.KObj(svc),
			)
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
					return status, err
				}
			} else {
				resourceapply.ReportDeleteEvent(scc.eventRecorder, svc, nil)

				// FIXME: The pod could have been already up and read the old ClusterIP - make sure it will restart.
				//        We can't delete the pod here as it wouldn't retry failures.

				// We have deleted the service. Wait for re-queue.
				return status, nil
			}
		} else {
			// Member is being replaced. Wait for readiness and clear the replace label.

			pod, err := scc.podLister.Pods(svc.Namespace).Get(svc.Name)
			if err != nil {
				if !apierrors.IsNotFound(err) {
					return status, err
				}

				klog.V(2).InfoS("Pod has not been recreated by the StatefulSet controller yet",
					"ScyllaCluster", klog.KObj(sc),
					"Pod", klog.KObj(pod),
				)
			} else {
				sc := scc.resolveScyllaClusterControllerThroughStatefulSet(pod)
				if sc == nil {
					klog.ErrorS(nil, "Pod matching our selector is not owned by us.",
						"ScyllaCluster", klog.KObj(sc),
						"Pod", klog.KObj(pod),
					)
					// User needs to fix the collision manually so there
					// is no point in returning an error and retrying.
					return status, nil
				}

				// We could still see an old pod in the caches - verify with a live call.
				podReady, pod, err := controllerhelpers.IsPodReadyWithPositiveLiveCheck(ctx, scc.kubeClient.CoreV1(), pod)
				if err != nil {
					return status, err
				}
				if podReady {
					scc.eventRecorder.Eventf(svc, corev1.EventTypeNormal, "FinishedReplacingNode", "New pod %s/%s is ready.", pod.Namespace, pod.Name)
					svcCopy := svc.DeepCopy()
					delete(svcCopy.Labels, naming.ReplaceLabel)
					_, err := scc.kubeClient.CoreV1().Services(svcCopy.Namespace).Update(ctx, svcCopy, metav1.UpdateOptions{})
					resourceapply.ReportUpdateEvent(scc.eventRecorder, svc, err)
					if err != nil {
						return status, err
					}

					delete(status.Racks[rackName].ReplaceAddressFirstBoot, svc.Name)
				} else {
					klog.V(2).InfoS("Pod isn't ready yet",
						"ScyllaCluster", klog.KObj(sc),
						"Pod", klog.KObj(pod),
					)
				}
			}
		}
	}

	return status, nil
}
