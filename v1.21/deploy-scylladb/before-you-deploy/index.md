# Before you deploy

Before deploying a ScyllaDB cluster, prepare your Kubernetes nodes and operator configuration.
These steps ensure ScyllaDB runs with optimal performance and isolation.

#### NOTE
If you are following a [reference deployment](https://operator.docs.scylladb.com/v1.21/deploy-scylladb/reference-deployments/index.md), it links to these pages at the appropriate steps — you do not need to follow them separately.

## Node preparation

ScyllaDB needs dedicated nodes with local NVMe storage, CPU pinning, and node tuning.
Complete these steps in order:

1. [Set up dedicated node pools](https://operator.docs.scylladb.com/v1.21/deploy-scylladb/before-you-deploy/set-up-dedicated-node-pools.md) — provision and label nodes, apply taints.
2. [Configure CPU pinning](https://operator.docs.scylladb.com/v1.21/deploy-scylladb/before-you-deploy/configure-cpu-pinning.md) — enable the static CPU manager policy.
3. [Configure nodes](https://operator.docs.scylladb.com/v1.21/deploy-scylladb/before-you-deploy/configure-nodes.md) — apply `NodeConfig` for disk setup and kernel tuning.

## Operator configuration

- [Configure the Operator](https://operator.docs.scylladb.com/v1.21/deploy-scylladb/before-you-deploy/configure-operator.md) — tune operator-level settings.
