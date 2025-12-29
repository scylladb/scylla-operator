# Configuring kernel parameters (sysctls)

This guide explains how to configure Linux kernel parameters (sysctls) for nodes running production-grade ScyllaDB deployments.

{{productName}} enables you to manage sysctl settings in a Kubernetes-native way using the NodeConfig resource.

:::{note}
To configure sysctls for multi-datacenter ScyllaDB deployments, simply perform the steps outlined in this guide in each worker cluster.
:::

## Configuring the recommended sysctls for ScyllaDB nodes

:::{caution}    
The recommended sysctl for production-grade ScyllaDB deployments provided throughout our examples follow the recommendations from the upstream ScyllaDB repository.
However, as we are not able to reliably keep them up to date at all times, always consult the {{ '[configuration files in the source branch of your ScyllaDB release](https://github.com/scylladb/scylladb/tree/scylla-{}/dist/common/sysctl.d)'.format(imageTag) }} before you configure your cluster.
:::

:::{caution}
The recommended sysctl values are provided with the assumption that your nodes are not shared between multiple ScyllaDB instances.
If your nodes are shared, consider adjusting the values accordingly.
:::

The below NodeConfig manifest configures the sysctls recommended for production-grade ScyllaDB deployments. Use the placement criteria to target the nodes used for running ScyllaDB workloads in your cluster.

:::{literalinclude} ../../../examples/common/configure-sysctls.nodeconfig.yaml
:language: yaml
:emphasize-lines: 6-18
:::

:::{tip}
You can learn more about the NodeConfig resource in the [NodeConfig tutorial](../resources/nodeconfigs.md) and the [generated API reference](../reference/api/groups/scylla.scylladb.com/nodeconfigs.rst).
:::

### Configuring sysctls via GitOps (kubectl)

To configure kernel parameters on selected nodes in your cluster, simply apply the NodeConfig manifest.

To apply the above example manifest with `kubectl`, run the following command:
:::{code-block} console
:substitutions:
kubectl apply --server-side -f=https://raw.githubusercontent.com/{{repository}}/{{revision}}/examples/common/configure-sysctls.nodeconfig.yaml
:::

After applying the manifest, wait for the NodeConfig to be reconciled.
:::{include} ./../.internal/wait-for-status-conditions.nodeconfig.code-block.md
:::

### Configuring sysctls via Helm

We do not currently provide Helm charts for NodeConfig configuration.
In case you installed {{productName}} using Helm, you still need use the GitOps (kubectl) instructions for NodeConfig setup.
