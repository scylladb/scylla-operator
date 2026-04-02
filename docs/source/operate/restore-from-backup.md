# Restore from backup

Restore a ScyllaDB cluster from a backup snapshot created by ScyllaDB Manager.
Restore is performed manually by running `sctool` commands inside the ScyllaDB Manager pod into a fresh, empty target cluster.

:::{warning}
Restoring data into a cluster that already contains data is not supported.
Always restore to an empty target cluster.
:::

## Step 1: Create the target cluster

Deploy a new ScyllaDB cluster with the same ScyllaDB version and configuration as the source.
The target cluster does not need to have the same number of nodes — ScyllaDB Manager handles the data distribution.

Ensure the target cluster:
- Is registered with ScyllaDB Manager (this happens automatically unless Manager integration is disabled).
- Has access to the backup bucket (same object storage credentials as the source).

Wait for the target cluster to be fully available:

```bash
kubectl -n scylla wait --timeout=10m --for='condition=Available' scyllaclusters.scylla.scylladb.com/target
```

## Step 2: Restore the schema

Restoring consists of two phases: first the schema, then the table data.

Create a schema restore task:

```bash
kubectl -n scylla-manager exec -it deployment.apps/scylla-manager -- \
  sctool restore -c <TARGET_CLUSTER_ID> -L <BACKUP_LOCATION> -T <SNAPSHOT_TAG> --restore-schema
```

Monitor progress:

```console
$ kubectl -n scylla-manager exec -it deployment.apps/scylla-manager -- \
    sctool restore -c scylla/target -L s3:my-backup-bucket -T sm_20240105115931UTC --restore-schema
restore/57228c52-7cf6-4271-8c8d-d446ff160747

$ kubectl -n scylla-manager exec -it deployment.apps/scylla-manager -- \
    sctool progress -c scylla/target restore/57228c52-7cf6-4271-8c8d-d446ff160747
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

For ScyllaDB 5.4 / ScyllaDB Enterprise 2024.1 and older, a rolling restart is required after the schema restore completes.
Trigger it by patching the cluster:

```bash
kubectl -n scylla patch scyllacluster/target --type=merge \
  -p='{"spec": {"forceRedeploymentReason": "schema restored"}}'
```

Wait for the restart to finish:

```bash
kubectl -n scylla wait --timeout=10m --for='condition=Progressing=False' scyllaclusters.scylla.scylladb.com/target
kubectl -n scylla wait --timeout=10m --for='condition=Degraded=False' scyllaclusters.scylla.scylladb.com/target
kubectl -n scylla wait --timeout=10m --for='condition=Available=True' scyllaclusters.scylla.scylladb.com/target
```

:::{note}
For ScyllaDB 2024.2 and newer, a rolling restart after schema restore is **not required**.
See [New Restore Schema Documentation](https://manager.docs.scylladb.com/stable/restore/restore-schema.html) for details.
:::

## Step 3: Restore the table data

After the schema is in place, restore the table data:

```bash
kubectl -n scylla-manager exec -it deployment.apps/scylla-manager -- \
  sctool restore -c <TARGET_CLUSTER_ID> -L <BACKUP_LOCATION> -T <SNAPSHOT_TAG> --restore-tables
```

Monitor progress:

```console
$ kubectl -n scylla-manager exec -it deployment.apps/scylla-manager -- \
    sctool restore -c scylla/target -L s3:my-backup-bucket -T sm_20240105115931UTC --restore-tables
restore/63642069-bed5-4def-ba0f-68c49e47ace1

$ kubectl -n scylla-manager exec -it deployment.apps/scylla-manager -- \
    sctool progress -c scylla/target restore/63642069-bed5-4def-ba0f-68c49e47ace1
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

ScyllaDB Manager automatically runs a post-restore repair to ensure data consistency.

## Step 4: Verify the restore

Connect to the target cluster and verify your data is present:

```bash
kubectl -n scylla exec -it target-us-east-1-us-east-1a-0 -c scylla -- cqlsh
```

```sql
DESCRIBE KEYSPACES;
SELECT COUNT(*) FROM users.users;
```

## Key considerations

| Consideration | Detail |
|---|---|
| Empty target cluster | Restore must be performed on a fresh cluster with no existing user data. |
| Same ScyllaDB version | The target cluster should run the same ScyllaDB version as the source to avoid compatibility issues. |
| Bucket access | Both the source and target clusters need credentials to access the same backup bucket. |
| Cross-cluster restore | You can restore into a cluster with a different name, namespace, or even a different Kubernetes cluster, as long as it has access to the backup location. |
| Backup retention | Set `retention` to control how many snapshots are kept. Older snapshots are automatically pruned. |
| Schema and data separation | Schema and table data are restored separately. Always restore the schema first. |
| Post-restore repair | Manager automatically runs a repair after table data restore to ensure consistency. |

## Related pages

- [Schedule backups](schedule-backups.md) — configure credentials and schedule automated backup tasks.
- [ScyllaDB Manager](../understand/manager.md) — how Manager integrates with the Operator, task synchronisation, and security model.
- [Rolling restart](perform-rolling-restart.md) — how to trigger a rolling restart (required after schema restore on older ScyllaDB versions).
- [ScyllaDB Manager Restore Documentation](https://manager.docs.scylladb.com/stable/restore/) — upstream Manager restore reference.
