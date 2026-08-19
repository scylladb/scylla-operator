# ScyllaDB Manager

Scylla Operator has a basic integration with ScyllaDB Manager. At this point there is one global ScyllaDBManager instance that manages all [ScyllaClusters](https://operator.docs.scylladb.com/v1.18/resources/scyllaclusters/basics.md). Scylla Operator automatically configures the ScyllaDB Manager to monitor the ScyllaDB instances and sync [repair](https://operator.docs.scylladb.com/v1.18/api-reference/groups/scylla.scylladb.com/scyllaclusters.md#api-scylla-scylladb-com-scyllaclusters-v1-spec-repairs) and [backup](https://operator.docs.scylladb.com/v1.18/api-reference/groups/scylla.scylladb.com/scyllaclusters.md#api-scylla-scylladb-com-scyllaclusters-v1-spec-backups) tasks based on [ScyllaCluster](https://operator.docs.scylladb.com/v1.18/api-reference/groups/scylla.scylladb.com/scyllaclusters.md) definition. Unfortunately, the rest of the functionality is not yet implemented in ScyllaCluster APIs and e.g. a restore of a cluster from a backup needs to be performed by executing into the shared ScyllaDB Manager deployment and using `sctool` directly by an administrator.

ScyllaDB Manager uses a small ScyllaCluster instance internally and thus depends on the Scylla Operator deployment and the CRD it provides.

#### NOTE
ScyllaDB Manager is available for ScyllaDB Enterprise customers and ScyllaDB Open Source users.
With ScyllaDB Open Source, ScyllaDB Manager is limited to 5 nodes.
See the ScyllaDB Manager [Proprietary Software License Agreement](https://www.scylladb.com/scylla-manager-software-license-agreement/) for details.

## Accessing ScyllaDB Manager

For the operations that are not yet supported on ScyllaClusters, you can access the ScyllaDB Manager manually.

To find the ScyllaDB Manager ID for your cluster, run:

```bash
kubectl get scyllacluster/basic --template='{{ .status.managerId }}'
```

#### NOTE
Note that some of the operations use *ScyllaDB Manager Agent* that runs within the ScyllaCluster that has to have access e.g. to buckets being used.

## Configuring backup and repair tasks for a ScyllaCluster

[Backup](https://operator.docs.scylladb.com/v1.18/api-reference/groups/scylla.scylladb.com/scyllaclusters.md#api-scylla-scylladb-com-scyllaclusters-v1-spec-backups) and [repair](https://operator.docs.scylladb.com/v1.18/api-reference/groups/scylla.scylladb.com/scyllaclusters.md#api-scylla-scylladb-com-scyllaclusters-v1-spec-repairs) tasks are configured for each [ScyllaCluster](https://operator.docs.scylladb.com/v1.18/api-reference/groups/scylla.scylladb.com/scyllaclusters.md) in its resource definition.

## Manual restore procedure

You can find more detail on how to perform the manual restore procedure in [this dedicated page](https://operator.docs.scylladb.com/v1.18/resources/scyllaclusters/nodeoperations/restore.md).
