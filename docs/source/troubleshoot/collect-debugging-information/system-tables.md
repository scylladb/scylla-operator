# Query system tables for debugging

Use ScyllaDB system tables to gather diagnostic information that is not captured by `must-gather`.

System tables contain ScyllaDB-internal state — repair history, SSTable activity, large partitions, runtime information, and more.

## When to use system tables

| Use case | Tool |
|---|---|
| Kubernetes-level diagnostics (pod status, events, logs) | [must-gather](must-gather.md) |
| ScyllaDB-internal state (repair history, SSTable stats, large partitions) | System tables (this page) |

Include system table output alongside must-gather archives when filing support tickets for ScyllaDB-level issues (performance, data consistency, repair).

## How to query system tables

Connect to a ScyllaDB pod and use `cqlsh`:

```bash
kubectl -n scylla exec -it <pod-name> -c scylla -- cqlsh
```

Or run a single query:

```bash
kubectl -n scylla exec -it <pod-name> -c scylla -- \
  cqlsh -e "<CQL query>"
```

## Useful system tables

### Cluster and node information

```sql
-- Node-local information (listen address, host ID, cluster name, ScyllaDB version)
SELECT * FROM system.local;

-- All known peers in the cluster
SELECT peer, data_center, rack, host_id, schema_version, tokens FROM system.peers;
```

### Large partitions

```sql
-- Partitions that exceed the large partition threshold
SELECT * FROM system.large_partitions LIMIT 20;

-- Large rows
SELECT * FROM system.large_rows LIMIT 20;

-- Large cells
SELECT * FROM system.large_cells LIMIT 20;
```

Large partitions are a common cause of performance issues and timeouts.
See the [ScyllaDB documentation on large partitions](https://docs.scylladb.com/stable/troubleshoot/large-partition-table.html).

### Repair history

```sql
-- Repair history (distributed across nodes)
SELECT * FROM system_distributed.repair_history LIMIT 20;
```

### SSTable activity

```sql
-- SSTable activity statistics
SELECT * FROM system.sstable_activity LIMIT 20;
```

### Runtime information

```sql
-- Runtime configuration and memory usage
SELECT * FROM system.runtime_info;
```

### Compaction history

```sql
-- Recent compaction events
SELECT * FROM system.compaction_history LIMIT 20;
```

## Including system table output in support tickets

When filing a support ticket, include:

1. Output from `system.local` and `system.peers` — provides cluster topology context.
2. Output from `system.large_partitions` — if the issue is performance-related.
3. Output from `system_distributed.repair_history` — if the issue involves data consistency.
4. Any other tables relevant to the specific issue.

Format the output for readability:

```bash
kubectl -n scylla exec -it <pod-name> -c scylla -- \
  cqlsh -e "EXPAND ON; SELECT * FROM system.local;"
```

The `EXPAND ON` command formats each row vertically, making wide tables easier to read.

## Related pages

- [must-gather](must-gather.md)
- [must-gather contents](must-gather-contents.md)
- [Cluster health](../check-cluster-health.md)
