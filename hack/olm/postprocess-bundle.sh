#!/bin/bash
#
# Copyright (C) 2025 ScyllaDB
#
# This script applies post-processing to an OLM bundle located at the specified target directory.

set -euxEo pipefail
shopt -s inherit_errexit

if [[ "$#" -ne 2 ]]; then
    echo -e "Missing arguments.\nUsage: ${0} <target> <operator_image_ref>" >&2
    exit 1
fi

target="${1}"
operator_image_ref="${2}"

# set_metadata_annotation_without_line_comment sets a metadata annotation without a line comment in the specified YAML file.
# $1 - target file
# $2 - annotation key
# $3 - annotation value
function set_metadata_annotation_without_line_comment() {
    local target_file="${1}"
    local key="${2}"
    local value="${3}"

    key="${key}" value="${value}" yq -e -i '
      .metadata.annotations[env(key)] = env(value) |
      .metadata.annotations[env(key)] line_comment= ""
    ' "${target_file}"
}

# Update all YAML manifests in the bundle to use the specified operator image reference as it can't be done in build time.
find "${target}/manifests" -name "*.yaml" -exec sed -i "s|docker\.io/scylladb/scylla-operator\([:@][^[:space:]]*\)\?|${operator_image_ref}|g" {} +

# Update the CSV annotations with the creation timestamp and operator image reference.
# Note: this is only required for publishing.

set_metadata_annotation_without_line_comment \
    "${target}/manifests/scylladb-operator.clusterserviceversion.yaml" \
    "createdAt" \
    "$( date -u '+%Y-%m-%dT%H:%M:%S' )"

set_metadata_annotation_without_line_comment \
    "${target}/manifests/scylladb-operator.clusterserviceversion.yaml" \
    "containerImage" \
    "${operator_image_ref}"
