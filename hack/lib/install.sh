#!/usr/bin/env bash
#
# Copyright (C) 2025 ScyllaDB
#

set -euExo pipefail
shopt -s inherit_errexit

source "$( dirname "${BASH_SOURCE[0]}" )/bash.sh"
source "$( dirname "${BASH_SOURCE[0]}" )/kube.sh"
source "$( dirname "${BASH_SOURCE[0]}" )/assets.sh"

# deploy-cert-manager deploys cert-manager and waits until its webhook is actually callable.
# $1 - root source path to use. It can either be an absolute file path or a URL.
function deploy-cert-manager() {
  if [[ "$#" -ne 1 ]]; then
    echo "Missing arguments.\nUsage: ${FUNCNAME[0]} <source_root>" > /dev/stderr
    exit 1
  fi

  kubectl_create -f="${1}/examples/third-party/cert-manager.yaml"

  # Wait for cert-manager crd and webhooks
  kubectl wait --for condition=established --timeout=60s crd/certificates.cert-manager.io crd/issuers.cert-manager.io
  for d in cert-manager{,-cainjector,-webhook}; do
      kubectl -n cert-manager rollout status --timeout=5m deployment.apps/"${d}"
  done
  wait-for-object-creation cert-manager secret/cert-manager-webhook-ca
  # Rollouts and the CA secret existing don't yet mean the webhook is callable: cainjector
  # still has to sync the CA into the webhook's caBundle. Probe the real apiserver->webhook
  # path with a server-side dry-run until it works, so the first actual object can't hit
  # "x509: certificate signed by unknown authority".
  timeout 5m bash -c "until kubectl create --dry-run=server -f=- <<< '{\"apiVersion\":\"cert-manager.io/v1\",\"kind\":\"Issuer\",\"metadata\":{\"name\":\"webhook-probe\",\"namespace\":\"cert-manager\"},\"spec\":{\"selfSigned\":{}}}'; do sleep 1; done"
}

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

  SO_CRYPTO_KEY_BUFFER_DELAY="${SO_CRYPTO_KEY_BUFFER_DELAY:-2s}"
  export SO_CRYPTO_KEY_BUFFER_DELAY

  SO_CRYPTO_KEY_BUFFER_SIZE_MIN="${SO_CRYPTO_KEY_BUFFER_SIZE_MIN:-6}"
  export SO_CRYPTO_KEY_BUFFER_SIZE_MIN
  SO_CRYPTO_KEY_BUFFER_SIZE_MAX="${SO_CRYPTO_KEY_BUFFER_SIZE_MAX:-10}"
  export SO_CRYPTO_KEY_BUFFER_SIZE_MAX

  SCYLLA_OPERATOR_FEATURE_GATES="${SCYLLA_OPERATOR_FEATURE_GATES:-AllAlpha=true,AllBeta=true}"
  export SCYLLA_OPERATOR_FEATURE_GATES

  SO_INSTALL_MODE="${SO_INSTALL_MODE:-manifests}"
  case "${SO_INSTALL_MODE}" in
      "manifests")
          if [ -z "${OPERATOR_IMAGE_REF+x}" ]; then
            echo "OPERATOR_IMAGE_REF must be set when SO_INSTALL_MODE is 'manifests'" > /dev/stderr
            exit 1
          fi

          _install-operator-manifests
          ;;
      "helm")
          if [ -z "${OPERATOR_IMAGE_REF+x}" ]; then
            echo "OPERATOR_IMAGE_REF must be set when SO_INSTALL_MODE is 'helm'" > /dev/stderr
            exit 1
          fi

          _install-operator-helm
          ;;
      "olm")
          if [ -z "${SO_OLM_CATALOG_IMAGE_REF+x}" ]; then
              echo "SO_OLM_CATALOG_IMAGE_REF must be set when SO_INSTALL_MODE is 'olm'" > /dev/stderr
              exit 1
          fi

          _install-operator-olm
          ;;
      *)
          echo "SO_INSTALL_MODE must be set to 'manifests', 'helm' or 'olm'"
          exit 1
          ;;
  esac
}

