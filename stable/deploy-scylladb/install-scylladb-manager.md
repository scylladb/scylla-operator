# Install ScyllaDB Manager

[ScyllaDB Manager](https://manager.docs.scylladb.com/) provides automated repair and backup scheduling for ScyllaDB clusters.
With Manager installed, ScyllaDB Operator can:

- **Schedule backups** — automatically snapshot your data and upload it to object storage.
- **Schedule repairs** — run automated anti-entropy repairs to keep data consistent across replicas.
- **Restore from backup** — recover a ScyllaDB cluster from a previously created backup snapshot.
  See [Restore from backup](https://operator.docs.scylladb.com/stable/operate/restore-from-backup.md).

For details on how Manager integrates with the Operator, see [ScyllaDB Manager](https://operator.docs.scylladb.com/stable/understand/manager.md).

## Prerequisites

- ScyllaDB Operator installed and running.
  See [Install with Helm](https://operator.docs.scylladb.com/stable/install-operator/install-with-helm.md) or [Install with GitOps](https://operator.docs.scylladb.com/stable/install-operator/install-with-gitops.md).
- Nodes configured with the local CSI driver installed.
  Manager deploys a small internal ScyllaCluster that uses the storage class provided by the local CSI driver.
  See [Configure nodes](https://operator.docs.scylladb.com/stable/deploy-scylladb/before-you-deploy/configure-nodes.md).

## Install ScyllaDB Manager

ScyllaDB Manager deploys into the `scylla-manager` namespace.
It runs a small internal ScyllaCluster for its own state.

#### NOTE
ScyllaDB Manager must be installed in the `scylla-manager` namespace.
The Operator expects Manager in this namespace and will not discover it otherwise.

GitOps (manifests)

Apply the manifest:

```shell
kubectl -n=scylla-manager apply --server-side -f=https://raw.githubusercontent.com/scylladb/scylla-operator/v1.22/deploy/manager-prod.yaml
```

Helm

Install the Helm chart:

```shell
helm install scylla-manager scylla/scylla-manager \
  --create-namespace \
  --namespace scylla-manager
```

Wait for Manager to become available:

```shell
kubectl -n=scylla-manager rollout status --timeout=10m deployment.apps/scylla-manager
```

## Verify the installation

Check that the Manager Pod is running:

```shell
kubectl -n=scylla-manager get pods
```

You should see the `scylla-manager` Deployment Pod and one or more Pods for the internal Manager ScyllaCluster.

## Next steps

- [ScyllaDB Manager](https://operator.docs.scylladb.com/stable/understand/manager.md) — understand Manager architecture, task synchronization, and security.
