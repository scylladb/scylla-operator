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

if [ -z "${ARTIFACTS_DEPLOY_DIR+x}" ]; then
  ARTIFACTS_DEPLOY_DIR=${ARTIFACTS}/deploy
fi

SO_DISABLE_SCYLLADB_MANAGER_DEPLOYMENT=${SO_DISABLE_SCYLLADB_MANAGER_DEPLOYMENT:-false}
SO_INSTALL_XFSPROGS_ON_NODES="${SO_INSTALL_XFSPROGS_ON_NODES:-}"

mkdir -p "${ARTIFACTS_DEPLOY_DIR}/"{operator,prometheus-operator,haproxy-ingress}

if [[ -n "${SO_INSTALL_XFSPROGS_ON_NODES:-}" ]]; then
  cp ./examples/gke/install-xfsprogs.daemonset.yaml "${ARTIFACTS_DEPLOY_DIR}/"
else
  echo "Skipping installing xfsprogs on all nodes"
fi

if [[ -n "${SO_DISABLE_PROMETHEUS_OPERATOR:-}" ]]; then
  echo "Skipping copying prometheus-operator manifests to ${ARTIFACTS_DEPLOY_DIR}"
else
  cp ./examples/third-party/prometheus-operator.yaml "${ARTIFACTS_DEPLOY_DIR}/prometheus-operator.yaml"
fi

if [[ "${SO_ENABLE_OPENSHIFT_USER_WORKLOAD_MONITORING:-}" == "true" ]]; then
  echo "Enabling OpenShift User Workload Monitoring"
  cp ./hack/.ci/manifests/namespaces/openshift-monitoring/openshift-uwm.cm.yaml "${ARTIFACTS_DEPLOY_DIR}/"
else
  echo "Skipping enabling OpenShift User Workload Monitoring"
fi

