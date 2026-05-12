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

## Verify external access

After applying your expose options, verify that the Services have received external addresses and that ScyllaDB is reachable.

### Check Service external addresses

```bash
kubectl -n scylla get services -l scylla/cluster=scylla
```

For LoadBalancer services, wait until `EXTERNAL-IP` is populated (this may take 1–2 minutes on cloud providers):

```
Expected output:
NAME                                      TYPE           CLUSTER-IP     EXTERNAL-IP       PORT(S)          AGE
scylla-us-east-1-us-east-1a-0             LoadBalancer   10.96.0.1      203.0.113.10      9042:30000/TCP   2m
```

### Verify broadcast addresses

Confirm that ScyllaDB is advertising the correct address to clients:

```bash
kubectl -n scylla get scyllacluster scylla -o jsonpath='{range .status.racks[*].members[*]}{.name}{"\t"}{.address}{"\n"}{end}'
```

### Test connectivity

Test a CQL connection using the external address:

```bash
kubectl run -it --rm --restart=Never cqlsh-test --image=scylladb/scylla \
  -- cqlsh <EXTERNAL-IP> 9042
```

Replace `<EXTERNAL-IP>` with the address shown in the Service output.

## Troubleshoot

**Service stuck in `<pending>` state**
: Cloud provider quota exceeded, or missing IAM permissions for load balancer creation. Check cloud provider events with `kubectl describe service <service-name> -n scylla`.

**Cannot connect from outside**
: Check firewall rules allow traffic on port 9042 (CQL) and 9142 (CQL/TLS) from client IP ranges. See [Prerequisites](../install-operator/prerequisites.md) for required firewall rules.

**Broadcast address mismatch**
: If ScyllaDB advertises an internal IP instead of the LoadBalancer IP, verify `broadcastOptions.clients.type` is set to `ServiceLoadBalancerIngress` and the LoadBalancer has an external IP assigned.

**TLS connection refused**
: Ensure clients use the correct CA certificate. See [Connect via CQL](connect-via-cql.md) for TLS connection setup.

## Related pages

- [Discovery endpoint](discovery.md) — exposing the discovery Service.
- [Connect via CQL](connect-via-cql.md) — client connection setup.
- [Networking architecture](../understand/networking.md) — how Services and expose options work.
- [Security](../understand/security.md) — TLS certificate management.
