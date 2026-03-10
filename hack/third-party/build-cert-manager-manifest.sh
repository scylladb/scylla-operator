#!/usr/bin/env bash
#
# Copyright (C) 2026 ScyllaDB
#
# This script builds the cert-manager manifest with a specified version following the upstream instructions.
# https://cert-manager.io/docs/installation/kubectl/

set -euxEo pipefail
shopt -s inherit_errexit

source "$( dirname "${BASH_SOURCE[0]}" )/../lib/assets.sh"

if [[ -n "${1+x}" ]]; then
    target="${1}"
else
    printf 'Missing target file.\nUsage: %s <target>\n' "$0" >&2
    exit 1
fi

version=$( get-config ".thirdParty.certManager.version" )

url="https://github.com/cert-manager/cert-manager/releases/download/v${version}/cert-manager.yaml"
curl --fail --retry 5 --retry-all-errors -L "${url}" -o "${target}"
