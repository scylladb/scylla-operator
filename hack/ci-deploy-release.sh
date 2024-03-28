#!/bin/bash
#
# Copyright (C) 2024 ScyllaDB
#
# This script deploys scylla-operator stack.
# Usage: ${0} <operator_image_ref>
# (Avoid using rolling tags.)

set -euxEo pipefail
shopt -s inherit_errexit

function wait-for-object-creation {
    for i in {1..30}; do
        { kubectl -n "${1}" get "${2}" && break; } || sleep 1
    done
}

if [[ -n ${1+x} ]]; then
    operator_image_ref=${1}
else
    echo "Missing operator image ref.\nUsage: ${0} <operator_image_ref>" >&2 >/dev/null
    exit 1
fi

function kubectl_create {
    if [[ -z ${REENTRANT+x} ]]; then
        # In an actual CI run we have to enforce that no two objects have the same name.
        kubectl create "$@"
    else
        # For development iterations we want to update the objects.
        kubectl apply --server-side=true --force-conflicts "$@"
    fi
}

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

ARTIFACTS_DIR=${ARTIFACTS_DIR:-$( mktemp -d )}

kubectl_create -n prometheus-operator -f "${source_url}/${revision}/examples/third-party/prometheus-operator.yaml"
kubectl_create -n haproxy-ingress -f "${source_url}/${revision}/examples/third-party/haproxy-ingress.yaml"

kubectl_create -f "${source_url}/${revision}/examples/common/cert-manager.yaml"
# Wait for cert-manager crd and webhooks
kubectl wait --for condition=established --timeout=60s crd/certificates.cert-manager.io crd/issuers.cert-manager.io
for d in cert-manager{,-cainjector,-webhook}; do
    kubectl -n cert-manager rollout status --timeout=5m "deployment.apps/${d}"
done
wait-for-object-creation cert-manager secret/cert-manager-webhook-ca

mkdir "${ARTIFACTS_DIR}/operator"
cat > "${ARTIFACTS_DIR}/operator/kustomization.yaml" << EOF
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
kubectl kustomize "${ARTIFACTS_DIR}/operator" | kubectl_create -n scylla-operator -f -

# Manager needs scylla CRD registered and the webhook running
kubectl wait --for condition=established crd/{scyllaclusters,nodeconfigs}.scylla.scylladb.com
kubectl -n scylla-operator rollout status --timeout=5m deployment.apps/scylla-operator
kubectl -n scylla-operator rollout status --timeout=5m deployment.apps/webhook-server

if [[ -n "${NODECONFIG_PATH+x}" ]]; then
    kubectl_create -f="${NODECONFIG_PATH}"
fi
kubectl_create -n local-csi-driver -f"${source_url}/${revision}/examples/common/local-volume-provisioner/local-csi-driver/"{00_namespace.yaml,00_scylladb-local-xfs.storageclass.yaml,10_csidriver.yaml,10_driver.serviceaccount.yaml,10_provisioner_clusterrole.yaml,20_provisioner_clusterrolebinding.yaml,50_daemonset.yaml}

mkdir "${ARTIFACTS_DIR}/manager"
cat > "${ARTIFACTS_DIR}/manager/kustomization.yaml" << EOF
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
      value: scylladb-local-xfs
EOF
kubectl kustomize "${ARTIFACTS_DIR}/manager" | kubectl_create -n scylla-manager -f -

wait-for-object-creation scylla-manager statefulset.apps/scylla-manager-cluster-manager-dc-manager-rack
kubectl -n scylla-manager rollout status --timeout=5m statefulset.apps/scylla-manager-cluster-manager-dc-manager-rack
kubectl -n scylla-manager rollout status --timeout=5m deployment.apps/scylla-manager
kubectl -n scylla-manager rollout status --timeout=5m deployment.apps/scylla-manager-controller

kubectl -n haproxy-ingress rollout status --timeout=5m deployment.apps/haproxy-ingress

kubectl wait --for condition=established crd/{scyllaoperatorconfigs,scylladbmonitorings}.scylla.scylladb.com
kubectl wait --for condition=established crd/{prometheuses,prometheusrules,servicemonitors}.monitoring.coreos.com
