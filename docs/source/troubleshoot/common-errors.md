# Common errors

Reference table of frequently encountered error messages, Kubernetes failure states, and log patterns in ScyllaDB Operator deployments.

## Kubernetes pod states

| State | Description | Common cause | Resolution |
|---|---|---|---|
| `Pending` | Pod cannot be scheduled | Insufficient resources, affinity mismatch, untolerated taints, PVC binding failure | [Node not starting](diagnose-node-not-starting.md) |
| `Init:0/N` | Init container not completing | Bootstrap barrier waiting, sysctl application failing | [Node not starting](diagnose-node-not-starting.md) |
| `CrashLoopBackOff` | Container starts and crashes repeatedly | Invalid ScyllaDB configuration, corrupt data, version incompatibility | [Investigating restarts](investigate-restarts.md) |
| `ImagePullBackOff` | Cannot pull container image | Wrong image tag, registry unreachable, authentication failure | Verify the image exists: `kubectl -n scylla describe pod <pod-name>` |
| `OOMKilled` | Container exceeded its memory limit | Memory limit too low for the workload | Increase memory limits in ScyllaCluster spec |
| `Evicted` | Kubelet evicted the pod due to node pressure | Node under memory or disk pressure | Use dedicated node pools; see [Set up dedicated node pools](../deploy-scylladb/before-you-deploy/set-up-dedicated-node-pools.md) |
| `Terminating` (stuck) | Pod stuck in terminating state | Finalizer not removed, PVC in use, node unreachable | Check finalizers: `kubectl -n scylla get pod <pod-name> -o jsonpath='{.metadata.finalizers}'` |

## Webhook errors

| Error message | Cause | Resolution |
|---|---|---|
| `failed calling webhook` / `connection refused` | API server cannot reach the webhook Service | [Installation troubleshooting](troubleshoot-installation.md) |
| `x509: certificate signed by unknown authority` | Webhook TLS certificate not trusted | Verify cert-manager is running and the Certificate is `Ready` |
| `Internal error occurred: failed calling webhook ... the server could not find the requested resource` | CRDs not installed or version mismatch | Re-apply CRDs; see [Installation](troubleshoot-installation.md#crd-not-installed) |

## ScyllaDB startup errors

| Log pattern | Cause | Resolution |
|---|---|---|
| `Could not reach any seeds` | Node cannot contact seed nodes | Check that the cluster Service has endpoints; verify network connectivity |
| `Address already in use` | Port conflict on the node | Verify no other process uses ScyllaDB ports (9042, 7000, 7001, 10000) |
| `storage_service - All attempts to join the cluster have failed` | Node repeatedly fails to join the ring | Check for ghost nodes with `nodetool status`; see [Recovering from failed replace](recover-from-failed-replace.md) |
| `sstables_manager - Exception` | Corrupt SSTable files | Run `nodetool scrub` or delete PVC (data loss — last resort) |
| `configuration error` / `unknown option` | Invalid command-line argument | Check `scyllaArgs` in ScyllaCluster spec |

## Operator log messages

| Log pattern | Cause | Resolution |
|---|---|---|
| `Failed to sync ScyllaCluster` | Operator cannot reconcile the ScyllaCluster | Check the full error message for details; common causes include RBAC issues, missing CRDs, or API server connectivity |
| `StatefulSet not ready` | Rolling update in progress or stuck | Wait for the rollout; if stuck, see [Changing log level](change-log-level.md) for stuck rollout workaround |
| `Webhook server certificate not found` | cert-manager has not provisioned the webhook certificate | Verify cert-manager installation and Certificate resources |

## ScyllaCluster condition messages

| Condition | Status | Message pattern | Meaning |
|---|---|---|---|
| `Available` | `False` | `No rack has all members ready` | No rack is fully healthy — cluster cannot serve traffic reliably |
| `Degraded` | `True` | `Rack <name> has <n> out of <m> members ready` | Some members are down or not ready |
| `Progressing` | `True` | `Waiting for StatefulSet <name> to reach <n> ready replicas` | Rolling update or scaling in progress |

## Network and connectivity errors

| Error | Cause | Resolution |
|---|---|---|
| `Connection refused` to port 9042 | CQL server not running or not ready | Check pod readiness; verify ScyllaDB started successfully |
| `No host available` (driver error) | Client cannot reach any ScyllaDB node | Verify Service endpoints; check network policies; see [Connecting](../connect-your-app/index.md) |
| DNS resolution failure | CoreDNS not running or misconfigured | `kubectl -n kube-system get pods -l k8s-app=kube-dns`; for IPv6 issues see [IPv6 troubleshooting](../deploy-scylladb/set-up-networking/ipv6/troubleshooting.md) |

## Storage errors

| Error | Cause | Resolution |
|---|---|---|
| `unbound immediate PersistentVolumeClaims` | PVC cannot be bound — no matching PV | Verify StorageClass exists; check CSI driver is running |
| `volume expansion not supported` | StorageClass does not support expansion | Use a StorageClass with `allowVolumeExpansion: true`; see [Volume expansion](../operate/expand-storage-volumes.md) |
| `disk full` / `No space left on device` | Data volume is full | Expand storage or reduce data; see [Volume expansion](../operate/expand-storage-volumes.md) |

## Related pages

- [Diagnostic flowchart](diagnostic-flowchart.md)
- [Investigating restarts](investigate-restarts.md)
- [Node not starting](diagnose-node-not-starting.md)
- [Installation troubleshooting](troubleshoot-installation.md)
