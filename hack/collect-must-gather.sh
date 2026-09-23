#!/usr/bin/env bash
#
# Copyright (C) 2026 ScyllaDB
#
# This script collects must-gather evidence from the cluster and archives it
# into <dest_dir>/must-gather.tar.gz. It is a no-op if the cluster is not reachable,
# so it can be run unconditionally in CI cleanup paths.
# Usage: ${0} <dest_dir>

set -euEo pipefail
shopt -s inherit_errexit

if [[ "$#" -ne 1 ]]; then
  echo -e "Missing arguments.\nUsage: ${0} <dest_dir>" > /dev/stderr
  exit 1
fi

dest_dir="${1}"

if ! kubectl cluster-info > /dev/null 2>&1; then
  echo "Kubernetes cluster is not reachable, skipping."
  exit 0
fi

source_root="$( realpath "$( dirname "${BASH_SOURCE[0]}" )/.." )"

mkdir -p "${dest_dir}/must-gather"
go run "${source_root}/cmd/scylla-operator" must-gather --all-resources --loglevel=2 --dest-dir="${dest_dir}/must-gather" || true
tar -czf "${dest_dir}/must-gather.tar.gz" -C "${dest_dir}" must-gather
