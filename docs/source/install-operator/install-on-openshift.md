# Install on OpenShift

This page walks you through installing ScyllaDB Operator and all its dependencies on Red Hat OpenShift using the Operator Lifecycle Manager (OLM) software catalog. If you are running on a generic Kubernetes distribution, see [Install with GitOps](install-with-gitops.md) or [Install with Helm](install-with-helm.md) instead.

ScyllaDB Operator is a Red Hat OpenShift Certified Operator. It is available in the embedded software catalog and can be installed through the OpenShift web console or CLI.

:::{note}
**ScyllaDB Operator must run in the `scylla-operator` namespace** and **ScyllaDB Manager must run in the `scylla-manager` namespace**. Using different namespaces for these components is [not currently supported](https://github.com/scylladb/scylla-operator/issues/2563).
:::

## Prerequisites

- An OpenShift Container Platform cluster (see the supported version range in [Releases](../reference/releases.md))
- An account with `cluster-admin` permissions
- `kubectl` or OpenShift CLI (`oc`) installed

Ensure that your cluster meets the [general prerequisites](prerequisites.md) for ScyllaDB Operator installation.

:::{tip}
All `kubectl` commands in this guide can be executed using the OpenShift CLI (`oc`) instead.
:::

## Step 1: Install ScyllaDB Operator

ScyllaDB Operator can be installed from the OpenShift software catalog using either the web console or the CLI.

:::::{tabs}

::::{group-tab} Web Console

This procedure follows the generic Operator installation steps outlined in the upstream documentation: [Installing from the software catalog by using the web console](https://docs.redhat.com/en/documentation/openshift_container_platform/4.20/html/operators/administrator-tasks#olm-installing-from-software-catalog-using-web-console_olm-adding-operators-to-a-cluster).

### Procedure

1. Navigate to **Ecosystem** → **Software Catalog**.
2. Search for **ScyllaDB Operator** and select the **Certified** version by setting the **Source** filter to **Certified**, or by verifying that the ScyllaDB Operator tile has the **Certified** tag.
3. Read the description and click **Install**.
4. In the **Install Operator** dialog, configure the installation:
   - For clusters on AWS with Security Token Service (STS): enter the Amazon Resource Name (ARN) of the AWS IAM role for your service account in the role ARN field.
   - Select the **stable** Update Channel.
   - Select the **All namespaces on the cluster** installation mode.
   - Select the Operator recommended installed namespace: **scylla-operator**.
   - Select the **Manual** update approval strategy to manually approve ScyllaDB Operator upgrades when new versions are available.

   :::{caution}
   Always review the [Upgrade ScyllaDB Operator](../upgrade/upgrade-operator.md) guide before approving a new version.
   :::

5. Click **Install**.
6. In the **Install Plan** dialog, review the manual install plan and click **Approve**.
7. Log in to the OpenShift cluster in the terminal. Ensure that `kubectl` is configured to communicate with your cluster.

::::

::::{group-tab} CLI

This procedure follows the generic Operator installation steps outlined in the upstream documentation: [Installing from the software catalog by using the CLI](https://docs.redhat.com/en/documentation/openshift_container_platform/4.20/html/operators/administrator-tasks#olm-installing-operator-from-software-catalog-using-cli_olm-adding-operators-to-a-cluster).

### Procedure

1. Log in to the OpenShift cluster in the terminal. Ensure that `kubectl` is configured to communicate with your cluster.

2. Verify that the ScyllaDB Operator package is available:

    ```shell
    kubectl get -n=openshift-marketplace packagemanifest scylladb-operator
    ```

    **Expected output:**

    ```console
    NAME                CATALOG               AGE
    scylladb-operator   Certified Operators   2d22h
    ```

3. Create the `scylla-operator` namespace:

    ```shell
    kubectl create namespace scylla-operator
    ```

4. Create an `OperatorGroup` in the `scylla-operator` namespace:

    ```shell
    kubectl apply --server-side -n=scylla-operator -f=- <<EOF
    apiVersion: operators.coreos.com/v1
    kind: OperatorGroup
    metadata:
      name: scylladb-operator
      namespace: scylla-operator
    EOF
    ```

5. Create a `Subscription` to install ScyllaDB Operator.

    Clusters on cloud providers with token-based authentication require additional fields in the `Subscription` config section. Apply the appropriate manifest based on your environment:

    :::::{tabs}
    ::::{group-tab} AWS STS

    If the cluster uses AWS Security Token Service (STS), include the role ARN in the `Subscription` manifest:

    :::{code-block} shell
    :substitutions:
    kubectl apply --server-side -n=scylla-operator -f=- <<EOF
    apiVersion: operators.coreos.com/v1alpha1
    kind: Subscription
    metadata:
      name: scylladb-operator
      namespace: scylla-operator
    spec:
      channel: stable
      installPlanApproval: Manual
      name: scylladb-operator
      source: certified-operators
      sourceNamespace: openshift-marketplace
      startingCSV: scylladb-operator.{{latestStableRelease}}
      config:
        env:
        - name: ROLEARN
          value: "<role_arn>"
    EOF
    :::

    Replace `<role_arn>` with the ARN of the AWS IAM role for your service account.

    ::::

    ::::{group-tab} Generic

    :::{code-block} shell
    :substitutions:
    kubectl apply --server-side -n=scylla-operator -f=- <<EOF
    apiVersion: operators.coreos.com/v1alpha1
    kind: Subscription
    metadata:
      name: scylladb-operator
      namespace: scylla-operator
    spec:
      channel: stable
      installPlanApproval: Manual
      name: scylladb-operator
      source: certified-operators
      sourceNamespace: openshift-marketplace
      startingCSV: scylladb-operator.{{latestStableRelease}}
    EOF
    :::

    ::::
    :::::

6. Locate, review, and approve the generated `InstallPlan`:

    ```shell
    kubectl -n=scylla-operator get installplan -l=operators.coreos.com/scylladb-operator.scylla-operator=""
    ```

    **Example expected output:**

    :::{code-block} console
    :substitutions:
    NAME            CSV                                    APPROVAL   APPROVED
    install-tw6bv   scylladb-operator.{{latestStableRelease}}   Manual     false
    :::

    Review the `InstallPlan`:

    ```shell
    kubectl -n=scylla-operator describe installplan <install-plan-name>
    ```

    Approve it to start the installation:

    ```shell
    kubectl -n=scylla-operator patch installplan --type=merge -p='{"spec":{"approved":true}}' <install-plan-name>
    ```

    **Example expected output:**

    ```console
    installplan.operators.coreos.com/install-tw6bv patched
    ```

7. Wait for the `ClusterServiceVersion` to reach `Succeeded` phase:

    :::{code-block} shell
    :substitutions:
    kubectl -n=scylla-operator wait --for=create --timeout=10m csv/scylladb-operator.{{latestStableRelease}}
    kubectl -n=scylla-operator wait --timeout=5m --for=jsonpath='{.status.phase}'=Succeeded clusterserviceversions.operators.coreos.com/scylladb-operator.{{latestStableRelease}}
    :::

    **Expected output:**

    :::{code-block} console
    :substitutions:
    clusterserviceversion.operators.coreos.com/scylladb-operator.{{latestStableRelease}} condition met
    clusterserviceversion.operators.coreos.com/scylladb-operator.{{latestStableRelease}} condition met
    :::

::::
:::::

### Verify the installation

Wait for CRDs to propagate to all API servers:

```shell
kubectl wait --for='condition=established' --timeout=60s \
  crd/scyllaclusters.scylla.scylladb.com \
  crd/nodeconfigs.scylla.scylladb.com \
  crd/scyllaoperatorconfigs.scylla.scylladb.com \
  crd/scylladbmonitorings.scylla.scylladb.com
```

**Expected output:**

```console
customresourcedefinition.apiextensions.k8s.io/scyllaclusters.scylla.scylladb.com condition met
customresourcedefinition.apiextensions.k8s.io/nodeconfigs.scylla.scylladb.com condition met
customresourcedefinition.apiextensions.k8s.io/scyllaoperatorconfigs.scylla.scylladb.com condition met
customresourcedefinition.apiextensions.k8s.io/scylladbmonitorings.scylla.scylladb.com condition met
```

Wait for the Operator and webhook server deployments:

```shell
kubectl -n=scylla-operator rollout status --timeout=10m deployment.apps/scylla-operator
kubectl -n=scylla-operator rollout status --timeout=10m deployment.apps/webhook-server
```

**Expected output:**

```console
deployment "scylla-operator" successfully rolled out
deployment "webhook-server" successfully rolled out
```

## Step 2: Set up NodeConfig

NodeConfig configures local storage (RAID, filesystem, mount points) and performance tuning on the Kubernetes nodes where ScyllaDB will run.

:::{caution}
Local storage configuration depends on the OpenShift deployment model and the underlying platform and infrastructure. Review the examples below and adjust the manifest to your specific environment. See [Configure nodes](../deploy-scylladb/configure-nodes.md) for details.
:::

:::{note}
Performance tuning is enabled for all nodes that are selected by `NodeConfig` by default, unless explicitly opted-out of.
:::

:::::{tabs}
::::{group-tab} ROSA (NVMe)

The following manifest creates a RAID0 array from the available NVMe devices, formats it with the XFS filesystem, and enables performance tuning recommended for production-grade ScyllaDB deployments on the selected nodes.

:::{literalinclude} ../../../examples/openshift/rosa/nodeconfig.yaml
:language: yaml
:::

:::{code-block} shell
:substitutions:
kubectl apply --server-side -f=https://raw.githubusercontent.com/{{repository}}/{{revision}}/examples/openshift/rosa/nodeconfig.yaml
:::

::::

::::{group-tab} Any platform (loop devices — dev only)

The following manifest creates a loop device, formats it with the XFS filesystem, and enables performance tuning on the selected nodes.

:::{caution}
This configuration is only intended for development purposes in environments with no local NVMe disks available. Do not expect production-level performance.
:::

:::{code-block} shell
:substitutions:
kubectl apply --server-side -f=https://raw.githubusercontent.com/{{repository}}/{{revision}}/examples/generic/nodeconfig-alpha.yaml
:::

::::
:::::

Wait for NodeConfig to finish applying changes:

```shell
kubectl wait --timeout=10m --for='condition=Progressing=False' nodeconfigs.scylla.scylladb.com/scylladb-nodepool-1
kubectl wait --timeout=10m --for='condition=Degraded=False' nodeconfigs.scylla.scylladb.com/scylladb-nodepool-1
kubectl wait --timeout=10m --for='condition=Available=True' nodeconfigs.scylla.scylladb.com/scylladb-nodepool-1
```

**Expected output:** All three conditions are satisfied. If `Degraded` is `True` or `Available` is `False` after the timeout, check the NodeConfig status and node events for errors.

## Step 3: Install the Local CSI Driver

The Local CSI Driver dynamically provisions PersistentVolumes from the local storage set up by NodeConfig.

The OpenShift installation includes an additional ClusterRole definition (`00_clusterrole_def_openshift`) that grants the CSI driver access to the `privileged` SecurityContextConstraints required to manage local disks.

:::{code-block} shell
:substitutions:
kubectl -n=local-csi-driver apply --server-side -f=https://raw.githubusercontent.com/{{repository}}/{{revision}}/examples/common/local-volume-provisioner/local-csi-driver/{00_clusterrole_def,00_clusterrole_def_openshift,00_clusterrole,00_namespace,00_scylladb-local-xfs.storageclass,10_csidriver,10_serviceaccount,20_clusterrolebinding,50_daemonset}.yaml
:::

```shell
kubectl -n=local-csi-driver rollout status --timeout=10m daemonset.apps/local-csi-driver
```

**Expected output:** The DaemonSet reports all pods ready. The `scylladb-local-xfs` StorageClass is now available.

## Step 4: Install ScyllaDB Manager

ScyllaDB Manager provides automated repair and backup scheduling. It deploys a small internal ScyllaDB cluster for its own database.

:::{note}
ScyllaDB Manager is available for ScyllaDB Enterprise customers and ScyllaDB Open Source users. With ScyllaDB Open Source, ScyllaDB Manager is limited to 5 nodes. See the ScyllaDB Manager [Proprietary Software License Agreement](https://www.scylladb.com/scylla-manager-software-license-agreement/) for details.
:::

:::{code-block} shell
:substitutions:
kubectl -n=scylla-manager apply --server-side -f=https://raw.githubusercontent.com/{{repository}}/{{revision}}/deploy/manager-prod.yaml
:::

```shell
kubectl -n=scylla-manager rollout status --timeout=10m deployment.apps/scylla-manager
```

**Expected output:** The deployment reports `successfully rolled out`.

## Step 5: Set up monitoring (optional)

On OpenShift, you can use the built-in User Workload Monitoring with Prometheus instead of deploying a standalone instance. See [Set up monitoring](../deploy-scylladb/set-up-monitoring.md#openshift-user-workload-monitoring) for OpenShift-specific instructions.

## Next steps

You now have ScyllaDB Operator, storage, and Manager installed. To deploy your first ScyllaDB cluster, see [Deploy a single-DC cluster](../deploy-scylladb/deploy-single-dc-cluster.md).

For production environments, review the [Production checklist](../deploy-scylladb/production-checklist.md) to verify that all recommended settings are in place.

## Related pages

- [Prerequisites](prerequisites.md) — Kubernetes and OpenShift version requirements, platform-specific setup.
- [Install with GitOps](install-with-gitops.md) — alternative installation path using manifests (generic Kubernetes).
- [Install with Helm](install-with-helm.md) — alternative installation path using Helm charts.
- [Configure nodes](../deploy-scylladb/configure-nodes.md) — customizing NodeConfig for your environment.
- [Upgrade ScyllaDB Operator](../upgrade/upgrade-operator.md) — version-specific upgrade steps.
