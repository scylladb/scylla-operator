#!/bin/bash
#
# Copyright (C) 2025 ScyllaDB
#
# This script builds the Prometheus Operator manifest with a specified version and namespace following the upstream instructions.
# https://github.com/prometheus-operator/prometheus-operator/blob/8337f851ab7550fb7f914461e810a722e4312e17/Documentation/getting-started/installation.md?plain=1#L28

set -euxEo pipefail
shopt -s inherit_errexit

source "$( dirname "${BASH_SOURCE[0]}" )/../lib/assets.sh"

if [[ -n "${1+x}" ]]; then
    target="${1}"
else
    echo "Missing target file.\nUsage: ${0} <target>" >&2 >/dev/null
    exit 1
fi


function get-url() {
    local version="${1}"
    local file="${2}"
    echo "https://raw.githubusercontent.com/prometheus-operator/prometheus-operator/refs/tags/v${version}/${file}"
}

version=$( get-config ".thirdParty.prometheusOperator.version" )
namespace=$( get-config ".thirdParty.prometheusOperator.namespace" )

tmp_dir=$( mktemp -d )
trap 'rm -rf "${tmp_dir}"' EXIT

for file in 'bundle.yaml' 'kustomization.yaml'; do
    url="$( get-url "${version}" "${file}" )"
    curl --fail --retry 5 --retry-all-errors -L "${url}" -o "${tmp_dir}/${file}"
done

# Modify kustomization.yaml to include namespace and set namespace
cat > "$tmp_dir/namespace.yaml" << EOF
apiVersion: v1
kind: Namespace
metadata:
  name: "${namespace}"
EOF

(
  cd "${tmp_dir}" &&
  kustomize edit add resource namespace.yaml &&
  kustomize edit set namespace "${namespace}"
)

kustomize build "${tmp_dir}" > "${target}"
