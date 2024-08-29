package internalapi

const (
	NodeAvailableConditionFormat   = "Node%sAvailable"
	NodeProgressingConditionFormat = "Node%sProgressing"
	NodeDegradedConditionFormat    = "Node%sDegraded"

	AsExpectedReason        = "AsExpected"
	ErrorReason             = "Error"
	ProgressingReason       = "Progressing"
	AwaitingConditionReason = "AwaitingCondition"
)
