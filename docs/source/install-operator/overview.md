# Overview

Install ScyllaDB Operator and its dependencies into your Kubernetes cluster.

Before installing, review [Provision infrastructure](provision-infrastructure/overview.md) to ensure your environment meets all requirements.

## Software prerequisites

### cert-manager

ScyllaDB Operator uses [cert-manager](https://cert-manager.io/) to manage TLS certificates for webhook servers.
cert-manager must be installed before the operator.
See [Install with GitOps](install-with-gitops.md) for installation steps.

### Prometheus Operator (optional)

If you plan to use ScyllaDB Operator's [monitoring integration](../deploy-scylladb/set-up-monitoring/index.md), the [Prometheus Operator](https://github.com/prometheus-operator/prometheus-operator) must be installed in the cluster.
This is not required for the operator itself to function.

## Installation methods

Choose the installation method that matches your environment:

- **[GitOps](install-with-gitops.md)** — install using `kubectl apply` with manifests from the project repository. Recommended for most environments.
- **[Helm](install-with-helm.md)** — install using Helm charts.
- **[OpenShift](install-on-openshift.md)** — install via the Operator Lifecycle Manager (OLM) software catalog on Red Hat OpenShift.
