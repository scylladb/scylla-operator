# Feature Gates

{{productName}} lets you enable or disable features using feature gates. This document provides an overview of the available feature gates and instructions on how to use them.

## Available feature gates

The following feature gates are available in {{productName}}:

:::{list-table}
:widths: 60 20 20
:header-rows: 1

* - Feature Gate
  - Default
  - Since
* - AutomaticTLSCertificates
  - `true`
  - v1.11
* - BootstrapSynchronisation
  - `false`
  - v1.19
:::

- The "Default" indicates if the feature is enabled when you don't set it explicitly.
- The "Since" column indicates the {{productName}} version in which the feature gate was introduced or its default was changed.

### AutomaticTLSCertificates

`AutomaticTLSCertificates` enables mTLS client connections to ScyllaDB. 
When this feature is enabled, {{productName}} automatically generates and rotates serving and client TLS certificates.
It also configures ScyllaDB nodes to use these certificates for secure client-to-node communication.

:::{note}
Client certificates are validated by ScyllaDB nodes (the certificate chain must be trusted), but ScyllaDB does **not** perform client identity or authorization checks based on certificate contents.
:::

:::{caution}
mTLS for node-to-node communication is [not yet supported](https://github.com/scylladb/scylla-operator/issues/2434).
:::

Refer to [this document](../resources/scyllaclusters/clients/cql.md#remote-cqlsh) for a guide to configuring ScyllaDB clients to use TLS certificates managed by {{productName}}.

### BootstrapSynchronisation

:::{include} ../.internal/bootstrap-sync-min-scylladb-version-caution.md
:::

`BootstrapSynchronisation` automates the process of ensuring that no nodes are down when a bootstrap operation is performed.
{{productName}} will verify the status of all nodes in the cluster before allowing a new ScyllaDB node to bootstrap.

For more information, refer to the [](../management/bootstrap-sync.md) document explaining the feature in detail.

## Using feature gates

Feature gates can be enabled or disabled by configuring the `--feature-gates` command-line argument of {{productName}}. 
It is a comma-separated list of key-value pairs, where the key is the feature gate name and the value is a boolean indicating whether to enable or disable the feature.
For example, to enable the `AutomaticTLSCertificates` and `BootstrapSynchronisation` feature gates, set the argument to `AutomaticTLSCertificates=true,BootstrapSynchronisation=true`.

:::::{tabs}

::::{group-tab} GitOps (kubectl)
To configure feature gates with GitOps (kubectl), modify the {{productName}} Deployment by configuring the `--feature-gates` command-line argument in the {{productName}} container.

:::{code-block} yaml
:emphasize-lines: 13
apiVersion: apps/v1
kind: Deployment
metadata:
  name: scylla-operator
  namespace: scylla-operator
spec:
  template:
    spec:
      containers:
      - name: scylla-operator
        args:
        - operator
        - --feature-gates=AutomaticTLSCertificates=true,BootstrapSynchronisation=true
:::

::::

::::{group-tab} Helm
To configure feature gates with Helm, configure the `--feature-gates` command-line argument through {{productName}}'s `values.yaml`:

:::{code-block} yaml
:emphasize-lines: 2
additionalArgs: 
- --feature-gates=AutomaticTLSCertificates=true,BootstrapSynchronisation=true
:::

::::

:::::