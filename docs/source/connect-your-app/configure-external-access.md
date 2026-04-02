# Configure external access

This page explains how to configure ScyllaDB clusters for access from outside the Kubernetes cluster using the `exposeOptions` API.

:::{note}
`exposeOptions` are immutable — they cannot be changed after the ScyllaDB cluster is created.
:::

## Expose options overview

The `exposeOptions` field controls two things:

1. **Node Service type** — what kind of Kubernetes Service is created for each ScyllaDB node.
2. **Broadcast options** — what address ScyllaDB advertises to clients and other nodes.

### Defaults

```yaml
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

### Node Service types

| Type | Description |
|------|-------------|
| `Headless` | No additional IP allocated. DNS resolves to Pod IP. Use when broadcasting Pod IPs. |
| `ClusterIP` | Allocates a cluster-internal virtual IP. Routable only within the Kubernetes cluster. |
| `LoadBalancer` | Provisions an external load balancer. Use for internet-facing or cross-VPC access. Supports custom annotations and `loadBalancerClass`. |

### Broadcast address types

| Type | Source | Use case |
|------|--------|----------|
| `PodIP` | `Pod.status.podIP` | When Pod IPs are routable (same VPC, VPC peering, multi-DC). |
| `ServiceClusterIP` | `Service.spec.clusterIP` | In-cluster access only. |
| `ServiceLoadBalancerIngress` | `Service.status.loadBalancer.ingress[0]` | External access via load balancer. |

## Common deployment scenarios

### In-cluster only (default for ScyllaCluster)

Clients and nodes communicate via ClusterIP. The cluster is not reachable from outside Kubernetes.

```yaml
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

### VPC-routable clients, in-cluster nodes

Clients within the VPC connect directly to Pod IPs. Nodes communicate via ClusterIP within the Kubernetes cluster.

```yaml
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

### Multi-VPC (cross-datacenter)

Both clients and nodes use Pod IPs. Requires VPC peering or a shared network between Kubernetes clusters. Use this configuration for multi-DC clusters with multiple `ScyllaCluster` resources connected via `externalSeeds`.

```yaml
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

### Internet-facing via LoadBalancer

Each ScyllaDB node gets a dedicated load balancer with a public or internal address. Clients connect through the load balancer addresses. Nodes communicate via ClusterIP within the same Kubernetes cluster.

::::{tabs}
:::{group-tab} EKS
```yaml
spec:
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
:::

:::{group-tab} GKE
```yaml
spec:
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
:::
::::

:::{note}
LoadBalancer Services should be configured for TCP passthrough. Check your cloud provider's documentation for available annotations and configuration options.
:::

## TLS for external clients

When exposing ScyllaDB externally, ensure TLS certificates include the external addresses as Subject Alternative Names (SANs). Use `operatorManagedOptions` to add custom DNS names or IP addresses:

```yaml
spec:
  exposeOptions:
    nodeService:
      type: LoadBalancer
  network:
    tlsConfig:
      servingCertificate:
        type: OperatorManaged
        operatorManagedOptions:
          additionalDNSNames:
          - scylladb.example.com
          additionalIPAddresses:
          - 203.0.113.10
```

Alternatively, use `UserManaged` certificates from your own PKI or cert-manager.

## Related pages

- [Discovery endpoint](discovery.md) — exposing the discovery Service.
- [Connect via CQL](connect-via-cql.md) — client connection setup.
- [Networking architecture](../understand/networking.md) — how Services and expose options work.
- [Security](../understand/security.md) — TLS certificate management.
