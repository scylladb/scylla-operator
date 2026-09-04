# Feature gates

ScyllaDB Operator lets you enable or disable features using feature gates.
This page lists the available feature gates and explains how to configure them.

## Configuring feature gates

Feature gates are set with the `--feature-gates` command-line argument of ScyllaDB Operator.
The value is a comma-separated list of `<gate>=<bool>` pairs.

For example, to enable both gates:

```default
--feature-gates=AutomaticTLSCertificates=true,BootstrapSynchronisation=true
```

GitOps (kubectl)

Modify the ScyllaDB Operator Deployment and add the `--feature-gates` argument to the container args:

```yaml
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
```

Helm

Add the `--feature-gates` argument through the `additionalArgs` value in `values.yaml`:

```yaml
additionalArgs:
- --feature-gates=AutomaticTLSCertificates=true,BootstrapSynchronisation=true
```

## Available feature gates

| Feature gate               | Default   | Last changed   |
|----------------------------|-----------|----------------|
| `AutomaticTLSCertificates` | `true`    | v1.11          |
| `BootstrapSynchronisation` | `false`   | v1.19          |
- **Default** — whether the feature is enabled when you don’t set it explicitly.
- **Last changed** — the Operator version in which the feature gate was introduced or its default was changed.

### AutomaticTLSCertificates

Enables automated TLS certificate provisioning for ScyllaDB clusters.
When enabled, the Operator generates and rotates serving and client TLS certificates and configures ScyllaDB nodes to use them for encrypted client-to-node CQL communication (mTLS).

Client certificates are validated by ScyllaDB nodes (the certificate chain must be trusted), but ScyllaDB does **not** perform client identity or authorization checks based on certificate contents.

See [Security — ScyllaDB cluster TLS](https://operator.docs.scylladb.com/v1.21/understand/security.md) for the full certificate architecture, and [Connect via CQL](https://operator.docs.scylladb.com/v1.21/connect-your-app/connect-via-cql.md) for client configuration.

### BootstrapSynchronisation

Automates ensuring that no nodes are down when a new ScyllaDB node bootstraps.
The Operator verifies the status of all existing nodes in the cluster and blocks the new node’s startup until every node is confirmed healthy.

See [Bootstrap synchronisation](https://operator.docs.scylladb.com/v1.21/understand/bootstrap-sync.md) for details on the mechanism.
