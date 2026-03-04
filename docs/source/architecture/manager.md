# ScyllaDB Manager

[ScyllaDB Manager](https://manager.docs.scylladb.com/) is a companion service that provides scheduled repairs and backups for ScyllaDB clusters.
ScyllaDB Operator integrates with Manager so that you can define repair and backup tasks declaratively in your cluster spec, without interacting with Manager directly.

## Deployment model

ScyllaDB Manager runs as a **single, shared Deployment** in the `scylla-manager` namespace.
One Manager instance serves all ScyllaDB clusters in the Kubernetes cluster.

Manager requires a small ScyllaDB database to store its own state (task definitions, run history, cluster metadata).
This is provided by a dedicated `ScyllaCluster` resource named `scylla-manager-cluster` in the `scylla-manager` namespace, running in developer mode with minimal resources (1 node, 1 CPU, 200 MiB memory).

:::{note}
The backing ScyllaCluster has the annotation `scylla-operator.scylladb.com/disable-global-scylladb-manager-integration: "true"` to prevent it from being registered with the very Manager instance it supports.
:::

Manager depends on ScyllaDB Operator — the Operator must be installed first because the backing cluster uses the `ScyllaCluster` CRD.

## Manager Agent

Each ScyllaDB pod runs a **ScyllaDB Manager Agent** as a sidecar container.
The Agent communicates with Manager to execute operations on the local node (streaming repair data, uploading backup snapshots to object storage, etc.).

The Agent:
- Listens on port 10001.
- Waits for [ignition](ignition.md) before starting — it does not run until ScyllaDB itself is ready.
- Is configured through layered YAML config files and an auth token that the Operator manages automatically.
- Uses the image specified by `agentVersion` and `agentRepository` (for `ScyllaCluster`) or the `scyllaDBManagerAgent.image` field (for `ScyllaDBDatacenter`).

## Task synchronisation

The Operator bridges your cluster spec to Manager tasks through a chain of internal resources:

### For `ScyllaCluster` (v1 API)

`ScyllaCluster` defines backup and repair tasks inline:

```yaml
spec:
  backups:
    - name: daily-backup
      location:
        - s3:my-bucket
      retention: 7
      cron: "0 2 * * *"
  repairs:
    - name: weekly-repair
      cron: "0 0 * * 0"
```

The ScyllaCluster controller translates each entry into a `ScyllaDBManagerTask` resource in the same namespace.

### For `ScyllaDBCluster` / `ScyllaDBDatacenter` (v1alpha1 API)

Tasks are defined as separate `ScyllaDBManagerTask` resources that reference the cluster:

```yaml
apiVersion: scylla.scylladb.com/v1alpha1
kind: ScyllaDBManagerTask
metadata:
  name: daily-backup
spec:
  scyllaDBClusterRef:
    name: my-cluster
  type: Backup
  backup:
    location:
      - s3:my-bucket
    retention: 7
    schedule:
      cron: "0 2 * * *"
```

### Reconciliation flow

1. The **ScyllaDBDatacenter controller** creates a `ScyllaDBManagerClusterRegistration` resource to register the cluster with Manager.
2. The **ScyllaDBManagerClusterRegistration controller** calls the Manager REST API to register the cluster and stores the resulting cluster ID in its status.
3. The **ScyllaDBManagerTask controller** reads the registration, then creates, updates, or deletes tasks in Manager via its REST API.
4. Task statuses (run history, next run time, errors) are propagated back to the `ScyllaDBManagerTask` status and, for `ScyllaCluster`, to the `.status.backups` and `.status.repairs` fields.

## Disabling Manager integration

If you do not want a ScyllaCluster to be managed by the shared Manager instance, add the annotation:

```yaml
metadata:
  annotations:
    scylla-operator.scylladb.com/disable-global-scylladb-manager-integration: "true"
```

This prevents the Operator from creating registration and task resources for that cluster.

## Security

Because Manager is a shared instance, access to the `scylla-manager` namespace grants control over **all** registered clusters' repair and backup tasks.

:::{caution}
Only cluster administrators should have access to the `scylla-manager` namespace.
Namespace-level RBAC should restrict non-admin users from viewing or modifying resources there.
:::

The Manager Agent authenticates with Manager using an auth token.
The Operator generates and distributes these tokens automatically — one per cluster — via Secrets in the cluster's namespace.

A `NetworkPolicy` in the `scylla-manager` namespace allows Manager to reach ScyllaDB pods across namespaces for agent communication.

## Limitations

- **Restore** is not yet available through the Operator's declarative API. To restore from a Manager backup, you must exec into the Manager pod and use `sctool` directly. See [Backup and Restore](../operating/backup-and-restore.md).
- There is one global Manager instance per Kubernetes cluster. Multi-tenancy isolation between clusters sharing the same Manager is limited to auth tokens.
- Manager functionality beyond backup and repair (e.g., healthcheck configuration) is not yet exposed through CRDs.

:::{note}
ScyllaDB Manager is available for ScyllaDB Enterprise customers and ScyllaDB Open Source users.
With ScyllaDB Open Source, ScyllaDB Manager is limited to 5 nodes.
See the ScyllaDB Manager [Proprietary Software License Agreement](https://www.scylladb.com/scylla-manager-software-license-agreement/) for details.
:::