function _install-operator-manifests() {
  if [ -z "${OPERATOR_IMAGE_REF+x}" ]; then
    echo "OPERATOR_IMAGE_REF must be set" > /dev/stderr
    exit 1
  fi

  deploy-cert-manager "${SOURCE_ROOT}"

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
- patch: |-
    apiVersion: apps/v1
    kind: Deployment
    metadata:
      name: webhook-server
      namespace: scylla-operator
    spec:
      template:
        spec:
          containers:
          - name: webhook-server
            image: "${OPERATOR_IMAGE_REF}"
- patch: |
    - op: add
      path: /spec/template/spec/containers/0/args/-
      value: "--loglevel=${SO_SCYLLA_OPERATOR_LOGLEVEL}"
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

function _install-operator-helm() {
  if [ -z "${OPERATOR_IMAGE_REF+x}" ]; then
    echo "OPERATOR_IMAGE_REF must be set" > /dev/stderr
    exit 1
  fi

  # The chart composes the image as "<repository>/scylla-operator:<tag>", so the ref has to be split around the
  # image name. A digest ref gets a placeholder tag, as "<name>:<tag>@<digest>" is pulled by the digest.
  local image_repository image_tag_or_digest
  image_repository="${OPERATOR_IMAGE_REF%%/scylla-operator[:@]*}"
  image_tag_or_digest="${OPERATOR_IMAGE_REF#"${image_repository}/scylla-operator"}"
  case "${image_tag_or_digest}" in
    :*)
      image_tag_or_digest="${image_tag_or_digest#:}"
      ;;
    @*)
      image_tag_or_digest="latest${image_tag_or_digest}"
      ;;
    *)
      echo "OPERATOR_IMAGE_REF must name a 'scylla-operator' image, got '${OPERATOR_IMAGE_REF}'" > /dev/stderr
      exit 1
      ;;
  esac

  deploy-cert-manager "${SOURCE_ROOT}"

  mkdir -p "${ARTIFACTS_DEPLOY_DIR}"/operator
  cat > "${ARTIFACTS_DEPLOY_DIR}/operator/values.yaml" << EOF
image:
  repository: "${image_repository}"
  tag: "${image_tag_or_digest}"
replicas: 1
webhookServerReplicas: 1
logLevel: ${SO_SCYLLA_OPERATOR_LOGLEVEL}
additionalArgs:
- "--crypto-key-buffer-size-min=${SO_CRYPTO_KEY_BUFFER_SIZE_MIN}"
- "--crypto-key-buffer-size-max=${SO_CRYPTO_KEY_BUFFER_SIZE_MAX}"
- "--crypto-key-buffer-delay=${SO_CRYPTO_KEY_BUFFER_DELAY}"
- "--qps=${SO_QPS}"
- "--burst=${SO_BURST}"
- "--feature-gates=${SCYLLA_OPERATOR_FEATURE_GATES}"
EOF

  helm upgrade --install scylla-operator "${SOURCE_ROOT}/helm/scylla-operator" \
    --create-namespace \
    --namespace=scylla-operator \
    --values="${ARTIFACTS_DEPLOY_DIR}/operator/values.yaml"
}

function _install-operator-olm() {
  if [ -z "${SO_OLM_CATALOG_IMAGE_REF+x}" ]; then
    echo "SO_OLM_CATALOG_IMAGE_REF must be set" > /dev/stderr
    exit 1
  fi

  mkdir -p "${ARTIFACTS_DEPLOY_DIR}"/olm
  cat > "${ARTIFACTS_DEPLOY_DIR}/olm/kustomization.yaml" << EOF
resources:
- ${SOURCE_ROOT}/hack/.ci/manifests/olm/00_scylladb-operator.catalogsource.yaml
- ${SOURCE_ROOT}/hack/.ci/manifests/olm/00_scylladb-operator.namespace.yaml
- ${SOURCE_ROOT}/hack/.ci/manifests/olm/10_scylladb-operator.operatorgroup.yaml
- ${SOURCE_ROOT}/hack/.ci/manifests/olm/50_scylladb-operator.subscription.yaml
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
      channel: $( get-metadata ".operatorTests.olm.channel" )
      config:
        env:
        # Setting SCYLLA_OPERATOR_V instead of SCYLLA_OPERATOR_LOGLEVEL is a dirty workaround to override the default LOGLEVEL being set in the manifests.
        # Normally, the flag takes precedence over the env var, so it is not possible to override it with SCYLLA_OPERATOR_LOGLEVEL.
        # However, the 'v' flag takes precedence over 'loglevel', so setting it through the env var overrides the default flag value.
        - name: SCYLLA_OPERATOR_V
          value: "${SO_SCYLLA_OPERATOR_LOGLEVEL}"
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

  kubectl kustomize --load-restrictor=LoadRestrictionsNone "${ARTIFACTS_DEPLOY_DIR}/olm" | kubectl_create -f=-
}

