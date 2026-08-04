package sidecar

import (
	"context"
	"fmt"
	"maps"
	"os/exec"
	"time"

	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/scyllaclient"
	"github.com/scylladb/scylla-operator/pkg/util/hash"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apimachineryutilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
)

const (
	// requeueWaitDuration is the delay used when the sidecar polls ScyllaDB's local state and has to wait for it to
	// advance. The happy path with tablets takes single-digit seconds. The value should keep the poll
	// count low while staying well below the slow paths (30s ring delay, 60s CDC generation propagation).
	requeueWaitDuration = 5 * time.Second
)

func (c *Controller) decommissionNode(ctx context.Context, svc *corev1.Service) error {
	scyllaClient, err := c.newScyllaClient()
	if err != nil {
		return fmt.Errorf("can't create a new ScyllaClient: %w", err)
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

	scyllaClient, err := c.newScyllaClient()
	if err != nil {
		return fmt.Errorf("can't create a new ScyllaClient: %w", err)
	}
	defer scyllaClient.Close()

	var errs []error
	annotations, requeue, err := c.getRequiredServiceAnnotations(ctx, scyllaClient)
	if err != nil {
		errs = append(errs, fmt.Errorf("can't get required service annotations: %w", err))
	}

	if requeue {
		klog.V(4).InfoS("Requeuing to sync Service annotations later", "Service", klog.KObj(svc), "After", requeueWaitDuration.String())
		c.queue.AddAfter(c.key, requeueWaitDuration)
	}

	err = c.updateServiceAnnotations(ctx, svc, annotations)
	if err != nil {
		errs = append(errs, fmt.Errorf("can't update service annotations: %w", err))
	}

	return apimachineryutilerrors.NewAggregate(errs)
}

// getRequiredServiceAnnotations reads the ScyllaDB API and returns the annotations required on the member Service,
// a boolean stating if it should be requeued, and an error. Explicit requeue is not requested on an error.
// Annotations are produced on a best-effort basis: only those backed by a successful observation are returned, so
// a non-nil error comes with whatever was produced before it and the caller should leave the rest untouched (keep existing).
func (c *Controller) getRequiredServiceAnnotations(ctx context.Context, scyllaClient *scyllaclient.Client) (map[string]string, bool, error) {
	annotations := map[string]string{}

	hostID, err := c.getHostID(ctx, scyllaClient, c.localhostAddress)
	if err != nil {
		return nil, false, fmt.Errorf("can't get HostID: %w", err)
	}

	annotations[naming.HostIDAnnotation] = hostID

	isMember, isKnown, err := nodeIsScyllaDBClusterMember(ctx, scyllaClient, c.localhostAddress, hostID)
	if err != nil {
		return annotations, false, fmt.Errorf("can't determine ScyllaDB cluster membership: %w", err)
	}

	if !isKnown {
		klog.V(4).InfoS("Node hasn't joined the ScyllaDB cluster yet", "HostID", hostID)
		return annotations, true, nil
	}

	if !isMember {
		klog.V(4).InfoS("Node doesn't own any tokens yet", "HostID", hostID)
		annotations[naming.NodeJoinedScyllaDBClusterAnnotation] = naming.LabelValueFalse
		return annotations, true, nil
	}

	annotations[naming.NodeJoinedScyllaDBClusterAnnotation] = naming.LabelValueTrue

	currentTokenRingHash, err := getTokenRingHash(ctx, scyllaClient, c.localhostAddress)
	if err != nil {
		return annotations, false, fmt.Errorf("can't get current token ring hash: %w", err)
	}
	annotations[naming.CurrentTokenRingHashAnnotation] = currentTokenRingHash

	return annotations, false, nil
}

// nodeIsScyllaDBClusterMember reports whether the node is a member of the ScyllaDB cluster, i.e. whether it owns normal
// tokens in the cluster's token metadata. Unlike the node's operation mode, this survives a restart: a bootstrapped node
// has its token metadata restored from disk before gossip starts, so it never stops owning normal tokens while it boots.
// A node that is bootstrapping holds only pending tokens and is not a member until the operation completes.
// The second return value reports whether membership could be determined at all. When it is false, the caller must not act
// on the first one.
func nodeIsScyllaDBClusterMember(ctx context.Context, scyllaClient *scyllaclient.Client, localhostAddr string, hostID string) (bool, bool, error) {
	ipToHostIDMap, err := scyllaClient.GetIPToHostIDMap(ctx, localhostAddr)
	if err != nil {
		return false, false, fmt.Errorf("can't get host id to ip mapping: %w", err)
	}
	klog.V(4).InfoS("Got IP to HostID mapping", "IPToHostIDMap", ipToHostIDMap)

	var localIP string
	for ip, id := range ipToHostIDMap {
		if id == hostID {
			localIP = ip
			break
		}
	}

	if len(localIP) == 0 {
		// The node is not present in the cluster's token metadata, so its membership can't be determined.
		return false, false, nil
	}

	// Only normal tokens are reported for the endpoint, pending tokens of a bootstrapping or replacing node are not.
	nodeTokens, err := scyllaClient.GetNodeTokens(ctx, localhostAddr, localIP)
	if err != nil {
		return false, false, fmt.Errorf("can't get node tokens: %w", err)
	}

	return len(nodeTokens) != 0, true, nil
}

func getTokenRingHash(ctx context.Context, scyllaClient *scyllaclient.Client, localhostAddr string) (string, error) {
	tokenRing, err := scyllaClient.GetTokenRing(ctx, localhostAddr)
	if err != nil {
		return "", fmt.Errorf("can't get token ring: %w", err)
	}

	h, err := hash.HashObjects(tokenRing)
	if err != nil {
		return "", fmt.Errorf("can't hash token ring: %w", err)
	}

	return h, nil
}

func (c *Controller) updateServiceAnnotations(ctx context.Context, svc *corev1.Service, annotations map[string]string) error {
	svcCopy := svc.DeepCopy()
	if svcCopy.Annotations == nil {
		svcCopy.Annotations = make(map[string]string)
	}

	maps.Insert(svcCopy.Annotations, maps.All(annotations))

	if equality.Semantic.DeepEqual(svc, svcCopy) {
		return nil
	}

	_, err := c.kubeClient.CoreV1().Services(svcCopy.Namespace).Update(ctx, svcCopy, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("can't update Service %q: %w", naming.ObjRef(svc), err)
	}

	klog.V(2).InfoS("Successfully updated Service annotations", "Service", klog.KObj(svc))

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
