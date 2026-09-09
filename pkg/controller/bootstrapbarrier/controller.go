// Copyright (C) 2025 ScyllaDB

package bootstrapbarrier

import (
	"context"
	"fmt"
	"sync"
	"time"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllertools"
	"github.com/scylladb/scylla-operator/pkg/ctrlclient"
	"github.com/scylladb/scylla-operator/pkg/naming"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	apimachineryutilsets "k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2"
	ctrlbuilder "sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	ctrlmanager "sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	// ControllerName names the controller within controller-runtime: in its logs, metrics and the workqueue.
	ControllerName = "scylladb-bootstrap-barrier"
)

// Controller is an observer: it re-derives from the member's Service and the ScyllaDBDatacenterNodesStatusReports
// whether the node may proceed with its bootstrap, and closes bootstrapPreconditionCh once it may.
type Controller struct {
	namespace                            string
	serviceName                          string
	selectorLabelValue                   string
	singleReportAllowNonReportingHostIDs bool
	bootstrapPreconditionCh              chan struct{}
	closeBootstrapPreconditionCh         sync.Once

	client client.Reader
}

func NewController(
	namespace string,
	serviceName string,
	selectorLabelValue string,
	singleReportAllowNonReportingHostIDs bool,
	bootstrapPreconditionCh chan struct{},
	c client.Reader,
) *Controller {
	return &Controller{
		namespace:                            namespace,
		serviceName:                          serviceName,
		selectorLabelValue:                   selectorLabelValue,
		singleReportAllowNonReportingHostIDs: singleReportAllowNonReportingHostIDs,
		bootstrapPreconditionCh:              bootstrapPreconditionCh,
		client:                               c,
	}
}

// CacheOptions restricts the manager's cache to what the controller watches: the member's Service and the
// ScyllaDBDatacenterNodesStatusReports selected by selectorLabelValue, both in namespace.
func CacheOptions(namespace, serviceName, selectorLabelValue string) cache.Options {
	return cache.Options{
		DefaultNamespaces: map[string]cache.Config{
			namespace: {},
		},
		ByObject: map[client.Object]cache.ByObject{
			&corev1.Service{}: {
				Field: fields.OneTermEqualSelector("metadata.name", serviceName),
			},
			&scyllav1alpha1.ScyllaDBDatacenterNodesStatusReport{}: {
				Label: labels.SelectorFromSet(labels.Set{
					naming.ScyllaDBDatacenterNodesStatusReportSelectorLabel: selectorLabelValue,
				}),
			},
		},
	}
}

// SetupWithManager registers the controller with the manager. Every event of the watched kinds re-runs the sync.
func (c *Controller) SetupWithManager(mgr ctrlmanager.Manager, options controller.Options) error {
	return ctrlbuilder.ControllerManagedBy(mgr).
		Named(ControllerName).
		Watches(&corev1.Service{}, controllertools.EnqueueSingleton(ControllerName)).
		Watches(&scyllav1alpha1.ScyllaDBDatacenterNodesStatusReport{}, controllertools.EnqueueSingleton(ControllerName)).
		WithOptions(options).
		Complete(c)
}

func (c *Controller) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	startTime := time.Now()
	klog.V(4).InfoS("Started syncing observer", "Name", ControllerName, "startTime", startTime)
	defer func() {
		klog.V(4).InfoS("Finished syncing observer", "Name", ControllerName, "duration", time.Since(startTime))
	}()

	svc, err := ctrlclient.Get[corev1.Service](ctx, c.client, c.namespace, c.serviceName)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("can't get service %q: %w", c.serviceName, err)
	}

	scyllaDBDatacenterNodesStatusReports, err := ctrlclient.List[scyllav1alpha1.ScyllaDBDatacenterNodesStatusReport](ctx, c.client, c.namespace, labels.SelectorFromSet(labels.Set{
		naming.ScyllaDBDatacenterNodesStatusReportSelectorLabel: c.selectorLabelValue,
	}))
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("can't list ScyllaDBDatacenterNodesStatusReports: %w", err)
	}

	proceedWithBootstrap, err := shouldProceedWithBootstrap(svc, scyllaDBDatacenterNodesStatusReports, isBootstrapPreconditionSatisfiedFn(c.singleReportAllowNonReportingHostIDs))
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("can't determine if bootstrap should proceed: %w", err)
	}
	if proceedWithBootstrap {
		// The sync can run again after the channel was closed, e.g. on a watch event racing the shutdown.
		c.closeBootstrapPreconditionCh.Do(func() {
			close(c.bootstrapPreconditionCh)
		})
	}

	return reconcile.Result{}, nil
}

