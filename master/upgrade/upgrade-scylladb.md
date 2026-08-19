# Upgrading ScyllaDB clusters

Upgrading your ScyllaDB cluster to a newer version is automated by ScyllaDB Operator and performed using a rolling update
strategy to maintain availability. It is as simple as updating the ScyllaDB image reference in your ScyllaDB cluster specification.

#### WARNING
While the cluster remains operational throughout the process, applications requiring strict consistency levels (such as `QUORUM`)
may experience transient unavailability. This can occur if the cluster topology view has not yet fully converged across all
nodes before the next node is restarted.

We recommend scheduling upgrades during periods of low application traffic to minimize potential disruptions.

Issue tracking fix for this behavior: [scylla-operator #1077](https://github.com/scylladb/scylla-operator/issues/1077).

#### WARNING
ScyllaDB version upgrades must be performed consecutively, meaning **you must not skip any major or minor version on the upgrade path**.
Before upgrading to the next version, ensure the entire ScyllaDB cluster has been successfully upgraded.
For details, refer to the [Upgrade procedure in ScyllaDB’s documentation](https://enterprise.docs.scylladb.com/stable/upgrade/index.html#upgrade-upgrade-procedures).

## Upgrade via GitOps (kubectl)

To upgrade your ScyllaDB cluster using GitOps (kubectl), adjust the ScyllaDB image tag/reference to the target one in your ScyllaDB cluster specification and re-apply the manifest.

ScyllaCluster

```yaml
apiVersion: scylla.scylladb.com/v1
kind: ScyllaCluster
metadata:
  name: scylladb
spec:
  version: 2026.2.4 # Specify the target ScyllaDB image tag.
  # ...
```

After reapplying the manifest, wait for your ScyllaCluster to roll out.

```bash
kubectl wait --for='condition=Progressing=False' scyllacluster.scylla.scylladb.com/scylladb
kubectl wait --for='condition=Degraded=False' scyllacluster.scylla.scylladb.com/scylladb
kubectl wait --for='condition=Available=True' scyllacluster.scylla.scylladb.com/scylladb
```

ScyllaDBCluster

```yaml
apiVersion: scylla.scylladb.com/v1alpha1
kind: ScyllaDBCluster
metadata:
  name: dev-cluster
spec:
  scyllaDB:
    image: docker.io/scylladb/scylla:2026.2.4 # Specify the target ScyllaDB image reference.
  # ...
```

After reapplying the manifest, wait for your ScyllaDBCluster to roll out.

```bash
kubectl --context=${CONTROL_PLANE_CONTEXT} wait --for='condition=Progressing=False' scylladbcluster.scylla.scylladb.com/dev-cluster
kubectl --context=${CONTROL_PLANE_CONTEXT} wait --for='condition=Degraded=False' scylladbcluster.scylla.scylladb.com/dev-cluster
kubectl --context=${CONTROL_PLANE_CONTEXT} wait --for='condition=Available=True' scylladbcluster.scylla.scylladb.com/dev-cluster
```

To verify the cluster state, execute `nodetool status` in each of the ScyllaDB cluster pods:

```bash
NAMESPACE=<namespace>
CLUSTER_NAME=<cluster-name>

# List all ScyllaDB pods in the cluster.
pods=$(kubectl -n "${NAMESPACE}" get pods \
    -l scylla/cluster="${CLUSTER_NAME}" \
    -l scylla-operator.scylladb.com/pod-type=scylladb-node \
    -o name)

# Execute nodetool status in each pod.
for pod in $pods; do
    kubectl -n "${NAMESPACE}" exec "${pod}" -c scylla -- nodetool status
done
```

All nodes should report all other nodes as `UN` (Up and Normal) in the output, e.g.:

```console
Datacenter: us-east-1
===========================
Status=Up/Down
|/ State=Normal/Leaving/Joining/Moving
-- Address        Load    Tokens Owns Host ID                              Rack      
UN 10.221.135.48  3.30 KB 256    ?    5dd7f301-62d7-4ab7-986a-e7ea9d21be4d us-east-1a
UN 10.221.140.203 3.48 KB 256    ?    2f725f88-33fa-4ca7-b366-fa35e63e7c72 us-east-1b
UN 10.221.150.121 3.67 KB 256    ?    7063a262-fa3f-4f69-8a60-720f464b1483 us-east-1c
```

## Upgrade via Helm

#### IMPORTANT
ScyllaDB Operator does not yet support Helm installation path for managed multi-datacenter ScyllaDB clusters.

To upgrade your ScyllaDB cluster using Helm, upgrade your Helm release with the target ScyllaDB image tag/reference.

ScyllaCluster

```shell
helm upgrade scylla scylla/scylla --reuse-values --set=scyllaImage.tag=2026.2.4
```

After upgrading the release, wait for your ScyllaCluster to roll out.

```bash
kubectl wait --for='condition=Progressing=False' scyllacluster.scylla.scylladb.com/scylladb
kubectl wait --for='condition=Degraded=False' scyllacluster.scylla.scylladb.com/scylladb
kubectl wait --for='condition=Available=True' scyllacluster.scylla.scylladb.com/scylladb
```

To verify the cluster state, execute `nodetool status` in each of the ScyllaDB cluster pods:

```bash
NAMESPACE=<namespace>
CLUSTER_NAME=<cluster-name>

# List all ScyllaDB pods in the cluster.
pods=$(kubectl -n "${NAMESPACE}" get pods \
    -l scylla/cluster="${CLUSTER_NAME}" \
    -l scylla-operator.scylladb.com/pod-type=scylladb-node \
    -o name)

# Execute nodetool status in each pod.
for pod in $pods; do
    kubectl -n "${NAMESPACE}" exec "${pod}" -c scylla -- nodetool status
done
```

All nodes should report all other nodes as `UN` (Up and Normal) in the output, e.g.:

```console
Datacenter: us-east-1
===========================
Status=Up/Down
|/ State=Normal/Leaving/Joining/Moving
-- Address        Load    Tokens Owns Host ID                              Rack      
UN 10.221.135.48  3.30 KB 256    ?    5dd7f301-62d7-4ab7-986a-e7ea9d21be4d us-east-1a
UN 10.221.140.203 3.48 KB 256    ?    2f725f88-33fa-4ca7-b366-fa35e63e7c72 us-east-1b
UN 10.221.150.121 3.67 KB 256    ?    7063a262-fa3f-4f69-8a60-720f464b1483 us-east-1c
```
