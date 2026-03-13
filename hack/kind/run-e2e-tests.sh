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

trap 'gather-artifacts-on-exit; rm -f "${KUBECONFIG}" "${IN_CLUSTER_KUBECONFIG}"' EXIT
trap gracefully-shutdown-e2es INT

build-and-push-operator-image "${repo_root}"

# Use 'standard' storage class that comes with KinD by default.
SO_SCYLLACLUSTER_STORAGECLASS_NAME="standard"
export SO_SCYLLACLUSTER_STORAGECLASS_NAME

# Use 'io_uring' reactor backend which does not need as high fs.aio-max-nr sysctl setting as the default reactor.
# We do not want to change sysctls on the host running the KinD cluster.
SO_SCYLLACLUSTER_REACTOR_BACKEND="io_uring"
export SO_SCYLLACLUSTER_REACTOR_BACKEND

SO_SUITE="kind-fast"
export SO_SUITE

SO_SCYLLACLUSTER_NODE_SERVICE_TYPE="Headless"
export SO_SCYLLACLUSTER_NODE_SERVICE_TYPE

SO_SCYLLACLUSTER_NODES_BROADCAST_ADDRESS_TYPE="PodIP"
export SO_SCYLLACLUSTER_NODES_BROADCAST_ADDRESS_TYPE

SO_SCYLLACLUSTER_CLIENTS_BROADCAST_ADDRESS_TYPE="PodIP"
export SO_SCYLLACLUSTER_CLIENTS_BROADCAST_ADDRESS_TYPE

SO_E2E_TIMEOUT="1h"
export SO_E2E_TIMEOUT

ARTIFACTS="${ARTIFACTS:-$( mktemp -d )}"
export ARTIFACTS

apply-e2e-workarounds
run-e2e
