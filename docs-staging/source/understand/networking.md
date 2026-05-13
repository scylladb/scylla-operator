# Networking

This page explains how ScyllaDB Operator exposes ScyllaDB nodes on the network, how broadcast addresses work, and how IP family selection affects the cluster.

## Services created by the Operator

For every ScyllaDB cluster the Operator manages two kinds of Kubernetes Service:

| Service | Count | Purpose |
|---------|-------|---------|
| **Identity** (named `<cluster>-client`) | One per datacenter | Stable DNS names for the StatefulSet. A regular `ClusterIP` Service shared by all racks' StatefulSets as their `serviceName`, so that each pod gets a predictable DNS record (`<pod>.<cluster>-client.<ns>.svc.cluster.local`). Also serves as a client entry point that load-balances across all ScyllaDB pods. |
| **Member** | One per pod | A dedicated Service whose type is controlled by `exposeOptions.nodeService.type`. Carries the address that is broadcast to clients or other nodes. |

The member Service uses a selector that matches exactly one pod, giving every ScyllaDB node its own stable network identity independent of the pod IP lifecycle.

## Node Service types

The `exposeOptions.nodeService.type` field controls what kind of member Service the Operator creates for each ScyllaDB node.

### Headless

Creates a headless Service (`clusterIP: None`). The DNS record for the Service resolves directly to the pod IP. No additional IP address is allocated.

Use Headless when pods broadcast their own IP and no cluster-internal virtual IP is needed — for example, in multi-VPC deployments where pod IPs are routable across VPCs.

### ClusterIP

Creates a standard ClusterIP Service backed by a single pod. The Service receives a virtual IP that is routable only inside the Kubernetes cluster.

This is the **default** for `ScyllaCluster`.

### LoadBalancer

Creates a LoadBalancer Service. On cloud platforms that support external load balancers the Service provisions one, giving each ScyllaDB node an externally reachable address.

Customisations such as restricting a load balancer to the internal network are managed through annotations on the Service. The `annotations` field in `nodeService` is merged into every member Service.

LoadBalancer Services are a superset of ClusterIP Services — every LoadBalancer Service also has a ClusterIP. Additional fields that propagate to member Services:

- `externalTrafficPolicy`
- `internalTrafficPolicy`
- `loadBalancerClass`
- `allocateLoadBalancerNodePorts`

#### Platform-specific annotations

::::{tabs}
:::{group-tab} EKS
```yaml
exposeOptions:
  nodeService:
    type: LoadBalancer
    annotations:
      service.beta.kubernetes.io/aws-load-balancer-scheme: internal
      service.beta.kubernetes.io/aws-load-balancer-backend-protocol: tcp
```
:::
:::{group-tab} GKE
```yaml
exposeOptions:
  nodeService:
    type: LoadBalancer
    annotations:
      networking.gke.io/load-balancer-type: Internal
```
:::
::::

## Broadcast options

ScyllaDB uses two broadcast addresses:

- **`broadcast_address`** — the address other ScyllaDB nodes use to reach this node (gossip, streaming, repair).
- **`broadcast_rpc_address`** — the address CQL clients use to connect to this node (returned during driver discovery).

The Operator lets you configure these independently via `exposeOptions.broadcastOptions.nodes` and `exposeOptions.broadcastOptions.clients`. Separating the two is useful when node-to-node traffic and client traffic travel over different networks for cost, latency, or security reasons.

### Broadcast address types

| Type | Address source | When to use |
|------|---------------|-------------|
| **`PodIP`** | Pod's IP from `status.podIP` (or a specific entry in `status.podIPs` selected by IP family) | Pod IPs are routable from wherever the consumer lives — for example, within a VPC or across peered VPCs. |
| **`ServiceClusterIP`** | `spec.clusterIP` of the member Service | Consumers are inside the same Kubernetes cluster. Requires `nodeService.type` to be `ClusterIP` or `LoadBalancer`. |
| **`ServiceLoadBalancerIngress`** | First entry in `status.loadBalancer.ingress` (IP or hostname) of the member Service | Consumers are outside the Kubernetes cluster and reach nodes through a load balancer. Requires `nodeService.type` to be `LoadBalancer`. |

### How broadcast addresses reach ScyllaDB

1. The controller passes `--nodes-broadcast-address-type` and `--clients-broadcast-address-type` flags to both the **sidecar** and **ignition** containers.
2. At startup, the sidecar resolves the flag value to a concrete IP address by inspecting the pod status or the member Service.
3. The resolved addresses are passed to the ScyllaDB binary as `--broadcast-address` and `--broadcast-rpc-address` command-line arguments.

The ScyllaDB process itself listens on all interfaces (`0.0.0.0` for IPv4, `::` for IPv6). The broadcast addresses only control what address is advertised to other nodes and clients.

## Defaults

A `ScyllaCluster` created without explicit `exposeOptions` uses the following defaults:

| Field | Default |
|-------|---------|
| `nodeService.type` | `ClusterIP` |
| `broadcastOptions.nodes.type` | `ServiceClusterIP` |
| `broadcastOptions.clients.type` | `ServiceClusterIP` |

For multi-datacenter deployments using multiple `ScyllaCluster` resources connected via `externalSeeds`, you typically need to override these defaults to use `Headless` / `PodIP` so that pod IPs are broadcast directly. See the [Multi-VPC / multi-datacenter](#multi-vpc-multi-datacenter) scenario below.

:::{note}
`exposeOptions` on `ScyllaCluster` are **immutable** — they cannot be changed after the cluster is created.
:::

## Common deployment scenarios

### In-cluster only (default)

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

Clients and nodes communicate through cluster-internal virtual IPs. The cluster is not reachable from outside Kubernetes.

### Pod IPs within a VPC

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

Nodes talk to each other via ClusterIP (they share a Kubernetes cluster). Clients in the same VPC connect directly using pod IPs, which are routable within the VPC.

(multi-vpc-multi-datacenter)=
### Multi-VPC / multi-datacenter

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

Two or more Kubernetes clusters in separate VPCs with VPC peering or a shared VPC. Each datacenter is a separate `ScyllaCluster` resource connected to the others via `externalSeeds`. Pod IPs are routable across VPCs, so both nodes and clients use them directly. No virtual IP is needed, hence Headless.

See [Deploy a multi-datacenter cluster](../deploy-scylladb/deploy-multi-dc-cluster.md) for the step-by-step procedure and [Set up multi-DC infrastructure](../install-operator/set-up-multi-dc-infrastructure.md) for the platform networking setup.

### External access via load balancers

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

Each node gets a load balancer with an externally reachable address. Clients outside the cluster connect through the load balancer address. Nodes still communicate within the cluster via ClusterIP.

## IP families and dual-stack

The Operator supports IPv4, IPv6, and dual-stack networking. See [IPv6](../deploy-scylladb/set-up-networking/ipv6/index.md) for configuration details.

## Per-rack overrides

Each rack can override the labels and annotations on its member Services via `exposeOptions` at the rack level. This does not change the Service type — only metadata. It is useful for applying rack-specific load balancer annotations (for example, targeting a particular availability zone).

## No NetworkPolicy by default

The Operator does **not** create Kubernetes `NetworkPolicy` resources. If your environment requires network-level isolation, you must create NetworkPolicy objects separately. See [Security](security.md) for related considerations.

## Related pages

- [Understand](index.md) — component diagram and CRD summary.
- [Security](security.md) — TLS certificates and authentication.
- [Sidecar](sidecar.md) — how the sidecar resolves broadcast addresses at startup.
- [StatefulSets and racks](statefulsets-and-racks.md) — StatefulSet naming and stable network identity.
