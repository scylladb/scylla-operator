// Copyright (C) 2021 ScyllaDB

package framework

import (
	g "github.com/onsi/ginkgo/v2"
)

const (
	SerialLabelName          = "Serial"
	MultiDatacenterLabelName = "MultiDatacenter"
	IPv6LabelName            = "IPv6"

	RequiresObjectStorageLabelName    = "RequiresObjectStorage"
	NotSupportedOnOpenShiftLabelName  = "NotSupportedOnOpenShift"
	SupportedOnlyOnOpenShiftLabelName = "SupportedOnlyOnOpenShift"
	NotSupportedOnKindLabelName       = "NotSupportedOnKind"
	LongRunningLabelName              = "LongRunning"
	SuiteOperatorUpgradeLabelName     = "SuiteOperatorUpgrade"
)

var (
	Serial = []interface{}{
		g.Serial,
		g.Label(SerialLabelName),
	}
	MultiDatacenter = g.Label(MultiDatacenterLabelName)
	IPv6            = g.Label(IPv6LabelName)

	RequiresObjectStorage    = g.Label(RequiresObjectStorageLabelName)
	NotSupportedOnOpenShift  = g.Label(NotSupportedOnOpenShiftLabelName)
	SupportedOnlyOnOpenShift = g.Label(SupportedOnlyOnOpenShiftLabelName)

	// NotSupportedOnKind is a label is for tests not supported on kind clusters (e.g., due to lack of access to host filesystem).
	NotSupportedOnKind = g.Label(NotSupportedOnKindLabelName)

	// LongRunning is a label for tests that are long-running (over ~20 minutes).
	LongRunning = g.Label(LongRunningLabelName)

	// SuiteOperatorUpgrade is a label for the operator upgrade suite, which requires a dedicated cluster with no
	// operator deployed and must not be picked up by any other suite.
	SuiteOperatorUpgrade = g.Label(SuiteOperatorUpgradeLabelName)
)
