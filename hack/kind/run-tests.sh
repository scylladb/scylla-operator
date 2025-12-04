#!/usr/bin/env bash

set -euo pipefail

# This script runs a single E2E test suite inside a KinD cluster.
# Usage: ./run-test.sh <test-suite-name>
# Example:
# SO_IMAGE="docker.io/czeslavo/scylla-operator:46c4d5a449fb4750a2f988dba70cf2e8a2797809" \
#   ./hack/kind/run-tests.sh kind-parallel

BUILD_OPERATOR=${BUILD_OPERATOR:-""}
USE_SUDO=${USE_SUDO:-""}
TEST_SUITE_NAME="$1"
SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
REPO_ROOT_DIR=$(cd "${SCRIPT_DIR}/../.." && pwd)
THIRD_PARTY_MANIFESTS=($(find "${REPO_ROOT_DIR}/examples/third-party/" -name "*.yaml"))
SO_IMAGE=${SO_IMAGE:-"scylladb/scylla-operator:latest"}
SCYLLA_OPERATOR_FEATURE_GATES="${SCYLLA_OPERATOR_FEATURE_GATES:-AllAlpha=true,AllBeta=true}"
SO_E2E_TIMEOUT="${SO_E2E_TIMEOUT:-120m}"
SO_E2E_PARALLELISM="${SO_E2E_PARALLELISM:-5}"
SO_SCYLLACLUSTER_STORAGECLASS_NAME="${SO_SCYLLACLUSTER_STORAGECLASS_NAME:-standard}"
KIND_CLUSTER_NAME="${KIND_CLUSTER_NAME:-scylla-operator-e2e}"
USE_EXISTING_CLUSTER="${USE_EXISTING_CLUSTER:-""}"

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

source "${SCRIPT_DIR}/images.sh"
source "${SCRIPT_DIR}/../lib/kube.sh"

KIND_BIN="kind"
if [[ -n "${USE_SUDO}" ]]; then
    KIND_BIN="sudo ${KIND_BIN}"
fi

function build_operator_image() {
    pushd "${REPO_ROOT_DIR}" > /dev/null

    git_rev="$(git rev-parse HEAD)"
    image_ref="docker.io/scylladb/scylla-operator:${git_rev}"
    podman build -t "${image_ref}" . > /dev/null 2>&1

    popd > /dev/null
    echo "${image_ref}"
}

# create a kind cluster
function create_kind_cluster() {
    echo "Creating kind IPv4-only network..."
    if ! podman network inspect kind >/dev/null 2>&1; then
        podman network create kind
    fi

    echo "Creating KinD cluster named ${KIND_CLUSTER_NAME}..."
    ${KIND_BIN} create cluster --name "${KIND_CLUSTER_NAME}" --config "${SCRIPT_DIR}/kind-config.yaml"
}

function ensure_kind_cluster() {
    if [[ -z "${USE_EXISTING_CLUSTER}" ]]; then
        # Delete existing KinD cluster if it exists
        if ${KIND_BIN} get clusters | grep -q "^${KIND_CLUSTER_NAME}$"; then
            echo "Deleting existing KinD cluster named ${KIND_CLUSTER_NAME}..."
            ${KIND_BIN} delete cluster --name "${KIND_CLUSTER_NAME}"
        fi

        create_kind_cluster
    else
        echo "Using existing KinD cluster named ${KIND_CLUSTER_NAME}..."
    fi
}

function cleanup() {
    if [[ "${USE_EXISTING_CLUSTER}" != "--use-existing-cluster" ]]; then
        echo "Deleting KinD cluster named ${KIND_CLUSTER_NAME}..."
        ${KIND_BIN} delete cluster --name "${KIND_CLUSTER_NAME}"
        rm -f "${KUBECONFIG}"
    fi
}

function ensure_images_loaded() {
  echo "Ensuring required Docker images are loaded into KinD cluster..."
  tmpdir=$(mktemp -d)
  third_party_images=($(extract_images_from_manifests "${THIRD_PARTY_MANIFESTS[@]}"))

  for image in "${third_party_images[@]}"; do
      if ! ${KIND_BIN} load docker-image "${image}" --name "${KIND_CLUSTER_NAME}"; then
          echo "Pulling image ${image} from Docker Hub..."
          podman pull "${image}" || true # Ignore pull errors as image may be local only.
          image_tar="${tmpdir}/${image//[:\/]/_}.tar"
          podman save -o "${image_tar}" "${image}"
          ${KIND_BIN} load image-archive "${image_tar}" --name "${KIND_CLUSTER_NAME}"
      fi
  done
  # TODO: load all images used in tests
  image_tar="${tmpdir}/scylla-operator.tar"
  podman pull "${SO_IMAGE}" || true # Ignore pull errors as image may be local only.
  podman save -o "${image_tar}" "${SO_IMAGE}"
  ${KIND_BIN} load image-archive "${image_tar}" --name "${KIND_CLUSTER_NAME}"
}

