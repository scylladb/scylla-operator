// Copyright (c) 2024 ScyllaDB.

package operator

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/scylladb/scylla-operator/pkg/genericclioptions"
	"github.com/scylladb/scylla-operator/pkg/signals"
	"github.com/scylladb/scylla-operator/pkg/version"
	"github.com/spf13/cobra"
	"golang.org/x/sys/unix"
	apierrors "k8s.io/apimachinery/pkg/util/errors"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/klog/v2"
)

type RlimitsJobOptions struct {
	ScyllaPIDs []int
}

func NewRlimitsJobOptions(streams genericclioptions.IOStreams) *RlimitsJobOptions {
	return &RlimitsJobOptions{}
}

func NewRlimitsJobCmd(streams genericclioptions.IOStreams) *cobra.Command {
	o := NewRlimitsJobOptions(streams)

	cmd := &cobra.Command{
		Use:   "rlimits-job",
		Short: "Changes rlimits of Scylla processes.",
		Long:  "Changes rlimits of Scylla processes.",
		RunE: func(cmd *cobra.Command, args []string) error {
			err := o.Validate()
			if err != nil {
				return err
			}

			err = o.Complete()
			if err != nil {
				return err
			}

			err = o.Run(streams, cmd)
			if err != nil {
				return err
			}

			return nil
		},

		SilenceErrors: true,
		SilenceUsage:  true,
	}

	cmd.Flags().IntSliceVarP(&o.ScyllaPIDs, "scylla-pids", "", o.ScyllaPIDs, "List of PIDs of Scylla processes")

	return cmd
}

func (o *RlimitsJobOptions) Validate() error {
	var errs []error

	if len(o.ScyllaPIDs) == 0 {
		errs = append(errs, fmt.Errorf("scylla-pids cannot be empty"))
	}

	return apierrors.NewAggregate(errs)
}

func (o *RlimitsJobOptions) Complete() error {
	return nil
}

func (o *RlimitsJobOptions) Run(streams genericclioptions.IOStreams, cmd *cobra.Command) error {
	klog.InfoS("Starting rlimits Job", "version", version.Get())

	defer func(startTime time.Time) {
		klog.InfoS("Rlimits Job completed", "duration", time.Since(startTime))
	}(time.Now())

	cliflag.PrintFlags(cmd.Flags())

	stopCh := signals.StopChannel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		<-stopCh
		cancel()
	}()

	err := o.changeRlimits(ctx)
	if err != nil {
		return fmt.Errorf("can't change rlimits: %w", err)
	} else {
		klog.InfoS("Rlimits were changed successfully")
	}

	return nil
}

func (o *RlimitsJobOptions) changeRlimits(ctx context.Context) error {
	klog.InfoS("Checking maximum possible nofile limit")

	const (
		fsNrOpenPath = "/proc/sys/fs/nr_open"
	)

	fsNrOpenStr, err := os.ReadFile(fsNrOpenPath)
	if err != nil {
		return fmt.Errorf("can't read %q: %w", fsNrOpenPath, err)
	}

	maxNofileLimit, err := strconv.ParseUint(strings.TrimSpace(string(fsNrOpenStr)), 10, 64)
	if err != nil {
		return fmt.Errorf("can't parse %q to uin64: %w", fsNrOpenStr, err)
	}

	klog.InfoS("Discovered maximum possible nofile limit", "path", fsNrOpenPath, "limit", maxNofileLimit)

	var errs []error
	for _, scyllaPID := range o.ScyllaPIDs {
		klog.InfoS("Changing Scylla process nofile rlimits", "PID", scyllaPID)

		err := unix.Prlimit(scyllaPID, unix.RLIMIT_NOFILE, &unix.Rlimit{
			// Soft limit
			Cur: maxNofileLimit,
			// Hard limit
			Max: maxNofileLimit,
		}, nil)

		if err != nil {
			errs = append(errs, fmt.Errorf("can't set nofile rlimit of %d process: %w", scyllaPID, err))
			continue
		}
	}

	err = apierrors.NewAggregate(errs)
	if err != nil {
		return err
	}

	return nil
}
