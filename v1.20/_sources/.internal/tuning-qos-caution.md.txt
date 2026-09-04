:::{caution}
Only Pods with [`Guaranteed` QoS class](https://kubernetes.io/docs/concepts/workloads/pods/pod-qos/#guaranteed) are eligible to be tuned, otherwise they would not have pinned CPUs.

Always verify that your [ScyllaCluster](/resources/scyllaclusters/basics.md) resource specifications meat [all the criteria](https://kubernetes.io/docs/concepts/workloads/pods/pod-qos/#criteria).

Don't forget you have to specify limits for both [resources](api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].resources)(ScyllaDB) and [agentResources](api-scylla.scylladb.com-scyllaclusters-v1-.spec.datacenter.racks[].agentResources)(ScyllaDB Manager Agent) that run in the same Pod.
:::
