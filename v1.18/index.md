# Scylla Operator Documentation

## Scylla Operator

![image](logo.svg)

Scylla Operator project helps users to run ScyllaDB on Kubernetes.
It extends the Kubernetes APIs using [CustomResourceDefinitions(CRDs)](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/) and runs controllers that reconcile the desired state declared  using these APIs.

Scylla Operator works with both ScyllaDB Open Source and ScyllaDB Enterprise. By default, our examples use ScyllaDB Open Source. To make the switch to ScyllaDB Enterprise, you only have to [change the ScyllaCluster image repository](https://operator.docs.scylladb.com/v1.18/resources/scyllaclusters/basics.md#scyllacluster-enterprise) and [adjust the ScyllaDB utils image using ScyllaOperatorConfig](https://operator.docs.scylladb.com/v1.18/resources/scyllaoperatorconfigs.md#tuning-with-scylladb-enterprise).

Here is a subset of items to start with.
You can also navigate through the documentation using the menu.

Learn about the components of Scylla Operator and how they fit together.

[Learn more »](https://operator.docs.scylladb.com/v1.18/architecture/overview.md)

Configure your Kubernetes platform, install prerequisites and all components of Scylla Operator.

[Learn more »](https://operator.docs.scylladb.com/v1.18/installation/overview.md)

Learn about the APIs that Scylla Operator provides. ScyllaClusters, ScyllaDBMonitorings and more.

[Learn more »](https://operator.docs.scylladb.com/v1.18/resources/overview.md)

Get it running right now. Simple GKE and EKS setups.

[Learn more »](https://operator.docs.scylladb.com/v1.18/quickstarts/index.md)

Tune your infra and ScyllaDB cluster for the best performance and low latency.

[Learn more »](https://operator.docs.scylladb.com/v1.18/architecture/tuning.md)

FAQs, support matrix, must-gather and more.

[Learn more »](https://operator.docs.scylladb.com/v1.18/support/overview.md)

Visit the automatically generated API reference for all our APIs.

[Learn more »](https://operator.docs.scylladb.com/v1.18/api-reference/index.md)
