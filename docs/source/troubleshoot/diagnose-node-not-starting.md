# Diagnose a node that is not starting

Identify and resolve the cause when a ScyllaDB node fails to start or become ready.

## Identify the pod state

```bash
kubectl -n scylla get pods -l scylla-operator.scylladb.com/pod-type=scylladb-node
kubectl -n scylla describe pod <pod-name>
```

Use the pod state to jump to the relevant section:

| Pod state | Section |
|---|---|
| `Pending` | [Pending — scheduling failure](#pending--scheduling-failure) |
| `Init:*` or `PodInitializing` | [Init container stuck](#init-container-stuck) |
| `Running` but not `Ready` | [Ignition not completing](#ignition-not-completing) |
| `CrashLoopBackOff` | [CrashLoopBackOff](#crashloopbackoff) |

## Pending — scheduling failure

The pod cannot be placed on any node.

### Common causes

| Cause | How to verify | Resolution |
|---|---|---|
| Insufficient CPU or memory | Event mentions `Insufficient cpu` or `Insufficient memory` | Add nodes to the node pool or reduce resource requests |
| Node affinity mismatch | Event mentions `didn't match Pod's node affinity/selector` | Verify `placement.nodeAffinity` in ScyllaCluster matches node labels |
| Taint not tolerated | Event mentions `had untolerated taint` | Add matching `tolerations` to the ScyllaCluster spec; see [Set up dedicated node pools](../deploy-scylladb/before-you-deploy/set-up-dedicated-node-pools.md) |
| PVC binding failure | Event mentions `unbound immediate PersistentVolumeClaims` | Verify the StorageClass exists and has available volumes; see [Local CSI driver](../install-operator/install-with-gitops.md) |
| No nodes with required label | No schedulable nodes match | Verify node pool has label `scylla.scylladb.com/node-type=scylla` |

### Diagnose

```bash
kubectl -n scylla describe pod <pod-name>
```

Look for `FailedScheduling` events.

## Init container stuck

### scylladb-bootstrap-barrier

This init container waits for the bootstrap synchronization to complete.
In a multi-datacenter deployment, it blocks until all datacenters are registered.

**Resolution:**
- Check that all datacenter `ScyllaCluster` resources are created and healthy.
- Verify cross-datacenter connectivity.
- See [Bootstrap synchronization](../understand/bootstrap-sync.md).

### sysctl-buddy

This init container applies sysctl settings configured via NodeConfig.

**Resolution:**
- Check the init container logs for permission errors.
- Verify NodeConfig is applied to the node: `kubectl get nodeconfig -o wide`
- See [Configure nodes](../deploy-scylladb/before-you-deploy/configure-nodes.md).

### Diagnose

Check which init container is running:

```bash
kubectl -n scylla get pod <pod-name> -o jsonpath='{.status.initContainerStatuses[*].name}'
kubectl -n scylla logs <pod-name> -c <init-container-name>
```

## Ignition not completing

The pod is `Running` but ScyllaDB has not started.
The ignition sidecar waits for prerequisites before starting ScyllaDB.

### Common causes

| Cause | How to verify | Resolution |
|---|---|---|
| NodeConfig tuning not ready | Check tuning ConfigMap: `kubectl -n scylla get configmap` | Verify NodeConfig matches the node; see [Configure nodes](../deploy-scylladb/before-you-deploy/configure-nodes.md) |
| LoadBalancer IP not assigned | `kubectl -n scylla get svc <pod-service-name>` shows `<pending>` for `EXTERNAL-IP` | Check cloud provider load balancer quotas; verify annotations |
| Service not created | `kubectl -n scylla get svc -l scylla-operator.scylladb.com/owner-uid=<pod-uid>` returns nothing | Check Operator logs for errors |

See [Ignition architecture](../understand/ignition.md) for details on the ignition sequence.

### Diagnose

```bash
kubectl -n scylla logs <pod-name> -c scylladb-ignition
```

## CrashLoopBackOff

ScyllaDB starts but crashes immediately.

### Common causes

| Cause | Log indicator | Resolution |
|---|---|---|
| Corrupt SSTables | `sstables_manager - Exception` or `Compaction error` | Run `nodetool scrub` from a healthy pod, or delete the PVC (data loss) |
| Invalid configuration | `configuration error` or `unknown option` | Check ScyllaCluster `scyllaArgs` for invalid arguments |
| Wrong seeds | `Could not reach any seeds` | Verify the cluster Service exists and has endpoints |
| Disk permission issues | `Permission denied` | Verify PVC ownership and StorageClass settings |
| Incompatible ScyllaDB version | Version mismatch errors | Verify the upgrade path is supported; see [Upgrading ScyllaDB](../upgrade/upgrade-scylladb.md) |

### Diagnose

```bash
kubectl -n scylla logs <pod-name> -c scylla --previous
```

## Recovery actions

| Action | When to use |
|---|---|
| `kubectl -n scylla delete pod <pod-name>` | Recreates the pod with the same PVC — useful for transient failures |
| Fix ScyllaCluster spec and apply | For configuration errors — triggers a rolling update |
| Fix NodeConfig and apply | For tuning issues — NodeConfig daemon re-applies settings |
| Fix node pool (labels, taints, instance types) | For scheduling failures |
| Delete PVC and pod | **Last resort** — causes data loss on that node; the Operator provisions a new node that streams data from peers |

:::{warning}
Deleting a PVC permanently destroys the data on that node.
Only do this when the data is replicated on other nodes and can be recovered via streaming.
:::

## Related pages

- [Diagnostic flowchart](diagnostic-flowchart.md)
- [Investigate pod restarts](investigate-restarts.md)
- [Ignition architecture](../understand/ignition.md)
- [Sidecar architecture](../understand/sidecar.md)
- [Set up dedicated node pools](../deploy-scylladb/before-you-deploy/set-up-dedicated-node-pools.md)
