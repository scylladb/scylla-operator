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

SO_NODECONFIG_PATH="${SO_NODECONFIG_PATH=${parent_dir}/manifests/cluster/nodeconfig.yaml}"
export SO_NODECONFIG_PATH

SO_SCYLLACLUSTER_STORAGECLASS_NAME="${SO_SCYLLACLUSTER_STORAGECLASS_NAME=scylladb-local-xfs}"
export SO_SCYLLACLUSTER_STORAGECLASS_NAME

for i in "${!KUBECONFIGS[@]}"; do
  KUBECONFIG="${KUBECONFIGS[$i]}" ARTIFACTS_DEPLOY_DIR="${ARTIFACTS}/deploy/${i}" timeout --foreground -v 10m "${parent_dir}/../ci-deploy-release.sh" "${SO_IMAGE}" &
  ci_deploy_bg_pids["${i}"]=$!
done

for pid in "${ci_deploy_bg_pids[@]}"; do
  wait "${pid}"
done

KUBECONFIG="${KUBECONFIGS[0]}" apply-e2e-workarounds
KUBECONFIG="${KUBECONFIGS[0]}" run-e2e
