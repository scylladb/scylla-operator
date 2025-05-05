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
	DatacenterAvailableConditionFormat   = MakeDatacenterConditionFormat(scyllav1alpha1.AvailableCondition)
	DatacenterProgressingConditionFormat = MakeDatacenterConditionFormat(scyllav1alpha1.ProgressingCondition)
	DatacenterDegradedConditionFormat    = MakeDatacenterConditionFormat(scyllav1alpha1.DegradedCondition)
)

// MakeDatacenterConditionFormat returns a formatted string for a datacenter condition using the provided condition type.
func MakeDatacenterConditionFormat(conditionType string) string {
	return fmt.Sprintf("Datacenter%%s%s", conditionType)
}

func MakeKindControllerCondition(kind, conditionType string) string {
	return fmt.Sprintf("%sController%s", kind, conditionType)
}

func MakeKindFinalizerCondition(kind, conditionType string) string {
	return fmt.Sprintf("%sFinalizer%s", kind, conditionType)
}
