// Copyright (C) 2021 ScyllaDB

package framework

import (
	g "github.com/onsi/ginkgo/v2"
)

const (
	SerialLabelName          = "Serial"
	MultiDatacenterLabelName = "MultiDatacenter"

	RequiresObjectStorageLabelName    = "RequiresObjectStorage"
	NotSupportedOnOpenShiftLabelName  = "NotSupportedOnOpenShift"
	SupportedOnlyOnOpenShiftLabelName = "SupportedOnlyOnOpenShift"
	NotSupportedOnKindLabelName       = "NotSupportedOnKind"
)

var (
	Serial = []interface{}{
		g.Serial,
		g.Label(SerialLabelName),
	}
	MultiDatacenter = g.Label(MultiDatacenterLabelName)

	RequiresObjectStorage    = g.Label(RequiresObjectStorageLabelName)
	NotSupportedOnOpenShift  = g.Label(NotSupportedOnOpenShiftLabelName)
	SupportedOnlyOnOpenShift = g.Label(SupportedOnlyOnOpenShiftLabelName)
	NotSupportedOnKind       = g.Label(NotSupportedOnKindLabelName)
)
