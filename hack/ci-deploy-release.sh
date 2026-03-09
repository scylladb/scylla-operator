#!/usr/bin/env bash
#
# Copyright (C) 2024 ScyllaDB
#
# This script deploys scylla-operator stack.
# Usage: ${0} <operator_image_ref>
# (Avoid using rolling tags.)

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

source_raw="$( skopeo inspect --format='{{ index .Labels "org.opencontainers.image.source" }}' "docker://${OPERATOR_IMAGE_REF}" )"
if [[ -z "${source_raw}" ]]; then
    echo "Image '${OPERATOR_IMAGE_REF}' is missing source label" >&2 >/dev/null
    exit 1
fi
source_url="${source_raw/"://github.com/"/"://raw.githubusercontent.com/"}"

revision="$( skopeo inspect --format='{{ index .Labels "org.opencontainers.image.revision" }}' "docker://${OPERATOR_IMAGE_REF}" )"
if [[ -z "${revision}" ]]; then
    echo "Image '${OPERATOR_IMAGE_REF}' is missing revision label" >&2 >/dev/null
    exit 1
fi

ARTIFACTS="${ARTIFACTS:-$( mktemp -d )}"

if [ -z "${ARTIFACTS_DEPLOY_DIR+x}" ]; then
  ARTIFACTS_DEPLOY_DIR="${ARTIFACTS}/deploy"
fi

SO_DISABLE_SCYLLADB_MANAGER_DEPLOYMENT=${SO_DISABLE_SCYLLADB_MANAGER_DEPLOYMENT:-false}

mkdir -p "${ARTIFACTS_DEPLOY_DIR}/"operator

# Do not install prometheus-operator if the platform already has it (e.g., OpenShift).
if [[ -n "${SO_DISABLE_PROMETHEUS_OPERATOR:-}" ]]; then
  echo "Skipping prometheus-operator deployment"
else
  kubectl_create -n=prometheus-operator -f="${source_url}/${revision}/examples/third-party/prometheus-operator.yaml"
fi

if [[ "${SO_ENABLE_OPENSHIFT_USER_WORKLOAD_MONITORING:-}" == "true" ]]; then
  echo "Enabling OpenShift User Workload Monitoring"
  kubectl_create -f="${source_url}/${revision}/hack/.ci/manifests/namespaces/openshift-monitoring/openshift-uwm.cm.yaml"
else
  echo "Skipping enabling OpenShift User Workload Monitoring"
fi

kubectl_create -n=haproxy-ingress -f="${source_url}/${revision}/examples/third-party/haproxy-ingress.yaml"

install-operator "${source_url}/${revision}"

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
fi

if [[ -z "${SO_CSI_DRIVER_PATH+x}" ]]; then
  kubectl_create -n=local-csi-driver -f="${source_url}/${revision}/examples/common/local-volume-provisioner/local-csi-driver/"{00_clusterrole.yaml,00_clusterrole_def.yaml,00_clusterrole_def_openshift.yaml,00_namespace.yaml,00_scylladb-local-xfs.storageclass.yaml,10_csidriver.yaml,10_serviceaccount.yaml,20_clusterrolebinding.yaml,50_daemonset.yaml}
  kubectl -n=local-csi-driver rollout status --timeout=5m daemonset.apps/local-csi-driver
elif [[ -n "${SO_CSI_DRIVER_PATH}" ]]; then
  kubectl_create -n=local-csi-driver -f="${SO_CSI_DRIVER_PATH}"
  kubectl -n=local-csi-driver rollout status --timeout=5m daemonset.apps/local-csi-driver
else
  echo "Skipping CSI driver creation"
fi

if [[ "${SO_DISABLE_SCYLLADB_MANAGER_DEPLOYMENT}" == "true" ]]; then
  echo "Skipping ScyllaDBManager deployment"
else
  mkdir -p "${ARTIFACTS_DEPLOY_DIR}/manager"

  cat > "${ARTIFACTS_DEPLOY_DIR}/manager/kustomization.yaml" << EOF
resources:
- ${source_url}/${revision}/deploy/manager-prod.yaml
EOF

  if [[ -n "${SO_SCYLLACLUSTER_STORAGECLASS_NAME:-}" ]]; then
    # SO_SCYLLACLUSTER_STORAGECLASS_NAME is set and nonempty.
    cat << EOF | \
    yq eval-all --inplace 'select(fileIndex == 0) as $f | select(fileIndex == 1) as $p | with( $f.patches; . += $p | ... style="") | $f' "${ARTIFACTS_DEPLOY_DIR}/manager/kustomization.yaml" -
