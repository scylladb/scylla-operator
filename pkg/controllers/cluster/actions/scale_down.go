package actions

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllers/cluster/util"
	"github.com/scylladb/scylla-operator/pkg/naming"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

const RackScaleDownAction = "rack-scale-down"

// Implements Action interface
var _ Action = &RackScaleDown{}

type RackScaleDown struct {
	Rack    scyllav1alpha1.RackSpec
	Cluster *scyllav1alpha1.ScyllaCluster
}

func NewRackScaleDownAction(r scyllav1alpha1.RackSpec, c *scyllav1alpha1.ScyllaCluster) *RackScaleDown {
	return &RackScaleDown{
		Rack:    r,
		Cluster: c,
	}
}

func (a *RackScaleDown) Name() string {
	return RackScaleDownAction
}

func (a *RackScaleDown) Execute(ctx context.Context, s *State) error {

	r, c := a.Rack, a.Cluster
	// Get the current actual number of Members
	members := c.Status.Racks[r.Name].Members

	// Find the member to decommission
	memberName := fmt.Sprintf("%s-%d", naming.StatefulSetNameForRack(r, c), members-1)
	memberService := &corev1.Service{}
	err := s.Get(ctx, naming.NamespacedName(memberName, c.Namespace), memberService)
	if err != nil {
		return errors.Wrap(err, "failed to get Member Service")
	}

	// Check if there is a scale down in progress that has completed
	if memberService.Labels[naming.DecommissionLabel] == naming.LabelValueTrue {
		return errors.WithStack(a.completeScaleDown(ctx, s))
	}

	if _, ok := memberService.Labels[naming.DecommissionLabel]; r.Members < c.Status.Racks[r.Name].Members && !ok {
		return errors.WithStack(a.beginScaleDown(ctx, s, memberService))
	}

	return nil
}

func (a *RackScaleDown) beginScaleDown(ctx context.Context, s *State, memberService *corev1.Service) error {
	r, c := a.Rack, a.Cluster

	// Record the intent to decommission the member
	old := memberService.DeepCopy()
	memberService.Labels[naming.DecommissionLabel] = naming.LabelValueFalse
	if err := util.PatchService(ctx, old, memberService, s.kubeclient); err != nil {
		return errors.Wrap(err, "error patching member service")
	}

	s.recorder.Event(c, corev1.EventTypeNormal, naming.SuccessSynced,
		fmt.Sprintf("Rack %s scaling down to %d members", r.Name, c.Status.Racks[r.Name].Members-1),
	)

	return nil
}

func (a *RackScaleDown) completeScaleDown(ctx context.Context, s *State) error {
	r, c := a.Rack, a.Cluster

	// Get rack's statefulset
	stsName := naming.StatefulSetNameForRack(r, c)
	sts := &appsv1.StatefulSet{}
	err := s.Get(ctx, naming.NamespacedName(stsName, c.Namespace), sts)
	if err != nil {
		return errors.Wrap(err, "error trying to get statefulset")
	}

	// Scale the statefulset
	err = util.ScaleStatefulSet(ctx, sts, -1, s.kubeclient)
	if err != nil {
		return errors.Wrap(err, "error trying to scale down statefulset")
	}

	// Cleanup is done on each sync loop, no need to do anything else here
	s.recorder.Event(c, corev1.EventTypeNormal, naming.SuccessSynced, fmt.Sprintf("Rack %s scaled down to %d members", r.Name, c.Status.Racks[r.Name].Members-1))
	return nil
}
