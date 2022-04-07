// Copyright (C) 2022 ScyllaDB

package proc

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"k8s.io/klog/v2"
)

var (
	pidRegex           = regexp.MustCompile("^[0-9]+$")
	scyllaCommandRegex = regexp.MustCompile("^*/scylla$")
)

type ProcessInfo struct {
	Pid uint64
}

func (pi *ProcessInfo) ReadCmdline() ([]string, error) {
	cmdlinePath := fmt.Sprintf("/proc/%d/cmdline", pi.Pid)
	bytes, err := os.ReadFile(cmdlinePath)
	if err != nil {
		return nil, fmt.Errorf("can't read file %q: %w", cmdlinePath, err)
	}

	return strings.Split(string(bytes), "\x00"), nil
}

func ListProcesses() ([]*ProcessInfo, error) {
	f, err := os.Open("/proc")
	if err != nil {
		return nil, fmt.Errorf("can't to open /proc: %w", err)
	}
	defer func() {
		err := f.Close()
		if err != nil {
			klog.ErrorS(err, "Can't close /proc file")
		}
	}()

	files, err := f.Readdirnames(-1)
	if err != nil {
		return nil, fmt.Errorf("can't read dir content: %w", err)
	}

	var processes []*ProcessInfo
	for _, f := range files {
		if !pidRegex.Match([]byte(f)) {
			continue
		}

		pid, err := strconv.ParseUint(f, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("can't parse file name %q: %w", f, err)
		}

		processes = append(processes, &ProcessInfo{
			Pid: pid,
		})
	}

	return processes, nil
}

func FindProcessesMatchingCommand(r *regexp.Regexp) ([]*ProcessInfo, error) {
	procInfos, err := ListProcesses()
	if err != nil {
		return nil, fmt.Errorf("can't list processes: %w", err)
	}

	var processes []*ProcessInfo
	for _, pi := range procInfos {
		cmdLine, err := pi.ReadCmdline()
		if err != nil {
			return nil, fmt.Errorf("can't read cmdline for process %d: %w", pi.Pid, err)
		}

		if len(cmdLine) == 0 {
			return nil, fmt.Errorf("cmdline for process %d is empty: %w", pi.Pid, err)
		}

		command := cmdLine[0]
		if r.MatchString(command) {
			processes = append(processes, pi)
		}
	}

	return processes, nil
}

func SignalScylla(signal os.Signal) error {
	procInfos, err := FindProcessesMatchingCommand(scyllaCommandRegex)
	if err != nil {
		return fmt.Errorf("can't find processes: %w", err)
	}

	switch len(procInfos) {
	case 0:
		return errors.New("no scylla process found")
	case 1:
		break
	default:
		klog.Warning("Found more then one scylla process!")
	}

	// FIXME: parallelize for execution guarantee.
	for _, pi := range procInfos {
		p, err := os.FindProcess(int(pi.Pid))
		if err != nil {
			return fmt.Errorf("can't find process %q: %w", pi.Pid, err)
		}

		klog.InfoS("Sending signal to Scylla", "Signal", signal, "PID", pi.Pid)
		err = p.Signal(signal)
		if err != nil {
			return fmt.Errorf("can't find process %q: %w", pi.Pid, err)
		}
		klog.InfoS("Signal to Scylla was sent", "Signal", signal, "PID", pi.Pid)
	}

	return nil
}
