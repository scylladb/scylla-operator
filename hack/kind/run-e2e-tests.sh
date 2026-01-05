#!/usr/bin/env bash

set -euExo pipefail
shopt -s inherit_errexit

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
KUBECONFIG="$( mktemp )"
kind get kubeconfig --name="${CLUSTER_NAME}" > "${KUBECONFIG}"
export KUBECONFIG
trap 'rm -f "${KUBECONFIG}"' EXIT

readonly parent_dir="$( dirname "${BASH_SOURCE[0]}" )"

source "${parent_dir}/../lib/kube.sh"
source "${parent_dir}/../.ci/lib/e2e.sh"
source "${parent_dir}/../.ci/run-e2e-shared.env.sh"

trap gather-artifacts-on-exit EXIT
trap gracefully-shutdown-e2es INT

# Use 'standard' storage class that comes with KinD by default.
SO_SCYLLACLUSTER_STORAGECLASS_NAME="standard"
export SO_SCYLLACLUSTER_STORAGECLASS_NAME

SO_SUITE="kind-fast"
export SO_SUITE

SO_SCYLLACLUSTER_NODE_SERVICE_TYPE="Headless"
export SO_SCYLLACLUSTER_NODE_SERVICE_TYPE

SO_SCYLLACLUSTER_NODES_BROADCAST_ADDRESS_TYPE="PodIP"
export SO_SCYLLACLUSTER_NODES_BROADCAST_ADDRESS_TYPE

SO_SCYLLACLUSTER_CLIENTS_BROADCAST_ADDRESS_TYPE="PodIP"
export SO_SCYLLACLUSTER_CLIENTS_BROADCAST_ADDRESS_TYPE

ARTIFACTS="${ARTIFACTS:-$( mktemp -d )}"
export ARTIFACTS

"${parent_dir}"/../ci-deploy.sh "${SO_IMAGE}"

apply-e2e-workarounds
run-e2e
