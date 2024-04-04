// Copyright (C) 2017 ScyllaDB

package trigger

import (
	"github.com/robfig/cron/v3"
	"github.com/scylladb/scylla-manager/v3/pkg/scheduler"
)

var cronParser = cron.NewParser(cron.SecondOptional | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)

// NewCron returns a cron Trigger for a given spec.
func NewCron(spec string) (scheduler.Trigger, error) {
	return cronParser.Parse(spec)
}
