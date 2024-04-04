// Copyright (C) 2017 ScyllaDB

package rclone

import (
	"bytes"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/accounting"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/lib/terminal"
)

const (
	// Interval between progress prints.
	defaultProgressInterval = 500 * time.Millisecond
)

// StartProgress starts the progress bar printing
//
// It returns a func which should be called to stop the stats.
func StartProgress() func() {
	stopStats := make(chan struct{})
	oldLogPrint := fs.LogPrint
	oldSyncPrint := operations.SyncPrintf

	// Intercept output from functions such as HashLister to stdout
	operations.SyncPrintf = func(format string, a ...interface{}) {
		printProgress(fmt.Sprintf(format, a...))
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		progressInterval := defaultProgressInterval
		ticker := time.NewTicker(progressInterval)
		for {
			select {
			case <-ticker.C:
				printProgress("")
			case <-stopStats:
				ticker.Stop()
				printProgress("")
				fs.LogPrint = oldLogPrint
				operations.SyncPrintf = oldSyncPrint
				fmt.Println("")
				return
			}
		}
	}()
	return func() {
		close(stopStats)
		wg.Wait()
	}
}

// State for the progress printing.
var (
	nlines     = 0 // number of lines in the previous stats block
	progressMu sync.Mutex
)

// printProgress prints the progress with an optional log.
func printProgress(logMessage string) {
	progressMu.Lock()
	defer progressMu.Unlock()

	var buf bytes.Buffer
	w, _ := terminal.GetSize()
	stats := strings.TrimSpace(accounting.GlobalStats().String())
	logMessage = strings.TrimSpace(logMessage)

	out := func(s string) {
		buf.WriteString(s)
	}

	if logMessage != "" {
		out("\n")
		out(terminal.MoveUp)
	}
	// Move to the start of the block we wrote erasing all the previous lines
	for i := 0; i < nlines-1; i++ {
		out(terminal.EraseLine)
		out(terminal.MoveUp)
	}
	out(terminal.EraseLine)
	out(terminal.MoveToStartOfLine)
	if logMessage != "" {
		out(terminal.EraseLine)
		out(logMessage + "\n")
	}
	fixedLines := strings.Split(stats, "\n")
	nlines = len(fixedLines)
	for i, line := range fixedLines {
		if len(line) > w {
			line = line[:w]
		}
		out(line)
		if i != nlines-1 {
			out("\n")
		}
	}
	terminal.Write(buf.Bytes())
}
