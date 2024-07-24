// Copyright (C) 2021 ScyllaDB

package framework

import (
	g "github.com/onsi/ginkgo/v2"
)

const (
	SerialLabelName          = "Serial"
	MultiDatacenterLabelName = "MultiDatacenter"

	RequiresClusterIPLabelName     = "RequiresClusterIP"
	RequiresObjectStorageLabelName = "RequiresObjectStorage"
)

var (
	Serial = []interface{}{
		g.Serial,
		g.Label(SerialLabelName),
	}
	MultiDatacenter = g.Label(MultiDatacenterLabelName)

	RequiresClusterIP     = g.Label(RequiresClusterIPLabelName)
	RequiresObjectStorage = g.Label(RequiresObjectStorageLabelName)
)
