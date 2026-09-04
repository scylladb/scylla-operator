# Synchronising bootstrap operations in ScyllaDB clusters

This document explains the automated bootstrap synchronisation in {{productName}}.

## Overview

[ScyllaDB requires that when a new node is added to a cluster, no existing nodes are down](https://docs.scylladb.com/manual/stable/operating-scylla/procedures/cluster-management/add-node-to-cluster.html#check-the-status-of-nodes).
Precisely speaking, no node in the cluster can consider any other node to be down when a topology change operation is performed.
If this is not the case and a non-idempotent operation of bootstrapping a new node is started, the coordinator will deny the node joining the cluster and leave it in a state which is difficult to recover from automatically.

To address this issue, by extending ScyllaDB `Pods` with init containers, {{productName}} implements a barrier which observes the statuses of all nodes in the cluster in order to verify that no node in the cluster is down before allowing a new ScyllaDB node to bootstrap.
Any restarting nodes which have already been bootstrapped are exempt from meeting this precondition.
The status of the nodes is propagated in the cluster using the internal `ScyllaDBDatacenterNodeStatuses` CRD. Please refer to the [ScyllaDBDatacenterNodeStatuses API reference](../reference/api/groups/scylla.scylladb.com/scylladbdatacenternodesstatusreports.rst) for more information.

:::{caution}
Bootstrap synchronisation does not currently extend to nodes replacing dead nodes in the cluster (i.e. the nodes which are being added to the cluster by a [Node replacement operation](../resources/scyllaclusters/nodeoperations/replace-node/)).
Ensure [the applicable prerequisites](https://docs.scylladb.com/manual/stable/operating-scylla/procedures/cluster-management/replace-dead-node.html#prerequisites) are met before initializing a node replacement operation.
:::

:::{caution}
Bootstrap synchronisation does not currently extend to [multi-datacenter ScyllaDB clusters configured manually using ScyllaCluster API](../resources/scyllaclusters/multidc/multidc.md). This is due to the fact that node statuses can only be propagated within a single ScyllaDB datacenter.
Before adding a new datacenter or a new node to an existing datacenter in a multi-datacenter ScyllaDB cluster, [check the status of nodes](https://docs.scylladb.com/manual/stable/operating-scylla/procedures/cluster-management/add-node-to-cluster.html#check-the-status-of-nodes) in all datacenters.
:::

## Enable bootstrap synchronisation

:::{include} ../.internal/bootstrap-sync-min-scylladb-version-caution.md
:::

Due to the fact that {{productName}} does not currently have a reliable way of determining the effective ScyllaDB version used in a `ScyllaCluster` and the required ScyllaDB version for this feature is 2025.2 or later,
this feature can only be enabled via the `BootstrapSynchronisation` feature gate. This feature will remain opt-in while versions prior to 2025.2 are supported by {{productName}}.

Please refer to [](../reference/feature-gates.md) for instructions on how enable or disable a feature gate.

## Override the precondition check

By default, when the `BootstrapSynchronisation` feature gate is enabled, bootstrap synchronisation is enforced for all `ScyllaClusters` in the cluster.
However, in scenarios where you might need to circumvent the precondition for a specific `ScyllaCluster` or a selected ScyllaDB node, you can do so by setting a dedicated annotation.
When the annotation is set on a `ScyllaCluster` or a ScyllaDB node `Service`, the corresponding entity will be forced to proceed with the bootstrap operation unconditionally.

You can override the precondition check by running the following command:
:::::{tabs}

::::{group-tab} ScyllaCluster

:::{code-block} bash
kubectl annotate -n scylla scyllacluster <scyllacluster-name> scylla-operator.scylladb.com/force-proceed-to-bootstrap=true
:::

::::

::::{group-tab} Selected ScyllaDB node

:::{code-block} bash
kubectl annotate -n scylla service <scylladb-node-service-name> scylla-operator.scylladb.com/force-proceed-to-bootstrap=true
:::

::::

:::::


