#!/usr/bin/env bash
#
# Copyright (C) 2026 ScyllaDB
#
# This script builds the operator image and pushes it to the local KinD registry set up by hack/kind/cluster-setup.sh
# as localhost:${KIND_REGISTRY_PORT}/scylladb/scylla-operator:<tag>.
# Usage: ${0} <tag>

set -euEo pipefail
shopt -s inherit_errexit

readonly repo_root="$( dirname "${BASH_SOURCE[0]}" )/../.."

source "${repo_root}/hack/kind/lib.sh"

if [[ "$#" -ne 1 ]]; then
  echo -e "Missing arguments.\nUsage: ${0} <tag>" > /dev/stderr
  exit 1
fi

operator_image=""
build-and-push-operator-image "${repo_root}" operator_image "${1}"
