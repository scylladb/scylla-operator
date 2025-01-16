#!/bin/bash
#
# Copyright (C) 2021 ScyllaDB
#
# This script deploys scylla-operator and scylla-manager.
# Usage: ${0} <operator_image_ref>

set -euxEo pipefail
shopt -s inherit_errexit

source "$( dirname "${BASH_SOURCE[0]}" )/lib/bash.sh"
source "$( dirname "${BASH_SOURCE[0]}" )/lib/kube.sh"

if [[ -z ${1+x} ]]; then
    echo "Missing operator image ref.\nUsage: ${0} <operator_image_ref>" >&2 >/dev/null
    exit 1
fi

trap cleanup-bg-jobs-on-exit EXIT

ARTIFACTS=${ARTIFACTS:-$( mktemp -d )}
OPERATOR_IMAGE_REF=${1}

if [ -z "${DEPLOY_DIR+x}" ]; then
  DEPLOY_DIR=${ARTIFACTS}/deploy
fi

mkdir -p "${DEPLOY_DIR}/"{operator,manager,prometheus-operator,haproxy-ingress}

cp ./deploy/manager/dev/*.yaml "${DEPLOY_DIR}/manager"
cp ./deploy/operator/*.yaml "${DEPLOY_DIR}/operator"
cp ./examples/third-party/prometheus-operator/*.yaml "${DEPLOY_DIR}/prometheus-operator"
cp ./examples/third-party/haproxy-ingress/*.yaml "${DEPLOY_DIR}/haproxy-ingress"
cp ./examples/third-party/cert-manager.yaml "${DEPLOY_DIR}/"

for f in $( find "${DEPLOY_DIR}"/ -type f -name '*.yaml' ); do
    sed -i -E -e "s~docker\.io/scylladb/scylla-operator:[^ @]+$~${OPERATOR_IMAGE_REF}~" "${f}"
done

# TODO: Replace it with ScyllaOperatorConfig field when available.
# Ref: https://github.com/scylladb/scylla-operator/issues/2314.
SO_SCYLLA_OPERATOR_LOGLEVEL="${SO_SCYLLA_OPERATOR_LOGLEVEL:-4}"
export SO_SCYLLA_OPERATOR_LOGLEVEL
yq e --inplace '.spec.template.spec.containers[0].args += "--loglevel=" + env(SO_SCYLLA_OPERATOR_LOGLEVEL)' "${DEPLOY_DIR}/operator/50_operator.deployment.yaml"
yq e --inplace '.spec.template.spec.containers[0].args += "--loglevel=" + env(SO_SCYLLA_OPERATOR_LOGLEVEL)' "${DEPLOY_DIR}/manager/50_controller_deployment.yaml"

if [[ -n "${SO_SCYLLA_OPERATOR_REPLICAS:-}" ]]; then
  yq e --inplace '.spec.replicas = env(SO_SCYLLA_OPERATOR_REPLICAS)' "${DEPLOY_DIR}/operator/50_operator.deployment.yaml"
fi

if [[ -n "${SO_SCYLLA_OPERATOR_RESOURCE_REQUESTS_CPU:-}" ]]; then
  yq e --inplace '.spec.template.spec.containers[0].resources.requests.cpu = env(SO_SCYLLA_OPERATOR_RESOURCE_REQUESTS_CPU)' "${DEPLOY_DIR}/operator/50_operator.deployment.yaml"
fi

if [[ -n "${SO_SCYLLA_OPERATOR_RESOURCE_REQUESTS_MEMORY:-}" ]]; then
  yq e --inplace '.spec.template.spec.containers[0].resources.requests.memory = env(SO_SCYLLA_OPERATOR_RESOURCE_REQUESTS_MEMORY)' "${DEPLOY_DIR}/operator/50_operator.deployment.yaml"
fi

if [[ -n "${SO_SCYLLA_OPERATOR_RESOURCE_LIMITS_CPU:-}" ]]; then
  yq e --inplace '.spec.template.spec.containers[0].resources.limits.cpu = env(SO_SCYLLA_OPERATOR_RESOURCE_LIMITS_CPU)' "${DEPLOY_DIR}/operator/50_operator.deployment.yaml"
fi

if [[ -n "${SO_SCYLLA_OPERATOR_RESOURCE_LIMITS_MEMORY:-}" ]]; then
  yq e --inplace '.spec.template.spec.containers[0].resources.limits.memory = env(SO_SCYLLA_OPERATOR_RESOURCE_LIMITS_MEMORY)' "${DEPLOY_DIR}/operator/50_operator.deployment.yaml"
fi

yq e --inplace '.spec.template.spec.containers[0].args += ["--qps=200", "--burst=400"]' "${DEPLOY_DIR}/operator/50_operator.deployment.yaml"
yq e --inplace '.spec.template.spec.containers[0].args += ["--crypto-key-buffer-size-min=3", "--crypto-key-buffer-size-max=6", "--crypto-key-buffer-delay=2s"]' "${DEPLOY_DIR}/operator/50_operator.deployment.yaml"

if [[ -n ${SCYLLA_OPERATOR_FEATURE_GATES+x} ]]; then
    yq e --inplace '.spec.template.spec.containers[0].args += "--feature-gates="+ strenv(SCYLLA_OPERATOR_FEATURE_GATES)' "${DEPLOY_DIR}/operator/50_operator.deployment.yaml"
fi

kubectl_create -n prometheus-operator -f "${DEPLOY_DIR}/prometheus-operator"
kubectl_create -n haproxy-ingress -f "${DEPLOY_DIR}/haproxy-ingress"
kubectl_create -f "${DEPLOY_DIR}"/cert-manager.yaml

# Wait for cert-manager
kubectl wait --for condition=established --timeout=60s crd/certificates.cert-manager.io crd/issuers.cert-manager.io
for d in cert-manager{,-cainjector,-webhook}; do
    kubectl -n cert-manager rollout status --timeout=5m deployment.apps/"${d}"
done
wait-for-object-creation cert-manager secret/cert-manager-webhook-ca

kubectl_create -f "${DEPLOY_DIR}"/operator

# Manager needs scylla CRD registered and the webhook running
kubectl wait --for condition=established crd/scyllaclusters.scylla.scylladb.com
kubectl -n scylla-operator rollout status --timeout=5m deployment.apps/scylla-operator
kubectl -n scylla-operator rollout status --timeout=5m deployment.apps/webhook-server

if [[ -z "${SO_NODECONFIG_PATH:-}" ]]; then
  echo "Skipping NodeConfig creation"
else
  kubectl_create -f="${SO_NODECONFIG_PATH}"
  kubectl wait --for='condition=Reconciled' --timeout=10m -f="${SO_NODECONFIG_PATH}"
fi

if [[ -z "${SO_CSI_DRIVER_PATH:-}" ]]; then
  echo "Skipping CSI driver creation"
else
  kubectl_create -n=local-csi-driver -f="${SO_CSI_DRIVER_PATH}"
  kubectl -n=local-csi-driver rollout status daemonset.apps/local-csi-driver
fi

if [[ -n "${SO_SCYLLACLUSTER_STORAGECLASS_NAME:-}" ]]; then
  yq e --inplace '.spec.datacenter.racks[0].storage.storageClassName = env(SO_SCYLLACLUSTER_STORAGECLASS_NAME)' "${DEPLOY_DIR}/manager/50_scyllacluster.yaml"
elif [[ -n "${SO_SCYLLACLUSTER_STORAGECLASS_NAME+x}" ]]; then
  yq e --inplace 'del(.spec.datacenter.racks[0].storage.storageClassName)' "${DEPLOY_DIR}/manager/50_scyllacluster.yaml"
fi
kubectl_create -f "${DEPLOY_DIR}"/manager

kubectl -n=scylla-manager wait --timeout=10m --for='condition=Progressing=False' scyllaclusters.scylla.scylladb.com/scylla-manager-cluster
kubectl -n=scylla-manager wait --timeout=10m --for='condition=Degraded=False' scyllaclusters.scylla.scylladb.com/scylla-manager-cluster
kubectl -n=scylla-manager wait --timeout=10m --for='condition=Available=True' scyllaclusters.scylla.scylladb.com/scylla-manager-cluster
kubectl -n scylla-manager rollout status --timeout=10m deployment.apps/scylla-manager
kubectl -n scylla-manager rollout status --timeout=10m deployment.apps/scylla-manager-controller

kubectl -n haproxy-ingress rollout status --timeout=5m deployment.apps/haproxy-ingress
kubectl -n haproxy-ingress rollout status --timeout=5m deployment.apps/haproxy-ingress deploy/ingress-default-backend deploy/prometheus

kubectl wait --for condition=established crd/nodeconfigs.scylla.scylladb.com
kubectl wait --for condition=established crd/scyllaoperatorconfigs.scylla.scylladb.com
kubectl wait --for condition=established crd/scylladbmonitorings.scylla.scylladb.com
kubectl wait --for condition=established $( find "${DEPLOY_DIR}/prometheus-operator/" -name '*.crd.yaml' -printf '-f=%p\n' )
kubectl -n=prometheus-operator rollout status deploy/prometheus-operator
