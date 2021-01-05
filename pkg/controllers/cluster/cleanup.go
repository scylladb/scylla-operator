package cluster

import (
	"context"

	"github.com/pkg/errors"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/v1"
	"github.com/scylladb/scylla-operator/pkg/controllers/cluster/util"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/util/nodeaffinity"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// cleanup deletes all resources remaining because of cluster scale downs
func (cc *ClusterReconciler) cleanup(ctx context.Context, c *scyllav1.ScyllaCluster) error {
	svcList := &corev1.ServiceList{}
	sts := &appsv1.StatefulSet{}
	logger := cc.Logger.With("cluster", c.Namespace+"/"+c.Name, "resourceVersion", c.ResourceVersion)

	for _, r := range c.Spec.Datacenter.Racks {
		// Get rack status. If it doesn't exist, the rack isn't yet created.
		stsName := naming.StatefulSetNameForRack(r, c)
		err := cc.Get(ctx, naming.NamespacedName(stsName, c.Namespace), sts)
		if apierrors.IsNotFound(err) {
			continue
		}
		if err != nil {
			return errors.Wrap(err, "error getting statefulset")
		}

		// Get all member services
		err = cc.List(ctx, svcList, &client.ListOptions{
			Namespace:     c.Namespace,
			LabelSelector: naming.RackSelector(r, c)},
		)
		if err != nil {
			return errors.Wrap(err, "listing member services")
		}
		logger.Debug(ctx, "Cleanup: service list", "len", len(svcList.Items), "items", svcList.Items)

		if err := cc.decommissionCleanup(ctx, c, sts, svcList); err != nil {
			return errors.Wrap(err, "decommission cleanup")
		}

		if c.Spec.AutomaticOrphanedNodeCleanup {
			if err := cc.orphanedCleanup(ctx, c, svcList); err != nil {
				return errors.Wrap(err, "orphaned cleanup")
			}
		}

	}
	return nil
}

// orphanedCleanup verifies if any Pod has PVC with node affinity set to not existing node.
// This may happen when node disappears.
// StatefulSet controller won't be able to schedule new pod on next available node
// due to orphaned affinity.
// Operator checks if any Pod PVC is orphaned, and if so, mark member service as
// a replacement candidate.
func (cc *ClusterReconciler) orphanedCleanup(ctx context.Context, c *scyllav1.ScyllaCluster, svcs *corev1.ServiceList) error {
	nodes := &corev1.NodeList{}
	if err := cc.List(ctx, nodes); err != nil {
		return errors.Wrap(err, "list nodes")
	}
	cc.Logger.Debug(ctx, "Found nodes", "len", len(nodes.Items))

	for _, svc := range svcs.Items {
		pvc := &corev1.PersistentVolumeClaim{}
		if err := cc.Get(ctx, naming.NamespacedName(naming.PVCNameForPod(svc.Name), c.Namespace), pvc); err != nil {
			if apierrors.IsNotFound(err) {
				cc.Logger.Debug(ctx, "Pod PVC not found", "pod", svc.Name, "name", naming.PVCNameForPod(svc.Name))
				continue
			}
			return errors.Wrap(err, "get pvc")
		}

		pv := &corev1.PersistentVolume{}
		if err := cc.Get(ctx, naming.NamespacedName(pvc.Spec.VolumeName, ""), pv); err != nil {
			if apierrors.IsNotFound(err) {
				cc.Logger.Debug(ctx, "PV not found", "pv", pvc.Spec.VolumeName)
				continue
			}
			return errors.Wrap(err, "get pv")
		}

		ns, err := nodeaffinity.NewNodeSelector(pv.Spec.NodeAffinity.Required)
		if err != nil {
			return errors.Wrap(err, "new node selector")
		}

		orphanedVolume := true
		for _, node := range nodes.Items {
			if ns.Match(&node) {
				cc.Logger.Debug(ctx, "PVC attachment found", "pvc", pvc.Name, "node", node.Name)
				orphanedVolume = false
				break
			}
		}

		if orphanedVolume {
			cc.Logger.Info(ctx, "Found orphaned PVC, triggering replace node", "member", svc.Name)
			if err := util.MarkAsReplaceCandidate(ctx, &svc, cc.KubeClient); err != nil {
				return errors.Wrap(err, "mark orphaned service as replace")
			}
		}
	}

	return nil
}

func (cc *ClusterReconciler) decommissionCleanup(ctx context.Context, c *scyllav1.ScyllaCluster, sts *appsv1.StatefulSet, svcs *corev1.ServiceList) error {
	memberCount := *sts.Spec.Replicas
	memberServiceCount := int32(len(svcs.Items))
	// If there are more services than members, some services need to be cleaned up
	if memberServiceCount > memberCount {
		// maxIndex is the maximum index that should be present in a
		// member service of this rack
		maxIndex := memberCount - 1
		for _, svc := range svcs.Items {
			svcIndex, err := naming.IndexFromName(svc.Name)
			if err != nil {
				return errors.WithStack(err)
			}
			if svcIndex > maxIndex && svc.Labels[naming.DecommissionLabel] == naming.LabelValueTrue {
				err := cc.cleanupMemberResources(ctx, &svc, c)
				if err != nil {
					return errors.WithStack(err)
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
func (cc *ClusterReconciler) cleanupMemberResources(ctx context.Context, memberService *corev1.Service, c *scyllav1.ScyllaCluster) error {
	memberName := memberService.Name
	logger := util.LoggerForCluster(c)
	logger.Info(ctx, "Cleaning up resources for member", "name", memberName)
	// Delete PVC if it exists
	pvc := &corev1.PersistentVolumeClaim{}
	err := cc.Get(ctx, naming.NamespacedName(naming.PVCNameForPod(memberName), c.Namespace), pvc)
	// If PVC is found
	if !apierrors.IsNotFound(err) {
		// If there's another error return
		if err != nil {
			return errors.Wrap(err, "failed to get pvc")
		}
		// Else delete the PVC
		if err = cc.Delete(ctx, pvc); err != nil {
			return errors.Wrap(err, "failed to delete pvc")
		}
	}

	// Delete Member Service
	if err = cc.Delete(ctx, memberService); err != nil && !apierrors.IsNotFound(err) {
		return errors.Wrap(err, "failed to delete member service")
	}

	return nil
}
