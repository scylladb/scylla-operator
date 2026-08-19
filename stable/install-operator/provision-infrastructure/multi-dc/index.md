# Multi-DC

A multi-datacenter ScyllaDB cluster runs each datacenter in a separate Kubernetes cluster.
The Kubernetes clusters have to be able to reach each other over Pod IPs, which requires additional networking setup beyond a single-cluster deployment.

Follow the guide for your platform:

- [Set up multiple GKE clusters](https://operator.docs.scylladb.com/stable/install-operator/provision-infrastructure/multi-dc/set-up-multi-dc-gke-clusters.md) — GKE clusters in a shared VPC, with inter-Kubernetes networking.
- [Set up multiple EKS clusters](https://operator.docs.scylladb.com/stable/install-operator/provision-infrastructure/multi-dc/set-up-multi-dc-eks-clusters.md) — EKS clusters in peered VPCs, with inter-Kubernetes networking.

Once the platform is ready, follow [Deploy a multi-datacenter ScyllaDB cluster](https://operator.docs.scylladb.com/stable/deploy-scylladb/deploy-multi-datacenter-cluster.md).
