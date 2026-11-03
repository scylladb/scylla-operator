#!/usr/bin/env bash
#
# Copyright (C) 2026 ScyllaDB
#
# This script validates a version to release and prints its tag, minor and whether it's a prerelease as key=value lines.

set -euExo pipefail
shopt -s inherit_errexit

if [[ "$#" -ne 1 ]]; then
  echo -e "Invalid arguments.\nUsage: ${0} <version>" >&2
  exit 1
fi

version="${1}"

readonly semver_pattern='^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?$'
if [[ ! "${version}" =~ ${semver_pattern} ]]; then
  echo "Error: Version '${version}' is not MAJOR.MINOR.PATCH or MAJOR.MINOR.PATCH-PRERELEASE." >&2
  exit 1
fi
prerelease="$( [[ -n "${BASH_REMATCH[1]}" ]] && echo true || echo false )"

echo "tag=v${version}"
echo "minor=$( cut -d . -f 1,2 <<< "${version}" )"
echo "prerelease=${prerelease}"
