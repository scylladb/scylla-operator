# Networking

This page explains how ScyllaDB Operator exposes ScyllaDB nodes on the network, how broadcast addresses work, and how IP family selection affects the cluster.

## Services created by the Operator

For every ScyllaDB cluster the Operator manages two kinds of Kubernetes Service:

| Service | Count | Purpose |
|---------|-------|---------|
| **Identity (headless)** | One per rack | Stable DNS names for the StatefulSet. Always `ClusterIP` with `clusterIP: None`. Used as `serviceName` in the StatefulSet so that each pod gets a predictable DNS record (`<pod>.<svc>.<ns>.svc.cluster.local`). |
| **Member** | One per pod | A dedicated Service whose type is controlled by `exposeOptions.nodeService.type`. Carries the address that is broadcast to clients or other nodes. |

The member Service uses a selector that matches exactly one pod, giving every ScyllaDB node its own stable network identity independent of the pod IP lifecycle.

### Ports exposed by the member Service

| Port | Name | Number |
|------|------|--------|
| CQL (native transport) | `cql` | 9042 |
| CQL over TLS | `cql-ssl` | 9142 |
| CQL shard-aware | `cql-shard-aware` | 19042 |
| CQL shard-aware over TLS | `cql-ssl-shard-aware` | 19142 |
| Inter-node (storage port) | `inter-node` | 7000 |
| Inter-node over TLS | `inter-node-ssl` | 7001 |
| JMX | `jmx` | 7199 |
| Prometheus (ScyllaDB) | `prometheus` | 9180 |
| Manager Agent API | `agent-api` | 10001 |
| Agent Prometheus | `agent-prometheus` | 5090 |
| Node Exporter | `node-exporter` | 9100 |
| Thrift | `thrift` | 9160 |
| Alternator (HTTPS) | `alternator-tls` | 8043 |
| Alternator (HTTP) | `alternator` | 8000 |

Alternator ports are included only when `alternator` is configured in the cluster spec.

## Node Service types

The `exposeOptions.nodeService.type` field controls what kind of member Service the Operator creates for each ScyllaDB node.

### Headless

Creates a headless Service (`clusterIP: None`). The DNS record for the Service resolves directly to the pod IP. No additional IP address is allocated.

Use Headless when pods broadcast their own IP and no cluster-internal virtual IP is needed — for example, in multi-VPC deployments where pod IPs are routable across VPCs.

### ClusterIP

Creates a standard ClusterIP Service backed by a single pod. The Service receives a virtual IP that is routable only inside the Kubernetes cluster.

This is the **default for `ScyllaCluster`** and `ScyllaDBDatacenter`.

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

## Defaults by API version

The default networking posture differs between the single-datacenter and multi-datacenter APIs:

| Field | `ScyllaCluster` (v1) | `ScyllaDBDatacenter` (v1alpha1) | `ScyllaDBCluster` (v1alpha1) |
|-------|----------------------|--------------------------------|------------------------------|
| `nodeService.type` | `ClusterIP` | `ClusterIP` | `Headless` |
| `broadcastOptions.nodes.type` | `ServiceClusterIP` | `ServiceClusterIP` | `PodIP` |
| `broadcastOptions.clients.type` | `ServiceClusterIP` | `ServiceClusterIP` | `PodIP` |

`ScyllaDBCluster` defaults to `Headless` / `PodIP` because multi-datacenter clusters span multiple Kubernetes clusters where pod IPs (routed across VPCs) are the natural addressing scheme.

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

Two Kubernetes clusters in separate VPCs with VPC peering. Pod IPs are routable across VPCs, so both nodes and clients use them directly. No virtual IP is needed, hence Headless.

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

## CQL Ingress

`ScyllaCluster` supports an optional CQL Ingress that routes CQL-over-TLS traffic (port 9142) through a Kubernetes Ingress resource. When `exposeOptions.cql.ingress` is configured, the Operator creates:

- One Ingress routing to the identity Service (any-node discovery).
- One Ingress per member Service (node-specific connections using the host ID as a subdomain).

The Ingress uses `dnsDomains` from the cluster spec to build host rules. This option is useful when a layer-7 load balancer is preferred over per-node LoadBalancer Services.

## IP families and dual-stack

The Operator supports IPv4, IPv6, and dual-stack networking. IP family settings control both Kubernetes Service addressing and ScyllaDB's internal protocol.

### Configuration fields

| Field | Location | Effect |
|-------|----------|--------|
| `ipFamilies` | `spec.network.ipFamilies` (v1) or `spec.ipFamilies` (v1alpha1) | Ordered list of IP families. The **first** entry determines which protocol ScyllaDB uses internally. |
| `ipFamilyPolicy` | Same locations | `SingleStack`, `PreferDualStack`, or `RequireDualStack`. Defaults to `SingleStack`. |
| `dnsPolicy` | `spec.network.dnsPolicy` (v1) or `spec.dnsPolicy` (v1alpha1) | Pod DNS policy. Should be `ClusterFirst` for IPv6. |

### Why the first IP family matters

ScyllaDB uses a single IP protocol for all internal operations:

- **Gossip** — nodes exchange membership information using one protocol. Mixed protocols break gossip.
- **Data replication** — inter-node connections must use a consistent address family.
- **Token ring** — the consistent hash ring requires uniform addressing across all nodes.

The first entry in `ipFamilies` determines this protocol. The second entry (if present) is used only at the Kubernetes Service level, giving clients the option to connect via either protocol.

### Dual-stack behaviour

"Dual-stack" in this context means Kubernetes Services have both IPv4 and IPv6 addresses. ScyllaDB itself always uses a single protocol — the first IP family.

```yaml
network:
  ipFamilies:
    - IPv4    # ScyllaDB communicates over IPv4
    - IPv6    # Services also get an IPv6 address
  ipFamilyPolicy: PreferDualStack
```

Services receive both an A record (IPv4) and an AAAA record (IPv6), so clients can connect over either protocol. The actual database traffic between nodes uses only the first family.

If the Kubernetes cluster does not support dual-stack but `ipFamilyPolicy` is `PreferDualStack`, Services gracefully fall back to the first IP family only.

### Multi-datacenter IP family consistency

All datacenters in a multi-datacenter cluster **must** use the same primary IP family. Cross-datacenter replication, gossip, and the global token ring require uniform addressing.

```yaml
# Both datacenters must agree on IPv6 as the primary family
network:
  ipFamilies:
    - IPv6
    - IPv4
```

The secondary family can differ at the Service level, but the first entry must match across all datacenters.

### DNS policy for IPv6

When using IPv6, set `dnsPolicy: ClusterFirst` to ensure that Kubernetes cluster DNS (CoreDNS) resolves AAAA records correctly. Without it, pods may fall back to the node's resolver, which might not handle IPv6 service discovery.

## Per-rack overrides

Each rack can override the labels and annotations on its member Services via `exposeOptions` at the rack level. This does not change the Service type — only metadata. It is useful for applying rack-specific load balancer annotations (for example, targeting a particular availability zone).

## No NetworkPolicy by default

The Operator does **not** create Kubernetes `NetworkPolicy` resources. If your environment requires network-level isolation, you must create NetworkPolicy objects separately. See [Security](security.md) for related considerations.

## Related pages

- [Overview](overview.md) — component diagram and CRD summary.
- [Security](security.md) — TLS certificates and authentication.
- [Sidecar](sidecar.md) — how the sidecar resolves broadcast addresses at startup.
- [StatefulSets and racks](statefulsets-and-racks.md) — StatefulSet naming and stable network identity.
