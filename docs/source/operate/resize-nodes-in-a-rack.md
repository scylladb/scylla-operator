# Resize nodes in a rack

Change the CPU or memory of the ScyllaDB nodes of a rack.
To grow the storage of the nodes, see [Expand storage volumes](expand-storage-volumes.md) instead.

## How resizing works

The `resources` of a rack are the resources of the ScyllaDB container of each of its nodes.
The CPU limit sets the number of shards of a node, and the memory limit sets the memory ScyllaDB uses.
It's advised to keep the requests equal to the limits, so that the Pods keep the Guaranteed QoS class that [CPU pinning](../understand/tuning.md) needs.

Make sure the Kubernetes nodes have room for the new size: a Pod that no Kubernetes node can fit stays `Pending`, and the change gets stuck on it. See [Recover from a stuck config change](../troubleshoot/recover-from-stuck-config-change.md).

Set the namespace and the name of your `ScyllaCluster`, and the name and the index of the rack to change:

```shell
NAMESPACE=scylla
CLUSTER=my-cluster
RACK=us-east-1a
# Zero-based index of the rack in spec.datacenter.racks.
RACK_INDEX=0
```

## Increase the CPU, or change the memory

Set the new limits on the rack, and the requests to match:

```shell
kubectl -n "${NAMESPACE}" patch scyllacluster "${CLUSTER}" --type=json -p="
- op: replace
  path: /spec/datacenter/racks/${RACK_INDEX}/resources/limits/cpu
  value: '8'
- op: replace
  path: /spec/datacenter/racks/${RACK_INDEX}/resources/requests/cpu
  value: '8'
- op: replace
  path: /spec/datacenter/racks/${RACK_INDEX}/resources/limits/memory
  value: 64Gi
- op: replace
  path: /spec/datacenter/racks/${RACK_INDEX}/resources/requests/memory
  value: 64Gi
"
```

Wait for the change to roll out:

```shell
kubectl -n "${NAMESPACE}" wait --for='condition=Progressing=False' --timeout=30m "scyllacluster/${CLUSTER}"
kubectl -n "${NAMESPACE}" wait --for='condition=Available=True' --timeout=10m "scyllacluster/${CLUSTER}"
```

## Decrease the CPU

ScyllaDB Operator rejects a decrease of the CPU limit unless the `ScyllaCluster` carries the `scylla-operator.scylladb.com/force-scylladb-cpu-decrease: "true"` annotation, because of the ScyllaDB limitation described below.
Keyspaces use tablets by default since ScyllaDB 2025.2, so lower the CPU limit by replacing the nodes of the rack one at a time, as described below.

Only if none of your keyspaces use tablets, set the annotation as in Step 1, change the CPU as in the previous section, and remove the annotation as in Step 4 instead.
To find out which keyspaces use tablets, run `cqlsh` from any node:

```shell
kubectl -n "${NAMESPACE}" exec my-cluster-us-east-1-us-east-1a-0 -c scylla -- cqlsh -e "SELECT keyspace_name, initial_tablets FROM system_schema.scylla_keyspaces"
```

```console
 keyspace_name | initial_tablets
---------------+-----------------
            ks |               0

(1 rows)
```

The keyspaces with a value in `initial_tablets` use tablets.

### Why a plain change doesn't work with tablets

The CPU limit of the ScyllaDB container sets the number of shards of a node.
ScyllaDB [can't reduce the shard count](https://github.com/scylladb/scylladb/issues/16739) of a node that holds tablet-based tables: a node restarted with fewer CPUs refuses to start, and the rack gets stuck on it.

A replacement node starts with a fresh data directory and streams its data from the other replicas, so it can have any number of shards.
This procedure lowers the CPU limit and replaces each node of the rack as its rollout reaches it.

### Prerequisites

- Every keyspace has a replication factor of at least 2 in the datacenter, so that the data of a replaced node is available from other replicas.
- The cluster is healthy: `Progressing=False`, `Available=True`, and all nodes are `UN` in `nodetool status`.
- There is no other node operation in progress in the datacenter.

The replacements stream the data of every node of the rack once. Plan for the load and the time this takes with your data volume.

### Procedure

#### Step 1: Allow the CPU decrease

```shell
kubectl -n "${NAMESPACE}" annotate scyllacluster "${CLUSTER}" scylla-operator.scylladb.com/force-scylladb-cpu-decrease=true
```

#### Step 2: Lower the CPU limit

Set the new CPU limit on the rack, and the request to match if you use guaranteed resources:

```shell
kubectl -n "${NAMESPACE}" patch scyllacluster "${CLUSTER}" --type=json -p="
- op: replace
  path: /spec/datacenter/racks/${RACK_INDEX}/resources/limits/cpu
  value: '4'
- op: replace
  path: /spec/datacenter/racks/${RACK_INDEX}/resources/requests/cpu
  value: '4'
"
```

ScyllaDB Operator rolls the rack one node at a time, starting from the highest ordinal.
The restarted node fails to start with fewer shards, which is expected, and the rollout waits on it.

#### Step 3: Replace each node as it fails to start

Find the Pod that isn't ready:

```shell
kubectl -n "${NAMESPACE}" get pods -l scylla/cluster="${CLUSTER}",scylla/rack="${RACK}"
```

```console
NAME                                READY   STATUS    RESTARTS      AGE
my-cluster-us-east-1-us-east-1a-0   5/5     Running   0             2d
my-cluster-us-east-1-us-east-1a-1   5/5     Running   0             2d
my-cluster-us-east-1-us-east-1a-2   4/5     Running   3 (2m ago)    15m
```

Label its member Service, which has the same name as the Pod, to replace the node:

```shell
POD=my-cluster-us-east-1-us-east-1a-2
kubectl -n "${NAMESPACE}" label svc "${POD}" scylla/replace=""
```

ScyllaDB Operator deletes the Pod and its data volume, and the new Pod bootstraps as a replacement with the new CPU limit, streaming its data from the other replicas.
Wait for it to become ready:

```shell
kubectl -n "${NAMESPACE}" wait --for='condition=Ready' --timeout=30m "pod/${POD}"
```

The rollout then moves on to the next node, which fails to start the same way.
Repeat this step for every node of the rack, one at a time, until all Pods are ready.

#### Step 4: Remove the annotation

```shell
kubectl -n "${NAMESPACE}" annotate scyllacluster "${CLUSTER}" scylla-operator.scylladb.com/force-scylladb-cpu-decrease-
```

#### Step 5: Verify

```shell
kubectl -n "${NAMESPACE}" wait --for='condition=Progressing=False' --timeout=10m "scyllacluster/${CLUSTER}"
kubectl -n "${NAMESPACE}" wait --for='condition=Available=True' --timeout=10m "scyllacluster/${CLUSTER}"
kubectl -n "${NAMESPACE}" exec "${POD}" -c scylla -- nodetool status
```

```console
Datacenter: us-east-1
===========================
Status=Up/Down
|/ State=Normal/Leaving/Joining/Moving
-- Address       Load    Tokens Owns Host ID                              Rack
UN 10.96.153.205 1.24 MB 256    ?    52d102f2-2e42-41f3-b397-9ffdb9d57a34 us-east-1a
UN 10.96.23.29   1.25 MB 256    ?    e7a019b1-f1c7-494f-826b-6dec95644772 us-east-1a
UN 10.96.245.235 1.26 MB 256    ?    c9978ee0-014c-49a8-96cb-882e9a10d5fa us-east-1a
```

All nodes report `UN`, and the replaced nodes have new host IDs.
