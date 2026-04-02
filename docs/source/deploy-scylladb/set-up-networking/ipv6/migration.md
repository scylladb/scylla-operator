# Migrate clusters to IPv6

Migrate an existing IPv4-only ScyllaDB cluster to dual-stack or IPv6-only networking.

## Before you begin

| Requirement | Details |
|---|---|
| Existing cluster | Running on IPv4 |
| Kubernetes cluster | Dual-stack networking enabled |
| Backups | A recent backup of the cluster ([Back up and restore](../../../operate/back-up-and-restore.md)) |

:::{warning}
Migration triggers a rolling restart of all ScyllaDB pods.
Plan the migration during a maintenance window.
Data is preserved during the restart.
:::

## Choose a migration path

| Path | Recommended | Description |
|---|---|---|
| [IPv4 → dual-stack](#migrate-from-ipv4-to-dual-stack) | Yes | Add IPv6 accessibility while keeping IPv4 for ScyllaDB inter-node communication |
| [IPv4 → IPv6-only](#migrate-from-ipv4-to-ipv6-only) | No (experimental) | Move entirely to IPv6 |

The recommended approach is to migrate to dual-stack first.
If you later need IPv6 for ScyllaDB inter-node communication, you can change the order of `ipFamilies` in a second step.

## Migrate from IPv4 to dual-stack

### Step 1: Back up your data

Verify that a recent backup exists before making any changes:

```bash
kubectl -n scylla get scylladbmanagertask
```

If you do not have backups configured, see [Back up and restore](../../../operate/back-up-and-restore.md).

### Step 2: Update the ScyllaCluster

Add `network` and `exposeOptions` to your ScyllaCluster manifest:

```yaml
apiVersion: scylla.scylladb.com/v1
kind: ScyllaCluster
metadata:
  name: my-cluster
  namespace: scylla
spec:
  # ... existing configuration ...

  network:
    dnsPolicy: ClusterFirst
    ipFamilyPolicy: PreferDualStack
    ipFamilies:
      - IPv4   # Keep IPv4 as the ScyllaDB protocol
      - IPv6   # Add IPv6 Service accessibility

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

### Step 3: Apply the configuration

```bash
kubectl apply --server-side -f scylla-cluster.yaml
```

The Operator performs a rolling restart of all ScyllaDB pods.

### Step 4: Monitor the rolling restart

```bash
kubectl -n scylla get pods -l scylla-operator.scylladb.com/pod-type=scylladb-node -w
```

Wait for every pod to return to `Running` with all containers ready.

### Step 5: Verify dual-stack Services

```bash
kubectl -n scylla get svc -o custom-columns=NAME:.metadata.name,IP-FAMILIES:.spec.ipFamilies,POLICY:.spec.ipFamilyPolicy
```

**Expected output:**

```
NAME                    IP-FAMILIES    POLICY
my-cluster-client       [IPv4 IPv6]    PreferDualStack
my-cluster-dc-rack-0    [IPv4 IPv6]    PreferDualStack
```

### Step 6: Verify cluster health

```bash
kubectl -n scylla exec -it my-cluster-dc-rack-0 -c scylla -- nodetool status
```

All nodes should show `UN` (Up / Normal).

### Step 7: Test connectivity

```bash
kubectl -n scylla get svc my-cluster-client -o jsonpath='{.spec.clusterIPs}'
```

You should see both an IPv4 and an IPv6 address.
Existing client applications continue to work over IPv4 without changes.
New clients can connect over IPv6 using the same service DNS name.

## Migrate from IPv4 to IPv6-only

:::{warning}
IPv6-only is **experimental**.
See [Configure IPv6-only single-stack](configure-single-stack.md) for details and limitations.
:::

### Step 1: Verify IPv6 readiness

Confirm that all nodes have IPv6 addresses:

```bash
kubectl get nodes -o jsonpath='{.items[*].status.addresses[?(@.type=="InternalIP")].address}' | tr ' ' '\n' | grep ':'
```

Verify that all client applications support IPv6.

### Step 2: Back up your data

```bash
kubectl -n scylla get scylladbmanagertask
```

### Step 3: Update client applications

Update all client applications to connect over IPv6 **before** migrating the cluster.
After migration, IPv4 Service addresses are no longer available.

### Step 4: Update the ScyllaCluster

```yaml
apiVersion: scylla.scylladb.com/v1
kind: ScyllaCluster
metadata:
  name: my-cluster
  namespace: scylla
spec:
  # ... existing configuration ...

  network:
    dnsPolicy: ClusterFirst
    ipFamilyPolicy: SingleStack
    ipFamilies:
      - IPv6

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

### Step 5: Apply and monitor

```bash
kubectl apply --server-side -f scylla-cluster.yaml
kubectl -n scylla get pods -l scylla-operator.scylladb.com/pod-type=scylladb-node -w
```

### Step 6: Verify IPv6-only operation

```bash
kubectl -n scylla exec -it my-cluster-dc-rack-0 -c scylla -- nodetool status
```

**Expected output** with IPv6 addresses:

```
Datacenter: dc
==============
Status=Up/Down
|/ State=Normal/Leaving/Joining/Moving
--  Address              Load       Tokens  Owns  Host ID                               Rack
UN  fd00:10:244:1::7f    501 KB     256     ?     4583fff5-...                          rack
UN  fd00:10:244:2::6d    494 KB     256     ?     b1f889b4-...                          rack
UN  fd00:10:244:3::6c    494 KB     256     ?     7a4bb6da-...                          rack
```

## Rolling back

If you encounter issues, revert the `network` section to the previous configuration:

```yaml
network:
  ipFamilyPolicy: SingleStack
  ipFamilies:
    - IPv4
```

Apply the change:

```bash
kubectl apply --server-side -f scylla-cluster.yaml
```

The Operator performs another rolling restart to return the cluster to IPv4.

## Best practices

1. **Test in a non-production environment first.**
2. **Back up data** before every migration step.
3. **Use dual-stack first**, then migrate to IPv6-only later if needed.
4. **Coordinate with application teams** — client connection strings may need updating.
5. **Monitor cluster health** throughout the migration with `nodetool status` and your monitoring stack.

## Related pages

- [Get started with IPv6](get-started.md)
- [Configure dual-stack networking](configure-dual-stack.md)
- [Configure IPv6-only single-stack](configure-single-stack.md)
- [Troubleshoot IPv6 issues](troubleshooting.md)
- [Back up and restore](../../../operate/back-up-and-restore.md)
