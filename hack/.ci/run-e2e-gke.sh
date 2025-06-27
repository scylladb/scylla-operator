#!/usr/bin/env bash
#
# Copyright (C) 2023 ScyllaDB
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

SO_NODECONFIG_PATH="${SO_NODECONFIG_PATH=./hack/.ci/manifests/cluster/nodeconfig.yaml}"
export SO_NODECONFIG_PATH
SO_CSI_DRIVER_PATH="${parent_dir}/manifests/namespaces/local-csi-driver/"
export SO_CSI_DRIVER_PATH
SO_SCYLLACLUSTER_STORAGECLASS_NAME="${SO_SCYLLACLUSTER_STORAGECLASS_NAME=scylladb-local-xfs}"
export SO_SCYLLACLUSTER_STORAGECLASS_NAME

# Backwards compatibility. Remove when release repo stops using SO_DISABLE_NODECONFIG.
if [[ "${SO_DISABLE_NODECONFIG:-false}" == "true" ]]; then
  SO_NODECONFIG_PATH=""
  SO_CSI_DRIVER_PATH=""
fi

DEPLOY_DIR="${ARTIFACTS}/deploy/cluster" timeout --foreground -v 10m "${parent_dir}/../ci-deploy.sh" "${SO_IMAGE}" &
ci_deploy_bg_pids=( $! )

for name in "${!WORKER_KUBECONFIGS[@]}"; do
  if [[ "${WORKER_KUBECONFIGS[$name]}" == "${KUBECONFIG}" ]]; then
    # Skip if the control plane cluster is also among the worker clusters.
    continue
  fi

  KUBECONFIG="${WORKER_KUBECONFIGS[$name]}" DEPLOY_DIR="${ARTIFACTS}/deploy/workers/${name}" SO_DISABLE_SCYLLADB_MANAGER_DEPLOYMENT=true timeout --foreground -v 10m "${parent_dir}/../ci-deploy.sh" "${SO_IMAGE}" &
  ci_deploy_bg_pids+=( $! )
done

for pid in "${ci_deploy_bg_pids[@]}"; do
  wait "${pid}"
done

# TODO: move ci-deployment to func in lib/e2e.sh
# TODO: link control plane to worker deploy dir

apply-e2e-workarounds
run-e2e
