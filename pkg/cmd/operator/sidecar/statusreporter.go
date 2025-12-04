// Copyright (C) 2025 ScyllaDB

package sidecar

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/scylladb/scylla-operator/pkg/controller/statusreport"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	apimachineryutilerrors "k8s.io/apimachinery/pkg/util/errors"
	corev1informers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
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

type StatusReporter struct {
	controller *statusreport.Controller

	interval time.Duration
}

func NewStatusReporter(
	namespace string,
	podName string,
	interval time.Duration,
	ipFamily corev1.IPFamily,
	kubeClient kubernetes.Interface,
	podInformer corev1informers.PodInformer,
) (*StatusReporter, error) {
	sr := &StatusReporter{
		interval: interval,
	}

	c, err := statusreport.NewController(
		namespace,
		podName,
		ipFamily,
		kubeClient,
		podInformer,
	)
	if err != nil {
		return nil, fmt.Errorf("can't create status report controller: %w", err)
	}

	sr.controller = c

	return sr, nil
}

func (sr *StatusReporter) Run(ctx context.Context) {
	var wg sync.WaitGroup
	defer wg.Wait()

	// Run status report controller.
	wg.Add(1)
	go func() {
		defer wg.Done()
		sr.controller.Run(ctx)
	}()

	// Enqueue periodically.
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(sr.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return

			case <-ticker.C:
				sr.controller.Enqueue()

			}
		}
	}()

	<-ctx.Done()
}
