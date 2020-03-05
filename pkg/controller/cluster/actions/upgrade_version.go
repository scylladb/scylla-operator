package actions

import (
	"context"
	"fmt"

	"github.com/scylladb/scylla-operator/pkg/controller/cluster/util"
	corev1 "k8s.io/api/core/v1"

	appsv1 "k8s.io/api/apps/v1"

	"github.com/pkg/errors"
	"github.com/scylladb/scylla-operator/pkg/naming"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/apis/scylla/v1alpha1"
)

const (
	officialScyllaRepo       = "scylladb/scylla"
	RackVersionUpgradeAction = "rack-version-upgrade"
)

type RackVersionUpgrade struct {
	Rack    scyllav1alpha1.RackSpec
	Cluster *scyllav1alpha1.Cluster
}

func NewRackVersionUpgradeAction(r scyllav1alpha1.RackSpec, c *scyllav1alpha1.Cluster) *RackVersionUpgrade {
	return &RackVersionUpgrade{
		Rack:    r,
		Cluster: c,
	}
}

func (a *RackVersionUpgrade) Name() string {
	return RackVersionUpgradeAction
}

func (a *RackVersionUpgrade) Execute(ctx context.Context, s *State) error {
	r, c := a.Rack, a.Cluster
	sts := &appsv1.StatefulSet{}
	err := s.Get(ctx, naming.NamespacedName(naming.StatefulSetNameForRack(r, c), c.Namespace), sts)
	if err != nil {
		return errors.Wrap(err, "failed to get statefulset")
	}
	image := imageForCluster(c)
	if err = util.UpgradeStatefulSetVersion(sts, image, s.kubeclient); err != nil {
		return errors.Wrap(err, "failed to upgrade statefulset")
	}

	// Record event for successful version upgrade
	s.recorder.Event(c, corev1.EventTypeNormal, naming.SuccessSynced, fmt.Sprintf("Rack %s upgraded up to version %s", r.Name, image))
	return nil
}

func imageForCluster(c *scyllav1alpha1.Cluster) string {
	repo := officialScyllaRepo
	if c.Spec.Repository != nil {
		repo = *c.Spec.Repository
	}
	return fmt.Sprintf("%s:%s", repo, c.Spec.Version)
}
