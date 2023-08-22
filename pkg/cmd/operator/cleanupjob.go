// Copyright (c) 2023 ScyllaDB.

package operator

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/genericclioptions"
	"github.com/scylladb/scylla-operator/pkg/helpers"
	"github.com/scylladb/scylla-operator/pkg/scyllaclient"
	"github.com/scylladb/scylla-operator/pkg/signals"
	"github.com/scylladb/scylla-operator/pkg/version"
	"github.com/spf13/cobra"
	apierrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/klog/v2"
)

const (
	stopCleanupOperationTimeout = 10 * time.Second
)

type CleanupJobOptions struct {
	ManagerAuthConfigPath string
	NodeAddress           string

	scyllaClient *scyllaclient.Client
}

func NewCleanupJobOptions(streams genericclioptions.IOStreams) *CleanupJobOptions {
	return &CleanupJobOptions{}
}

func NewCleanupJobCmd(streams genericclioptions.IOStreams) *cobra.Command {
	o := NewCleanupJobOptions(streams)

	cmd := &cobra.Command{
		Use:   "cleanup-job",
		Short: "Runs a cleanup procedure against a node.",
		Long:  "Runs a cleanup procedure against a node.",
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

	cmd.Flags().StringVarP(&o.ManagerAuthConfigPath, "manager-auth-config-path", "", o.ManagerAuthConfigPath, "Path to a file containing Scylla Manager config containing auth token.")
	cmd.Flags().StringVarP(&o.NodeAddress, "node-address", "", o.NodeAddress, "Address of a node where cleanup will be performed.")

	return cmd
}

func (o *CleanupJobOptions) Validate() error {
	var errs []error

	if len(o.ManagerAuthConfigPath) == 0 {
		errs = append(errs, fmt.Errorf("manager-auth-config-path cannot be empty"))
	}

	if len(o.NodeAddress) == 0 {
		errs = append(errs, fmt.Errorf("node-address cannot be empty"))
	}

	return apierrors.NewAggregate(errs)
}

func (o *CleanupJobOptions) Complete() error {
	var err error

	buf, err := os.ReadFile(o.ManagerAuthConfigPath)
	if err != nil {
		return fmt.Errorf("can't read auth token file at %q: %w", o.ManagerAuthConfigPath, err)
	}

	authToken, err := helpers.ParseTokenFromConfig(buf)
	if err != nil {
		return fmt.Errorf("can't parse auth token file at %q: %w", o.ManagerAuthConfigPath, err)
	}

	if len(authToken) == 0 {
		return fmt.Errorf("manager agent auth token cannot be empty")
	}

	o.scyllaClient, err = controllerhelpers.NewScyllaClientFromToken([]string{o.NodeAddress}, authToken)
	if err != nil {
		return fmt.Errorf("can't create scylla client: %w", err)
	}

	return nil
}

func (o *CleanupJobOptions) Run(streams genericclioptions.IOStreams, cmd *cobra.Command) error {
	klog.InfoS("Starting the node cleanup", "version", version.Get())

	defer func(startTime time.Time) {
		klog.InfoS("Node cleanup completed", "duration", time.Since(startTime))
	}(time.Now())

	cliflag.PrintFlags(cmd.Flags())

	stopCh := signals.StopChannel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		<-stopCh
		cancel()
	}()

	var errs []error

	cleanupErr := o.runCleanup(ctx)
	if cleanupErr != nil {
		errs = append(errs, fmt.Errorf("can't run the cleanup: %w", cleanupErr))
	} else {
		klog.InfoS("Node cleanup finished successfully")
	}

	select {
	case <-stopCh:
		if cleanupErr == nil {
			break
		}

		klog.InfoS("Stopping any ongoing cleanup due to stop signal")
		stopCleanupCtx, stopCleanupCtxCancel := context.WithTimeout(context.Background(), stopCleanupOperationTimeout)
		defer stopCleanupCtxCancel()

		wait.UntilWithContext(stopCleanupCtx, func(ctx context.Context) {
			err := o.scyllaClient.StopCleanup(ctx, o.NodeAddress)
			if err != nil {
				errs = append(errs, fmt.Errorf("can't stop the cleanup: %w", err))
			} else {
				stopCleanupCtxCancel()
			}
		}, time.Second)
	default:
	}

	err := apierrors.NewAggregate(errs)
	if err != nil {
		return fmt.Errorf("can't clean up: %w", err)
	}

	return nil
}

func (o *CleanupJobOptions) runCleanup(ctx context.Context) error {
	klog.InfoS("Stopping any ongoing cleanup")
	err := o.scyllaClient.StopCleanup(ctx, o.NodeAddress)
	if err != nil {
		return fmt.Errorf("can't stop the cleanup: %w", err)
	}

	keyspaces, err := o.scyllaClient.Keyspaces(ctx)
	if err != nil {
		return fmt.Errorf("can't get list of keyspaces: %w", err)
	}

	klog.InfoS("Discovered keyspaces for cleanup", "keyspaces", keyspaces)

	var errs []error
	for _, keyspace := range keyspaces {
		klog.InfoS("Starting a keyspace cleanup", "keyspace", keyspace)
		startTime := time.Now()

		err = o.scyllaClient.Cleanup(ctx, o.NodeAddress, keyspace)
		if err != nil {
			klog.Warningf("Can't cleanup keyspace %q: %s", keyspace, err)
			errs = append(errs, fmt.Errorf("can't cleanup keyspace %q: %w", keyspace, err))
			continue
		}

		klog.InfoS("Finished keyspace cleanup", "keyspace", keyspace, "duration", time.Since(startTime))
	}

	err = apierrors.NewAggregate(errs)
	if err != nil {
		return err
	}

	return nil
}
