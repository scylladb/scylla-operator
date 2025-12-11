#!/bin/bash
#
# Copyright (C) 2025 ScyllaDB
#
# This script applies post-processing to an OLM bundle located at the specified target directory.

set -euxEo pipefail
shopt -s inherit_errexit

source "$( dirname "${BASH_SOURCE[0]}" )/../lib/olm.sh"

if [[ "$#" -ne 2 ]]; then
  echo -e "Missing arguments.\nUsage: ${0} <target> <operator_image_ref>" >&2
  exit 1
fi

target="${1}"
operator_image_ref="${2}"

# Update the bundle's scylla-operator deployment spec to use the specified operator image reference as it can't be done in build time.
operator_image_ref="${operator_image_ref}" yq -e -i '
  (.spec.install.spec.deployments[]
    | select(.name == "scylla-operator").spec.template.spec.containers[]
    | select(.name == "scylla-operator")
  ) |= with(
    .;
    .image = env(operator_image_ref) |
    .env[] |= select(.name == "SCYLLA_OPERATOR_IMAGE").value = env(operator_image_ref)
  )
' "${target}/manifests/scylladb-operator.clusterserviceversion.yaml"

# Update the bundle's webhook-server deployment spec to use the specified operator image reference as it can't be done in build time.
operator_image_ref="${operator_image_ref}" yq -e -i '
  (.spec.install.spec.deployments[]
    | select(.name == "webhook-server").spec.template.spec.containers[]
    | select(.name == "webhook-server")
  ).image = env(operator_image_ref)
' "${target}/manifests/scylladb-operator.clusterserviceversion.yaml"

# Update the CSV annotations with the creation timestamp and operator image reference.
# Note: this is only required for publishing.

set_yaml_field_without_line_comment \
  "${target}/manifests/scylladb-operator.clusterserviceversion.yaml" \
  ".metadata.annotations.createdAt" \
  "$( date -u '+%Y-%m-%dT%H:%M:%S' )"

set_yaml_field_without_line_comment \
  "${target}/manifests/scylladb-operator.clusterserviceversion.yaml" \
  ".metadata.annotations.containerImage" \
  "${operator_image_ref}"
