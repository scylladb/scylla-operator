package actions

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/controllers/cluster/resource"
	"github.com/scylladb/scylla-operator/pkg/controllers/cluster/util"
	"github.com/scylladb/scylla-operator/pkg/controllers/helpers"
	"github.com/scylladb/scylla-operator/pkg/naming"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

const RackCreateAction = "rack-create"

// Implements Action interface
var _ Action = &RackCreate{}

type RackCreate struct {
	Rack          scyllav1.RackSpec
	Cluster       *scyllav1.ScyllaCluster
	OperatorImage string
	CloudPlatform helpers.CloudPlatform
}

func NewRackCreateAction(r scyllav1.RackSpec, c *scyllav1.ScyllaCluster, image string, platform helpers.CloudPlatform) *RackCreate {
	return &RackCreate{
		Rack:          r,
		Cluster:       c,
		OperatorImage: image,
		CloudPlatform: platform,
	}
}

func (a *RackCreate) Name() string {
	return RackCreateAction
}

func (a *RackCreate) Execute(ctx context.Context, s *State) error {
	r, c := a.Rack, a.Cluster
	newSts, err := resource.StatefulSetForRack(r, c, a.OperatorImage, a.CloudPlatform)
	if err != nil {
		return err
	}

	existingSts := &appsv1.StatefulSet{}
	err = s.Get(ctx, naming.NamespacedNameForObject(newSts), existingSts)
	// Check if StatefulSet already exists
	// TODO: Check this logic
	if err == nil {
		err = util.VerifyOwner(existingSts, c)
		return errors.Wrap(err, "failed to verify owner")
	}
	// Check if an error occured
	if !apierrors.IsNotFound(err) {
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