func shouldProceedWithBootstrap(
	svc *corev1.Service,
	scyllaDBDatacenterNodesStatusReports []*scyllav1alpha1.ScyllaDBDatacenterNodesStatusReport,
	isBoostrapPreconditionSatisfied func([]*scyllav1alpha1.ScyllaDBDatacenterNodesStatusReport, string, string, int) bool,
) (bool, error) {
	svcDC, ok := svc.Labels[naming.DatacenterNameLabel]
	if !ok {
		return false, fmt.Errorf("service %q is missing label %q", naming.ObjRef(svc), naming.DatacenterNameLabel)
	}

	svcRack, ok := svc.Labels[naming.RackNameLabel]
	if !ok {
		return false, fmt.Errorf("service %q is missing label %q", naming.ObjRef(svc), naming.RackNameLabel)
	}

	svcOrdinal, err := naming.IndexFromName(svc.Name)
	if err != nil {
		return false, fmt.Errorf("can't get ordinal from name of service %q: %w", naming.ObjRef(svc), err)
	}

	if forceProceedToBootstrapString, ok := svc.Annotations[naming.ForceProceedToBootstrapAnnotation]; ok {
		switch forceProceedToBootstrapString {
		case "true":
			klog.V(4).InfoS(`Force proceed to bootstrap annotation is set to "true", proceeding without verifying the precondition.`, "Service", naming.ObjRef(svc))
			return true, nil

		default:
			return false, fmt.Errorf("service %q has an unsupported value for annotation %q: %q", naming.ObjRef(svc), naming.ForceProceedToBootstrapAnnotation, forceProceedToBootstrapString)

		}
	}

	if _, ok := svc.Labels[naming.ReplacingNodeHostIDLabel]; ok {
		klog.V(4).InfoS("Node is replacing another node, proceeding without verifying the precondition.", "Service", naming.ObjRef(svc))
		return true, nil
	}

	bootstrapPreconditionSatisfied := isBoostrapPreconditionSatisfied(scyllaDBDatacenterNodesStatusReports, svcDC, svcRack, int(svcOrdinal))
	return bootstrapPreconditionSatisfied, nil
}

func isBootstrapPreconditionSatisfiedFn(
	singleReportAllowNonReportingHostIDs bool,
) func([]*scyllav1alpha1.ScyllaDBDatacenterNodesStatusReport, string, string, int) bool {
	return func(
		scyllaDBDatacenterNodesStatusReports []*scyllav1alpha1.ScyllaDBDatacenterNodesStatusReport,
		selfDC string,
		selfRack string,
		selfOrdinal int,
	) bool {
		return isBootstrapPreconditionSatisfied(scyllaDBDatacenterNodesStatusReports, selfDC, selfRack, selfOrdinal, singleReportAllowNonReportingHostIDs)
	}
}

