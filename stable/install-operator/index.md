# Install Operator

Install ScyllaDB Operator and its dependencies into your Kubernetes cluster.

Before installing, review [Provision infrastructure](https://operator.docs.scylladb.com/stable/install-operator/provision-infrastructure/index.md) to ensure your environment meets all requirements.

## Software prerequisites

### cert-manager

ScyllaDB Operator uses [cert-manager](https://cert-manager.io/) to manage TLS certificates for webhook servers.
cert-manager must be installed before the operator.
See [Install with GitOps](https://operator.docs.scylladb.com/stable/install-operator/install-with-gitops.md) for installation steps.

### Prometheus Operator (optional)

If you plan to use ScyllaDB Operator’s [monitoring integration](https://operator.docs.scylladb.com/stable/deploy-scylladb/set-up-monitoring/index.md), the [Prometheus Operator](https://github.com/prometheus-operator/prometheus-operator) must be installed in the cluster.
This is not required for the operator itself to function.

## Installation methods

Choose the installation method that matches your environment:

- **[GitOps](https://operator.docs.scylladb.com/stable/install-operator/install-with-gitops.md)** — install using `kubectl apply` with manifests from the project repository. Recommended for most environments.
- **[Helm](https://operator.docs.scylladb.com/stable/install-operator/install-with-helm.md)** — install using Helm charts.
- **[OpenShift](https://operator.docs.scylladb.com/stable/install-operator/install-on-openshift.md)** — install via the Operator Lifecycle Manager (OLM) software catalog on Red Hat OpenShift.
