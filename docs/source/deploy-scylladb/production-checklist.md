# Production checklist

This page provides a checklist of settings and configurations to verify before running ScyllaDB in production on Kubernetes.

:::{tip}
Work through this checklist after your cluster is deployed and functional. Each item links to the how-to page that covers the configuration in detail.
:::

## Checklist

| # | Item | Expected state | Guide |
|---|------|----------------|-------|
| 1 | **NodeConfig deployed** | A `NodeConfig` resource is applied and `Available=True` in each cluster running ScyllaDB. NodeConfig configures disk setup, kernel parameters, and performance tuning — without it, ScyllaDB runs on untuned nodes with default kernel settings. | [Node configuration](before-you-deploy/configure-nodes.md) |
| 2 | **Dedicated node pool** | ScyllaDB runs on isolated nodes with label `scylla.scylladb.com/node-type=scylla` and taint `scylla-operator.scylladb.com/dedicated=scyllaclusters:NoSchedule`. No other workloads are scheduled on these nodes. | [Set up dedicated node pools](before-you-deploy/set-up-dedicated-node-pools.md) |
| 3 | **Performance tuning co-located** | NodeConfig `placement` targets the same nodes as the ScyllaCluster placement. Both use the same label selector and toleration. If they target different node sets, tuning does not apply to ScyllaDB pods. | [Set up dedicated node pools](before-you-deploy/set-up-dedicated-node-pools.md) |
| 4 | **`fs.nr_open` set** | The sysctl `fs.nr_open` is set to a high value (e.g., `1073741816`) via NodeConfig. This defines the ceiling for `RLIMIT_NOFILE`, which the Operator raises on each ScyllaDB process. Without it, ScyllaDB may fail to open files under heavy load. | [Configure nodes](before-you-deploy/configure-nodes.md) |
| 5 | **Coredumps enabled** | `systemd-coredump` is configured on the host with sufficient backing storage. Coredumps are essential for diagnosing crashes. | [Coredumps](../troubleshoot/configure-coredumps.md) |
| 6 | **Monitoring deployed** | A `ScyllaDBMonitoring` resource is created with Prometheus scraping ScyllaDB metrics and Grafana displaying dashboards. Alerts are configured for key failure modes. | [Set up monitoring](set-up-monitoring.md) |
| 7 | **Resource requests and limits set** | Both `resources` (ScyllaDB container) and `agentResources` (Manager Agent sidecar) have explicit requests and limits. ScyllaDB derives its memory allocation from the container memory limit. Ensure values match your instance type and workload. | [Deploy a single-DC cluster](deploy-single-dc-cluster.md) |
| 8 | **CPU pinning active** | The kubelet uses `cpuManagerPolicy: static`. The ScyllaDB pod has Guaranteed QoS class (requests equal limits for all containers). ScyllaDB starts with `--overprovisioned=0`. | [Configure CPU pinning](before-you-deploy/configure-cpu-pinning.md) |
| 9 | **XFS online discard enabled** | The `discard` mount option is set on ScyllaDB data volumes via NodeConfig's `unsupportedOptions`. This enables SSD TRIM for consistent write performance. | [Configure nodes](before-you-deploy/configure-nodes.md) |
| 10 | **I/O properties configured** | Precomputed I/O properties are set to avoid the automatic I/O benchmark that runs on first boot, which can produce inconsistent results in cloud environments. | [Configure precomputed IO properties](../operate/configure-io-properties.md) |
| 11 | **Backups scheduled** | ScyllaDB Manager backup tasks are configured with object storage (S3, GCS, or Azure Blob) and appropriate IAM credentials. Backup schedules are tested with a restore drill. | [Back up and restore](../operate/back-up-and-restore.md) |

## Verification commands

Run these commands to quickly check the most critical items:

### NodeConfig status

```shell
kubectl get nodeconfigs.scylla.scylladb.com -o wide
```

All NodeConfig resources should show `AVAILABLE=True`, `PROGRESSING=False`, `DEGRADED=False`.

### Pod QoS class

```shell
kubectl get pods -l scylla/cluster=scylladb -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.status.qosClass}{"\n"}{end}'
```

All pods should show `Guaranteed`.

### ScyllaDB startup flags

```shell
kubectl logs <pod-name> -c scylla | grep overprovisioned
```

Expected: `--overprovisioned=0`

### Monitoring status

```shell
kubectl get scylladbmonitoring -o wide
```

Should show `AVAILABLE=True`.

## Related pages

- [Deploy a single-DC cluster](deploy-single-dc-cluster.md) — creating a ScyllaCluster.
- [Deploy a multi-DC cluster](deploy-multi-dc-cluster.md) — multi-datacenter deployment using multiple `ScyllaCluster` resources.
- [Configure nodes](before-you-deploy/configure-nodes.md) — disk setup, sysctls, and performance tuning.
- [Configure CPU pinning](before-you-deploy/configure-cpu-pinning.md) — configuring CPU exclusivity.
- [Set up monitoring](set-up-monitoring.md) — Prometheus and Grafana setup.
