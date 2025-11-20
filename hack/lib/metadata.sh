#!/bin/bash

set -euExo pipefail
shopt -s inherit_errexit

readonly metadata_file="assets/metadata/metadata.yaml"

# get-metadata retrieves a value from the metadata YAML file.
# Usage: get-metadata <yaml-key>
function get-metadata() {
    local key="${1}"
    yq -e "$key" "${metadata_file}" || {
        echo "Failed to get key ${key} from ${metadata_file}" >&2
        exit 1
    }
}
