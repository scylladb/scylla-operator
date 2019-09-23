package actions

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/apis/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controller/cluster/resource"
	"github.com/scylladb/scylla-operator/pkg/controller/cluster/util"
	"github.com/scylladb/scylla-operator/pkg/naming"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

const RackCreateAction = "rack-create"

// Implements Action interface
var _ Action = &RackCreate{}

type RackCreate struct {
	Rack          scyllav1alpha1.RackSpec
	Cluster       *scyllav1alpha1.Cluster
	OperatorImage string
}

func NewRackCreateAction(r scyllav1alpha1.RackSpec, c *scyllav1alpha1.Cluster, image string) *RackCreate {
	return &RackCreate{
		Rack:          r,
		Cluster:       c,
		OperatorImage: image,
	}
}

func (a *RackCreate) Name() string {
	return RackCreateAction
}

func (a *RackCreate) Execute(ctx context.Context, s *State) error {
	r, c := a.Rack, a.Cluster
	newSts := resource.StatefulSetForRack(r, c, a.OperatorImage)
	existingSts := &appsv1.StatefulSet{}
	err := s.Get(ctx, naming.NamespacedNameForObject(newSts), existingSts)
	// Check if StatefulSet already exists
	// TODO: Check this logic
	if err == nil {
		err = util.VerifyOwner(existingSts, c)
		return errors.Wrap(err, "failed to verify owner")
	}
	// Check if an error occured
	if err != nil && !apierrors.IsNotFound(err) {
		return errors.Wrapf(err, "error trying to get statefulset %s", existingSts.Name)
	}

	// StatefulSet doesn't exist, so we create it
	err = s.Create(ctx, newSts)
	if err != nil {
		return errors.Wrapf(err, "error creating statefulset '%s' for rack '%s'", newSts.Name, r.Name)
	}

	s.recorder.Event(c, corev1.EventTypeNormal, naming.SuccessSynced, fmt.Sprintf("Rack %s created", r.Name))
	return nil
}
