#!/bin/bash
#
# Copyright (C) 2025 ScyllaDB

set -euExo pipefail
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
