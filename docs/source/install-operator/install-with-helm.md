# Install with Helm

This page walks you through installing ScyllaDB Operator and its dependencies using Helm charts. If you prefer applying raw manifests, see [Install with GitOps](install-with-gitops.md). For Red Hat OpenShift, see [Install on OpenShift](install-on-openshift.md).

:::{note}
The Helm installation path supports single-datacenter deployments only.
For multi-datacenter ScyllaDB clusters, use the [GitOps installation path](install-with-gitops.md) and follow [Deploy a multi-datacenter cluster](../deploy-scylladb/deploy-multi-dc-cluster.md).
:::

:::{warning}
Helm does not support managing CustomResourceDefinition resources ([helm#5871](https://github.com/helm/helm/issues/5871), [helm#7735](https://github.com/helm/helm/issues/7735)). Helm only creates CRDs on the first install and **never updates them**. You must update CRDs manually with every Operator upgrade. For this reason, the [Install with GitOps](install-with-gitops.md) path provides a more consistent experience.
:::

:::{note}
ScyllaDB Operator must run in the `scylla-operator` namespace.
:::

## Prerequisites

- A Kubernetes cluster meeting the [infrastructure requirements](provision-infrastructure/overview.md).
- [`kubectl`](https://kubernetes.io/docs/tasks/tools/#kubectl) configured to communicate with the cluster.
- [Helm 3+](https://helm.sh/docs/intro/install/) installed.

## Add the Helm chart repository

```shell
helm repo add scylla https://scylla-operator-charts.storage.googleapis.com/stable
helm repo update
```

Verify the charts are available:

```shell
helm search repo scylla
```

**Expected output:** At least three charts: `scylla/scylla-operator`, `scylla/scylla-manager`, `scylla/scylla`.

## Install cert-manager

cert-manager provisions the TLS certificate for ScyllaDB Operator's webhook server. If you already have cert-manager installed in your cluster, skip this step. If you want to provide your own webhook certificate, set `webhook.createSelfSignedCertificate: false` and provide `webhook.certificateSecretName` when installing the Operator chart.

You can install cert-manager using the bundled manifest:

:::{code-block} shell
:substitutions:
kubectl apply --server-side -f=https://raw.githubusercontent.com/{{repository}}/{{revision}}/examples/third-party/cert-manager.yaml
:::

Or follow the [upstream cert-manager installation instructions](https://cert-manager.io/docs/installation/).

Wait for cert-manager to become available:

```shell
kubectl wait -n=cert-manager --for='condition=ready' pod -l app=cert-manager --timeout=60s
kubectl wait -n=cert-manager --for='condition=ready' pod -l app=cainjector --timeout=60s
kubectl wait -n=cert-manager --for='condition=ready' pod -l app=webhook --timeout=60s
```

## Install ScyllaDB Operator

```shell
helm install scylla-operator scylla/scylla-operator \
  --create-namespace \
  --namespace scylla-operator
```

To customize the deployment (image, resources, replicas, log level), create a values file and pass it with `--values`:

```shell
helm install scylla-operator scylla/scylla-operator \
  --create-namespace \
  --namespace scylla-operator \
  --values my-operator-values.yaml
```

See the chart's {{ '[values.yaml](https://raw.githubusercontent.com/{}/{}/helm/scylla-operator/values.yaml)'.format(repository, revision) }} for all available options.

Wait for ScyllaDB Operator to become available:

```shell
kubectl -n=scylla-operator rollout status --timeout=10m deployment.apps/scylla-operator
kubectl -n=scylla-operator rollout status --timeout=10m deployment.apps/webhook-server
```

**Expected output:** Both Deployments report `successfully rolled out`.

## Install Prometheus Operator (optional)

Prometheus Operator is required only if you plan to use ScyllaDB monitoring (`ScyllaDBMonitoring` CRD).
If you do not need monitoring, skip this step.

:::{code-block} shell
:substitutions:
kubectl apply -n=prometheus-operator --server-side -f=https://raw.githubusercontent.com/{{repository}}/{{revision}}/examples/third-party/prometheus-operator.yaml
:::

```shell
kubectl -n=prometheus-operator rollout status --timeout=10m deployment.apps/prometheus-operator
```

## Next steps

- [Deploy ScyllaDB](../deploy-scylladb/overview.md) — choose a platform-specific reference deployment or deploy your first cluster.

## Clean up

To remove the Helm releases:

```shell
helm uninstall scylla -n scylla
helm uninstall scylla-manager -n scylla-manager
helm uninstall scylla-operator -n scylla-operator
```

:::{note}
Helm uninstall does not remove CRDs. To fully clean up, delete the CRDs manually after uninstalling:

```shell
kubectl delete crd scyllaclusters.scylla.scylladb.com nodeconfigs.scylla.scylladb.com scyllaoperatorconfigs.scylla.scylladb.com scylladbmonitorings.scylla.scylladb.com
```
:::

## Related pages

- [Prerequisites](overview.md) — Kubernetes version requirements and platform-specific setup.
- [Install with GitOps](install-with-gitops.md) — alternative installation path using manifests.
- [Install on OpenShift](install-on-openshift.md) — installation path for Red Hat OpenShift via OLM.