function create_e2e_namespace() {
    echo "Creating e2e namespace..."
    kubectl create ns e2e --dry-run=client -o yaml | kubectl apply -f -
}

# prepare_kubeconfig creates a temporary kubeconfig file that can be used to access the cluster from host.
function prepare_kubeconfig() {
    local kubeconfig
    kubeconfig=$(mktemp)

    ${KIND_BIN} get kubeconfig --name="${KIND_CLUSTER_NAME}" > "${kubeconfig}"

    echo "${kubeconfig}"
}

# prepare_internal_kubeconfig creates a temporary kubeconfig file that can be used from within the cluster.
function prepare_internal_kubeconfig() {
    local incluster_kubeconfig
    incluster_kubeconfig=$(mktemp)

    ${KIND_BIN} get kubeconfig --name="${KIND_CLUSTER_NAME}" --internal > "${incluster_kubeconfig}"

    echo "${incluster_kubeconfig}"
}

function create_kubeconfig_secret() {
    echo "Preparing kubeconfig for in-cluster use..."
    local incluster_kubeconfig
    incluster_kubeconfig="$(prepare_internal_kubeconfig)"

    echo "Creating kubeconfig secret..."
    kubectl create -n=e2e secret generic kubeconfig --from-file=kubeconfig="${incluster_kubeconfig}" --dry-run=client -o=yaml | kubectl apply -f -
}

function install_dependencies() {
    echo "Installing dependencies..."
    install_cert_manager
    install_prometheus_operator
    install_local_csi_driver
}

function install_cert_manager() {
    echo "Installing cert-manager..."
    kubectl apply --server-side -f="${REPO_ROOT_DIR}/examples/third-party/cert-manager.yaml"
    kubectl wait --for condition=established --timeout=60s crd/certificates.cert-manager.io crd/issuers.cert-manager.io
    for deploy in cert-manager{,-cainjector,-webhook}; do
        kubectl -n=cert-manager rollout status --timeout=10m deployment.apps/"${deploy}"
    done
    for i in {1..30}; do
        { kubectl -n=cert-manager get secret/cert-manager-webhook-ca && break; } || sleep 1
    done
}

function install_prometheus_operator() {
    echo "Installing Prometheus Operator..."
    kubectl apply --server-side -f="${REPO_ROOT_DIR}/examples/third-party/prometheus-operator.yaml"
    kubectl wait --for condition=established --timeout=60s crd/prometheuses.monitoring.coreos.com crd/prometheusrules.monitoring.coreos.com crd/servicemonitors.monitoring.coreos.com
    kubectl -n=prometheus-operator rollout status --timeout=10m deployment.apps/prometheus-operator
}

function install_scylla_operator() {
    echo "Installing Scylla Operator..."
    sed "s|docker.io/scylladb/scylla-operator:latest|${SO_IMAGE}|g" "${REPO_ROOT_DIR}/deploy/operator.yaml" | \
      kubectl -n=scylla-operator apply --server-side -f=-
    kubectl wait --for='condition=established' crd/scyllaclusters.scylla.scylladb.com crd/nodeconfigs.scylla.scylladb.com crd/scyllaoperatorconfigs.scylla.scylladb.com crd/scylladbmonitorings.scylla.scylladb.com
    kubectl -n=scylla-operator rollout status --timeout=10m deployment.apps/{scylla-operator,webhook-server}
}

function install_scylla_manager() {
    echo "Installing ScyllaDB Manager..."
    kustomize build "${REPO_ROOT_DIR}/deploy/manager/kind" | kubectl -n=scylla-manager apply --server-side -f=-
    kubectl -n=scylla-manager wait --timeout=10m --for='condition=Progressing=False' scyllaclusters.scylla.scylladb.com/scylla-manager-cluster
    kubectl -n=scylla-manager wait --timeout=10m --for='condition=Degraded=False' scyllaclusters.scylla.scylladb.com/scylla-manager-cluster
    kubectl -n=scylla-manager wait --timeout=10m --for='condition=Available=True' scyllaclusters.scylla.scylladb.com/scylla-manager-cluster
    kubectl -n=scylla-manager rollout status --timeout=10m deployment.apps/scylla-manager
}

function install_ha_proxy_ingress() {
  echo "Installing HAProxy Ingress Controller..."
  kubectl apply -n=haproxy-ingress --server-side -f="${REPO_ROOT_DIR}/examples/third-party/haproxy-ingress"
  kubectl -n=haproxy-ingress rollout status --timeout=5m deployment.apps/haproxy-ingress
}

