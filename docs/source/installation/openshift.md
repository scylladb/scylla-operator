# Install ScyllaDB Operator in Red Hat OpenShift

ScyllaDB Operator is a Red Hat OpenShift Certified Operator available for installation through the embedded Red Hat OpenShift OperatorHub.
This guide describes how to install ScyllaDB Operator in a Red Hat OpenShift cluster with OLM (Operator Lifecycle Manager).

## Prerequisites

This guide requires you to have access to an OpenShift Container Platform cluster using an account with `cluster-admin` permissions.

Ensure that your OpenShift cluster meets the [general prerequisites](./kubernetes-prerequisites.md) for ScyllaDB Operator installation.

Additionally, ensure that you have `kubectl` or OpenShift CLI (`oc`) installed.

:::{tip}
Commands in this guide can be executed using the OpenShift CLI (`oc`) in place of `kubectl`.
:::

## Install ScyllaDB Operator

ScyllaDB Operator can be installed in a Red Hat OpenShift cluster with one of the below methods.

:::::{tabs}

::::{group-tab} Web Console

This procedure describes how to install and subscribe to ScyllaDB Operator from OperatorHub by using the OpenShift Container Platform web console.

:::{note}
This procedure aims to follow the generic Operator installation steps outlined in the upstream documentation: [Installing from OperatorHub by using the web console](https://docs.redhat.com/en/documentation/openshift_container_platform/4.19/html/operators/administrator-tasks#olm-installing-from-operatorhub-using-web-console_olm-adding-operators-to-a-cluster).
You can refer to it for more detailed instructions, or follow the below steps for a more contained, ScyllaDB Operator specific guidance.
:::

### Procedure

1. Navigate to `Operators` -> `OperatorHub`.
2. Search for ScyllaDB Operator and select the `Certified` version by setting the `Source` filter to `Certified` or ensuring that the ScyllaDB Operator tile has the `Certified` tag next to it.
3. In the description dialog, read the information about the operator and click `Install`.
4. In the `Install Operator` dialog, configure your ScyllaDB Operator installation:
   * Select the `stable` Update Channel and {{latestStableVersion}} version from the list.
   * Select the `All namespaces on the cluster` installation mode.
   * Select the Operator recommended installed namespace: `scylla-operator`.
     :::{include} ../.internal/operator-namespace.warning.md
     :::
   * Select the `Automatic` update approval strategy to automatically upgrade ScyllaDB Operator when new versions are available.
5. Click `Install` to install and subscribe to ScyllaDB Operator.
6. Log in to the OpenShift cluster in the terminal. Ensure that `kubectl` is configured to communicate with your OpenShift cluster.
::::

::::{group-tab} CLI

This procedure describes how to install and subscribe to ScyllaDB Operator from OperatorHub by using the CLI.

:::{note}
This procedure aims to follow the generic Operator installation steps outlined in the upstream documentation: [Installing from OperatorHub by using the CLI](https://docs.redhat.com/en/documentation/openshift_container_platform/4.19/html/operators/administrator-tasks#olm-installing-operator-from-operatorhub-using-cli_olm-adding-operators-to-a-cluster).
You can refer to it for more detailed instructions, or follow the below steps for a more contained, ScyllaDB Operator specific guidance.
:::

### Procedure

1. Log in to the OpenShift cluster in the terminal. Ensure that `kubectl` is configured to communicate with your OpenShift cluster.
2. Ensure that the ScyllaDB Operator package is available:
    :::{code-block} bash
    kubectl get -n=openshift-marketplace packagemanifest scylladb-operator
    :::

    **Expected output**:
    :::{code-block} console
    NAME                CATALOG               AGE
    scylladb-operator   Certified Operators   2d22h
    :::

3. Create the `scylla-operator` namespace:
    :::{include} ../.internal/operator-namespace.warning.md
    :::

    :::{code-block} bash
    kubectl create namespace scylla-operator
    :::

4. Create an `OperatorGroup` object in the `scylla-operator` namespace:
    :::{code-block} bash
    kubectl apply --server-side -n=scylla-operator -f=- <<EOF
    apiVersion: operators.coreos.com/v1
    kind: OperatorGroup
    metadata:
      name: scylladb-operator
      namespace: scylla-operator
    EOF
    :::

5. Create a `Subscription` object to install and subscribe to ScyllaDB Operator:
    :::{code-block} bash
    :substitutions:
    kubectl apply --server-side -n=scylla-operator -f=- <<EOF
    apiVersion: operators.coreos.com/v1alpha1
    kind: Subscription
    metadata:
      name: scylladb-operator
      namespace: scylla-operator
    spec:
      channel: stable
      installPlanApproval: Automatic
      name: scylladb-operator
      source: certified-operators
      sourceNamespace: openshift-marketplace
      startingCSV: scylladb-operator.{{latestStableRelease}}
    EOF
    :::

6. Wait for the ScyllaDB Operator to be installed:
    
    Wait for `ClusterServiceVersion` to be created:
    :::{code-block} bash 
    :substitutions:
    kubectl -n=scylla-operator wait --for=create --timeout=10m csv/scylladb-operator.{{latestStableRelease}} 
    :::
    
    **Expected output**:
    :::{code-block} console
    :substitutions:
    clusterserviceversion.operators.coreos.com/scylladb-operator.{{latestStableRelease}} condition met
    :::

    Wait for `ClusterServiceVersion` to reach `Succeeded` phase:
    :::{code-block} bash 
    :substitutions:
    kubectl wait -n=scylla-operator --timeout=5m --for=jsonpath='{.status.phase}'=Succeeded clusterserviceversions.operators.coreos.com/scylladb-operator.{{latestStableRelease}}
    :::
    
    **Expected output**:
    :::{code-block} console
    :substitutions:
    clusterserviceversion.operators.coreos.com/scylladb-operator.{{latestStableRelease}} condition met
    :::
::::
:::::

### Verify the ScyllaDB Operator installation

:::{include} ./../.internal/verify-operator-installation-gitops.md
:::

## Set up local storage on dedicated nodes and enable tuning

ScyllaDB Operator enables local storage configuration and performance tuning through the [`NodeConfig`](../resources/nodeconfigs.md) resource.
The below table contains example `NodeConfig` manifests for a selected set of OpenShift deployment models and platforms:
- A production-ready configuration for Red Hat OpenShift Service on AWS (ROSA) clusters with local NVMe storage.
- A development-oriented configuration using loop devices intended for environments with no local disks available.

:::{caution}
Local storage configuration depends on the OpenShift deployment model and the underlying platform and infrastructure.
Review the [`NodeConfig`](../resources/nodeconfigs.md) reference and adjust the manifest to your specific environment.
:::

:::{include} ../.internal/node-tuning.note.md
:::

:::::{tabs}
::::{group-tab} ROSA (NVMe)

The following manifest creates a RAID0 array from the available NVMe devices, formats it with the XFS filesystem, and enables performance tuning recommended for production-grade ScyllaDB deployments on the selected nodes.
:::{literalinclude} ../../../examples/openshift/rosa/nodeconfig.yaml
:language: yaml
:::

You can apply it using the following command:

:::{code-block} shell
:substitutions:
kubectl -n=scylla-operator apply --server-side -f=https://raw.githubusercontent.com/{{repository}}/{{revision}}/examples/openshift/rosa/nodeconfig.yaml
:::
::::

::::{group-tab} Any platform (Loop devices)

The following manifest creates a loop device, formats it with the XFS filesystem, and enables performance tuning on the selected nodes.

:::{caution}
This configuration is only intended for development purposes in environments with no local NVMe disks available.
Expect performance to be significantly degraded with this setup.
:::

:::{literalinclude} ../../../examples/generic/nodeconfig-alpha.yaml
:language: yaml
:::

:::{code-block} shell
:substitutions:
kubectl -n=scylla-operator apply --server-side -f=https://raw.githubusercontent.com/{{repository}}/{{revision}}/examples/generic/nodeconfig-alpha.yaml
:::

::::

:::::

Having applied the `NodeConfig` manifest, wait for it to apply changes to the selected Kubernetes nodes:
:::{include} ./../.internal/wait-for-status-conditions.nodeconfig.code-block.md
:::

## Install Local CSI Driver

Local CSI Driver dynamically provisions PersistentVolumes on local storage configured on the nodes using the `NodeConfig` resource.
To install Local CSI Driver in a Red Hat OpenShift cluster, run the following command:

:::{code-block} shell
:substitutions:
kubectl -n=local-csi-driver apply --server-side -f=https://raw.githubusercontent.com/{{repository}}/{{revision}}/examples/common/local-volume-provisioner/local-csi-driver/{00_clusterrole_def,00_clusterrole_def_openshift,00_clusterrole,00_namespace,00_scylladb-local-xfs.storageclass,10_csidriver,10_serviceaccount,20_clusterrolebinding,50_daemonset}.yaml
:::

Having applied the manifests, wait for the Local CSI Driver `DaemonSet` to be deployed:
:::{code-block} shell
kubectl -n=local-csi-driver rollout status --timeout=10m daemonset.apps/local-csi-driver
:::

## Install ScyllaDB Manager

ScyllaDB Manager enables you to schedule second-day operations tasks, such as backups and repairs.

Run the following command to install a production-ready ScyllaDB Manager deployment:

:::{warning}
ScyllaDB Manager must run in a reserved `scylla-manager` namespace. It is [not currently possible](https://github.com/scylladb/scylla-operator/issues/2563) to use a different namespace for ScyllaDB Manager deployment.
:::

:::{code-block} shell
:substitutions:
kubectl -n=scylla-manager apply --server-side -f=https://raw.githubusercontent.com/{{repository}}/{{revision}}/deploy/manager-prod.yaml
:::

Having applied the manifest, wait for ScyllaDB Manager to deploy:
:::{code-block} shell
kubectl -n=scylla-manager rollout status --timeout=10m deployment.apps/scylla-manager
:::

## Next steps
- Deploy a ScyllaDB cluster by following the [ScyllaCluster deployment guide](../resources/scyllaclusters/basics.md).
- To set up ScyllaDB Monitoring, refer to [](../management/monitoring/external-prometheus-on-openshift.md). Visit the [ScyllaDB Monitoring overview](../management/monitoring/overview.md) for more information about the monitoring stack.
