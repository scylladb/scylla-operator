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
# It's a no-op outside GitHub Actions.
# $1 - group title
function gha-group {
  if [[ "${GITHUB_ACTIONS:-}" != "true" ]]; then
    return
  fi

  gha-endgroup
  echo "::group::${1}" > /dev/stderr
  gha_group_open="true"
}

# gha-endgroup ends the open GitHub Actions log group, if any.
function gha-endgroup {
  if [[ "${gha_group_open:-}" != "true" ]]; then
    return
  fi

  echo "::endgroup::" > /dev/stderr
  gha_group_open="false"
}

function cleanup-bg-jobs-on-exit() {
  ec=$?
  cleanup-bg-jobs "${ec}"
}
