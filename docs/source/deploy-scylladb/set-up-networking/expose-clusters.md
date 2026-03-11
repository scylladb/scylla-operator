# Expose ScyllaDB clusters

Configure how ScyllaDB nodes are exposed on the network using `exposeOptions` — choosing the Service type, broadcast address sources, and platform-specific settings.

For the conceptual background on Services, broadcast addresses, and IP families, see [Networking architecture](../understand/networking.md).

## Configure node Services

The `exposeOptions.nodeService.type` field controls the Kubernetes Service type created for each ScyllaDB node.

:::::{tabs}
::::{group-tab} ScyllaCluster

```yaml
apiVersion: scylla.scylladb.com/v1
kind: ScyllaCluster
metadata:
  name: scylla
  namespace: scylla
spec:
  exposeOptions:
    nodeService:
      type: ClusterIP    # ClusterIP (default), Headless, or LoadBalancer
    broadcastOptions:
      nodes:
        type: ServiceClusterIP
      clients:
        type: ServiceClusterIP
  datacenter:
    name: us-east-1
    racks:
      - name: us-east-1a
        members: 3
        storage:
          capacity: 500Gi
        resources:
          limits:
            cpu: 4
            memory: 8Gi
```

:::{warning}
`exposeOptions` on ScyllaCluster are **immutable** — they cannot be changed after the cluster is created.
Plan your networking topology before deploying the cluster.
:::
::::
::::{group-tab} ScyllaDBDatacenter

```yaml
apiVersion: scylla.scylladb.com/v1alpha1
kind: ScyllaDBDatacenter
metadata:
  name: scylla
  namespace: scylla
spec:
  exposeOptions:
    nodeService:
      type: ClusterIP
    broadcastOptions:
      nodes:
        type: ServiceClusterIP
      clients:
        type: ServiceClusterIP
  rackTemplate:
    nodes: 3
    scyllaDB:
      storage:
        capacity: 500Gi
    resources:
      limits:
        cpu: 4
        memory: 8Gi
  racks:
    - name: us-east-1a
```
::::
::::{group-tab} ScyllaDBCluster

```yaml
apiVersion: scylla.scylladb.com/v1alpha1
kind: ScyllaDBCluster
metadata:
  name: scylla
  namespace: scylla
spec:
  exposeOptions:
    nodeService:
      type: Headless     # Headless is the default for ScyllaDBCluster
    broadcastOptions:
      nodes:
        type: PodIP      # PodIP is the default for ScyllaDBCluster
      clients:
        type: PodIP
  datacenterTemplate:
    rackTemplate:
      nodes: 3
      scyllaDB:
        storage:
          capacity: 500Gi
      resources:
        limits:
          cpu: 4
          memory: 8Gi
    racks:
      - name: a
  datacenters:
    - name: us-east-1
      remoteKubernetesClusterName: us-east-1
```

:::{caution}
ScyllaDBCluster is designed for use across multiple Kubernetes clusters.
ClusterIP Services are typically only routable within a single Kubernetes cluster.
If you change the default from Headless to ClusterIP, ensure your CNI allows external ClusterIP connectivity, or use LoadBalancer instead.
:::
::::
:::::

## Deployment scenarios

### In-cluster only (default for ScyllaCluster)

Clients and nodes communicate through cluster-internal virtual IPs.
The cluster is not reachable from outside Kubernetes.

```yaml
exposeOptions:
  nodeService:
    type: ClusterIP
  broadcastOptions:
    clients:
      type: ServiceClusterIP
    nodes:
      type: ServiceClusterIP
```

### VPC-internal clients with pod IPs

Nodes use ClusterIP for inter-node traffic.
Clients in the same VPC connect directly using routable pod IPs.

```yaml
exposeOptions:
  nodeService:
    type: ClusterIP
  broadcastOptions:
    clients:
      type: PodIP
    nodes:
      type: ServiceClusterIP
```

### Multi-VPC / multi-datacenter

Pod IPs are routable across peered VPCs.
No virtual IP is needed.

```yaml
exposeOptions:
  nodeService:
    type: Headless
  broadcastOptions:
    clients:
      type: PodIP
    nodes:
      type: PodIP
```

### External access via load balancers

Each node gets an externally reachable load balancer address.
Nodes communicate within the cluster via ClusterIP.

```yaml
exposeOptions:
  nodeService:
    type: LoadBalancer
  broadcastOptions:
    clients:
      type: ServiceLoadBalancerIngress
    nodes:
      type: ServiceClusterIP
```

## Configure load balancer annotations

When using `LoadBalancer` node Services, use annotations to control platform-specific behaviour such as internal-only load balancers.

