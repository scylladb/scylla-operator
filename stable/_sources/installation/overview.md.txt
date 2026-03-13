# Overview

## Kubernetes

ScyllaDB Operator is a set of controllers and Kubernetes API extensions.
Therefore, we assume you either have an existing, conformant Kubernetes cluster and/or are already familiar with how a Kubernetes cluster is deployed and operated.

ScyllaDB Operator controllers and API extensions may have dependencies on some of the newer Kubernetes features and APIs that need to be available.
More over, ScyllaDB Operator implements additional features like performance tuning, some of which are platform/OS specific.
While we do our best to implement these routines as generically as possible, sometimes there isn't any low level API to base them on and they may work only on a subset of platforms.

:::{caution}
We *strongly* recommend using a [supported Kubernetes environment](../support/releases.md#supported-kubernetes-environments).
Issues on unsupported environments are unlikely to be addressed.
:::

:::{note}
Before reporting an issue, please see our [support page](../support/overview.md) and [troubleshooting installation issues](../support/troubleshooting/installation)
:::

## ScyllaDB Operator components

ScyllaDB Operator consists of multiple components that need to be installed in your cluster.
This is by no means a complete list of all resources, rather it aims to show the major components in one place.


```{figure} deploy.svg
:class: sd-m-auto
:name: deploy-overview
```

:::{note}
Depending on [which storage provisioner you choose](../architecture/storage/overview.md), the `local-csi-driver` may be replaced or complemented by a different component.
:::

### ScyllaDB Operator

ScyllaDB Operator contains the Kubernetes API extensions and corresponding controllers and admission hooks that run inside `scylla-operator` namespace.

You can learn more about the APIs in [resources section](../resources/overview.md) and the [generated API reference](../reference/api/index.rst). 

### ScyllaDB Manager

ScyllaDB Manager is a global deployment that is responsible for operating all [ScyllaClusters](../reference/api/groups/scylla.scylladb.com/scyllaclusters.rst) and runs inside `scylla-manager` namespace.
ScyllaDB Operator syncs the [ScyllaCluster](../reference/api/groups/scylla.scylladb.com/scyllaclusters.rst) metadata, [backup](#api-scylla.scylladb.com-scyllaclusters-v1-.spec.backups[]) and [repair](#api-scylla.scylladb.com-scyllaclusters-v1-.spec.repairs[]) tasks into the manager (and vice versa). This way it allows avoiding direct access to the shared instance by users. Unfortunately, at this point, other task like restoring from a backup require executing into the shared ScyllaDB Manager deployment which effectively needs administrator privileges. 

ScyllaDB Manager uses a small ScyllaCluster instance internally and thus depends on the ScyllaDB Operator deployment and the CRD it provides.

### NodeConfig

[NodeConfig](../resources/nodeconfigs.md) is a cluster-scoped custom resource provided by ScyllaDB Operator that helps you set local disks on Kubernetes nodes, create and mount a file system, configure performance tuning and more. 

### ScyllaOperatorConfig

[ScyllaOperatorConfig](../resources/scyllaoperatorconfigs.md) is a cluster-coped custom resource provided by ScyllaDB Operator to help you configure ScyllaDB Operator. It helps you configure auxiliary images, see which ones are in use and more. 

### Local CSI driver

ScyllaDB provides you with a custom [Local CSI driver](../architecture/storage/local-csi-driver.md) that lets you dynamically provision PersistentVolumes, share the disk space but still track the capacity and use quotas.

## Installation

You can install ScyllaDB Operator on Red Hat OpenShift using OLM (Operator Lifecycle Manager), or on any Kubernetes cluster using GitOps manifests or Helm charts.

### Red Hat OpenShift

ScyllaDB Operator is available as a Red Hat Certified Operator in the embedded OperatorHub. 
You can install it using OLM (Operator Lifecycle Manager), which handles installation, upgrades, and management of operators and their dependencies, including CRDs.

The following components need to be deployed and maintained using other deployment methods:
- [Local CSI Driver](#local-csi-driver)
- [ScyllaDB Manager](#scylladb-manager)

We provide [GitOps](#gitops) manifests and installation instructions for these components.

For details, please see the [dedicated section describing the deployment on Red Hat OpenShift](./openshift.md).

### Generic Kubernetes

For generic Kubernetes clusters, we provide [GitOps/manifests](#gitops) and [Helm charts](#helm).

Given we provide only a subset of helm charts for the main resources and because **Helm can't update CRDs** - you still have to resort to using the manifests or GitOps anyway. 
For a consistent experience we'd recommend using the [GitOps flow](#gitops) which will also give you a better idea about what you actually deploy and maintain.

:::{caution}
Do not use rolling tags (like `latest`, `1.14`) with our manifests in production.
The manifests and images for a particular release are tightly coupled and any update requires updating both of them, while the rolling tags may surprisingly update only the images.
You should always use the full version (e.g. `1.14.0`) and/or using the corresponding *sha* reference.
:::

:::{note}
To avoid races, when you create a CRD, you need to wait for it to be propagated to other instances of the kubernetes-apiserver, before you can reliably create the corresponding CRs.
:::

:::{note}
When you create [ValidatingWebhookConfiguration](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/#webhook-configuration) or [MutatingWebhookConfiguration](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/#webhook-configuration), you have to wait for the corresponding webhook deployments to be available, or the kubernetes-apiserver will fail all requests for resources affected by these webhook configurations.
Also note that some platforms have non-conformant networking setups by default that prevents the kube-apiserver from talking to the webhooks - [see our troubleshooting guide for more info](../support/troubleshooting/installation.md#webhooks).
::: 

#### GitOps

We provide a set of Kubernetes manifest that contain all necessary objects to apply to your Kubernetes cluster.
Depending on your preference applying them may range from using `git` with a declarative CD to `git` and `kubectl`.
To keep the instructions clear for everyone we'll demonstrate applying the manifests using `kubectl`, that everyone is familiar with and is able to translate it to the GitOps platform of his choosing.

For details, please see the [dedicated section describing the deployment using GitOps (kubectl)](./gitops.md). 

#### Helm

:::{include} ../.internal/helm-crd-warning.md
:::

For details, please see the [dedicated section describing the deployment using Helm](./helm.md). 

## Upgrades

Please see the [dedicated section describing the upgrade process](./../management/upgrading/upgrade.md).
