# Feature gates

ScyllaDB Operator lets you enable or disable features using feature gates.
This page lists the available feature gates and explains how to configure them.

## Available feature gates

:::{list-table}
:widths: 50 15 15 20
:header-rows: 1

* - Feature gate
  - Default
  - Stage
  - Since
* - `AutomaticTLSCertificates`
  - `true`
  - Beta
  - alpha: v1.8, beta: v1.11
* - `BootstrapSynchronisation`
  - `false`
  - Alpha
  - v1.19
:::

- **Default** — whether the feature is enabled when you don't set it explicitly.
- **Stage** — `Alpha` features are disabled by default and may change without notice. `Beta` features are enabled by default and are considered stable, but may still change.
- **Since** — the Operator version in which the feature gate was introduced or its default was changed.

### AutomaticTLSCertificates

Enables automated TLS certificate provisioning for ScyllaDB clusters.
When enabled, the Operator generates and rotates serving and client TLS certificates and configures ScyllaDB nodes to use them for encrypted client-to-node CQL communication (mTLS).

Client certificates are validated by ScyllaDB nodes (the certificate chain must be trusted), but ScyllaDB does **not** perform client identity or authorization checks based on certificate contents.

See [Security — ScyllaDB cluster TLS](../understand/security.md) for the full certificate architecture, and [Connect via CQL](../connect-your-app/connect-via-cql.md) for client configuration.

:::{caution}
mTLS for node-to-node communication is [not yet supported](https://github.com/scylladb/scylla-operator/issues/2434).
:::

### BootstrapSynchronisation

:::{caution}
This feature requires ScyllaDB ≥ 2025.2.0. The Operator checks the container image version and only adds the bootstrap-barrier init container when the version satisfies this requirement.
:::

Automates ensuring that no nodes are down when a new ScyllaDB node bootstraps.
The Operator verifies the status of all existing nodes in the cluster and blocks the new node's startup until every node is confirmed healthy.

See [Bootstrap synchronisation](../understand/bootstrap-sync.md) for details on the mechanism.

## Configuring feature gates

Feature gates are set with the `--feature-gates` command-line argument of ScyllaDB Operator.
The value is a comma-separated list of `<gate>=<bool>` pairs.

For example, to enable both gates:

```
--feature-gates=AutomaticTLSCertificates=true,BootstrapSynchronisation=true
```

:::::{tabs}

::::{group-tab} GitOps (kubectl)
Modify the ScyllaDB Operator Deployment and add the `--feature-gates` argument to the container args:

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
Add the `--feature-gates` argument through the `additionalArgs` value:

:::{code-block} yaml
:emphasize-lines: 2
additionalArgs:
- --feature-gates=AutomaticTLSCertificates=true,BootstrapSynchronisation=true
:::

::::

:::::