func isBootstrapPreconditionSatisfied(scyllaDBDatacenterNodesStatusReports []*scyllav1alpha1.ScyllaDBDatacenterNodesStatusReport, selfDC string, selfRack string, selfOrdinal int, singleReportAllowNonReportingHostIDs bool) bool {
	klog.V(4).InfoS("Verifying if bootstrap precondition is satisfied.", "Datacenter", selfDC, "Rack", selfRack, "Ordinal", selfOrdinal)

	// allHostIDs is a set of host IDs of all nodes which appeared in the status report, including the reportees.
	allHostIDs := apimachineryutilsets.New[string]()
	// reportingHostIDToObservedNodeStatusesMap maps a reporting node's host ID to a map of observed nodes' host IDs to their statuses as observed by the reporting node.
	reportingHostIDToObservedNodeStatusesMap := map[string]map[string]scyllav1alpha1.NodeStatus{}

	// The reports only contain nodes that have already joined the ScyllaDB cluster, so nodes that are still bootstrapping
	// are absent from rack.Nodes by construction. They can still enter allHostIDs through other nodes' ObservedNodes.
	// A host ID that appears there without a corresponding report entry fails the precondition below,
	// so a node joining concurrently may hold up other nodes' bootstrap.
	// That's the conservative direction: nodes that joined but haven't had their own entries included yet don't drop out of the required set.
	for _, report := range scyllaDBDatacenterNodesStatusReports {
		for _, rack := range report.Racks {
			for _, node := range rack.Nodes {
				if report.DatacenterName == selfDC && rack.Name == selfRack && node.Ordinal == selfOrdinal {
					// Skip self.
					// The node is bootstrapping, so it won't have a report nor a host ID propagated.
					continue
				}

				if node.HostID == nil {
					klog.V(4).InfoS("A required node is missing a host ID, can't proceed with verifying the bootstrap precondition.", "RequiredNodeDatacenter", report.DatacenterName, "RequiredNodeRack", rack.Name, "RequiredNodeOrdinal", node.Ordinal)
					return false
				}

				allHostIDs.Insert(*node.HostID)

				observedNodeHostIDToNodeStatusesMap := map[string]scyllav1alpha1.NodeStatus{}
				for _, observedNode := range node.ObservedNodes {
					allHostIDs.Insert(observedNode.HostID)

					observedNodeHostIDToNodeStatusesMap[observedNode.HostID] = observedNode.Status
				}

				reportingHostIDToObservedNodeStatusesMap[*node.HostID] = observedNodeHostIDToNodeStatusesMap
			}
		}
	}

	// In non-automated multi-datacenter deployments, we expect nodes from external DCs to appear in the status report as reportees only.
	// It is required to check that ALL nodes, not just reporter nodes, are present and UP in each report.
	allowNonReportingHostIDs := false
	if singleReportAllowNonReportingHostIDs &&
		len(scyllaDBDatacenterNodesStatusReports) == 1 &&
		scyllaDBDatacenterNodesStatusReports[0].DatacenterName == selfDC {
		allowNonReportingHostIDs = true
	}

	for hostID := range allHostIDs {
		nodeStatuses, ok := reportingHostIDToObservedNodeStatusesMap[hostID]
		if !ok {
			if allowNonReportingHostIDs {
				// In non-automated multi-datacenter deployments, we expect nodes from external DCs to appear in the status report as reportees only.
				// Users are expected to manually ensure the cross-DC precondition is satisfied.
				// In every other case, we expect all nodes which appeared in the status report to also have reported their own status.
				klog.V(4).InfoS("Non-required node's status report is missing. Skipping.", "HostID", hostID)
				continue
			}

			// The node's status report is missing.
			// We don't know what it thinks about other nodes, so we must assume the worst.
			klog.V(4).InfoS("Required node's status report is missing. Bootstrap precondition is not satisfied.", "HostID", hostID)
			return false
		}

		for otherHostID := range allHostIDs {
			otherNodeStatus, hasOtherNodeStatus := nodeStatuses[otherHostID]
			if !hasOtherNodeStatus {
				// The other node is missing from this node's report.
				klog.V(4).InfoS("Node's status report is missing another node. Bootstrap precondition is not satisfied.", "HostID", hostID, "MissingHostID", otherHostID)
				return false
			}

			if otherNodeStatus != scyllav1alpha1.NodeStatusUp {
				// The other node is considered DOWN by this node.
				klog.V(4).InfoS("Node's status report considers another node DOWN. Bootstrap precondition is not satisfied.", "HostID", hostID, "OtherHostID", otherHostID, "OtherNodeStatus", otherNodeStatus)
				return false
			}
		}
	}

	klog.V(4).InfoS("Bootstrap precondition is satisfied, proceeding.", "Datacenter", selfDC, "Rack", selfRack, "Ordinal", selfOrdinal, "ObservedNodes", reportingHostIDToObservedNodeStatusesMap)
	return true
}
