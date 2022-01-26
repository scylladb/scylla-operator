# Multi-datacenter support

## Summary

With the current state of the operator, it is impossible to create a multi-datacenter installation in a simple enough way.
This proposal introduces the changes necessary to make it possible to manually connect multiple clusters across different datacenters.
Additionally, while it is now impossible to expose the cluster externally, the document proposes different levels of exposure and an interface for doing so. TODO

## Motivation

TODO

### Goals

* Provide multi-datacenter support
* Introduce different cluster  exposure types

### Non-Goals

* Automate multi-datacenter setup

## Proposal

### ScyllaNodeIdentity

In Kubernetes, we expect the Pods to be replaced rather regularly. Therefore, there is no guarantee that a Scylla node will be available at the same IP, were it to be replaced. Because Scylla uses IP (value of `broadcast-rpc-address`) as a node identity for its peers, we can't simply allow for the IP to change regularly.
Currently, each Scylla node has a corresponding service of type ClusterIP, with the ClusterIP used as the node's identity. Both `broadcast-address` and `broadcast-rpc-address` are currently set to the corresponding ClusterIP. Such service is also currently used for a range of node operations, such as node replacement and switching to maintenance mode.
ClusterIP is however only a virtual address, so it is not possible to expose it outside a cluster. TODO move to motivations and explain why it's not routable

To overcome these limitations, a new CRD is introduced. The CRD, hereafter referred to as ScyllaNodeIdentity, replaces the existing service as a source of Scylla node's identity and an object used as an endpoint for performing node operations.
ScyllaNodeIdentity will specify an IP field and will act as a source of truth for Scylla nodes to configure the attributes that define its identity.
ScyllaNodeIdentity will be controlled by the operator and created for each Scylla node with cluster's creation. 

The source of the IP address would depend on cluster's network configuration, and would transiently result in the following Scylla configuration:


| Config                                         |   broadcast_rpc_address   |     broadcast_address     | listen_address | rpc_address |
|------------------------------------------------|:-------------------------:|:-------------------------:|:--------------:|:-----------:|
| ExposurePolicy = Internal<br>NodePort disabled |           PodIP           |           PodIP           |    0.0.0.0     |   0.0.0.0   |
| ExposurePolicy = Internal<br>NodePort enabled  |          HostIP           |          HostIP           |    0.0.0.0     |   0.0.0.0   |
| ExposurePolicy = External                      | ExternalIP (LoadBalancer) | ExternalIP (LoadBalancer) |    0.0.0.0     |   0.0.0.0   |


### ScyllaCluster

#### Network

ScyllaCluster's Network specification will be extended as follows:

```go
type Network struct {
	...
	
	ExposurePolicy ExposurePolicy `json:"exposurePolicy,omitempty"`

	NodePort *NodePortConfig `json:"nodePort,omitempty"`
}
```


##### ExposurePolicy

To allow for exposing the cluster externally, either for other clusters or external clients to communicate with, ScyllaCluster API is going to be enhanced with a way of selecting its exposure type.
The selected type, together with the `NodePort` option, will determine Scylla nodes' configuration as well as services created.

ExposurePolicy is an enum defining two options: `Internal` and `External`.

```go
type ExposurePolicy string

const (
	Internal ExposurePolicy = "internal"
	
	External ExposurePolicy = "external"
)
```

`External` will result in creating a service of type LoadBalancer with each Scylla node and setting the IP address of each ScyllaNodeIdentity to its corresponding Pod service's ExternalIP.

`Internal` exposure policy will not result in creating any additional services and will result in setting the IP address of ScyllaNodeIdentity to its Pod's `PodIP` in case NodePort is disabled or `HostIP` otherwise.

##### NodePort

ScyllaCluster will also be enhanced with an option of requesting a NodePort service, in which a single NodePort service will be created for the entire datacenter. Such service routes specified external ports to internal ones on all hosts.

```go
type NodePortConfig struct {
	Native int `json:"native,omitempty"`
	
	Internode int `json:"internode,omitempty"`
}
```

TODO describe ports

#### HostNetworking 

Although it is now possible to enable `HostNetworking`, it doesn't result in exposing Scylla nodes on IPs of their host machines. With the proposed configuration, running the cluster with `Internal` exposure policy and host networking enabled would result in exposing the nodes on its' hosts' IPs. 

##### External seeds

ScyllaCluster will also be enhanced with an option to provide additional seeds, which will allow for connecting multiple clusters.
```go
type ScyllaClusterSpec struct {
    ...
    // +optional
    ExternalSeeds []string `json:"externalSeeds,omitempty`
}
```
External seeds will be validated and later merged with seeds from the cluster while setting up an entrypoint. 


### User Stories

TODO

### Notes/Constraints/Caveats [Optional]

TODO

### Risks and Mitigations

TODO

## Design Details

TODO

### Test Plan

TODO unit tests? + e2e testing whether fields are setup correctly + QA taking care of testing scylla

### Upgrade / Downgrade Strategy

TODO

### Version Skew Strategy

TODO

## Implementation History

- 1.8: enhancement proposal created

## Drawbacks

TODO

## Alternatives

TODO tried vpc peering, tried cilium, explain why clusterIP is a no-go

## Infrastructure Needed [optional]

TODO