# Set up an EKS cluster for ScyllaDB

This guide provisions an Amazon Elastic Kubernetes Service (EKS) cluster suitable for running ScyllaDB.
At the end, you will have:

- An EKS cluster with an infrastructure node group.
- A dedicated ScyllaDB node group with local NVMe SSDs, static CPU manager policy, and ScyllaDB labels and taints.

:::{tip}
All the steps in this guide are also available as a single executable script:
{{ '[`examples/eks/setup-eks-cluster.sh`](https://github.com/{}/blob/{}/examples/eks/setup-eks-cluster.sh)'.format(repository, revision) }}.
Set `AWS_REGION`, then run the script to provision everything in one go.
:::

## Prerequisites

- An AWS account with permissions to create EKS clusters, EC2 instances, and VPC resources.
- The [`eksctl` CLI](https://eksctl.io/installation/) installed.
- [`kubectl`](https://kubernetes.io/docs/tasks/tools/#kubectl) installed.
- AWS credentials configured (`aws configure` or environment variables).
- Sufficient quota for `i` instances in your target region, as advised in the  [ScyllaDB cloud instance recommendations for AWS](https://docs.scylladb.com/manual/stable/getting-started/cloud-instance-recommendations.html#amazon-web-services-aws) and the [minimum system requirements](https://docs.scylladb.com/manual/stable/getting-started/system-requirements.html).

## Set environment variables

The rest of the guide refers to the variables defined here.

Set your AWS region — this has no default and must be provided:

:::{code-block} console
export AWS_REGION="<your-region>"  # e.g. eu-central-1
:::

The remaining variables have sensible defaults and can be copied as-is.
Override any value before running if needed:

:::{literalinclude} ../../../../examples/eks/setup-eks-cluster.sh
:language: bash
:start-after: "# [START env_defaults]"
:end-before: "# [END env_defaults]"
:::

## Create a temporary directory

Create a temporary directory for configuration files used in this guide:

:::{literalinclude} ../../../../examples/eks/setup-eks-cluster.sh
:language: bash
:start-after: "# [START tmpdir]"
:end-before: "# [END tmpdir]"
:::

## Create the eksctl cluster configuration

`eksctl` uses a declarative `ClusterConfig` to define the cluster and its node groups.
Generate the configuration file:

:::{literalinclude} ../../../../examples/eks/setup-eks-cluster.sh
:language: bash
:start-after: "# [START create_cluster_config]"
:end-before: "# [END create_cluster_config]"
:::

The ScyllaDB node group uses storage-optimized `i` instances, as advised in the [ScyllaDB cloud instance recommendations for AWS](https://docs.scylladb.com/manual/stable/getting-started/cloud-instance-recommendations.html#amazon-web-services-aws)

- `cpuManagerPolicy: static` for CPU pinning.
- ScyllaDB labels and taints so only ScyllaDB pods are scheduled on these nodes.
- Nodes spread across 3 availability zones for fault tolerance.

## Create the EKS cluster

:::{literalinclude} ../../../../examples/eks/setup-eks-cluster.sh
:language: bash
:start-after: "# [START create_cluster]"
:end-before: "# [END create_cluster]"
:::

:::{note}
Cluster creation typically takes 15–20 minutes.
`eksctl` automatically configures `kubectl` to use the new cluster.
:::

Verify connectivity and node readiness:

:::{code-block} console
kubectl get nodes -L scylla.scylladb.com/node-type
:::

Example expected output:

:::{code-block} console
NAME                                              STATUS   ROLES    AGE   VERSION   NODE-TYPE
ip-192-168-xx-xx.eu-central-1.compute.internal    Ready    <none>   10m   v1.32.1   scylla
ip-192-168-xx-xx.eu-central-1.compute.internal    Ready    <none>   10m   v1.32.1   scylla
ip-192-168-xx-xx.eu-central-1.compute.internal    Ready    <none>   10m   v1.32.1   scylla
ip-192-168-xx-xx.eu-central-1.compute.internal    Ready    <none>   10m   v1.32.1   infra
:::

## Clean up

Delete the EKS cluster and all associated resources:

:::{code-block} console
eksctl delete cluster --name="${EKS_CLUSTER_NAME}" --region="${AWS_REGION}" --force --disable-nodegroup-eviction
:::

## Next steps

- Follow the [Reference deployment: EKS](../../deploy-scylladb/reference-deployments/reference-deployment-eks.md) for a complete ScyllaDB deployment on this cluster.
