# Expose ScyllaCluster to external clients

## Summary

The proposed change aims to improve the accessibility of ScyllaDB nodes by extending options in which they are exposed. 
By implementing a switch to specify the type of IP addresses to use, such as Pod IPs or LoadBalancer services, it would
allow reaching the nodes from external clients or other data centers.

## Motivation

The motivation behind the proposed change is to improve the accessibility and reachability of ScyllaDB nodes. Currently,
the Scylla Operator exposes the nodes with a service of type ClusterIP, which limits access to virtual IPs only within
the cluster. This poses difficulties in routing external traffic to the cluster, even if manual load balancers are set
up. To enable communication with nodes from another data center or external clients, it is necessary to publish
different IP addresses.

### Goals

* Expand ScyllaCluster API with configuration options allowing for more advanced networking setups.
* Expand ScyllaCluster API with configuration options allowing to choose which address is broadcasted to clients and nodes.
* Expand ScyllaCluster API with configuration options allowing to choose on which interface nodes will listen to given type of traffic.
* Allow for connecting to ScyllaCluster nodes from external networks.
* Allow for network separation of node-to-node and client-to-node traffic types.

### Non-Goals

* Cover all possible network configurations.
* Support changing exposing options after ScyllaCluster creation.

## Proposal

I propose to extend the ScyllaCluster API with new fields adding a switch which will control how nodes of ScyllaDB
broadcast their IP address to their clients and other nodes and on which interfaces they are listening. 
Depending on the value of these switches, nodes may be accessible from external networks. 
Operator will be responsible for reconciling required resources and preparing the right ScyllaDB 
configuration. Existing procedures relying on node IP identifiers will be aligned to these changes.

### Risks and Mitigations

#### More misconfiguration opportunities

New API allows for user to specify which address is broadcasted to other clients and nodes, and to choose which 
Service is created for every ScyllaCluster node. Service types may not provide address which user set up to broadcast,
for example ClusterIP Service doesn't have external IP.
To prevent from these misconfigurations, Operator validating webhook will be extended with appropriate validation logic.

#### ScyllaDB minimal version requirement

From ScyllaDB Open Source 5.2 and ScyllaDB Enterprise 2023.1 nodes can be replaced using HostID. It provides a stable node
identifier throughout entire node lifetime. Previous replace logic required IP of a node being replaced. New types of 
exposure provides ephemeral IP addresses, so it's hard to tell which IP should be used.
As Operator is not able to replace other types of Services than ClusterIP when Scylla version is not high enough, we will restrict setting
different NodeServiceType if Scylla version in ScyllaCluster Status is not high enough via webhook validation. 
Users wanting to expose their clusters will have to upgrade first. 

## Design Details

### API changes

Existing ExposeOptions - `scyllacluster.spec.exposeOptions` - will be extended with new fields:

