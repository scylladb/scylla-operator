```{caution}
ScyllaDBCluster is designated for usage across multiple Kubernetes clusters. As ClusterIP Services are conventionally only routable within a single Kubernetes cluster,
make sure your CNI allows for external ClusterIP connectivity. Otherwise, switch to a different type of exposure.
```