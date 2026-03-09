#!/usr/bin/env bash

set -euExo pipefail
shopt -s inherit_errexit

readonly config_file="assets/config/config.yaml"
readonly metadata_file="assets/metadata/metadata.yaml"

# get-yaml-value retrieves a value from a YAML file.
# Usage: get-yaml-value <file-path> <yaml-key>
function get-yaml-value() {
    local file="${1}"
    local key="${2}"
    yq -e "$key" "${file}" || {
        echo "Failed to get key ${key} from ${file}" >&2
        exit 1
    }
}

# get-metadata retrieves a value from the metadata YAML file.
# Usage: get-metadata <yaml-key>
function get-metadata() {
    get-yaml-value "${metadata_file}" "${1}"
}

# get-config retrieves a value from the config YAML file.
# Usage: get-config <yaml-key>
function get-config() {
    get-yaml-value "${config_file}" "${1}"
}
