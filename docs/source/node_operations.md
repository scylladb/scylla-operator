# Node operations using Scylla Operator

### Upgrading version of Scylla

To upgrade Scylla version using Operator user have to modify existing ScyllaCluster definition.

In this example cluster will be upgraded to `2020.1.0` version.
```bash
kubectl -n scylla patch ScyllaCluster simple-cluster  -p '{"spec":{"version": "4.2.2}}' --type=merge
```

Operator supports two types of version upgrades:
1. Patch upgrade
1. Generic upgrade


**Patch upgrade**

Patch upgrade is executed when only patch version change is detected according to [semantic versioning format](https://semver.org/).
Procedure simply rolls out a restart of whole cluster and upgrades Scylla container image for each node one by one.

Example: `4.0.0 -> 4.0.1`

**Generic upgrade**

Generic upgrades are executed for the non patch version changes.

Example: `4.0.0 -> 2020.1.0` or `4.0.0 -> 4.1.0` or even `4.0.0 -> nightly`

User can observe current state of upgrade in ScyllaCluster status.
```bash
kubectl -n scylla describe ScyllaCluster simple-cluster
[...]
Status:
  Racks:
    us-east-1a:
      Members:        3
      Ready Members:  3
      Version:        4.1.9
  Upgrade:
    Current Node:         simple-cluster-us-east-1-us-east-1a-2
    Current Rack:         us-east-1a
    Data Snapshot Tag:    so_data_20201228135002UTC
    From Version:         4.1.9
    State:                validate_upgrade
    System Snapshot Tag:  so_system_20201228135002UTC
    To Version:           4.2.2
```

Each upgrade begins with taking a snapshot of `system` and `system_schema` keyspaces on all nodes in parallel.
Name of this snapshot tag is saved in upgrade status under `System Snapshot Tag`.

Before nodes in rack are upgraded, underlying StatefulSet is changed to use `OnDelete` UpgradeStrategy.
This allows Operator have a full control over when Pod image is changed.

When a node is being upgraded, [maintenance mode](#maintenance-mode) is enabled, then the node is drained and snapshot of all data keyspaces is taken.
Snapshot tag is saved under `Data Snapshot Tag` and is the same for all nodes during the procedure.
Once everything is set up, maintenance mode is disabled and Scylla Pod is deleted. Underlying StatefulSet will bring up a new
Pod with upgraded version.
Once Pod will become ready, data snapshot from this particular node is removed, and Operator moves to next node.

Once every rack is upgraded, system snapshot is removed from all nodes in parallel and previous StatefulSet UpgradeStrategy is restored.
At this point, all your nodes should be already in desired version.

Current state of upgrade can be traced using `Current Node`, `Current Rack` and `State` status fields.
* `Current Node` shows which node is being upgraded.
* `Current Rack` displays which rack is being upgraded.
* `State` contain information at which stage upgrade is.

`State` can have following values:
* `begin_upgrade` - upgrade is starting
* `check_schema_agreement` - Operator waits until all nodes reach schema agreement. It waits for it for 1 minute, prints an error log message and check is retried.
* `create_system_backup` - system keyspaces snapshot is being taken
* `find_next_rack` - Operator finds out which rack must be upgraded next, decision is saved in `Current Rack`
* `upgrade_image_in_pod_spec` - Image and UpgradeStrategy is upgraded in underlying StatefulSet
* `find_next_node` - Operator finds out which node must be upgraded next, decision is saved in `Current Node`
* `enable_maintenance_mode` - maintenance mode is being enabled
* `drain_node` - node is being drained
* `backup_data` - snapshot of data keyspaces is being taken
* `disable_maintenance_mode` - maintenance mode is being disabled
* `delete_pod` - Scylla Pod is being deleted
* `validate_upgrade` - Operator validates if new pod enters Ready state and if Scylla version is upgraded
* `clear_data_backup` - snapshot of data keyspaces is being removed
* `clear_system_backup` - snapshot of system keyspaces is being removed
* `restore_upgrade_strategy` - restore UpgradeStrategy in underlying StatefulSet
* `finish_upgrade` - upgrade cleanup

**Recovering from upgrade failure**

Upgrade may get stuck on `validate_upgrade` stage. This happens when Scylla Pod refuses to properly boot up.

To continue with upgrade, first turn off operator by scaling Operator replicas to zero:
```bash
kubectl -n scylla-operator-system scale sts scylla-operator-controller-manager --replicas=0
```
Then user have to manually resolve issue with Scylla by checking what is the root cause of a failure in Scylla container logs.
If needed data and system keyspaces SSTable snapshots are available on the node. You can check ScyllaCluster status for their names.

Once issue is resolved and Scylla Pod is up and running (Pod is in Ready state), scale Operator back to one replica:
```bash
kubectl -n scylla-operator-system scale sts scylla-operator-controller-manager --replicas=1
```

Operator should continue upgrade process from where it left off.

### Replacing a Scylla node
In the case of a host failure, it may not be possible to bring back the node to life. 

Replace dead node operation will cause the other nodes in the cluster to stream data to the node that was replaced. 
This operation can take some time (depending on the data size and network bandwidth).

_This procedure is for replacing one dead node. To replace more than one dead node, run the full procedure to completion one node at a time_

**Procedure**

1. Verify the status of the node using `nodetool status` command, the node with status DN is down and need to be replaced
    ```bash
    kubectl -n scylla exec -ti simple-cluster-us-east-1-us-east-1a-0 -c scylla -- nodetool status
    Datacenter: us-east-1
    =====================
    Status=Up/Down
    |/ State=Normal/Leaving/Joining/Moving
    --  Address        Load       Tokens       Owns    Host ID                               Rack
    UN  10.43.125.110  74.63 KB   256          ?       8ebd6114-969c-44af-a978-87a4a6c65c3e  us-east-1a
    UN  10.43.231.189  91.03 KB   256          ?       35d0cb19-35ef-482b-92a4-b63eee4527e5  us-east-1a
    DN  10.43.43.51    74.77 KB   256          ?       1ffa7a82-c41c-4706-8f5f-4d45a39c7003  us-east-1a
    ```
1. Identify service which is bound to down node by checking IP address
    ```bash
    kubectl -n scylla get svc 
    NAME                                    TYPE        CLUSTER-IP      EXTERNAL-IP   PORT(S)                                                           AGE
    simple-cluster-client                   ClusterIP   None            <none>        9180/TCP                                                          3h12m
    simple-cluster-us-east-1-us-east-1a-0   ClusterIP   10.43.231.189   <none>        7000/TCP,7001/TCP,7199/TCP,10001/TCP,9042/TCP,9142/TCP,9160/TCP   3h12m
    simple-cluster-us-east-1-us-east-1a-1   ClusterIP   10.43.125.110   <none>        7000/TCP,7001/TCP,7199/TCP,10001/TCP,9042/TCP,9142/TCP,9160/TCP   3h11m
    simple-cluster-us-east-1-us-east-1a-2   ClusterIP   10.43.43.51     <none>        7000/TCP,7001/TCP,7199/TCP,10001/TCP,9042/TCP,9142/TCP,9160/TCP   3h5m
    ```
1. Drain node which we would like to replace using. **This command may delete your data from local disks attached to given node!**
    ```bash
    kubectl drain gke-scylla-demo-default-pool-b4b390a1-6j12 --ignore-daemonsets --delete-local-data 
    ```

    Pod which will be replaced should enter the `Pending` state
    ```bash
    kubectl -n scylla get pods        
    NAME                                    READY   STATUS    RESTARTS   AGE
    simple-cluster-us-east-1-us-east-1a-0   2/2     Running   0          3h21m
    simple-cluster-us-east-1-us-east-1a-1   2/2     Running   0          3h19m
    simple-cluster-us-east-1-us-east-1a-2   0/2     Pending   0          8m14s
    ```
1. To being node replacing, add `scylla/replace=""` label to service bound to pod we are replacing. 
    ```bash
    kubectl -n scylla label svc simple-cluster-us-east-1-us-east-1a-2 scylla/replace=""
    ```
    Your failed Pod should be recreated on available k8s node
    ```bash
    kubectl -n scylla get pods        
    NAME                                    READY   STATUS    RESTARTS   AGE
    simple-cluster-us-east-1-us-east-1a-0   2/2     Running   0          3h27m
    simple-cluster-us-east-1-us-east-1a-1   2/2     Running   0          3h25m
    simple-cluster-us-east-1-us-east-1a-2   1/2     Running   0          9s
    ```
   Because other nodes in cluster must stream data to new node this operation might take some time depending on how much data your cluster stores.
   After bootstraping is over, your new Pod should be ready to go.
   Old one shouldn't be no longer visible in `nodetool status`
   ```bash
   kubectl -n scylla exec -ti simple-cluster-us-east-1-us-east-1a-0 -c scylla -- nodetool status
   Datacenter: us-east-1
   =====================
   Status=Up/Down
   |/ State=Normal/Leaving/Joining/Moving
   --  Address        Load       Tokens       Owns    Host ID                               Rack
   UN  10.43.125.110  74.62 KB   256          ?       8ebd6114-969c-44af-a978-87a4a6c65c3e  us-east-1a
   UN  10.43.231.189  91.03 KB   256          ?       35d0cb19-35ef-482b-92a4-b63eee4527e5  us-east-1a
   UN  10.43.191.172  74.77 KB   256          ?       1ffa7a82-c41c-4706-8f5f-4d45a39c7003  us-east-1a
   ```
1. Run the repair on the cluster to make sure that the data is synced with the other nodes in the cluster. You can use [Scylla Manager](manager.md) to run the repair.

### Automatic cleanup and replacement in case when k8s node is lost

In case when your k8s cluster loses one of the nodes due to incident or explicit removal, Scylla Pods may become unschedulable due to PVC node affinity.

When `automaticOrphanedNodeCleanup` flag is enabled in your ScyllaCluster, Scylla Operator will perform automatic 
node replacement of a Pod which lost his bound resources.

### Maintenance mode

When maintenance mode is enabled, readiness probe of Scylla Pod will always return failure and liveness probe will always succeed. This causes that Pod under maintenance
is being removed from K8s Load Balancer and DNS registry but Pod itself stays alive.

This allows the Scylla Operator to interact with Scylla and Scylla dependencies inside the Pod.
For example user may turn off Scylla process, do something with the filesystem and bring the process back again.

To enable maintenance mode add `scylla/node-maintenance` label to service in front of Scylla Pod.

```bash
kubectl -n scylla label svc simple-cluster-us-east1-b-us-east1-2 scylla/node-maintenance=""
```

To disable, simply remove this label from service.

```bash
kubectl -n scylla label svc simple-cluster-us-east1-b-us-east1-2 scylla/node-maintenance-
```


### Restore from backup

This procedure will describe how to restore from backup taken using [Scylla Manager](manager.md) to a fresh **empty** cluster of any size.

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
kubectl -n scylla-manager-system exec scylla-manager-controller-0 -- sctool backup files -c simple-cluster -L s3:backups -T sm_20201228145917UTC > backup_files.out
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
