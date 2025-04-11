package internalapi

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

	DatacenterAvailableConditionFormat   = "Datacenter%sAvailable"
	DatacenterProgressingConditionFormat = "Datacenter%sProgressing"
	DatacenterDegradedConditionFormat    = "Datacenter%sDegraded"

	AsExpectedReason        = "AsExpected"
	ErrorReason             = "Error"
	ProgressingReason       = "Progressing"
	AwaitingConditionReason = "AwaitingCondition"
)
