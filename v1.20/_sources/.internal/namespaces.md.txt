:::{note}
You can run your ScyllaDB cluster in the Kubernetes **namespace of your choice** (you can change the namespace used in this guide to your preference). It is a best practice (but not strictly required) to run your ScyllaDB cluster in a namespace separate from other applications.

However, **the ScyllaDB Operator and ScyllaDB Manager must run in namespaces `scylla-operator` and `scylla-manager`, respectively**. It is [not currently possible](https://github.com/scylladb/scylla-operator/issues/2563) to use different namespaces for these two components.
:::