# validate-image-ref fails unless the image ref is tagged, optionally pinned by a digest. On success, BASH_REMATCH[1]
# holds the repository and BASH_REMATCH[4] the version.
# $1 - image ref, e.g. docker.io/scylladb/scylla:2026.1.0
function validate-image-ref() {
  if [[ ! "${1}" =~ ^([^:@]+(:[0-9]+)?(/[^:@]+)?):([A-Za-z0-9_][A-Za-z0-9_.-]{0,127}(@sha256:[0-9a-f]{64})?)$ ]]; then
    echo "Invalid image ref '${1}', expected <repository>:<tag>[@sha256:<digest>]" > /dev/stderr
    exit 1
  fi
}

# set-image-from-ref splits a tagged image ref, optionally pinned by a digest, into a repository and a version, and
# sets them in a manifest.
# $1 - image ref, e.g. docker.io/scylladb/scylla:2026.1.0
# $2 - yq path to set the repository at, e.g. .spec.repository
# $3 - yq path to set the version at, e.g. .spec.version
# $4 - manifest file
function set-image-from-ref() {
  if [[ "$#" -ne 4 ]]; then
    echo "Missing arguments.\nUsage: ${FUNCNAME[0]} <image_ref> <repository_path> <version_path> <file>" > /dev/stderr
    exit 1
  fi

  validate-image-ref "${1}"
  IMAGE_REPOSITORY="${BASH_REMATCH[1]}" \
  IMAGE_VERSION="${BASH_REMATCH[4]}" \
  yq e --inplace "${2} = strenv(IMAGE_REPOSITORY) | ${3} = strenv(IMAGE_VERSION)" "${4}"
}

# install-scylladb-manager installs ScyllaDB Manager with the CI overrides applied from the environment, through the
# method matching SO_INSTALL_MODE.
# $1 - root source path to use.
function install-scylladb-manager() {
  if [[ "$#" -ne 1 ]]; then
    echo "Missing arguments.\nUsage: ${FUNCNAME[0]} <source_root>" > /dev/stderr
    exit 1
  fi

  if [ -z "${ARTIFACTS_DEPLOY_DIR+x}" ]; then
    echo "ARTIFACTS_DEPLOY_DIR must be set" > /dev/stderr
    exit 1
  fi

  # Don't run the manager's ScyllaCluster with the production cpuset and resources, so it doesn't take an exclusive
  # core on the ScyllaDB nodes the e2e clusters share.
  local scyllacluster_cpuset="false"
  local scyllacluster_resources='{"limits": {"cpu": "200m", "memory": "200Mi"}, "requests": {"cpu": "10m", "memory": "100Mi"}}'

  local scyllacluster_scylla_args=""
  if [[ -n "${SO_SCYLLACLUSTER_REACTOR_BACKEND:-}" ]]; then
    scyllacluster_scylla_args="--reactor-backend=${SO_SCYLLACLUSTER_REACTOR_BACKEND}"
  fi

  case "${SO_INSTALL_MODE:-manifests}" in
      "manifests" | "olm")
          _install-scylladb-manager-manifests "${1}"
          ;;
      "helm")
          _install-scylladb-manager-helm "${1}"
          ;;
      *)
          echo "SO_INSTALL_MODE must be set to 'manifests', 'helm' or 'olm'"
          exit 1
          ;;
  esac
}

