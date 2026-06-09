# Use maintenance mode

Temporarily remove a ScyllaDB node from service so you can perform maintenance operations — such as filesystem repairs, manual ScyllaDB restarts, or debugging — without Kubernetes killing the pod.

## How it works

Each ScyllaDB pod has a corresponding member Service.
When you add the `scylla/node-maintenance` label to this Service, the probe server running inside the pod changes its behaviour:

- **Readiness probe** always returns failure — Kubernetes removes the pod from all Service endpoints and load balancers, so client traffic stops reaching the node.
- **Liveness probe** always returns success — Kubernetes does not restart the pod, keeping it alive for you to interact with.

The pod remains running with full access to its storage and network.
You can exec into the container, stop and start the ScyllaDB process, inspect or repair the filesystem, or perform any other maintenance task.

When you remove the label, the probe server resumes normal behaviour.
The readiness probe begins checking ScyllaDB's actual state again, and the pod is added back to Service endpoints once ScyllaDB reports itself as Up and Normal (UN) with native transport enabled.

:::{note}
Maintenance mode is applied to the **member Service**, not the pod.
The label key is `scylla/node-maintenance` and the value can be any string (an empty string is conventional).
:::

## Enable maintenance mode

Add the `scylla/node-maintenance` label to the member Service of the node you want to maintain.

For a ScyllaCluster named `scylla` in the `scylla` namespace, with the target node's Service named `scylla-us-east-1-us-east-1a-0`:

```bash
kubectl -n scylla label svc scylla-us-east-1-us-east-1a-0 scylla/node-maintenance=""
```

Verify the node has been removed from endpoints:

```bash
kubectl -n scylla get endpoints scylla-client
```

The maintained node's IP should no longer appear in the list.

## Disable maintenance mode

Remove the label from the Service:

```bash
kubectl -n scylla label svc scylla-us-east-1-us-east-1a-0 scylla/node-maintenance-
```

The readiness probe resumes normal checks.
Once ScyllaDB is up and accepting connections, Kubernetes adds the pod back to Service endpoints automatically.

## Operator use during upgrades

The Operator enables maintenance mode automatically during ScyllaDB version upgrades.
Before draining a node, the Operator patches the member Service to add the `scylla/node-maintenance` label.
This prevents the liveness probe from failing while ScyllaDB is being drained (a drained node cannot respond to health checks normally).

After draining and backing up data, the Operator removes the label and deletes the pod.
The StatefulSet controller creates a new pod running the upgraded ScyllaDB version.

You do not need to manage maintenance mode manually during upgrades — the Operator handles it as part of the upgrade workflow.
For details, see [Upgrade ScyllaDB](../upgrade/upgrade-scylladb.md).
