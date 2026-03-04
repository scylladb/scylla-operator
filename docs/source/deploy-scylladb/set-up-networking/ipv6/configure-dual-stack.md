# Configure dual-stack networking

Set up a ScyllaDB cluster with dual-stack networking so that Kubernetes Services are accessible over both IPv4 and IPv6.

## How dual-stack works

In a dual-stack deployment:

- **ScyllaDB** uses a single IP family (the first entry in `ipFamilies`) for gossip, replication, and broadcast addresses.
- **Kubernetes Services** get addresses in both IP families, allowing clients on either protocol to reach the database.

The first entry in `ipFamilies` determines which protocol ScyllaDB uses internally.
The second entry adds Service-level accessibility in the other protocol.

## Prerequisites

| Requirement | Details |
|---|---|
| Kubernetes cluster | Dual-stack networking enabled |
| ScyllaDB Operator | Already installed ([Installation](../../install-operator/index.md)) |
| ScyllaDB version | 2024.1 or newer recommended |

## IPv4-first dual-stack

ScyllaDB uses IPv4 for inter-node communication. Services are also accessible over IPv6.

This is the **recommended** configuration for most deployments.

### Apply the configuration

```bash
kubectl create namespace scylla
kubectl apply --server-side -f=https://raw.githubusercontent.com/{{repository}}/{{revision}}/examples/ipv6/scylla-cluster-dual-stack.yaml
```

The network section of the manifest:

```yaml
network:
  dnsPolicy: ClusterFirst
  ipFamilyPolicy: PreferDualStack
  ipFamilies:
    - IPv4   # ScyllaDB uses IPv4
    - IPv6   # Services also get an IPv6 address

exposeOptions:
  broadcastOptions:
    nodes:
      type: PodIP
      podIP:
        source: Status
    clients:
      type: PodIP
      podIP:
        source: Status
```

### Wait for the cluster to be ready

```bash
kubectl -n scylla get pods -l scylla-operator.scylladb.com/pod-type=scylladb-node -w
```

Wait until all pods show `Running` status.

### Verify dual-stack Services

```bash
kubectl -n scylla get svc -o custom-columns=NAME:.metadata.name,IP-FAMILIES:.spec.ipFamilies,POLICY:.spec.ipFamilyPolicy
```

**Expected output:**

```
NAME                                       IP-FAMILIES    POLICY
scylla-dual-stack-example-client           [IPv4 IPv6]    PreferDualStack
scylla-dual-stack-example-...-rack-a-0     [IPv4 IPv6]    PreferDualStack
```

### Verify cluster health

```bash
kubectl -n scylla exec -it scylla-dual-stack-example-dual-stack-datacenter-dual-stack-rack-a-0 \
  -c scylla -- nodetool status
```

All nodes should show `UN` (Up / Normal) with IPv4 addresses, because IPv4 is the first entry in `ipFamilies`:

```
Datacenter: dual-stack-datacenter
=================================
Status=Up/Down
|/ State=Normal/Leaving/Joining/Moving
--  Address      Load       Tokens  Owns  Host ID                               Rack
UN  10.244.1.5   501 KB     256     ?     4583fff5-...                          dual-stack-rack-a
UN  10.244.2.6   494 KB     256     ?     b1f889b4-...                          dual-stack-rack-a
UN  10.244.3.7   494 KB     256     ?     7a4bb6da-...                          dual-stack-rack-a
```

## IPv6-first dual-stack

ScyllaDB uses IPv6 for inter-node communication. Services are also accessible over IPv4.

Use this configuration when your network infrastructure is primarily IPv6 but you still need to support IPv4 clients.

### Create the ScyllaCluster

Create a file `scylla-ipv6-first.yaml`:

```yaml
apiVersion: scylla.scylladb.com/v1
kind: ScyllaCluster
metadata:
  name: scylla-ipv6-first
  namespace: scylla
spec:
  version: 6.2.2
  agentVersion: 3.3.3
  datacenter:
    name: datacenter1
    racks:
      - name: rack1
        members: 3
        storage:
          capacity: 100Gi
          storageClassName: scylladb-local-xfs
        resources:
          limits:
            cpu: 4
            memory: 16Gi
          requests:
            cpu: 4
            memory: 16Gi
  network:
    dnsPolicy: ClusterFirst
    ipFamilyPolicy: PreferDualStack
    ipFamilies:
      - IPv6   # ScyllaDB uses IPv6
      - IPv4   # Services also get an IPv4 address
  exposeOptions:
    nodeService:
      type: Headless
    broadcastOptions:
      nodes:
        type: PodIP
        podIP:
          source: Status
      clients:
        type: ServiceClusterIP
```

### Apply and verify

```bash
kubectl create namespace scylla
kubectl apply --server-side -f scylla-ipv6-first.yaml
```

Wait for pods:

```bash
kubectl -n scylla get pods -l scylla-operator.scylladb.com/pod-type=scylladb-node -w
```

Verify Services show `[IPv6 IPv4]`:

```bash
kubectl -n scylla get svc -o custom-columns=NAME:.metadata.name,IP-FAMILIES:.spec.ipFamilies,POLICY:.spec.ipFamilyPolicy
```

**Expected output:**

```
NAME                               IP-FAMILIES    POLICY
scylla-ipv6-first-client           [IPv6 IPv4]    PreferDualStack
scylla-ipv6-first-datacenter1-...  [IPv6 IPv4]    PreferDualStack
```

Verify that ScyllaDB is using IPv6 addresses:

```bash
kubectl -n scylla exec -it scylla-ipv6-first-datacenter1-rack1-0 \
  -c scylla -- nodetool status
```

**Expected output** with IPv6 addresses (colons instead of dots):

```
Datacenter: datacenter1
=======================
Status=Up/Down
|/ State=Normal/Leaving/Joining/Moving
--  Address              Load       Tokens  Owns  Host ID                               Rack
UN  fd00:10:244:1::7f    501 KB     256     ?     4583fff5-...                          rack1
UN  fd00:10:244:2::6d    494 KB     256     ?     b1f889b4-...                          rack1
UN  fd00:10:244:3::6c    494 KB     256     ?     7a4bb6da-...                          rack1
```

## Configuration reference

| Field | Values | Effect |
|---|---|---|
| `network.ipFamilies` | `[IPv4, IPv6]` | ScyllaDB uses IPv4; Services support both |
| `network.ipFamilies` | `[IPv6, IPv4]` | ScyllaDB uses IPv6; Services support both |
| `network.ipFamilyPolicy` | `PreferDualStack` | Creates dual-stack Services when the cluster supports it; falls back to single-stack if not |
| `network.dnsPolicy` | `ClusterFirst` | Required for AAAA record resolution by CoreDNS |

:::{note}
`PreferDualStack` is recommended over `RequireDualStack` because it degrades gracefully on clusters that do not support dual-stack, creating single-stack Services with the first IP family instead of failing.
:::

## Multi-datacenter considerations

All datacenters in a multi-datacenter deployment **must** use the same first IP family.
ScyllaDB requires consistent addressing for gossip, replication, and the global token ring.

```yaml
# Datacenter 1
network:
  ipFamilies: [IPv6, IPv4]

# Datacenter 2 — must match the first IP family
network:
  ipFamilies: [IPv6, IPv4]
```

## Related pages

- [Get started with IPv6](get-started.md)
- [Configure IPv6-only single-stack](configure-single-stack.md)
- [Migrate existing clusters to IPv6](migration.md)
- [Expose ScyllaDB clusters](../expose-clusters.md)
- [Networking architecture](../../understand/networking.md)
