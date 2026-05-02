// Copyright (C) 2021 ScyllaDB

package framework

import (
	g "github.com/onsi/ginkgo/v2"
)

// Suite labels declare which test suite a spec belongs to. Every spec MUST be
// labeled with at least one of these. A spec may belong to multiple suites.
const (
	SuiteParallelLabelName                = "SuiteParallel"
	SuiteParallelOpenShiftLabelName       = "SuiteParallelOpenShift"
	SuiteSerialLabelName                  = "SuiteSerial"
	SuiteMultiDatacenterParallelLabelName = "SuiteMultiDatacenterParallel"
	SuiteParallelIPv6LabelName            = "SuiteParallelIPv6"
	SuiteKindFastLabelName                = "SuiteKindFast"
)

var (
	SuiteParallel                = g.Label(SuiteParallelLabelName)
	SuiteParallelOpenShift       = g.Label(SuiteParallelOpenShiftLabelName)
	SuiteMultiDatacenterParallel = g.Label(SuiteMultiDatacenterParallelLabelName)
	SuiteParallelIPv6            = g.Label(SuiteParallelIPv6LabelName)
	SuiteKindFast                = g.Label(SuiteKindFastLabelName)

	// SuiteSerial bundles the Ginkgo serial-execution decorator with the
	// suite label, so a single value is enough to mark a spec as part of the
	// serial suite.
	SuiteSerial = []interface{}{
		g.Serial,
		g.Label(SuiteSerialLabelName),
	}
)
