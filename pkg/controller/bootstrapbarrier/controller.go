// Copyright (C) 2025 ScyllaDB

package bootstrapbarrier

import (
	"context"
	"fmt"
	"time"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	scyllav1alpha1informers "github.com/scylladb/scylla-operator/pkg/client/scylla/informers/externalversions/scylla/v1alpha1"
	scyllav1alpha1listers "github.com/scylladb/scylla-operator/pkg/client/scylla/listers/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllertools"
	"github.com/scylladb/scylla-operator/pkg/naming"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	apimachineryutilsets "k8s.io/apimachinery/pkg/util/sets"
	corev1informers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/klog/v2"
)

type Controller struct {
	*controllertools.Observer

	namespace                            string
	serviceName                          string
	selectorLabelValue                   string
	singleReportAllowNonReportingHostIDs bool
	bootstrapPreconditionCh              chan struct{}

	serviceLister                             corev1listers.ServiceLister
	scyllaDBDatacenterNodesStatusReportLister scyllav1alpha1listers.ScyllaDBDatacenterNodesStatusReportLister
}

func NewController(
	namespace string,
	serviceName string,
	selectorLabelValue string,
	singleReportAllowNonReportingHostIDs bool,
	bootstrapPreconditionCh chan struct{},
	kubeClient kubernetes.Interface,
	serviceInformer corev1informers.ServiceInformer,
	scyllaDBDatacenterNodesStatusReportInformer scyllav1alpha1informers.ScyllaDBDatacenterNodesStatusReportInformer,
) (*Controller, error) {
	c := &Controller{
		namespace:                            namespace,
		serviceName:                          serviceName,
		selectorLabelValue:                   selectorLabelValue,
		singleReportAllowNonReportingHostIDs: singleReportAllowNonReportingHostIDs,
		bootstrapPreconditionCh:              bootstrapPreconditionCh,
		serviceLister:                        serviceInformer.Lister(),
		scyllaDBDatacenterNodesStatusReportLister: scyllaDBDatacenterNodesStatusReportInformer.Lister(),
	}

	observer := controllertools.NewObserver(
		"scylladb-bootstrap-barrier",
		kubeClient.CoreV1().Events(corev1.NamespaceAll),
		c.Sync,
	)

	serviceHandler, err := serviceInformer.Informer().AddEventHandler(observer.GetGenericHandlers())
	if err != nil {
		return nil, fmt.Errorf("can't add Service event handler: %w", err)
	}
	observer.AddCachesToSync(serviceHandler.HasSynced)

	scyllaDBDatacenterNodesStatusReportHandler, err := scyllaDBDatacenterNodesStatusReportInformer.Informer().AddEventHandler(observer.GetGenericHandlers())
	if err != nil {
		return nil, fmt.Errorf("can't add ScyllaDBDatacenterNodesStatusReport event handler: %w", err)
	}
	observer.AddCachesToSync(scyllaDBDatacenterNodesStatusReportHandler.HasSynced)

	c.Observer = observer

	return c, nil
}

func (c *Controller) Sync(ctx context.Context) error {
	startTime := time.Now()
	klog.V(4).InfoS("Started syncing observer", "Name", c.Observer.Name(), "startTime", startTime)
	defer func() {
		klog.V(4).InfoS("Finished syncing observer", "Name", c.Observer.Name(), "duration", time.Since(startTime))
	}()

	svc, err := c.serviceLister.Services(c.namespace).Get(c.serviceName)
	if err != nil {
		return fmt.Errorf("can't get service %q: %w", c.serviceName, err)
	}

	scyllaDBDatacenterNodesStatusReports, err := c.scyllaDBDatacenterNodesStatusReportLister.ScyllaDBDatacenterNodesStatusReports(c.namespace).List(labels.SelectorFromSet(labels.Set{
		naming.ScyllaDBDatacenterNodesStatusReportSelectorLabel: c.selectorLabelValue,
	}))
	if err != nil {
		return fmt.Errorf("can't list ScyllaDBDatacenterNodesStatusReports: %w", err)
	}

	proceedWithBootstrap, err := shouldProceedWithBootstrap(svc, scyllaDBDatacenterNodesStatusReports, isBootstrapPreconditionSatisfiedFn(c.singleReportAllowNonReportingHostIDs))
	if err != nil {
		return fmt.Errorf("can't determine if bootstrap should proceed: %w", err)
	}
	if proceedWithBootstrap {
		close(c.bootstrapPreconditionCh)
	}

	return nil
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