function run_test_suite_in_pod() {
    local suite_name=$1
    echo "Running test suite: ${suite_name}..."

    ingress_class_name='haproxy'
    ingress_custom_annotations='haproxy.org/ssl-passthrough=true,route.openshift.io/termination=passthrough'
    ingress_controller_address="$( kubectl -n=haproxy-ingress get svc haproxy-ingress --template='{{ .spec.clusterIP }}' ):9142"

    e2e_command_args=(
      # "--skip=${SO_SKIPPED_TESTS}"
      "--kubeconfig=/var/run/secrets/kubeconfig"
      "--loglevel=5"
      "--color=false"
      "--artifacts-dir=/tmp/artifacts"
      "--ingress-controller-address=${ingress_controller_address}"
      "--ingress-controller-ingress-class-name=${ingress_class_name}"
      "--ingress-controller-custom-annotations=${ingress_custom_annotations}"
      "--parallelism=${SO_E2E_PARALLELISM}"
      "--timeout=${SO_E2E_TIMEOUT}"
      "--feature-gates=${SCYLLA_OPERATOR_FEATURE_GATES}"
      "--scyllacluster-node-service-type=ClusterIP"
      "--scyllacluster-nodes-broadcast-address-type=PodIP"
      "--scyllacluster-clients-broadcast-address-type=PodIP"
      "--scyllacluster-storageclass-name=${SO_SCYLLACLUSTER_STORAGECLASS_NAME}"
      "--crypto-key-size=${SO_CRYPTO_KEY_SIZE}"
      "--crypto-key-buffer-size-min=${SO_CRYPTO_KEY_BUFFER_SIZE_MIN}"
      "--crypto-key-buffer-size-max=${SO_CRYPTO_KEY_BUFFER_SIZE_MAX}"

    )

    kubectl_create -n=e2e -f=- <<EOF
apiVersion: v1
kind: Pod
metadata:
  labels:
    app: e2e
  name: e2e
spec:
  restartPolicy: Never
  containers:
  - name: wait-for-artifacts
    command:
    - /usr/bin/sleep
    - infinity
    image: "${SO_IMAGE}"
    volumeMounts:
    - name: artifacts
      mountPath: /tmp/artifacts
  - name: e2e
    command:
    - scylla-operator-tests
    - run
    - "${suite_name}"
$(printf '    - "%s"\n' "${e2e_command_args[@]}")
    image: "${SO_IMAGE}"
    volumeMounts:
    - name: artifacts
      mountPath: /tmp/artifacts
    - name: kubeconfig
      mountPath: /var/run/secrets/kubeconfig
      subPath: kubeconfig
      readOnly: true
  volumes:
  - name: artifacts
    emptyDir: {}
  - name: kubeconfig
    secret:
      secretName: kubeconfig
      items:
      - key: kubeconfig
        path: kubeconfig
EOF
    echo "Testing pod created. Waiting for completion..."
    wait-for-object-creation e2e pod/e2e
    exit_code=$(wait-for-container-exit-with-logs e2e e2e e2e)
    echo "Test suite completed with exit code ${exit_code}."

    kubectl_cp -n=e2e e2e:/tmp/artifacts -c=wait-for-artifacts "${ARTIFACTS}"
    ls -l "${ARTIFACTS}"

    kubectl -n=e2e delete pod/e2e --wait=false

    if [[ "${exit_code}" != "0" ]]; then
      echo "E2E tests failed"
      exit "${exit_code}"
    fi

    kubectl_cp e2e/e2e:/tmp/artifacts "./artifacts/${suite_name}" || true
}

if [[ -n "${BUILD_OPERATOR}" ]]; then
    echo "Building Scylla Operator image..."
    SO_IMAGE=$(build_operator_image)
    echo "Built Scylla Operator image: ${SO_IMAGE}"
fi

kind_start_time=$(date +%s)
ensure_kind_cluster
kind_end_time=$(date +%s)
# TODO: verify why it doesn't work in CI
ensure_images_loaded
KUBECONFIG=$(prepare_kubeconfig)

env_setup_start_time=$(date +%s)
create_e2e_namespace
create_kubeconfig_secret

install_cert_manager
install_prometheus_operator
install_scylla_operator
install_scylla_manager
install_ha_proxy_ingress
env_setup_end_time=$(date +%s)

tests_run_start_time=$(date +%s)
run_test_suite_in_pod "${TEST_SUITE_NAME}"
tests_run_end_time=$(date +%s)

echo "KinD cluster setup time: $(( kind_end_time - kind_start_time )) seconds"
echo "Environment setup time: $(( env_setup_end_time - env_setup_start_time )) seconds"
echo "Tests run time: $(( tests_run_end_time - tests_run_start_time )) seconds"

# TODOs:
# - Add ingress controller installation and configuration
# - Add support for multi-DC tests by configuring multiple worker kubeconfigs
