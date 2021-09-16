// Copyright (C) 2021 ScyllaDB

package nodeconfigpod

import (
	"context"
	"fmt"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllertools"
	"github.com/scylladb/scylla-operator/pkg/naming"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

func (c *Controller) sync(ctx context.Context, key string) error {
	_, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		klog.ErrorS(err, "can't split meta namespace cache key", "cacheKey", key)
		return err
	}

	// Cache contains only Pods running on the same Node so it's cheap.
	scyllaPods, err := c.podLister.Pods(corev1.NamespaceAll).List(naming.ScyllaSelector())
	if err != nil {
		return fmt.Errorf("can't list all Pods on the node: %w", err)
	}

	klog.V(4).Infof("There are %d Scylla Pods running on the node", len(scyllaPods))

	snc, err := c.scyllaNodeConfigLister.Get(name)
	if err != nil {
		return fmt.Errorf("can't fetch ScyllaNodeConfig %s: %w", name, err)
	}

	jobs, err := c.getJobs(ctx, snc)
	if err != nil {
		return fmt.Errorf("can't get Jobs: %w", err)
	}

	var errs []error
	if err = c.syncJobs(ctx, snc, scyllaPods, jobs); err != nil {
		errs = append(errs, fmt.Errorf("can't sync Jobs: %w", err))
	}

	return utilerrors.NewAggregate(errs)
}

func (c *Controller) getJobs(ctx context.Context, snc *scyllav1alpha1.ScyllaNodeConfig) (map[string]*batchv1.Job, error) {
	// List all Job to find even those that no longer match our selector.
	// They will be orphaned in ClaimJob().
	jobs, err := c.jobLister.Jobs(naming.ScyllaOperatorNodeTuningNamespace).List(labels.Everything())
	if err != nil {
		return nil, fmt.Errorf("can't list Jobs: %w", err)
	}

	selector := labels.SelectorFromSet(labels.Set{
		naming.NodeConfigNameLabel:       snc.Name,
		naming.NodeConfigControllerLabel: c.nodeName,
	})

	canAdoptFunc := func() error {
		fresh, err := c.scyllaClient.ScyllaV1alpha1().ScyllaNodeConfigs().Get(ctx, snc.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		if fresh.UID != snc.UID {
			return fmt.Errorf("original ScyllaNodeConfig %v is gone: got uid %v, wanted %v", snc.Name, fresh.UID, snc.UID)
		}

		if fresh.GetDeletionTimestamp() != nil {
			return fmt.Errorf("%v has just been deleted at %v", snc.Name, snc.DeletionTimestamp)
		}
		return nil
	}
	cm := controllertools.NewJobControllerRefManager(
		ctx,
		snc,
		controllerGVK,
		selector,
		canAdoptFunc,
		controllertools.RealJobControl{
			KubeClient: c.kubeClient,
			Recorder:   c.eventRecorder,
		},
	)

	claimedJobs, err := cm.ClaimJobs(jobs)
	if err != nil {
		return nil, fmt.Errorf("can't claim jobs in %q namespace, %w", snc.Namespace, err)
	}

	return claimedJobs, nil
}
