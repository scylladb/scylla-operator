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

mkdir -p "${ARTIFACTS_DEPLOY_DIR}/prometheus-operator"

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

cp ./examples/third-party/haproxy-ingress.yaml "${ARTIFACTS_DEPLOY_DIR}/haproxy-ingress.yaml"

# Do not install prometheus-operator if the platform already has it (e.g., OpenShift).
if [[ -n "${SO_DISABLE_PROMETHEUS_OPERATOR:-}" ]]; then
  echo "Skipping prometheus-operator deployment"
else
  kubectl_create -n prometheus-operator -f "${ARTIFACTS_DEPLOY_DIR}/prometheus-operator.yaml"
fi

if [[ "${SO_ENABLE_OPENSHIFT_USER_WORKLOAD_MONITORING:-}" == "true" ]]; then
  kubectl_create -f "${ARTIFACTS_DEPLOY_DIR}/openshift-uwm.cm.yaml"
fi

kubectl_create -n haproxy-ingress -f "${ARTIFACTS_DEPLOY_DIR}/haproxy-ingress.yaml"

install-operator "$( realpath "$( dirname "${BASH_SOURCE[0]}" )/../" )"

# Wait for operator and webhook server to roll out.
# The manager deployed below needs the ScyllaCluster CRD registered.
wait-for-scylla-operator-rollout

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
  install-scylladb-manager "$( realpath "$( dirname "${BASH_SOURCE[0]}" )/../" )"

  wait-for-scyllacluster-rollout scylla-manager scylla-manager-cluster
  kubectl -n scylla-manager rollout status --timeout=10m deployment.apps/scylla-manager
fi

# The default backend is pinned to an amd64-only image, so its rollout can't complete on arm64 clusters and isn't
# waited for. It only answers requests that match no Ingress and nothing in the suites needs it.
# TODO: put deploy/ingress-default-backend back once https://scylladb.atlassian.net/browse/OPERATOR-451 is fixed.
kubectl -n haproxy-ingress rollout status --timeout=5m deployment.apps/haproxy-ingress deploy/prometheus

kubectl wait --for condition=established crd/nodeconfigs.scylla.scylladb.com
kubectl wait --for condition=established crd/scyllaoperatorconfigs.scylla.scylladb.com
kubectl wait --for condition=established crd/scylladbmonitorings.scylla.scylladb.com

if [[ -n "${SO_DISABLE_PROMETHEUS_OPERATOR:-}" ]]; then
  echo "Skipping waiting for prometheus-operator"
else
  kubectl wait --for condition=established crd/{prometheuses,prometheusrules,servicemonitors}.monitoring.coreos.com
  kubectl -n=prometheus-operator rollout status deploy/prometheus-operator
fi
