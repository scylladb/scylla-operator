# Automatic cleanup and replacement in case when k8s node is lost

In case when your k8s cluster loses one of the nodes due to incident or explicit removal, Scylla Pods may become unschedulable due to PVC node affinity.

When `automaticOrphanedNodeCleanup` flag is enabled in your ScyllaCluster, Scylla Operator will perform automatic
node replacement of a Pod which lost his bound resources.
