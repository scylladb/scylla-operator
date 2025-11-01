# Bootstrap Synchronisation

## Summary

TODO

## Motivation

TODO

### Goals

- Introduce a mechanism ensuring all nodes in the cluster see all nodes as alive (UP) as a precondition for bootstrapping new nodes.

### Non-Goals

- Introduce a mechanism ensuring preconditions are met before bootstrapping a node as part of a node replacement procedure.
- Address transient quorum/availability loss issues during voluntary and involuntary restarts.

## Proposal

I propose to introduce a mechanism ensuring all nodes in the cluster see all nodes as alive (UP) as a precondition for bootstrapping new nodes.

For this measure, I propose introducing a new API object: `ScyllaDBStatusReport.scylla.scylladb.com/v1alpha1`.
The purpose of this object is to reflect the status of a ScyllaDB cluster as seen by a collection of ScyllaDB nodes.
The Operator will be extended to create and manage `ScyllaDBStatusReport` objects for each ScyllaDB cluster it manages - for both single-datacenter and multi-datacenter clusters.

ScyllaDB Pods will be extended with an init container determining whether the corresponding ScyllaDB node has already been bootstrapped.
This is done to avoid blocking the startup progression of restarting nodes that have already been bootstrapped, as stated in the non-goals section.

Additionally, ScyllaDB Pods will be extended with an init container implementing a barrier.
If the node has not been bootstrapped yet, the barrier will block progressing with the ScyllaDB container startup until the `ScyllaDBStatusReport` object indicates that all nodes in the cluster are seen as alive (UP) by all nodes in the cluster.
Nodes bootstrapping to replace an existing node will be exempt from this requirement, as stated in the non-goals section.

Both init containers will be put behind a feature gate, which will be disabled by default.

### User Stories

TODO

### Notes/Constraints/Caveats

TODO

### Risks and Mitigations

#### Determining if a node has already been bootstrapped relies on internal implementation of ScyllaDB state

The init container determining whether the corresponding ScyllaDB node has already been bootstrapped relies on the internal implementation of ScyllaDB state, specifically the presence and values in the `bootstrapped` column within `system.local` table, as well as the interface of the `scylla sstable` tool itself.
The `query` command is only present in ScyllaDB versions >=2025.2.

To mitigate this, we will:
1. implement a regression test extracting the information from dynamically created ScyllaDB data volume, in which we will expect the command to succeed and return the expected output,
2. document the ScyllaDB version requirement for this feature.

#### Bootstrap synchronisation can't be enabled in manually configured multi-datacenter clusters

Due to the lack of a mechanism to collect, aggregate and propagate the status reports between nodes in different datacenters in a manually configured multi-datacenter cluster, bootstrap synchronisation barrier will not be able to correctly assess the precondition for bootstrapping new nodes.

TODO: mitigation?
TODO: reevaluate this in a PoC

## Design Details

### Reflecting and propagating the status of ScyllaDB nodes

#### Definition of `ScyllaDBStatusReport.scylla.scylladb.com/v1alpha1`

The `ScyllaDBStatusReport.scylla.scylladb.com/v1alpha1` API is defined as follows:

```go
type NodeStatus string

const (
	NodeStatusUp   NodeStatus = "UP"
	NodeStatusDown NodeStatus = "DOWN"
)

type ObservedNodeStatusReport struct {
	//  HostID is the ScyllaDB node's host ID.
	HostID string `json:"hostID"`

	// Status is the status of the node.
	// +kubebuilder:validation:Enum="Up";"Down"
	Status NodeStatus `json:"status"`
}

// NodeStatusReport holds a report for a single node.
type NodeStatusReport struct {
	// HostID is the ScyllaDB node's host ID.
	HostID string `json:"hostID"`

	// ObservedNodes holds the list of nodes as observed by this node.
	// +optional
	ObservedNodes []ObservedNodeStatusReport `json:"observedNodes,omitempty"`
}

// DatacenterStatusReport holds a report for a single datacenter.
type DatacenterStatusReport struct {
	// Name is the name of the datacenter.
	Name string `json:"name"`

	// Nodes holds the list of node reports from this datacenter.
	// +optional
	Nodes []NodeStatusReport `json:"nodes,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ScyllaDBStatusReport struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Datacenters holds the list of datacenter reports.
	// +optional
	Datacenters []DatacenterStatusReport `json:"datacenters,omitempty"`
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ScyllaDBStatusReportList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	
	Items           []ScyllaDBStatusReport `json:"items"`
}
```

The CRD does not follow the conventional Kubernetes pattern of `spec` and `status` fields, as the object's semantics do not adhere to the concept of desired and observed states.

#### Reporting the status of a ScyllaDB node

The Operator sidecar, running as part of a ScyllaDB Pod, will be extended to periodically report the status of its corresponding ScyllaDB node by annotating the Pod with a `internal.scylla.scylladb.com/scylladb-node-status-report` annotation.
The annotation's value will be a JSON-encoded object of the following type:
```go
type NodeStatusReportAnnotation struct {
    // NodeStatusReport holds the status report for this node.
    NodeStatusReport *NodeStatusReport `json:"nodeStatusReport,omitempty"`

    // Error holds an error message if the report could not be built.
    Error *string `json:"error,omitempty"`
}
```
The error field will be set if the sidecar is unable to build the `NodeStatusReport` object for any reason, e.g. if the local ScyllaDB node is not yet ready to serve API requests.
The sidecar builds the `NodeStatusReport` object by invoking the local ScyllaDB node API operations: `GET /storage_service/host_id` to retrieve the mapping of endpoint to host ID for all nodes in the cluster that own tokens, and `GET /gossiper/endpoint/live/` to retrieve the addresses of all live endpoints as seen by the local node.
All endpoints not returned by invoking the `GET /gossiper/endpoint/live/` operation are considered down.

The sidecar will extract the statuses and update the annotation at a configurable interval, defaulting to 5 seconds.

#### Status report aggregation

The Operator's ScyllaDBDatacenter controller will be extended to collect the status reports from all ScyllaDB Pods constituting the datacenter.
The controller will extract the `internal.scylla.scylladb.com/scylladb-node-status-report` annotation from each Pod.
The successfully decoded reports will be aggregated to create or update the `ScyllaDBStatusReport` object corresponding to the ScyllaDBDatacenter. 
TODO: naming convention

TODO: sequence diagram

The Operator's ScyllaDBCluster controller will be extended to aggregate the status reports for all of its children ScyllaDBDatacenter objects.
The aggregated status report will be backpropagated to the remote dataplane clusters by creating or updating separate `ScyllaDBStatusReport` objects in each remote cluster.
TODO: naming convention

ScyllaDBDatacenters created as children of ScyllaDBClusters will be annotated with a `internal.scylla-operator.scylladb.com/scylladb-status-report-override-ref` annotation configured to the name of the `ScyllaDBStatusReport` object created by the ScyllaDBCluster controller.

### Bootstrap synchronization

#### Determining if a node has already been bootstrapped

ScyllaDB Pods will be extended with an init container determining whether the corresponding ScyllaDB node has already been bootstrapped.
The init container implementing the bootstrap status check will use the `scylla sstable` tool within the ScyllaDB container image to query the data volume for values in the `bootstrapped` column within `system.local` table by running the following command:

```bash
/usr/bin/scylla sstable query --system-schema --scylla-data-dir=/var/lib/scylla/data --output-format=json --keyspace=system --table=local --query="SELECT bootstrapped FROM scylla_sstable.local" /var/lib/scylla/data/system/local-*/*-Data.db
```

Any errors encountered while running the command will be ignored and the node will be considered **not bootstrapped**.
The standard output will be shared with the subsequent init container, implementing the barrier, through a file created in a shared EmptyDir volume.

The init container implementing the barrier will attempt decoding the shared file as into a struct of a type defined as follows:
```go
type BootstrappedQueryResult []struct {
    Bootstrapped string `json:"bootstrapped"`
}
```

If the decoded array is non-empty and the `Bootstrapped` field of the first element of the array is equal to `COMPLETED`, the node will be considered **bootstrapped**.
Otherwise, the node will be considered **not bootstrapped**.

#### TODO: passing reports to the init container

#### Deciding if ScyllaDB container startup can proceed

If the node is determined to be **bootstrapped**, the init container implementing the barrier will exit immediately, allowing the ScyllaDB container to start.
Otherwise, if the node is determined to be **not bootstrapped**, but the node is replacing an existing node, as indicated by the presence of the `scylla/replace` label on the corresponding Service, the init container will exit immediately, allowing the ScyllaDB container to start.
Otherwise, the init container will block progressing with the ScyllaDB container startup until it determines that the required precondition is met.

The following code describes the algorithm determining whether the required precondition (all nodes in the cluster are seen as alive by all nodes in the cluster) is met:
```go
func isBootstrapPreconditionMet(statusReport *scyllav1alpha1.ScyllaDBStatusReport) bool {
	// hostIDs is a set of all IDs that appeared in the status report.
	hostIDs := map[string]bool{}
	// hostIDToNodeStatusesMap maps a node's host ID to a map of observed nodes' host IDs to their statuses as observed by the node.
	hostIDToNodeStatusesMap := map[string]map[string]bool{}

	for _, dc := range statusReport.Datacenters {
		for _, node := range dc.Nodes {
			hostIDs[node.HostID] = true

			observedNodeHostIDToNodeStatusesMap := map[string]bool{}
			for _, observedNode := range node.ObservedNodes {
				hostIDs[observedNode.HostID] = true
				observedNodeHostIDToNodeStatusesMap[observedNode.HostID] = observedNode.Status == scyllav1alpha1.NodeStatusUp
			}

			hostIDToNodeStatusesMap[node.HostID] = observedNodeHostIDToNodeStatusesMap
		}
	}

	for hostID := range hostIDs {
		nodeStatuses, ok := hostIDToNodeStatusesMap[hostID]
		if !ok {
			// This node did not report a status, but it must be a part of a cluster as it appeared in the status report.
			// We don't know what it thinks about other nodes, so we must assume the worst.
			return false
		}

		for otherHostID := range hostIDs {
			if !nodeStatuses[otherHostID] {
				// The other node is either missing from this node's report or is considered DOWN.
				return false
			}
		}
	}

	return true
}
```

The init container will watch for changes in the referenced `ScyllaDBStatusReport` object and will re-evaluate the precondition each time the object is updated.

#### Feature gate

Both init containers will be put behind an alpha `BootstrapSynchronisation` feature gate, which will be disabled by default.
TODO: promotion plan?

### Test Plan

TODO

### Upgrade / Downgrade Strategy

TODO

### Version Skew Strategy

TODO

## Implementation History

TODO

## Drawbacks

TODO

## Alternatives

TODO

