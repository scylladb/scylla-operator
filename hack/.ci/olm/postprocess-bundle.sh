#!/bin/bash
#
# Copyright (C) 2025 ScyllaDB
#
# This script applies post-processing required for a release to an OLM bundle located at the specified target directory.

set -euxEo pipefail
shopt -s inherit_errexit

# set_yaml_field_without_line_comment sets a YAML field to a value without a line comment in the specified YAML file.
# $1 - target file
# $2 - field path (e.g., .spec.version)
# $3 - value
function set_yaml_field_without_line_comment() {
    local target_file="${1}"
    local field_path="${2}"
    local value="${3}"

    field_path="${field_path}" value="${value}" yq -e -i '
      (. as $in | eval(strenv(field_path))) = env(value) |
      (eval(strenv(field_path))) line_comment= ""
    ' "${target_file}"
}

readonly script_dir="$( realpath $( dirname "${BASH_SOURCE[0]}" ) )"
source "${script_dir}/../../lib/semver.sh"

readonly semver_pattern='^([0-9]+)\.([0-9]+)\.([0-9]+)(-([0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*))?(\+([0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*))?$'

if [[ "$#" -ne 3 ]]; then
  echo -e "Missing arguments.\nUsage: ${0} <target_bundle_dir> <version> <channel>" >&2
  exit 1
fi

target_bundle_dir="${1}"
version="${2}"
channel="${3}"

if [[ ! "${version}" =~ ${semver_pattern} ]]; then
  echo "Error: Version '${version}' is not semver compliant"
  exit 1
fi

major="${BASH_REMATCH[1]}"
minor="${BASH_REMATCH[2]}"
patch="${BASH_REMATCH[3]}"
prerelease="${BASH_REMATCH[5]:-}"

case "${channel}" in
  dev)
    ;;
  stable)
    # Cross-validate version for stable channel.
    if [[ "${major}" -ne 1 ]]; then
      echo "Error: Version '${version}' has unsupported major version '${major}', only major version '1' is supported" >&2
      exit 1
    fi

    if [[ "${minor}" -eq 0 ]]; then
      echo "Error: Version '${version}' has unsupported minor version '0', only minor versions > 0 are supported" >&2
      exit 1
    fi

    if [[ -n "${prerelease}" ]]; then
      echo "Error: stable channel must not contain prerelease versions" >&2
      exit 1
    fi
    ;;
  *)
    echo "Error: Channel '${channel}' must be either 'dev' or 'stable'" >&2
    exit 1
    ;;
esac

OPERATOR_IMAGE_REF="${OPERATOR_IMAGE_REF:-docker.io/scylladb/scylla-operator:${version}}"
skopeo inspect docker://${OPERATOR_IMAGE_REF} 2>&1 >/dev/null || (echo "Error: operator_image_ref '${OPERATOR_IMAGE_REF}' is not a valid image reference or is not accessible" >&2 && exit 1)

ARTIFACTS=${ARTIFACTS:-$( mktemp -d )}
OPERATOR_MANIFEST_TOOLS_ARTIFACTS="${ARTIFACTS}/operator-manifest-tools"
mkdir "${OPERATOR_MANIFEST_TOOLS_ARTIFACTS}"

# Update the bundle's scylla-operator deployment spec to use the specified operator image reference as it can't be done in build time.
operator_image_ref="${OPERATOR_IMAGE_REF}" yq -e -i '
  (.spec.install.spec.deployments[]
    | select(.name == "scylla-operator").spec.template.spec.containers[]
    | select(.name == "scylla-operator")
  ) |= with(
    .;
    .image = env(operator_image_ref) |
    .env[] |= select(.name == "SCYLLA_OPERATOR_IMAGE").value = env(operator_image_ref)
  )
' "${target_bundle_dir}/manifests/scylladb-operator.clusterserviceversion.yaml"

# Update the bundle's webhook-server deployment spec to use the specified operator image reference as it can't be done in build time.
operator_image_ref="${OPERATOR_IMAGE_REF}" yq -e -i '
  (.spec.install.spec.deployments[]
    | select(.name == "webhook-server").spec.template.spec.containers[]
    | select(.name == "webhook-server")
  ).image = env(operator_image_ref)
' "${target_bundle_dir}/manifests/scylladb-operator.clusterserviceversion.yaml"

# Update the CSV annotations with the creation timestamp and operator image reference.

set_yaml_field_without_line_comment \
  "${target_bundle_dir}/manifests/scylladb-operator.clusterserviceversion.yaml" \
  ".metadata.annotations.createdAt" \
  "$( date -u '+%Y-%m-%dT%H:%M:%S' )"

set_yaml_field_without_line_comment \
  "${target_bundle_dir}/manifests/scylladb-operator.clusterserviceversion.yaml" \
  ".metadata.annotations.containerImage" \
  "${OPERATOR_IMAGE_REF}"

# Update the CSV's name to reflect the version.
set_yaml_field_without_line_comment \
  "${target_bundle_dir}/manifests/scylladb-operator.clusterserviceversion.yaml" \
  ".metadata.name" \
  "scylladb-operator.v${version}"

# Update the CSV's version.
set_yaml_field_without_line_comment \
  "${target_bundle_dir}/manifests/scylladb-operator.clusterserviceversion.yaml" \
  ".spec.version" \
  "${version}"

# Update the channel in the metadata.
set_yaml_field_without_line_comment \
  "${target_bundle_dir}/metadata/annotations.yaml" \
  '.annotations."operators.operatorframework.io.bundle.channels.v1"' \
  "${channel}"

if [[ "${channel}" == "stable" ]]; then
  previous_version=""

  tag="v${version}"
  tags=$( git tag -l | grep -E '^v[0-9]+\.[0-9]+\.[0-9]$' )
  previous_tag="$( find_previous_semver "${tags}" "${tag}" )"
  if [[ ! -n "${previous_tag}" ]]; then
    echo "Error: Cannot find a previous version tag for version '${version}'" >&2
    exit 1
  fi
  previous_version="${previous_tag/v/}"

  # Always set to ">=X.(Y-1).0 <${version}".
  skip_range=">=1.$((minor - 1)).0 <${version}"

  previous_version="${previous_version}" yq -e -i '
    .spec.replaces = "scylladb-operator.v" + env(previous_version)
  ' "${target_bundle_dir}/manifests/scylladb-operator.clusterserviceversion.yaml"

  skip_range="${skip_range}" yq -e -i '
    .metadata.annotations."olm.skipRange" = strenv(skip_range)
  ' "${target_bundle_dir}/manifests/scylladb-operator.clusterserviceversion.yaml"
else
  yq -e -i 'del(.spec.replaces)' "${target_bundle_dir}/manifests/scylladb-operator.clusterserviceversion.yaml" 2>/dev/null || true
  yq -e -i 'del(.metadata.annotations."olm.skipRange")' "${target_bundle_dir}/manifests/scylladb-operator.clusterserviceversion.yaml" 2>/dev/null || true
fi

# Pin digests in the manifests.
operator-manifest-tools pinning pin \
  --resolver=skopeo \
  --output-extract="${OPERATOR_MANIFEST_TOOLS_ARTIFACTS}/references.json" \
  --output-replace="${OPERATOR_MANIFEST_TOOLS_ARTIFACTS}/replacements.json" \
  "${target_bundle_dir}/manifests/"
