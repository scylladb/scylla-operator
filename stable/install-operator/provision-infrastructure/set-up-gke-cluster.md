# Set up a GKE cluster for ScyllaDB

This guide provisions a Google Kubernetes Engine (GKE) cluster suitable for running ScyllaDB.
At the end, you will have:

- A regional GKE cluster with a system node pool for infrastructure workloads.
- A dedicated node pool with local NVMe SSDs, static CPU manager policy, and ScyllaDB labels and taints, spread across 3 zones.

## Prerequisites

- A Google Cloud project with the [Kubernetes Engine API](https://console.cloud.google.com/apis/api/container.googleapis.com) enabled.
- A GCP account with the [`Kubernetes Engine Admin`](https://cloud.google.com/iam/docs/understanding-roles#container.admin) (`roles/container.admin`) IAM role on the project.
- The [`gcloud` CLI](https://cloud.google.com/sdk/docs/install) installed and configured (`gcloud init`).
- [`kubectl`](https://kubernetes.io/docs/tasks/tools/#kubectl) installed.
- Sufficient quota for `n2-highmem-16` instances with local SSDs in your target region.
  See [ScyllaDB cloud instance recommendations for GCE](https://docs.scylladb.com/manual/stable/getting-started/cloud-instance-recommendations.html#google-compute-engine-gce) for recommended machine types and [system requirements](https://docs.scylladb.com/manual/stable/getting-started/system-requirements.html) for minimum specifications.

## Set environment variables

The rest of the guide refers to the variables defined here.

Set your GCP project and region — these have no defaults and must be provided:

```console
export GCP_PROJECT="<your-project>"  # e.g. my-scylla-project
export GCP_REGION="<your-region>"    # e.g. us-central1
```

The remaining variables have sensible defaults and can be copied as-is.
Override any value before running if needed:

```bash
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
```

## Create a temporary directory

Create a temporary directory for configuration files used in this guide:

```bash
TMPDIR="$(mktemp -d)"
trap 'rm -rf "${TMPDIR}"' EXIT
```

## Create the kubelet configuration

The dedicated ScyllaDB node pool requires [static CPU manager policy](https://kubernetes.io/docs/tasks/administer-cluster/cpu-management-policies/#static-policy-configuration) for CPU pinning.
Create a `systemconfig.yaml` that will be passed to the node pool:

```bash
cat > "${TMPDIR}/systemconfig.yaml" <<EOF
kubeletConfig:
  cpuManagerPolicy: static
EOF
```

## Create the GKE cluster

Create a regional GKE cluster with a system node pool for infrastructure workloads (cert-manager, ScyllaDB Operator, etc.).
The system pool is placed in a single zone to keep costs down:

```bash
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
```

## Create the dedicated ScyllaDB node pool

This pool is dedicated to ScyllaDB workloads.
Each node has local NVMe SSDs for storage and the static CPU manager policy enabled via the system config file created earlier.
The pool spans 3 zones so each ScyllaDB rack can be placed in a separate zone for fault tolerance:

```bash
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
```

#### NOTE
The `--local-nvme-ssd-block` flag provisions raw NVMe block devices that `NodeConfig` will configure as a RAID0 array.
Do not use `--ephemeral-storage-local-ssd`, which makes the SSDs managed by the kubelet and unavailable for direct RAID setup.

## Get cluster credentials

```bash
gcloud container clusters get-credentials "${GKE_CLUSTER_NAME}" \
  --project="${GCP_PROJECT}" \
  --region="${GCP_REGION}"
```

Verify connectivity and node readiness:

```console
kubectl get nodes -L scylla.scylladb.com/node-type
```

Example expected output:

```console
NAME                                              STATUS   ROLES    AGE   VERSION   NODE-TYPE
gke-scylladb-demo-default-pool-xxxxxxxx-xxxx      Ready    <none>   10m   v1.32.1
gke-scylladb-demo-default-pool-xxxxxxxx-xxxx      Ready    <none>   10m   v1.32.1
gke-scylladb-demo-scyllaclusters-xxxxxxxx-xxxx    Ready    <none>   5m    v1.32.1   scylla
gke-scylladb-demo-scyllaclusters-xxxxxxxx-xxxx    Ready    <none>   5m    v1.32.1   scylla
gke-scylladb-demo-scyllaclusters-xxxxxxxx-xxxx    Ready    <none>   5m    v1.32.1   scylla
```

## Clean up

Delete the GKE cluster (this also deletes all node pools):

```console
gcloud container clusters delete "${GKE_CLUSTER_NAME}" \
  --project="${GCP_PROJECT}" \
  --region="${GCP_REGION}" \
  --quiet
```

## Next steps

- Follow the [Reference deployment: GKE](https://operator.docs.scylladb.com/stable/deploy-scylladb/reference-deployments/reference-deployment-gke.md) for a complete ScyllaDB deployment on this cluster.
