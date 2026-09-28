#!/usr/bin/env bash

# Copyright (C) 2026 ScyllaDB

set -euExo pipefail
shopt -s inherit_errexit

# Runs the multi-datacenter e2e suite against a single KinD cluster impersonating multiple datacenters.
# The suite requires multiple Kubernetes contexts with mutually routable Pod IPs, but the test framework
# never requires the worker kubeconfigs to point at distinct physical clusters. A single cluster referenced
# by several worker identifiers satisfies it: each worker gets its own namespace, and Pod IPs are trivially
# routable. Cross-cluster networking is not exercised by this setup.

readonly repo_root="$( dirname "${BASH_SOURCE[0]}" )/../.."

# Ensure all kind calls use podman.
export KIND_EXPERIMENTAL_PROVIDER=podman

if [ -z "${CLUSTER_NAME}" ]; then
  echo "CLUSTER_NAME must be set" > /dev/stderr
  exit 1
fi

REENTRANT="${REENTRANT:-true}"
export REENTRANT

# Set up the KinD cluster and deploy the operator stack.
"${repo_root}/hack/kind/cluster-setup.sh"

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

# Worker cluster identifiers; each becomes a datacenter in multi-datacenter tests.
worker_names=(
  "dc0"
  "dc1"
  "dc2"
)

# All worker identifiers point at the same KinD cluster. Host-side helpers (artifact gathering) use
# WORKER_KUBECONFIGS and skip entries equal to KUBECONFIG, so the shared cluster is only handled once.
# The e2e Pod uses the in-cluster variants, which need distinct basenames as they are projected into
# a single Secret keyed by file basename.
worker_in_cluster_kubeconfigs_dir="$( mktemp -d --suffix ".worker-kubeconfigs" )"
for name in "${worker_names[@]}"; do
  WORKER_KUBECONFIGS["${name}"]="${KUBECONFIG}"
  cp "${IN_CLUSTER_KUBECONFIG}" "${worker_in_cluster_kubeconfigs_dir}/${name}.kubeconfig"
  WORKER_IN_CLUSTER_KUBECONFIGS["${name}"]="${worker_in_cluster_kubeconfigs_dir}/${name}.kubeconfig"
done

trap 'gather-artifacts-on-exit; rm -f "${KUBECONFIG}" "${IN_CLUSTER_KUBECONFIG}"; rm -rf "${worker_in_cluster_kubeconfigs_dir}"' EXIT
trap gracefully-shutdown-e2es INT

build-and-push-operator-image "${repo_root}" SO_IMAGE

# Use 'standard' storage class that comes with KinD by default.
SO_SCYLLACLUSTER_STORAGECLASS_NAME="${SO_SCYLLACLUSTER_STORAGECLASS_NAME:-standard}"
export SO_SCYLLACLUSTER_STORAGECLASS_NAME

# Use 'io_uring' reactor backend which does not need as high fs.aio-max-nr sysctl setting as the default reactor.
# We do not want to change sysctls on the host running the KinD cluster.
SO_SCYLLACLUSTER_REACTOR_BACKEND="${SO_SCYLLACLUSTER_REACTOR_BACKEND:-io_uring}"
export SO_SCYLLACLUSTER_REACTOR_BACKEND

SO_SUITE="${SO_SUITE:-scylla-operator/conformance/multi-datacenter-parallel}"
export SO_SUITE

SO_SCYLLACLUSTER_NODE_SERVICE_TYPE="${SO_SCYLLACLUSTER_NODE_SERVICE_TYPE:-Headless}"
export SO_SCYLLACLUSTER_NODE_SERVICE_TYPE

SO_SCYLLACLUSTER_NODES_BROADCAST_ADDRESS_TYPE="${SO_SCYLLACLUSTER_NODES_BROADCAST_ADDRESS_TYPE:-PodIP}"
export SO_SCYLLACLUSTER_NODES_BROADCAST_ADDRESS_TYPE

SO_SCYLLACLUSTER_CLIENTS_BROADCAST_ADDRESS_TYPE="${SO_SCYLLACLUSTER_CLIENTS_BROADCAST_ADDRESS_TYPE:-PodIP}"
export SO_SCYLLACLUSTER_CLIENTS_BROADCAST_ADDRESS_TYPE

# Multi-datacenter specs create one ScyllaDB cluster per worker each, so bound the parallelism to keep
# the resource footprint manageable on a single KinD cluster.
SO_E2E_PARALLELISM="${SO_E2E_PARALLELISM:-1}"
export SO_E2E_PARALLELISM

SO_E2E_TIMEOUT="${SO_E2E_TIMEOUT:-2h}"
export SO_E2E_TIMEOUT

ARTIFACTS="${ARTIFACTS:-$( mktemp -d )}"
export ARTIFACTS

# TODO: Once the multi-datacenter ScyllaDB Manager e2e coverage is ported onto the external-seeds setup,
#       provision a per-worker MinIO bucket here and populate WORKER_OBJECT_STORAGE_BUCKETS and
#       WORKER_S3_CREDENTIALS_PATHS accordingly.

apply-e2e-workarounds
run-e2e
