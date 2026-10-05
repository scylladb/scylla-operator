#!/usr/bin/env bash
#
# Copyright (C) 2026 ScyllaDB
#
# This script performs the steps necessary to certify the OLM bundle for a given ScyllaDB Operator tag.
# It postprocesses the OLM bundle, creates a new branch in the target repository (fork of certified-operators),
# and triggers the certification pipeline. A version that's already in certified-operators or has an open PR from the
# bot is left unchanged.
#
# Usage: certify-bundle.sh <tag> <ssh_scylladb_operator_cd_bot_key_path>
# The working directory must be a checkout of the repository at <tag>, which the bundle is taken from.
# Environment:
#   KUBECONFIG          required, kubeconfig of the cluster the certification pipeline runs in
#   GH_TOKEN            required by gh unless it's logged in otherwise
#   SUBMIT              optional, "true" opens the PR in certified-operators, defaults to "false"
#   TARGET_BRANCH       optional, branch to push to in the fork, defaults to scylladb-operator-<version>
#   OPERATOR_IMAGE_REF  optional, passed through to postprocess-bundle.sh
# Exits 0 without changes for an existing submission, otherwise with the status of the pipelinerun.

set -euExo pipefail
shopt -s inherit_errexit

readonly script_dir="$( realpath $( dirname "${BASH_SOURCE[0]}" ) )"
source "${script_dir}/../../lib/assets.sh"

readonly semver_pattern='^([0-9]+)\.([0-9]+)\.([0-9]+)(-([0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*))?(\+([0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*))?$'

temp_dir="$( mktemp -d )"
trap "rm -rf ${temp_dir}" EXIT

if [[ "$#" -ne 2 ]]; then
  echo -e "Invalid arguments.\nUsage: ${0} <tag> <ssh_scylladb_operator_cd_bot_key_path>" >&2
  exit 1
fi

tag="${1}"
ssh_scylladb_operator_cd_bot_key_path="${2}"

# Validate the arguments.

# The bundle comes from the checkout in the working directory, which must be at the tag. The script itself may
# run from a different checkout so that a fixed script can certify an existing tag.
current_hash=$(git rev-parse HEAD)
if [[ "${current_hash}" != "$(git rev-parse "${tag}"^{commit})" ]]; then
  echo "Error: The current commit hash '${current_hash}' does not match the tag '${tag}'" >&2
  exit 1
fi
release_root="$( git rev-parse --show-toplevel )"

# Determine the version and channel based on the provided tag.

version="${tag#v}"
if [[ ! "${version}" =~ ${semver_pattern} ]]; then
  echo "Error: Version '${version}' derived from tag '${tag}' is not semver compliant" >&2
  exit 1
fi

channel="stable"
# Use "dev" channel for prereleases.
if [[ -n "${BASH_REMATCH[4]}" ]]; then
  channel="dev"
fi

# Ensure required environment variables are set.

# KUBECONFIG must be set to point to the kubeconfig file of the cluster where the certification pipeline runs.
if [[ -z "${KUBECONFIG+x}" ]]; then
  echo "Error: KUBECONFIG environment variable must be set" >&2
  exit 1
fi

# Set defaults for optional environment variables.

OPERATOR_IMAGE_REF="${OPERATOR_IMAGE_REF:-}"
export OPERATOR_IMAGE_REF

SUBMIT=${SUBMIT:-false}
TARGET_BRANCH="${TARGET_BRANCH:-scylladb-operator-${version}}"
# The branch is based on upstream main and pushed to the fork, so targeting main would move the fork's main onto a
# bundle commit.
if [[ "${TARGET_BRANCH}" == "main" ]]; then
  echo "Error: TARGET_BRANCH must not be 'main'" >&2
  exit 1
fi

# Leave an existing submission unchanged, since the PR is only opened once the pipeline passes and the branch is
# force-pushed. To replace the bundle of an open PR, close it and run the certification again.

certified_operators_repo=redhat-openshift-ecosystem/certified-operators

# Listing the directory keeps a missing version apart from a failed request, which a 404 wouldn't.
upstream_version="$( gh api "repos/${certified_operators_repo}/contents/operators/scylladb-operator" --jq ".[] | select(.name == \"${version}\") | .name" )"
if [[ -n "${upstream_version}" ]]; then
  echo "::notice::Version ${version} is already in ${certified_operators_repo}, leaving it unchanged."
  exit 0
