#!/usr/bin/env bash

# Copyright (C) 2026 ScyllaDB

set -euExo pipefail
shopt -s inherit_errexit
shopt -s extglob

# Verifies that the release deploy script (hack/ci-deploy-release.sh) works against a KinD cluster.
# It deploys a *released* operator (SO_IMAGE, defaulting to the latest release), with the manifests
# resolved from the image's OCI source/revision labels, i.e. the release's exact git SHA.

readonly repo_root="$( realpath "$( dirname "${BASH_SOURCE[0]}" )/../.." )"

# Ensure all kind calls use podman.
export KIND_EXPERIMENTAL_PROVIDER=podman

if [ -z "${CLUSTER_NAME}" ]; then
  echo "CLUSTER_NAME must be set" > /dev/stderr
  exit 1
fi

REENTRANT="${REENTRANT:-true}"
export REENTRANT

# The released operator image to deploy.
SO_IMAGE="${SO_IMAGE:-docker.io/scylladb/scylla-operator:latest}"
# We need to strip the tag because skopeo doesn't support references with both a tag and a digest.
SO_IMAGE="${SO_IMAGE/:*([^:\/])@/@}"
export SO_IMAGE

ARTIFACTS="${ARTIFACTS:-$( mktemp -d )}/release-deploy"
mkdir -p "${ARTIFACTS}"
export ARTIFACTS

# cleanup collects artifacts (best-effort, before teardown as it needs the cluster) and always tears the cluster
# down, preserving the original exit code. The release deploy dirties the environment (deploys the operator stack),
# so it must not leave the cluster around for reuse.
function cleanup {
  local exit_code=$?
  ( gather-artifacts-on-exit ) || true
  "${repo_root}/hack/kind/cluster-teardown.sh" || true
  rm -f "${KUBECONFIG:-}" || true
  exit "${exit_code}"
}

# The release deploy script deploys the operator stack itself, so cluster-setup.sh only prepares the cluster
# and registry. Force a fresh cluster (RECREATE) in case a previous run was killed before its teardown trap
# fired and left a dirty cluster behind.
export SO_SKIP_DEPLOYMENT=true
export RECREATE=true
"${repo_root}/hack/kind/cluster-setup.sh"
trap cleanup EXIT

# Set KUBECONFIG to point to the kind cluster.
KUBECONFIG="$(mktemp --suffix ".kubeconfig")"
kind get kubeconfig --name="${CLUSTER_NAME}" > "${KUBECONFIG}"
export KUBECONFIG

source "${repo_root}/hack/kind/lib.sh"
source "${repo_root}/hack/lib/kube.sh"
source "${repo_root}/hack/.ci/lib/e2e.sh"

# Use 'standard' storage class that comes with KinD by default.
# This must be set before sourcing run-e2e-shared.env.sh, which would otherwise default to 'scylladb-local-xfs'.
SO_SCYLLACLUSTER_STORAGECLASS_NAME="${SO_SCYLLACLUSTER_STORAGECLASS_NAME:-standard}"
export SO_SCYLLACLUSTER_STORAGECLASS_NAME

# Force-skip the local-csi-driver: KinD uses the 'standard' storage class and has no XFS local disks, so the driver's
# daemonset would never roll out. Note that ci-deploy-release.sh treats an *unset* SO_CSI_DRIVER_PATH as "install the
# released local-csi-driver manifests", so it must be explicitly set to an empty value here.
export SO_CSI_DRIVER_PATH=""

# Leave SO_NODECONFIG_PATH unset: KinD nodes are containers and must not be subject to host-level disk setup
# or performance tuning.

# Use 'io_uring' reactor backend which does not need as high fs.aio-max-nr sysctl setting as the default reactor.
# We do not want to change sysctls on the host running the KinD cluster.
SO_SCYLLACLUSTER_REACTOR_BACKEND="${SO_SCYLLACLUSTER_REACTOR_BACKEND:-io_uring}"
export SO_SCYLLACLUSTER_REACTOR_BACKEND

source "${repo_root}/hack/.ci/run-e2e-shared.env.sh"

run-deploy-script-in-all-clusters "${repo_root}/hack/ci-deploy-release.sh"
