#!/usr/bin/env bash

# Copyright (C) 2026 ScyllaDB

set -euExo pipefail
shopt -s inherit_errexit

# Absolute, so it stays valid when the test is run from the build root below.
readonly repo_root="$( realpath "$( dirname "${BASH_SOURCE[0]}" )/../.." )"

# Ensure all kind calls use podman.
export KIND_EXPERIMENTAL_PROVIDER=podman

if [ -z "${CLUSTER_NAME}" ]; then
  echo "CLUSTER_NAME must be set" > /dev/stderr
  exit 1
fi

REENTRANT="${REENTRANT:-true}"
export REENTRANT

# Release branch worktrees created by resolve-operator-version below; removed by cleanup.
worktrees=()

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
  for worktree in "${worktrees[@]}"; do
    git -C "${repo_root}" worktree remove --force "${worktree}" || true
  done
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

# Versions to upgrade between; env-overridable (e.g. parsed from the triggering CI comment), defaulting from the
# config assets like run-e2e in hack/.ci/lib/e2e.sh. Each accepts a released version, a full image ref, "latest"
# (build the current tree) or "<major.minor>-latest" (e.g. "1.21-latest": build the tip of the corresponding
# release branch, v1.21); the "-latest" forms are kind-runner-only, as they require building an image.
OPERATOR_UPGRADE_FROM_VERSION="${OPERATOR_UPGRADE_FROM_VERSION:-$( yq '.operatorTests.operatorVersions.upgradeFrom' "${repo_root}/assets/config/config.yaml" )}"
OPERATOR_UPGRADE_TO_VERSION="${OPERATOR_UPGRADE_TO_VERSION:-$( yq '.operatorTests.operatorVersions.upgradeTo' "${repo_root}/assets/config/config.yaml" )}"

# resolve-operator-version resolves the "latest"/"<major.minor>-latest" forms in the version variable named by $1:
# it builds the image from the corresponding tree (the current one, or a temporary release branch worktree) and
# replaces the value with the SHA-pinned image ref. The tree is stored in the variable named by $2 — the test runs
# that version's deploy script from it, so the deployed manifests match the tree the image was built from.
# Released versions and full image refs are left as-is; a released version deploys via hack/ci-deploy-release.sh,
# which resolves the manifests from the image's OCI source/revision labels — the release's exact git SHA.
function resolve-operator-version {
  local -n version="${1:?Missing version variable name}"
  local -n deploy_dir="${2:?Missing deploy dir variable name}"

  deploy_dir="${repo_root}"
  if [[ "${version}" =~ ^([0-9]+\.[0-9]+)-latest$ ]]; then
    local release_branch="v${BASH_REMATCH[1]}"
    deploy_dir="$( mktemp -d )"
    worktrees+=( "${deploy_dir}" )
    # Fetch by URL: CI checkouts (Prow clonerefs) have no "origin" remote, and a local clone's
    # "origin" may be a fork without the release branches.
    git -C "${repo_root}" fetch https://github.com/scylladb/scylla-operator.git "${release_branch}"
    git -C "${repo_root}" worktree add --detach "${deploy_dir}" FETCH_HEAD
    version=""
  elif [ "${version}" == "latest" ]; then
    version=""
  else
    # A released version or a full image ref needs no build.
    return 0
  fi
  build-and-push-operator-image "${deploy_dir}" "${1}"
}

resolve-operator-version OPERATOR_UPGRADE_FROM_VERSION operator_upgrade_from_deploy_dir
resolve-operator-version OPERATOR_UPGRADE_TO_VERSION operator_upgrade_to_deploy_dir

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
  --operator-upgrade-from-deploy-dir="${operator_upgrade_from_deploy_dir}" \
  --operator-upgrade-to-version="${OPERATOR_UPGRADE_TO_VERSION}" \
  --operator-upgrade-to-deploy-dir="${operator_upgrade_to_deploy_dir}"
