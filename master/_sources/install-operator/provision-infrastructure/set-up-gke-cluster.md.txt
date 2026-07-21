# Set up a GKE cluster for ScyllaDB

This guide provisions a Google Kubernetes Engine (GKE) cluster suitable for running ScyllaDB.
At the end, you will have:

- A regional GKE cluster with a system node pool for infrastructure workloads.
- A dedicated node pool with local NVMe SSDs, static CPU manager policy, and ScyllaDB labels and taints, spread across 3 zones.

:::{tip}
All the steps in this guide are also available as a single executable script:
{{ '[`examples/gke/setup-gke-cluster.sh`](https://github.com/{}/blob/{}/examples/gke/setup-gke-cluster.sh)'.format(repository, revision) }}.
Set `GCP_PROJECT` and `GCP_REGION`, then run the script to provision everything in one go.
:::

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

:::{code-block} console
export GCP_PROJECT="<your-project>"  # e.g. my-scylla-project
export GCP_REGION="<your-region>"    # e.g. us-central1
:::

The remaining variables have sensible defaults and can be copied as-is.
Override any value before running if needed:

:::{literalinclude} ../../../../examples/gke/setup-gke-cluster.sh
:language: bash
:start-after: "# [START env_defaults]"
:end-before: "# [END env_defaults]"
:::

## Create a temporary directory

Create a temporary directory for configuration files used in this guide:

:::{literalinclude} ../../../../examples/gke/setup-gke-cluster.sh
:language: bash
:start-after: "# [START tmpdir]"
:end-before: "# [END tmpdir]"
:::

## Create the kubelet configuration

The dedicated ScyllaDB node pool requires [static CPU manager policy](https://kubernetes.io/docs/tasks/administer-cluster/cpu-management-policies/#static-policy-configuration) for CPU pinning.
Create a `systemconfig.yaml` that will be passed to the node pool:

:::{literalinclude} ../../../../examples/gke/setup-gke-cluster.sh
:language: bash
:start-after: "# [START systemconfig]"
:end-before: "# [END systemconfig]"
:::

## Create the GKE cluster

Create a regional GKE cluster with a system node pool for infrastructure workloads (cert-manager, ScyllaDB Operator, etc.).
The system pool is placed in a single zone to keep costs down:

:::{literalinclude} ../../../../examples/gke/setup-gke-cluster.sh
:language: bash
:start-after: "# [START create_cluster]"
:end-before: "# [END create_cluster]"
:::

## Create the dedicated ScyllaDB node pool

This pool is dedicated to ScyllaDB workloads.
Each node has local NVMe SSDs for storage and the static CPU manager policy enabled via the system config file created earlier.
The pool spans 3 zones so each ScyllaDB rack can be placed in a separate zone for fault tolerance:

:::{literalinclude} ../../../../examples/gke/setup-gke-cluster.sh
:language: bash
:start-after: "# [START create_scylla_pool]"
:end-before: "# [END create_scylla_pool]"
:::

:::{note}
The `--local-nvme-ssd-block` flag provisions raw NVMe block devices that `NodeConfig` will configure as a RAID0 array.
Do not use `--ephemeral-storage-local-ssd`, which makes the SSDs managed by the kubelet and unavailable for direct RAID setup.
:::

## Get cluster credentials

:::{literalinclude} ../../../../examples/gke/setup-gke-cluster.sh
:language: bash
:start-after: "# [START get_credentials]"
:end-before: "# [END get_credentials]"
:::

Verify connectivity and node readiness:

:::{code-block} console
kubectl get nodes -L scylla.scylladb.com/node-type
:::

Example expected output:

:::{code-block} console
NAME                                              STATUS   ROLES    AGE   VERSION   NODE-TYPE
gke-scylladb-demo-default-pool-xxxxxxxx-xxxx      Ready    <none>   10m   v1.32.1
gke-scylladb-demo-default-pool-xxxxxxxx-xxxx      Ready    <none>   10m   v1.32.1
gke-scylladb-demo-scyllaclusters-xxxxxxxx-xxxx    Ready    <none>   5m    v1.32.1   scylla
gke-scylladb-demo-scyllaclusters-xxxxxxxx-xxxx    Ready    <none>   5m    v1.32.1   scylla
gke-scylladb-demo-scyllaclusters-xxxxxxxx-xxxx    Ready    <none>   5m    v1.32.1   scylla
:::

## Clean up

Delete the GKE cluster (this also deletes all node pools):

:::{code-block} console
gcloud container clusters delete "${GKE_CLUSTER_NAME}" \
  --project="${GCP_PROJECT}" \
  --region="${GCP_REGION}" \
  --quiet
:::

## Next steps

- Follow the [Reference deployment: GKE](../../deploy-scylladb/reference-deployments/reference-deployment-gke.md) for a complete ScyllaDB deployment on this cluster.
