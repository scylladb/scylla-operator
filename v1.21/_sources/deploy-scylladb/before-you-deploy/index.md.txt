# Before you deploy

Before deploying a ScyllaDB cluster, prepare your Kubernetes nodes and operator configuration.
These steps ensure ScyllaDB runs with optimal performance and isolation.

:::{note}
If you are following a [reference deployment](../reference-deployments/index.md), it links to these pages at the appropriate steps — you do not need to follow them separately.
:::

## Node preparation

ScyllaDB needs dedicated nodes with local NVMe storage, CPU pinning, and node tuning.
Complete these steps in order:

1. [Set up dedicated node pools](set-up-dedicated-node-pools.md) — provision and label nodes, apply taints.
2. [Configure CPU pinning](configure-cpu-pinning.md) — enable the static CPU manager policy.
3. [Configure nodes](configure-nodes.md) — apply `NodeConfig` for disk setup and kernel tuning.

## Operator configuration

- [Configure the Operator](configure-operator.md) — tune operator-level settings.

:::{toctree}
:hidden:

set-up-dedicated-node-pools
configure-cpu-pinning
configure-nodes
configure-operator
:::
