package orphanedpv

import (
	"context"
	"fmt"
	"time"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/ctrlclient"
	"github.com/scylladb/scylla-operator/pkg/naming"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	apimachineryutilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type PVItem struct {
	PV          *corev1.PersistentVolume
	ServiceName string
}

func (opc *Controller) getPVsForScyllaDBDatacenter(ctx context.Context, sdc *scyllav1alpha1.ScyllaDBDatacenter) ([]*PVItem, []string, error) {
	var errs []error
	var requeueReasons []string
	var pis []*PVItem
	for _, rack := range sdc.Spec.Racks {
		stsName := naming.StatefulSetNameForRack(rack, sdc)
		rackNodeCount, err := controllerhelpers.GetRackNodeCount(sdc, rack.Name)
		if err != nil {
			return nil, nil, fmt.Errorf("can't get rack %q node count of ScyllaDBDatacenter %q: %w", rack.Name, naming.ObjRef(sdc), err)
		}

		for i := int32(0); i < *rackNodeCount; i++ {
			svcName := fmt.Sprintf("%s-%d", stsName, i)
			pvcName := fmt.Sprintf("%s-%s", naming.PVCTemplateName, svcName)
			pvc, err := ctrlclient.Get[corev1.PersistentVolumeClaim](ctx, opc.client, sdc.Namespace, pvcName)
			if err != nil {
				if apierrors.IsNotFound(err) {
					klog.V(2).InfoS("PVC not found", "PVC", fmt.Sprintf("%s/%s", sdc.Namespace, pvcName))
					// We aren't watching PVCs so we need to requeue manually
					requeueReasons = append(requeueReasons, "PVC not found")
					continue
				}
				errs = append(errs, err)
				continue
			}

			if len(pvc.Spec.VolumeName) == 0 {
				klog.V(2).InfoS("PVC not bound yet", "PVC", klog.KObj(pvc))
				requeueReasons = append(requeueReasons, "PVC not bound yet")
				continue
			}

			pv, err := ctrlclient.Get[corev1.PersistentVolume](ctx, opc.client, corev1.NamespaceAll, pvc.Spec.VolumeName)
			if err != nil {
				errs = append(errs, err)
				continue
			}

			pis = append(pis, &PVItem{
				PV:          pv,
				ServiceName: svcName,
			})
		}
	}

	return pis, requeueReasons, apimachineryutilerrors.NewAggregate(errs)
}

func (opc *Controller) sync(ctx context.Context, key types.NamespacedName) error {
	namespace, name := key.Namespace, key.Name

	startTime := time.Now()
	klog.V(4).InfoS("Started syncing ScyllaDBDatacenter", "ScyllaDBDatacenter", klog.KRef(namespace, name), "startTime", startTime)
	defer func() {
		klog.V(4).InfoS("Finished syncing ScyllaDBDatacenter", "ScyllaDBDatacenter", klog.KRef(namespace, name), "duration", time.Since(startTime))
	}()

	sdc, err := ctrlclient.Get[scyllav1alpha1.ScyllaDBDatacenter](ctx, opc.client, namespace, name)
	if apierrors.IsNotFound(err) {
		klog.V(2).InfoS("ScyllaDBDatacenter has been deleted", "ScyllaDBDatacenter", klog.KRef(namespace, name))
		return nil
	}
	if err != nil {
		return err
	}

	if sdc.DeletionTimestamp != nil {
		return nil
	}

	if sdc.Spec.DisableAutomaticOrphanedNodeReplacement == nil || *sdc.Spec.DisableAutomaticOrphanedNodeReplacement {
		klog.V(4).InfoS("ScyllaDBDatacenter has AutomaticOrphanedNodeReplacement disabled", "ScyllaDBDatacenter", klog.KObj(sdc))
		return nil
	}

	nodes, err := ctrlclient.List[corev1.Node](ctx, opc.client, corev1.NamespaceAll, labels.Everything())
	if err != nil {
		return err
	}

	var errs []error

	pis, requeueReasons, err := opc.getPVsForScyllaDBDatacenter(ctx, sdc)
	// Process at least some PVs even if there were errors retrieving the rest
	if err != nil {
		errs = append(errs, err)
	}

	for _, pi := range pis {
		orphaned, err := controllerhelpers.IsOrphanedPV(pi.PV, nodes)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		if !orphaned {
			continue
		}

		klog.V(2).InfoS("PV is orphaned", "ScyllaDBDatacenter", klog.KObj(sdc), "PV", klog.KObj(pi.PV))

		// Verify that the node doesn't exist with a live call.
		freshNodes, err := ctrlclient.List[corev1.Node](ctx, opc.apiReader, corev1.NamespaceAll, labels.Everything())
		if err != nil {
			errs = append(errs, err)
			continue
		}

		freshOrphaned, err := controllerhelpers.IsOrphanedPV(pi.PV, freshNodes)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		if !freshOrphaned {
			continue
		}

		klog.V(2).InfoS("PV is verified as orphaned.", "ScyllaDBDatacenter", klog.KObj(sdc), "PV", klog.KObj(pi.PV))

		err = opc.client.Patch(ctx, &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: sdc.Namespace,
				Name:      pi.ServiceName,
			},
		}, client.RawPatch(types.MergePatchType, []byte(fmt.Sprintf(`{"metadata": {"labels": {%q: ""} } }`, naming.ReplaceLabel))))
		if err != nil {
			errs = append(errs, err)
			continue
		}

		klog.V(2).InfoS("Marked service for replacement", "ScyllaDBDatacenter", klog.KObj(sdc), "Service", klog.KRef(sdc.Namespace, pi.ServiceName))
	}

	err = apimachineryutilerrors.NewAggregate(errs)
	if err != nil {
		return err
	}

	if len(requeueReasons) > 0 {
		return controllerhelpers.NewRequeueError(requeueReasons...)
	}

	return nil
}
