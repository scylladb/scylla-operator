# ScyllaDBDatacenterNodesStatusReport (scylla.scylladb.com/v1alpha1)

**APIVersion**: scylla.scylladb.com/v1alpha1
<br/>
**Kind**: ScyllaDBDatacenterNodesStatusReport
<br/>
**PluralName**: scylladbdatacenternodesstatusreports
<br/>
**SingularName**: scylladbdatacenternodesstatusreport
<br/>
**Scope**: Namespaced
<br/>
**ListKind**: ScyllaDBDatacenterNodesStatusReportList
<br/>
**Served**: true
<br/>
**Storage**: true
<br/>

## Description

ScyllaDBDatacenterNodesStatusReport serves as an internal interface for propagating and aggregating node statuses observed by nodes in a ScyllaDB datacenter.
It is used by ScyllaDB Operator for coordinating operations such as topology changes and is not intended for direct user interaction.

## Specification

| Property                                                                                    | Type           | Description                                                                                                                                                                                                                                                                                                                                                                                           |
|---------------------------------------------------------------------------------------------|----------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| apiVersion                                                                                  | string         | APIVersion defines the versioned schema of this representation of an object. Servers should convert recognized schemas to the latest internal value, and may reject unrecognized values. More info: [https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources](https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources)    |
| datacenterName                                                                              | string         | DatacenterName is the name of the reported ScyllaDB datacenter.                                                                                                                                                                                                                                                                                                                                       |
| kind                                                                                        | string         | Kind is a string value representing the REST resource this object represents. Servers may infer this from the endpoint the client submits requests to. Cannot be updated. In CamelCase. More info: [https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds](https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds) |
| [metadata](#api-scylla-scylladb-com-scylladbdatacenternodesstatusreports-v1alpha1-metadata) | object         |                                                                                                                                                                                                                                                                                                                                                                                                       |
| [racks](#api-scylla-scylladb-com-scylladbdatacenternodesstatusreports-v1alpha1-racks)       | array (object) | Racks holds the list of rack status reports.                                                                                                                                                                                                                                                                                                                                                          |

<a id="api-scylla-scylladb-com-scylladbdatacenternodesstatusreports-v1alpha1-metadata"></a>

### .metadata

#### Description

#### Type

object

<a id="api-scylla-scylladb-com-scylladbdatacenternodesstatusreports-v1alpha1-racks"></a>

### .racks[]

#### Description

#### Type

object

| Property                                                                                    | Type           | Description                                                                      |
|---------------------------------------------------------------------------------------------|----------------|----------------------------------------------------------------------------------|
| name                                                                                        | string         | Name is the name of the rack.                                                    |
| [nodes](#api-scylla-scylladb-com-scylladbdatacenternodesstatusreports-v1alpha1-racks-nodes) | array (object) | Nodes holds the list of node status reports collected from nodes from this rack. |

<a id="api-scylla-scylladb-com-scylladbdatacenternodesstatusreports-v1alpha1-racks-nodes"></a>

### .racks[].nodes[]

#### Description

#### Type

object

| Property                                                                                                          | Type           | Description                                                                     |
|-------------------------------------------------------------------------------------------------------------------|----------------|---------------------------------------------------------------------------------|
| hostID                                                                                                            | string         | HostID is the ScyllaDB reporter node’s host ID.                                 |
| [observedNodes](#api-scylla-scylladb-com-scylladbdatacenternodesstatusreports-v1alpha1-racks-nodes-observednodes) | array (object) | ObservedNodes holds the list of node statuses as observed by the reporter node. |
| ordinal                                                                                                           | integer        | Ordinal is the ordinal of the reporter node within its rack.                    |

<a id="api-scylla-scylladb-com-scylladbdatacenternodesstatusreports-v1alpha1-racks-nodes-observednodes"></a>

### .racks[].nodes[].observedNodes[]

#### Description

#### Type

object

| Property   | Type   | Description                            |
|------------|--------|----------------------------------------|
| hostID     | string | HostID is the ScyllaDB node’s host ID. |
| status     | string | Status is the status of the node.      |
