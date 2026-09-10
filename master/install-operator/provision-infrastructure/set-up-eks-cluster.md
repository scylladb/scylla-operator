# Set up an EKS cluster for ScyllaDB

This guide provisions an Amazon Elastic Kubernetes Service (EKS) cluster suitable for running ScyllaDB.
At the end, you will have:

- An EKS cluster with an infrastructure node group.
- A dedicated ScyllaDB node group with local NVMe SSDs, static CPU manager policy, and ScyllaDB labels and taints.

## Prerequisites

- An AWS account with permissions to create EKS clusters, EC2 instances, and VPC resources.
- The [`eksctl` CLI](https://eksctl.io/installation/) installed.
- [`kubectl`](https://kubernetes.io/docs/tasks/tools/#kubectl) installed.
- AWS credentials configured (`aws configure` or environment variables).
- Sufficient quota for `i` instances in your target region, as advised in the  [ScyllaDB cloud instance recommendations for AWS](https://docs.scylladb.com/manual/stable/getting-started/cloud-instance-recommendations.html#amazon-web-services-aws) and the [minimum system requirements](https://docs.scylladb.com/manual/stable/getting-started/system-requirements.html).

## Set environment variables

The rest of the guide refers to the variables defined here.

Set your AWS region — this has no default and must be provided:

```console
export AWS_REGION="<your-region>"  # e.g. eu-central-1
```

The remaining variables have sensible defaults and can be copied as-is.
Override any value before running if needed:

```bash
# Cluster name.
export EKS_CLUSTER_NAME="${EKS_CLUSTER_NAME:-scylladb-demo}"

# Availability zones — one per ScyllaDB rack.
export EKS_AZ_1="${EKS_AZ_1:-${AWS_REGION}a}"
export EKS_AZ_2="${EKS_AZ_2:-${AWS_REGION}b}"
export EKS_AZ_3="${EKS_AZ_3:-${AWS_REGION}c}"

# Dedicated ScyllaDB node group. i7i.2xlarge provides 8 vCPU, 64 GiB RAM, and 1x1875 GB NVMe.
# See https://docs.scylladb.com/manual/stable/getting-started/cloud-instance-recommendations.html#amazon-web-services-aws
export SCYLLA_INSTANCE_TYPE="${SCYLLA_INSTANCE_TYPE:-i7i.2xlarge}"
export SCYLLA_NODE_COUNT="${SCYLLA_NODE_COUNT:-3}"

# Infrastructure node group.
export INFRA_INSTANCE_TYPE="${INFRA_INSTANCE_TYPE:-m7i.large}"
export INFRA_NODE_COUNT="${INFRA_NODE_COUNT:-1}"
```

## Create a temporary directory

Create a temporary directory for configuration files used in this guide:

```bash
TMPDIR="$(mktemp -d)"
trap 'rm -rf "${TMPDIR}"' EXIT
```

## Create the eksctl cluster configuration

`eksctl` uses a declarative `ClusterConfig` to define the cluster and its node groups.
Generate the configuration file:

```bash
cat > "${TMPDIR}/clusterconfig.eksctl.yaml" <<EOF
apiVersion: eksctl.io/v1alpha5
kind: ClusterConfig
metadata:
  name: ${EKS_CLUSTER_NAME}
  region: ${AWS_REGION}
availabilityZones:
- ${EKS_AZ_1}
- ${EKS_AZ_2}
- ${EKS_AZ_3}
nodeGroups:
- name: scylla-pool
  instanceType: ${SCYLLA_INSTANCE_TYPE}
  desiredCapacity: ${SCYLLA_NODE_COUNT}
  amiFamily: AmazonLinux2023
  labels:
    scylla.scylladb.com/node-type: scylla
  taints:
    scylla-operator.scylladb.com/dedicated: "scyllaclusters:NoSchedule"
  kubeletExtraConfig:
    cpuManagerPolicy: static
  availabilityZones:
  - ${EKS_AZ_1}
  - ${EKS_AZ_2}
  - ${EKS_AZ_3}
- name: infra-pool
  instanceType: ${INFRA_INSTANCE_TYPE}
  desiredCapacity: ${INFRA_NODE_COUNT}
  amiFamily: AmazonLinux2023
  labels:
    scylla.scylladb.com/node-type: infra
EOF
```

The ScyllaDB node group uses storage-optimized `i` instances, as advised in the [ScyllaDB cloud instance recommendations for AWS](https://docs.scylladb.com/manual/stable/getting-started/cloud-instance-recommendations.html#amazon-web-services-aws)

- `cpuManagerPolicy: static` for CPU pinning.
- ScyllaDB labels and taints so only ScyllaDB pods are scheduled on these nodes.
- Nodes spread across 3 availability zones for fault tolerance.

## Create the EKS cluster

```bash
eksctl create cluster -f="${TMPDIR}/clusterconfig.eksctl.yaml"
```

#### NOTE
Cluster creation typically takes 15–20 minutes.
`eksctl` automatically configures `kubectl` to use the new cluster.

Verify connectivity and node readiness:

```console
kubectl get nodes -L scylla.scylladb.com/node-type
```

Example expected output:

```console
NAME                                              STATUS   ROLES    AGE   VERSION   NODE-TYPE
ip-192-168-xx-xx.eu-central-1.compute.internal    Ready    <none>   10m   v1.32.1   scylla
ip-192-168-xx-xx.eu-central-1.compute.internal    Ready    <none>   10m   v1.32.1   scylla
ip-192-168-xx-xx.eu-central-1.compute.internal    Ready    <none>   10m   v1.32.1   scylla
ip-192-168-xx-xx.eu-central-1.compute.internal    Ready    <none>   10m   v1.32.1   infra
```

## Clean up

Delete the EKS cluster and all associated resources:

```console
eksctl delete cluster --name="${EKS_CLUSTER_NAME}" --region="${AWS_REGION}" --force --disable-nodegroup-eviction
```

## Next steps

- Follow the [Reference deployment: EKS](https://operator.docs.scylladb.com/master/deploy-scylladb/reference-deployments/reference-deployment-eks.md) for a complete ScyllaDB deployment on this cluster.
