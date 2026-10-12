#!/usr/bin/env bash
#
# Copyright (C) 2026 ScyllaDB
#
# This script decides which rolling image tags a version gets and whether its release is the latest. `X.Y` follows the
# highest GA patch of its minor and `latest` the highest GA overall, so a patch of an older minor can't move `latest`
# backwards. Prereleases get neither. It prints the results as key=value lines.

set -euExo pipefail
shopt -s inherit_errexit

if [[ "$#" -ne 4 ]]; then
  echo -e "Invalid arguments.\nUsage: ${0} <repository> <tag> <minor> <prerelease>" >&2
  exit 1
fi

repository="${1}"
tag="${2}"
minor="${3}"
prerelease="${4}"

rolling_tags=()
latest=false
if [[ "${prerelease}" == "false" ]]; then
  released_tags="$( git ls-remote --tags --refs "https://github.com/${repository}.git" 'v*' | sed -e 's#.*refs/tags/##' )"
  ga_tags="$( { grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$' <<< "${released_tags}" || true; echo "${tag}"; } | sort --version-sort --unique )"
  if [[ "$( grep -F "v${minor}." <<< "${ga_tags}" | tail -n 1 )" == "${tag}" ]]; then
    rolling_tags+=( "${minor}" )
  fi
  if [[ "$( tail -n 1 <<< "${ga_tags}" )" == "${tag}" ]]; then
    rolling_tags+=( "latest" )
    latest=true
  fi
fi

echo "rolling-tags=${rolling_tags[*]}"
echo "latest=${latest}"