:::::{tabs}
::::{group-tab} EKS (AWS)

```yaml
exposeOptions:
  nodeService:
    type: LoadBalancer
    annotations:
      service.beta.kubernetes.io/aws-load-balancer-scheme: internal
      service.beta.kubernetes.io/aws-load-balancer-backend-protocol: tcp
  broadcastOptions:
    clients:
      type: ServiceLoadBalancerIngress
    nodes:
      type: ServiceClusterIP
```
::::
::::{group-tab} GKE (Google Cloud)

```yaml
exposeOptions:
  nodeService:
    type: LoadBalancer
    annotations:
      networking.gke.io/load-balancer-type: Internal
  broadcastOptions:
    clients:
      type: ServiceLoadBalancerIngress
    nodes:
      type: ServiceClusterIP
```
::::
:::::

Additional LoadBalancer fields that propagate to member Services:

| Field | Description |
|---|---|
| `externalTrafficPolicy` | Controls whether traffic is routed node-local or cluster-wide. Use `Local` to preserve client source IPs. |
| `internalTrafficPolicy` | Controls internal traffic routing. |
| `loadBalancerClass` | Specifies the load balancer implementation to use. |
| `allocateLoadBalancerNodePorts` | Controls whether NodePorts are allocated for the LoadBalancer. |

## Per-rack Service overrides

Each rack can add labels and annotations to its member Services without changing the Service type.
This is useful for rack-specific load balancer settings, such as targeting a particular availability zone.

:::::{tabs}
::::{group-tab} ScyllaCluster

```yaml
spec:
  datacenter:
    racks:
      - name: us-east-1a
        exposeOptions:
          nodeService:
            labels:
              topology.kubernetes.io/zone: us-east-1a
            annotations:
              service.beta.kubernetes.io/aws-load-balancer-subnets: subnet-abc123
```
::::
::::{group-tab} ScyllaDBDatacenter

```yaml
spec:
  racks:
    - name: us-east-1a
      exposeOptions:
        nodeService:
          labels:
            topology.kubernetes.io/zone: us-east-1a
          annotations:
            service.beta.kubernetes.io/aws-load-balancer-subnets: subnet-abc123
```
::::
:::::

## Configure CQL Ingress

ScyllaCluster supports optional CQL Ingress that routes CQL-over-TLS traffic (port 9142) through a Kubernetes Ingress resource.
This is useful when a layer-7 load balancer is preferred over per-node LoadBalancer Services.

```yaml
apiVersion: scylla.scylladb.com/v1
kind: ScyllaCluster
metadata:
  name: scylla
  namespace: scylla
spec:
  dnsDomains:
    - cql.scylla.example.com
  exposeOptions:
    cql:
      ingress:
        ingressClassName: nginx
  datacenter:
    name: us-east-1
    racks:
      - name: us-east-1a
        members: 3
        storage:
          capacity: 500Gi
        resources:
          limits:
            cpu: 4
            memory: 8Gi
```

When `cql.ingress` is configured, the Operator creates:

- One Ingress routing to the identity Service (any-node discovery).
- One Ingress per member Service (node-specific connections using the host ID as a subdomain).

The Ingress host rules use the domains specified in `dnsDomains`.

## Verify the configuration

After applying the cluster spec, verify the member Services:

```bash
kubectl -n scylla get svc -l scylla/cluster=scylla
```

Check the broadcast addresses by examining the ScyllaDB configuration:

```bash
kubectl -n scylla exec -it scylla-us-east-1-us-east-1a-0 -c scylla -- \
  cat /etc/scylla/scylla.yaml | grep broadcast
```

## Defaults by API version

| Field | `ScyllaCluster` (v1) | `ScyllaDBDatacenter` (v1alpha1) | `ScyllaDBCluster` (v1alpha1) |
|-------|----------------------|--------------------------------|------------------------------|
| `nodeService.type` | `ClusterIP` | `ClusterIP` | `Headless` |
| `broadcastOptions.nodes.type` | `ServiceClusterIP` | `ServiceClusterIP` | `PodIP` |
| `broadcastOptions.clients.type` | `ServiceClusterIP` | `ServiceClusterIP` | `PodIP` |

## Related pages

- [Networking architecture](../understand/networking.md) — conceptual overview of Services, broadcast addresses, IP families, and dual-stack
- [Connecting to ScyllaDB from inside a Kubernetes cluster](../connect-your-app/inside-kubernetes.md) — using member Service DNS or ClusterIP to connect
- [Connecting to ScyllaDB from outside a Kubernetes cluster](../connect-your-app/outside-kubernetes.md) — using LoadBalancer addresses or CQL Ingress
