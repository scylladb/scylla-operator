// Copyright (C) 2025 ScyllaDB

package sidecar

import (
	"fmt"
	"time"

	"github.com/scylladb/scylla-operator/pkg/controller/statusreport"
	"github.com/scylladb/scylla-operator/pkg/controllertools"
	"github.com/scylladb/scylla-operator/pkg/scyllaclient"
	"github.com/spf13/cobra"
	apimachineryutilerrors "k8s.io/apimachinery/pkg/util/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	ctrlmanager "sigs.k8s.io/controller-runtime/pkg/manager"
)

const (
	minStatusReportInterval = 1 * time.Second
)

type statusReporterOptions struct {
	statusReportInterval time.Duration
}

func (o *statusReporterOptions) AddFlags(cmd *cobra.Command) {
	cmd.Flags().DurationVarP(&o.statusReportInterval, "status-report-interval", "", o.statusReportInterval, "How often to poll the ScyllaDB node for status and report it.")
}

func (o *statusReporterOptions) Validate() error {
	var errs []error

	if o.statusReportInterval < 1 {
		errs = append(errs, fmt.Errorf("status-report-interval must not be lower than %s", minStatusReportInterval))
	}

	return apimachineryutilerrors.NewAggregate(errs)
}

func (o *statusReporterOptions) Complete() error {
	return nil
}

// StatusReporter runs the status report controller and re-runs it every interval.
type StatusReporter struct {
	controller *statusreport.Controller

	interval time.Duration
}

func NewStatusReporter(
	namespace string,
	podName string,
	interval time.Duration,
	c client.Client,
	newScyllaClient func() (*scyllaclient.Client, error),
) *StatusReporter {
	return &StatusReporter{
		controller: statusreport.NewController(
			namespace,
			podName,
			c,
			newScyllaClient,
		),
		interval: interval,
	}
}

// SetupWithManager registers the status report controller and its periodic trigger with the manager.
func (sr *StatusReporter) SetupWithManager(mgr ctrlmanager.Manager, options controller.Options) error {
	err := sr.controller.SetupWithManager(mgr, options)
	if err != nil {
		return fmt.Errorf("can't set up status report controller: %w", err)
	}

	err = mgr.Add(controllertools.PeriodicTrigger(sr.controller.Trigger(), sr.interval))
	if err != nil {
		return fmt.Errorf("can't add periodic trigger: %w", err)
	}

	return nil
}
