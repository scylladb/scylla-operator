// Copyright (C) 2022 ScyllaDB

package proc

import (
	"os"
	"regexp"
	"testing"
)

func TestFindProcessesMatchingCommand(t *testing.T) {
	selfCmd := os.Args[0]
	t.Logf("self command: %s", selfCmd)

	selfPid := os.Getpid()
	t.Logf("self pid: %d", selfPid)

	selfCmdRegex := regexp.MustCompile(regexp.QuoteMeta(selfCmd))
	procInfos, err := FindProcessesMatchingCommand(selfCmdRegex)
	if err != nil {
		t.Fatal(err)
	}

	if len(procInfos) != 1 {
		t.Errorf("expected 1 proc info, got %d", len(procInfos))
	}

	gotPid := procInfos[0].Pid
	if int(gotPid) != selfPid {
		t.Errorf("expected pid %d, got %d", selfPid, gotPid)
	}
}
