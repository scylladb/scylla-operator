# Install with Helm

This page walks you through installing ScyllaDB Operator and all its dependencies using Helm charts. If you prefer applying raw manifests, see [Install with GitOps](install-with-gitops.md). For Red Hat OpenShift, see [Install on OpenShift](install-on-openshift.md).

:::{warning}
Helm does not support managing CustomResourceDefinition resources ([helm#5871](https://github.com/helm/helm/issues/5871), [helm#7735](https://github.com/helm/helm/issues/7735)). Helm only creates CRDs on the first install and **never updates them**. You must update CRDs manually with every Operator upgrade. For this reason, the [Install with GitOps](install-with-gitops.md) path provides a more consistent experience.
:::

:::{note}
ScyllaDB clusters can run in any Kubernetes namespace. However, **ScyllaDB Operator must run in the `scylla-operator` namespace** and **ScyllaDB Manager must run in the `scylla-manager` namespace**. Using different namespaces for these components is [not currently supported](https://github.com/scylladb/scylla-operator/issues/2563).
:::

## Prerequisites

- Kubernetes (see the supported version range in [Releases](../reference/releases.md))
- Helm 3+

## Step 1: Add the Helm chart repository

```shell
helm repo add scylla https://scylla-operator-charts.storage.googleapis.com/stable
helm repo update
```

Verify the charts are available:

```shell
helm search repo scylla
```

**Expected output:** At least three charts: `scylla/scylla-operator`, `scylla/scylla-manager`, `scylla/scylla`.

## Step 2: Install cert-manager

cert-manager provisions the TLS certificate for ScyllaDB Operator's webhook server. If you already have cert-manager installed in your cluster, you can skip this step. If you want to provide your own webhook certificate, set `webhook.createSelfSignedCertificate: false` and provide `webhook.certificateSecretName` when installing the Operator chart.

You can install cert-manager using the bundled manifest:

:::{code-block} shell
:substitutions:
kubectl apply --server-side -f=https://raw.githubusercontent.com/{{repository}}/{{revision}}/examples/third-party/cert-manager.yaml
:::

Or follow the [upstream cert-manager installation instructions](https://cert-manager.io/docs/install-operator/).

Wait for cert-manager to become available:

```shell
kubectl wait -n=cert-manager --for='condition=ready' pod -l app=cert-manager --timeout=60s
kubectl wait -n=cert-manager --for='condition=ready' pod -l app=cainjector --timeout=60s
kubectl wait -n=cert-manager --for='condition=ready' pod -l app=webhook --timeout=60s
```

## Step 3: Install Prometheus Operator

:::{note}
ScyllaDB Operator references Prometheus Operator CRDs in its RBAC rules. If the CRDs are not installed, the Operator may log warnings about missing Prometheus types. These warnings do not affect core functionality. Support for making this dependency fully optional is tracked in [#3075](https://github.com/scylladb/scylla-operator/issues/3075).
:::

:::{code-block} shell
:substitutions:
kubectl apply -n=prometheus-operator --server-side -f=https://raw.githubusercontent.com/{{repository}}/{{revision}}/examples/third-party/prometheus-operator.yaml
:::

```shell
kubectl -n=prometheus-operator rollout status --timeout=10m deployment.apps/prometheus-operator
```

## Step 4: Install ScyllaDB Operator

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

Wait for the Operator to become available:

```shell
kubectl -n=scylla-operator rollout status --timeout=10m deployment.apps/scylla-operator
kubectl -n=scylla-operator rollout status --timeout=10m deployment.apps/webhook-server
```

**Expected output:** Both deployments report `successfully rolled out`.

## Step 5: Set up NodeConfig

NodeConfig configures local storage (RAID, filesystem, mount points) and performance tuning on the Kubernetes nodes where ScyllaDB will run. This step is the same as in the [Install with GitOps](install-with-gitops.md#step-4-set-up-nodeconfig).

:::{caution}
The NodeConfig manifest depends on your platform, machine type, and node pool configuration. Review the example for your platform and adjust it if your disk layout differs. See [Configure nodes](../deploy-scylladb/before-you-deploy/configure-nodes.md) for details.
:::

::::{tabs}
:::{group-tab} GKE (NVMe)

:::{caution}
Starting with GKE version `1.32.1-gke.1002000`, the Ubuntu image no longer provides `xfsprogs` by default. Install it before applying NodeConfig — see [XFS filesystem tools](prerequisites.md#xfs-filesystem-tools).
:::

:::{code-block} shell
:substitutions:
kubectl apply --server-side -f=https://raw.githubusercontent.com/{{repository}}/{{revision}}/examples/gke/nodeconfig-alpha.yaml
:::

:::

:::{group-tab} EKS (NVMe)

:::{code-block} shell
:substitutions:
kubectl apply --server-side -f=https://raw.githubusercontent.com/{{repository}}/{{revision}}/examples/eks/nodeconfig-alpha.yaml
:::

:::

:::{group-tab} Any platform (loop devices — dev only)

:::{caution}
This NodeConfig sets up loop devices instead of NVMe disks and is intended for development and testing only.
:::

:::{code-block} shell
:substitutions:
kubectl apply --server-side -f=https://raw.githubusercontent.com/{{repository}}/{{revision}}/examples/generic/nodeconfig-alpha.yaml
:::

:::
::::

Wait for NodeConfig to finish applying changes:

```shell
kubectl wait --timeout=10m --for='condition=Progressing=False' nodeconfigs.scylla.scylladb.com/scylladb-nodepool-1
kubectl wait --timeout=10m --for='condition=Degraded=False' nodeconfigs.scylla.scylladb.com/scylladb-nodepool-1
kubectl wait --timeout=10m --for='condition=Available=True' nodeconfigs.scylla.scylladb.com/scylladb-nodepool-1
```

## Step 6: Install the Local CSI Driver

The Local CSI Driver dynamically provisions PersistentVolumes from the local storage set up by NodeConfig. This step is the same as in the [Install with GitOps](install-with-gitops.md#step-5-install-the-local-csi-driver).

:::{code-block} shell
:substitutions:
kubectl -n=local-csi-driver apply --server-side -f=https://raw.githubusercontent.com/{{repository}}/{{revision}}/examples/common/local-volume-provisioner/local-csi-driver/{00_clusterrole_def,00_clusterrole_def_openshift,00_clusterrole,00_namespace,00_scylladb-local-xfs.storageclass,10_csidriver,10_serviceaccount,20_clusterrolebinding,50_daemonset}.yaml
:::

```shell
kubectl -n=local-csi-driver rollout status --timeout=10m daemonset.apps/local-csi-driver
```

**Expected output:** The DaemonSet reports all pods ready. The `scylladb-local-xfs` StorageClass is now available.

## Step 7: Install ScyllaDB Manager

```shell
helm install scylla-manager scylla/scylla-manager \
  --create-namespace \
  --namespace scylla-manager
```

:::{note}
ScyllaDB Manager is available for ScyllaDB Enterprise customers and ScyllaDB users. With ScyllaDB, ScyllaDB Manager is limited to 5 nodes. See the ScyllaDB Manager [Proprietary Software License Agreement](https://www.scylladb.com/scylla-manager-software-license-agreement/) for details.
:::

Wait for the Manager to become available:

```shell
kubectl -n=scylla-manager rollout status --timeout=10m deployment.apps/scylla-manager
```

## Step 8: Set up monitoring (optional)

To deploy managed Prometheus and Grafana instances for ScyllaDB metrics and dashboards, see [Set up monitoring](../deploy-scylladb/set-up-monitoring.md).

## Next steps

You now have ScyllaDB Operator, storage, and Manager installed. To deploy your first ScyllaDB cluster, see [Deploy a single-DC cluster](../deploy-scylladb/deploy-single-dc-cluster.md).

For production environments, review the [Production checklist](../deploy-scylladb/production-checklist.md) to verify that all recommended settings are in place.

## Cleanup

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

- [Prerequisites](prerequisites.md) — Kubernetes version requirements and platform-specific setup.
- [Install with GitOps](install-with-gitops.md) — alternative installation path using manifests.
- [Install on OpenShift](install-on-openshift.md) — installation path for Red Hat OpenShift via OLM.
- [Configure nodes](../deploy-scylladb/before-you-deploy/configure-nodes.md) — customizing NodeConfig for your environment.
