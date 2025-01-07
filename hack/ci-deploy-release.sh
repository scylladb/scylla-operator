#!/bin/bash
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

if [[ -n "${1+x}" ]]; then
    operator_image_ref="${1}"
else
    echo "Missing operator image ref.\nUsage: ${0} <operator_image_ref>" >&2 >/dev/null
    exit 1
fi

trap cleanup-bg-jobs-on-exit EXIT

source_raw="$( skopeo inspect --format='{{ index .Labels "org.opencontainers.image.source" }}' "docker://${operator_image_ref}" )"
if [[ -z "${source_raw}" ]]; then
    echo "Image '${operator_image_ref}' is missing source label" >&2 >/dev/null
    exit 1
fi
source_url="${source_raw/"://github.com/"/"://raw.githubusercontent.com/"}"

revision="$( skopeo inspect --format='{{ index .Labels "org.opencontainers.image.revision" }}' "docker://${operator_image_ref}" )"
if [[ -z "${revision}" ]]; then
    echo "Image '${operator_image_ref}' is missing revision label" >&2 >/dev/null
    exit 1
fi

ARTIFACTS="${ARTIFACTS:-$( mktemp -d )}"

if [ -z "${ARTIFACTS_DEPLOY_DIR+x}" ]; then
  ARTIFACTS_DEPLOY_DIR="${ARTIFACTS}/deploy"
fi

mkdir -p "${ARTIFACTS_DEPLOY_DIR}/"{operator,manager}

kubectl_create -n=prometheus-operator -f="${source_url}/${revision}/examples/third-party/prometheus-operator.yaml"
kubectl_create -n=haproxy-ingress -f="${source_url}/${revision}/examples/third-party/haproxy-ingress.yaml"

kubectl_create -f="${source_url}/${revision}/examples/third-party/cert-manager.yaml"
# Wait for cert-manager crd and webhooks
kubectl wait --for condition=established --timeout=60s crd/certificates.cert-manager.io crd/issuers.cert-manager.io
for d in cert-manager{,-cainjector,-webhook}; do
    kubectl -n=cert-manager rollout status --timeout=5m "deployment.apps/${d}"
done
wait-for-object-creation cert-manager secret/cert-manager-webhook-ca

cat > "${ARTIFACTS_DEPLOY_DIR}/operator/kustomization.yaml" << EOF
resources:
- ${source_url}/${revision}/deploy/operator.yaml
patches:
- patch: |-
    apiVersion: apps/v1
    kind: Deployment
    metadata:
      name: scylla-operator
      namespace: scylla-operator
    spec:
      template:
        spec:
          containers:
          - name: scylla-operator
            image: "${operator_image_ref}"
            env:
            - name: SCYLLA_OPERATOR_IMAGE
              value: "${operator_image_ref}"
EOF
kubectl kustomize "${ARTIFACTS_DEPLOY_DIR}/operator" | kubectl_create -n=scylla-operator -f=-

# Manager needs scylla CRD registered and the webhook running
kubectl wait --for condition=established crd/{scyllaclusters,nodeconfigs}.scylla.scylladb.com
kubectl -n=scylla-operator rollout status --timeout=5m deployment.apps/scylla-operator
kubectl -n=scylla-operator rollout status --timeout=5m deployment.apps/webhook-server

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

if [[ -z "${SO_SCYLLACLUSTER_STORAGECLASS_NAME+x}" ]]; then
  kubectl_create -n=scylla-manager -f="${source_url}/${revision}/deploy/manager-prod.yaml"
elif [[ -n "${SO_SCYLLACLUSTER_STORAGECLASS_NAME}" ]]; then
  cat > "${ARTIFACTS_DEPLOY_DIR}/manager/kustomization.yaml" << EOF
resources:
- ${source_url}/${revision}/deploy/manager-prod.yaml
patches:
- target:
    group: scylla.scylladb.com
    version: v1
    kind: ScyllaCluster
    name: scylla-manager-cluster
  patch: |
    - op: replace
      path: /spec/datacenter/racks/0/storage/storageClassName
      value: "${SO_SCYLLACLUSTER_STORAGECLASS_NAME}"
EOF
  kubectl kustomize "${ARTIFACTS_DEPLOY_DIR}/manager" | kubectl_create -n=scylla-manager -f=-
else
  cat > "${ARTIFACTS_DEPLOY_DIR}/manager/kustomization.yaml" << EOF
resources:
- ${source_url}/${revision}/deploy/manager-prod.yaml
patches:
- target:
    group: scylla.scylladb.com
    version: v1
    kind: ScyllaCluster
    name: scylla-manager-cluster
  patch: |
    - op: remove
      path: /spec/datacenter/racks/0/storage/storageClassName
EOF
  kubectl kustomize "${ARTIFACTS_DEPLOY_DIR}/manager" | kubectl_create -n=scylla-manager -f=-
fi

kubectl -n=scylla-manager wait --timeout=5m --for='condition=Progressing=False' scyllaclusters.scylla.scylladb.com/scylla-manager-cluster
kubectl -n=scylla-manager wait --timeout=5m --for='condition=Degraded=False' scyllaclusters.scylla.scylladb.com/scylla-manager-cluster
kubectl -n=scylla-manager wait --timeout=5m --for='condition=Available=True' scyllaclusters.scylla.scylladb.com/scylla-manager-cluster
kubectl -n=scylla-manager rollout status --timeout=5m deployment.apps/scylla-manager
kubectl -n=scylla-manager rollout status --timeout=5m deployment.apps/scylla-manager-controller

kubectl -n=haproxy-ingress rollout status --timeout=5m deployment.apps/haproxy-ingress

kubectl wait --for condition=established crd/{scyllaoperatorconfigs,scylladbmonitorings}.scylla.scylladb.com
kubectl wait --for condition=established crd/{prometheuses,prometheusrules,servicemonitors}.monitoring.coreos.com
