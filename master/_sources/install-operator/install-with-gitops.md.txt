# Install with GitOps

Install ScyllaDB Operator and its dependencies by applying raw manifests from the project repository.
This method works with any GitOps tool (Argo CD, Flux, etc.) or plain `kubectl apply`.

:::{caution}
The manifests in this guide use a rolling tag for the ScyllaDB Operator image.
For production deployments, replace the image reference in all manifests with a pinned version tag or SHA digest.
:::

:::{note}
ScyllaDB Operator must run in the `scylla-operator` namespace.
:::

## Prerequisites

- A Kubernetes cluster meeting the [infrastructure requirements](provision-infrastructure/index.md).
- [`kubectl`](https://kubernetes.io/docs/tasks/tools/#kubectl) configured to communicate with the cluster.

## Install cert-manager

ScyllaDB Operator requires [cert-manager](https://cert-manager.io/) for TLS certificate management.
If you already have cert-manager running in your cluster, skip this step.

Install cert-manager:

:::{code-block} console
:substitutions:
kubectl apply --server-side -f=https://raw.githubusercontent.com/{{repository}}/{{revision}}/examples/third-party/cert-manager.yaml
:::

Wait for cert-manager to become ready:

:::{code-block} console
kubectl wait --for='condition=established' --timeout=60s crd/certificates.cert-manager.io crd/issuers.cert-manager.io
for deploy in cert-manager{,-cainjector,-webhook}; do
    kubectl -n=cert-manager rollout status --timeout=10m deployment.apps/"${deploy}"
done
:::

## Install ScyllaDB Operator

Install the ScyllaDB Operator:

:::{code-block} console
:substitutions:
kubectl -n=scylla-operator apply --server-side -f=https://raw.githubusercontent.com/{{repository}}/{{revision}}/deploy/operator.yaml
:::

:::{caution}
The ScyllaDB Operator deployment contains a `SCYLLA_OPERATOR_IMAGE` environment variable that must always match the operator's container image.
If you pin the image to a specific version, update both the `image` field and the `SCYLLA_OPERATOR_IMAGE` value:

```{code-block} yaml
:substitutions:
:linenos:
:emphasize-lines: 5,8
# In deploy/operator.yaml — Deployment scylla-operator
containers:
- name: scylla-operator
  # ...
  image: docker.io/scylladb/scylla-operator:{{latestStableVersion}}
  env:
  - name: SCYLLA_OPERATOR_IMAGE
    value: docker.io/scylladb/scylla-operator:{{latestStableVersion}}
```

The two values must always match.
Using a rolling tag for one and a pinned version for the other causes version skew.
:::

Wait for the operator to become ready:

:::{code-block} console
kubectl wait --for='condition=established' --timeout=60s crd/scyllaclusters.scylla.scylladb.com crd/nodeconfigs.scylla.scylladb.com crd/scyllaoperatorconfigs.scylla.scylladb.com crd/scylladbmonitorings.scylla.scylladb.com
kubectl -n=scylla-operator rollout status --timeout=10m deployment.apps/{scylla-operator,webhook-server}
:::

## Install Prometheus Operator (optional)

Prometheus Operator is required only if you plan to use ScyllaDB monitoring (`ScyllaDBMonitoring` CRD).
If you do not need monitoring, skip this step.

:::{code-block} console
:substitutions:
kubectl apply --server-side -f=https://raw.githubusercontent.com/{{repository}}/{{revision}}/examples/third-party/prometheus-operator.yaml
:::

:::{code-block} console
kubectl wait --for='condition=established' --timeout=60s crd/prometheuses.monitoring.coreos.com crd/servicemonitors.monitoring.coreos.com
:::

## Next steps

- [Deploy ScyllaDB](../deploy-scylladb/index.md) — choose a platform-specific reference deployment or deploy your first cluster.
