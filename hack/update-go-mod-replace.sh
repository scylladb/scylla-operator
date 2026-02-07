#!/bin/bash

set -euxEo pipefail
shopt -s inherit_errexit

# This script updates the 'replace' directives in go.mod for dependencies that we want to keep in sync with their versions
# defined in `assets/metadata/metadata.yaml`.

source "$( dirname "${BASH_SOURCE[0]}" )/lib/assets.sh"

function update_replace_directive() {
    local module=$1
    local version=$2

    go mod edit -replace=${module}=${module}@${version}
}

# Update prometheus-operator modules.
PROMETHEUS_OPERATOR_VERSION="v$( get-config ".thirdParty.prometheusOperator.version" )"
update_replace_directive "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring" "${PROMETHEUS_OPERATOR_VERSION}"
update_replace_directive "github.com/prometheus-operator/prometheus-operator/pkg/client" "${PROMETHEUS_OPERATOR_VERSION}"

go mod tidy && go mod vendor
