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
	corev1informers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/klog/v2"
)

type Controller struct {
	*controllertools.Observer

	namespace               string
	serviceName             string
	selectorLabelValue      string
	bootstrapPreconditionCh chan struct{}

	serviceLister                             corev1listers.ServiceLister
	scyllaDBDatacenterNodesStatusReportLister scyllav1alpha1listers.ScyllaDBDatacenterNodesStatusReportLister
}

func NewController(
	namespace string,
	serviceName string,
	selectorLabelValue string,
	bootstrapPreconditionCh chan struct{},
	kubeClient kubernetes.Interface,
	serviceInformer corev1informers.ServiceInformer,
	scyllaDBDatacenterNodesStatusReportInformer scyllav1alpha1informers.ScyllaDBDatacenterNodesStatusReportInformer,
) (*Controller, error) {
	c := &Controller{
		namespace:               namespace,
		serviceName:             serviceName,
		selectorLabelValue:      selectorLabelValue,
		bootstrapPreconditionCh: bootstrapPreconditionCh,
		serviceLister:           serviceInformer.Lister(),
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

/*
TODO: encapsulate the logic to unit-test it with mocked listers
The tests will cover the following scenarios:
- basic scenarios with mocked isBootstrapPreconditionSatisfied func
- service is missing required labels
- service has node replacement label set
- service has bootstrap synchronisation override annotation set to "true"
- service has bootstrap synchronisation override annotation set to "false"
- no status reports are listed
- status reports are listed
*/

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

	// TODO: check for bootstrap synchronisation override annotation
	// if set to "true", block regardless of preconditions
	// if set to "false", proceed regardless of preconditions

	svcDC, ok := svc.Labels[naming.DatacenterNameLabel]
	if !ok {
		return fmt.Errorf("service %q is missing label %q", naming.ObjRef(svc), naming.DatacenterNameLabel)
	}

	svcRack, ok := svc.Labels[naming.RackNameLabel]
	if !ok {
		return fmt.Errorf("service %q is missing label %q", naming.ObjRef(svc), naming.RackNameLabel)
	}

	svcOrdinal, err := naming.IndexFromName(svc.Name)
	if err != nil {
		return fmt.Errorf("can't get ordinal from name of service %q: %w", naming.ObjRef(svc), err)
	}

	if _, ok := svc.Labels[naming.ReplacingNodeHostIDLabel]; ok {
		klog.V(2).InfoS("Node is replacing another node, proceeding without verifying the precondition.", "Service", c.serviceName)
		close(c.bootstrapPreconditionCh)
		return nil
	}

	scyllaDBDatacenterNodesStatusReports, err := c.scyllaDBDatacenterNodesStatusReportLister.ScyllaDBDatacenterNodesStatusReports(c.namespace).List(labels.SelectorFromSet(labels.Set{
		naming.ScyllaDBDatacenterNodesStatusReportSelectorLabel: c.selectorLabelValue,
	}))
	if err != nil {
		return fmt.Errorf("can't list ScyllaDBDatacenterNodesStatusReports: %w", err)
	}

	bootstrapPreconditionSatisfied := isBootstrapPreconditionSatisfied(scyllaDBDatacenterNodesStatusReports, svcDC, svcRack, int(svcOrdinal))
	if bootstrapPreconditionSatisfied {
		close(c.bootstrapPreconditionCh)
	}

	return nil
}

func isBootstrapPreconditionSatisfied(scyllaDBDatacenterNodesStatusReports []*scyllav1alpha1.ScyllaDBDatacenterNodesStatusReport, selfDC string, selfRack string, selfOrdinal int) bool {
	klog.V(4).InfoS("Verifying if bootstrap precondition is satisfied.", "Datacenter", selfDC, "Rack", selfRack, "Ordinal", selfOrdinal)

	// allHostIDs is a set of host IDs of all nodes which appeared in the status report, including the reportees.
	allHostIDs := map[string]bool{}
	// reportingHostIDToObservedNodeStatusesMap maps a reporting node's host ID to a map of observed nodes' host IDs to their statuses as observed by the reporting node.
	reportingHostIDToObservedNodeStatusesMap := map[string]map[string]bool{}

	for _, sdcnsr := range scyllaDBDatacenterNodesStatusReports {
		for _, rack := range sdcnsr.Racks {
			for _, node := range rack.Nodes {
				if sdcnsr.DatacenterName == selfDC && rack.Name == selfRack && node.Ordinal == selfOrdinal {
					// Skip self.
					// The node is bootstrapping, so it won't have a report nor a host ID propagated.
					continue
				}

				if node.HostID == nil {
					klog.V(4).InfoS("A required node is missing a host ID, can't proceed with verifying the bootstrap precondition.", "RequiredNodeDatacenter", sdcnsr.DatacenterName, "RequiredNodeRack", rack.Name, "RequiredNodeOrdinal", node.Ordinal)
					return false
				}

				allHostIDs[*node.HostID] = true

				observedNodeHostIDToNodeStatusesMap := map[string]bool{}
				for _, observedNode := range node.ObservedNodes {
					allHostIDs[observedNode.HostID] = true
					observedNodeHostIDToNodeStatusesMap[observedNode.HostID] = observedNode.Status == scyllav1alpha1.NodeStatusUp
				}

				reportingHostIDToObservedNodeStatusesMap[*node.HostID] = observedNodeHostIDToNodeStatusesMap
			}
		}
	}

	// In non-automated multi-datacenter deployments, we expect nodes from external DCs to appear in the status report as reportees only.
	// It is required to check that ALL nodes, not just reporter nodes, are present and UP in each report.
	allowNonReportingHostIDs := false
	if len(scyllaDBDatacenterNodesStatusReports) == 1 && scyllaDBDatacenterNodesStatusReports[0].DatacenterName == selfDC {
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
			if !nodeStatuses[otherHostID] {
				// The other node is either missing from this node's report or is considered DOWN.
				klog.V(4).InfoS("Node's status report is missing another node or considers it DOWN. Bootstrap precondition is not satisfied.", "HostID", hostID, "MissingOrDownHostID", otherHostID)
				return false
			}
		}
	}

	klog.V(4).InfoS("Bootstrap precondition is satisfied, proceeding.", "Datacenter", selfDC, "Rack", selfRack, "Ordinal", selfOrdinal, "ObservedNodes", reportingHostIDToObservedNodeStatusesMap)
	return true
}
