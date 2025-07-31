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

````{grid}
:type: default

```{topic-box}
:title: ScyllaClusters
:icon: icon-database
:link: scyllaclusters/basics
:anchor: Learn more »
:class: large-6

ScyllaCluster defines a **single datacenter** ScyllaDB cluster and manages the racks within.
```

```{topic-box}
:title: ScyllaDBClusters
:icon: icon-database
:link: scylladbclusters/scylladbclusters
:anchor: Learn more »
:class: large-6

ScyllaDBCluster defines a **multi datacenter** ScyllaDB cluster spanned across multiple Kubernetes clusters and manages the datacenters and racks within.
```

```{topic-box}
:title: ScyllaDBMonitorings
:icon: icon-monitoring
:link: scylladbmonitorings
:anchor: Learn more »
:class: large-6

ScyllaDBMonitoring defines and manages a ScyllaDB monitoring stack.
```
````

## Cluster scoped resources

Cluster scoped resources declare the state for the whole cluster or control something at the cluster level which isn't multitenant.
Therefore, working with these resources requires elevated privileges.

Generally, there can be multiple instances of these resources but for some of them, like for [ScyllaOperatorConfigs](./scyllaoperatorconfigs.md), it only makes sense to have one instance. We call them *singletons* and the single instance is named `cluster`.

````{grid}
:type: default

```{topic-box}
:title: NodeConfigs
:icon: icon-tune
:link: nodeconfigs
:anchor: Learn more »
:class: large-6

NodeConfig manages setup and tuning for a set of Kubernetes nodes.
```

```{topic-box}
:title: ScyllaOperatorConfigs
:icon: icon-settings
:link: scyllaoperatorconfigs
:anchor: Learn more »
:class: large-6

ScyllaOperatorConfig configures the {{productName}} deployment and reports the status.
```

```{topic-box}
:title: RemoteKubernetesClusters
:icon: icon-server-3
:link: remotekubernetesclusters
:anchor: Learn more »
:class: large-6

RemoteKubernetesCluster configures the access for {{productName}} to remotely deployed Kubernetes clusters.
```
````
