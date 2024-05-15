#!/bin/bash
#
# Copyright (C) 2021 ScyllaDB
#
# This script deploys scylla-operator and scylla-manager.
# Usage: ${0} <operator_image_ref>

set -euxEo pipefail
shopt -s inherit_errexit

source "$( dirname "${BASH_SOURCE[0]}" )/lib/kube.sh"

if [[ -z ${1+x} ]]; then
    echo "Missing operator image ref.\nUsage: ${0} <operator_image_ref>" >&2 >/dev/null
    exit 1
fi

ARTIFACTS_DIR=${ARTIFACTS_DIR:-$( mktemp -d )}
OPERATOR_IMAGE_REF=${1}

deploy_dir=${ARTIFACTS_DIR}/deploy
mkdir -p "${deploy_dir}/"{operator,manager,prometheus-operator,haproxy-ingress}

cp ./deploy/manager/dev/*.yaml "${deploy_dir}/manager"
cp ./deploy/operator/*.yaml "${deploy_dir}/operator"
cp ./examples/third-party/prometheus-operator/*.yaml "${deploy_dir}/prometheus-operator"
cp ./examples/third-party/haproxy-ingress/*.yaml "${deploy_dir}/haproxy-ingress"
cp ./examples/common/cert-manager.yaml "${deploy_dir}/"

for f in $( find "${deploy_dir}"/ -type f -name '*.yaml' ); do
    sed -i -E -e "s~docker.io/scylladb/scylla-operator(:|@sha256:)[^ ]*~${OPERATOR_IMAGE_REF}~" "${f}"
done

yq e --inplace '.spec.template.spec.containers[0].args += ["--qps=200", "--burst=400"]' "${deploy_dir}/operator/50_operator.deployment.yaml"
yq e --inplace '.spec.template.spec.containers[0].args += ["--crypto-key-buffer-size-min=3", "--crypto-key-buffer-size-max=6", "--crypto-key-buffer-delay=2s"]' "${deploy_dir}/operator/50_operator.deployment.yaml"

if [[ -n ${SCYLLA_OPERATOR_FEATURE_GATES+x} ]]; then
    yq e --inplace '.spec.template.spec.containers[0].args += "--feature-gates="+ strenv(SCYLLA_OPERATOR_FEATURE_GATES)' "${deploy_dir}/operator/50_operator.deployment.yaml"
fi

kubectl_create -n prometheus-operator -f "${deploy_dir}/prometheus-operator"
kubectl_create -n haproxy-ingress -f "${deploy_dir}/haproxy-ingress"
kubectl_create -f "${deploy_dir}"/cert-manager.yaml

# Wait for cert-manager
kubectl wait --for condition=established --timeout=60s crd/certificates.cert-manager.io crd/issuers.cert-manager.io
for d in cert-manager{,-cainjector,-webhook}; do
    wait-for-object-creation cert-manager deployment.apps/"${d}"
    kubectl -n cert-manager rollout status --timeout=5m deployment.apps/"${d}"
done
wait-for-object-creation cert-manager secret/cert-manager-webhook-ca

kubectl_create -f "${deploy_dir}"/operator

# Manager needs scylla CRD registered and the webhook running
kubectl wait --for condition=established crd/scyllaclusters.scylla.scylladb.com
kubectl -n scylla-operator rollout status --timeout=5m deployment.apps/scylla-operator
kubectl -n scylla-operator rollout status --timeout=5m deployment.apps/webhook-server

if [[ -z "${SO_NODECONFIG_PATH:-}" ]]; then
  echo "Skipping NodeConfig creation"
else
  kubectl_create -f="${SO_NODECONFIG_PATH}"
fi

if [[ -z "${SO_CSI_DRIVER_PATH:-}" ]]; then
  echo "Skipping CSI driver creation"
else
  kubectl_create -n=local-csi-driver -f="${SO_CSI_DRIVER_PATH}"
fi

kubectl_create -f "${deploy_dir}"/manager

wait-for-object-creation scylla-manager statefulset.apps/scylla-manager-cluster-manager-dc-manager-rack
kubectl -n scylla-manager rollout status --timeout=5m statefulset.apps/scylla-manager-cluster-manager-dc-manager-rack
kubectl -n scylla-manager rollout status --timeout=5m deployment.apps/scylla-manager
kubectl -n scylla-manager rollout status --timeout=5m deployment.apps/scylla-manager-controller

kubectl -n haproxy-ingress rollout status --timeout=5m deployment.apps/haproxy-ingress

kubectl wait --for condition=established crd/nodeconfigs.scylla.scylladb.com
kubectl wait --for condition=established crd/scyllaoperatorconfigs.scylla.scylladb.com
kubectl wait --for condition=established crd/scylladbmonitorings.scylla.scylladb.com
kubectl wait --for condition=established $( find "${deploy_dir}/prometheus-operator/" -name '*.crd.yaml' -printf '-f=%p\n' )
