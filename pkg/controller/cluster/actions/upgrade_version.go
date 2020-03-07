package actions

import (
	"context"
	"fmt"
	"github.com/scylladb/scylla-operator/pkg/controller/cluster/resource"

	"github.com/scylladb/scylla-operator/pkg/controller/cluster/util"
	corev1 "k8s.io/api/core/v1"

	appsv1 "k8s.io/api/apps/v1"

	"github.com/pkg/errors"
	"github.com/scylladb/scylla-operator/pkg/naming"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/apis/scylla/v1alpha1"
)

const (
	ClusterVersionUpgradeAction = "rack-version-upgrade"
)

type ClusterVersionUpgrade struct {
	Cluster *scyllav1alpha1.Cluster
}

func NewClusterVersionUpgradeAction(c *scyllav1alpha1.Cluster) *ClusterVersionUpgrade {
	return &ClusterVersionUpgrade{
		Cluster: c,
	}
}

func (a *ClusterVersionUpgrade) Name() string {
	return ClusterVersionUpgradeAction
}

func (a *ClusterVersionUpgrade) Execute(ctx context.Context, s *State) error {
	c := a.Cluster
	for _, r := range c.Spec.Datacenter.Racks {
		if c.Status.Racks[r.Name].Version != c.Spec.Version {
			sts := &appsv1.StatefulSet{}
			err := s.Get(ctx, naming.NamespacedName(naming.StatefulSetNameForRack(r, c), c.Namespace), sts)
			if err != nil {
				return errors.Wrap(err, "failed to get statefulset")
			}
			image := resource.ImageForCluster(c)
			if err = util.UpgradeStatefulSetImage(sts, image, s.kubeclient); err != nil {
				return errors.Wrap(err, "failed to upgrade statefulset")
			}
			// Record event for successful version upgrade
			s.recorder.Event(
				c,
				corev1.EventTypeNormal,
				naming.SuccessSynced,
				fmt.Sprintf("Rack %s upgraded up to version %s", r.Name, c.Spec.Version),
			)
		}
	}
	return nil
}
