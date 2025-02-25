# Overview

There are generally 2 types of resources in Kubernetes: [namespaced](#namespaced-resources) and [cluster-scoped](#cluster-scoped-resources) resources.

::::{tip}
You can discover the available resources (for the installed version in your cluster) by running
:::{code} bash
kubectl api-resources --api-group=scylla.scylladb.com
:::
and explore their schema using
:::{code} bash
kubectl explain --api-version='scylla.scylladb.com/v1alpha1' NodeConfig.spec
:::
::::

## Namespaced resources

::::{grid} 1 1 2 3
:gutter: 4

:::{grid-item-card} {material-regular}`storage;2em` ScyllaClusters
:link: /resources/scyllaclusters/basics

ScyllaCluster defines a ScyllaDB datacenter and manages the racks within.
+++
[Learn more »](./scyllaclusters/basics.md)
:::

:::{grid-item-card} {material-round}`insights;2em` ScyllaDBMonitorings
:link: /resources/scylladbmonitorings

ScyllaDBMonitoring defines and manages a ScyllaDB monitoring stack.
+++
[Learn more »](./scylladbmonitorings.md)
:::

::::

## Cluster scoped resources

Cluster scoped resources declare the state for the whole cluster or control something at the cluster level which isn't multitenant.
Therefore, working with these resources requires elevated privileges.

Generally, there can be multiple instances of these resources but for some of them, like for [ScyllaOperatorConfigs](./scyllaoperatorconfigs.md), it only makes sense to have one instance. We call them *singletons* and the single instance is named `cluster`.

::::{grid} 1 1 2 3
:gutter: 4

:::{grid-item-card} {material-regular}`bolt;2em` NodeConfigs
:link: /resources/nodeconfigs

NodeConfig manages setup and tuning for a set of Kubernetes nodes.
+++
[Learn more »](./nodeconfigs.md)
:::

:::{grid-item-card} {material-regular}`settings;2em` ScyllaOperatorConfigs
:link: /resources/scyllaoperatorconfigs

ScyllaOperatorConfig configures the {{productName}} deployment and reports the status.
+++
[Learn more »](./scyllaoperatorconfigs.md)
:::

::::
