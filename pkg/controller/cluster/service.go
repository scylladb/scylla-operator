package cluster

import (
	"context"
	"github.com/pkg/errors"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/apis/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controller/cluster/resource"
	"github.com/scylladb/scylla-operator/pkg/naming"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// syncClusterHeadlessService checks if a Headless Service exists
// for the given Cluster, in order for the StatefulSets to utilize it.
// If it doesn't exists, then create it.
func (cc *ClusterController) syncClusterHeadlessService(c *scyllav1alpha1.Cluster) error {

	clusterHeadlessService := resource.HeadlessServiceForCluster(c)
	_, err := controllerutil.CreateOrUpdate(
		context.TODO(),
		cc.Client,
		clusterHeadlessService,
		serviceMutateFn(clusterHeadlessService, cc.Client),
	)
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
func (cc *ClusterController) syncMemberServices(c *scyllav1alpha1.Cluster) error {

	podlist := &corev1.PodList{}

	// For every Pod of the cluster that exists, check that a
	// a corresponding ClusterIP Service exists, and if it doesn't,
	// create it.
	for _, r := range c.Spec.Datacenter.Racks {

		// Get all Pods for this rack
		opts := client.MatchingLabels(naming.RackLabels(r, c))
		err := cc.List(context.TODO(), opts, podlist)
		if err != nil {
			return errors.Wrapf(err, "listing pods for rack %s failed", r.Name)
		}
		for _, pod := range podlist.Items {
			memberService := resource.MemberServiceForPod(&pod, c)
			_, err := controllerutil.CreateOrUpdate(
				context.TODO(),
				cc.Client,
				memberService,
				serviceMutateFn(memberService, cc.Client),
			)
			if err != nil {
				return errors.Wrapf(err, "error syncing member service %s", memberService.Name)
			}
		}
	}
	return nil
}

// syncService checks if the given Service exists and creates it if it doesn't
// it creates it
func serviceMutateFn(newService *corev1.Service, client client.Client) func(existing runtime.Object) error {
	return func(existing runtime.Object) error {
		existingService := existing.(*corev1.Service)
		if !reflect.DeepEqual(newService.Spec, existingService.Spec) {
			return client.Update(context.TODO(), existing)
		}
		return nil
	}
}
