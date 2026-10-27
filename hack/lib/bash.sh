#!/usr/bin/env bash
#
# Copyright (C) 2024 ScyllaDB
#

set -euExo pipefail
shopt -s inherit_errexit

function cleanup-bg-jobs() {
  ec=$1
  mapfile -t bg_jobs < <( jobs -p )
  if [[ ${#bg_jobs[@]} -ne 0 ]]; then
    printf "Found %d leftover jobs" "${#bg_jobs[@]}" >> /dev/stderr
    jobs
    printf "Killing %d leftover jobs" "${#bg_jobs[@]}" >> /dev/stderr
    kill "${bg_jobs[@]}"
    echo "Waiting for terminating jobs to finish" >> /dev/stderr
    wait
    exit $(( 127 + ec ))
  fi
}

# gha-group starts a collapsible group in the GitHub Actions log, ending the open one first, as groups can't nest.
# The commands go to stderr, along with the trace and most of the output, so they don't get reordered against it.
# Whether a group is open is tracked in the exported GHA_GROUP_OPEN, so that a script can end a group its caller
# started. The helpers aren't traced, as their trace would show between the groups. It's a no-op outside GitHub Actions.
# $1 - group title
function gha-group {
  { local xtrace="${-//[^x]/}"; set +x; } 2>/dev/null

  if [[ "${GITHUB_ACTIONS:-}" == "true" ]]; then
    gha-endgroup
    echo "::group::${1}" >&2
    export GHA_GROUP_OPEN="true"
  fi

  if [[ -n "${xtrace}" ]]; then
    set -x
  fi
}

# gha-endgroup ends the open GitHub Actions log group, if any.
function gha-endgroup {
  { local xtrace="${-//[^x]/}"; set +x; } 2>/dev/null

  if [[ "${GHA_GROUP_OPEN:-}" == "true" ]]; then
    echo "::endgroup::" >&2
    export GHA_GROUP_OPEN="false"
  fi

  if [[ -n "${xtrace}" ]]; then
    set -x
  fi
}

function cleanup-bg-jobs-on-exit() {
  ec=$?
  cleanup-bg-jobs "${ec}"
}