```go

type PodIPSourceType string

const (
    // StatusPodIPSource specifies that the PodIP is taken from Pod.Status.PodIP
    StatusPodIPSource PodIPSourceType = "Status"
    
    // InterfacePodIPSource specifies the PodIP is taken from the interface.
    InterfacePodIPSource PodIPSourceType = "Interface"
)

type PodIPInterfaceOptions struct {
    // interfaceName specifies interface name within a Pod from which address is taken from.
    InterfaceName string `json:"interfaceName,omitempty"`
}

// PodIPAddressOptions hold options related to Pod IP address.
type PodIPAddressOptions struct {
    // source specifies source of the Pod IP.
    // +kubebuilder:default:="Status"
    Source PodIPSourceType `json:"source"`
    
    // interfaceOptions hold options for Interface SourceType.
    // +optional
    InterfaceOptions *PodIPInterfaceOptions `json:"interfaceOptions,omitempty"`
}

type BroadcastAddressType string

const (
    // PodIPBroadcastAddressType selects the IP address from Pod.
    PodIPBroadcastAddressType BroadcastAddressType = "PodIP"
    
    // ServiceClusterIPBroadcastAddressType selects the IP address from Service.spec.ClusterIP
    ServiceClusterIPBroadcastAddressType BroadcastAddressType = "ServiceClusterIP"
    
    // ServiceLoadBalancerIngressIPBroadcastAddressType selects the IP address from Service.status.ingress[0].ip
    ServiceLoadBalancerIngressIPBroadcastAddressType BroadcastAddressType = "ServiceLoadBalancerIngressIP"
)

// BroadcastOptions hold options related to broadcasted address.
type BroadcastOptions struct {
    // type of broadcasted address.
    Type BroadcastAddressType `json:"type"`
    
    // podIP hold options related to Pod IP address.
    // +optional
    PodIP *PodIPAddressOptions `json:"podIP,omitempty"`
}

// NodeBroadcastOptions hold options related to addresses broadcasted by ScyllaDB node.
type NodeBroadcastOptions struct {
    // nodes specifies options related to the address that is broadcasted to other nodes for node to node for communication.
    // This field controls the `broadcast_address` value in ScyllaDB config.
    // +kubebuilder:default:={type:"ServiceClusterIP"}
    Nodes *BroadcastOptions `json:"nodes"`
    
    // clients specifies options related to the address that is broadcasted to client for communication with ScyllaDB nodes.
    // This field controls the `broadcast_rpc_address` value in ScyllaDB config.
    // +kubebuilder:default:={type:"ServiceClusterIP"}
    Clients *BroadcastOptions `json:"clients"`
}

type ListenAddressType string

const (
    // AnyListenAddressType specifies that listen socket will be bound to any local interface.
    AnyListenAddressType ListenAddressType = "Any"
    
    // PodIPListenAddressType specifies that listen socket will be bound to Pod IP address.
    PodIPListenAddressType ListenAddressType = "PodIP"
)

// ListenOptions hold options related to listening socket of ScyllaDB node.
type ListenOptions struct {
    // type of listen address.
    // +kubebuilder:default:="Any"
    Type ListenAddressType `json:"type"`
    
    // podIP hold options related to PodIP address.
    // +optional
    PodIP *PodIPAddressOptions `json:"podIP,omitempty"`
}

// NodeListenOptions hold options related to listening on interfaces.
type NodeListenOptions struct {
    // nodes specifies options related to the address that is bound to socket used for communication with other nodes.
    // This field controls the `listen_address` value in ScyllaDB config.
    // +kubebuilder:default:={type:"Any"}
    Nodes ListenOptions `json:"nodes"`
    
    // clients specifies options related to the address that is bound to socket used for communication with clients.
    // This field controls the `rpc_address` value in ScyllaDB config.
    // +kubebuilder:default:={type:"Any"}
    Clients ListenOptions `json:"clients"`
}

type NodeServiceType string

const (
    // HeadlessServiceType means nodes will be exposed via Headless Service.
    HeadlessServiceType NodeServiceType = "Headless"
    
    // ClusterIPServiceType means nodes will be exposed via ClusterIP Service.
    ClusterIPServiceType NodeServiceType = "ClusterIP"
    
    // LoadBalancerServiceType means nodes will be exposed via LoadBalancer Service.
    LoadBalancerServiceType NodeServiceType = "LoadBalancer"
)

type NodeServiceTemplate struct {
    // annotations is a custom key value map merged with managed node Service annotations.
    // +optional
    Annotations map[string]string `json:"annotations,omitempty"`
    
    // type is the Kubernetes Service type.
    // +kubebuilder:validation:Required
    Type NodeServiceType `json:"type"`
    
    // externalTrafficPolicy controls value of service.spec.externalTrafficPolicy of each node Service.
    // Check Kubernetes corev1.Service documentation about semantic of this field.
    // +optional
    ExternalTrafficPolicy *corev1.ServiceExternalTrafficPolicy `json:"externalTrafficPolicy,omitempty"`
    
    // allocateLoadBalancerNodePorts controls value of service.spec.allocateLoadBalancerNodePorts of each node Service.
    // Check Kubernetes corev1.Service documentation about semantic of this field.
    // +optional
    AllocateLoadBalancerNodePorts *bool `json:"allocateLoadBalancerNodePorts,omitempty"`
    
    // loadBalancerClass controls value of service.spec.loadBalancerClass of each node Service.
    // Check Kubernetes corev1.Service documentation about semantic of this field.
    // +optional
    LoadBalancerClass *string `json:"loadBalancerClass,omitempty"`
    
    // internalTrafficPolicy controls value of service.spec.internalTrafficPolicy of each node Service.
    // Check Kubernetes corev1.Service documentation about semantic of this field.
    // +optional
    InternalTrafficPolicy *corev1.ServiceInternalTrafficPolicy `json:"internalTrafficPolicy,omitempty"`
}

// ExposeOptions hold options related to exposing ScyllaCluster backends.
type ExposeOptions struct {
    // nodeService controls properties of Service dedicated for each ScyllaCluster node.
    // +kubebuilder:default:={type:"ClusterIP"}
    NodeService *NodeServiceTemplate `json:"nodeService,omitempty"`
    
    // BroadcastOptions defines how ScyllaDB node publishes its IP address to other nodes and clients
    BroadcastOptions *NodeBroadcastOptions `json:"broadcastOptions,omitempty"`
    
    // ListenOptions defined how ScyllaDB node listens on interfaces.
    ListenOptions *NodeListenOptions `json:"listenOptions,omitempty"`
}
```

