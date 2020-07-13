package actions

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"github.com/scylladb/go-log"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllers/cluster/resource"
	"github.com/scylladb/scylla-operator/pkg/controllers/cluster/util"
	"github.com/scylladb/scylla-operator/pkg/naming"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

const (
	ClusterVersionUpgradeAction = "rack-version-upgrade"
)

type ClusterVersionUpgrade struct {
	Cluster *scyllav1alpha1.Cluster
	logger  log.Logger
}

func NewClusterVersionUpgradeAction(c *scyllav1alpha1.Cluster, l log.Logger) *ClusterVersionUpgrade {
	return &ClusterVersionUpgrade{
		Cluster: c,
		logger:  l,
	}
}

func (a *ClusterVersionUpgrade) Name() string {
	return ClusterVersionUpgradeAction
}

func (a *ClusterVersionUpgrade) Execute(ctx context.Context, s *State) error {
	c := a.Cluster
	for _, r := range c.Spec.Datacenter.Racks {
		a.logger.Debug(ctx, fmt.Sprintf("Rack: %s, Rack Members: %d, Spec members: %d\n", r.Name, r.Members, c.Status.Racks[r.Name].Members))
		if c.Status.Racks[r.Name].Version != c.Spec.Version {
			sts := &appsv1.StatefulSet{}
			if err := s.Get(ctx, naming.NamespacedName(naming.StatefulSetNameForRack(r, c), c.Namespace), sts); err != nil {
				return errors.Wrap(err, "failed to get statefulset")
			}
			image := resource.ImageForCluster(c)
			if err := util.UpgradeStatefulSetScyllaImage(ctx, sts, image, s.kubeclient); err != nil {
				return errors.Wrap(err, "failed to upgrade statefulset")
			}
			// Record event for successful version upgrade
			s.recorder.Event(c, corev1.EventTypeNormal, naming.SuccessSynced, fmt.Sprintf("Rack %s upgraded up to version %s", r.Name, c.Spec.Version))
			return nil
		}
	}
	return nil
}
