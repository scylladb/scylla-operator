// Copyright (C) 2017 ScyllaDB

package schedules

import (
	"github.com/robfig/cron/v3"
)

var cronParser = cron.NewParser(cron.SecondOptional | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)

// NewCronTrigger returns a cron Trigger for a given spec.
func NewCronTrigger(spec string) (Trigger, error) {
	return cronParser.Parse(spec)
}
