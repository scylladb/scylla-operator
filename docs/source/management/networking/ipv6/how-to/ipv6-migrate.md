# Migrate clusters to IPv6

This guide shows you how to migrate existing ScyllaDB clusters to IPv6 networking.

## Before you begin

### Prerequisites

- Existing ScyllaDB cluster running on IPv4
- Kubernetes cluster with IPv6 support enabled
- Administrative access to the cluster

### Important considerations

- **Downtime**: Migration requires a rolling restart of all pods
- **Data safety**: Data is preserved during migration
- **Client updates**: Client applications need updated connection strings after migration
- **Testing**: Test the migration process in a non-production environment first

:::{warning}
Migration involves a rolling restart of your cluster. Plan the migration during a maintenance window.
:::

## Choose your migration path

Select the migration approach that fits your needs:

1. [Migrate from IPv4 to dual-stack](#migrate-from-ipv4-to-dual-stack): Add IPv6 support while keeping IPv4 (recommended)
2. [Migrate from IPv4 to IPv6-only](#migrate-from-ipv4-to-ipv6-only): Completely migrate to IPv6 (experimental)

## Migrate from IPv4 to dual-stack

This is the recommended migration path because:
- Minimizes disruption to existing clients
- Allows gradual client migration
- Provides fallback to IPv4 if issues arise

### Step 1: Backup your data

Before making any changes, ensure you have recent backups. See [Configuring backup tasks](../../../../architecture/manager.md#configuring-backup-and-repair-tasks-for-a-scyllacluster) for details on setting up backups for your ScyllaCluster.

### Step 2: Update cluster configuration

Edit your ScyllaCluster manifest to add dual-stack support:

```yaml
apiVersion: scylla.scylladb.com/v1
kind: ScyllaCluster
metadata:
  name: your-cluster-name
  namespace: scylla
spec:
  # ... existing configuration ...
  
  # Add network configuration
  network:
    ipFamilyPolicy: PreferDualStack
    ipFamilies:
      - IPv4  # Keep IPv4 as primary
      - IPv6  # Add IPv6 support
    dnsPolicy: ClusterFirst
```

### Step 3: Apply the configuration

Apply the updated configuration:

```bash
kubectl apply -f scylla-cluster.yaml
```

The operator will perform a rolling update of all pods.

### Step 4: Monitor the migration

Watch the rolling update progress:

```bash
# Check cluster status
kubectl exec -it <pod-name> -n scylla -c scylla -- nodetool status
```

**Expected output:**
```
Datacenter: dc
===================
Status=Up/Down
|/ State=Normal/Leaving/Joining/Moving
--  Address           Load      Tokens  Owns  Host ID                              Rack
UN  10.244.2.7        501.79 KB 256     ?     4583fff5-2aa6-4041-9be8-c74bcabaff8c rack
UN  10.244.2.8        494.49 KB 256     ?     b1f889b4-80e7-4685-a3c5-1b81797c2ce4 rack
UN  10.244.2.9        494.96 KB 256     ?     7a4bb6da-415e-4fc3-a6ca-0369c0e76bf0 rack
```

Wait for all nodes to show `UN` (Up/Normal) status.

### Step 5: Verify dual-stack configuration

Confirm services have both IP families:

```bash
kubectl get svc -n scylla -o custom-columns=NAME:.metadata.name,IP-FAMILIES:.spec.ipFamilies,POLICY:.spec.ipFamilyPolicy
```

**Expected output:**
```
NAME                            IP-FAMILIES        POLICY
scylla-dual-stack-client        [IPv4 IPv6]        PreferDualStack
scylla-dual-stack-us-east-1a-0  [IPv4 IPv6]        PreferDualStack
```

### Step 6: Test connectivity

Test that clients can connect via both protocols:

```bash
# Get service IPs
kubectl get svc your-cluster-name-client -n scylla -o jsonpath='{.spec.clusterIPs}'
```

**Example output**
```
[
  "10.96.136.229",
  "fd00:10:96::6277"
]
```
Test connection using service name:

```bash
kubectl run -it --rm cqlsh --image=scylladb/scylla-cqlsh:latest --restart=Never -n scylla-test -- \
    your-cluster-name-client.scylla.svc.cluster.local 9042 \
  -e "SELECT cluster_name,broadcast_address FROM system.local;"  
```

**Example output**
```
-------------------+---------------------
 cluster_name      | scylla-cluster
 broadcast_address | 10.244.2.42

(1 rows)
pod "cqlsh" deleted
```

### Step 7: Update client applications

Update your client applications to use the dual-stack service. Most clients will automatically work with dual-stack services without changes.

### Step 8: Verify cluster health

After migration completes:

```bash
# Verify all nodes are up
kubectl exec -it <pod-name> -n scylla -c scylla -- nodetool status
```

## Migrate from IPv4 to IPv6-only

:::{warning}
**Experimental Feature**: IPv6-only configurations are experimental. See [Production readiness](../reference/ipv6-configuration.md#production-readiness) for details. This migration path requires careful planning and testing.
:::

### Step 1: Verify IPv6-only readiness

Ensure your environment supports IPv6-only:

```bash
# Check that all nodes have IPv6 addresses
kubectl get nodes -o jsonpath='{.items[*].status.addresses[?(@.type=="InternalIP")].address}' | tr ' ' '\n' | grep ':'
```

**Example output**
```
fc00:f853:ccd:e793::2
fc00:f853:ccd:e793::4
fc00:f853:ccd:e793::3
fc00:f853:ccd:e793::5
```

### Step 2: Update client applications

Update all client applications to support IPv6 before migrating the cluster.

### Step 3: Backup your data

Create a backup before proceeding. See [Configuring backup tasks](../../../../architecture/manager.md#configuring-backup-and-repair-tasks-for-a-scyllacluster) for details on setting up backups for your ScyllaCluster.

### Step 4: Update cluster configuration

Edit your ScyllaCluster manifest for IPv6-only:

```yaml
apiVersion: scylla.scylladb.com/v1
kind: ScyllaCluster
metadata:
  name: your-cluster-name
  namespace: scylla
spec:
  # ... existing configuration ...
  
  # Update network configuration
  network:
    ipFamilyPolicy: SingleStack
    ipFamilies:
      - IPv6  # IPv6 only
    dnsPolicy: ClusterFirst
```

### Step 5: Apply and monitor

Apply the configuration and monitor the migration:

```bash
kubectl apply -f scylla-cluster.yaml

# Monitor the rolling update
kubectl get pods -n scylla -w
```

### Step 6: Verify IPv6-only operation

Check that cluster is using IPv6:

```bash
# Verify pod IPv6 addresses
kubectl get pods -n scylla -o wide

# Check cluster status
kubectl exec -it <pod-name> -n scylla -c scylla -- nodetool status
```

**Expected output with IPv6 addresses:**
```
Datacenter: datacenter
===================
Status=Up/Down
|/ State=Normal/Leaving/Joining/Moving
--  Address           Load      Tokens  Owns  Host ID                              Rack
UN  fd00:10:244:1::7f 501.79 KB 256     ?     4583fff5-2aa6-4041-9be8-c74bcabaff8c rack
UN  fd00:10:244:2::6d 494.49 KB 256     ?     b1f889b4-80e7-4685-a3c5-1b81797c2ce4 rack
UN  fd00:10:244:3::6c 494.96 KB 256     ?     7a4bb6da-415e-4fc3-a6ca-0369c0e76bf0 rack
```

### Step 7: Test client connectivity

Test that clients can connect:

```bash
kubectl run -it --rm cqlsh --image=scylladb/scylla-cqlsh:latest --restart=Never -n scylla-test -- \
    your-cluster-name-client.scylla.svc.cluster.local 9042 \
  -e "SELECT cluster_name,broadcast_address FROM system.local;"  
```

**Expected output:**
```
-------------------+---------------------
 cluster_name      | scylla-cluster
 broadcast_address | fd00:10:244:2::25

(1 rows)
pod "cqlsh" deleted
```

### Step 8: Verify cluster health

Check cluster health:

```bash
# Verify all nodes are up
kubectl exec -it <pod-name> -n scylla -c scylla -- nodetool status
```

## Rollback to IPv4

If you encounter issues during migration, you can roll back to the previous IPv4-only configuration.

Update your ScyllaCluster manifest:

```yaml
network:
  ipFamilyPolicy: SingleStack
  ipFamilies:
    - IPv4
```

Apply the rollback:

```bash
kubectl apply -f scylla-cluster.yaml
```

The operator will perform a rolling update back to IPv4.

## Troubleshooting

If you encounter problems during migration:

### Nodes not joining cluster

**Symptom**: Pods are running but nodes show as down

**Solution**:
1. Check DNS resolution:
   ```bash
   kubectl exec -it <pod-name> -n scylla -- nslookup <service-name>
   ```

2. Verify network configuration:
   ```bash
   kubectl get svc -n scylla -o yaml | grep -A 5 -i family
   ```

3. Review pod logs:
   ```bash
   kubectl logs <pod-name> -n scylla -c scylla
   ```

### Connection failures

**Symptom**: Clients cannot connect after migration

**Solution**:
1. Verify service IPs:
   ```bash
   kubectl get svc -n scylla -o wide
   ```

2. Check client configuration for IPv6 support

For more troubleshooting steps, see [Troubleshoot IPv6 issues](ipv6-troubleshoot.md).

## Migration best practices

1. **Test first**: Always test migration in a non-production environment
2. **Backup**: Create backups before starting migration
3. **Monitoring**: Set up alerts for cluster health during migration
4. **Gradual approach**: Use dual-stack first, then migrate to IPv6-only if needed
5. **Client coordination**: Coordinate with application teams before migration
6. **Documentation**: Document your specific migration steps and any customizations

## Next steps

- [Troubleshoot IPv6 issues](ipv6-troubleshoot.md)
- [Configure IPv6 networking](ipv6-configure.md)
- [Understand IPv6 networking concepts](../concepts/ipv6-networking.md)

## Related documentation

- [How to configure IPv6 networking](ipv6-configure.md)
- [Troubleshoot IPv6 networking](ipv6-troubleshoot.md)
- [IPv6 networking concepts](../concepts/ipv6-networking.md)
- [IPv6 configuration reference](../reference/ipv6-configuration.md)
