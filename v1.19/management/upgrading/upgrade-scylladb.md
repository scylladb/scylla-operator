# Upgrading ScyllaDB clusters

Upgrading your ScyllaDB cluster to a newer version is automated by Scylla Operator and performed without any downtime.
It is as simple as updating the ScyllaDB image reference in your ScyllaDB cluster specification.

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
  version: 2025.3.3 # Specify the target ScyllaDB image tag.
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
    image: docker.io/scylladb/scylla:2025.3.3 # Specify the target ScyllaDB image reference.
  # ...
```

After reapplying the manifest, wait for your ScyllaDBCluster to roll out.

```bash
kubectl --context=${CONTROL_PLANE_CONTEXT} wait --for='condition=Progressing=False' scylladbcluster.scylla.scylladb.com/dev-cluster
kubectl --context=${CONTROL_PLANE_CONTEXT} wait --for='condition=Degraded=False' scylladbcluster.scylla.scylladb.com/dev-cluster
kubectl --context=${CONTROL_PLANE_CONTEXT} wait --for='condition=Available=True' scylladbcluster.scylla.scylladb.com/dev-cluster
```

## Upgrade via Helm

#### IMPORTANT
Scylla Operator does not yet support Helm installation path for managed multi-datacenter ScyllaDB clusters.

To upgrade your ScyllaDB cluster using Helm, upgrade your Helm release with the target ScyllaDB image tag/reference.

ScyllaCluster

```shell
helm upgrade scylla scylla/scylla --reuse-values --set=scyllaImage.tag=2025.3.3
```

After upgrading the release, wait for your ScyllaCluster to roll out.

```bash
kubectl wait --for='condition=Progressing=False' scyllacluster.scylla.scylladb.com/scylladb
kubectl wait --for='condition=Degraded=False' scyllacluster.scylla.scylladb.com/scylladb
kubectl wait --for='condition=Available=True' scyllacluster.scylla.scylladb.com/scylladb
```
