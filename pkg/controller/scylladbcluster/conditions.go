package scylladbcluster

import (
	"fmt"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/internalapi"
)

var (
	makeRemoteRemoteOwnerControllerDatacenterProgressingCondition        = MakeRemoteKindControllerDatacenterConditionFunc("RemoteOwner", scyllav1alpha1.ProgressingCondition)
	makeRemoteRemoteOwnerControllerDatacenterDegradedCondition           = MakeRemoteKindControllerDatacenterConditionFunc("RemoteOwner", scyllav1alpha1.DegradedCondition)
	makeRemoteScyllaDBDatacenterControllerDatacenterProgressingCondition = MakeRemoteKindControllerDatacenterConditionFunc("ScyllaDBDatacenter", scyllav1alpha1.ProgressingCondition)
	makeRemoteScyllaDBDatacenterControllerDatacenterDegradedCondition    = MakeRemoteKindControllerDatacenterConditionFunc("ScyllaDBDatacenter", scyllav1alpha1.DegradedCondition)
	makeRemoteNamespaceControllerDatacenterProgressingCondition          = MakeRemoteKindControllerDatacenterConditionFunc("Namespace", scyllav1alpha1.ProgressingCondition)
	makeRemoteNamespaceControllerDatacenterDegradedCondition             = MakeRemoteKindControllerDatacenterConditionFunc("Namespace", scyllav1alpha1.DegradedCondition)
	makeRemoteServiceControllerDatacenterProgressingCondition            = MakeRemoteKindControllerDatacenterConditionFunc("Service", scyllav1alpha1.ProgressingCondition)
	makeRemoteServiceControllerDatacenterDegradedCondition               = MakeRemoteKindControllerDatacenterConditionFunc("Service", scyllav1alpha1.DegradedCondition)
	makeRemoteEndpointSliceControllerDatacenterProgressingCondition      = MakeRemoteKindControllerDatacenterConditionFunc("EndpointSlice", scyllav1alpha1.ProgressingCondition)
	makeRemoteEndpointSliceControllerDatacenterDegradedCondition         = MakeRemoteKindControllerDatacenterConditionFunc("EndpointSlice", scyllav1alpha1.DegradedCondition)
	makeRemoteEndpointsControllerDatacenterProgressingCondition          = MakeRemoteKindControllerDatacenterConditionFunc("Endpoints", scyllav1alpha1.ProgressingCondition)
	makeRemoteEndpointsControllerDatacenterDegradedCondition             = MakeRemoteKindControllerDatacenterConditionFunc("Endpoints", scyllav1alpha1.DegradedCondition)
	makeRemoteConfigMapControllerDatacenterProgressingCondition          = MakeRemoteKindControllerDatacenterConditionFunc("ConfigMap", scyllav1alpha1.ProgressingCondition)
	makeRemoteConfigMapControllerDatacenterDegradedCondition             = MakeRemoteKindControllerDatacenterConditionFunc("ConfigMap", scyllav1alpha1.DegradedCondition)
	makeRemoteSecretControllerDatacenterProgressingCondition             = MakeRemoteKindControllerDatacenterConditionFunc("Secret", scyllav1alpha1.ProgressingCondition)
	makeRemoteSecretControllerDatacenterDegradedCondition                = MakeRemoteKindControllerDatacenterConditionFunc("Secret", scyllav1alpha1.DegradedCondition)

	scyllaDBClusterFinalizerProgressingCondition = internalapi.MakeKindFinalizerCondition("ScyllaDBCluster", scyllav1alpha1.ProgressingCondition)
	scyllaDBClusterFinalizerDegradedCondition    = internalapi.MakeKindFinalizerCondition("ScyllaDBCluster", scyllav1alpha1.DegradedCondition)

	serviceControllerProgressingCondition       = internalapi.MakeKindControllerCondition("Service", scyllav1alpha1.ProgressingCondition)
	serviceControllerDegradedCondition          = internalapi.MakeKindControllerCondition("Service", scyllav1alpha1.DegradedCondition)
	endpointSliceControllerProgressingCondition = internalapi.MakeKindControllerCondition("EndpointSlice", scyllav1alpha1.ProgressingCondition)
	endpointSliceControllerDegradedCondition    = internalapi.MakeKindControllerCondition("EndpointSlice", scyllav1alpha1.DegradedCondition)
	endpointsControllerProgressingCondition     = internalapi.MakeKindControllerCondition("Endpoints", scyllav1alpha1.ProgressingCondition)
	endpointsControllerDegradedCondition        = internalapi.MakeKindControllerCondition("Endpoints", scyllav1alpha1.DegradedCondition)
	secretControllerProgressingCondition        = internalapi.MakeKindControllerCondition("Secret", scyllav1alpha1.ProgressingCondition)
	secretControllerDegradedCondition           = internalapi.MakeKindControllerCondition("Secret", scyllav1alpha1.DegradedCondition)
)

// MakeRemoteKindControllerDatacenterConditionFunc returns a format string for a remote kind controller datacenter condition.
func MakeRemoteKindControllerDatacenterConditionFunc(kind, conditionType string) func(dcName string) string {
	return func(dcName string) string {
		return fmt.Sprintf("Remote%s", internalapi.MakeKindControllerCondition(kind, internalapi.MakeDatacenterConditionFunc(conditionType)(dcName)))
	}
}
