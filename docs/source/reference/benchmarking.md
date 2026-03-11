# Benchmarking

This page explains how to run benchmarks against ScyllaDB on Kubernetes and how to interpret the results.

## Prerequisites

Before benchmarking, verify that your cluster meets production requirements. Benchmarking a misconfigured cluster produces misleading results.

- Complete every item in the [production checklist](../deploying/production-checklist.md).
- In particular, ensure [CPU pinning](../deploying/cpu-pinning.md) is active and [I/O properties](../operating/configuring-io-properties.md) are configured.
- Do **not** benchmark clusters running in developer mode (`developerMode: true`). Developer mode disables performance tuning and is not representative of production performance.

## Benchmarking tools

Two tools are commonly used to benchmark ScyllaDB:

| Tool | Language | Key advantage | Container image |
|---|---|---|---|
| [cassandra-stress](https://github.com/scylladb/cassandra-stress) | Java | Flexible custom schemas via YAML profiles | `scylladb/cassandra-stress` |
| [scylla-bench](https://github.com/scylladb/scylla-bench) | Go | Shard-aware driver, low client-side overhead | `scylladb/scylla-bench` |

`scylla-bench` uses the shard-aware ScyllaDB Go driver, which routes requests directly to the correct shard. This eliminates cross-shard coordination overhead and gives a more accurate picture of ScyllaDB's throughput.

## Running benchmarks as Kubernetes Jobs

Run benchmark tools as Kubernetes `Job` resources inside the same cluster to minimize network latency between the benchmark client and ScyllaDB nodes.

### cassandra-stress example

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: cassandra-stress-write
spec:
  template:
    spec:
      containers:
      - name: cassandra-stress
        image: scylladb/cassandra-stress:latest
        command:
        - cassandra-stress
        args:
        - write
        - "n=1000000"
        - cl=ONE
        - "-mode"
        - native
        - cql3
        - "-col"
        - "n=FIXED(5)"
        - "size=FIXED(64)"
        - "-node"
        - "<cluster-name>-client.<namespace>.svc
        - "-rate"
        - "threads=100"
      restartPolicy: Never
```

Replace `<cluster-name>` and `<namespace>` with your ScyllaDB cluster name and namespace.

### scylla-bench example

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: scylla-bench-write
spec:
  template:
    spec:
      containers:
      - name: scylla-bench
        image: scylladb/scylla-bench:latest
        args:
        - -workload=sequential
        - -mode=write
        - -nodes=<cluster-name>-client.<namespace>.svc
        - -concurrency=64
        - -partition-count=1000000
        - -clustering-row-count=10
        - -clustering-row-size=uniform:100..200
        - -replication-factor=3
      restartPolicy: Never
```

## Placement considerations

### Dedicated stress nodes

For accurate results, run benchmark pods on **separate** Kubernetes nodes from ScyllaDB. Use node affinity or a dedicated node pool for stress pods to avoid competing for CPU and memory with ScyllaDB.

### Multiple benchmark pods

To saturate a ScyllaDB cluster, you may need multiple benchmark pods running in parallel. Create the Job with `spec.parallelism` or submit multiple Jobs. Divide the keyspace range across pods to avoid write conflicts.

## Interpreting results

### cassandra-stress output

Key metrics in the cassandra-stress summary:

- **Op rate** — operations per second (throughput).
- **Latency mean** / **median** / **95th** / **99th** — response time distribution.
- **Partition rate** — partitions written/read per second.

### scylla-bench output

Key metrics:

- **Operations/s** — throughput.
- **Latency (avg, P50, P99, P999, max)** — response time at various percentiles.
- **Errors** — any failed operations (should be zero in a healthy cluster).

### What to look for

- **P99 latency** is more meaningful than average latency for production workloads.
- Compare results with and without CPU pinning to verify that tuning is effective.
- Run both write and read benchmarks. Mixed workloads (`-mode=mixed` in scylla-bench) simulate real application patterns.
- If throughput does not scale linearly with CPU cores, check for bottlenecks: disk I/O (`iostat`), network (`iftop`), or client-side saturation (add more benchmark pods).

## Related pages

- [Production checklist](../deploying/production-checklist.md) — verify cluster readiness before benchmarking.
- [CPU pinning](../deploying/cpu-pinning.md) — ensure ScyllaDB has dedicated CPU cores.
- [Configuring I/O properties](../operating/configuring-io-properties.md) — precompute disk performance parameters.
- [Sizing guide](sizing-guide.md) — instance types and resource sizing.
