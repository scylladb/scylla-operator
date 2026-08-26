#!/usr/bin/env bash

# Copyright (C) 2026 ScyllaDB

set -euExo pipefail
shopt -s inherit_errexit

# Absolute, so it stays valid when the test is run from the build root below.
readonly repo_root="$( realpath "$( dirname "${BASH_SOURCE[0]}" )/../.." )"
# The test invokes the deploy scripts via relative paths, so everything runs from the repository root.
cd "${repo_root}"

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
  # The must-gather image has to be a full ref; by now the "to" ref is resolved (an image ref) or a bare
  # released version, which resolves against the released repository (mirroring getOperatorImageRef in the test).
  local must_gather_image="${OPERATOR_UPGRADE_TO_REF:-}"
  if [ -n "${must_gather_image}" ] && [[ "${must_gather_image}" != */* ]]; then
    must_gather_image="docker.io/scylladb/scylla-operator:${must_gather_image}"
  fi
  ( gather-artifacts-on-exit "${must_gather_image}" ) || true
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

# Endpoints of the upgrade; env-overridable. Each accepts a full image ref (used verbatim) or a tag of the
# released repository, docker.io/scylladb/scylla-operator - e.g. a released version ("1.20.2") or a CI build
# ("1.21-<git-commit-hash>", the exact artifact a release would promote).
# Defaults: the ref upgraded from is the previous minor's highest released patch when the current ref is
# a release branch (vX.Y) or a release(-candidate) tag (vX.Y.Z[-rc.N]), the highest released version otherwise
# (master, feature branches); the ref upgraded to is the checked-out tree (empty ref), built from source.
# The current ref comes from the CI event context - the PR target branch on pull requests, the ref the workflow
# runs on otherwise - because checkouts in CI are detached HEADs, often shallow, so git can't tell.

# operator-upgrade-from-ref prints the highest stable released version (without the leading "v") - among the
# minors strictly below X.Y (the previous minor's highest patch) when the given ref is a release branch "vX.Y"
# or a release(-candidate) tag "vX.Y.Z[-rc.N]", overall otherwise (master, feature branches).
function operator-upgrade-from-ref {
  local ref="${1-}"
  local tags

  # Always list the canonical repository's tags - the local clone may be shallow, a fork without the release
  # tags, or simply stale (cloned before the newest release was tagged).
  tags="$( git ls-remote --tags --refs https://github.com/scylladb/scylla-operator.git | sed -e 's|.*refs/tags/||' | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$' )"

  if [[ "${ref}" =~ ^v([0-9]+)\.([0-9]+)(\.[0-9]+(-rc\.[0-9]+)?)?$ ]]; then
    tags="$( awk -F '[v.]' -v major="${BASH_REMATCH[1]}" -v minor="${BASH_REMATCH[2]}" '$2+0 < major+0 || ($2+0 == major+0 && $3+0 < minor+0)' <<< "${tags}" )"
  fi

  if [ -z "${tags}" ]; then
    echo "Can't determine the released version to upgrade from: no release tag matching ref \"${ref}\"" >&2
    return 1
  fi

  sort -V <<< "${tags}" | tail -n 1 | sed -e 's/^v//'
}

current_ref="${GITHUB_BASE_REF:-${GITHUB_REF_NAME:-$( git -C "${repo_root}" rev-parse --abbrev-ref HEAD )}}"
OPERATOR_UPGRADE_FROM_REF="${OPERATOR_UPGRADE_FROM_REF:-$( operator-upgrade-from-ref "${current_ref}" )}"

# Build the checked-out tree and push it to the kind registry (digest-pinned ref) when no "to" ref is given;
# no-op for a non-empty ref.
build-and-push-operator-image "${repo_root}" OPERATOR_UPGRADE_TO_REF

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
  --operator-upgrade-to-version="${OPERATOR_UPGRADE_TO_REF}"
