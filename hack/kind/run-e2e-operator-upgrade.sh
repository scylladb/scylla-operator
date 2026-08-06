#!/usr/bin/env bash

# Copyright (C) 2026 ScyllaDB

set -euExo pipefail
shopt -s inherit_errexit

readonly repo_root="$( dirname "${BASH_SOURCE[0]}" )/../.."

# Ensure all kind calls use podman.
export KIND_EXPERIMENTAL_PROVIDER=podman

if [ -z "${CLUSTER_NAME}" ]; then
  echo "CLUSTER_NAME must be set" > /dev/stderr
  exit 1
fi

REENTRANT="${REENTRANT:-true}"
export REENTRANT

# cleanup collects artifacts (best-effort, before teardown as it needs the cluster) and always tears the cluster
# down, preserving the original exit code. Unlike other kind e2e tests, this one dirties the environment (deploys
# and upgrades the operator stack), so it must not leave the cluster around for reuse.
function cleanup {
  local exit_code=$?
  # The must-gather image has to be a full ref; resolve a bare released version against the released repository
  # (mirroring getOperatorImageRef in the test).
  local must_gather_image="${OPERATOR_UPGRADE_TO_VERSION:-}"
  if [ -n "${must_gather_image}" ] && [[ "${must_gather_image}" != */* ]]; then
    must_gather_image="docker.io/scylladb/scylla-operator:${must_gather_image}"
  fi
  ( gather-artifacts-on-exit "${must_gather_image}" ) || true
  "${repo_root}/hack/kind/cluster-teardown.sh" || true
  rm -f "${KUBECONFIG:-}" || true
  exit "${exit_code}"
}

# The test deploys the operator stack itself (released version, then the upgrade target); cluster-setup.sh only
# prepares the cluster and registry. Force a fresh cluster (RECREATE) in case a previous run was killed before its
# teardown trap fired and left a dirty cluster behind.
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
# The upgrade re-applies the scylla-manager ScyllaCluster, and changing its storage class is forbidden by the webhook.
# This must be set before sourcing run-e2e-shared.env.sh, which would otherwise default to 'scylladb-local-xfs'.
SO_SCYLLACLUSTER_STORAGECLASS_NAME="${SO_SCYLLACLUSTER_STORAGECLASS_NAME:-standard}"
export SO_SCYLLACLUSTER_STORAGECLASS_NAME

# Force-skip the local-csi-driver: KinD uses the 'standard' storage class and has no XFS local disks, so the driver's
# daemonset would never roll out.
export SO_CSI_DRIVER_PATH=""

source "${repo_root}/hack/.ci/run-e2e-shared.env.sh"

# Keep this test's artifacts (test output and must-gather) under a dedicated sub-dir so they are easy to find,
# in particular locally where ARTIFACTS otherwise defaults to a throwaway temp dir.
ARTIFACTS="${ARTIFACTS:-$( mktemp -d )}/upgrade-operator"
mkdir -p "${ARTIFACTS}"
export ARTIFACTS

# Optional upgrade target override (e.g. parsed from the triggering CI comment). A value other than "latest"
# (a released version or a full image ref) is used directly as the upgrade target; unset or "latest" builds the
# current tree and pushes it to the local registry. Env name matches run-e2e in hack/.ci/lib/e2e.sh.
if [ "${OPERATOR_UPGRADE_TO_VERSION:-latest}" == "latest" ]; then
  OPERATOR_UPGRADE_TO_VERSION=""
fi
build-and-push-operator-image "${repo_root}" OPERATOR_UPGRADE_TO_VERSION

# Version to upgrade from; env-overridable, defaulting from the config assets like run-e2e in hack/.ci/lib/e2e.sh.
OPERATOR_UPGRADE_FROM_VERSION="${OPERATOR_UPGRADE_FROM_VERSION:-$( yq '.operatorTests.operatorVersions.upgradeFrom' "${repo_root}/assets/config/config.yaml" )}"

apply-e2e-workarounds

# Run the test directly on the host (not in a pod) because it needs access to the repo
# to call hack/ci-deploy.sh for the operator upgrade.
go run "${repo_root}/cmd/scylla-operator-tests" run kind-operator-upgrade \
  --kubeconfig="${KUBECONFIG}" \
  --loglevel=2 \
  --color=false \
  --artifacts-dir="${ARTIFACTS}" \
  --scyllacluster-node-service-type=Headless \
  --scyllacluster-nodes-broadcast-address-type=PodIP \
  --scyllacluster-clients-broadcast-address-type=PodIP \
  --scyllacluster-storageclass-name=standard \
  --scyllacluster-reactor-backend=io_uring \
  --operator-upgrade-from-version="${OPERATOR_UPGRADE_FROM_VERSION}" \
  --operator-upgrade-to-version="${OPERATOR_UPGRADE_TO_VERSION}"
