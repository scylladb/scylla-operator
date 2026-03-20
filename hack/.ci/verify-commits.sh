#!/usr/bin/env bash
#
# Copyright (C) 2026 ScyllaDB
#
# Verifies that commit messages in the given range follow the Conventional Commits format.
# Usage: verify-commits.sh <base-sha>
#   base-sha: the commit (exclusive) from which to start checking (e.g. PULL_BASE_SHA in Prow).

set -euEo pipefail
shopt -s inherit_errexit

if [[ "${#}" -ne 1 ]]; then
  echo "Usage: ${0} <base-sha>" >&2
  exit 1
fi

base_sha="${1}"

command -v git >/dev/null || { echo "git is not available" >&2; exit 1; }

rc=0
for c in $( git rev-list --no-merges "${base_sha}..HEAD" ); do
  m=$( git log -1 --pretty=%s "${c}" )
  if [[ ${m} =~ ^(build|chore|ci|docs|feat|fix|perf|refactor|revert|style|test)(\(.*\))?(!)?:\ .*$ ]]; then
    echo "[OK]   ${m}"
  else
    echo "[FAIL] ${m}"
    rc=1
  fi
done

exit "${rc}"
