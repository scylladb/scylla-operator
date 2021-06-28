package sidecar

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	"github.com/scylladb/scylla-operator/pkg/controller/helpers"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/scyllaclient"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
)

const requeueWaitDuration = 5 * time.Second

func (c *Controller) getScyllaClient() (*scyllaclient.Client, error) {
	secret, err := c.secretLister.Secrets(c.namespace).Get(c.secretName)
	if err != nil {
		return nil, fmt.Errorf("can't get manager agent auth secret %s/%s: %w", c.namespace, c.secretName, err)
	}

	return helpers.NewScyllaClientFromSecret(secret, []string{c.hostAddr})
}

func (c *Controller) decommissionNode(ctx context.Context, svc *corev1.Service) error {
	scyllaClient, err := c.getScyllaClient()
	if err != nil {
		return err
	}

	opMode, err := scyllaClient.OperationMode(ctx, c.hostAddr)
	if err != nil {
		return fmt.Errorf("can't get node operation mode: %w", err)
	}

	klog.V(4).InfoS("Scylla operation mode", "Mode", opMode)
	switch opMode {
	case scyllaclient.OperationalModeLeaving, scyllaclient.OperationalModeDecommissioning, scyllaclient.OperationalModeDraining:
		// If node is leaving/draining/decommissioning, keep retrying.
		klog.V(2).InfoS("Waiting for scylla to finish the operation, requeuing", "Mode", opMode)
		c.queue.AddAfter(c.key, requeueWaitDuration)
		return nil

	case scyllaclient.OperationalModeDrained:
		klog.InfoS("Node is in DRAINED state, restarting scylla to make it decommissionable")
		// TODO: Label pod/service that it is in restarting state to avoid liveness probe race
		_, err := exec.Command("supervisorctl", "restart", "scylla").Output()
		if err != nil {
			return fmt.Errorf("can't restart scylla node: %w", err)
		}
		klog.InfoS("Successfully restarted scylla.")
		return nil

	case scyllaclient.OperationalModeNormal:
		// Node can be in NORMAL mode while still starting up.
		// Last thing that scylla is doing as part of startup process is brining native transport up
		// so we check if native port is up as sign that it is not loading.
		nativeUp, err := scyllaClient.IsNativeTransportEnabled(ctx, c.hostAddr)
		if err != nil {
			return fmt.Errorf("can't get native transport status: %w", err)
		}

		if !nativeUp {
			klog.V(2).InfoS("Node native transport is down, it is sign that node is starting up. Waiting a bit.")
			c.queue.AddAfter(c.key, requeueWaitDuration)
			return nil
		}

		// Decommission the node only if it is in normal mode and native transport is up.
		decommissionErr := scyllaClient.Decommission(ctx, c.hostAddr)
		if decommissionErr != nil {
			// Decommission is long running task, so request fails due to the timeout in most cases.
			// To not raise an error, when it is in progress, we check opMode.
			opMode, err := scyllaClient.OperationMode(ctx, c.hostAddr)
			if err == nil && (opMode.IsDecommissioned() || opMode.IsLeaving() || opMode.IsDecommissioning()) {
				klog.V(2).InfoS("Decommissioning is in progress. Waiting a bit.", "Mode", opMode)
				c.queue.AddAfter(c.key, requeueWaitDuration)
				return nil
			}

			return fmt.Errorf("can't decommission the node: %w", decommissionErr)
		}

	case scyllaclient.OperationalModeJoining:
		// If node is joining we need to wait till it reaches Normal state and then decommission it
		klog.V(2).InfoS("Can't decommission a joining node. Waiting a bit.")
		c.queue.AddAfter(c.key, requeueWaitDuration)
		return nil

	case scyllaclient.OperationalModeDecommissioned:
		klog.V(2).InfoS("The node is already decommissioned")

	default:
		return fmt.Errorf("unexpected node operation mode: %s", opMode)
	}

	// Update Label to signal that decommission has completed
	svcCopy := svc.DeepCopy()
	svcCopy.Labels[naming.DecommissionedLabel] = naming.LabelValueTrue
	_, err = c.kubeClient.CoreV1().Services(svcCopy.Namespace).Update(ctx, svcCopy, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	return nil
}

func (c *Controller) sync(ctx context.Context) error {
	startTime := time.Now()
	klog.V(4).InfoS("Started syncing Service", "Service", klog.KRef(c.namespace, c.serviceName), "startTime", startTime)
	defer func() {
		klog.V(4).InfoS("Finished syncing Service", "Service", klog.KRef(c.namespace, c.serviceName), "duration", time.Since(startTime))
	}()

	svc, err := c.singleServiceLister.Services(c.namespace).Get(c.serviceName)
	if errors.IsNotFound(err) {
		klog.V(2).InfoS("Service has been deleted", "ScyllaCluster", klog.KObj(svc))
		return nil
	}
	if err != nil {
		return err
	}

	if svc.DeletionTimestamp != nil {
		return nil
	}

	var errs []error

	decommissionValue, hasDecommissionLabel := svc.Labels[naming.DecommissionedLabel]
	if hasDecommissionLabel && decommissionValue != "true" {
		err := c.decommissionNode(ctx, svc)
		if err != nil {
			errs = append(errs, err)
		}
	}

	return utilerrors.NewAggregate(errs)
}
