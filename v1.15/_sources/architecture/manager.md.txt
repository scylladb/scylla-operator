# ScyllaDB Manager

{{productName}} has a basic integration with ScyllaDB Manager. At this point there is one global ScyllaDBManager instance that manages all [ScyllaClusters](../resources/scyllaclusters/basics.md) and a corresponding controller that automatically configures the ScyllaDB Manager to monitor the ScyllaDB instances and sync [repair](#api-scylla.scylladb.com-scyllaclusters-v1-.spec.repairs[]) and [backup](#api-scylla.scylladb.com-scyllaclusters-v1-.spec.backups[]) tasks based on [ScyllaCluster](../api-reference/groups/scylla.scylladb.com/scyllaclusters.rst) definition. Unfortunately, the rest of the functionality is not yet implemented in ScyllaCluster APIs and e.g. a restore of a cluster from a backup needs to be performed by executing into the shared ScyllaDB Manager deployment and using `sctool` directly by an administrator.

:::{caution}
Because ScyllaDB Manager instance is shared by all users and their ScyllaClusters, only administrators should have privileges to access the `scylla-manager` namespace.
:::

ScyllaDB Manager uses a small ScyllaCluster instance internally and thus depends on the {{productName}} deployment and the CRD it provides.

:::{include} ../.internal/manager-license-note.md
:::

## Accessing ScyllaDB Manager

For the operations that are not yet supported on ScyllaClusters, you can access the ScyllaDB Manager manually.

To find the ScyllaDB Manager ID for your cluster, run:

:::{code-block} bash
kubectl get scyllacluster/basic --template='{{ .status.managerId }}'
:::

:::{note}
Note that some of the operations use *ScyllaDB Manager Agent* that runs within the ScyllaCluster that has to have access e.g. to buckets being used.
:::

## Configuring backup and repair tasks for a ScyllaCluster

[Backup](#api-scylla.scylladb.com-scyllaclusters-v1-.spec.backups[]) and [repair](#api-scylla.scylladb.com-scyllaclusters-v1-.spec.repairs[]) tasks are configured for each [ScyllaCluster](../api-reference/groups/scylla.scylladb.com/scyllaclusters.rst) in its resource definition.

## Manual restore procedure

You can find more detail on how to perform the manual restore procedure in [this dedicated page](../resources/scyllaclusters/nodeoperations/restore.md). 
