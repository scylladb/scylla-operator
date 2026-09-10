Beginning with GKE version `1.32.1-gke.1002000`, the Ubuntu image used by GKE clusters no longer provides the `xfsprogs` package by default.
This package is required to format the local NVMe disks used by ScyllaDB. Please refer to the [xfsprogs section](../installation/kubernetes-prerequisites.md#xfsprogs) of the Kubernetes prerequisites page for more details.
