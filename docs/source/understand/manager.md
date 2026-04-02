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
- Uses the image specified by the `agentVersion` and `agentRepository` fields on the `ScyllaCluster` spec.

## Task synchronisation

The Operator bridges your cluster spec to Manager tasks through a chain of internal resources.

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

### Reconciliation flow

1. The Operator creates an internal **`ScyllaDBManagerClusterRegistration`** resource to register the cluster with Manager.
2. The **ScyllaDBManagerClusterRegistration controller** calls the Manager REST API to register the cluster and stores the resulting cluster ID in its status.
3. The **ScyllaDBManagerTask controller** reads the registration, then creates, updates, or deletes tasks in Manager via its REST API.
4. Task statuses (run history, next run time, errors) are propagated back to the `ScyllaDBManagerTask` status and to the `.status.backups` and `.status.repairs` fields on the `ScyllaCluster`.

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

## Multi-datacenter Manager integration

In a multi-datacenter cluster built from multiple `ScyllaCluster` resources (one per Kubernetes cluster), ScyllaDB Manager must be deployed in **only one** datacenter. Manager communicates with all nodes across datacenters through the Manager Agent running in each pod.

Every `ScyllaCluster` is provisioned with a unique, randomly generated auth token stored in a Secret named `<cluster>-auth-token`. For Manager to manage nodes in all datacenters, every datacenter must use the **same** auth token. You must manually synchronize the token:

1. **Extract the token** from the datacenter where Manager is deployed:

   ```shell
   kubectl --context="${CONTEXT_DC1}" -n=<namespace> get secrets/<cluster>-auth-token \
     --template='{{ index .data "auth-token.yaml" }}' | base64 -d
   ```

2. **Patch the token** into each remote datacenter's Secret:

   ```shell
   kubectl --context="${CONTEXT_DC2}" -n=<namespace> patch secret/<cluster>-auth-token \
     --type='json' \
     -p='[{"op": "add", "path": "/stringData", "value": {"auth-token.yaml": "<output-from-step-1>"}}]'
   ```

3. **Rolling restart** the remote datacenter so the Agents pick up the new token:

   ```shell
   kubectl --context="${CONTEXT_DC2}" -n=<namespace> patch scyllacluster/<cluster> \
     --type='merge' \
     -p='{"spec": {"forceRedeploymentReason": "sync manager-agent auth token"}}'
   ```

4. **Define Manager tasks** on the `ScyllaCluster` in the Kubernetes cluster where Manager is running.

For the full procedure, see [Deploy a multi-datacenter cluster](../deploy-scylladb/deploy-multi-dc-cluster.md).

## Limitations

- **Restore** is not yet available through the Operator's declarative API. To restore from a Manager backup, you must exec into the Manager pod and use `sctool` directly. See [Back up and restore](../operate/back-up-and-restore.md).
- There is one global Manager instance per Kubernetes cluster. Multi-tenancy isolation between clusters sharing the same Manager is limited to auth tokens.
- Manager functionality beyond backup and repair (e.g., healthcheck configuration) is not yet exposed through CRDs.

:::{note}
ScyllaDB Manager is available for ScyllaDB Enterprise customers and ScyllaDB Open Source users.
With ScyllaDB Open Source, ScyllaDB Manager is limited to 5 nodes.
See the ScyllaDB Manager [Proprietary Software License Agreement](https://www.scylladb.com/scylla-manager-software-license-agreement/) for details.
:::
