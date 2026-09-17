#!/usr/bin/env bash
#
# Copyright (C) 2026 ScyllaDB
#
# Verifies that the commits in the given range follow the repository's commit policy:
# no merge commits, no leftover autosquash or revert commits, Conventional Commits messages, and no
# issue closing keywords.
# Usage: verify-commits.sh <base-sha>
#   base-sha: the commit (exclusive) from which to start checking (e.g. PULL_BASE_SHA in Prow).

set -euEo pipefail
shopt -s inherit_errexit

if [[ "${#}" -ne 1 ]]; then
  echo "Usage: ${0} <base-sha>" >&2
  exit 1
fi

command -v git >/dev/null || { echo "git is not available" >&2; exit 1; }

base_sha="$( git merge-base "${1}" HEAD )"

types=(build chore ci docs feat fix perf refactor revert style test)
types_re="$( IFS='|'; echo "${types[*]}" )"

# Issue closing keywords, see CONTRIBUTING.md. Plain '#1234' references used as context are allowed.
close_issue_re='(^|[^[:alnum:]_])(clos(e|es|ed)|fix(es|ed)?|resolv(e|es|ed))[[:space:]:]+([[:alnum:]_.-]+/[[:alnum:]_.-]+)?#[0-9]+'

rc=0

merges="$( git rev-list --merges "${base_sha}..HEAD" )"
if [[ -n "${merges}" ]]; then
  while read -r c; do
    echo "[FAIL] $( git log -1 --pretty=%s "${c}" )"
    echo "       Commit $( git log -1 --pretty=%h "${c}" ) is a merge commit."
    echo "       Rebase your branch on top of the target one instead of merging it: 'git fetch origin && git rebase origin/master'."
  done <<<"${merges}"
  rc=1
fi

while read -r c; do
  [[ -n "${c}" ]] || continue

  m="$( git log -1 --pretty=%s "${c}" )"

  if [[ "${m}" =~ ^(fixup|squash|amend)! ]]; then
    echo "[FAIL] ${m}"
    echo "       Commit $( git log -1 --pretty=%h "${c}" ) is an autosquash commit that wasn't squashed into its target."
    echo "       Squash it with 'git rebase -i --autosquash ${base_sha}'."
    rc=1
    continue
  fi

  if [[ "${m}" =~ ^Revert\ \" ]]; then
    echo "[FAIL] ${m}"
    echo "       Commit $( git log -1 --pretty=%h "${c}" ) uses the default git-revert message."
    echo "       Reword it to the Conventional Commits format, e.g. 'revert(scyllacluster): add the foo option', with the reverted commit's hash in the body."
    rc=1
    continue
  fi

  if [[ ! "${m}" =~ ^(${types_re})(\(.*\))?(!)?:\ .*$ ]]; then
    echo "[FAIL] ${m}"
    echo "       Commit $( git log -1 --pretty=%h "${c}" ) doesn't follow the Conventional Commits format."
    echo "       Expected '<type>[optional scope][!]: <description>', e.g. 'fix(scyllacluster): don't requeue on a missing Service'."
    echo "       Allowed types: ${types[*]}."
    echo "       Amend it with 'git commit --amend' or rewrite the history with 'git rebase -i ${base_sha}'."
    rc=1
    continue
  fi

  closing="$( git log -1 --pretty=%B "${c}" | grep -nEi -e "${close_issue_re}" || true )"
  if [[ -n "${closing}" ]]; then
    echo "[FAIL] ${m}"
    echo "       Commit $( git log -1 --pretty=%h "${c}" ) contains an issue closing keyword:"
    sed -e 's/^/         /' <<<"${closing}"
    echo "       GitHub closes the referenced issue when the commit lands on the default branch, whichever PR carries it."
    echo "       Move the keyword to the PR description, or reference the issue without one, e.g. 'see #1234'."
    rc=1
    continue
  fi

  echo "[OK]   ${m}"
done <<<"$( git rev-list --no-merges --reverse "${base_sha}..HEAD" )"

if [[ "${rc}" -ne 0 ]]; then
  echo
  echo "Commit policy violations found. See the 'Commits and PRs' section of CONTRIBUTING.md for the full policy." >&2
fi

exit "${rc}"