cp ./deploy/operator/*.yaml "${ARTIFACTS_DEPLOY_DIR}/operator"
cp ./examples/third-party/haproxy-ingress/*.yaml "${ARTIFACTS_DEPLOY_DIR}/haproxy-ingress"
cp ./examples/third-party/cert-manager.yaml "${ARTIFACTS_DEPLOY_DIR}/"

for f in $( find "${ARTIFACTS_DEPLOY_DIR}"/ -type f -name '*.yaml' ); do
    sed -i -E -e "s~docker\.io/scylladb/scylla-operator:[^ @]+$~${OPERATOR_IMAGE_REF}~" "${f}"
done

# TODO: Replace it with ScyllaOperatorConfig field when available.
# Ref: https://github.com/scylladb/scylla-operator/issues/2314.
SO_SCYLLA_OPERATOR_LOGLEVEL="${SO_SCYLLA_OPERATOR_LOGLEVEL:-4}"
export SO_SCYLLA_OPERATOR_LOGLEVEL
yq e --inplace '.spec.template.spec.containers[0].args += "--loglevel=" + env(SO_SCYLLA_OPERATOR_LOGLEVEL)' "${ARTIFACTS_DEPLOY_DIR}/operator/50_operator.deployment.yaml"

yq e --inplace '.spec.template.spec.containers[0].args += ["--qps=200", "--burst=400"]' "${ARTIFACTS_DEPLOY_DIR}/operator/50_operator.deployment.yaml"

SO_CRYPTO_KEY_SIZE="${SO_CRYPTO_KEY_SIZE:-4096}"
export SO_CRYPTO_KEY_SIZE
SO_CRYPTO_KEY_BUFFER_SIZE_MIN="${SO_CRYPTO_KEY_BUFFER_SIZE_MIN:-6}"
export SO_CRYPTO_KEY_BUFFER_SIZE_MIN
SO_CRYPTO_KEY_BUFFER_SIZE_MAX="${SO_CRYPTO_KEY_BUFFER_SIZE_MAX:-10}"
export SO_CRYPTO_KEY_BUFFER_SIZE_MAX
yq e --inplace '.spec.template.spec.containers[0].args += ["--crypto-key-size="+env(SO_CRYPTO_KEY_SIZE), "--crypto-key-buffer-size-min="+env(SO_CRYPTO_KEY_BUFFER_SIZE_MIN), "--crypto-key-buffer-size-max="+env(SO_CRYPTO_KEY_BUFFER_SIZE_MAX), "--crypto-key-buffer-delay=2s"]' "${ARTIFACTS_DEPLOY_DIR}/operator/50_operator.deployment.yaml"

if [[ -n ${SCYLLA_OPERATOR_FEATURE_GATES+x} ]]; then
    yq e --inplace '.spec.template.spec.containers[0].args += "--feature-gates="+ env(SCYLLA_OPERATOR_FEATURE_GATES)' "${ARTIFACTS_DEPLOY_DIR}/operator/50_operator.deployment.yaml"
fi

# Do not install prometheus-operator if the platform already has it (e.g., OpenShift).
if [[ -n "${SO_DISABLE_PROMETHEUS_OPERATOR:-}" ]]; then
  echo "Skipping prometheus-operator deployment"
else
  kubectl_create -n prometheus-operator -f "${ARTIFACTS_DEPLOY_DIR}/prometheus-operator.yaml"
fi

if [[ -n ${SO_INSTALL_XFSPROGS_ON_NODES:-} ]]; then
  kubectl_create -f "${ARTIFACTS_DEPLOY_DIR}"/install-xfsprogs.daemonset.yaml
fi

if [[ "${SO_ENABLE_OPENSHIFT_USER_WORKLOAD_MONITORING:-}" == "true" ]]; then
  kubectl_create -f "${ARTIFACTS_DEPLOY_DIR}/openshift-uwm.cm.yaml"
fi

kubectl_create -n haproxy-ingress -f "${ARTIFACTS_DEPLOY_DIR}/haproxy-ingress"
kubectl_create -f "${ARTIFACTS_DEPLOY_DIR}"/cert-manager.yaml

# Wait for cert-manager
kubectl wait --for condition=established --timeout=60s crd/certificates.cert-manager.io crd/issuers.cert-manager.io
for d in cert-manager{,-cainjector,-webhook}; do
    kubectl -n cert-manager rollout status --timeout=5m deployment.apps/"${d}"
done
wait-for-object-creation cert-manager secret/cert-manager-webhook-ca

kubectl_create -f "${ARTIFACTS_DEPLOY_DIR}"/operator

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

if [[ "${SO_DISABLE_SCYLLADB_MANAGER_DEPLOYMENT}" == "true" ]]; then
  echo "Skipping ScyllaDBManager deployment"
else
  mkdir -p "${ARTIFACTS_DEPLOY_DIR}/manager"
  cp ./deploy/manager/dev/*.yaml "${ARTIFACTS_DEPLOY_DIR}/manager"

  if [[ -n "${SO_SCYLLACLUSTER_STORAGECLASS_NAME:-}" ]]; then
    yq e --inplace '.spec.datacenter.racks[0].storage.storageClassName = env(SO_SCYLLACLUSTER_STORAGECLASS_NAME)' "${ARTIFACTS_DEPLOY_DIR}/manager/50_scyllacluster.yaml"
  elif [[ -n "${SO_SCYLLACLUSTER_STORAGECLASS_NAME+x}" ]]; then
    yq e --inplace 'del(.spec.datacenter.racks[0].storage.storageClassName)' "${ARTIFACTS_DEPLOY_DIR}/manager/50_scyllacluster.yaml"
  fi

  if [[ -n "${SCYLLADB_VERSION:-}" ]]; then
    yq e --inplace '.spec.version = env(SCYLLADB_VERSION)' "${ARTIFACTS_DEPLOY_DIR}/manager/50_scyllacluster.yaml"
  fi

  if [[ -n "${SCYLLA_MANAGER_VERSION:-}" ]]; then
    yq e --inplace '.spec.template.spec.containers[0].image |= "docker.io/scylladb/scylla-manager:" + env(SCYLLA_MANAGER_VERSION)' "${ARTIFACTS_DEPLOY_DIR}/manager/50_manager_deployment.yaml"
  fi

  if [[ -n "${SCYLLA_MANAGER_AGENT_VERSION:-}" ]]; then
    yq e --inplace '.spec.agentVersion = env(SCYLLA_MANAGER_AGENT_VERSION)' "${ARTIFACTS_DEPLOY_DIR}/manager/50_scyllacluster.yaml"
  fi

  kubectl_create -f "${ARTIFACTS_DEPLOY_DIR}"/manager

  kubectl -n=scylla-manager wait --timeout=10m --for='condition=Progressing=False' scyllaclusters.scylla.scylladb.com/scylla-manager-cluster
  kubectl -n=scylla-manager wait --timeout=10m --for='condition=Degraded=False' scyllaclusters.scylla.scylladb.com/scylla-manager-cluster
  kubectl -n=scylla-manager wait --timeout=10m --for='condition=Available=True' scyllaclusters.scylla.scylladb.com/scylla-manager-cluster
  kubectl -n scylla-manager rollout status --timeout=10m deployment.apps/scylla-manager
fi

kubectl -n haproxy-ingress rollout status --timeout=5m deployment.apps/haproxy-ingress
kubectl -n haproxy-ingress rollout status --timeout=5m deployment.apps/haproxy-ingress deploy/ingress-default-backend deploy/prometheus

kubectl wait --for condition=established crd/nodeconfigs.scylla.scylladb.com
kubectl wait --for condition=established crd/scyllaoperatorconfigs.scylla.scylladb.com
kubectl wait --for condition=established crd/scylladbmonitorings.scylla.scylladb.com

if [[ -n "${SO_DISABLE_PROMETHEUS_OPERATOR:-}" ]]; then
  echo "Skipping waiting for prometheus-operator"
else
  kubectl wait --for condition=established crd/{prometheuses,prometheusrules,servicemonitors}.monitoring.coreos.com
  kubectl -n=prometheus-operator rollout status deploy/prometheus-operator
fi

if [[ -n "${SO_INSTALL_XFSPROGS_ON_NODES:-}" ]]; then
  kubectl -n=kube-system rollout status --timeout=5m daemonset.apps/install-xfsprogs
else
  echo "Skipping waiting for xfsprogs daemonset"
fi
