### Restore from backup

This procedure will describe how to restore from backup taken using [Scylla Manager](../manager.md) to a fresh **empty** cluster of any size.

First identify to which snapshot you want to restore. To get list of available snapshot execute following command on Scylla Manager Pod.
```bash
sctool backup list -c <CLUSTER_ID> --all-clusters -L <BACKUP_LOCATION>
```

Where:
* `CLUSTER_ID` - is a name of a cluster or ID under which ScyllaCluster was registered. You can find it in ScyllaCluster Status.
* `BACKUP_LOCATION` - is a location where backup is stored. For example, for bucket called `backups` stored in AWS S3, location is `s3:backups`.

```bash
sctool backup list -c simple-cluster --all-clusters -L s3:backups
Snapshots:
  - sm_20201227144037UTC (409MiB)
  - sm_20201228145917UTC (434MiB)
Keyspaces:
  - users (9 tables)
  - system_auth (2 tables)
  - system_distributed (3 tables)
  - system_schema (13 tables)
  - system_traces (5 tables)
```

To get the list of files use:

```bash
sctool backup files -c <CLUSTER_ID> -L <BACKUP_LOCATION> -T <SNAPSHOT_TAG>
```

Where:
* `SNAPSHOT_TAG` - name of snapshot you want to restore.

Before we start restoring the data, we have to restore the schema.
The first output line is a path to schemas archive, for example:
```bash
s3://backups/backup/schema/cluster/ed63b474-2c05-4f4f-b084-94541dd86e7a/task_287791d9-c257-4850-aef5-7537d6e69d90_tag_sm_20201228145917UTC_schema.tar.gz      ./
```

To download this archive you can use AWS CLI tool `aws s3 cp`.

This archive contains a single CQL file for each keyspace in the backup.
```bash
tar -ztvf task_287791d9-c257-4850-aef5-7537d6e69d90_tag_sm_20201228145917UTC_schema.tar.gz
-rw------- 0/0           12671 2020-12-28 13:17 users.cql
-rw------- 0/0            2216 2020-12-28 13:17 system_auth.cql
-rw------- 0/0             921 2020-12-28 13:17 system_distributed.cql
-rw------- 0/0           12567 2020-12-28 13:17 system_schema.cql
-rw------- 0/0            4113 2020-12-28 13:17 system_traces.cql
```

Extract this archive and copy each schema file to one of the cluster Pods by:
```bash
kubectl -n scylla cp users.cql simple-cluster-us-east-1-us-east-1a-0:/tmp/users.cql -c scylla
```

To import schema simply execute:
```bash
kubectl -n scylla exec simple-cluster-us-east-1-us-east-1a-0 -c scylla -- cqlsh -f /tmp/users.cql
```

Once the schema is recreated we can proceed to downloading data files.

First let's save a list of snapshot files to file called `backup_files.out`:

```bash
kubectl -n scylla-manager exec deployment.apps/scylla-manager-controller -- sctool backup files -c simple-cluster -L s3:backups -T sm_20201228145917UTC > backup_files.out
```

We will be using `sstableloader` to restore data. `sstableloader` needs a specific directory structure to work namely: `<keyspace>/<table>/<contents>`
To create this directory structure and download all the files execute these commands:
```bash
mkdir snapshot
cd snapshot
# Create temporary directory structure.
cat ../backup_files.out | awk '{print $2}' | xargs mkdir -p
# Download snapshot files.
cat ../backup_files.out | xargs -n2 aws s3 cp
```

To load data into cluster pass cluster address to `sstableloader` together with path to data files and credentials:
```bash
sstableloader -d 'simple-cluster-us-east-1-us-east-1a-0.scylla.svc,simple-cluster-us-east-1-us-east-1a-1.scylla.svc,simple-cluster-us-east-1-us-east-1a-2.scylla.svc' ./users/data_0 --username scylla --password <password>
```

Depending on how big is your data set, this operation may take some time.
Once it finishes, data from the snapshot is restored and you may clean up the host.
