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

types=(build chore ci docs feat fix perf refactor revert style test)
types_re="$( IFS='|'; echo "${types[*]}" )"

rc=0
while read -r c; do
  [[ -n "${c}" ]] || continue

  m="$( git log -1 --pretty=%s "${c}" )"
  if [[ "${m}" =~ ^(${types_re})(\(.*\))?(!)?:\ .*$ ]]; then
    echo "[OK]   ${m}"
    continue
  fi

  echo "[FAIL] ${m}"
  echo "       Commit $( git log -1 --pretty=%h "${c}" ) doesn't follow the Conventional Commits format."
  echo "       Expected '<type>[optional scope][!]: <description>', e.g. 'fix(scyllacluster): don't requeue on a missing Service'."
  echo "       Allowed types: ${types[*]}."
  echo "       Amend it with 'git commit --amend' or rewrite the history with 'git rebase -i ${base_sha}'."
  rc=1
done <<<"$( git rev-list --no-merges --reverse "${base_sha}..HEAD" )"

if [[ "${rc}" -ne 0 ]]; then
  echo
  echo "Commit policy violations found. See the 'Commits and PRs' section of CONTRIBUTING.md for the full policy." >&2
fi

exit "${rc}"
