// Copyright (C) 2022 ScyllaDB

package managerclient

import (
	"github.com/lnquy/cron"
)

var cronDesc *cron.ExpressionDescriptor

func init() {
	cronDesc, _ = cron.NewDescriptor() // nolint: errcheck
}

// DescribeCron returns description of cron expression in plain English.
func DescribeCron(s string) string {
	if cronDesc == nil {
		return ""
	}
	d, _ := cronDesc.ToDescription(s, cron.Locale_en) // nolint: errcheck
	return d
}
