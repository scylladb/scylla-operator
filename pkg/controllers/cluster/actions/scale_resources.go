package actions

import (
	"context"
	"fmt"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/v1"
	"reflect"
	"github.com/pkg/errors"
	"github.com/scylladb/scylla-operator/pkg/controllers/cluster/util"
	"github.com/scylladb/scylla-operator/pkg/naming"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

const RackScaleResourcesAction = "rack-scale-resources"

// Implements Action interface
var _ Action = &RackScaleResources{}

type RackScaleResources struct {
	Rack    scyllav1.RackSpec
	Cluster *scyllav1.ScyllaCluster
}

func NewRackScaleResourcesAction(r scyllav1.RackSpec, c *scyllav1.ScyllaCluster) *RackScaleResources {
	return &RackScaleResources{
		Rack:    r,
		Cluster: c,
	}
}

func (a *RackScaleResources) Name() string {
	return RackScaleResourcesAction
}

func (a *RackScaleResources) Execute(ctx context.Context, s *State) error {
	r, c := a.Rack, a.Cluster

	sts := &appsv1.StatefulSet{}
	err := s.Get(ctx, naming.NamespacedName(naming.StatefulSetNameForRack(r, c), c.Namespace), sts)
	if err != nil {
		return errors.Wrap(err, "failed to get statefulset")
	}

	idx, err := naming.FindScyllaContainer(sts.Spec.Template.Spec.Containers)
	if err != nil {
		s.recorder.Event(c, corev1.EventTypeWarning, naming.ErrSyncFailed, fmt.Sprintf("Error trying to find container named scylla to scale it's resources"))
		return errors.WithStack(err)
	}
	// Scale if resources have been changed
	// else do nothing and return nil.
	if !reflect.DeepEqual(sts.Spec.Template.Spec.Containers[idx].Resources, r.Resources) {
		err = util.ScaleScyllaContainerResources(ctx, sts, r.Resources, s.kubeclient, c, s.recorder)
		if err != nil {
			return errors.Wrap(err, "error trying to scale sts resources")
		}
		// Record event for successful scaling
		s.recorder.Event(c, corev1.EventTypeNormal, naming.SuccessScaled, fmt.Sprintf("Resources of rack %s have been scaled", r.Name))
		return nil
	} else {
		return nil
	}

}
