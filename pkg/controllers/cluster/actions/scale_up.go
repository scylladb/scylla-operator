package actions

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/controllers/cluster/util"
	"github.com/scylladb/scylla-operator/pkg/naming"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

const RackScaleUpAction = "rack-scale-up"

// Implements Action interface
var _ Action = &RackScaleUp{}

type RackScaleUp struct {
	Rack    scyllav1.RackSpec
	Cluster *scyllav1.ScyllaCluster
}

func NewRackScaleUpAction(r scyllav1.RackSpec, c *scyllav1.ScyllaCluster) *RackScaleUp {
	return &RackScaleUp{
		Rack:    r,
		Cluster: c,
	}
}

func (a *RackScaleUp) Name() string {
	return RackScaleUpAction
}

func (a *RackScaleUp) Execute(ctx context.Context, s *State) error {
	r, c := a.Rack, a.Cluster
	sts := &appsv1.StatefulSet{}
	err := s.Get(ctx, naming.NamespacedName(naming.StatefulSetNameForRack(r, c), c.Namespace), sts)
	if err != nil {
		return errors.Wrap(err, "failed to get statefulset")
	}

	if err = util.ScaleStatefulSet(ctx, sts, 1, s.kubeclient); err != nil {
		return errors.Wrap(err, "failed to scale statefulset")
	}

	// Record event for successful scale-up
	s.recorder.Event(c, corev1.EventTypeNormal, naming.SuccessSynced, fmt.Sprintf("Rack %s scaled up to %d members", r.Name, *sts.Spec.Replicas+1))
	return nil
}
