# Troubleshoot

Diagnose and resolve issues with ScyllaDB Operator, ScyllaDB clusters, and related infrastructure.

Start with the [diagnostic flowchart](diagnostic-flowchart.md) to narrow down the problem category, then follow the relevant guide.

## By symptom

| Symptom | Guide |
|---|---|
| Pods stuck in `Pending`, `Init`, `CrashLoopBackOff`, or not becoming `Ready` | [Diagnose a node that is not starting](diagnose-node-not-starting.md) |
| Pods restarting unexpectedly | [Investigate pod restarts](investigate-restarts.md) |
| Cluster degraded, nodes showing `DN` | [Check cluster health](check-cluster-health.md) |
| Application cannot connect to ScyllaDB | [Diagnostic flowchart](diagnostic-flowchart.md) — see also [Connect Your App](../connect-your-app/index.md) and [Set up networking](../set-up-networking/index.md) |
| Operator or webhook installation failures | [Troubleshoot installation issues](troubleshoot-installation.md) |
| Upgrade failed or stuck | [Diagnostic flowchart](diagnostic-flowchart.md) — see also [Upgrade](../upgrade/index.md) |
| Node replace stuck or failed | [Recover from a failed node replace](recover-from-failed-replace.md) |
| Slow queries or throughput degradation | [Troubleshoot performance](troubleshoot-performance.md) |
| Need to change log level without a rolling restart | [Change log level](change-log-level.md) |
| Unfamiliar error message | [Common errors](common-errors.md) |
| Need to collect data for a support ticket | [Collect debugging information](collect-debugging-information/index.md) |
| Need to configure coredumps | [Coredumps](configure-coredumps.md) |

:::{toctree}
:maxdepth: 1

diagnostic-flowchart
troubleshoot-installation
check-cluster-health
investigate-restarts
diagnose-node-not-starting
change-log-level
recover-from-failed-replace
troubleshoot-performance
common-errors
collect-debugging-information/index
configure-coredumps
:::
