# Overview

## Kubernetes

{{productName}} is a set of controllers and Kubernetes API extensions.
Therefore, we assume you either have an existing, conformant Kubernetes cluster and/or are already familiar with how a Kubernetes cluster is deployed and operated.

{{productName}} controllers and API extensions may have dependencies on some of the newer Kubernetes features and APIs that need to be available.
More over, {{productName}} implements additional features like performance tuning, some of which are platform/OS specific.
While we do our best to implement these routines as generically as possible, sometimes there isn't any low level API to base them on and they may work only on a subset of platforms.

:::{caution}
We *strongly* recommend using a [supported Kubernetes platform](../support/releases.md#supported-kubernetes-platforms).
Issues on unsupported platforms are unlikely to be addressed.
:::

:::{note}
Before reporting an issue, please see our [support page](../support/overview.md) and [troubleshooting installation issues](../support/troubleshooting/installation)
:::

## {{productName}} components

{{productName}} consists of multiple components that need to be installed in your cluster.
This is by no means a complete list of all resources, rather it aims to show the major components in one place.


```{figure} deploy.svg
:class: sd-m-auto
:name: deploy-overview
```

:::{note}
Depending on [which storage provisioner you choose](../architecture/storage/overview.md), the `local-csi-driver` may be replaced or complemented by a different component.
:::

### {{productName}}

{{productName}} contains the Kubernetes API extensions and corresponding controllers and admission hooks that run inside `scylla-operator` namespace.

You can learn more about the APIs in [resources section](../resources/overview.md) and the [generated API reference](../api-reference/index.rst). 

### ScyllaDB Manager

ScyllaDB Manager is a global deployment that is responsible for operating all [ScyllaClusters](../api-reference/groups/scylla.scylladb.com/scyllaclusters.rst) and runs inside `scylla-manager` namespace.
{{productName}} syncs the [ScyllaCluster](../api-reference/groups/scylla.scylladb.com/scyllaclusters.rst) metadata, [backup](#api-scylla.scylladb.com-scyllaclusters-v1-.spec.backups[]) and [repair](#api-scylla.scylladb.com-scyllaclusters-v1-.spec.repairs[]) tasks into the manager (and vice versa). This way it allows avoiding direct access to the shared instance by users. Unfortunately, at this point, other task like restoring from a backup require executing into the shared ScyllaDB Manager deployment which effectively needs administrator privileges. 

ScyllaDB Manager uses a small ScyllaCluster instance internally and thus depends on the {{productName}} deployment and the CRD it provides.

### NodeConfig

[NodeConfig](../resources/nodeconfigs.md) is a cluster-scoped custom resource provided by {{productName}} that helps you set local disks on Kubernetes nodes, create and mount a file system, configure performance tuning and more. 

### ScyllaOperatorConfig

[ScyllaOperatorConfig](../resources/scyllaoperatorconfigs.md) is a cluster-coped custom resource provided by {{productName}} to help you configure {{productName}}. It helps you configure auxiliary images, see which ones are in use and more. 

### Local CSI driver

ScyllaDB provides you with a custom [Local CSI driver](../architecture/storage/local-csi-driver.md) that lets you dynamically provision PersistentVolumes, share the disk space but still track the capacity and use quotas.

## Notes

:::{note}
Before reporting and issue, please see our [support page](../support/overview.md) and [troubleshooting installation issues](../support/troubleshooting/installation.md#troubleshooting-installation-issues)
:::

## Installation modes

Depending on your preference, there is more than one way to install {{productName}} and there may be more to come / or provided by other parties or supply chains.

At this point, we provide 2 ways to install the operator - [GitOps/manifests](#gitops) and [Helm charts](#helm). Given we provide only a subset of helm charts for the main resources and because **Helm can't update CRDs** - you still have to resort to using the manifests or GitOps anyway. For a consistent experience we'd recommend using the [GitOps flow](#gitops) which will also give you a better idea about what you actually deploy and maintain.

:::{caution}
Do not use rolling tags (like `latest`, `1.14`) with our manifests in production.
The manifests and images for a particular release are tightly coupled and any update requires updating both of them, while the rolling tags may surprisingly update only the images.
You should always use the full version (e.g. `1.14.0`) and/or using the corresponding *sha* reference.
:::

:::{note}
To avoid races, when you create a CRD, you need to wait for it to be propagated to other instances of the kubernetes-apiserver, before you can reliably create the corresponding CRs.
:::

:::{note}
When you create [ValidatingWebhookConfiguration](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/#webhook-configuration) or [MutatingWebhookConfiguration](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/#webhook-configuration), you have to wait for the corresponding webhook deployments to be available, or the kubernetes-apiserver will fail all requests for resources affected by these webhhok configurations.
Also note that some platforms have non-conformant networking setups by default that prevents the kube-apiserver from talking to the webhooks - [see our troubleshooting guide for more info](../support/troubleshooting/installation.md#webhooks).
::: 

### GitOps

We provide a set of Kubernetes manifest that contain all necessary objects to apply to your Kubernetes cluster.
Depending on your preference applying them may range from using `git` with a declarative CD to `git` and `kubectl`.
To keep the instructions clear for everyone we'll demonstrate applying the manifests using `kubectl`, that everyone is familiar with and is able to translate it to the GitOps platform of his choosing.

For details, please see the [dedicated section describing the deployment using GitOps (kubectl)](./gitops.md). 

### Helm

:::{include} ../.internal/helm-crd-warning.md
:::

For details, please see the [dedicated section describing the deployment using Helm](./helm.md). 

## Upgrades

Please see the [dedicated section describing the upgrade process](./upgrade.md).
