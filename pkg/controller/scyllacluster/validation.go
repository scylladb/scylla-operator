// Copyright (c) 2023 ScyllaDB.

package scyllacluster

import (
	"fmt"

	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/scyllafeatures"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
)

func runPreRolloutChecks(sc *scyllav1.ScyllaCluster, eventRecorder record.EventRecorder) error {
	supportsExposing, err := scyllafeatures.Supports(sc, scyllafeatures.ExposingScyllaClusterViaServiceOtherThanClusterIP)
	if err != nil {
		return fmt.Errorf("can't determine if ScyllaDB version %q supports exposing via Service other than ClusterIP: %w", sc.Spec.Version, err)
	}

	exposingSupportRequired := false

	if sc.Spec.ExposeOptions != nil && sc.Spec.ExposeOptions.NodeService != nil && sc.Spec.ExposeOptions.NodeService.Type != scyllav1.NodeServiceTypeClusterIP {
		exposingSupportRequired = true
	}

	if sc.Spec.ExposeOptions != nil && sc.Spec.ExposeOptions.BroadcastOptions != nil {
		bo := sc.Spec.ExposeOptions.BroadcastOptions
		exposingSupportRequired = bo.Clients.Type != scyllav1.BroadcastAddressTypeServiceClusterIP || bo.Nodes.Type != scyllav1.BroadcastAddressTypeServiceClusterIP
	}

	if exposingSupportRequired && !supportsExposing {
		eventRecorder.Eventf(
			sc,
			corev1.EventTypeWarning,
			"InvalidScyllaDBVersion",
			fmt.Sprintf("Can't proceed with the rollout. Specified ScyllaDB version %q does not support exposing via Service other than ClusterIP. Please use different ScyllaDB version.", sc.Spec.Version),
		)
		return fmt.Errorf("can't proceed with the rollout. Specified ScyllaDB version %q does not support exposing via Service other than ClusterIP. Please use different ScyllaDB version", sc.Spec.Version)
	}

	return nil
}
