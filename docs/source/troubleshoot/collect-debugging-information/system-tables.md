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

**Example output (`system.local`):**

```
 key   | bootstrapped | cluster_name | cql_version | data_center
-------+--------------+--------------+-------------+-------------
 local | COMPLETED    | my-cluster   | 3.4.5       | us-east-1
```

**Example output (`system.peers`):**

```
 peer      | data_center | host_id                              | rack       | release_version | tokens
-----------+-------------+--------------------------------------+------------+-----------------+--------
 10.0.1.5  | us-east-1   | a1b2c3d4-e5f6-7890-abcd-ef1234567890 | us-east-1a | 2025.1.0        | {-9223372036854775808, ...}
 10.0.1.6  | us-east-1   | e5f6a7b8-c9d0-1234-5678-9abcdef01234 | us-east-1b | 2025.1.0        | {-1234567890123456789, ...}
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

**Example output (`system.large_partitions`):**

```
 keyspace_name | table_name | sstable_name    | partition_size | partition_key | rows
---------------+------------+-----------------+----------------+---------------+------
 mykeyspace    | mytable    | md-123-big-Data | 1048576        | 'mykey'       | 1000
 mykeyspace    | mytable    | md-124-big-Data | 524288         | 'otherkey'    | 500
```

**Example output (`system.large_rows`):**

```
 keyspace_name | table_name | sstable_name    | row_size | partition_key | clustering_key
---------------+------------+-----------------+----------+---------------+---------------
 mykeyspace    | mytable    | md-124-big-Data | 524288   | 'mykey'       | 'myrow'
```

**Example output (`system.large_cells`):**

```
 keyspace_name | table_name | sstable_name    | cell_size | partition_key | clustering_key | column_name
---------------+------------+-----------------+-----------+---------------+----------------+------------
 mykeyspace    | mytable    | md-125-big-Data | 262144    | 'mykey'       | 'myrow'        | 'mycolumn'
```

Large partitions are a common cause of performance issues and timeouts.
See the [ScyllaDB documentation on large partitions](https://docs.scylladb.com/stable/troubleshoot/large-partition-table.html).

### Repair history

```sql
-- Repair history (distributed across nodes)
SELECT * FROM system_distributed.repair_history LIMIT 20;
```

**Example output (`system_distributed.repair_history`):**

```
 id   | completed_at                    | keyspace_name | options | range_end | range_start | status  | table_names
------+---------------------------------+---------------+---------+-----------+-------------+---------+------------
 uuid | 2026-04-01 10:30:00.000000+0000 | mykeyspace    | {}      | ...       | ...         | SUCCESS | {'mytable'}
 uuid | 2026-04-01 09:15:00.000000+0000 | mykeyspace    | {}      | ...       | ...         | SUCCESS | {'mytable'}
```

### SSTable activity

```sql
-- SSTable activity statistics
SELECT * FROM system.sstable_activity LIMIT 20;
```

**Example output (`system.sstable_activity`):**

```
 keyspace_name | columnfamily_name | generation | rate_120m | rate_15m
---------------+-------------------+------------+-----------+---------
 mykeyspace    | mytable           | 123        |       0.5 |      1.2
 mykeyspace    | mytable           | 124        |       0.2 |      0.8
```

### Runtime information

```sql
-- Runtime configuration and memory usage
SELECT * FROM system.runtime_info;
```

**Example output (`system.runtime_info`):**

```
 group  | item                | value
--------+---------------------+------------
 lsa    | total_space_bytes   | 1073741824
 lsa    | used_space_bytes    | 536870912
 memory | free_memory_bytes   | 2147483648
```

### Compaction history

```sql
-- Recent compaction events
SELECT * FROM system.compaction_history LIMIT 20;
```

**Example output (`system.compaction_history`):**

```
 id   | bytes_in | bytes_out | columnfamily_name | compaction_properties | ended_at                        | keyspace_name | rows_merged
------+----------+-----------+-------------------+-----------------------+---------------------------------+---------------+------------
 uuid | 10485760 |   8388608 | mytable           | {}                    | 2026-04-01 09:00:00.000000+0000 | mykeyspace    | {1: 5000}
 uuid |  5242880 |   4194304 | mytable           | {}                    | 2026-04-01 08:30:00.000000+0000 | mykeyspace    | {1: 2500}
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
