#!/bin/bash
#
# Copyright (C) 2025 ScyllaDB
#

set -euExo pipefail
shopt -s inherit_errexit

source "$( dirname "${BASH_SOURCE[0]}" )/bash.sh"
source "$( dirname "${BASH_SOURCE[0]}" )/kube.sh"

# install-operator installs the Scylla Operator and its dependencies in the cluster through the specified method.
# $1 - root source path to use. It can either be an absolute file path or a URL.
function install-operator() {
  if [[ "$#" -ne 1 ]]; then
    echo "Missing arguments.\nUsage: ${FUNCNAME[0]} <source_root>" > /dev/stderr
    exit 1
  fi

  SOURCE_ROOT="$1"
  export SOURCE_ROOT

  if [ -z "${ARTIFACTS_DEPLOY_DIR+x}" ]; then
    echo "ARTIFACTS_DEPLOY_DIR must be set" > /dev/stderr
    exit 1
  fi

  SO_QPS="${SO_QPS:-200}"
  export SO_QPS
  SO_BURST="${SO_BURST:-400}"
  export SO_BURST

  # TODO: Replace it with ScyllaOperatorConfig field when available.
  # Ref: https://github.com/scylladb/scylla-operator/issues/2314.
  SO_SCYLLA_OPERATOR_LOGLEVEL="${SO_SCYLLA_OPERATOR_LOGLEVEL:-4}"
  export SO_SCYLLA_OPERATOR_LOGLEVEL

  SO_CRYPTO_KEY_SIZE="${SO_CRYPTO_KEY_SIZE:-2048}"
  export SO_CRYPTO_KEY_SIZE
  SO_CRYPTO_KEY_BUFFER_DELAY="${SO_CRYPTO_KEY_BUFFER_DELAY:-2s}"
  export SO_CRYPTO_KEY_BUFFER_DELAY

  SO_CRYPTO_KEY_BUFFER_SIZE_MIN="${SO_CRYPTO_KEY_BUFFER_SIZE_MIN:-6}"
  export SO_CRYPTO_KEY_BUFFER_SIZE_MIN
  SO_CRYPTO_KEY_BUFFER_SIZE_MAX="${SO_CRYPTO_KEY_BUFFER_SIZE_MAX:-10}"
  export SO_CRYPTO_KEY_BUFFER_SIZE_MAX

  SCYLLA_OPERATOR_FEATURE_GATES="${SCYLLA_OPERATOR_FEATURE_GATES:-AllAlpha=true,AllBeta=true}"
  export SCYLLA_OPERATOR_FEATURE_GATES

  _install-operator-manifests
}

function _install-operator-manifests() {
  if [ -z "${OPERATOR_IMAGE_REF+x}" ]; then
    echo "OPERATOR_IMAGE_REF must be set" > /dev/stderr
    exit 1
  fi

  kubectl_create -f="${SOURCE_ROOT}/examples/third-party/cert-manager.yaml"

  # Wait for cert-manager crd and webhooks
  kubectl wait --for condition=established --timeout=60s crd/certificates.cert-manager.io crd/issuers.cert-manager.io
  for d in cert-manager{,-cainjector,-webhook}; do
      kubectl -n cert-manager rollout status --timeout=5m deployment.apps/"${d}"
  done
  wait-for-object-creation cert-manager secret/cert-manager-webhook-ca

  mkdir -p "${ARTIFACTS_DEPLOY_DIR}"/operator
  cat > "${ARTIFACTS_DEPLOY_DIR}/operator/kustomization.yaml" << EOF
resources:
- ${SOURCE_ROOT}/deploy/operator.yaml
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
            image: "${OPERATOR_IMAGE_REF}"
            env:
            - name: SCYLLA_OPERATOR_IMAGE
              value: "${OPERATOR_IMAGE_REF}"
- patch: |
    - op: add
      path: /spec/template/spec/containers/0/args/-
      value: "--loglevel=${SO_SCYLLA_OPERATOR_LOGLEVEL}"
    - op: add
      path: /spec/template/spec/containers/0/args/-
      value: "--crypto-key-size=${SO_CRYPTO_KEY_SIZE}"
    - op: add
      path: /spec/template/spec/containers/0/args/-
      value: "--crypto-key-buffer-size-min=${SO_CRYPTO_KEY_BUFFER_SIZE_MIN}"
    - op: add
      path: /spec/template/spec/containers/0/args/-
      value: "--crypto-key-buffer-size-max=${SO_CRYPTO_KEY_BUFFER_SIZE_MAX}"
    - op: add
      path: /spec/template/spec/containers/0/args/-
      value: "--crypto-key-buffer-delay=${SO_CRYPTO_KEY_BUFFER_DELAY}"
    - op: add
      path: /spec/template/spec/containers/0/args/-
      value: "--qps=${SO_QPS}"
    - op: add
      path: /spec/template/spec/containers/0/args/-
      value: "--burst=${SO_BURST}"
    - op: add
      path: /spec/template/spec/containers/0/args/-
      value: "--feature-gates=${SCYLLA_OPERATOR_FEATURE_GATES}"
  target:
      group: apps
      version: v1
      kind: Deployment
      name: scylla-operator
EOF

  kubectl kustomize --load-restrictor=LoadRestrictionsNone "${ARTIFACTS_DEPLOY_DIR}/operator" | kubectl_create -n=scylla-operator -f=-
}
