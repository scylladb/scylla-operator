#!/usr/bin/env bash
#
# Copyright (C) 2026 ScyllaDB
#
# This script packages the Helm charts as the given version and publishes them to a chart repository served from GCS.
# A published chart version must never change, so a version the repository index already lists is left as it is. It
# packages the charts of the repository in the working directory.

set -euExo pipefail
shopt -s inherit_errexit

if [[ "$#" -ne 4 ]]; then
  echo -e "Invalid arguments.\nUsage: ${0} <bucket_url> <repository_url> <chart_version> <app_version>" >&2
  exit 1
fi

bucket_url="${1}"
repository_url="${2}"
chart_version="${3}"
app_version="${4}"

temp_dir="$( mktemp -d )"
trap 'rm -rf "${temp_dir}"' EXIT

build_dir="${temp_dir}/charts"
mkdir "${build_dir}"
make helm-build --warn-undefined-variables HELM_BUILD_DIR="${build_dir}" HELM_CHART_VERSION="${chart_version}" HELM_APP_VERSION="${app_version}"

# Only the index is downloaded. The charts it lists are left in the bucket untouched, and the index refers to them by
# absolute URLs.
gcloud storage cp "${bucket_url}/index.yaml" "${temp_dir}/index.yaml"

# A redispatch can repackage a version with another Helm, e.g. after a runner image update, which gives different
# bytes, so a version the index already lists is left as it is.
chart_files=( "${build_dir}"/*.tgz )
listed=0
for chart_file in "${chart_files[@]}"; do
  chart="$( basename "${chart_file}" "-${chart_version}.tgz" )"
  entry="$( CHART="${chart}" VERSION="${chart_version}" yq '.entries[strenv(CHART)][] | select(.version == strenv(VERSION))' "${temp_dir}/index.yaml" )"
  if [[ -n "${entry}" ]]; then
    listed=$(( listed + 1 ))
  fi
done

if [[ "${listed}" -eq "${#chart_files[@]}" ]]; then
  echo "::notice::Charts '${chart_version}' are already published, leaving them unchanged."
  exit 0
fi
# The index is written in one upload, so it lists either all of a version's charts or none of them.
if [[ "${listed}" -ne 0 ]]; then
  echo "Error: The index lists only some of the charts '${chart_version}'." >&2
  exit 1
fi

# Entries already in the index are kept as they are, the new charts are added.
helm repo index "${build_dir}" --url "${repository_url}" --merge "${temp_dir}/index.yaml"

# The index goes last, so it never lists a chart that isn't uploaded yet. The content types are set explicitly rather
# than guessed from the extensions.
gcloud storage cp --content-type=application/gzip "${chart_files[@]}" "${bucket_url}/"
gcloud storage cp --content-type=application/yaml "${build_dir}/index.yaml" "${bucket_url}/index.yaml"
