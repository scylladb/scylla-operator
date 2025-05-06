package internalapi

import (
	"fmt"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
)

const (
	NodeAvailableConditionFormat   = "Node%sAvailable"
	NodeProgressingConditionFormat = "Node%sProgressing"
	NodeDegradedConditionFormat    = "Node%sDegraded"

	NodeSetupAvailableConditionFormat   = "NodeSetup%sAvailable"
	NodeSetupProgressingConditionFormat = "NodeSetup%sProgressing"
	NodeSetupDegradedConditionFormat    = "NodeSetup%sDegraded"

	NodeTuneAvailableConditionFormat   = "NodeTune%sAvailable"
	NodeTuneProgressingConditionFormat = "NodeTune%sProgressing"
	NodeTuneDegradedConditionFormat    = "NodeTune%sDegraded"

	AsExpectedReason        = "AsExpected"
	ErrorReason             = "Error"
	ProgressingReason       = "Progressing"
	AwaitingConditionReason = "AwaitingCondition"
)

var (
	MakeDatacenterAvailableCondition   = MakeDatacenterConditionFunc(scyllav1alpha1.AvailableCondition)
	MakeDatacenterProgressingCondition = MakeDatacenterConditionFunc(scyllav1alpha1.ProgressingCondition)
	MakeDatacenterDegradedCondition    = MakeDatacenterConditionFunc(scyllav1alpha1.DegradedCondition)
)

// MakeDatacenterConditionFunc returns a function that creates a datacenter condition using the provided condition type.
func MakeDatacenterConditionFunc(conditionType string) func(dcName string) string {
	return func(dcName string) string {
		return fmt.Sprintf("Datacenter%s%s", dcName, conditionType)
	}
}

func MakeKindControllerCondition(kind, conditionType string) string {
	return fmt.Sprintf("%sController%s", kind, conditionType)
}

func MakeKindFinalizerCondition(kind, conditionType string) string {
	return fmt.Sprintf("%sFinalizer%s", kind, conditionType)
}
