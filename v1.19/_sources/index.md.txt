---
sd_hide_title: true
full-width:
landing:
hide-secondary-sidebar:
hide-version-warning:
hide-pre-content:
hide-post-content:
---

```{title} Scylla Operator Documentation
```

:::{toctree}
:hidden:
:maxdepth: 1

architecture/index
installation/index
management/index
resources/index
quickstarts/index
support/index
api-reference/index
:::

```{hero-box}
:title: Scylla Operator
:image: /_static/mascots/logo.svg

{{productName}} project helps users to run ScyllaDB on Kubernetes. 
It extends the Kubernetes APIs using [CustomResourceDefinitions(CRDs)](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/) and runs controllers that reconcile the desired state declared using these APIs.
```

```{raw} html
<div class="topics-grid">
```

```{note}
{{productName}} works with both ScyllaDB Open Source and ScyllaDB Enterprise. By default, our examples use ScyllaDB Open Source. To make the switch to ScyllaDB Enterprise, you only have to [change the ScyllaCluster image repository](#scyllacluster-enterprise) and [adjust the ScyllaDB utils image using ScyllaOperatorConfig](resources/scyllaoperatorconfigs.md#tuning-with-scylladb-enterprise).
```

```{raw} html
</div>
```

```{raw} html
<div class="topics-grid topics-grid--products">
    <h2 class="topics-grid__title">Topics</h2>
    <p class="topics-grid__text">Here is a subset of items to start with. You can also navigate through the documentation using the menu.</p>
```

````{grid}
:type: default

```{topic-box}
:title: Architecture
:icon: icon-apartment
:link: architecture/overview
:anchor: Learn more »
:class: large-4

Learn about the components of Scylla Operator and how they fit together.
```

```{topic-box}
:title: Installation
:icon: icon-download
:link: installation/overview
:anchor: Learn more »
:class: large-4

Configure your Kubernetes platform, install prerequisites and all components of {{productName}}.
```

```{topic-box}
:title: Management
:icon: icon-operations
:link: management/overview
:anchor: Learn more »
:class: large-4

Manage your ScyllaDB clusters using {{productName}}. 
```

```{topic-box}
:title: Working with Resources
:icon: icon-database
:link: resources/overview
:anchor: Learn more »
:class: large-4

Learn about the APIs that {{productName}} provides.
```

```{topic-box}
:title: Quickstarts
:icon: icon-rocket
:link: quickstarts/index
:anchor: Learn more »
:class: large-4

Get it running right now. Simple GKE and EKS setups.
```

```{topic-box}
:title: Performance Tuning
:icon: icon-tune
:link: architecture/tuning
:anchor: Learn more »
:class: large-4

Tune your infra and ScyllaDB cluster for the best performance and low latency.
```

```{topic-box}
:title: Support
:icon: icon-support
:link: support/overview
:anchor: Learn more »
:class: large-4

FAQs, support matrix, must-gather and more.
```

```{topic-box}
:title: API Reference
:icon: icon-docs-commands
:link: api-reference/index
:anchor: Learn more »
:class: large-4

Visit the automatically generated API reference for all our APIs.
```
````

```{raw} html
</div>
```
