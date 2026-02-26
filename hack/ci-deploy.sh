#!/usr/bin/env bash
#
# Copyright (C) 2021 ScyllaDB
#
# This script deploys scylla-operator and scylla-manager.
# Usage: ${0} <operator_image_ref>

set -euxEo pipefail
shopt -s inherit_errexit

source "$( dirname "${BASH_SOURCE[0]}" )/lib/bash.sh"
source "$( dirname "${BASH_SOURCE[0]}" )/lib/kube.sh"
source "$( dirname "${BASH_SOURCE[0]}" )/lib/install.sh"

if [[ "$#" -ne 1 ]]; then
  echo "Missing arguments.\nUsage: ${0} <operator_image_ref>" > /dev/stderr
  exit 1
fi

OPERATOR_IMAGE_REF="${1}"
export OPERATOR_IMAGE_REF

trap cleanup-bg-jobs-on-exit EXIT

ARTIFACTS=${ARTIFACTS:-$( mktemp -d )}

if [ -z "${ARTIFACTS_DEPLOY_DIR+x}" ]; then
  ARTIFACTS_DEPLOY_DIR=${ARTIFACTS}/deploy
fi

SO_DISABLE_SCYLLADB_MANAGER_DEPLOYMENT=${SO_DISABLE_SCYLLADB_MANAGER_DEPLOYMENT:-false}
SO_INSTALL_XFSPROGS_ON_NODES="${SO_INSTALL_XFSPROGS_ON_NODES:-}"

mkdir -p "${ARTIFACTS_DEPLOY_DIR}/"{prometheus-operator,haproxy-ingress}

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

cp ./examples/third-party/haproxy-ingress/*.yaml "${ARTIFACTS_DEPLOY_DIR}/haproxy-ingress"

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

install-operator "$( realpath "$( dirname "${BASH_SOURCE[0]}" )/../" )"

# Wait for operator and webhook server to roll out
wait-for-object-creation scylla-operator deployment.apps/scylla-operator 5m
kubectl -n scylla-operator rollout status --timeout=5m deployment.apps/scylla-operator
wait-for-object-creation scylla-operator deployment.apps/webhook-server 5m
kubectl -n scylla-operator rollout status --timeout=5m deployment.apps/webhook-server

# Manager needs scylla CRD registered
wait-for-object-creation scylla-operator crd/scyllaclusters.scylla.scylladb.com 5m
kubectl wait --for condition=established --timeout=5m crd/scyllaclusters.scylla.scylladb.com

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