Scylla Operator will configure dedicated node Services based on user provided `NodeServiceTemplate`.  
ScyllaDB configuration parameters will be mapped accordingly:
* `broadcast_address` - from `BroadcastOptions.Nodes`.
* `broadcast_rpc_address` - from `BroadcastOptions.Clients`.
* `rpc_address` - from `ListenOptions.Clients`.
* `listen_address` - from `ListenOptions.Nodes`.

### Selected Network Configuration Examples

#### Private Kubernetes

This is the current setup type (Scylla Operator <= 1.10), only Pods deployed in the same K8s cluster can connect to
Scylla Pods. Connecting from outside the Kubernetes cluster is impossible, as Service ClusterIP are virtual IPs which
are not routable outside the cluster.

Scylla Operator is configured to create ClusterIP Services.

![Private-K8s](private-k8s.svg "Private K8s")

##### ScyllaDB configuration

```yaml
broadcast_address: ClusterIP
broadcast_rpc_address: ClusterIP
```

---

#### Private VPC

In Private VPC type, Scylla Pods broadcast their ephemeral PodIP both to clients and other nodes.
Connectivity of Pod IP subnet depends on the network configuration. In major Cloud Providers (GKE, EKS), 
these are natively routable, meaning it allows for connecting to ScyllaCluster from the same VPC, or any other peered VPC.

Scylla Operator is configured to create Headless Services.

![Private-VPC](private-vpc.svg "Private VPC")

##### ScyllaDB configuration

```yaml
broadcast_address: PodIP
broadcast_rpc_address: PodIP
```

---

#### Private VPC + Public LB

Scylla Nodes talk to each other over private IP, but all the clients, no matter where they are deployed, need to talk
through the public Internet. MultiDC is possible over private IPs.
Big disadvantage of this type is the cost associated with the necessity of going through the public Internet, and the
cost of Load Balancer Services required for each Node in the ScyllaCluster.

Scylla Operator is configured to create LoadBalancer Services.

![Private-VPC+Public-LB](private-vpc-public-lb.svg "Private VPC + Public LB")

##### ScyllaDB configuration

```yaml
broadcast_address: PodIP
broadcast_rpc_address: LoadBalancer External IP
```

---

#### Private VPC + Public SNI Proxy

This type allows for all client connection origins, can establish MultiDC connection using private IPs, while being cost-efficient.
VMs deployed in the same VPC can either go directly to the pods, or use SNI proxy with a specialized driver.
Customers either coming from public Internet or Private Link go through SNI proxy.
Scylla Operator is configured to create Headless Services.

![Private-VPC+Public-SNI-Proxy](private-vpc-public-sni-proxy.svg "Private VPC + Public SNI Proxy")

##### ScyllaDB configuration

```yaml
broadcast_address: PodIP
broadcast_rpc_address: PodIP
```

---

#### Multicloud

In this multicloud example we rely on any solution providing inter-VPC connectivity. Scylla's nodes talk over VPN spread across 
cloud VPCs with each other, where clients talk through SNI proxy either for traffic originating from the same VPC, 
Private Link connection or public Internet.

