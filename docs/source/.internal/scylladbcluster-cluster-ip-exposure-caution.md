```{caution}
ScyllaDBCluster is most likely used across multiple Kubernetes clusters, because ClusterIP Service's are usually routable within single Kubernetes cluster,
make sure your CNI allows for external ClusterIP connectivity. If not, switch to other type of exposure.
```