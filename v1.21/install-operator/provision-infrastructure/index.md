# Provision infrastructure

Before installing ScyllaDB Operator, ensure your environment meets the following requirements.

## Kubernetes cluster

ScyllaDB Operator requires a [supported Kubernetes environment](https://operator.docs.scylladb.com/v1.21/reference/releases.md).
Issues on unsupported environments are unlikely to be addressed.

If you do not have a cluster yet, follow one of the platform-specific guides:

- [Set up a GKE cluster](https://operator.docs.scylladb.com/v1.21/install-operator/provision-infrastructure/set-up-gke-cluster.md) — Google Kubernetes Engine.
- [Set up an EKS cluster](https://operator.docs.scylladb.com/v1.21/install-operator/provision-infrastructure/set-up-eks-cluster.md) — Amazon Elastic Kubernetes Service.
- [Set up an OKE cluster](https://operator.docs.scylladb.com/v1.21/install-operator/provision-infrastructure/set-up-oke-cluster.md) — Oracle Container Engine for Kubernetes.
- [Set up an OpenShift cluster](https://operator.docs.scylladb.com/v1.21/install-operator/provision-infrastructure/set-up-openshift-cluster.md) — Red Hat OpenShift.

For a multi-datacenter ScyllaDB cluster, you need several interconnected Kubernetes clusters — see [Multi-DC](https://operator.docs.scylladb.com/v1.21/install-operator/provision-infrastructure/multi-dc/index.md).
