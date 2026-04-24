# Getting started with IPv6 networking

This tutorial teaches you how to deploy your first ScyllaDB cluster with IPv6 networking. You'll learn IPv6 networking concepts while setting up a working cluster.

## What you'll learn

In this tutorial, you'll:
- Understand what IPv6 networking means for ScyllaDB
- Verify your Kubernetes cluster supports IPv6
- Deploy a ScyllaDB cluster with dual-stack networking
- Verify your cluster is working correctly
- Understand the configuration you created

:::{note}
This tutorial uses **dual-stack** configuration (both IPv4 and IPv6) because it's production-ready and well-tested. IPv6-only configurations are experimental. See [Production readiness](../reference/ipv6-configuration.md#production-readiness) for details.
:::

## Prerequisites

Before you begin:

### Kubernetes cluster

- **Dual-stack support**: Your cluster needs both IPv4 and IPv6 network ranges configured

### ScyllaDB

- **ScyllaDB version**: 2024.1 or newer (recommended for IPv6 support)
- **ScyllaDB Operator**: Already installed in your cluster

### Tools

- `kubectl` configured to access your cluster
- Basic familiarity with ScyllaDB and Kubernetes concepts

## Step 1: Verify IPv6 support

Check if your cluster has at least one node that has an IPv6 address of type InternalIP:

```bash
kubectl get nodes -o jsonpath='{.items[*].status.addresses[?(@.type=="InternalIP")].address}' | tr ' ' '\n' | grep ':'
```

This command filters for IPv6 addresses. You can quickly tell if an IP address is IPv4 if it uses dots `.` as separators (e.g. 172.16.57.29); IPv6 addresses use colons `:` as separators (e.g. 2001:db8:1::1).

**Example output for an IPv6-ready cluster:**
```
2001:db8:1::1
2001:db8:1::2
2001:db8:1::3
```

If you see IPv6 addresses in the output, then your cluster has some IPv6-enabled nodes.

:::{tip}
**No IPv6 addresses?** Your Kubernetes cluster may not have IPv6 enabled. Check your cluster's network configuration or consult your cluster administrator.
:::

## Step 2: Understand dual-stack networking

Before deploying, let's understand what dual-stack means:

- **Kubernetes services**: Have both IPv4 and IPv6 addresses
- **ScyllaDB nodes**: Communicate using a single IP family (IPv4 in this tutorial)
- **Client connectivity**: Clients can discover services via either protocol

This is useful when you have clients on different network types that all need to access your database.

## Step 3: Create your first IPv6-enabled cluster

Download the example dual-stack configuration:

```bash
kubectl apply -f=https://raw.githubusercontent.com/{{repository}}/{{revision}}/examples/ipv6/scylla-cluster-minimal-dual-stack.yaml
```

You can view the complete example configuration in the repository: [scylla-cluster-minimal-dual-stack.yaml](../../../../../../examples/ipv6/scylla-cluster-minimal-dual-stack.yaml)

The key networking configuration is:

```yaml
network:
  ipFamilyPolicy: PreferDualStack
  ipFamilies:
    - IPv4  # ScyllaDB will use IPv4 internally
    - IPv6  # Services will also be accessible via IPv6
  dnsPolicy: ClusterFirst
```

Or deploy directly:

```bash
kubectl create namespace scylla
kubectl apply -f=https://raw.githubusercontent.com/{{repository}}/{{revision}}/examples/ipv6/scylla-cluster-minimal-dual-stack.yaml
```

## Step 4: Watch the cluster deploy

Monitor the cluster deployment:

```bash
kubectl get pods -n scylla -w
```

Wait until all pods show `Running` status with `4/4` ready:

```
NAME                                           READY   STATUS    RESTARTS   AGE   IP            NODE
scylla-ipv6-tutorial-us-east-1-us-east-1a-0    4/4     Running   0          5m    10.244.1.5    worker-1
scylla-ipv6-tutorial-us-east-1-us-east-1a-1    4/4     Running   0          4m    10.244.2.6    worker-2
scylla-ipv6-tutorial-us-east-1-us-east-1a-2    4/4     Running   0          3m    10.244.3.7    worker-3
```

## Step 5: Verify IPv6 configuration

Check that your services have both IPv4 and IPv6 addresses:

```bash
kubectl get svc -n scylla -o custom-columns=NAME:.metadata.name,IP-FAMILIES:.spec.ipFamilies,POLICY:.spec.ipFamilyPolicy
```

