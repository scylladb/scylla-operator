// Copyright (C) 2025 ScyllaDB

package statusreport

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/controllertools"
	"github.com/scylladb/scylla-operator/pkg/ctrlclient"
	"github.com/scylladb/scylla-operator/pkg/internalapi"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	"github.com/scylladb/scylla-operator/pkg/scyllaclient"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	ctrlbuilder "sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	ctrlmanager "sigs.k8s.io/controller-runtime/pkg/manager"
)

const (
	localhost = "localhost"
)

const (
	// ControllerName names the controller within controller-runtime: in its logs, metrics and the workqueue.
	ControllerName = "status-report"
)

// Controller is an observer: it reports what the local ScyllaDB node sees of the cluster in an annotation on the node's
// Pod, on every Pod event and whenever it is triggered.
type Controller struct {
	namespace string
	podName   string

	client          client.Client
	newScyllaClient func() (*scyllaclient.Client, error)

	trigger *controllertools.Trigger
}

func NewController(
	namespace string,
	podName string,
	c client.Client,
	newScyllaClient func() (*scyllaclient.Client, error),
) *Controller {
	return &Controller{
		namespace: namespace,
		podName:   podName,

		client:          c,
		newScyllaClient: newScyllaClient,

		trigger: controllertools.NewTrigger(),
	}
}

// CacheOptions restricts the manager's cache to the node's Pod in namespace.
func CacheOptions(namespace, podName string) cache.Options {
	return cache.Options{
		DefaultNamespaces: map[string]cache.Config{
			namespace: {},
		},
		ByObject: map[client.Object]cache.ByObject{
			&corev1.Pod{}: {
				Field: fields.OneTermEqualSelector("metadata.name", podName),
			},
		},
	}
}

// SetupWithManager registers the controller with the manager. Every Pod event and every Enqueue re-runs the sync.
func (c *Controller) SetupWithManager(mgr ctrlmanager.Manager, options controller.Options) error {
	return ctrlbuilder.ControllerManagedBy(mgr).
		Named(ControllerName).
		Watches(&corev1.Pod{}, controllertools.EnqueueSingleton(ControllerName)).
		WatchesRawSource(c.trigger.Source(ControllerName)).
		WithOptions(options).
		Complete(controllertools.NewObserverReconciler(ControllerName, c.Sync))
}

// Enqueue requests a sync outside the Pod's watch.
func (c *Controller) Enqueue() {
	c.trigger.Enqueue()
}

// Trigger returns the trigger that requests a sync outside the Pod's watch.
func (c *Controller) Trigger() *controllertools.Trigger {
	return c.trigger
}

func (c *Controller) Sync(ctx context.Context) error {
	startTime := time.Now()
	klog.V(4).InfoS("Started syncing observer", "Name", ControllerName, "startTime", startTime)
	defer func() {
		klog.V(4).InfoS("Finished syncing observer", "Name", ControllerName, "duration", time.Since(startTime))
	}()

	pod, err := ctrlclient.Get[corev1.Pod](ctx, c.client, c.namespace, c.podName)
	if err != nil {
		return fmt.Errorf("can't get Pod %q: %v", naming.ManualRef(c.namespace, c.podName), err)
	}

	nodeStatusReport := c.getNodeStatusReport(ctx)
	encodedNodeStatusReport, err := nodeStatusReport.Encode()
	if err != nil {
		return fmt.Errorf("can't encode node status report: %w", err)
	}

	encodedNodeStatusReportString := string(encodedNodeStatusReport)

	if controllerhelpers.HasMatchingAnnotation(pod, naming.NodeStatusReportAnnotation, encodedNodeStatusReportString) {
		klog.V(5).InfoS("Pod already has up-to-date node status report annotation", "Pod", naming.ObjRef(pod))
		return nil
	}

	klog.V(4).InfoS("Patching Pod with new node status report annotation", "Pod", naming.ObjRef(pod), "NodeStatusReport", nodeStatusReport)
	patch, err := controllerhelpers.PrepareSetAnnotationPatch(pod, naming.NodeStatusReportAnnotation, pointer.Ptr[string](string(encodedNodeStatusReport)))
	if err != nil {
		return fmt.Errorf("can't prepare annotation patch: %w", err)
	}

	err = c.client.Patch(ctx, &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: c.namespace,
			Name:      c.podName,
		},
	}, client.RawPatch(types.StrategicMergePatchType, patch))
	if err != nil {
		return fmt.Errorf("can't patch pod %q: %w", naming.ObjRef(pod), err)
	}

	klog.V(4).InfoS("Finished patching Pod with new node status report annotation", "Pod", naming.ObjRef(pod))

	return nil
}

func (c *Controller) getNodeStatusReport(ctx context.Context) *internalapi.NodeStatusReport {
	scyllaClient, err := c.newScyllaClient()
	if err != nil {
		return &internalapi.NodeStatusReport{
			Error: pointer.Ptr(fmt.Errorf("can't create Scylla client for localhost: %w", err).Error()),
		}
	}
	defer scyllaClient.Close()

	nodeStatuses, err := scyllaClient.NodesStatusInfo(ctx, localhost)
	if err != nil {
		return &internalapi.NodeStatusReport{
			Error: pointer.Ptr(fmt.Errorf("can't get node status info: %w", err).Error()),
		}
	}

	observedNodeStatuses := make([]scyllav1alpha1.ObservedNodeStatus, 0, len(nodeStatuses))
	for _, ns := range nodeStatuses {
		observedNodeStatuses = append(observedNodeStatuses, scyllav1alpha1.ObservedNodeStatus{
			HostID: ns.HostID,
			Status: scyllaClientNodeStatusToScyllaV1Alpha1NodeStatus(ns.Status),
		})
	}

	// Ordering by HostID guarantees stability of the annotation and prevents unnecessary state changes that would result only from reshuffling.
	slices.SortFunc(observedNodeStatuses, func(a, b scyllav1alpha1.ObservedNodeStatus) int {
		return strings.Compare(a.HostID, b.HostID)
	})

	return &internalapi.NodeStatusReport{
		ObservedNodes: observedNodeStatuses,
	}
}

func scyllaClientNodeStatusToScyllaV1Alpha1NodeStatus(status scyllaclient.NodeStatus) scyllav1alpha1.NodeStatus {
	switch status {
	case scyllaclient.NodeStatusUp:
		return scyllav1alpha1.NodeStatusUp

	default:
		return scyllav1alpha1.NodeStatusDown

	}
}
