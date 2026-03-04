# nodetool alternatives

This page is for ScyllaDB administrators transitioning to Kubernetes. It explains why direct `nodetool` operations that change cluster state must not be used with the Operator, and what to do instead.

:::{note}
ScyllaDB's `nodetool` is a distinct implementation from Apache Cassandra's — it communicates with ScyllaDB via the [REST API](https://docs.scylladb.com/stable/operating-scylla/rest.html) (not JMX), though many command names are the same.
:::

## Why this matters

The Operator reconciles cluster state continuously. Any out-of-band change to cluster membership or topology creates a mismatch between the Operator's expected state and reality, leading to stuck rollouts, failed replacements, or data loss.

## Read-only commands are always safe

If a `nodetool` command is read-only, it is always safe to use via `kubectl exec`. Examples:

[`status`](https://docs.scylladb.com/stable/operating-scylla/nodetool-commands/status.html), [`gossipinfo`](https://docs.scylladb.com/stable/operating-scylla/nodetool-commands/gossipinfo.html), [`info`](https://docs.scylladb.com/stable/operating-scylla/nodetool-commands/info.html), [`ring`](https://docs.scylladb.com/stable/operating-scylla/nodetool-commands/ring.html), [`cfstats`](https://docs.scylladb.com/stable/operating-scylla/nodetool-commands/cfstats.html) / [`tablestats`](https://docs.scylladb.com/stable/operating-scylla/nodetool-commands/tablestats.html), [`compactionstats`](https://docs.scylladb.com/stable/operating-scylla/nodetool-commands/compactionstats.html), [`toppartitions`](https://docs.scylladb.com/stable/operating-scylla/nodetool-commands/toppartitions.html)

```shell
kubectl exec -it <pod-name> -c scylla -- nodetool status
```

## State-changing commands

If a command changes cluster state, consult the table below.

### High risk — use Operator alternatives

These commands change cluster membership or topology. Using them directly will desync the Operator's state.

:::{list-table}
:widths: 18 35 47
:header-rows: 1

* - Command
  - Risk
  - Operator alternative
* - [`decommission`](https://docs.scylladb.com/stable/operating-scylla/nodetool-commands/decommission.html)
  - Desyncs StatefulSet replica count and Operator tracking labels. Operator will not know the node was decommissioned.
  - Scale down the rack's `members` count by 1. The Operator labels the service, the sidecar calls decommission, and the StatefulSet scales down after completion. See [Scaling](../operating/scaling.md).
* - [`removenode`](https://docs.scylladb.com/stable/operating-scylla/nodetool-commands/removenode.html)
  - Removes a node from the ring without Operator knowledge. Operator expects to manage membership via scale-down or replace.
  - Use the `scylla/replace` label on the member Service to trigger Operator-managed replacement. For dead nodes, see [Recovering from failed replace](../troubleshooting/recovering-from-failed-replace.md).
* - `move`
  - Changes token ownership without Operator awareness.
  - Not supported. Use scaling (add/remove nodes) to rebalance.
* - [`rebuild`](https://docs.scylladb.com/stable/operating-scylla/nodetool-commands/rebuild.html)
  - Streams data from another datacenter; Operator does not track rebuild state. Running during an Operator-managed operation can cause conflicts.
  - No Operator alternative. If rebuild is needed (for example, adding a new DC), coordinate manually — ensure no other operations are in progress.
* - [`disablebinary`](https://docs.scylladb.com/stable/operating-scylla/nodetool-commands/disablebinary.html) / [`enablebinary`](https://docs.scylladb.com/stable/operating-scylla/nodetool-commands/enablebinary.html)
  - Disabling native transport makes the node unreachable to clients and may cause readiness probe failures, triggering Operator recovery actions.
  - Do not use. If you need to stop traffic to a node, use [maintenance mode](../operating/maintenance-mode.md).
* - [`disablegossip`](https://docs.scylladb.com/stable/operating-scylla/nodetool-commands/disablegossip.html) / [`enablegossip`](https://docs.scylladb.com/stable/operating-scylla/nodetool-commands/enablegossip.html)
  - Disabling gossip effectively marks the node as down in the cluster, confusing the Operator's health checks and potentially triggering unwanted recovery.
  - Do not use.
:::

### Medium risk — automatic or use with caution

:::{list-table}
:widths: 18 35 47
:header-rows: 1

* - Command
  - Risk
  - Operator alternative
* - [`drain`](https://docs.scylladb.com/stable/operating-scylla/nodetool-commands/drain.html)
  - Puts node in DRAINED state; sidecar may restart ScyllaDB if a decommission is pending.
  - Automatic — drain runs as a `preStop` lifecycle hook before every pod shutdown. No manual invocation needed.
* - [`disableautocompaction`](https://docs.scylladb.com/stable/operating-scylla/nodetool-commands/disableautocompaction.html) / [`enableautocompaction`](https://docs.scylladb.com/stable/operating-scylla/nodetool-commands/enableautocompaction.html)
  - Ephemeral change, but disabling auto-compaction can cause unbounded SSTable growth if forgotten. Operator does not track this state.
  - No Operator alternative. Use with caution and re-enable promptly.
* - [`stop`](https://docs.scylladb.com/stable/operating-scylla/nodetool-commands/stop.html) (compaction)
  - Stops in-progress compaction. Safe but may need to be repeated if compaction restarts.
  - No Operator alternative. Use with caution.
:::

### Low risk — safe to use

These commands are safe to use directly. The Operator either handles them automatically or does not conflict with them.

:::{list-table}
:widths: 18 35 47
:header-rows: 1

* - Command
  - Notes
  - Operator alternative
* - [`repair`](https://docs.scylladb.com/stable/operating-scylla/nodetool-commands/repair.html)
  - Safe to run manually but redundant.
  - Configure repair tasks via ScyllaDB Manager (managed through the ScyllaCluster spec). Manager handles scheduling and distributed coordination.
* - [`cleanup`](https://docs.scylladb.com/stable/operating-scylla/nodetool-commands/cleanup.html)
  - Safe and idempotent, but the Operator runs cleanup automatically when it detects token ring changes.
  - Automatic — the Operator tracks token ring hash changes per service and spawns cleanup Jobs when they diverge. Manual cleanup is only needed after replication factor changes (which the Operator does not detect).
* - [`snapshot`](https://docs.scylladb.com/stable/operating-scylla/nodetool-commands/snapshot.html) / [`clearsnapshot`](https://docs.scylladb.com/stable/operating-scylla/nodetool-commands/clearsnapshot.html)
  - Safe to use.
  - Snapshots are taken automatically before rolling updates. For backup, use ScyllaDB Manager backup tasks.
* - [`compact`](https://docs.scylladb.com/stable/operating-scylla/nodetool-commands/compact.html)
  - Safe to trigger manually.
  - No Operator alternative; manual compaction is acceptable.
* - [`flush`](https://docs.scylladb.com/stable/operating-scylla/nodetool-commands/flush.html)
  - Safe to trigger manually.
  - No Operator alternative; flushing memtables is acceptable.
* - [`scrub`](https://docs.scylladb.com/stable/operating-scylla/nodetool-commands/scrub.html)
  - Safe to trigger manually.
  - No Operator alternative; manual scrub is acceptable.
* - [`setlogginglevel`](https://docs.scylladb.com/stable/operating-scylla/nodetool-commands/setlogginglevel.html)
  - Ephemeral change, lost on pod restart. Safe for live debugging.
  - Preferred method for emergency log-level changes when a rolling restart is not possible. See [Changing log level](../troubleshooting/changing-log-level.md). For persistent changes, use the ScyllaCluster spec — see [Passing ScyllaDB arguments](../operating/passing-scylladb-arguments.md).
* - [`refresh`](https://docs.scylladb.com/stable/operating-scylla/nodetool-commands/refresh.html)
  - Loads SSTables placed on disk; does not affect cluster membership.
  - No Operator alternative; safe to use.
* - [`upgradesstables`](https://docs.scylladb.com/stable/operating-scylla/nodetool-commands/upgradesstables.html)
  - Rewrites SSTables to the latest format; safe but resource-intensive.
  - No Operator alternative; safe to use, typically needed after a ScyllaDB version upgrade.
* - [`setcompactionthroughput`](https://docs.scylladb.com/stable/operating-scylla/nodetool-commands/setcompactionthroughput.html) / [`setstreamthroughput`](https://docs.scylladb.com/stable/operating-scylla/nodetool-commands/setstreamthroughput.html)
  - Ephemeral tuning changes, lost on restart.
  - No Operator alternative; safe to use for temporary tuning. For persistent changes, configure via the ScyllaCluster spec.
* - [`settraceprobability`](https://docs.scylladb.com/stable/operating-scylla/nodetool-commands/settraceprobability.html)
  - Ephemeral diagnostic setting.
  - No Operator alternative; safe to use for debugging.
:::

## Related pages

- [StatefulSets and racks](../architecture/statefulsets-and-racks.md) — how the Operator maps ScyllaDB topology onto Kubernetes primitives.
- [Scaling](../operating/scaling.md) — adding and removing nodes via the Operator.
- [Replacing nodes](../operating/replacing-nodes.md) — Operator-managed node replacement.
- [Automatic data cleanup](../architecture/automatic-data-cleanup.md) — how the Operator handles post-scaling cleanup.
