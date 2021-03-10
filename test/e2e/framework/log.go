// Copyright (C) 2021 ScyllaDB

package framework

import (
	"fmt"
	"time"

	g "github.com/onsi/ginkgo"
	gconfig "github.com/onsi/ginkgo/config"
)

func nowStamp() string {
	return time.Now().Format(time.StampMilli)
}

func logf(level string, format string, args ...interface{}) {
	_, _ = fmt.Fprintf(g.GinkgoWriter, fmt.Sprintf("%v: %s: %s\n", nowStamp(), level, format), args...)
}

func Infof(format string, args ...interface{}) {
	logf("INFO", format, args...)
}

func Warnf(format string, args ...interface{}) {
	logf("WARN", format, args...)
}

func Errorf(format string, args ...interface{}) {
	logf("ERROR", format, args...)
}

func Failf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	logf("FAIL", msg)
	g.Fail(nowStamp()+": "+msg, 1)
}

func Skipf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	logf("INFO", msg)
	g.Skip(nowStamp() + ": " + msg)
}

func By(format string, args ...interface{}) {
	preamble := "\x1b[1mSTEP\x1b[0m"
	if gconfig.DefaultReporterConfig.NoColor {
		preamble = "STEP"
	}
	logf(preamble, format, args...)
}
