#!/usr/bin/env bash
#
# Set up an EKS cluster suitable for running ScyllaDB.
#
# This script creates:
# - An EKS cluster with an infrastructure node group
# - A dedicated ScyllaDB node group with local NVMe SSDs and static CPU policy
#
# Prerequisites:
# - eksctl CLI installed and configured
# - kubectl installed
# - AWS credentials configured
#
# Usage:
#   export AWS_REGION='eu-central-1'
#   ./setup-eks-cluster.sh

set -euo pipefail

# [START tmpdir]
TMPDIR="$(mktemp -d)"
trap 'rm -rf "${TMPDIR}"' EXIT
# [END tmpdir]

# Target region.
: "${AWS_REGION:?'Set AWS_REGION (e.g. eu-central-1)'}"

# [START env_defaults]
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
# [END env_defaults]

# [START create_cluster_config]
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
# [END create_cluster_config]

# [START create_cluster]
eksctl create cluster -f="${TMPDIR}/clusterconfig.eksctl.yaml"
# [END create_cluster]

# [START cleanup]
# eksctl delete cluster --name="${EKS_CLUSTER_NAME}" --region="${AWS_REGION}" --force --disable-nodegroup-eviction
# [END cleanup]
