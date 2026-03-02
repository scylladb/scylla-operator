# Configure dual-stack networking with IPv4

**What you'll achieve**: Deploy a ScyllaDB cluster that uses IPv4 for ScyllaDB communication and provides services accessible via both IPv4 and IPv6.

**Before you begin**: 
- You have a Kubernetes cluster with dual-stack networking enabled
- You have ScyllaDB Operator installed
- You have `kubectl` configured

**After completion**: Your ScyllaDB cluster will use IPv4 for inter-node communication while services are accessible via both IPv4 and IPv6.

:::{note}
This is the recommended configuration for production IPv6 deployments. For other configurations, see:
- [Configure IPv6-first dual-stack](ipv6-configure-ipv6-first.md) - ScyllaDB uses IPv6
- [Configure IPv6-only](ipv6-configure-ipv6-only.md) - Experimental

For configuration details, see the [IPv6 configuration reference](../reference/ipv6-configuration.md).
:::

:::

## Step 1: Apply the configuration

Apply the dual-stack configuration:

```bash
kubectl create namespace scylla
kubectl apply -f=https://raw.githubusercontent.com/{{repository}}/{{revision}}/examples/ipv6/scylla-cluster-dual-stack.yaml
```

## Step 2: Wait for the cluster to be ready

Monitor pod creation:

```bash
kubectl get pods -n scylla -l scylla-operator.scylladb.com/pod-type=scylladb-node -w
```

Wait until all pods show `Running` status.

## Step 3: Verify dual-stack configuration

Check that services have both IP families:

```bash
kubectl get svc -n scylla -o custom-columns=NAME:.metadata.name,IP-FAMILIES:.spec.ipFamilies,POLICY:.spec.ipFamilyPolicy
```

Expected output shows `[IPv4 IPv6]` for IP families:

```
NAME                            IP-FAMILIES        POLICY
scylla-dual-stack-client        [IPv4 IPv6]        PreferDualStack
scylla-dual-stack-us-east-1a-0  [IPv4 IPv6]        PreferDualStack
```

## Step 4: Verify cluster health

Check that all nodes are up:

```bash
kubectl exec -it scylla-dual-stack-example-rack-0 -n scylla -c scylla -- nodetool status
```

**Expected output:**
```
Datacenter: dual-stack-datacenter
===================
Status=Up/Down
|/ State=Normal/Leaving/Joining/Moving
--  Address           Load      Tokens  Owns  Host ID                              Rack
UN  fd00:10:244:1::7f 501.79 KB 256     ?     4583fff5-2aa6-4041-9be8-c74bcabaff8c dual-stack-rack-a
UN  fd00:10:244:2::6d 494.49 KB 256     ?     b1f889b4-80e7-4685-a3c5-1b81797c2ce4 dual-stack-rack-a
UN  fd00:10:244:3::6c 494.96 KB 256     ?     7a4bb6da-415e-4fc3-a6ca-0369c0e76bf0 dual-stack-rack-a
```

All nodes should show `UN` (Up/Normal) status.

## Next steps

- [Migrate existing clusters to IPv6](ipv6-migrate.md)
- [Troubleshoot IPv6 issues](ipv6-troubleshoot.md)
