#!/usr/bin/env bash

set -euExo pipefail
shopt -s inherit_errexit

readonly repo_root="$( dirname "${BASH_SOURCE[0]}" )/../.."

# Ensure all kind calls use podman.
export KIND_EXPERIMENTAL_PROVIDER=podman

if [ -z "${CLUSTER_NAME}" ]; then
  echo "CLUSTER_NAME must be set" > /dev/stderr
  exit 1
fi

# Sanity check: make sure kind cluster exists.
if ! kind get clusters | grep -q "^${CLUSTER_NAME}$"; then
    echo "Kind cluster ${CLUSTER_NAME} does not exist" > /dev/stderr
    exit 1
fi

REENTRANT="${REENTRANT:-true}"
export REENTRANT

# Set KUBECONFIG to point to the kind cluster.
KUBECONFIG="$(mktemp --suffix ".kubeconfig")"
kind get kubeconfig --name="${CLUSTER_NAME}" > "${KUBECONFIG}"
export KUBECONFIG

# Set IN_CLUSTER_KUBECONFIG for use by the e2e tests Pod itself.
IN_CLUSTER_KUBECONFIG="$(mktemp --suffix ".kubeconfig")"
kind get kubeconfig --name="${CLUSTER_NAME}" --internal > "${IN_CLUSTER_KUBECONFIG}"
export IN_CLUSTER_KUBECONFIG

# Per-worker copies of the kubeconfig and of the object storage credentials, used by the multi-datacenter suite only.
worker_assets_dir=""

# Replaced with the full exit trap once the e2e library is sourced.
trap 'rm -rf "${KUBECONFIG}" "${IN_CLUSTER_KUBECONFIG}" ${worker_assets_dir:+"${worker_assets_dir}"}' EXIT

SO_SUITE="${SO_SUITE:-kind-fast}"
export SO_SUITE

# Object storage for the backup/restore e2e tests comes from the MinIO instance deployed by cluster-setup.sh.
# The single bucket settings and the per-worker ones are mutually exclusive in the tests binary, so the suite decides
# which of the two is configured.
case "${SO_SUITE}" in
*multi-datacenter*)
  # The multi-datacenter suite expects a control plane cluster hosting ScyllaDB Manager and a set of worker clusters.
  # Locally there is only one cluster, so every worker entry points at it: the datacenters end up in separate namespaces
  # of the same kind cluster, joined through external seeds. That covers everything except the control plane versus
  # worker topology itself, which only a real multi-cluster job can exercise.

  # Every worker entry needs its own file with a distinct basename, because the basename becomes both the Secret key
  # and the path the file is mounted at inside the e2e Pod; entries sharing a path would collide.
  worker_assets_dir="$( mktemp -d )"

  # Three worker entries, mirroring the CI multi-datacenter cluster set in hack/.ci/manifests/ci-clusters/gke-multi-dc.yaml.
  worker_cluster_keys=( "worker-1" "worker-2" "worker-3" )

  worker_kubeconfigs=()
  worker_object_storage_buckets=()
  worker_s3_credentials_paths=()
  worker_s3_agent_config_paths=()

  for worker_cluster_key in "${worker_cluster_keys[@]}"; do
    worker_kubeconfig="${worker_assets_dir}/${worker_cluster_key}.kubeconfig"
    # The e2e binary runs in a Pod, so worker clusters have to be reached through the internal kubeconfig.
    cp "${IN_CLUSTER_KUBECONFIG}" "${worker_kubeconfig}"
    worker_kubeconfigs+=( "${worker_cluster_key}=${worker_kubeconfig}" )

    worker_bucket="manager-multi-datacenter-tests-${worker_cluster_key}"
    kubectl -n minio exec deployment/minio -- mkdir -p "/data/${worker_bucket}"
    worker_object_storage_buckets+=( "${worker_cluster_key}=${worker_bucket}" )

    # It's the same MinIO instance for every entry, so the credentials and the agent config are shared. Only the file
    # names have to differ.
    worker_s3_credentials="${worker_assets_dir}/${worker_cluster_key}.credentials"
    cp "${repo_root}/hack/kind/minio/credentials" "${worker_s3_credentials}"
    worker_s3_credentials_paths+=( "${worker_cluster_key}=${worker_s3_credentials}" )

    worker_s3_agent_config="${worker_assets_dir}/${worker_cluster_key}.agent-config.yaml"
    cp "${repo_root}/hack/kind/minio/agent-config.yaml" "${worker_s3_agent_config}"
    worker_s3_agent_config_paths+=( "${worker_cluster_key}=${worker_s3_agent_config}" )
  done

  WORKER_KUBECONFIGS="$( IFS=','; echo "${worker_kubeconfigs[*]}" )"
  export WORKER_KUBECONFIGS

  WORKER_OBJECT_STORAGE_BUCKETS="$( IFS=','; echo "${worker_object_storage_buckets[*]}" )"
  export WORKER_OBJECT_STORAGE_BUCKETS

  WORKER_S3_CREDENTIALS_PATHS="$( IFS=','; echo "${worker_s3_credentials_paths[*]}" )"
  export WORKER_S3_CREDENTIALS_PATHS

  WORKER_S3_AGENT_CONFIG_PATHS="$( IFS=','; echo "${worker_s3_agent_config_paths[*]}" )"
  export WORKER_S3_AGENT_CONFIG_PATHS
  ;;
