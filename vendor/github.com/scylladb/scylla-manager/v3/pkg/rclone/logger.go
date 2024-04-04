// Copyright (C) 2017 ScyllaDB

package rclone

import (
	"context"

	"github.com/rclone/rclone/fs"
	"github.com/scylladb/go-log"
)

// RedirectLogPrint redirects fs.LogPrint to the logger.
func RedirectLogPrint(logger log.Logger) {
	fs.LogPrint = func(level fs.LogLevel, text string) {
		switch level {
		case fs.LogLevelEmergency, fs.LogLevelAlert, fs.LogLevelCritical:
			logger.Fatal(context.TODO(), text)
		case fs.LogLevelError, fs.LogLevelWarning:
			logger.Error(context.TODO(), text)
		case fs.LogLevelNotice, fs.LogLevelInfo:
			logger.Info(context.TODO(), text)
		case fs.LogLevelDebug:
			logger.Debug(context.TODO(), text)
		}
	}
}
