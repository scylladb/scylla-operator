# Install with GitOps

Install ScyllaDB Operator and its dependencies by applying raw manifests from the project repository.
This method works with any GitOps tool (Argo CD, Flux, etc.) or plain `kubectl apply`.

#### NOTE
ScyllaDB Operator must run in the `scylla-operator` namespace.

## Prerequisites

- A Kubernetes cluster meeting the [infrastructure requirements](https://operator.docs.scylladb.com/stable/install-operator/provision-infrastructure/index.md).
- [`kubectl`](https://kubernetes.io/docs/tasks/tools/#kubectl) configured to communicate with the cluster.

## Install cert-manager

ScyllaDB Operator requires [cert-manager](https://cert-manager.io/) for TLS certificate management.
If you already have cert-manager running in your cluster, skip this step.

Install cert-manager:

```console
kubectl apply --server-side -f=https://raw.githubusercontent.com/scylladb/scylla-operator/v1.21/examples/third-party/cert-manager.yaml
```

Wait for cert-manager to become ready:

```console
kubectl wait --for='condition=established' --timeout=60s crd/certificates.cert-manager.io crd/issuers.cert-manager.io
for deploy in cert-manager{,-cainjector,-webhook}; do
    kubectl -n=cert-manager rollout status --timeout=10m deployment.apps/"${deploy}"
done
```

## Install ScyllaDB Operator

Install the ScyllaDB Operator:

```console
kubectl -n=scylla-operator apply --server-side -f=https://raw.githubusercontent.com/scylladb/scylla-operator/v1.21/deploy/operator.yaml
```

Wait for the operator to become ready:

```console
kubectl wait --for='condition=established' --timeout=60s crd/scyllaclusters.scylla.scylladb.com crd/nodeconfigs.scylla.scylladb.com crd/scyllaoperatorconfigs.scylla.scylladb.com crd/scylladbmonitorings.scylla.scylladb.com
kubectl -n=scylla-operator rollout status --timeout=10m deployment.apps/{scylla-operator,webhook-server}
```

## Install Prometheus Operator (optional)

Prometheus Operator is required only if you plan to use ScyllaDB monitoring (`ScyllaDBMonitoring` CRD).
If you do not need monitoring, skip this step.

```console
kubectl apply --server-side -f=https://raw.githubusercontent.com/scylladb/scylla-operator/v1.21/examples/third-party/prometheus-operator.yaml
```

```console
kubectl wait --for='condition=established' --timeout=60s crd/prometheuses.monitoring.coreos.com crd/servicemonitors.monitoring.coreos.com
```

## Next steps

- [Deploy ScyllaDB](https://operator.docs.scylladb.com/stable/deploy-scylladb/index.md) — choose a platform-specific reference deployment or deploy your first cluster.
