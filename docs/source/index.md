---
sd_hide_title: true
---

# Scylla Operator Documentation

:::{toctree}
:hidden:
:maxdepth: 1

architecture/index
installation/index
resources/index
quickstarts/index
support/index
api-reference/index
:::

## Scylla Operator

::::{grid} 1 1 1 2
:reverse:

:::{grid-item}
:columns: 12 12 12 4

```{image} logo.png
:width: 150pt
:class: sd-m-auto
:name: landing-page-logo
```

:::

:::{grid-item}
:child-align: justify
:columns: 12 12 12 8
:class: sd-d-flex-column

{{productName}} project helps users to run ScyllaDB on Kubernetes. 
It extends the Kubernetes APIs using [CustomResourceDefinitions(CRDs)](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/) and runs controllers that reconcile the desired state declared  using these APIs.

{{productName}} works with both ScyllaDB Open Source and ScyllaDB Enterprise. By default, our examples use ScyllaDB Open Source. To make the switch to ScyllaDB Enterprise, you only have to [change the ScyllaCluster image repository](#scyllacluster-enterprise) and [adjust the ScyllaDB utils image using ScyllaOperatorConfig](resources/scyllaoperatorconfigs.md#tuning-with-scylladb-enterprise).

Here is a subset of items to start with.
You can also navigate through the documentation using the menu.

:::

::::


::::{grid} 1 1 2 3
:gutter: 4

:::{grid-item-card} {material-regular}`architecture;2em` Architecture

Learn about the components of Scylla Operator and how they fit together.
+++
[Learn more »](architecture/overview)
:::

:::{grid-item-card} {material-regular}`electric_bolt;2em` Installation

Configure your Kubernetes platform, install prerequisites and all components of {{productName}}.
+++
[Learn more »](installation/overview)
:::

:::{grid-item-card} {material-regular}`storage;2em` Working with Resources

Learn about the APIs that {{productName}} provides. ScyllaClusters, ScyllaDBMonitorings and more.  
+++
[Learn more »](resources/overview)
:::

:::{grid-item-card} {material-regular}`explore;2em` Quickstarts

Get it running right now. Simple GKE and EKS setups.

+++
[Learn more »](quickstarts/index)
:::

:::{grid-item-card} {material-regular}`fitness_center;2em` Performance Tuning

Tune your infra and ScyllaDB cluster for the best performance and low latency. 
+++
[Learn more »](architecture/tuning)
:::

:::{grid-item-card} {material-regular}`build;2em` Support

FAQs, support matrix, must-gather and more. 
+++
[Learn more »](support/overview)
:::

:::{grid-item-card} {material-regular}`menu_book;2em` API Rererence

Visit the automatically generated API reference for all our APIs.
+++
[Learn more »](api-reference/index)
:::

::::
