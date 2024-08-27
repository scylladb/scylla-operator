# Exposing ScyllaCluster

This document explains how ScyllaDB Operator exposes ScyllaClusters in different network setups.
A ScyllaCluster can be exposed in various network configurations, independently to clients and nodes.

:::{note}
ScyllaClusters can be only exposed when the ScyllaDB version used version is `>=2023.1` ScyllaDB Enterprise or `>=5.2` ScyllaDB Open Source.
:::

## Expose Options

:::{note}
`exposeOptions` are immutable, they cannot be changed after ScyllaCluster is created.
:::

`exposeOptions` specifies configuration options for exposing ScyllaCluster's. 
A ScyllaCluster created without any `exposeOptions` is equivalent to the following:

```yaml
apiVersion: scylla.scylladb.com/v1
kind: ScyllaCluster
spec:
  exposeOptions:
    nodeService:
     type: ClusterIP
    broadcastOptions:
      clients:
        type: ServiceClusterIP
      nodes:
        type: ServiceClusterIP
```

The following sections cover what every field controls and what the configuration options are.

### Node Service Template

`nodeService` serves as a template for a node-dedicated Service managed by the Scylla Operator for each node within a ScyllaCluster. 
The properties of the Services depend on the selected type. 
Additionally, there's an option to define custom annotations, incorporated into each node's Service,
which might be useful for further tweaking the Service properties or related objects.

#### Headless Type

For `Headless` type, Scylla Operator creates a Headless Service with a selector pointing to the particular node in the ScyllaCluster.
Such Service doesn't provide any additional IP addresses, and the internal DNS record resolves to the PodIP of a node.

This type of Service is useful when ScyllaCluster nodes broadcast PodIPs to clients and other nodes.

Example:
```yaml
apiVersion: scylla.scylladb.com/v1
kind: ScyllaCluster
spec:
  exposeOptions:
    nodeService:
     type: Headless
```

#### ClusterIP Type

For `ClusterIP` type, Scylla Operator creates a ClusterIP Service backed by a specific node in the ScyllaCluster.

These IP addresses are only routable within the same Kubernetes cluster, so it's a good fit, if you don't want to expose them to other networks.

Example:
```yaml
apiVersion: scylla.scylladb.com/v1
kind: ScyllaCluster
spec:
  exposeOptions:
    nodeService:
     type: ClusterIP
```

#### LoadBalancer Type

For the `LoadBalancer` type, Scylla Operator generates a LoadBalancer Service that directs traffic to a specific node within the ScyllaCluster. 
On platforms with support for external load balancers, this Service provisions one. 
The accessibility of this load balancer's address depends on the platform and any customizations made; in some cases it may be reachable from the internal network or public Internet.

Customizations are usually managed via Service annotations, key-value pairs provided in `annotations` field are merged into each Service object.
LoadBalancer Services should be configured to pass through entire traffic.  
For example, to expose LoadBalancer only to internal network use the following annotations:

::::{tab-set}
:::{tab-item} EKS
```yaml
apiVersion: scylla.scylladb.com/v1
kind: ScyllaCluster
spec:
  exposeOptions:
    nodeService:
     type: LoadBalancer
     annotations:
       service.beta.kubernetes.io/aws-load-balancer-scheme: internal
       service.beta.kubernetes.io/aws-load-balancer-backend-protocol: tcp
```
:::
:::{tab-item} GKE
```yaml
apiVersion: scylla.scylladb.com/v1
kind: ScyllaCluster
spec:
  exposeOptions:
    nodeService:
     type: LoadBalancer
     annotations:
       networking.gke.io/load-balancer-type: Internal
```
:::
::::

Check platform-specific documentation regarding LoadBalancer configuration to learn more about available options.

LoadBalancer Service is a superset of ClusterIP Service, implying that each LoadBalancer Service also contains an allocated ClusterIP. 
They can be configured using the following fields, which propagate to every node Service:
* externalTrafficPolicy
* internalTrafficPolicy
* loadBalancerClass
* allocateLoadBalancerNodePorts

