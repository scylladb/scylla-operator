// Copyright (C) 2021 ScyllaDB

package framework

import (
	g "github.com/onsi/ginkgo/v2"
)

const (
	SerialLabelName  = "Serial"
	UpgradeLabelName = "Upgrade"
)

var (
	Serial = []interface{}{
		g.Serial,
		g.Label(SerialLabelName),
	}

	Upgrade = []interface{}{
		g.Label(UpgradeLabelName),
	}
)