*)
  SO_S3_CREDENTIALS_PATH="${repo_root}/hack/kind/minio/credentials"
  export SO_S3_CREDENTIALS_PATH

  SO_S3_AGENT_CONFIG_PATH="${repo_root}/hack/kind/minio/agent-config.yaml"
  export SO_S3_AGENT_CONFIG_PATH

  SO_BUCKET_NAME="manager-backup-tests"
  kubectl -n minio exec deployment/minio -- mkdir -p "/data/${SO_BUCKET_NAME}"
  ;;
esac

source "${repo_root}/hack/kind/lib.sh"
source "${repo_root}/hack/lib/kube.sh"
source "${repo_root}/hack/.ci/lib/e2e.sh"
source "${repo_root}/hack/.ci/run-e2e-shared.env.sh"

# In the multi-datacenter suite every worker entry points at this very cluster through the internal kubeconfig, which
# is not resolvable from the host. The artifacts are gathered once, for the only cluster there is, instead of through
# gather-artifacts-on-exit, which would also run a must-gather per worker entry and fail on each.
trap 'ec=$?; gather-artifacts "${ARTIFACTS}/must-gather/cluster"; cleanup-bg-jobs "${ec}"; rm -rf "${KUBECONFIG}" "${IN_CLUSTER_KUBECONFIG}" ${worker_assets_dir:+"${worker_assets_dir}"}' EXIT
trap gracefully-shutdown-e2es INT

build-and-push-operator-image "${repo_root}" SO_IMAGE

# Use 'standard' storage class that comes with KinD by default.
SO_SCYLLACLUSTER_STORAGECLASS_NAME="standard"
export SO_SCYLLACLUSTER_STORAGECLASS_NAME

# Use 'io_uring' reactor backend which does not need as high fs.aio-max-nr sysctl setting as the default reactor.
# We do not want to change sysctls on the host running the KinD cluster.
SO_SCYLLACLUSTER_REACTOR_BACKEND="io_uring"
export SO_SCYLLACLUSTER_REACTOR_BACKEND

SO_SCYLLACLUSTER_NODE_SERVICE_TYPE="Headless"
export SO_SCYLLACLUSTER_NODE_SERVICE_TYPE

SO_SCYLLACLUSTER_NODES_BROADCAST_ADDRESS_TYPE="PodIP"
export SO_SCYLLACLUSTER_NODES_BROADCAST_ADDRESS_TYPE

SO_SCYLLACLUSTER_CLIENTS_BROADCAST_ADDRESS_TYPE="PodIP"
export SO_SCYLLACLUSTER_CLIENTS_BROADCAST_ADDRESS_TYPE

SO_E2E_TIMEOUT="${SO_E2E_TIMEOUT:-1h}"
export SO_E2E_TIMEOUT

ARTIFACTS="${ARTIFACTS:-$( mktemp -d )}"
export ARTIFACTS

apply-e2e-workarounds
run-e2e
