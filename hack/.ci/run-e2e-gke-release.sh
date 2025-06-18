#!/usr/bin/env bash
#
# Copyright (C) 2024 ScyllaDB
#

set -euExo pipefail
shopt -s inherit_errexit

if [ -z "${ARTIFACTS+x}" ]; then
  echo "ARTIFACTS can't be empty" > /dev/stderr
  exit 2
fi

source "$( dirname "${BASH_SOURCE[0]}" )/../lib/kube.sh"
source "$( dirname "${BASH_SOURCE[0]}" )/lib/e2e.sh"
source "$( dirname "${BASH_SOURCE[0]}" )/run-e2e-shared.env.sh"

parent_dir="$( dirname "${BASH_SOURCE[0]}" )"

trap gather-artifacts-on-exit EXIT
trap gracefully-shutdown-e2es INT

SO_NODECONFIG_PATH="${SO_NODECONFIG_PATH=${parent_dir}/manifests/cluster/nodeconfig.yaml}"
export SO_NODECONFIG_PATH

run-deploy-script-in-all-clusters "${parent_dir}/../ci-deploy-release.sh"

apply-e2e-workarounds
run-e2e
