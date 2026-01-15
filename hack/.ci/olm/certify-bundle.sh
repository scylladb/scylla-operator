#!/usr/bin/env bash
#
# Copyright (C) 2026 ScyllaDB
#
# This script performs the steps necessary to certify the OLM bundle for a given ScyllaDB Operator tag.
# It postprocesses the OLM bundle, creates a new branch in the target repository (fork of certified-operators),
# and triggers the certification pipeline.

set -euExo pipefail
shopt -s inherit_errexit

readonly script_dir="$( realpath $( dirname "${BASH_SOURCE[0]}" ) )"
source "${script_dir}/../../lib/metadata.sh"

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

current_hash=$(git rev-parse HEAD)
if [[ "${current_hash}" != "$(git rev-parse "${tag}"^{commit})" ]]; then
  echo "Error: The current commit hash '${current_hash}' does not match the tag '${tag}'" >&2
  exit 1
fi

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

# Clone the target repository (fork of certified-operators).

target_repo=git@github.com:scylladb-operator-cd-bot/certified-operators.git

repo_target_dir="${temp_dir}/certified-operators"
GIT_SSH_COMMAND="ssh -o IdentitiesOnly=yes -i '${ssh_scylladb_operator_cd_bot_key_path}' -o StrictHostKeyChecking=accept-new" \
  git clone --depth 1 --branch main "${target_repo}" "${repo_target_dir}"

parent_target_dir="${repo_target_dir}/operators/scylladb-operator"
mkdir -p "${parent_target_dir}"

# Create the ci.yaml file with the certification project ID.
cert_project_id=$( get-metadata ".operator.redHatCertificationProjectID" )
cat <<EOF > "${parent_target_dir}/ci.yaml"
cert_project_id: "${cert_project_id}"
EOF

target_dir="${parent_target_dir}/${version}"
mkdir "${target_dir}"

cp -r "${script_dir}/../../../bundle/"{manifests,metadata} "${target_dir}/"

# Postprocess the bundle.
"${script_dir}/postprocess-bundle.sh" "${target_dir}" "${version}" "${channel}"

# Create a new branch in the target repository and push the changes.

(
  cd "${repo_target_dir}"
  git config user.name "ScyllaDB Operator Continuous Delivery Bot"
  git config user.email "251034767+scylladb-operator-cd-bot@users.noreply.github.com"
  git checkout -B "${TARGET_BRANCH}"
  git add .
  # https://github.com/redhat-openshift-ecosystem/certification-releases/blob/main/4.9/ga/troubleshooting.md#pull-request-title
  git commit -s -am "operator scylladb-operator (${version})"
  GIT_SSH_COMMAND="ssh -o IdentitiesOnly=yes -i ${ssh_scylladb_operator_cd_bot_key_path}" git push origin "${TARGET_BRANCH}"
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
