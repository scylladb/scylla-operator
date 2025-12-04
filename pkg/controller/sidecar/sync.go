package sidecar

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/helpers"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/scyllaclient"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apimachineryutilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
)

const (
	requeueWaitDuration = 5 * time.Second
)

func (c *Controller) decommissionNode(ctx context.Context, svc *corev1.Service) error {
	parsedIP, err := helpers.ParseIP(c.localhostAddress)
	if err != nil {
		return fmt.Errorf("can't parse localhost address %q: %w", c.localhostAddress, err)
	}
	ipFamily := helpers.GetIPFamily(parsedIP)

	scyllaClient, err := controllerhelpers.NewScyllaClientForLocalhost(ipFamily)
	if err != nil {
		return err
	}
	defer scyllaClient.Close()

	opMode, err := scyllaClient.OperationMode(ctx, c.localhostAddress)
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
		c.queue.AddAfter(c.key, requeueWaitDuration)
		return nil

	case scyllaclient.OperationalModeNormal:
		// Node can be in NORMAL mode while still starting up.
		// Last thing that scylla is doing as part of startup process is brining native transport up
		// so we check if native port is up as sign that it is not loading.
		nativeUp, err := scyllaClient.IsNativeTransportEnabled(ctx, c.localhostAddress)
		if err != nil {
			return fmt.Errorf("can't get native transport status: %w", err)
		}

		if !nativeUp {
			klog.V(2).InfoS("Node native transport is down, it is sign that node is starting up. Waiting a bit.")
			c.queue.AddAfter(c.key, requeueWaitDuration)
			return nil
		}

		// Decommission the node only if it is in normal mode and native transport is up.
		decommissionErr := scyllaClient.Decommission(ctx, c.localhostAddress)
		if decommissionErr != nil {
			// Decommission is long running task, so request fails due to the timeout in most cases.
			// To not raise an error, when it is in progress, we check opMode.
			opMode, err := scyllaClient.OperationMode(ctx, c.localhostAddress)
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

func (c *Controller) syncAnnotations(ctx context.Context, svc *corev1.Service) error {
	startTime := time.Now()
	klog.V(4).InfoS("Started syncing Service annotation", "Service", klog.KObj(svc), "startTime", startTime)
	defer func() {
		klog.V(4).InfoS("Finished syncing Service annotation", "Service", klog.KObj(svc), "duration", time.Since(startTime))
	}()

	parsedIP, err := helpers.ParseIP(c.localhostAddress)
	if err != nil {
		return fmt.Errorf("can't parse localhost address %q: %w", c.localhostAddress, err)
	}
	ipFamily := helpers.GetIPFamily(parsedIP)

	scyllaClient, err := controllerhelpers.NewScyllaClientForLocalhost(ipFamily)
	if err != nil {
		return fmt.Errorf("can't create a new ScyllaClient for localhost: %w", err)
	}
	defer scyllaClient.Close()

	hostID, err := c.getHostID(ctx, scyllaClient, c.localhostAddress)
	if err != nil {
		return fmt.Errorf("can't get HostID: %w", err)
	}

	ipToHostIDMap, err := scyllaClient.GetIPToHostIDMap(ctx, c.localhostAddress)
	if err != nil {
		return fmt.Errorf("can't get host id to ip mapping: %w", err)
	}

	var localIP string
	for ip, id := range ipToHostIDMap {
		if id == hostID {
			localIP = ip
			break
		}
	}

	if len(localIP) == 0 {
		// It most likely means that the Scylla node the sidecar is running on has not joined the cluster yet. We need
		// to requeue and try again later.
		klog.V(2).InfoS("The node with the expected HostID has not joined the cluster yet, will retry in a bit", "HostID", hostID, "IPToHostIDMap", ipToHostIDMap)
		c.queue.AddRateLimited(c.key)
		return nil
	}

	nodeTokens, err := scyllaClient.GetNodeTokens(ctx, c.localhostAddress, localIP)
	if err != nil {
		return fmt.Errorf("can't get node tokens: %w", err)
	}

	svcCopy := svc.DeepCopy()
	svcCopy.Annotations[naming.HostIDAnnotation] = hostID

	var currentTokenRingHash string
	if len(nodeTokens) == 0 {
		klog.V(4).InfoS("Node doesn't have any tokens assigned, looks like it's still bootstrapping, requeueing")
		c.queue.AddAfter(c.key, requeueWaitDuration)
	} else {
		currentTokenRingHash, err = c.getTokenRingHash(ctx, scyllaClient, c.localhostAddress)
		if err != nil {
			return fmt.Errorf("can't get token hash: %w", err)
		}

		svcCopy.Annotations[naming.CurrentTokenRingHashAnnotation] = currentTokenRingHash
	}

	if !equality.Semantic.DeepEqual(svc, svcCopy) {
		_, err = c.kubeClient.CoreV1().Services(svcCopy.Namespace).Update(ctx, svcCopy, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("can't update service %q: %w", naming.ObjRef(svc), err)
		}

		klog.V(2).InfoS("Successfully updated service annotations", "Service", klog.KObj(svc))
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
		klog.V(2).InfoS("Service has been deleted", "Service", klog.KObj(svc))
		return nil
	}
	if err != nil {
		return err
	}

	if svc.DeletionTimestamp != nil {
		return nil
	}

	var errs []error

	err = c.syncAnnotations(ctx, svc)
	if err != nil {
		errs = append(errs, fmt.Errorf("can't sync the HostID annotation: %w", err))
	}

	decommissionValue, hasDecommissionLabel := svc.Labels[naming.DecommissionedLabel]
	if hasDecommissionLabel && decommissionValue != "true" {
		err := c.decommissionNode(ctx, svc)
		if err != nil {
			errs = append(errs, fmt.Errorf("can't decommision a node: %w", err))
		}
	}

	return apimachineryutilerrors.NewAggregate(errs)
}
