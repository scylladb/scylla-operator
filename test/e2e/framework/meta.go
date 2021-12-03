// Copyright (C) 2021 ScyllaDB

package framework

import (
	g "github.com/onsi/ginkgo/v2"
)

const (
	SerialLabelName = "Serial"
)

var (
	Serial = []interface{}{
		g.Serial,
		g.Label(SerialLabelName),
	}
)
