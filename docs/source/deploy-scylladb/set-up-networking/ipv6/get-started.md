# Get started with IPv6 networking

This tutorial walks you through deploying your first ScyllaDB cluster with IPv6 networking.

## What you will learn

By the end of this tutorial you will:

- Verify that your Kubernetes cluster supports IPv6
- Deploy a ScyllaDB cluster with dual-stack networking
- Confirm that services are accessible over both IPv4 and IPv6
- Understand how IPv6 configuration maps to ScyllaDB behaviour

:::{note}
This tutorial uses **dual-stack** configuration (both IPv4 and IPv6) because it is production-ready and well-tested.
IPv6-only single-stack configurations are experimental — see [Configure IPv6-only single-stack](configure-single-stack.md) for details.
:::

## Prerequisites

| Requirement | Details |
|---|---|
| Kubernetes cluster | Dual-stack networking enabled (both IPv4 and IPv6 CIDR ranges configured) |
| ScyllaDB Operator | Already installed ([Installation](../../../install-operator/index.md)) |
| ScyllaDB version | 2024.1 or newer recommended |
| Tools | `kubectl` configured to access the cluster |

## Step 1: Verify IPv6 support

Check that your cluster nodes have IPv6 addresses:

```bash
kubectl get nodes -o jsonpath='{.items[*].status.addresses[?(@.type=="InternalIP")].address}' | tr ' ' '\n' | grep ':'
```

IPv6 addresses use colons (`:`) as separators (for example `2001:db8:1::1`), while IPv4 addresses use dots (`.`).

**Expected output** (one IPv6 address per node):

```
2001:db8:1::1
2001:db8:1::2
2001:db8:1::3
```

:::{tip}
If no IPv6 addresses appear, your Kubernetes cluster does not have IPv6 enabled.
Check the cluster network configuration or consult your cluster administrator.
:::

## Step 2: Deploy a dual-stack ScyllaDB cluster

Apply the minimal dual-stack example:

```bash
kubectl create namespace scylla
kubectl apply --server-side -f=https://raw.githubusercontent.com/{{repository}}/{{revision}}/examples/ipv6/scylla-cluster-dual-stack.yaml
```

The key section in this manifest is:

```yaml
network:
  dnsPolicy: ClusterFirst
  ipFamilyPolicy: PreferDualStack
  ipFamilies:
    - IPv4   # ScyllaDB uses IPv4 for inter-node communication
    - IPv6   # Kubernetes Services also get an IPv6 address

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

The **first** entry in `ipFamilies` determines which protocol ScyllaDB uses internally.
The second entry adds IPv6 accessibility at the Kubernetes Service level.

## Step 3: Wait for the cluster to be ready

Watch the pods until every pod shows `Running` and all containers are ready:

```bash
kubectl -n scylla get pods -w
```

**Expected output:**

```
NAME                                                READY   STATUS    RESTARTS   AGE
scylla-dual-stack-example-dual-stack-datacenter-dual-stack-rack-a-0   4/4     Running   0          5m
scylla-dual-stack-example-dual-stack-datacenter-dual-stack-rack-a-1   4/4     Running   0          4m
scylla-dual-stack-example-dual-stack-datacenter-dual-stack-rack-a-2   4/4     Running   0          3m
```

## Step 4: Verify dual-stack Services

Confirm that Kubernetes Services have both IPv4 and IPv6 addresses:

```bash
kubectl -n scylla get svc -o custom-columns=NAME:.metadata.name,IP-FAMILIES:.spec.ipFamilies,POLICY:.spec.ipFamilyPolicy
```

**Expected output:**

```
NAME                                                       IP-FAMILIES    POLICY
scylla-dual-stack-example-client                           [IPv4 IPv6]    PreferDualStack
scylla-dual-stack-example-dual-stack-datacenter-dual-stack-rack-a-0   [IPv4 IPv6]    PreferDualStack
scylla-dual-stack-example-dual-stack-datacenter-dual-stack-rack-a-1   [IPv4 IPv6]    PreferDualStack
scylla-dual-stack-example-dual-stack-datacenter-dual-stack-rack-a-2   [IPv4 IPv6]    PreferDualStack
```

The `IP-FAMILIES` column shows both `IPv4` and `IPv6`, confirming dual-stack.

## Step 5: Verify cluster health

Run `nodetool status` on any ScyllaDB pod:

```bash
kubectl -n scylla exec -it scylla-dual-stack-example-dual-stack-datacenter-dual-stack-rack-a-0 \
  -c scylla -- nodetool status
```

**Expected output:**

```
Datacenter: dual-stack-datacenter
=================================
Status=Up/Down
|/ State=Normal/Leaving/Joining/Moving
--  Address      Load       Tokens  Owns  Host ID                               Rack
UN  10.244.1.5   256 KB     256     ?     a1b2c3d4-...                          dual-stack-rack-a
UN  10.244.2.6   256 KB     256     ?     e5f6g7h8-...                          dual-stack-rack-a
UN  10.244.3.7   256 KB     256     ?     i9j0k1l2-...                          dual-stack-rack-a
```

All nodes should show status `UN` (Up / Normal).
The addresses match the first entry in `ipFamilies` — in this case IPv4.

## Step 6: Test client connectivity

Retrieve the client Service ClusterIPs:

```bash
kubectl -n scylla get svc scylla-dual-stack-example-client \
  -o jsonpath='{.spec.clusterIPs}' | jq .
```

**Example output:**

```json
[
  "10.96.10.20",
  "fd00:10:96::1234"
]
```

Connect with CQL using the service DNS name:

```bash
kubectl run -it --rm cqlsh --image=scylladb/scylla-cqlsh:latest --restart=Never -- \
  scylla-dual-stack-example-client.scylla.svc.cluster.local 9042 \
  -e "SELECT cluster_name, broadcast_address FROM system.local;"
```

## Understanding the configuration

| Field | Purpose |
|---|---|
| `network.ipFamilies` | The first entry sets the protocol ScyllaDB uses for gossip, replication, and broadcast addresses. The second entry adds Service accessibility in the other protocol. |
| `network.ipFamilyPolicy` | `PreferDualStack` creates dual-stack Services when the cluster supports it, and falls back to single-stack otherwise. |
| `network.dnsPolicy` | `ClusterFirst` is required so that AAAA (IPv6) DNS records are resolved correctly by CoreDNS. |

The Operator automatically translates these high-level settings into the corresponding ScyllaDB command-line arguments (`--enable-ipv6-dns-lookup`, `--listen-address`, `--broadcast-address`, and so on).
You do not need to set ScyllaDB arguments manually.

## Clean up

Remove the tutorial cluster:

```bash
kubectl delete scyllacluster -n scylla scylla-dual-stack-example
kubectl delete namespace scylla
```

## Next steps

- [Configure dual-stack networking](configure-dual-stack.md) — IPv4-first and IPv6-first variants
- [Configure IPv6-only single-stack](configure-single-stack.md) — experimental
- [Migrate existing clusters to IPv6](migration.md)

## Related pages

- [Expose ScyllaDB clusters](../expose-clusters.md)
- [Networking architecture](../../understand/networking.md)
