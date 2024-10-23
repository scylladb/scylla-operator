# Restore from backup

This procedure will describe how to restore from backup taken using [Scylla Manager](../manager.md) to a fresh **empty** cluster of any size.

:::{warning}
Restoring schema with **ScyllaDB OS 5.4.X** or **ScyllaDB Enterprise 2024.1.X** and `consistent_cluster_management` isn’t supported.

When creating the `target` ScyllaDB cluster, configure it with `consistent_cluster_management: false`.
Refer to [API Reference](../api-reference/index.rst) to learn how to customize ScyllaDB configuration files.

When following the steps for schema restore, ensure you follow the additional steps dedicated to affected ScyllaDB versions.
:::

In the following example, the ScyllaCluster, which was used to take the backup, is called `source`. Backup will be restored into the ScyllaCluster named `target`.

::::{tab-set}
:::{tab-item} Source ScyllaCluster
```yaml
apiVersion: scylla.scylladb.com/v1
kind: ScyllaCluster
metadata:
  name: source
spec:
  agentVersion: 3.3.3
  version: 6.1.1
  developerMode: true
  backups:
  - name: foo
    location:
    - s3:source-backup
    keyspace:
    - '*'
  datacenter:
    name: us-east-1
    racks:
    - name: us-east-1a
      members: 1
      storage:
        capacity: 1Gi
      resources:
        limits:
          cpu: 1
          memory: 1Gi
```
:::
:::{tab-item} Target ScyllaCluster
```yaml
apiVersion: scylla.scylladb.com/v1
kind: ScyllaCluster
metadata:
  name: target
spec:
  agentVersion: 3.3.3
  version: 6.1.1
  developerMode: true
  datacenter:
    name: us-east-1
    racks:
    - name: us-east-1a
      members: 1
      storage:
        capacity: 1Gi
      resources:
        limits:
          cpu: 1
          memory: 1Gi
```
:::
::::

Make sure your target cluster is already registered in Scylla Manager. To get a list of all registered clusters, execute the following command:
```console
$ kubectl -n scylla-manager exec -ti deployment.apps/scylla-manager -- sctool cluster list
+--------------------------------------+---------------------------------------+---------+-----------------+
| ID                                   | Name                                  | Port    | CQL credentials |
+--------------------------------------+---------------------------------------+---------+-----------------+
| af1dd5cd-0406-4974-949f-dc9842980080 | scylla/target                        | default | set             |
| ebd82268-efb7-407e-a540-3619ae053778 | scylla/source                        | default | set             |
+--------------------------------------+---------------------------------------+---------+-----------------+
```

Identify the tag of a snapshot which you want to restore. To get a list of all available snapshots, execute following command:
```console
kubectl -n scylla-manager exec -ti deployment.apps/scylla-manager -- sctool backup list -c <CLUSTER_ID> --all-clusters -L <BACKUP_LOCATION>
```

Where:
* `CLUSTER_ID` - the name or ID of a registered cluster with access to `BACKUP_LOCATION`. 
* `BACKUP_LOCATION` - the location in which the backup is stored.

In this example, `BACKUP_LOCATION` is `s3:source-backup`. Use the name of cluster which has access to the backup location for `CLUSTER_ID`. 
In this example, it's `scylla/target`.

```console
$ kubectl -n scylla-manager exec -ti deployment.apps/scylla-manager -- sctool backup list -c scylla/target --all-clusters -L s3:source-backup
backup/ff36d7e0-af2e-458c-afe6-868e0f3396b2
Snapshots:
  - sm_20240105115931UTC (409MiB, 1 nodes)
Keyspaces:
  - system_schema (15 tables)
  - users (9 tables)

```

## Restore schema

In the below commands, we are restoring the `sm_20240105115931UTC` snapshot. Replace it with a tag of a snapshot that you want to restore.
Restoring consist of two steps. First, you'll restore the schema, and then the data.
To restore schema, create a restore task manually on target ScyllaCluster by executing following command:
```console
kubectl -n scylla-manager exec -ti deployment.apps/scylla-manager --  sctool restore -c <CLUSTER_ID> -L <BACKUP_LOCATION> -T <SNAPSHOT_TAG> --restore-schema
```

Where:
* `CLUSTER_ID` -  a name or ID of a cluster you want to restore into.
* `BACKUP_LOCATION` - the location in which the backup is stored.
* `SNAPSHOT_TAG` - a tag of a snapshot that you want to restore.

When the task is created, the command will output the ID of a restore task.
```console
$ kubectl -n scylla-manager exec -ti deployment.apps/scylla-manager -- sctool restore -c scylla/target -L s3:source-backup -T sm_20240105115931UTC --restore-schema
restore/57228c52-7cf6-4271-8c8d-d446ff160747
```

