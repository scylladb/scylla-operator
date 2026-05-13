# ScyllaDB Concepts on Kubernetes

If you are familiar with ScyllaDB but new to Kubernetes, this page maps the ScyllaDB concepts you already know to their Kubernetes equivalents when running ScyllaDB with the Operator.

## Concept mapping

| ScyllaDB concept | Kubernetes equivalent | Notes |
|---|---|---|
| **ScyllaDB node** | **Pod** | Each ScyllaDB node runs inside a Kubernetes pod. A pod is the smallest deployable unit in Kubernetes and contains one or more containers. |
| **ScyllaDB process** | **Container** (inside the pod) | The ScyllaDB process runs inside a container within the pod. The pod also contains sidecar containers for monitoring, tuning, and management. |
| **Datacenter** | **`ScyllaCluster`** | A `ScyllaCluster` resource represents one ScyllaDB datacenter. For multi-DC setups, create one `ScyllaCluster` per datacenter in each Kubernetes cluster and connect them using `externalSeeds`. |
| **Rack** | **StatefulSet** | Each rack in the ScyllaCluster spec maps to a Kubernetes StatefulSet. The StatefulSet guarantees stable pod names, persistent storage, and ordered startup/shutdown. |
| **Cluster** | **`ScyllaCluster`** resource (one per datacenter) | You declare the desired cluster state as a YAML resource. The Operator continuously reconciles the actual state to match. For multi-DC clusters, create one `ScyllaCluster` per datacenter and connect them via `externalSeeds`. |
| **`scylla.yaml` config file** | **ConfigMap** referenced by `scyllaConfig` | ScyllaDB configuration is stored in a Kubernetes ConfigMap and referenced from the ScyllaCluster spec. The Operator also generates configuration automatically. |
| **Data directory** | **PersistentVolumeClaim (PVC)** | Each ScyllaDB node's data is stored on a PersistentVolume, provisioned via a PVC. Data survives pod restarts. |
| **Node IP / listen address** | **Service** (per member) | Each ScyllaDB node gets a dedicated Kubernetes Service that provides a stable network identity, independent of pod restarts. |
| **Seed nodes** | Managed automatically | The Operator selects seed nodes and configures them. You do not need to manage seeds manually. |
| **`nodetool`** | `kubectl exec` + `nodetool` | Run `nodetool` commands by executing into the ScyllaDB container: `kubectl exec -it <pod> -c scylla -- nodetool status`. Read-only commands are safe; state-changing commands should be avoided (see [nodetool alternatives](../reference/nodetool-alternatives.md)). |
| **ScyllaDB Manager** | **ScyllaDB Manager deployment** | The Operator deploys and configures ScyllaDB Manager automatically. Repair and backup tasks are defined in the cluster spec or as `ScyllaDBManagerTask` resources. |
| **Repair / Backup tasks** | **ScyllaDBManagerTask** resource or cluster spec fields | Instead of running `sctool` commands, you declare tasks as Kubernetes resources or in the ScyllaCluster spec. |

## Common Kubernetes commands for ScyllaDB

Here are copy-paste-ready commands for common tasks. Replace `<namespace>` with the namespace where your ScyllaDB cluster is deployed, and `<cluster>` with the cluster name.

### Check cluster status

:::{code-block} shell
kubectl -n=<namespace> get scyllaclusters.scylla.scylladb.com/<cluster>
:::

### List ScyllaDB pods

:::{code-block} shell
kubectl -n=<namespace> get pods -l=scylla/cluster=<cluster>
:::

### Check a pod's status

:::{code-block} shell
kubectl -n=<namespace> describe pod/<pod-name>
:::

Look for:
- **Status**: `Running`, `Pending`, `CrashLoopBackOff`, etc.
- **Conditions**: whether the pod is `Ready`.
- **Events**: recent events that may indicate problems.

### View ScyllaDB logs

:::{code-block} shell
# Current container logs
kubectl -n=<namespace> logs pod/<pod-name> -c=scylla

# Previous container logs (after a restart)
kubectl -n=<namespace> logs pod/<pod-name> -c=scylla --previous
:::

### Run nodetool

:::{code-block} shell
kubectl -n=<namespace> exec -it pod/<pod-name> -c=scylla -- nodetool status
:::

### Run cqlsh

:::{code-block} shell
kubectl -n=<namespace> exec -it pod/<pod-name> -c=scylla -- cqlsh localhost -u cassandra -p cassandra
:::

## Understanding pod names

ScyllaDB pod names follow a predictable pattern:

```
<cluster>-<datacenter>-<rack>-<ordinal>
```

For example, `scylladb-us-east-1-us-east-1a-0` is:
- Cluster: `scylladb`
- Datacenter: `us-east-1`
- Rack: `us-east-1a`
- Ordinal: `0` (first node in the rack)

The ordinal is zero-based. When you scale up a rack, new nodes get the next ordinal. When you scale down, the node with the highest ordinal is removed.

## Key differences from bare-metal ScyllaDB

| Aspect | Bare metal | Kubernetes with Operator |
|---|---|---|
| **Starting/stopping nodes** | `systemctl start/stop scylla` | The Operator manages pod lifecycle. Delete a pod to restart it; the Operator recreates it automatically. |
| **Adding nodes** | Install ScyllaDB on a new machine, configure seeds, start. | Increase `members` in the rack spec. The Operator handles everything. |
| **Removing nodes** | Run `nodetool decommission`, then shut down. | Decrease `members` in the rack spec. The Operator runs decommission before removing the pod. |
| **Replacing a dead node** | Start a new node with `replace_address_first_boot`. | Label the node's Service for replacement. The Operator handles the rest. |
| **Configuration changes** | Edit `scylla.yaml` and restart. | Update the ConfigMap or ScyllaCluster spec. The Operator performs a rolling restart. |
| **Monitoring** | Deploy monitoring stack manually. | Create a `ScyllaDBMonitoring` resource. |
| **Upgrades** | Update packages, restart nodes one by one. | Change the `version` field. The Operator performs a rolling upgrade. |

## Kubernetes failure states and what they mean for ScyllaDB

When troubleshooting, you may encounter these Kubernetes-specific states:

| State | What it means | What to check |
|---|---|---|
| **Pending** | The pod cannot be scheduled onto a node. Common causes: no nodes match the affinity/toleration rules, or insufficient CPU/memory resources on available nodes. | Node resources, node selectors, taints/tolerations, PVC binding. |
| **CrashLoopBackOff** | The container keeps crashing and Kubernetes is backing off before restarting. ScyllaDB is starting then crashes repeatedly. Common causes: misconfigured `scylla.yaml`, insufficient memory, corrupt data on the PVC, or init containers not completing. | Container logs (`kubectl logs --previous`), ScyllaDB startup errors. |
| **ImagePullBackOff** | Kubernetes cannot pull the container image. Check the image tag, registry access from the node, and any pull secrets. | Image name/tag, registry credentials, network connectivity. |
| **Init:0/N** | Init containers have not completed yet. An init container has not yet completed. Possible causes: the bootstrap barrier is waiting for a prerequisite, or a sidecar init is failing. | Init container logs, NodeConfig status, bootstrap barrier. |
| **Running but not Ready** | The container is running but the readiness probe is failing. | ScyllaDB may still be starting up, or it may be unhealthy. Check logs. |

For detailed troubleshooting procedures, see the [troubleshooting guide](../troubleshoot/index.md).
