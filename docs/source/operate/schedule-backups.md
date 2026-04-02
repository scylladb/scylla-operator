# Schedule backups

Configure object storage credentials and schedule automated backups of your ScyllaDB data using ScyllaDB Manager.

Backup is provided by [ScyllaDB Manager](../understand/manager.md).
The Manager Agent sidecar running on each ScyllaDB pod uploads snapshots to your object storage bucket.

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

### Google Cloud Storage

Create a Kubernetes Secret containing a GCS service account key:

```bash
kubectl -n scylla create secret generic scylla-agent-backup-secret \
  --from-file=gcs-service-account.json=/path/to/service-account-key.json
```

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

### Workload identity

On managed Kubernetes platforms, you can configure the Manager Agent to authenticate using workload identity, eliminating the need to manage static credential files.

#### GKE (Workload Identity Federation)

1. Create a Google Service Account (GSA) with the `storage.objectAdmin` role on the backup bucket:

   ```bash
   gcloud iam service-accounts create <gsa-name> --project=<project-id>
   gsutil iam ch serviceAccount:<gsa-name>@<project-id>.iam.gserviceaccount.com:objectAdmin gs://<bucket-name>
   ```

2. Annotate the ScyllaDB Manager Agent Kubernetes Service Account with the GSA email:

   ```bash
   kubectl annotate serviceaccount scylla-manager-controller \
     -n scylla-manager \
     iam.gke.io/gcp-service-account=<gsa-name>@<project-id>.iam.gserviceaccount.com
   ```

3. Bind the Kubernetes Service Account to the GSA:

   ```bash
   gcloud iam service-accounts add-iam-policy-binding <gsa-name>@<project-id>.iam.gserviceaccount.com \
     --role roles/iam.workloadIdentityUser \
     --member "serviceAccount:<project-id>.svc.id.goog[scylla-manager/scylla-manager-controller]"
   ```

4. Set `backupLocation` in the `ScyllaDBManagerTask` backup spec to `gcs:<bucket-name>` without embedding credentials.

#### EKS (IAM Roles for Service Accounts)

1. Create an IAM policy granting the required S3 permissions on the backup bucket:

   ```json
   {
     "Version": "2012-10-17",
     "Statement": [
       {
         "Effect": "Allow",
         "Action": ["s3:GetObject", "s3:PutObject", "s3:DeleteObject", "s3:ListBucket"],
         "Resource": [
           "arn:aws:s3:::<bucket-name>",
           "arn:aws:s3:::<bucket-name>/*"
         ]
       }
     ]
   }
   ```

2. Create an IAM role with a trust policy for the EKS OIDC provider, scoped to `system:serviceaccount:scylla-manager:scylla-manager-controller`:

   ```json
   {
     "Version": "2012-10-17",
     "Statement": [
       {
         "Effect": "Allow",
         "Principal": {
           "Federated": "arn:aws:iam::<account-id>:oidc-provider/<oidc-provider-url>"
         },
         "Action": "sts:AssumeRoleWithWebIdentity",
         "Condition": {
           "StringEquals": {
             "<oidc-provider-url>:sub": "system:serviceaccount:scylla-manager:scylla-manager-controller"
           }
         }
       }
     ]
   }
   ```

3. Annotate the Kubernetes Service Account with the IAM role ARN:

   ```bash
   kubectl annotate serviceaccount scylla-manager-controller \
     -n scylla-manager \
     eks.amazonaws.com/role-arn=arn:aws:iam::<account-id>:role/<role-name>
   ```

4. Set `backupLocation` to `s3:<bucket-name>` in the backup task spec.

:::{note}
After annotating the Service Account, the Manager pod must be restarted for the annotation to take effect:

```bash
kubectl rollout restart deployment/scylla-manager -n scylla-manager
```
:::

## Schedule a backup

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

:::{note}
In multi-DC clusters using multiple `ScyllaCluster` resources, configure backup tasks independently on each datacenter's `ScyllaCluster` resource.
:::

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

## Related pages

- [Restore from backup](restore-from-backup.md) — restore a ScyllaDB cluster from a Manager backup snapshot.
- [ScyllaDB Manager](../understand/manager.md) — how Manager integrates with the Operator, task synchronisation, and security model.
- [ScyllaDB Manager Backup Documentation](https://manager.docs.scylladb.com/stable/backup/) — upstream Manager backup reference.
