// Copyright (C) 2021 ScyllaDB

package nodeconfigdaemon

import (
	"context"
	"fmt"
	"time"

	"github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/controllertools"
	"github.com/scylladb/scylla-operator/pkg/naming"
	batchv1 "k8s.io/api/batch/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
)

func (ncdc *Controller) getCanAdoptFunc(ctx context.Context) func() error {
	return func() error {
		fresh, err := ncdc.scyllaClient.ScyllaV1alpha1().NodeConfigs().Get(ctx, ncdc.nodeConfigName, metav1.GetOptions{})
		if err != nil {
			return err
		}

		if fresh.UID != ncdc.nodeConfigUID {
			return fmt.Errorf("original NodeConfig %v is gone: got uid %v, wanted %v", ncdc.nodeConfigName, fresh.UID, ncdc.nodeConfigUID)
		}

		if fresh.GetDeletionTimestamp() != nil {
			return fmt.Errorf("%v has just been deleted at %v", fresh.Name, fresh.DeletionTimestamp)
		}

		return nil
	}
}

func (ncdc *Controller) getJobs(ctx context.Context) (map[string]*batchv1.Job, error) {
	// List all Job to find even those that no longer match our selector.
	// They will be orphaned in ClaimJob().
	allJobs, err := ncdc.namespacedJobLister.Jobs(ncdc.namespace).List(labels.Everything())
	if err != nil {
		return nil, fmt.Errorf("can't list Jobs: %w", err)
	}

	// This is a special case where the ownership is sharded between different nodes,
	// so we have to limit the set for orphaning.
	var jobs []*batchv1.Job
	for _, j := range allJobs {
		if j.Spec.Template.Spec.NodeName == ncdc.nodeName {
			jobs = append(jobs, j)
		}
	}

	selector := labels.SelectorFromSet(labels.Set{
		naming.NodeConfigJobForNodeUIDLabel: string(ncdc.nodeUID),
	})

	cr, err := ncdc.newOwningDSControllerRef()
	if err != nil {
		return nil, fmt.Errorf("can't get controller ref: %w", err)
	}

	cm := controllertools.NewJobControllerRefManager(
		ctx,
		&metav1.ObjectMeta{
			Name:              cr.Name,
			UID:               cr.UID,
			DeletionTimestamp: nil,
		},
		daemonSetControllerGVK,
		selector,
		ncdc.getCanAdoptFunc(ctx),
		controllertools.RealJobControl{
			KubeClient: ncdc.kubeClient,
			Recorder:   ncdc.eventRecorder,
		},
	)

	claimedJobs, err := cm.ClaimJobs(jobs)
	if err != nil {
		return nil, fmt.Errorf("can't claim jobs in %q namespace, %w", ncdc.namespace, err)
	}

	return claimedJobs, nil
}

func (ncdc *Controller) getCurrentNodeConfig(ctx context.Context) (*v1alpha1.NodeConfig, error) {
	nc, err := ncdc.nodeConfigLister.Get(ncdc.nodeConfigName)
	if err != nil {
		return nil, fmt.Errorf("can't get current node config %q: %w", ncdc.nodeConfigName, err)
	}

	if nc.UID != ncdc.nodeConfigUID {
		// In normal circumstances we should be deleted first by GC because of an ownerRef to the NodeConfig.
		return nil, fmt.Errorf("nodeConfig UID %q doesn't match the expected UID %q", nc.UID, nc.UID)
	}

	return nc, nil
}

func (ncdc *Controller) updateNodeStatus(ctx context.Context, nodeStatus *v1alpha1.NodeConfigNodeStatus) error {
	oldNC, err := ncdc.getCurrentNodeConfig(ctx)
	if err != nil {
		return err
	}

	nc := oldNC.DeepCopy()

	nc.Status.NodeStatuses = controllerhelpers.SetNodeStatus(nc.Status.NodeStatuses, nodeStatus)

	if apiequality.Semantic.DeepEqual(nc.Status.NodeStatuses, oldNC.Status.NodeStatuses) {
		return nil
	}

	klog.V(2).InfoS("Updating status", "NodeConfig", klog.KObj(oldNC), "Node", nodeStatus.Name)

	_, err = ncdc.scyllaClient.ScyllaV1alpha1().NodeConfigs().UpdateStatus(ctx, nc, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("can't update node config status %q: %w", ncdc.nodeConfigName, err)
	}

	klog.V(2).InfoS("Status updated", "NodeConfig", klog.KObj(oldNC), "Node", nodeStatus.Name)

	return nil
}

func (ncdc *Controller) sync(ctx context.Context) error {
	startTime := time.Now()
	klog.V(4).InfoS("Started sync", "startTime", startTime)
	defer func() {
		klog.V(4).InfoS("Finished sync", "duration", time.Since(startTime))
	}()

	jobs, err := ncdc.getJobs(ctx)
	if err != nil {
		return fmt.Errorf("can't get Jobs: %w", err)
	}

	nodeStatus := &v1alpha1.NodeConfigNodeStatus{
		Name: ncdc.nodeName,
	}

	var errs []error

	err = ncdc.syncJobs(ctx, jobs, nodeStatus)
	if err != nil {
		errs = append(errs, fmt.Errorf("can't sync jobs: %w", err))
	}

	err = ncdc.updateNodeStatus(ctx, nodeStatus)
	if err != nil {
		errs = append(errs, fmt.Errorf("can't update status: %w", err))
	}

	return utilerrors.NewAggregate(errs)
}