**Expected output:**
```
NAME                                                  IP-FAMILIES        POLICY
scylla-ipv6-tutorial-client                           [IPv4 IPv6]        PreferDualStack
scylla-ipv6-tutorial-tutorial-dc-tutorial-rack-0      [IPv4 IPv6]        PreferDualStack
scylla-ipv6-tutorial-tutorial-dc-tutorial-rack-1      [IPv4 IPv6]        PreferDualStack
scylla-ipv6-tutorial-tutorial-dc-tutorial-rack-2      [IPv4 IPv6]        PreferDualStack
```

The `IP-FAMILIES` column shows both IPv4 and IPv6, confirming dual-stack configuration.

## Step 6: Verify cluster health

Check the cluster status using nodetool:

```bash
kubectl exec -it scylla-ipv6-tutorial-tutorial-dc-tutorial-rack-0 -n scylla -c scylla -- nodetool status
```

**Expected output:**
```
Datacenter: tutorial-dc
=======================
Status=Up/Down
|/ State=Normal/Leaving/Joining/Moving
--  Address        Load       Tokens       Owns    Host ID                               Rack
UN  10.244.1.5     256 KB     256          ?       a1b2c3d4-...                          tutorial-rack
UN  10.244.2.6     256 KB     256          ?       e5f6g7h8-...                          tutorial-rack
UN  10.244.3.7     256 KB     256          ?       i9j0k1l2-...                          tutorial-rack
```

All nodes should have status `UN` (Up/Normal).

:::{tip}
Notice that the addresses are of the family (either IPv4 or IPv6) that matches the first family that we set under `ipFamilies` in an earlier step. In this case, the addresses are IPv4 (like `10.244.x.x`), while services are accessible via both protocols.  
:::

## Step 7: Test client connectivity

Let's test connecting to the cluster via both IPv4 and IPv6:

```bash
# Get the client service IP addresses
kubectl get svc scylla-ipv6-tutorial-client -n scylla -o jsonpath='{.spec.clusterIPs}' | jq .
```

**Example output:**
```
[
  "10.96.10.20",           # IPv4 address
  "fd00:10:96::1234"       # IPv6 address
]
```

Test connectivity with cqlsh:

```bash
kubectl run -it --rm cqlsh --image=scylladb/scylla-cqlsh:latest --restart=Never -n scylla-test -- \
    scylla-ipv6-tutorial-client.scylla.svc.cluster.local 9042 \
  -e "SELECT cluster_name,broadcast_address FROM system.local;"  
```

**Example output**
```
-------------------+---------------------
 cluster_name      | tutorial
 broadcast_address | 10.96.136.225

(1 rows)
pod "cqlsh" deleted
```

## Understanding your configuration

Let's break down what you configured:

### Network settings

```yaml
network:
  ipFamilyPolicy: PreferDualStack  # Use dual-stack if available
  ipFamilies:
    - IPv4  # First = ScyllaDB's internal protocol
    - IPv6  # Second = Additional service accessibility
  dnsPolicy: ClusterFirst  # Essential for proper DNS resolution
```

**Key points:**
- The **first** IP family (`IPv4`) determines what protocol ScyllaDB uses internally
- The **second** IP family (`IPv6`) makes services accessible via IPv6 too
- `PreferDualStack` falls back to single-stack if dual-stack isn't available

For more details on how ScyllaDB nodes advertise themselves, see [Exposing ScyllaDB clusters](../../../../resources/common/exposing.md).

## What's next?

Now that you have a working IPv6-enabled cluster, you can:

- Learn how to [configure different IPv6 setups](../how-to/ipv6-configure.md) (IPv6-only, different dual-stack configurations)
- Understand [how IPv6 networking works](../concepts/ipv6-networking.md) in ScyllaDB
- Explore the [IPv6 configuration reference](../reference/ipv6-configuration.md) for all available options
- Learn how to [migrate existing clusters to IPv6](../how-to/ipv6-migrate.md)

## Clean up

To remove the tutorial cluster:

```bash
kubectl delete -f scylla-cluster-ipv6.yaml
kubectl delete namespace scylla
```

## Related documentation

- [How to configure IPv6 networking](../how-to/ipv6-configure.md)
- [IPv6 networking concepts](../concepts/ipv6-networking.md)
- [IPv6 configuration reference](../reference/ipv6-configuration.md)
