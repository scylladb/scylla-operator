#!/bin/bash
#
# Copyright (C) 2025 ScyllaDB
#
# This script applies post-processing required for a release to an OLM bundle located at the specified target directory.

set -euxEo pipefail
shopt -s inherit_errexit

source "$( dirname "${BASH_SOURCE[0]}" )/../lib/olm.sh"

semver_pattern='^([0-9]+)\.([0-9]+)\.([0-9]+)(-([0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*))?(\+([0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*))?$'

if [[ "$#" -ne 2 ]]; then
  echo -e "Missing arguments.\nUsage: ${0} <target> <version>" >&2
  exit 1
fi

target="${1}"
version="${2}"

if [[ ! "${version}" =~ ${semver_pattern} ]]; then
  echo "Error: Version '${version}' is not semver compliant"
  exit 1
fi

OPERATOR_IMAGE_REF="${OPERATOR_IMAGE_REF:-docker.io/scylladb/scylla-operator:${version}}"

"$(dirname "${BASH_SOURCE[0]}")/postprocess-bundle.sh" "${target}" "${OPERATOR_IMAGE_REF}"

# Update the CSV's name to reflect the version.
set_yaml_field_without_line_comment \
  "${target}/manifests/scylladb-operator.clusterserviceversion.yaml" \
  ".metadata.name" \
  "scylladb-operator.v${version}"

# Update the CSV's version.
set_yaml_field_without_line_comment \
  "${target}/manifests/scylladb-operator.clusterserviceversion.yaml" \
  ".spec.version" \
  "${version}"