function _install-scylladb-manager-manifests() {
  mkdir -p "${ARTIFACTS_DEPLOY_DIR}/manager"
  cp "${1}"/deploy/manager/*.yaml "${ARTIFACTS_DEPLOY_DIR}/manager"

  yq e --inplace ".spec.cpuset = ${scyllacluster_cpuset} | .spec.datacenter.racks[0].resources = ${scyllacluster_resources}" "${ARTIFACTS_DEPLOY_DIR}/manager/50_scyllacluster.yaml"

  if [[ -n "${SO_SCYLLACLUSTER_STORAGECLASS_NAME:-}" ]]; then
    yq e --inplace '.spec.datacenter.racks[0].storage.storageClassName = env(SO_SCYLLACLUSTER_STORAGECLASS_NAME)' "${ARTIFACTS_DEPLOY_DIR}/manager/50_scyllacluster.yaml"
  elif [[ -n "${SO_SCYLLACLUSTER_STORAGECLASS_NAME+x}" ]]; then
    yq e --inplace 'del(.spec.datacenter.racks[0].storage.storageClassName)' "${ARTIFACTS_DEPLOY_DIR}/manager/50_scyllacluster.yaml"
  fi

  if [[ -n "${SCYLLADB_IMAGE_REF:-}" ]]; then
    set-image-from-ref "${SCYLLADB_IMAGE_REF}" '.spec.repository' '.spec.version' "${ARTIFACTS_DEPLOY_DIR}/manager/50_scyllacluster.yaml"
  fi

  if [[ -n "${SCYLLADB_MANAGER_IMAGE_REF:-}" ]]; then
    yq e --inplace '.spec.template.spec.containers[0].image = strenv(SCYLLADB_MANAGER_IMAGE_REF)' "${ARTIFACTS_DEPLOY_DIR}/manager/50_manager_deployment.yaml"
  fi

  if [[ -n "${SCYLLADB_MANAGER_AGENT_IMAGE_REF:-}" ]]; then
    set-image-from-ref "${SCYLLADB_MANAGER_AGENT_IMAGE_REF}" '.spec.agentRepository' '.spec.agentVersion' "${ARTIFACTS_DEPLOY_DIR}/manager/50_scyllacluster.yaml"
  fi

  if [[ -n "${scyllacluster_scylla_args}" ]]; then
    yq e --inplace ".spec.scyllaArgs = \"${scyllacluster_scylla_args}\"" "${ARTIFACTS_DEPLOY_DIR}/manager/50_scyllacluster.yaml"
  fi

  kubectl_create -f "${ARTIFACTS_DEPLOY_DIR}"/manager
}

function _install-scylladb-manager-helm() {
  mkdir -p "${ARTIFACTS_DEPLOY_DIR}/manager"
  local values="${ARTIFACTS_DEPLOY_DIR}/manager/values.yaml"
  cp "${1}/helm/deploy/manager_prod.yaml" "${values}"

  yq e --inplace ".scylla.cpuset = ${scyllacluster_cpuset} | .scylla.racks[0].resources = ${scyllacluster_resources}" "${values}"

  if [[ -n "${SO_SCYLLACLUSTER_STORAGECLASS_NAME:-}" ]]; then
    yq e --inplace '.scylla.racks[0].storage.storageClassName = env(SO_SCYLLACLUSTER_STORAGECLASS_NAME)' "${values}"
  elif [[ -n "${SO_SCYLLACLUSTER_STORAGECLASS_NAME+x}" ]]; then
    yq e --inplace 'del(.scylla.racks[0].storage.storageClassName)' "${values}"
  fi

  if [[ -n "${SCYLLADB_IMAGE_REF:-}" ]]; then
    set-image-from-ref "${SCYLLADB_IMAGE_REF}" '.scylla.scyllaImage.repository' '.scylla.scyllaImage.tag' "${values}"
  fi

  if [[ -n "${SCYLLADB_MANAGER_IMAGE_REF:-}" ]]; then
    set-image-from-ref "${SCYLLADB_MANAGER_IMAGE_REF}" '.image.repository' '.image.tag' "${values}"
    # The chart appends the image name to the repository.
    if [[ "$( yq e '.image.repository' "${values}" )" != */scylla-manager ]]; then
      echo "SCYLLADB_MANAGER_IMAGE_REF must point to a scylla-manager image with the helm install mode" > /dev/stderr
      exit 1
    fi
    yq e --inplace '.image.repository |= sub("/scylla-manager$"; "")' "${values}"
  fi

  if [[ -n "${SCYLLADB_MANAGER_AGENT_IMAGE_REF:-}" ]]; then
    set-image-from-ref "${SCYLLADB_MANAGER_AGENT_IMAGE_REF}" '.scylla.agentImage.repository' '.scylla.agentImage.tag' "${values}"
  fi

  if [[ -n "${scyllacluster_scylla_args}" ]]; then
    yq e --inplace ".scylla.scyllaArgs = \"${scyllacluster_scylla_args}\"" "${values}"
  fi

  helm upgrade --install scylla-manager "${1}/helm/scylla-manager" \
    --create-namespace \
    --namespace=scylla-manager \
    --values="${values}"
}