Use the following command to check progress of the restore task:
```console
$ kubectl -n scylla-manager exec -ti deployment.apps/scylla-manager -- sctool progress -c scylla/target restore/57228c52-7cf6-4271-8c8d-d446ff160747
Restore progress
Run:            0dd20cdf-abc4-11ee-951c-6e7993cf42ed
Status:         DONE - restart required (see restore docs)
Start time:     05 Jan 24 12:15:02 UTC
End time:       05 Jan 24 12:15:09 UTC
Duration:       6s
Progress:       100% | 100%
Snapshot Tag:   sm_20240105115931UTC

+---------------+-------------+----------+----------+------------+--------+
| Keyspace      |    Progress |     Size |  Success | Downloaded | Failed |
+---------------+-------------+----------+----------+------------+--------+
| system_schema | 100% | 100% | 214.150k | 214.150k |   214.150k |      0 |
+---------------+-------------+----------+----------+------------+--------+
```

As suggested in the progress output, you will need to execute a rolling restart of the ScyllaCluster.
```console
kubectl patch scyllacluster/target --type=merge -p='{"spec": {"forceRedeploymentReason": "schema restored"}}'
```

Use the following commands to wait until restart is finished:
```console
$ kubectl wait --for='condition=Progressing=False' scyllaclusters.scylla.scylladb.com/target
scyllacluster.scylla.scylladb.com/target condition met

$ kubectl wait --for='condition=Degraded=False' scyllaclusters.scylla.scylladb.com/target
scyllacluster.scylla.scylladb.com/target condition met

$ kubectl wait --for='condition=Available=True' scyllaclusters.scylla.scylladb.com/target
scyllacluster.scylla.scylladb.com/target condition met
```

:::{caution}
### Restoring schema with **ScyllaDB OS 5.4.X** or **ScyllaDB Enterprise 2024.1.X** and `consistent_cluster_management`

After you've followed the above steps with a ScyllaDB target cluster with `consistent_cluster_management` disabled, you'll need to enable Raft by configuring the target cluster with `consistent_cluster_management: true`.
Refer to [API Reference](../api-reference/index.rst) to learn how to customize ScyllaDB configuration files.

You will then need to execute a rolling restart of the ScyllaCluster for the change to take effect.
```console
kubectl patch scyllacluster/target --type=merge -p='{"spec": {"forceRedeploymentReason": "raft enabled"}}'
```

Use the following commands to wait until restart is finished:
```console
$ kubectl wait --for='condition=Progressing=False' scyllaclusters.scylla.scylladb.com/target
scyllacluster.scylla.scylladb.com/target condition met

$ kubectl wait --for='condition=Degraded=False' scyllaclusters.scylla.scylladb.com/target
scyllacluster.scylla.scylladb.com/target condition met

$ kubectl wait --for='condition=Available=True' scyllaclusters.scylla.scylladb.com/target
scyllacluster.scylla.scylladb.com/target condition met
```
:::

## Restore tables

To restore the tables content, create a restore task manually on target ScyllaCluster by executing the following command:
```console
kubectl -n scylla-manager exec -ti deployment.apps/scylla-manager -- sctool restore -c <CLUSTER_ID> -L <BACKUP_LOCATION> -T <SNAPSHOT_TAG> --restore-tables
```

Where:
* `CLUSTER_ID` - a name or ID of a cluster you want to restore into.
* `BACKUP_LOCATION` - the location in which the backup is stored.
* `SNAPSHOT_TAG` - a tag of a snapshot that you want to restore.

When the task is created, the command will output the ID of a restore task.
```console
$ kubectl -n scylla-manager exec -ti deployment.apps/scylla-manager -- sctool restore -c scylla/target -L s3:source-backup -T sm_20240105115931UTC --restore-tables
restore/63642069-bed5-4def-ba0f-68c49e47ace1
```

Use the following command to check progress of the restore task:
```console
$ kubectl -n scylla-manager exec -ti deployment.apps/scylla-manager -- sctool progress -c scylla/target restore/63642069-bed5-4def-ba0f-68c49e47ace1
Restore progress
Run:            ab015cef-abc8-11ee-9521-6e7993cf42ed
Status:         DONE
Start time:     05 Jan 24 12:48:04 UTC
End time:       05 Jan 24 12:48:15 UTC
Duration:       11s
Progress:       100% | 100%
Snapshot Tag:   sm_20240105115931UTC

+-------------+-------------+--------+---------+------------+--------+
| Keyspace    |    Progress |   Size | Success | Downloaded | Failed |
+-------------+-------------+--------+---------+------------+--------+
| users       | 100% | 100% | 409MiB |  409MiB |     409MiB |      0 |
+-------------+-------------+--------+---------+------------+--------+

Post-restore repair progress
Run:            ab015cef-abc8-11ee-9521-6e7993cf42ed
Status:         DONE
Start time:     05 Jan 24 12:48:04 UTC
End time:       05 Jan 24 12:48:15 UTC
Duration:       11s
Progress:       100%
Intensity:      1
Parallel:       0
Datacenters:
  - us-east-1

+-------------+--------------+----------+----------+
| Keyspace    |        Table | Progress | Duration |
+-------------+--------------+----------+----------+
| users       | users        | 100%     | 0s       |
+-------------+--------------+----------+----------+

```
