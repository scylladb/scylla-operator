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
parent_dir="$( dirname "${BASH_SOURCE[0]}" )"

trap gather-artifact-on-exit EXIT


SO_NODECONFIG_PATH="${SO_NODECONFIG_PATH=${parent_dir}/manifests/cluster/nodeconfig.yaml}"
export SO_NODECONFIG_PATH

# Make sure there is no default storage class before we create our own so we always use our own provisioner from the release.
for r in $( kubectl get storageclasses -o name ); do kubectl patch "${r}" -p '{"metadata": {"annotations":{"storageclass.kubernetes.io/is-default-class":"false"}}}'; done

SCYLLA_OPERATOR_FEATURE_GATES="${SCYLLA_OPERATOR_FEATURE_GATES:-AllAlpha=true,AllBeta=true}"
export SCYLLA_OPERATOR_FEATURE_GATES

timeout --foreground -v 10m "${parent_dir}/../ci-deploy-release.sh" "${SO_IMAGE}"

apply-e2e-workarounds
run-e2e