Check [Kubernetes Service documentation](https://kubernetes.io/docs/concepts/services-networking/service) to learn more about these options.

Example:
```yaml
apiVersion: scylla.scylladb.com/v1
kind: ScyllaCluster
spec:
  exposeOptions:
    nodeService:
     type: LoadBalancer
     loadBalancerClass: my-custom-load-balancer-class
```

---

### Broadcast Options

Broadcast options control what is the source of the address being broadcasted to clients and nodes.
It's configured independently for clients and nodes because you may want to expose these two types of traffic on different networks.
Using different networks can help manage costs, reliability, latency, security policies or other metrics you care about.

#### PodIP Type

Address broadcasted to clients/nodes is taken from Pod.
By default, the address is taken from Pod's `status.PodIP` field.
Because a Pod can use multiple address, you may want to provide source options by specifying `podIP.source`.

Example:
```yaml
apiVersion: scylla.scylladb.com/v1
kind: ScyllaCluster
spec:
  exposeOptions:
    broadcastOptions:
       clients:
         type: PodIP
         podIP:
           source: Status
```

#### ServiceClusterIP Type

Address broadcasted to clients or nodes is taken from `spec.ClusterIP` field of a node's dedicated Service.

In order to configure it, the `nodeService` template must specify a Service having a ClusterIP assigned.

Example:
```yaml
apiVersion: scylla.scylladb.com/v1
kind: ScyllaCluster
spec:
  exposeOptions:
    broadcastOptions:
       clients:
         type: ServiceClusterIP
```

#### ServiceLoadBalancerIngress Type

Address broadcasted to clients/nodes is taken from the node dedicated Service, from `status.ingress[0].ipAddress` or `status.ingress[0].hostname` field.

In order to configure it, the `nodeService` template must specify the LoadBalancer Service.

Example:
```yaml
apiVersion: scylla.scylladb.com/v1
kind: ScyllaCluster
spec:
  exposeOptions:
    broadcastOptions:
       clients:
         type: ServiceLoadBalancerIngress
         podIP:
           source: Status
```

## Deployment Examples

The following section contains several specific examples of various network scenarios and explains how nodes and clients communicate with one another.
### In-cluster only

ScyllaCluster definition:
```yaml
apiVersion: scylla.scylladb.com/v1
kind: ScyllaCluster
spec:
  exposeOptions:
    nodeService:
      type: ClusterIP
    broadcastOptions:
      clients:
        type: ServiceClusterIP
      nodes:
        type: ServiceClusterIP
```

Both client and nodes are deployed within the same Kubernetes cluster. 
They talk through ClusterIP addresses taken from the Service.
Because ClusterIP Services are only routable within the same Kubernetes cluster, this cluster won't be reachable from outside.

![ClusterIPs](static/exposing/clusterip.svg)

### In-cluster node-to-node, VPC clients-to-nodes

ScyllaCluster definition:
```yaml
apiVersion: scylla.scylladb.com/v1
kind: ScyllaCluster
spec:
  exposeOptions:
    nodeService:
      type: ClusterIP
    broadcastOptions:
      clients:
        type: PodIP
      nodes:
        type: ServiceClusterIP
```

In this scenario, we assume that the Pod IP subnet is routable within a VPC. 
Clients within the VPC network can communicate directly with ScyllaCluster nodes using PodIPs.
Nodes communicate with each other exclusively within the same Kubernetes cluster.

![PodIPs](static/exposing/podips.svg)

### Multi VPC

ScyllaCluster definition:
```yaml
apiVersion: scylla.scylladb.com/v1
kind: ScyllaCluster
spec:
  exposeOptions:
    nodeService:
      type: Headless
    broadcastOptions:
      clients:
        type: PodIP
      nodes:
        type: PodIP
```

In this scenario, we set up two separate Kubernetes clusters in distinct VPCs.
These VPCs are interconnected to facilitate inter-VPC connectivity. 
We operate on the assumption that the Pod IP subnet is routable within each VPC.

Both ScyllaClusters use the same `exposeOptions`, nodes broadcast their Pod IP addresses, enabling them to establish connections with one another.
****Check other documentation pages to know how to connect two ScyllaClusters into one logical cluster.

Clients, whether deployed within the same Kubernetes cluster or within a VPC, have the capability to reach nodes using their Pod IPs.
Since there is no requirement for any address other than the Pod IP, the `Headless` service type is sufficient.

![MultiVPC](static/exposing/multivpc.svg)

### Internet

ScyllaCluster definition:
```yaml
apiVersion: scylla.scylladb.com/v1
kind: ScyllaCluster
spec:
  exposeOptions:
    nodeService:
      type: LoadBalancer
    broadcastOptions:
      clients:
        type: ServiceLoadBalancerIngress
      nodes:
        type: ClusterIP 
```

We assume that a Kubernetes cluster has been deployed in a cloud provider environment that supports external load balancers. 
By specifying the LoadBalancer type in the nodeService template, the Scylla Operator generates a dedicated LB Service for each node. 
The cloud provider then establishes an external load balancer with an internet-accessible address. 
ScyllaDB nodes broadcast this external address to clients, enabling drivers to connect and discover other nodes. 
Since all ScyllaDB nodes reside within the same Kubernetes cluster, there is no need to route traffic through the internet. 
Consequently, the nodes are configured to communicate via ClusterIP, which is also accessible within LoadBalancer Services.

![Internet](static/exposing/loadbalancer.svg)

---

Other more complex scenarios can be built upon these simple ones.
