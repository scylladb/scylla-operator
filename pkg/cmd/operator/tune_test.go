// Copyright (C) 2021 ScyllaDB

package operator

import (
	"context"
	"reflect"
	"testing"

	linuxproc "github.com/c9s/goprocinfo/linux"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/util/cpuset"
)

func TestPerftuneCommand(t *testing.T) {
	ts := []struct {
		Name            string
		NumCPUs         int
		AllowedCPUs     []int
		ExpectedIRQCPUs []int
	}{
		{
			Name:            "single allowed",
			NumCPUs:         4,
			AllowedCPUs:     []int{0},
			ExpectedIRQCPUs: []int{1, 2, 3},
		},
		{
			Name:            "gap in the middle",
			NumCPUs:         4,
			AllowedCPUs:     []int{0, 1, 3},
			ExpectedIRQCPUs: []int{2},
		},
		{
			Name:            "last forbidden",
			NumCPUs:         4,
			AllowedCPUs:     []int{0, 1, 2},
			ExpectedIRQCPUs: []int{3},
		},
		{
			Name:            "all allowed",
			NumCPUs:         4,
			AllowedCPUs:     []int{0, 1, 2, 3},
			ExpectedIRQCPUs: []int{},
		},
	}

	for i := range ts {
		test := ts[i]
		cpuset.NewCPUSet(1, 2, 3)
		t.Run(test.Name, func(t *testing.T) {
			ctx := context.Background()
			cpuInfo := &linuxproc.CPUInfo{
				Processors: make([]linuxproc.Processor, test.NumCPUs),
			}
			procStatus := &linuxproc.ProcessStatus{
				CpusAllowed: MustMask(test.AllowedCPUs...),
			}

			cmd, err := perftuneCommand(ctx, cpuInfo, procStatus)
			if err != nil {
				t.Errorf("Expected non-nil error, got %s", err)
			}
			expectedArgs := []string{
				"/opt/scylladb/scripts/perftune.py",
				"--tune", "net", "--irq-cpu-mask", cpuset.NewCPUSet(test.ExpectedIRQCPUs...).FormatMask(),
				"--tune", "disks", "--dir", naming.DataDir,
			}
			if !reflect.DeepEqual(cmd.Args, expectedArgs) {
				t.Errorf("Expected args %v, got %v", expectedArgs, cmd.Args)
			}
		})
	}
}

func MustMask(cpus ...int) []uint32 {
	cs := cpuset.NewCPUSet(cpus...)
	mask, err := cs.Mask()
	if err != nil {
		panic(err)
	}
	return mask
}
