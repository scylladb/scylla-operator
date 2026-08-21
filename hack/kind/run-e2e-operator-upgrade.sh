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

# Git ref worktrees created by resolve-operator-ref below; removed by cleanup.
worktrees=()

# cleanup collects artifacts (best-effort, before teardown as it needs the cluster) and always tears the cluster
# down, preserving the original exit code. Unlike other kind e2e tests, this one dirties the environment (deploys
# and upgrades the operator stack), so it must not leave the cluster around for reuse.
function cleanup {
  local exit_code=$?
  # The must-gather image has to be a full ref; by now the "to" ref is resolved (an image ref) or a bare
  # released version, which resolves against the released repository (mirroring getOperatorImageRef in the test).
  local must_gather_image="${OPERATOR_UPGRADE_TO_REF:-}"
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

# The test deploys the operator stack itself (released version, then the upgrade target), so cluster-setup.sh only
# prepares the cluster and registry. Force a fresh cluster (RECREATE) in case a previous run was killed before its
# teardown trap fired and left a dirty cluster behind.
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

# Use 'standard' storage class that comes with KinD by default, matching cluster-setup.sh's initial deploy.
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

# Endpoints of the upgrade; env-overridable. Each accepts a full image ref (used verbatim), a released version
# like "1.20.2" (pulled from the released repository), or any git ref of the canonical repository - branch, tag
# or full commit SHA - built from source.
# Defaults: the ref upgraded from is the previous minor's highest released patch when the current branch is
# a release branch (vX.Y), the highest released version otherwise (master, feature branches); the ref upgraded
# to is the checked-out tree (empty ref), built from source. The branch comes from the CI event context - the PR
# target branch on pull requests, the ref the workflow runs on otherwise - because checkouts in CI are detached
# HEADs, often shallow, so git can't tell.

# operator-upgrade-from-ref prints the highest stable released version (without the leading "v") - among the
# minors strictly below X.Y (the previous minor's highest patch) when the given branch is a release branch
# "vX.Y", overall otherwise (master, feature branches).
function operator-upgrade-from-ref {
  local branch="${1-}"
  local tags

  # Always list the canonical repository's tags - the local clone may be shallow, a fork without the release
  # tags, or simply stale (cloned before the newest release was tagged).
  tags="$( git ls-remote --tags --refs https://github.com/scylladb/scylla-operator.git | sed -e 's|.*refs/tags/||' | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$' )"

  if [[ "${branch}" =~ ^v([0-9]+)\.([0-9]+)$ ]]; then
    tags="$( awk -F '[v.]' -v major="${BASH_REMATCH[1]}" -v minor="${BASH_REMATCH[2]}" '$2+0 < major+0 || ($2+0 == major+0 && $3+0 < minor+0)' <<< "${tags}" )"
  fi

  if [ -z "${tags}" ]; then
    echo "Can't determine the released version to upgrade from: no release tag matching branch \"${branch}\"" >&2
    return 1
  fi

  sort -V <<< "${tags}" | tail -n 1 | sed -e 's/^v//'
}

current_branch="${GITHUB_BASE_REF:-${GITHUB_REF_NAME:-$( git -C "${repo_root}" rev-parse --abbrev-ref HEAD )}}"
OPERATOR_UPGRADE_FROM_REF="${OPERATOR_UPGRADE_FROM_REF:-$( operator-upgrade-from-ref "${current_branch}" )}"
OPERATOR_UPGRADE_TO_REF="${OPERATOR_UPGRADE_TO_REF:-}"

# resolve-operator-ref resolves the ref in the variable named by $1 into a deployable image ref, and stores the
# tree to deploy from in the variable named by $2 — the test runs that tree's deploy script, so the deployed
# manifests match the tree the image was built from.
# A full image ref is used verbatim and a released version (with or without the git-style "v" prefix) resolves
# against the released repository — those deploy via hack/ci-deploy-release.sh, which takes the manifests from
# the image's OCI source/revision labels, the release's exact git SHA. Anything else is built from source and
# pushed to the kind registry as a digest-pinned ref: an empty value builds the checked-out tree; a git ref of
# the canonical repository (branch, tag, full commit SHA) is fetched into a temporary worktree and built there.
function resolve-operator-ref {
  local -n ref="${1:?Missing ref variable name}"
  local -n deploy_dir="${2:?Missing deploy dir variable name}"

  deploy_dir="${repo_root}"
  if [[ "${ref}" == */* ]]; then
    # A full image ref needs no build.
    return 0
  elif [[ "${ref}" =~ ^v?([0-9]+\.[0-9]+\.[0-9]+)$ ]]; then
    # A released version needs no build; normalize the git-style "v" prefix away to match the image tags.
    ref="${BASH_REMATCH[1]}"
    return 0
  elif [ -n "${ref}" ]; then
    deploy_dir="$( mktemp -d )"
    worktrees+=( "${deploy_dir}" )
    # Fetch by URL: CI checkouts have no "origin" remote, and a local clone's "origin" may be a fork without
    # the ref. GitHub serves branches, tags and full commit SHAs; anything else fails here.
    if ! git -C "${repo_root}" fetch https://github.com/scylladb/scylla-operator.git "${ref}"; then
      echo "Can't resolve \"${ref}\": expected a full image ref, a released version or a git ref (branch, tag or full commit SHA) of the canonical repository" >&2
      return 1
    fi
    git -C "${repo_root}" worktree add --detach "${deploy_dir}" FETCH_HEAD
    ref=""
  fi
  build-and-push-operator-image "${deploy_dir}" "${1}"
}

resolve-operator-ref OPERATOR_UPGRADE_FROM_REF operator_upgrade_from_deploy_dir
resolve-operator-ref OPERATOR_UPGRADE_TO_REF operator_upgrade_to_deploy_dir

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
  --operator-upgrade-from-version="${OPERATOR_UPGRADE_FROM_REF}" \
  --operator-upgrade-from-deploy-dir="${operator_upgrade_from_deploy_dir}" \
  --operator-upgrade-to-version="${OPERATOR_UPGRADE_TO_REF}" \
  --operator-upgrade-to-deploy-dir="${operator_upgrade_to_deploy_dir}"
