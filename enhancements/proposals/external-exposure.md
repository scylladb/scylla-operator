# External exposure

## Summary

Currently, it is impossible to expose ScyllaClusters externally, including exposing clusters in cloud-managed SDN networks.
This document proposes an enhancement providing different levels of cluster exposure as well as an interface to do so.

## Motivation

In Kubernetes, we expect the Pods to be replaced rather regularly. Therefore, there is no guarantee that a Scylla node will be available at the same IP, were it to be replaced. Because Scylla uses IP (value of `broadcast-rpc-address`) as a node identity for its peers, we can't simply allow for the IP to change regularly.
Currently, each Scylla node has a corresponding service of type ClusterIP, with the ClusterIP used as the node's identity. Both `broadcast-address` and `broadcast-rpc-address` are currently set to the corresponding ClusterIP. Such service is also currently used for a range of node operations, such as node replacement and switching to maintenance mode.
ClusterIP is however only a virtual address, for which kube-proxy, running on each node, installs iptables rules, which capture any traffic targeting it and redirects it to one of the backend endpoints. Hence, it is not possible to expose ClusterIPs outside a cluster while running kube-proxy.

During our efforts to come up with a way of providing multi-datacenter support, this issue has proven to be virtually impossible to circumvent.
In the meantime, many users reported a need for exposing the cluster for external clients. 
This enhancement proposal arose as a remedy to these issues.

### Goals

* Introduce different exposure policies
* Introduce an interface for exposing the cluster

### Non-Goals

* Provide multi-datacenter support. This enhancement is only a prerequisite.

## Proposal

### ScyllaNodeIdentity

To overcome the limitations set by using a service as a source of identity, a new CRD is introduced. The CRD, hereafter referred to as ScyllaNodeIdentity, replaces the existing service as a source of Scylla node's identity and an object used as an endpoint for performing node operations.
ScyllaNodeIdentity will specify an IP field and will act as a source of truth for Scylla nodes to configure the attributes that define its identity.
ScyllaNodeIdentity will be controlled by the operator and created for each Scylla node with cluster's creation. 

The source of the IP address would depend on cluster's network configuration, and would transiently result in the following Scylla configuration:


| Config                                         |   broadcast_rpc_address   |     broadcast_address     | listen_address | rpc_address |
|------------------------------------------------|:-------------------------:|:-------------------------:|:--------------:|:-----------:|
| ExposurePolicy = Internal<br>NodePort disabled |           PodIP           |           PodIP           |    0.0.0.0     |   0.0.0.0   |
| ExposurePolicy = Internal<br>NodePort enabled  |          HostIP           |          HostIP           |    0.0.0.0     |   0.0.0.0   |
| ExposurePolicy = External                      | ExternalIP (LoadBalancer) | ExternalIP (LoadBalancer) |    0.0.0.0     |   0.0.0.0   |


TODO replace-address-first-boot

### ScyllaCluster

#### Network

ScyllaCluster's Network specification will be extended as follows:

```go
type Network struct {
	... TODO hostnetworking
	
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

### User Stories

TODO explain in more detail? paste yamls instead of descriptions?

#### Connecting from inside the cluster 

A user only wants to be able to reach Scylla nodes from inside the Kubernetes cluster. 
They configure `exposurePolicy` as `internal`. They are able to reach the nodes using their PodIPs within the Kubernetes cluster.

#### Connecting from outside the cluster in a VPC-peered network with connectivity over Pod IPs

A user has a VPC-peered network with routable Pod IPs setup with one of the supported cloud providers.
They configure `exposurePolicy` as `internal`. They are able to reach the nodes using their Pod IPs within the VPC-peered network.

#### Connecting from outside the cluster in a VPC-peered network with connectivity over Node IPs

A user has a VPC-peered network with routable Host IPs setup with one of the supported cloud providers.
They configure `exposurePolicy` as `internal` and enable `hostNetworking`. They are able to reach the Scylla nodes using internal IPs of the Nodes they are deployed on within the VPC-peered network.

#### Connecting from outside the public cluster

A user has a public cluster with each node being assigned a static IP. They configure `exposurePolicy` as `internal` and configure `nodePort`.
They are able to reach the Scylla nodes using the public IPs of the nodes they are deployed on and the ports set in `nodePort` configuration.

#### Connecting from outside the private cluster

A user has a private cluster. They configure `exposurePolicy` as `external`. They are able to reach the Scylla nodes using the static IPs of their corresponding LoadBalancers.

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