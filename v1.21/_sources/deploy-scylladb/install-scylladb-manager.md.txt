# Install ScyllaDB Manager

[ScyllaDB Manager](https://manager.docs.scylladb.com/) provides automated repair and backup scheduling for ScyllaDB clusters.
With Manager installed, ScyllaDB Operator can:

- **Schedule backups** — automatically snapshot your data and upload it to object storage.
- **Schedule repairs** — run automated anti-entropy repairs to keep data consistent across replicas.
- **Restore from backup** — recover a ScyllaDB cluster from a previously created backup snapshot.
  See [Restore from backup](../operate/restore-from-backup.md).

For details on how Manager integrates with the Operator, see [ScyllaDB Manager](../understand/manager.md).

## Prerequisites

- ScyllaDB Operator installed and running.
  See [Install with Helm](../install-operator/install-with-helm.md) or [Install with GitOps](../install-operator/install-with-gitops.md).
- Nodes configured with the local CSI driver installed.
  Manager deploys a small internal ScyllaCluster that uses the storage class provided by the local CSI driver.
  See [Configure nodes](before-you-deploy/configure-nodes.md).

## Install ScyllaDB Manager

ScyllaDB Manager deploys into the `scylla-manager` namespace.
It runs a small internal ScyllaCluster for its own state.

:::{note}
ScyllaDB Manager must be installed in the `scylla-manager` namespace.
The Operator expects Manager in this namespace and will not discover it otherwise.
:::

::::{tabs}
:::{tab} GitOps (manifests)

Apply the manifest:

```{code-block} shell
:substitutions:
kubectl -n=scylla-manager apply --server-side -f=https://raw.githubusercontent.com/{{repository}}/{{revision}}/deploy/manager-prod.yaml
```

:::

:::{tab} Helm

Install the Helm chart:

```shell
helm install scylla-manager scylla/scylla-manager \
  --create-namespace \
  --namespace scylla-manager
```

:::
::::

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

- [ScyllaDB Manager](../understand/manager.md) — understand Manager architecture, task synchronization, and security.
