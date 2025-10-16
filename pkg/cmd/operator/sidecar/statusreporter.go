// Copyright (C) 2025 ScyllaDB

package sidecar

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/scylladb/scylla-operator/pkg/controller/statusreport"
	corev1informers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
)

type StatusReporter struct {
	statusreport.Controller

	interval time.Duration
}

func NewStatusReporter(
	namespace string,
	podName string,
	interval time.Duration,
	kubeClient kubernetes.Interface,
	podInformer corev1informers.PodInformer,
) (*StatusReporter, error) {
	sr := &StatusReporter{
		interval: interval,
	}

	c, err := statusreport.NewController(
		namespace,
		podName,
		kubeClient,
		podInformer,
	)
	if err != nil {
		return nil, fmt.Errorf("can't create status report controller: %w", err)
	}

	sr.Controller = *c

	return sr, nil
}

func (sr *StatusReporter) Run(ctx context.Context) {
	var wg sync.WaitGroup
	defer wg.Wait()

	// Run status report controller.
	wg.Add(1)
	go func() {
		defer wg.Done()
		sr.Controller.Run(ctx)
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
				sr.Enqueue()

			}
		}
	}()

	<-ctx.Done()
}
