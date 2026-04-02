# Pass additional ScyllaDB arguments

Pass extra command-line arguments to the ScyllaDB binary at startup to tune behaviour or enable features that are not exposed through the ScyllaDB configuration files.

:::{caution}
When additional ScyllaDB arguments are set, ScyllaDB may behave unexpectedly.
Every such setup is considered unsupported.
Prefer using a custom ScyllaDB configuration file (`customConfigMapRef` / `scyllaConfig`) for configuration options that can be set through `scylla.yaml`.
:::

## How it works

The Operator appends the additional arguments to the ScyllaDB binary command line when starting each pod.
Because the arguments are part of the pod spec (via the StatefulSet), changing them triggers a **rolling restart** of all nodes in the cluster — each node is updated one at a time.

## ScyllaCluster (v1 API)

Set `spec.scyllaArgs` to a string of additional arguments:

```yaml
apiVersion: scylla.scylladb.com/v1
kind: ScyllaCluster
metadata:
  name: scylla
  namespace: scylla
spec:
  scyllaArgs: "--blocked-reactor-notify-ms 10 --abort-on-seastar-signal-handling-failure 1"
  datacenter:
    name: us-east-1
    racks:
      - name: us-east-1a
        members: 3
        storage:
          capacity: 500Gi
        resources:
          limits:
            cpu: 4
            memory: 8Gi
```

:::{note}
In the v1 API, `scyllaArgs` is a single string.
Multiple arguments are separated by spaces.
:::

## Emergency scenarios

In emergency scenarios — such as a stuck rollout or a degraded cluster where a rolling restart is not possible — you may need to change the ScyllaDB log level or other runtime settings without triggering a restart.
For such cases, see [Changing the log level](../troubleshoot/change-log-level.md).

## Related pages

- [Perform a rolling restart](perform-rolling-restart.md) — how the Operator performs rolling restarts when the pod spec changes
- [Changing the log level](../troubleshoot/change-log-level.md) — adjusting ScyllaDB runtime settings without a restart
