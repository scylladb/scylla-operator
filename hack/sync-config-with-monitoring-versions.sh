#!/usr/bin/env bash
#
# Copyright (C) 2026 ScyllaDB
#
# This script synchronizes version fields in assets/config/config.yaml with versions
# defined in the scylla-monitoring submodule's versions.sh file.

set -euxEo pipefail
shopt -s inherit_errexit

readonly script_dir="$( realpath $( dirname "${BASH_SOURCE[0]}" ) )"
readonly repo_root="${script_dir}/../"

readonly grafana_image_repo="docker.io/grafana/grafana"
readonly permissive_semver_pattern='^v?([0-9]+)\.([0-9]+)\.([0-9]+)(-([0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*))?(\+([0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*))?$'
readonly scylla_monitoring_versions_path="${repo_root}/submodules/github.com/scylladb/scylla-monitoring/versions.sh"

source "${script_dir}/lib/assets.sh"

if [[ "$#" -ne 1 ]]; then
  echo -e "Invalid arguments.\nUsage: ${0} <target_config_file>" >&2
  exit 1
fi

target_config_file="${1}"

if [[ ! -f "${scylla_monitoring_versions_path}" ]]; then
  echo "Error: versions.sh not found at ${scylla_monitoring_versions_path}" >&2
  exit 1
fi

# Extract version variables from versions.sh.
prometheus_version=$(sed -n 's/^PROMETHEUS_VERSION=//p' "${scylla_monitoring_versions_path}")
grafana_version=$(sed -n 's/^GRAFANA_VERSION="\(.*\)"/\1/p' "${scylla_monitoring_versions_path}")

if [[ ! "${prometheus_version}" =~ ${permissive_semver_pattern} ]]; then
  echo "Error: PROMETHEUS_VERSION '${prometheus_version}' is not semver compliant" >&2
  exit 1
fi

if [[ ! "${grafana_version}" =~ ${permissive_semver_pattern} ]]; then
  echo "Error: GRAFANA_VERSION '${grafana_version}' is not semver compliant" >&2
  exit 1
fi

prometheus_version="${prometheus_version}" yq -e -i '
  .operator.prometheusVersion = env(prometheus_version) | .operator.prometheusVersion line_comment = "Generated from scylla-monitoring/versions.sh PROMETHEUS_VERSION."
' "${target_config_file}"

grafana_image="${grafana_image_repo}:${grafana_version}" yq -e -i '
  .operator.grafanaImage = env(grafana_image) | .operator.grafanaImage line_comment = "Generated from scylla-monitoring/versions.sh GRAFANA_VERSION."
' "${target_config_file}"
