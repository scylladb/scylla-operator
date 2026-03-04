# GitOps installation

This page walks you through installing ScyllaDB Operator and all its dependencies using `kubectl apply` with manifests from the project repository. If you prefer Helm, see [Helm installation](helm.md).

:::{note}
ScyllaDB clusters can run in any Kubernetes namespace. However, **ScyllaDB Operator must run in the `scylla-operator` namespace** and **ScyllaDB Manager must run in the `scylla-manager` namespace**. Using different namespaces for these components is [not currently supported](https://github.com/scylladb/scylla-operator/issues/2563).
:::

:::{caution}
The commands below use `{{revision}}` references to match the documentation version you are reading. For production deployments, always pin manifest and image references to a full version (e.g., `1.19.0`) or SHA digest. Do not use rolling tags like `latest` or `1.19` in production — the manifests and images for a particular release are tightly coupled.
:::

## Step 1: Install cert-manager

cert-manager provisions the TLS certificate for ScyllaDB Operator's webhook server. Install it first and wait for it to become available.

:::{code-block} shell
:substitutions:
kubectl apply --server-side -f=https://raw.githubusercontent.com/{{repository}}/{{revision}}/examples/third-party/cert-manager.yaml
:::

```shell
# Wait for CRDs to propagate to all API servers.
kubectl wait --for='condition=established' --timeout=60s \
  crd/certificates.cert-manager.io \
  crd/issuers.cert-manager.io

# Wait for all cert-manager deployments.
for deploy in cert-manager cert-manager-cainjector cert-manager-webhook; do
    kubectl -n=cert-manager rollout status --timeout=10m deployment.apps/"${deploy}"
done

# Wait for the webhook CA secret to be created.
for i in $(seq 1 30); do
    kubectl -n=cert-manager get secret/cert-manager-webhook-ca && break || sleep 1
done
```

**Expected output:** All three deployments report `successfully rolled out` and the `cert-manager-webhook-ca` secret exists.

## Step 2: Install Prometheus Operator

Prometheus Operator provides the CRDs and controllers for running managed Prometheus and Grafana instances via `ScyllaDBMonitoring`.

:::{note}
ScyllaDB Operator references Prometheus Operator CRDs in its RBAC rules. If the CRDs are not installed, the Operator may log warnings about missing Prometheus types. These warnings do not affect core functionality (cluster creation, scaling, upgrades). Support for making this dependency fully optional is tracked in [#3075](https://github.com/scylladb/scylla-operator/issues/3075).
:::

:::{code-block} shell
:substitutions:
kubectl apply -n=prometheus-operator --server-side -f=https://raw.githubusercontent.com/{{repository}}/{{revision}}/examples/third-party/prometheus-operator.yaml
:::

```shell
# Wait for CRDs to propagate.
kubectl wait --for='condition=established' --timeout=60s \
  crd/prometheuses.monitoring.coreos.com \
  crd/prometheusrules.monitoring.coreos.com \
  crd/servicemonitors.monitoring.coreos.com

# Wait for the Prometheus Operator deployment.
kubectl -n=prometheus-operator rollout status --timeout=10m deployment.apps/prometheus-operator
```

**Expected output:** The deployment reports `successfully rolled out`.

## Step 3: Install ScyllaDB Operator

:::{code-block} shell
:substitutions:
kubectl -n=scylla-operator apply --server-side -f=https://raw.githubusercontent.com/{{repository}}/{{revision}}/deploy/operator.yaml
:::

::::{caution}
The ScyllaDB Operator deployment references its own image in an environment variable (`SCYLLA_OPERATOR_IMAGE`) that is injected into ScyllaDB pods. When pinning images for production, you must update **both** the container `image` and the `SCYLLA_OPERATOR_IMAGE` environment variable to the same reference:

:::{code-block} yaml
:substitutions:
:linenos:
:emphasize-lines: 6,9
containers:
- name: scylla-operator
  image: docker.io/scylladb/scylla-operator:{{latestStableImageTag}}
  env:
  - name: SCYLLA_OPERATOR_IMAGE
    value: docker.io/scylladb/scylla-operator:{{latestStableImageTag}}
:::

These two values must always match. A mismatch can cause version skew between the Operator and the sidecar running inside ScyllaDB pods.
::::

```shell
# Wait for CRDs to propagate.
kubectl wait --for='condition=established' --timeout=60s \
  crd/scyllaclusters.scylla.scylladb.com \
  crd/nodeconfigs.scylla.scylladb.com \
  crd/scyllaoperatorconfigs.scylla.scylladb.com \
  crd/scylladbmonitorings.scylla.scylladb.com

# Wait for the Operator and webhook server deployments.
kubectl -n=scylla-operator rollout status --timeout=10m deployment.apps/scylla-operator
kubectl -n=scylla-operator rollout status --timeout=10m deployment.apps/webhook-server
```

**Expected output:** Both deployments report `successfully rolled out`.

## Step 4: Set up NodeConfig

NodeConfig configures local storage (RAID, filesystem, mount points) and performance tuning on the Kubernetes nodes where ScyllaDB will run.

:::{caution}
The NodeConfig manifest depends on your platform, machine type, and node pool configuration. Review the example for your platform and adjust it if your disk layout differs. See [Node configuration](../deploying/node-configuration.md) for details.
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
This NodeConfig sets up loop devices instead of NVMe disks and is intended for development and testing only. Do not expect production-level performance.
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

**Expected output:** All three conditions are satisfied. If `Degraded` is `True` or `Available` is `False` after the timeout, check the NodeConfig status and node events for errors.

## Step 5: Install the Local CSI Driver

The Local CSI Driver dynamically provisions PersistentVolumes from the local storage set up by NodeConfig.

:::{code-block} shell
:substitutions:
kubectl -n=local-csi-driver apply --server-side -f=https://raw.githubusercontent.com/{{repository}}/{{revision}}/examples/common/local-volume-provisioner/local-csi-driver/{00_clusterrole_def,00_clusterrole_def_openshift,00_clusterrole,00_namespace,00_scylladb-local-xfs.storageclass,10_csidriver,10_serviceaccount,20_clusterrolebinding,50_daemonset}.yaml
:::

```shell
# Wait for the DaemonSet to deploy on all selected nodes.
kubectl -n=local-csi-driver rollout status --timeout=10m daemonset.apps/local-csi-driver
```

**Expected output:** The DaemonSet reports all pods ready. The `scylladb-local-xfs` StorageClass is now available.

## Step 6: Install ScyllaDB Manager

ScyllaDB Manager provides automated repair and backup scheduling. It deploys a small internal ScyllaDB cluster for its own database.

:::{note}
ScyllaDB Manager is available for ScyllaDB Enterprise customers and ScyllaDB Open Source users. With ScyllaDB Open Source, ScyllaDB Manager is limited to 5 nodes. See the ScyllaDB Manager [Proprietary Software License Agreement](https://www.scylladb.com/scylla-manager-software-license-agreement/) for details.
:::

::::{tabs}
:::{group-tab} Production

:::{code-block} shell
:substitutions:
kubectl -n=scylla-manager apply --server-side -f=https://raw.githubusercontent.com/{{repository}}/{{revision}}/deploy/manager-prod.yaml
:::

:::

:::{group-tab} Development

:::{code-block} shell
:substitutions:
kubectl -n=scylla-manager apply --server-side -f=https://raw.githubusercontent.com/{{repository}}/{{revision}}/deploy/manager-dev.yaml
:::

:::
::::

```shell
# Wait for the Manager deployment.
kubectl -n=scylla-manager rollout status --timeout=10m deployment.apps/scylla-manager
```

**Expected output:** The deployment reports `successfully rolled out`.

## Step 7: Set up monitoring (optional)

To deploy managed Prometheus and Grafana instances for ScyllaDB metrics and dashboards, see [Monitoring setup](../deploying/monitoring.md).

## Next steps

You now have ScyllaDB Operator, storage, and Manager installed. To deploy your first ScyllaDB cluster, see [Deploying a single-DC cluster](../deploying/single-dc-cluster.md).

For production environments, review the [Production checklist](../deploying/production-checklist.md) to verify that all recommended settings are in place.

## Related pages

- [Prerequisites](prerequisites.md) — Kubernetes version requirements and platform-specific setup.
- [Helm installation](helm.md) — alternative installation path using Helm charts.
- [Node configuration](../deploying/node-configuration.md) — customizing NodeConfig for your environment.
- [Architecture overview](../architecture/overview.md) — how the Operator components fit together.
