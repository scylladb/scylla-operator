package scylladbcluster

import (
	"fmt"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/internalapi"
)

var (
	remoteRemoteOwnerControllerDatacenterProgressingConditionFormat        = MakeRemoteKindControllerDatacenterConditionFormat("RemoteOwner", scyllav1alpha1.ProgressingCondition)
	remoteRemoteOwnerControllerDatacenterDegradedConditionFormat           = MakeRemoteKindControllerDatacenterConditionFormat("RemoteOwner", scyllav1alpha1.DegradedCondition)
	remoteScyllaDBDatacenterControllerDatacenterProgressingConditionFormat = MakeRemoteKindControllerDatacenterConditionFormat("ScyllaDBDatacenter", scyllav1alpha1.ProgressingCondition)
	remoteScyllaDBDatacenterControllerDatacenterDegradedConditionFormat    = MakeRemoteKindControllerDatacenterConditionFormat("ScyllaDBDatacenter", scyllav1alpha1.DegradedCondition)
	remoteNamespaceControllerDatacenterProgressingConditionFormat          = MakeRemoteKindControllerDatacenterConditionFormat("Namespace", scyllav1alpha1.ProgressingCondition)
	remoteNamespaceControllerDatacenterDegradedConditionFormat             = MakeRemoteKindControllerDatacenterConditionFormat("Namespace", scyllav1alpha1.DegradedCondition)
	remoteServiceControllerDatacenterProgressingConditionFormat            = MakeRemoteKindControllerDatacenterConditionFormat("Service", scyllav1alpha1.ProgressingCondition)
	remoteServiceControllerDatacenterDegradedConditionFormat               = MakeRemoteKindControllerDatacenterConditionFormat("Service", scyllav1alpha1.DegradedCondition)
	remoteEndpointSliceControllerDatacenterProgressingConditionFormat      = MakeRemoteKindControllerDatacenterConditionFormat("EndpointSlice", scyllav1alpha1.ProgressingCondition)
	remoteEndpointSliceControllerDatacenterDegradedConditionFormat         = MakeRemoteKindControllerDatacenterConditionFormat("EndpointSlice", scyllav1alpha1.DegradedCondition)
	remoteEndpointsControllerDatacenterProgressingConditionFormat          = MakeRemoteKindControllerDatacenterConditionFormat("Endpoints", scyllav1alpha1.ProgressingCondition)
	remoteEndpointsControllerDatacenterDegradedConditionFormat             = MakeRemoteKindControllerDatacenterConditionFormat("Endpoints", scyllav1alpha1.DegradedCondition)
	remoteConfigMapControllerDatacenterProgressingConditionFormat          = MakeRemoteKindControllerDatacenterConditionFormat("ConfigMap", scyllav1alpha1.ProgressingCondition)
	remoteConfigMapControllerDatacenterDegradedConditionFormat             = MakeRemoteKindControllerDatacenterConditionFormat("ConfigMap", scyllav1alpha1.DegradedCondition)
	remoteSecretControllerDatacenterProgressingConditionFormat             = MakeRemoteKindControllerDatacenterConditionFormat("Secret", scyllav1alpha1.ProgressingCondition)
	remoteSecretControllerDatacenterDegradedConditionFormat                = MakeRemoteKindControllerDatacenterConditionFormat("Secret", scyllav1alpha1.DegradedCondition)

	scyllaDBClusterFinalizerProgressingCondition = internalapi.MakeKindFinalizerCondition("ScyllaDBCluster", scyllav1alpha1.ProgressingCondition)
	scyllaDBClusterFinalizerDegradedCondition    = internalapi.MakeKindFinalizerCondition("ScyllaDBCluster", scyllav1alpha1.DegradedCondition)
)

// MakeRemoteKindControllerDatacenterConditionFormat returns a format string for a remote kind controller datacenter condition.
func MakeRemoteKindControllerDatacenterConditionFormat(kind, conditionType string) string {
	return fmt.Sprintf("Remote%s", internalapi.MakeKindControllerCondition(kind, internalapi.MakeDatacenterConditionFormat(conditionType)))
}
