#!/bin/bash
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

function cleanup-bg-jobs-on-exit() {
  ec=$?
  cleanup-bg-jobs "${ec}"
}
