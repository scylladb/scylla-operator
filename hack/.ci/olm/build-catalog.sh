#!/bin/bash
#
# Copyright (C) 2025 ScyllaDB
#
# This script builds an OLM catalog for testing purposes from a given OLM bundle image and outputs it to the specified directory.

set -euxEo pipefail
shopt -s inherit_errexit

source "$( dirname "${BASH_SOURCE[0]}" )/../../lib/metadata.sh"

if [[ "$#" -ne 2 ]]; then
    echo -e "Missing arguments.\nUsage: ${0} <bundle_image_ref> <dest_dir>" >&2
    exit 1
fi

bundle_image_ref="${1}"
dest_dir="${2}"

mkdir -p "${dest_dir}" 2>/dev/null
if [[ -n "$( find "${dest_dir}" -mindepth 1 -print -quit )" ]]; then
  echo "Error: dest_dir '${dest_dir}' must be empty" >&2
  exit 1
fi

temp_dir="$( mktemp -d )"
trap "rm -rf ${temp_dir}" EXIT

raw_bundle_path="${temp_dir}/raw.bundle.yaml"
opm render "${bundle_image_ref}" --output=yaml > "${raw_bundle_path}"
schema="$( yq -e '.schema' "${raw_bundle_path}" )"
if [[ "${schema}" != "olm.bundle" ]]; then
  echo "Error: provided image is not an OLM bundle image, expected schema 'olm.bundle', got '${schema}'" >&2
  exit 1
fi

package="$( yq -e '.package' "${raw_bundle_path}" )"
bundle_name="$( yq -e '.name' "${raw_bundle_path}" )"

channel=$( get-metadata ".operatorTests.olm.channel" )

catalog_dir="${dest_dir}/catalog"
mkdir "${catalog_dir}"
catalog_operator_path="${catalog_dir}/operator.yaml"

opm generate dockerfile "${catalog_dir}"
opm init "${package}" \
  --default-channel="${channel}" \
  --output=yaml > "${catalog_operator_path}"

cat << EOF >> "${catalog_operator_path}"
---
schema: olm.channel
package: ${package}
name: ${channel}
entries:
- name: ${bundle_name}
EOF

cat "${raw_bundle_path}" >> "${catalog_operator_path}"

# Sanity check.
opm validate "${catalog_dir}"
