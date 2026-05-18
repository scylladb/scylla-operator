# must-gather contents

This page describes the structure of a must-gather archive and how to find specific information within it.

## Archive structure

### Collection log

`/scylla-operator-must-gather.log` describes collection success or failure on the resource level.

### ScyllaCluster objects

Find them with:

```bash
grep -rl "^kind: ScyllaCluster" <must-gather-directory>
```

Each file contains:

- Status conditions (`Available` / `Progressing` / `Degraded`) and their details.
- ScyllaDB configuration.
- Rack configurations (number of nodes, resource requests).

### ScyllaDB racks

Racks are represented by StatefulSets in the same namespace as the ScyllaCluster object:

```
/namespaces/<namespace>/statefulsets.apps/<cluster>-<datacenter>-<rack>
```

### ScyllaDB nodes

Each node is represented by several Kubernetes resources in the same namespace as the ScyllaCluster:

**Pod** — `/namespaces/<namespace>/pods/<cluster>-<datacenter>-<rack>-<ordinal>.yaml`

The corresponding directory `/namespaces/<namespace>/pods/<cluster>-<datacenter>-<rack>-<ordinal>/` includes items collected from the ScyllaDB node:

- `nodetool-status.log` — output of `nodetool status`
- `nodetool-gossipinfo.log` — output of `nodetool gossipinfo`
- `df.log` — disk usage
- `io_properties.yaml` — ScyllaDB IO properties configuration
- `scylla-rlimits.log` — resource limits of the ScyllaDB process
- `scylla.current` and `scylla.previous` — ScyllaDB logs

**Service** — `/namespaces/<namespace>/services/<cluster>-<datacenter>-<rack>-<ordinal>.yaml`

Contains:

- The state of a node replace in progress.
- The HostID as seen by Operator.
- Network configuration, including IP addresses.

**PersistentVolumeClaim** — `/namespaces/<namespace>/persistentvolumeclaims/...`

Contains:

- The type of storage (StorageClass) in use.
- A reference to the PersistentVolume that maps to an actual directory on the disk.

**Jobs** — `/namespaces/<namespace>/jobs.batch/...`

Contains references to Pods (for example, cleanup Jobs).

## Structure of a Kubernetes object

A typical object is located at `/namespaces/<namespace>/<resource-type>/<object-name>.yaml`:

```yaml
apiVersion: <group>/v1
kind: <Kind>
metadata:
  name: <object-name>
  namespace: <namespace>
  creationTimestamp: ...
  # deletionTimestamp and finalizers are present when deletion is in progress
  # (useful for investigating why `kubectl delete` is hanging)
spec:
  # Declarative configuration
status:
  # This is the interesting part most of the time
  conditions:
    # Contains Available/Progressing/Degraded details
```

## Find information in the archive

### Diagnose a stuck resource

Look at status conditions:

```bash
grep -rl '^kind: ScyllaCluster' <must-gather-directory>
grep -rl '^kind: NodeConfig' <must-gather-directory>
grep -rl '^kind: ScyllaDBManagerTask' <must-gather-directory>
grep -rl '^kind: ScyllaOperatorConfig' <must-gather-directory>
```

Open the files and look under `.status.conditions`. Each condition has a `type` (`Available`, `Progressing`, `Degraded`, or something more granular).
See [Conditions](../../reference/conditions.md) for a detailed reference.

- **Available**: Everything should be up and running. Check `Progressing` for any non-disruptive operations in progress.
- **Progressing**: Operator is trying to make a change. If stuck, look at the Operator logs.
- **Degraded**: Something failed. Look at the `reason` and the Operator logs.

Pay attention to conditions where `reason` is not `AsExpected`.

### Find Operator logs

```
/namespaces/scylla-operator/pods/scylla-operator-<hash>/scylla-operator.current
```

:::{note}
There will typically be more than one Operator Pod (in an active-standby configuration).
:::

### Find ScyllaDB logs by rack name and node number

List all ScyllaDB logs:

```bash
find <must-gather-directory> -type f -name scylla.current
```

To find logs for a specific node, first identify the ScyllaCluster:

```bash
grep -rl "^kind: ScyllaCluster" <must-gather-directory>
# Example output: namespaces/scylla/scyllaclusters.scylla.scylladb.com/my-cluster.yaml
```

Then find the logs at:

```
namespaces/<namespace>/pods/<cluster>-<datacenter>-<rack>-<ordinal>/scylla.current
```

### Find ScyllaDB logs by HostID

Look at the HostIDs of node Services:

```bash
grep "internal.scylla-operator.scylladb.com/host-id" \
    <must-gather-directory>/namespaces/<namespace>/services/<cluster>*.yaml
```

Example output:

```
namespaces/scylla/services/my-cluster-dc-rack-0.yaml:    internal.scylla-operator.scylladb.com/host-id: 1849fd66-13ef-490e-846d-fabe55e08000
namespaces/scylla/services/my-cluster-dc-rack-1.yaml:    internal.scylla-operator.scylladb.com/host-id: d2241fb5-1cf0-4c2e-86fb-b21cfd936f91
```

Find the Service matching your HostID, then look at the corresponding Pod logs:

```
namespaces/<namespace>/pods/<matching-pod-name>/scylla.current
```

### Find ScyllaDB configuration

- Look at `.spec` of the ScyllaCluster object, particularly the ConfigMap referenced by `.spec.datacenter.racks[].scyllaConfig`.
- Look at the ScyllaDB command line arguments in the ScyllaDB node logs.

:::{note}
Operator generates `scylla.yaml` from settings in the ScyllaCluster spec.
must-gather does not currently collect the resulting effective `scylla.yaml`.
:::

## Too much data collected?

- [Limit collection to a particular namespace](must-gather.md#limit-collection-to-a-particular-namespace) by passing the `--namespace` flag.
- [Exclude resources](must-gather.md#exclude-resources) by passing the `--exclude-resource` flag.
- Manually curate the archive to remove information you do not want to include.

## Missing information?

The information collected from ScyllaDB Pods is implemented in [podcollector.go](https://github.com/scylladb/scylla-operator/blob/master/pkg/gather/collect/podcollector.go).
If must-gather does not collect information you need, [file a feature request](https://github.com/scylladb/scylla-operator/issues/new) or submit a PR.

## Related pages

- [Collect data with must-gather](must-gather.md)
- [Query system tables](system-tables.md)
