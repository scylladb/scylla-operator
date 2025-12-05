#!/bin/bash
#
# Copyright (C) 2025 ScyllaDB
#

set -euExo pipefail
shopt -s inherit_errexit

source "$( dirname "${BASH_SOURCE[0]}" )/bash.sh"
source "$( dirname "${BASH_SOURCE[0]}" )/kube.sh"
parent_dir="$( dirname "${BASH_SOURCE[0]}" )"

# TODO: make paths relative to parent dir
# TODO: move qps, burst, delay to vars
# TODO: switch manifests to kustomize? Share with release path?

function install-operator() {
  if [ -z "${ARTIFACTS_DEPLOY_DIR+x}" ]; then
    echo "ARTIFACTS_DEPLOY_DIR must be set" > /dev/stderr
    exit 1
  fi

  # TODO: Replace it with ScyllaOperatorConfig field when available.
  # Ref: https://github.com/scylladb/scylla-operator/issues/2314.
  SO_SCYLLA_OPERATOR_LOGLEVEL="${SO_SCYLLA_OPERATOR_LOGLEVEL:-4}"
  export SO_SCYLLA_OPERATOR_LOGLEVEL

  crypto_key_buffer_size_multiplier=1
  if [[ -n "${SO_E2E_PARALLELISM:-}" && "${SO_E2E_PARALLELISM}" -ne 0 ]]; then
    crypto_key_buffer_size_multiplier="${SO_E2E_PARALLELISM}"
  fi

  SO_CRYPTO_KEY_SIZE="${SO_CRYPTO_KEY_SIZE:-2048}"
  export SO_CRYPTO_KEY_SIZE
  SO_CRYPTO_KEY_BUFFER_SIZE_MIN="${SO_CRYPTO_KEY_BUFFER_SIZE_MIN:-$(( 6 * "${crypto_key_buffer_size_multiplier}" ))}"
  export SO_CRYPTO_KEY_BUFFER_SIZE_MIN
  SO_CRYPTO_KEY_BUFFER_SIZE_MAX="${SO_CRYPTO_KEY_BUFFER_SIZE_MAX:-$(( 10 * "${crypto_key_buffer_size_multiplier}" ))}"
  export SO_CRYPTO_KEY_BUFFER_SIZE_MAX
  SO_CRYPTO_KEY_BUFFER_DELAY="${SO_CRYPTO_KEY_BUFFER_DELAY:-2s}"
  export SO_CRYPTO_KEY_BUFFER_DELAY

  SCYLLA_OPERATOR_FEATURE_GATES="${SCYLLA_OPERATOR_FEATURE_GATES:-AllAlpha=true,AllBeta=true}"
  export SCYLLA_OPERATOR_FEATURE_GATES

  SO_QPS="${SO_QPS:-200}"
  export SO_QPS
  SO_BURST="${SO_BURST:-400}"
  export SO_BURST

  case "${SO_SCYLLA_OPERATOR_INSTALL_MODE:-}" in
      "manifests")
          _install-operator-manifests
          ;;
      "olm")
          _install-operator-olm
          ;;
      *)
          echo "SO_SCYLLA_OPERATOR_INSTALL_MODE must be set to 'manifests' or 'olm'"
          exit 1
          ;;
  esac
}