fi

# --head matches the branch name only, so the PR is also filtered by the fork it comes from.
pr_url="$( gh pr list --repo "${certified_operators_repo}" --head "scylladb-operator-${version}" --state open \
  --json url,headRepositoryOwner -q '.[] | select(.headRepositoryOwner.login == "scylladb-operator-cd-bot") | .url' )"
if [[ -n "${pr_url}" ]]; then
  echo "::notice::Version ${version} is already submitted in ${pr_url}, leaving it unchanged."
  exit 0
fi

# Clone upstream certified-operators. The branch is pushed to the fork, which shares upstream's objects.

target_repo=git@github.com:scylladb-operator-cd-bot/certified-operators.git

# A fresh runner has no known_hosts, so accept-new would trust whatever answers for github.com. Pin the published key
# instead: https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/githubs-ssh-key-fingerprints
known_hosts_file="${temp_dir}/known_hosts"
echo "github.com ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIOMqqnkVzrm0SdG6UOoqKLsabgH5C9okWi0dh2l9GKJl" > "${known_hosts_file}"
export GIT_SSH_COMMAND="ssh -o IdentitiesOnly=yes -i '${ssh_scylladb_operator_cd_bot_key_path}' -o UserKnownHostsFile='${known_hosts_file}' -o StrictHostKeyChecking=yes"

repo_target_dir="${temp_dir}/certified-operators"
git clone --depth 1 --branch main https://github.com/redhat-openshift-ecosystem/certified-operators.git "${repo_target_dir}"

parent_target_dir="${repo_target_dir}/operators/scylladb-operator"
mkdir -p "${parent_target_dir}"

# Create the ci.yaml file with the certification project ID.
cert_project_id=$( get-yaml-value "${release_root}/assets/metadata/metadata.yaml" ".operator.redHatCertificationProjectID" )
cat <<EOF > "${parent_target_dir}/ci.yaml"
cert_project_id: "${cert_project_id}"
EOF

target_dir="${parent_target_dir}/${version}"
mkdir "${target_dir}"

cp -r "${release_root}/bundle/"{manifests,metadata} "${target_dir}/"

# Postprocess the bundle.
"${script_dir}/postprocess-bundle.sh" "${target_dir}" "${version}" "${channel}"

# Create a new branch in the target repository and push the changes.

(
  cd "${repo_target_dir}"
  git config user.name "ScyllaDB Operator Continuous Delivery Bot"
  git config user.email "251034767+scylladb-operator-cd-bot@users.noreply.github.com"
  git add .
  # https://github.com/redhat-openshift-ecosystem/certification-releases/blob/main/4.9/ga/troubleshooting.md#pull-request-title
  git commit -s -am "operator scylladb-operator (${version})"
  # The branch is rebuilt from upstream main on every run, so a retry of a failed run has to replace it.
  git push --force "${target_repo}" "HEAD:refs/heads/${TARGET_BRANCH}"
)

# Create a volume claim template file for the pipeline workspace.
workspace_volume_claim_template_file="${temp_dir}/workspace-volume-claim-template.yaml"
touch "${workspace_volume_claim_template_file}"

cat <<EOF > "${workspace_volume_claim_template_file}"
spec:
  accessModes:
  - ReadWriteOnce
  resources:
    requests:
      storage: 5Gi
EOF

# Execute the certification pipeline.

tkn pipeline start operator-ci-pipeline \
  --kubeconfig="${KUBECONFIG}" \
  --namespace=oco \
  --workspace name=pipeline,volumeClaimTemplateFile="${workspace_volume_claim_template_file}" \
  --param git_repo_url=https://github.com/scylladb-operator-cd-bot/certified-operators \
  --param git_branch="${TARGET_BRANCH}" \
  --param bundle_path="operators/scylladb-operator/${version}" \
  --param env=prod \
  --param pin_digests=true \
  --param upstream_repo_name=redhat-openshift-ecosystem/certified-operators \
  --param submit="${SUBMIT}" \
  --use-param-defaults \
  --showlog \
  --exit-with-pipelinerun-error
