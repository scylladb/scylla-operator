# Troubleshoot

Diagnose and resolve issues with ScyllaDB Operator, ScyllaDB clusters, and related infrastructure.

## By symptom

| Symptom | Guide |
|---|---|
| Pods restarting unexpectedly | [Investigate pod restarts](investigate-restarts.md) |
| Application cannot connect to ScyllaDB | [Connect Your App](../connect-your-app/index.md) and [Set up networking](../deploy-scylladb/set-up-networking/index.md) |
| Upgrade failed or stuck | [Upgrade](../upgrade/index.md) |
| Node replace stuck or failed | [Recover from a failed node replace](recover-from-failed-replace.md) |
| Slow queries, throughput degradation, or CPU pinning not working | [Troubleshoot performance](troubleshoot-performance.md) |
| Need to change log level without a rolling restart | [Change log level](change-log-level.md) |
| Need to collect data for a support ticket | [Collect debugging information](collect-debugging-information/index.md) |
| Need to configure or collect core dumps | [Collect core dumps](configure-coredumps.md) |

:::{toctree}
:maxdepth: 1

investigate-restarts
change-log-level
recover-from-failed-replace
troubleshoot-performance
collect-debugging-information/index
configure-coredumps
:::