function _install-operator-manifests() {
  if [ -z "${OPERATOR_IMAGE_REF+x}" ]; then
    echo "OPERATOR_IMAGE_REF must be set" > /dev/stderr
    exit 1
  fi

  mkdir -p "${ARTIFACTS_DEPLOY_DIR}"/operator

  cp ./deploy/operator/*.yaml "${ARTIFACTS_DEPLOY_DIR}/operator"
  cp ./examples/third-party/cert-manager.yaml "${ARTIFACTS_DEPLOY_DIR}/"

  for f in $( find "${ARTIFACTS_DEPLOY_DIR}"/ -type f -name '*.yaml' ); do
      sed -i -E -e "s~docker\.io/scylladb/scylla-operator:[^ @]+$~${OPERATOR_IMAGE_REF}~" "${f}"
  done

  yq e --inplace '.spec.template.spec.containers[0].args += "--loglevel=" + env(SO_SCYLLA_OPERATOR_LOGLEVEL)' "${ARTIFACTS_DEPLOY_DIR}/operator/50_operator.deployment.yaml"
  yq e --inplace '.spec.template.spec.containers[0].args += ["--qps="+env(SO_QPS), "--burst="+env(SO_BURST)]' "${ARTIFACTS_DEPLOY_DIR}/operator/50_operator.deployment.yaml"
  yq e --inplace '.spec.template.spec.containers[0].args += ["--crypto-key-size="+env(SO_CRYPTO_KEY_SIZE), "--crypto-key-buffer-size-min="+env(SO_CRYPTO_KEY_BUFFER_SIZE_MIN), "--crypto-key-buffer-size-max="+env(SO_CRYPTO_KEY_BUFFER_SIZE_MAX), "--crypto-key-buffer-delay="+env(SO_CRYPTO_KEY_BUFFER_DELAY)]' "${ARTIFACTS_DEPLOY_DIR}/operator/50_operator.deployment.yaml"
  yq e --inplace '.spec.template.spec.containers[0].args += "--feature-gates="+ env(SCYLLA_OPERATOR_FEATURE_GATES)' "${ARTIFACTS_DEPLOY_DIR}/operator/50_operator.deployment.yaml"

  kubectl_create -f "${ARTIFACTS_DEPLOY_DIR}"/cert-manager.yaml

  # Wait for cert-manager
  kubectl wait --for condition=established --timeout=60s crd/certificates.cert-manager.io crd/issuers.cert-manager.io
  for d in cert-manager{,-cainjector,-webhook}; do
      kubectl -n cert-manager rollout status --timeout=5m deployment.apps/"${d}"
  done
  wait-for-object-creation cert-manager secret/cert-manager-webhook-ca

  kubectl_create -f "${ARTIFACTS_DEPLOY_DIR}"/operator
}

function _install-operator-olm() {
  if [ -z "${SO_OLM_CATALOG_IMAGE_REF+x}" ]; then
    echo "SO_OLM_CATALOG_IMAGE_REF must be set" > /dev/stderr
    exit 1
  fi

  mkdir -p "${ARTIFACTS_DEPLOY_DIR}"/olm
  cp "$(realpath "${parent_dir}/../.ci/manifests/.olm/")"/*.yaml "${ARTIFACTS_DEPLOY_DIR}/olm/"

  cat > "${ARTIFACTS_DEPLOY_DIR}/olm/kustomization.yaml" << EOF
resources:
- 00_scylladb-operator.catalogsource.yaml
- 00_scylladb-operator.namespace.yaml
- 10_scylladb-operator.operatorgroup.yaml
- 50_scylladb-operator.subscription.yaml
patches:
- patch: |-
    apiVersion: operators.coreos.com/v1alpha1
    kind: CatalogSource
    metadata:
      name: scylladb-operator-catalog
      namespace: openshift-marketplace
    spec:
      image: "${SO_OLM_CATALOG_IMAGE_REF}"
- patch: |-
    apiVersion: operators.coreos.com/v1alpha1
    kind: Subscription
    metadata:
      name: scylladb-operator-subscription
      namespace: scylla-operator
    spec:
      config:
        env:
        # TODO: Fix loglevel flag reading
        - name: SCYLLA_OPERATOR_V
          value: "${SO_SCYLLA_OPERATOR_LOGLEVEL}"
        - name: SCYLLA_OPERATOR_CRYPTO_KEY_SIZE
          value: "${SO_CRYPTO_KEY_SIZE}"
        - name: SCYLLA_OPERATOR_CRYPTO_KEY_BUFFER_SIZE_MIN
          value: "${SO_CRYPTO_KEY_BUFFER_SIZE_MIN}"
        - name: SCYLLA_OPERATOR_CRYPTO_KEY_BUFFER_SIZE_MAX
          value: "${SO_CRYPTO_KEY_BUFFER_SIZE_MAX}"
        - name: SCYLLA_OPERATOR_CRYPTO_KEY_BUFFER_DELAY
          value: "${SO_CRYPTO_KEY_BUFFER_DELAY}"
        - name: SCYLLA_OPERATOR_QPS
          value: "${SO_QPS}"
        - name: SCYLLA_OPERATOR_BURST
          value: "${SO_BURST}"
        - name: SCYLLA_OPERATOR_FEATURE_GATES
          value: "${SCYLLA_OPERATOR_FEATURE_GATES}"
EOF

  kubectl kustomize "${ARTIFACTS_DEPLOY_DIR}/olm" | kubectl_create -f=-
}
