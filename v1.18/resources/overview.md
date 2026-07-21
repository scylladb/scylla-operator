# Overview

There are generally 2 types of resources in Kubernetes: [namespaced]() and [cluster-scoped]() resources.

## Namespaced resources

ScyllaCluster defines a **single datacenter** ScyllaDB cluster and manages the racks within.

[Learn more »](https://operator.docs.scylladb.com/v1.18/resources/scyllaclusters/basics.md)

ScyllaDBCluster defines a **multi datacenter** ScyllaDB cluster spanned across multiple Kubernetes clusters and manages the datacenters and racks within.

[Learn more »](https://operator.docs.scylladb.com/v1.18/resources/scylladbclusters/scylladbclusters.md)

ScyllaDBMonitoring defines and manages a ScyllaDB monitoring stack.

[Learn more »](https://operator.docs.scylladb.com/v1.18/resources/scylladbmonitorings.md)

## Cluster scoped resources

Cluster scoped resources declare the state for the whole cluster or control something at the cluster level which isn’t multitenant.
Therefore, working with these resources requires elevated privileges.

Generally, there can be multiple instances of these resources but for some of them, like for [ScyllaOperatorConfigs](https://operator.docs.scylladb.com/v1.18/resources/scyllaoperatorconfigs.md), it only makes sense to have one instance. We call them *singletons* and the single instance is named `cluster`.

NodeConfig manages setup and tuning for a set of Kubernetes nodes.

[Learn more »](https://operator.docs.scylladb.com/v1.18/resources/nodeconfigs.md)

ScyllaOperatorConfig configures the Scylla Operator deployment and reports the status.

[Learn more »](https://operator.docs.scylladb.com/v1.18/resources/scyllaoperatorconfigs.md)

RemoteKubernetesCluster configures the access for Scylla Operator to remotely deployed Kubernetes clusters.

[Learn more »](https://operator.docs.scylladb.com/v1.18/resources/remotekubernetesclusters.md)
