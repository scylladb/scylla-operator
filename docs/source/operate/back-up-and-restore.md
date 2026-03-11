# Back up and restore

Schedule automated backups of your ScyllaDB data to object storage and restore from a snapshot to a new cluster.

Backup and restore are provided by [ScyllaDB Manager](../understand/manager.md).
The Manager Agent sidecar running on each ScyllaDB pod uploads snapshots to your object storage bucket.
Restore is performed by executing `sctool` commands against the Manager deployment.

## Prerequisites

- A running ScyllaDB cluster managed by ScyllaDB Operator.
- ScyllaDB Manager installed and the cluster registered with Manager.
  See [ScyllaDB Manager architecture](../understand/manager.md) for how registration works.
- An object storage bucket (Amazon S3 or Google Cloud Storage) accessible from the ScyllaDB pods.
- Object storage credentials configured on the Manager Agent (see [Configure object storage credentials](#configure-object-storage-credentials) below).

## Configure object storage credentials

The Manager Agent running on each ScyllaDB pod needs credentials to access the object storage bucket.
You provide these by mounting a credentials file into the Agent container.

### Amazon S3

Create a Kubernetes Secret containing AWS credentials:

```bash
kubectl -n scylla create secret generic scylla-agent-backup-secret \
  --from-file=credentials=/path/to/aws-credentials
```

The credentials file should follow the standard AWS credentials format:

```ini
[default]
aws_access_key_id = AKIAIOSFODNN7EXAMPLE
aws_secret_access_key = wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
```

:::::{tabs}
::::{group-tab} ScyllaCluster

Mount the Secret into the Agent container using `volumes` and `agentVolumeMounts` on the rack spec:

```yaml
apiVersion: scylla.scylladb.com/v1
kind: ScyllaCluster
metadata:
  name: scylla
  namespace: scylla
spec:
  datacenter:
    name: us-east-1
    racks:
      - name: us-east-1a
        members: 3
        storage:
          capacity: 500Gi
        resources:
          limits:
            cpu: 4
            memory: 8Gi
        volumes:
          - name: backup-credentials
            secret:
              secretName: scylla-agent-backup-secret
        agentVolumeMounts:
          - name: backup-credentials
            mountPath: /var/lib/scylla-manager/.aws
            readOnly: true
```
::::
::::{group-tab} ScyllaDBDatacenter

Mount the Secret into the Agent container using `scyllaDBManagerAgent.volumes` and `scyllaDBManagerAgent.volumeMounts` on the rack template:

```yaml
apiVersion: scylla.scylladb.com/v1alpha1
kind: ScyllaDBDatacenter
metadata:
  name: scylla
  namespace: scylla
spec:
  rackTemplate:
    scyllaDBManagerAgent:
      volumes:
        - name: backup-credentials
          secret:
            secretName: scylla-agent-backup-secret
      volumeMounts:
        - name: backup-credentials
          mountPath: /var/lib/scylla-manager/.aws
          readOnly: true
    nodes: 3
    scyllaDB:
      storage:
        capacity: 500Gi
    resources:
      limits:
        cpu: 4
        memory: 8Gi
  racks:
    - name: us-east-1a
```
::::
:::::

### Google Cloud Storage

Create a Kubernetes Secret containing a GCS service account key:

```bash
kubectl -n scylla create secret generic scylla-agent-backup-secret \
  --from-file=gcs-service-account.json=/path/to/service-account-key.json
```

:::::{tabs}
::::{group-tab} ScyllaCluster

```yaml
apiVersion: scylla.scylladb.com/v1
kind: ScyllaCluster
metadata:
  name: scylla
  namespace: scylla
spec:
  datacenter:
    name: us-east-1
    racks:
      - name: us-east-1a
        members: 3
        storage:
          capacity: 500Gi
        resources:
          limits:
            cpu: 4
            memory: 8Gi
        volumes:
          - name: backup-credentials
            secret:
              secretName: scylla-agent-backup-secret
        agentVolumeMounts:
          - name: backup-credentials
            mountPath: /etc/scylla-manager-agent/gcs
            readOnly: true
```

Additionally, you must tell the Agent where to find the GCS credentials file.
Create a ConfigMap with a custom agent configuration:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: scylla-agent-config
  namespace: scylla
data:
  scylla-manager-agent.yaml: |
    gcs:
      service_account_file: /etc/scylla-manager-agent/gcs/gcs-service-account.json
```

Then reference it on the rack:

```yaml
scyllaAgentConfig: scylla-agent-config
```
::::
::::{group-tab} ScyllaDBDatacenter

```yaml
apiVersion: scylla.scylladb.com/v1alpha1
kind: ScyllaDBDatacenter
metadata:
  name: scylla
  namespace: scylla
spec:
  rackTemplate:
    scyllaDBManagerAgent:
      volumes:
        - name: backup-credentials
          secret:
            secretName: scylla-agent-backup-secret
      volumeMounts:
        - name: backup-credentials
          mountPath: /etc/scylla-manager-agent/gcs
          readOnly: true
      customConfigSecretRef: scylla-agent-gcs-config
    nodes: 3
    scyllaDB:
      storage:
        capacity: 500Gi
    resources:
      limits:
        cpu: 4
        memory: 8Gi
  racks:
    - name: us-east-1a
```

The `scylla-agent-gcs-config` Secret must contain a `scylla-manager-agent.yaml` key with:

```yaml
gcs:
  service_account_file: /etc/scylla-manager-agent/gcs/gcs-service-account.json
```
::::
:::::

:::{note}
<!-- TODO: Document workload identity (IAM roles for service accounts) for EKS and GKE. Tracked in https://github.com/scylladb/scylla-operator/issues/1697. -->
The examples above use static credentials.
On EKS (IAM Roles for Service Accounts) and GKE (Workload Identity), you can instead configure the Agent to use workload identity, eliminating the need to manage static credential files.
This is not yet documented — see [#1697](https://github.com/scylladb/scylla-operator/issues/1697).
:::

## Schedule a backup

:::::{tabs}
::::{group-tab} ScyllaCluster

Add a backup task to the `spec.backups` array:

```yaml
apiVersion: scylla.scylladb.com/v1
kind: ScyllaCluster
metadata:
  name: scylla
  namespace: scylla
spec:
  backups:
    - name: daily-backup
      location:
        - s3:my-backup-bucket
      keyspace:
        - '*'
      retention: 7
      cron: "0 2 * * *"
      timezone: UTC
  datacenter:
    name: us-east-1
    racks:
      - name: us-east-1a
        members: 3
        storage:
          capacity: 500Gi
        resources:
          limits:
            cpu: 4
            memory: 8Gi
```

The Operator translates each backup entry into a `ScyllaDBManagerTask` resource and synchronises it with Manager.
::::
::::{group-tab} ScyllaDBDatacenter / ScyllaDBCluster

Create a `ScyllaDBManagerTask` resource referencing your cluster:

```yaml
apiVersion: scylla.scylladb.com/v1alpha1
kind: ScyllaDBManagerTask
metadata:
  name: daily-backup
  namespace: scylla
spec:
  scyllaDBClusterRef:
    kind: ScyllaDBDatacenter
    name: scylla
  type: Backup
  backup:
    location:
      - s3:my-backup-bucket
    keyspace:
      - '*'
    retention: 7
    cron: "0 2 * * *"
```

The `scyllaDBClusterRef` supports both `ScyllaDBDatacenter` and `ScyllaDBCluster` kinds.
::::
:::::

### Backup task fields

| Field | Description | Default |
|---|---|---|
| `location` | Backup location in `[<dc>:]<provider>:<bucket>` format. Supported providers: `s3`, `gcs`. | *(required)* |
| `keyspace` | Glob patterns for keyspaces/tables to include or exclude, e.g. `'*'`, `'!system_*'`. | All keyspaces |
| `dc` | Glob patterns for datacenters to include or exclude from backup. | All DCs |
| `retention` | Number of backup snapshots to retain. | `3` |
| `cron` | Schedule in cron format. Supports `@daily`, `@weekly`, `@every 6h`, etc. | *(no automatic schedule)* |
| `rateLimit` | Upload rate limit in MiB/s, format `[<dc>:]<limit>`. | `100` |
| `snapshotParallel` | Number of nodes that can take snapshots in parallel, format `[<dc>:]<limit>`. | Unlimited |
| `uploadParallel` | Number of nodes that can upload in parallel, format `[<dc>:]<limit>`. | Unlimited |

### Verify backup task status

For ScyllaCluster, check the `.status.backups` field:

```bash
kubectl -n scylla get scyllacluster/scylla -o jsonpath='{.status.backups}' | jq .
```

For ScyllaDBManagerTask, check the task conditions:

```bash
kubectl -n scylla get scylladbmanagertask/daily-backup -o jsonpath='{.status.conditions}' | jq .
```

## List backup snapshots

To list available snapshots, execute `sctool backup list` inside the Manager pod:

```bash
kubectl -n scylla-manager exec -it deployment.apps/scylla-manager -- \
  sctool backup list -c <CLUSTER_ID> --all-clusters -L <BACKUP_LOCATION>
```

Where:
- `CLUSTER_ID` — the name or ID of a registered cluster with access to the backup location.
  Use `sctool cluster list` to find it.
- `BACKUP_LOCATION` — the location where backups are stored, e.g. `s3:my-backup-bucket`.

Example:

```console
$ kubectl -n scylla-manager exec -it deployment.apps/scylla-manager -- \
    sctool cluster list
+--------------------------------------+---------------------------------------+---------+-----------------+
| ID                                   | Name                                  | Port    | CQL credentials |
+--------------------------------------+---------------------------------------+---------+-----------------+
| af1dd5cd-0406-4974-949f-dc9842980080 | scylla/scylla                        | default | set             |
+--------------------------------------+---------------------------------------+---------+-----------------+

$ kubectl -n scylla-manager exec -it deployment.apps/scylla-manager -- \
    sctool backup list -c scylla/scylla --all-clusters -L s3:my-backup-bucket
backup/ff36d7e0-af2e-458c-afe6-868e0f3396b2
Snapshots:
  - sm_20240105115931UTC (409MiB, 1 nodes)
Keyspaces:
  - system_schema (15 tables)
  - users (9 tables)
```

Note the snapshot tag (e.g. `sm_20240105115931UTC`) — you will need it when restoring.

## Restore from backup

Restore is performed manually by running `sctool` commands inside the ScyllaDB Manager pod.
You restore into a fresh, empty ScyllaDB cluster (the "target" cluster).

:::{warning}
Restoring data into a cluster that already contains data is not supported.
Always restore to an empty target cluster.
:::

### Step 1: Create the target cluster

Deploy a new ScyllaDB cluster with the same ScyllaDB version and configuration as the source.
The target cluster does not need to have the same number of nodes — ScyllaDB Manager handles the data distribution.

Ensure the target cluster:
- Is registered with ScyllaDB Manager (this happens automatically unless Manager integration is disabled).
- Has access to the backup bucket (same object storage credentials as the source).

Wait for the target cluster to be fully available:

```bash
kubectl -n scylla wait --timeout=10m --for='condition=Available' scyllaclusters.scylla.scylladb.com/target
```

### Step 2: Restore the schema

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

### Step 3: Restore the table data

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

### Step 4: Verify the restore

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

- [ScyllaDB Manager](../understand/manager.md) — how Manager integrates with the Operator, task synchronisation, and security model
- [Rolling restart](perform-rolling-restart.md) — how to trigger a rolling restart (required after schema restore on older ScyllaDB versions)
- [ScyllaDB Manager Backup Documentation](https://manager.docs.scylladb.com/stable/backup/) — upstream Manager backup reference
- [ScyllaDB Manager Restore Documentation](https://manager.docs.scylladb.com/stable/restore/) — upstream Manager restore reference