- patch: |-
    - op: replace
      path: /spec/datacenter/racks/0/storage/storageClassName
      value: "${SO_SCYLLACLUSTER_STORAGECLASS_NAME}"
  target:
    group: scylla.scylladb.com
    version: v1
    kind: ScyllaCluster
    name: scylla-manager-cluster
EOF
  elif [[ -n "${SO_SCYLLACLUSTER_STORAGECLASS_NAME+x}" ]]; then
    # SO_SCYLLACLUSTER_STORAGECLASS_NAME is set and empty.
    cat << EOF | \
    yq eval-all --inplace 'select(fileIndex == 0) as $f | select(fileIndex == 1) as $p | with( $f.patches; . += $p | ... style="") | $f' "${ARTIFACTS_DEPLOY_DIR}/manager/kustomization.yaml" -
- patch: |-
    - op: remove
      path: /spec/datacenter/racks/0/storage/storageClassName
  target:
    group: scylla.scylladb.com
    version: v1
    kind: ScyllaCluster
    name: scylla-manager-cluster
EOF
  fi

  if [[ -n "${SCYLLADB_VERSION:-}" ]]; then
    cat << EOF | \
    yq eval-all --inplace 'select(fileIndex == 0) as $f | select(fileIndex == 1) as $p | with( $f.patches; . += $p | ... style="") | $f' "${ARTIFACTS_DEPLOY_DIR}/manager/kustomization.yaml" -
- patch: |-
    - op: replace
      path: /spec/version
      value: "${SCYLLADB_VERSION}"
  target:
    group: scylla.scylladb.com
    version: v1
    kind: ScyllaCluster
    name: scylla-manager-cluster
EOF
  fi

  if [[ -n "${SCYLLA_MANAGER_VERSION:-}" ]]; then
    cat << EOF | \
    yq eval-all --inplace 'select(fileIndex == 0) as $f | select(fileIndex == 1) as $p | with( $f.patches; . += $p | ... style="") | $f' "${ARTIFACTS_DEPLOY_DIR}/manager/kustomization.yaml" -
- patch: |-
    - op: replace
      path: /spec/template/spec/containers/0/image
      value: "docker.io/scylladb/scylla-manager:${SCYLLA_MANAGER_VERSION}"
  target:
    group: apps
    version: v1
    kind: Deployment
    name: scylla-manager
EOF
  fi

  if [[ -n "${SCYLLA_MANAGER_AGENT_VERSION:-}" ]]; then
    cat << EOF | \
    yq eval-all --inplace 'select(fileIndex == 0) as $f | select(fileIndex == 1) as $p | with( $f.patches; . += $p | ... style="") | $f' "${ARTIFACTS_DEPLOY_DIR}/manager/kustomization.yaml" -
- patch: |-
    - op: replace
      path: /spec/agentVersion
      value: "${SCYLLA_MANAGER_AGENT_VERSION}"
  target:
    group: scylla.scylladb.com
    version: v1
    kind: ScyllaCluster
    name: scylla-manager-cluster
EOF
  fi

  kubectl kustomize "${ARTIFACTS_DEPLOY_DIR}/manager" | kubectl_create -n=scylla-manager -f=-

  kubectl -n=scylla-manager wait --timeout=5m --for='condition=Progressing=False' scyllaclusters.scylla.scylladb.com/scylla-manager-cluster
  kubectl -n=scylla-manager wait --timeout=5m --for='condition=Degraded=False' scyllaclusters.scylla.scylladb.com/scylla-manager-cluster
  kubectl -n=scylla-manager wait --timeout=5m --for='condition=Available=True' scyllaclusters.scylla.scylladb.com/scylla-manager-cluster
  kubectl -n=scylla-manager rollout status --timeout=5m deployment.apps/scylla-manager
fi

kubectl -n=haproxy-ingress rollout status --timeout=5m deployment.apps/haproxy-ingress

kubectl wait --for condition=established crd/{scyllaoperatorconfigs,scylladbmonitorings}.scylla.scylladb.com

if [[ -n "${SO_DISABLE_PROMETHEUS_OPERATOR:-}" ]]; then
  echo "Skipping waiting for prometheus-operator"
else
  kubectl wait --for condition=established crd/{prometheuses,prometheusrules,servicemonitors}.monitoring.coreos.com
fi
