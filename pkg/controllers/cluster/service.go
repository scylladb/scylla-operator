package cluster

import (
	"context"

	"github.com/pkg/errors"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/v1"
	"github.com/scylladb/scylla-operator/pkg/controllers/cluster/resource"
	"github.com/scylladb/scylla-operator/pkg/naming"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// syncClusterHeadlessService checks if a Headless Service exists
// for the given Cluster, in order for the StatefulSets to utilize it.
// If it doesn't exists, then create it.
func (cc *ClusterReconciler) syncClusterHeadlessService(ctx context.Context, c *scyllav1.ScyllaCluster) error {
	clusterHeadlessService := resource.HeadlessServiceForCluster(c)
	_, err := controllerutil.CreateOrUpdate(ctx, cc.Client, clusterHeadlessService, serviceMutateFn(ctx, clusterHeadlessService, cc.Client))
	if err != nil {
		return errors.Wrapf(err, "error syncing headless service %s", clusterHeadlessService.Name)
	}
	return nil
}

// syncMemberServices checks, for every Pod of the Cluster that
// has been created, if a corresponding ClusterIP Service exists,
// which will serve as a static ip.
// If it doesn't exist, it creates it.
// It also assigns the first two members of each rack as seeds.
func (cc *ClusterReconciler) syncMemberServices(ctx context.Context, c *scyllav1.ScyllaCluster) error {
	podlist := &corev1.PodList{}

	// For every Pod of the cluster that exists, check that a
	// a corresponding ClusterIP Service exists, and if it doesn't,
	// create it.
	for _, r := range c.Spec.Datacenter.Racks {
		// Get all Pods for this rack
		opts := client.MatchingLabels(naming.RackLabels(r, c))
		err := cc.List(ctx, podlist, opts)
		if err != nil {
			return errors.Wrapf(err, "listing pods for rack %s failed", r.Name)
		}
		for _, pod := range podlist.Items {
			memberService := resource.MemberServiceForPod(&pod, c)
			op, err := controllerutil.CreateOrUpdate(ctx, cc.Client, memberService, serviceMutateFn(ctx, memberService, cc.Client))
			if err != nil {
				return errors.Wrapf(err, "error syncing member service %s", memberService.Name)
			}
			switch op {
			case controllerutil.OperationResultCreated:
				cc.Logger.Info(ctx, "Member service created", "member", memberService.Name, "labels", memberService.Labels)
			case controllerutil.OperationResultUpdated:
				cc.Logger.Info(ctx, "Member service updated", "member", memberService.Name, "labels", memberService.Labels)
			}
		}
	}
	return nil
}

// syncService checks if the given Service exists and creates it if it doesn't
// it creates it
func serviceMutateFn(ctx context.Context, newService *corev1.Service, client client.Client) func() error {
	return func() error {
		// TODO: probably nothing has to be done, check v1 implementation of CreateOrUpdate
		//existingService := existing.(*corev1.Service)
		//if !reflect.DeepEqual(newService.Spec, existingService.Spec) {
		//	return client.Update(ctx, existing)
		//}
		return nil
	}
}
