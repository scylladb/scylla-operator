package cluster

import (
	"context"
	"github.com/davecgh/go-spew/spew"
	"github.com/pkg/errors"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/apis/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controller/cluster/util"
	"github.com/scylladb/scylla-operator/pkg/naming"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// cleanup deletes all resources remaining because of cluster scale downs
func (cc *ClusterController) cleanup(c *scyllav1alpha1.Cluster) error {

	svcList := &corev1.ServiceList{}
	sts := &appsv1.StatefulSet{}
	logger := util.LoggerForCluster(c)

	for _, r := range c.Spec.Datacenter.Racks {

		// Get rack status. If it doesn't exist, the rack isn't yet created.
		stsName := naming.StatefulSetNameForRack(r, c)
		err := cc.Get(context.TODO(), naming.NamespacedName(stsName, c.Namespace), sts)
		if apierrors.IsNotFound(err) {
			continue
		}
		if err != nil {
			return errors.Wrap(err, "error getting statefulset")
		}

		// Get all member services
		err = cc.List(
			context.TODO(),
			&client.ListOptions{LabelSelector: naming.RackSelector(r, c)},
			svcList,
		)
		if err != nil {
			return errors.Wrap(err, "error listing member services")
		}
		logger.Debugf("Cleanup: servicelist is %v", spew.Sdump(svcList.Items))

		memberCount := *sts.Spec.Replicas
		memberServiceCount := int32(len(svcList.Items))
		// If there are more services than members, some services need to be cleaned up
		if memberServiceCount > memberCount {
			// maxIndex is the maximum index that should be present in a
			// member service of this rack
			maxIndex := memberCount - 1
			for _, svc := range svcList.Items {
				svcIndex, err := naming.IndexFromName(svc.Name)
				if err != nil {
					return errors.WithStack(err)
				}
				if svcIndex > maxIndex && svc.Labels[naming.DecommissionLabel] == naming.LabelValueTrue {
					err := cc.cleanupMemberResources(&svc, r, c)
					if err != nil {
						return errors.WithStack(err)
					}
				}
			}
		}
	}
	return nil
}

// cleanupMemberResources deletes all resources associated with a given member.
// Currently those are :
//  - A PVC
//  - A ClusterIP Service
func (cc *ClusterController) cleanupMemberResources(memberService *corev1.Service, r scyllav1alpha1.RackSpec, c *scyllav1alpha1.Cluster) error {

	memberName := memberService.Name
	logger := util.LoggerForCluster(c)
	logger.Infof("Cleaning up resources for member %s", memberName)
	// Delete PVC if it exists
	pvc := &corev1.PersistentVolumeClaim{}
	err := cc.Get(
		context.TODO(),
		naming.NamespacedName(naming.PVCNameForPod(memberName), c.Namespace),
		pvc,
	)
	// If PVC is found
	if !apierrors.IsNotFound(err) {
		// If there's another error return
		if err != nil {
			return errors.Wrap(err, "failed to get pvc")
		}
		// Else delete the PVC
		if err = cc.Delete(context.TODO(), pvc); err != nil {
			return errors.Wrap(err, "failed to delete pvc")
		}
	}

	// Delete Member Service
	if err = cc.Delete(context.TODO(), memberService); err != nil && !apierrors.IsNotFound(err) {
		return errors.Wrap(err, "failed to delete member service")
	}

	return nil
}
