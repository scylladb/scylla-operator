#!/bin/bash
#
# Copyright (C) 2021 ScyllaDB
#
# This script deploys scylla-operator and scylla-manager.
# Usage: ${0} <operator_image_ref>

set -euxEo pipefail

function wait-for-object-creation {
    for i in {1..30}; do
        { kubectl -n "${1}" get "${2}" && break; } || sleep 1
    done
}

if [[ -z ${1+x} ]]; then
    echo "Missing operator image ref.\nUsage: ${0} <operator_image_ref>" >&2 >/dev/null
    exit 1
fi

ARTIFACTS_DIR=${ARTIFACTS_DIR:-$( mktemp -d )}
OPERATOR_IMAGE_REF=${1}

deploy_dir=${ARTIFACTS_DIR}/deploy
mkdir -p "${deploy_dir}/"{operator,manager}

cp ./deploy/manager/dev/*.yaml "${deploy_dir}/manager"
cp ./deploy/operator/*.yaml "${deploy_dir}/operator"
cp ./examples/common/cert-manager.yaml "${deploy_dir}/"

for f in $( find "${deploy_dir}"/ -type f -name '*.yaml' ); do
    sed -i -E -e "s~docker.io/scylladb/scylla-operator(:|@sha256:)[^ ]*~${OPERATOR_IMAGE_REF}~" "${f}"
done

kubectl apply -f "${deploy_dir}"/cert-manager.yaml

# Wait for cert-manager
kubectl wait --for condition=established --timeout=60s crd/certificates.cert-manager.io crd/issuers.cert-manager.io
wait-for-object-creation cert-manager deployment.apps/cert-manager-webhook
kubectl -n cert-manager rollout status --timeout=5m deployment.apps/cert-manager-webhook

kubectl apply -f "${deploy_dir}"/operator

# Manager needs scylla CRD registered and the webhook running
kubectl wait --for condition=established crd/scyllaclusters.scylla.scylladb.com
kubectl -n scylla-operator rollout status --timeout=5m deployment.apps/scylla-operator

kubectl apply -f "${deploy_dir}"/manager

wait-for-object-creation scylla-manager statefulset.apps/scylla-manager-cluster-manager-dc-manager-rack
kubectl -n scylla-manager rollout status --timeout=5m statefulset.apps/scylla-manager-cluster-manager-dc-manager-rack
kubectl -n scylla-manager rollout status --timeout=5m deployment.apps/scylla-manager-controller