Scylla Operator is configured to create Headless Services.

![Multicloud](multicloud.svg "Multicloud")

##### ScyllaDB configuration

```yaml
broadcast_address: PodIP
broadcast_rpc_address: PodIP
```

---

#### Multiple interfaces in Pods

Current Scylla Operator binds ScyllaDB listening sockets to every interface available in the Pod. In case when user 
would want to separate node-to-node traffic from client-to-node traffic on separate network interfaces, we would need to 
configure ScyllaDB to bind given socket type only on specific interface. In multi-interface setups, PodIP is ambiguous, 
so new `ListenOptions` allow users to specify from which interface we will take address to configure `listen_address`, and 
`rpc_address`.  
In this proposal we won't support interfaces having multiple addresses, but API is prepared to be extended with additional 
options specifying desired address family, scope or index.

---

#### Scylla support

##### Ephemeral IP

From ScyllaDB Open Source 5.1 and ScyllaDB Enterprise 2022.1 nodes detect conflict when any node boot ups with the same
Host ID and different IP, nodes acknowledge IP change, system tables are updated accordingly so drivers should pick up
the change as well.

#### Node Procedures

Procedures relying on node identity by their IP address need to be aligned to changes.

##### Replace node

Moved to separate proposal - [#1311](https://github.com/scylladb/scylla-operator/tree/master/enhancements/proposals/1311-replace-nodes-using-hostid)

##### Readiness probe

Scylla Pod readiness probe checks whether node status is UN. Cluster information it gathers is represented by following
structure:

```go
type NodeStatusInfo struct {
    Datacenter string
    HostID     string
    Addr       string
    Status     NodeStatus
    State      NodeState
}
```

Scylla always returns it as a slice of statuses of all nodes in the cluster. Currently, the readiness probe searches for
matching Addr which is the IP address of a node.
Instead of looking for IP in the structure, we will search for HostID which is constant across node lifetime and also
available from the Scylla API.

##### SANs in TLS certificates

Scylla Operator depending on ExposeOptions will add an address that is broadcasted to clients to node certificate SANs.

##### Seeds

Current initial contact point list (seeds) is a single ClusterIP of a member Service of a Pod passed readiness check
and sorted by their creationTimestamp.
Logic will be changed to use the broadcasted address for the first node in the cluster - ScyllaDB detects that node 
broadcast address is the same as seed, so it form fresh cluster. 
For the rest of the nodes, we will list all other Pods that passed readiness check, sort them by their creationTimestamp,
select the first one and use ClusterIP from the Service if it's available. If not, seed will be a PodIP from the status. 

## Test Plan

E2E tests validating whether correct Service resources are created based on the provided `NodeServiceTemplate`, and
whether Scylla is set up to broadcast right addresses based on `BroadcastOptions`, and listening options based on `ListenOptions`.
Connectivity will be covered by QA as it relies on cloud specific network settings.

## Upgrade / Downgrade Strategy

Regular Operator upgrade/downgrade strategy.

## Version Skew Strategy

#### Old Operator being a leader:
It doesn't support any other Service type than ClusterIP. In case there would be a ScyllaCluster with different Service
type and Service options, it would be updated to ClusterIP Service without these options. ScyllaDB configuration would
be updated to broadcast ClusterIP addresses. Existing clients and other ScyllaDB nodes connected from outside would lose
connections as nodes iteratively picking out the new config would start broadcasting private IP addresses. Clients and
other ScyllaDB nodes deployed in the same Kubernetes clusters would switch into these private IPs.
In case when replace would be in progress, it would use Host ID identifiers for new types of cluster exposure - we restrict 
ScyllaDB version for new exposure types to be able to support replace using Host ID - it would be continued by the older 
Operator. In case of older ScyllaDB version, ScyllaCluster would use ClusterIP exposure which supports replacing using 
IP address.

#### Updated Operator being a leader:
It supports new Service types as well as options. Default values of new fields - CRD and operator code - are in sync 
with the only supported Service type, so existing ScyllaClusters wouldn't need any change as they are already deployed 
with ClusterIP Services, and they broadcast ClusterIP both to servers and clients.

Readiness check is done via sidecar so there's no skew.
