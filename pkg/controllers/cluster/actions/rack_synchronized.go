// Copyright (C) 2021 ScyllaDB

package actions

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"github.com/scylladb/go-log"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/controllers/cluster/util"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type rackSynchronizedSubAction interface {
	// RackUpdated should return whether requested update is already applied to a rack sts.
	RackUpdated(sts *appsv1.StatefulSet) (bool, error)
	// Update performs desired update.
	Update(sts *appsv1.StatefulSet) error
	// Name of action.
	Name() string
}

// rackSynchronizedAction can be used to perform an multi rack update in synchronous manner.
// Racks are upgraded in the same order as in ScyllaCluster Spec.
// Single Execute call performs at most a single update.
type rackSynchronizedAction struct {
	subAction rackSynchronizedSubAction
	cluster   *scyllav1.ScyllaCluster
	logger    log.Logger
}

const (
	RackSynchronizedActionPrefix = "rack-synchronized-"
)

func (a rackSynchronizedAction) Name() string {
	return fmt.Sprintf("%s%s", RackSynchronizedActionPrefix, a.subAction.Name())
}

func (a rackSynchronizedAction) Execute(ctx context.Context, s *State) error {
	for _, rack := range a.cluster.Spec.Datacenter.Racks {
		sts := &appsv1.StatefulSet{}
		sts, err := util.GetStatefulSetForRack(ctx, rack, a.cluster, s.kubeclient)
		if err != nil {
			return errors.Wrap(err, "get rack statefulset")
		}

		rackUpdated, err := a.subAction.RackUpdated(sts)
		if err != nil {
			return errors.Wrap(err, "determine if rack needs update")
		}
		if !rackUpdated {
			if err := a.subAction.Update(sts); err != nil {
				return errors.Wrap(err, "update rack")
			}

			a.logger.Info(ctx, "Updating rack definition", "rack", rack.Name)
			sts, err = s.kubeclient.AppsV1().StatefulSets(sts.Namespace).Update(ctx, sts, metav1.UpdateOptions{})
			if err != nil {
				// Do not raise error in case of a conflict, reconcilation loop will be triggered again
				// because new version of STS we are watching is available.
				if apierrors.IsConflict(err) {
					return nil
				}
				return err
			}

			// Early exit, next rack will be updated once current one becomes ready.
			return nil
		}

		if !a.stsReady(rack, sts) {
			a.logger.Info(ctx, "Rack still upgrading, awaiting readiness", "rack", rack.Name)
			return nil
		}

		a.logger.Info(ctx, "Rack updated", "rack", rack.Name)
	}

	return nil
}

func (a rackSynchronizedAction) stsReady(rack scyllav1.RackSpec, sts *appsv1.StatefulSet) bool {
	return sts.Generation == sts.Status.ObservedGeneration &&
		sts.Status.ReadyReplicas == rack.Members &&
		sts.Status.UpdateRevision == sts.Status.CurrentRevision
}
