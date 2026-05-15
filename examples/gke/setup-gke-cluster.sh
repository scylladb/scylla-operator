#!/usr/bin/env bash
#
# Set up a GKE cluster suitable for running ScyllaDB.
#
# This script creates:
# - A regional GKE cluster with a system node pool
# - A dedicated ScyllaDB node pool with local NVMe SSDs and static CPU policy,
#   spread across 3 zones for fault tolerance
#
# Prerequisites:
# - gcloud CLI installed and configured
# - kubectl installed
#
# Usage:
#   export GCP_PROJECT='my-project'
#   export GCP_REGION='us-central1'
#   ./setup-gke-cluster.sh

set -euo pipefail

# [START tmpdir]
TMPDIR="$(mktemp -d)"
trap 'rm -rf "${TMPDIR}"' EXIT
# [END tmpdir]

# Target project and region.
: "${GCP_PROJECT:?'Set GCP_PROJECT'}"
: "${GCP_REGION:?'Set GCP_REGION (e.g. us-central1)'}"

# [START env_defaults]
# Cluster name.
export GKE_CLUSTER_NAME="${GKE_CLUSTER_NAME:-scylladb-demo}"

# Availability zones — one per ScyllaDB rack.
export GKE_ZONE_1="${GKE_ZONE_1:-${GCP_REGION}-a}"
export GKE_ZONE_2="${GKE_ZONE_2:-${GCP_REGION}-b}"
export GKE_ZONE_3="${GKE_ZONE_3:-${GCP_REGION}-c}"

# System node pool machine type.
export SYSTEM_MACHINE_TYPE="${SYSTEM_MACHINE_TYPE:-n2-standard-8}"
export SYSTEM_NODE_COUNT="${SYSTEM_NODE_COUNT:-2}"

# Dedicated ScyllaDB node pool. n2-highmem-16 provides 16 vCPU and 128 GiB RAM.
# See https://docs.scylladb.com/manual/stable/getting-started/cloud-instance-recommendations.html#google-compute-engine-gce
export SCYLLA_MACHINE_TYPE="${SCYLLA_MACHINE_TYPE:-n2-highmem-16}"
# Node count is per zone. With 3 zones this gives 3 nodes total.
export SCYLLA_NODE_COUNT="${SCYLLA_NODE_COUNT:-1}"
export SCYLLA_LOCAL_SSD_COUNT="${SCYLLA_LOCAL_SSD_COUNT:-4}"
# [END env_defaults]

# [START systemconfig]
cat > "${TMPDIR}/systemconfig.yaml" <<EOF
kubeletConfig:
  cpuManagerPolicy: static
EOF
# [END systemconfig]

# [START create_cluster]
gcloud container clusters create "${GKE_CLUSTER_NAME}" \
  --project="${GCP_PROJECT}" \
  --region="${GCP_REGION}" \
  --node-locations="${GKE_ZONE_1}" \
  --cluster-version="latest" \
  --machine-type="${SYSTEM_MACHINE_TYPE}" \
  --num-nodes="${SYSTEM_NODE_COUNT}" \
  --disk-type='pd-ssd' --disk-size='20' \
  --image-type='UBUNTU_CONTAINERD' \
  --no-enable-autoupgrade \
  --no-enable-autorepair
# [END create_cluster]

# [START create_scylla_pool]
gcloud container node-pools create 'scyllaclusters' \
  --project="${GCP_PROJECT}" \
  --region="${GCP_REGION}" \
  --cluster="${GKE_CLUSTER_NAME}" \
  --node-locations="${GKE_ZONE_1},${GKE_ZONE_2},${GKE_ZONE_3}" \
  --node-version="latest" \
  --machine-type="${SCYLLA_MACHINE_TYPE}" \
  --num-nodes="${SCYLLA_NODE_COUNT}" \
  --disk-type='pd-ssd' --disk-size='20' \
  --local-nvme-ssd-block="count=${SCYLLA_LOCAL_SSD_COUNT}" \
  --image-type='UBUNTU_CONTAINERD' \
  --system-config-from-file="${TMPDIR}/systemconfig.yaml" \
  --no-enable-autoupgrade \
  --no-enable-autorepair \
  --node-labels='scylla.scylladb.com/node-type=scylla' \
  --node-taints='scylla-operator.scylladb.com/dedicated=scyllaclusters:NoSchedule'
# [END create_scylla_pool]

# [START get_credentials]
gcloud container clusters get-credentials "${GKE_CLUSTER_NAME}" \
  --project="${GCP_PROJECT}" \
  --region="${GCP_REGION}"
# [END get_credentials]

# [START cleanup]
# gcloud container clusters delete "${GKE_CLUSTER_NAME}" \
#   --project="${GCP_PROJECT}" \
#   --region="${GCP_REGION}" \
#   --quiet
# [END cleanup]
