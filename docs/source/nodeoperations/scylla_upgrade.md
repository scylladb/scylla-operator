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
kubectl -n scylla-operator scale deployment.apps/scylla-operator --replicas=0
```
Then user have to manually resolve issue with Scylla by checking what is the root cause of a failure in Scylla container logs.
If needed data and system keyspaces SSTable snapshots are available on the node. You can check ScyllaCluster status for their names.

Once issue is resolved and Scylla Pod is up and running (Pod is in Ready state), scale Operator back to two replicas:
```bash
kubectl -n scylla-operator scale deployment.apps/scylla-operator --replicas=2
```

Operator should continue upgrade process from where it left off.
