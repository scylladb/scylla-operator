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

source "${repo_root}/hack/kind/lib.sh"
source "${repo_root}/hack/lib/kube.sh"
source "${repo_root}/hack/.ci/lib/e2e.sh"
source "${repo_root}/hack/.ci/run-e2e-shared.env.sh"

# Per-worker copies of the kubeconfig and of the object storage credentials, used by the multi-datacenter suite only.
worker_assets_dir=""

# In the multi-datacenter suite every worker entry points at this very cluster through the internal kubeconfig, which
# is not resolvable from the host. Emptying the array before gathering artifacts collects them once, for the only
# cluster there is, instead of failing on a must-gather per worker entry. It is a no-op for the other suites.
trap 'WORKER_KUBECONFIGS=(); gather-artifacts-on-exit; rm -rf "${KUBECONFIG}" "${IN_CLUSTER_KUBECONFIG}" ${worker_assets_dir:+"${worker_assets_dir}"}' EXIT
trap gracefully-shutdown-e2es INT

build-and-push-operator-image "${repo_root}" SO_IMAGE

# Use 'standard' storage class that comes with KinD by default.
SO_SCYLLACLUSTER_STORAGECLASS_NAME="standard"
export SO_SCYLLACLUSTER_STORAGECLASS_NAME

# Use 'io_uring' reactor backend which does not need as high fs.aio-max-nr sysctl setting as the default reactor.
# We do not want to change sysctls on the host running the KinD cluster.
SO_SCYLLACLUSTER_REACTOR_BACKEND="io_uring"
export SO_SCYLLACLUSTER_REACTOR_BACKEND

SO_SUITE="${SO_SUITE:-kind-fast}"
export SO_SUITE

# Headless node services with PodIP broadcast addresses are what the manual multi-datacenter guide uses, and they make
# external seeds work across namespaces of a single kind cluster.
SO_SCYLLACLUSTER_NODE_SERVICE_TYPE="Headless"
export SO_SCYLLACLUSTER_NODE_SERVICE_TYPE

SO_SCYLLACLUSTER_NODES_BROADCAST_ADDRESS_TYPE="PodIP"
export SO_SCYLLACLUSTER_NODES_BROADCAST_ADDRESS_TYPE

SO_SCYLLACLUSTER_CLIENTS_BROADCAST_ADDRESS_TYPE="PodIP"
export SO_SCYLLACLUSTER_CLIENTS_BROADCAST_ADDRESS_TYPE

ARTIFACTS="${ARTIFACTS:-$( mktemp -d )}"
export ARTIFACTS

# Object storage for the backup/restore e2e tests comes from the MinIO instance deployed by cluster-setup.sh.
# The single bucket settings and the per-worker ones are mutually exclusive in the tests binary, so the suite decides
# which of the two is configured.
case "${SO_SUITE}" in
*multi-datacenter*)
  # The multi-datacenter suite expects a control plane cluster hosting ScyllaDB Manager and a set of worker clusters.
  # Locally there is only one cluster, so every worker entry points at it: the datacenters end up in separate namespaces
  # of the same kind cluster, joined through external seeds. That covers everything except the control plane versus
  # worker topology itself, which only a real multi-cluster job can exercise.

  # A multi-datacenter spec rolls out several ScyllaClusters in sequence, and the specs run one at a time, so the suite
  # needs more headroom than the default single-datacenter run.
  SO_E2E_TIMEOUT="${SO_E2E_TIMEOUT:-2h}"
  export SO_E2E_TIMEOUT

  # The kind cluster has two nodes labelled for ScyllaDB, and every spec in this suite needs both of them for its own
  # datacenters. Running more than one spec at a time deadlocks on node capacity, so this is a requirement of the local
  # setup rather than a throughput choice.
  SO_E2E_PARALLELISM="${SO_E2E_PARALLELISM:-1}"
  export SO_E2E_PARALLELISM

  # Every worker entry needs its own file with a distinct basename, because the basename becomes both the Secret key
  # and the path the file is mounted at inside the e2e Pod; entries sharing a path would collide.
  worker_assets_dir="$( mktemp -d )"

  # Worker cluster identifiers. They name the object storage entries and the kubeconfigs, and are deliberately not the
  # ScyllaDB datacenter names: the specs pick which datacenter draws its bucket from which worker entry. The suite has
  # specs requiring three worker clusters.
  worker_cluster_keys=( "worker-1" "worker-2" "worker-3" )

  for worker_cluster_key in "${worker_cluster_keys[@]}"; do
    worker_kubeconfig="${worker_assets_dir}/${worker_cluster_key}.kubeconfig"
    # The e2e binary runs in a Pod, so worker clusters have to be reached through the internal kubeconfig.
    cp "${IN_CLUSTER_KUBECONFIG}" "${worker_kubeconfig}"
    WORKER_KUBECONFIGS["${worker_cluster_key}"]="${worker_kubeconfig}"

    worker_bucket="manager-multi-datacenter-tests-${worker_cluster_key}"
    kubectl -n minio exec deployment/minio -- mkdir -p "/data/${worker_bucket}"
    WORKER_OBJECT_STORAGE_BUCKETS["${worker_cluster_key}"]="${worker_bucket}"

    # It's the same MinIO instance for every entry, so the credentials and the agent config are shared. Only the file
    # names have to differ.
    worker_s3_credentials="${worker_assets_dir}/${worker_cluster_key}.credentials"
    cp "${repo_root}/hack/kind/minio/credentials" "${worker_s3_credentials}"
    WORKER_S3_CREDENTIALS_PATHS["${worker_cluster_key}"]="${worker_s3_credentials}"

    worker_s3_agent_config="${worker_assets_dir}/${worker_cluster_key}.agent-config.yaml"
    cp "${repo_root}/hack/kind/minio/agent-config.yaml" "${worker_s3_agent_config}"
    WORKER_S3_AGENT_CONFIG_PATHS["${worker_cluster_key}"]="${worker_s3_agent_config}"
  done
  ;;
*)
  SO_E2E_TIMEOUT="${SO_E2E_TIMEOUT:-1h}"
  export SO_E2E_TIMEOUT

  SO_S3_CREDENTIALS_PATH="${repo_root}/hack/kind/minio/credentials"
  export SO_S3_CREDENTIALS_PATH

  SO_S3_AGENT_CONFIG_PATH="${repo_root}/hack/kind/minio/agent-config.yaml"
  export SO_S3_AGENT_CONFIG_PATH

  SO_BUCKET_NAME="manager-backup-tests"
  kubectl -n minio exec deployment/minio -- mkdir -p "/data/${SO_BUCKET_NAME}"
  ;;
esac

apply-e2e-workarounds
run-e2e
