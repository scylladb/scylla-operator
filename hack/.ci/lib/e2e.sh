#!/bin/bash
#
# Copyright (C) 2024 ScyllaDB
#

set -euExo pipefail
shopt -s inherit_errexit

source "$( dirname "${BASH_SOURCE[0]}" )/../../lib/bash.sh"
source "$( dirname "${BASH_SOURCE[0]}" )/../../lib/kube.sh"

# WORKER_KUBECONFIGS is an associative array that maps worker cluster identifiers to their kubeconfig paths.
# It is used in multi-datacenter setups.
declare -A WORKER_KUBECONFIGS

# WORKER_OBJECT_STORAGE_BUCKETS is an associative array that maps worker cluster identifiers to their object storage
# bucket names. It is used in multi-datacenter setups.
declare -A WORKER_OBJECT_STORAGE_BUCKETS

# WORKER_S3_CREDENTIALS_PATHS is an associative array that maps worker cluster identifiers to their S3 credentials
# file paths. It is used in multi-datacenter setups.
declare -A WORKER_S3_CREDENTIALS_PATHS

# WORKER_GCS_SERVICE_ACCOUNT_CREDENTIALS_PATHS is an associative array that maps worker cluster identifiers to their
# GCS service account credentials file paths. It is used in multi-datacenter setups.
declare -A WORKER_GCS_SERVICE_ACCOUNT_CREDENTIALS_PATHS

# KUBECONFIG is the kubeconfig file used to connect to the cluster.
# In multi-datacenter setups, it is the control plane cluster kubeconfig.
if [ -z "${KUBECONFIG+x}" ]; then
  echo "KUBECONFIG can't be empty" > /dev/stderr
  exit 2
fi

# run-deploy-script-in-all-clusters runs the deployment script in all clusters.
# In a single-datacenter setup, it deploys the operator stack in the only cluster using the KUBECONFIG environment variable.
# In a multi-datacenter setup, it runs in the control plane cluster using the KUBECONFIG environment variable, and in all worker clusters using the WORKER_KUBECONFIGS associative array.
# $1 - deployment script path
function run-deploy-script-in-all-clusters {
  if [ -z "${1+x}" ]; then
    echo -e "Missing deployment script path.\nUsage: ${FUNCNAME[0]} deployment_script_path" > /dev/stderr
    exit 2
  fi

  if [ -z "${SO_IMAGE+x}" ]; then
    echo "SO_IMAGE can't be empty" > /dev/stderr
    exit 2
  fi

  if [ -z "${ARTIFACTS+x}" ]; then
    echo "ARTIFACTS can't be empty" > /dev/stderr
    exit 2
  fi

  ARTIFACTS_DEPLOY_DIR="${ARTIFACTS}/deploy/cluster" \
  timeout --foreground -v 10m "${1}" "${SO_IMAGE}" &
  ci_deploy_bg_pids=( $! )

  for name in "${!WORKER_KUBECONFIGS[@]}"; do
    if [[ "${WORKER_KUBECONFIGS[$name]}" == "${KUBECONFIG}" ]]; then
      # Skip if the control plane cluster is also among the worker clusters.
      worker_as_control_plane="${name}"
      continue
    fi

    KUBECONFIG="${WORKER_KUBECONFIGS[$name]}" \
    ARTIFACTS_DEPLOY_DIR="${ARTIFACTS}/deploy/workers/${name}" \
    SO_DISABLE_SCYLLADB_MANAGER_DEPLOYMENT=true \
    timeout --foreground -v 10m "${1}" "${SO_IMAGE}" &
    ci_deploy_bg_pids+=( $! )
  done

  for pid in "${ci_deploy_bg_pids[@]}"; do
    wait "${pid}"
  done

  if [[ -n "${worker_as_control_plane+x}" ]]; then
    mkdir -p "${ARTIFACTS}/deploy/workers"
    cp -r ${REENTRANT:+-f} "${ARTIFACTS}/deploy/cluster" "${ARTIFACTS}/deploy/workers/${worker_as_control_plane}"
  fi
}

# gather-artifacts is a self sufficient function that collects artifacts without depending on any external objects.
# $1- target directory
function gather-artifacts {
  if [ -z "${1+x}" ]; then
    echo -e "Missing target directory.\nUsage: ${FUNCNAME[0]} target_directory" > /dev/stderr
    exit 2
  fi

  if [ -z "${SO_IMAGE+x}" ]; then
    echo "SO_IMAGE can't be empty" > /dev/stderr
    exit 2
  fi

  kubectl create namespace gather-artifacts --dry-run=client -o=yaml | kubectl_apply -f=-
  kubectl create clusterrolebinding gather-artifacts --clusterrole=cluster-admin --serviceaccount=gather-artifacts:default --dry-run=client -o=yaml | kubectl_apply -f=-
  kubectl create -n=gather-artifacts pdb must-gather --selector='app=must-gather' --max-unavailable=0 --dry-run=client -o=yaml | kubectl_apply -f=-

  kubectl_create -n=gather-artifacts -f=- <<EOF
apiVersion: v1
kind: Pod
metadata:
  labels:
    app: must-gather
  name: must-gather
spec:
  restartPolicy: Never
  containers:
  - name: wait-for-artifacts
    command:
    - /usr/bin/sleep
    - infinity
    image: "${SO_IMAGE}"
    imagePullPolicy: Always
    volumeMounts:
    - name: artifacts
      mountPath: /tmp/artifacts
  - name: must-gather
    args:
    - must-gather
    - --all-resources
    - --loglevel=2
    - --dest-dir=/tmp/artifacts
    image: "${SO_IMAGE}"
    imagePullPolicy: Always
    volumeMounts:
    - name: artifacts
      mountPath: /tmp/artifacts
  volumes:
  - name: artifacts
    emptyDir: {}
EOF
  kubectl -n=gather-artifacts wait --timeout=300s --for=condition=Ready pod/must-gather ||
    kubectl -n gather-artifacts describe pod must-gather && \
    kubectl -n gather-artifacts logs pod/must-gather --all-containers=true

  exit_code="$( wait-for-container-exit-with-logs gather-artifacts must-gather must-gather )"

  kubectl_cp -n=gather-artifacts -c=wait-for-artifacts must-gather:/tmp/artifacts "${1}"

  ls -l "${1}"

  kubectl -n=gather-artifacts delete pod/must-gather --wait=false

  if [[ "${exit_code}" -ne "0" ]]; then
    echo "Collecting artifacts using must-gather failed"
    exit "${exit_code}"
  fi
}

function gather-artifacts-on-exit {
  ec=$?

  gather-artifacts "${ARTIFACTS}/must-gather/cluster" &
  gather_artifacts_bg_pids=( $! )

  for name in "${!WORKER_KUBECONFIGS[@]}"; do
    if [[ "${WORKER_KUBECONFIGS[$name]}" == "${KUBECONFIG}" ]]; then
      # Skip if the control plane cluster is also among the worker clusters.
      worker_as_control_plane="${name}"
      continue
    fi

    KUBECONFIG="${WORKER_KUBECONFIGS[$name]}" gather-artifacts "${ARTIFACTS}/must-gather/workers/${name}" &
    gather_artifacts_bg_pids+=( $! )
  done

  for pid in "${gather_artifacts_bg_pids[@]}"; do
    wait "${pid}"
  done

  if [[ -n "${worker_as_control_plane+x}" ]]; then
    mkdir -p "${ARTIFACTS}/must-gather/workers"
    cp -r ${REENTRANT:+-f} "${ARTIFACTS}/must-gather/cluster" "${ARTIFACTS}/must-gather/workers/${worker_as_control_plane}"
  fi

  cleanup-bg-jobs "${ec}"
}

function gracefully-shutdown-e2es {
  kubectl -n e2e exec e2e -c e2e -- bash -euEo pipefail -O inherit_errexit -c 'kill -s SIGINT $(pidof scylla-operator-tests)' || true
  kubectl_cp -n=e2e e2e:/tmp/artifacts -c=wait-for-artifacts "${ARTIFACTS}" || true
}

function apply-e2e-workarounds-in-all-clusters {
  apply-e2e-workarounds

  for name in "${!WORKER_KUBECONFIGS[@]}"; do
    if [[ "${WORKER_KUBECONFIGS[$name]}" == "${KUBECONFIG}" ]]; then
      # Skip if the control plane cluster is also among the worker clusters.
      continue
    fi

    KUBECONFIG="${WORKER_KUBECONFIGS[$name]}" apply-e2e-workarounds
  done
}

function apply-e2e-workarounds {
  # Allow admin to use ephemeralcontainers
  kubectl_create -f=- <<EOF
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: scylladb-e2e:hotfixes
  labels:
    rbac.authorization.k8s.io/aggregate-to-admin: "true"
rules:
- apiGroups:
  - ""
  resources:
  - pods/ephemeralcontainers
  verbs:
  - patch
EOF
}

function run-e2e {
  if [ -z "${SO_SUITE+x}" ]; then
    echo "SO_SUITE can't be empty" > /dev/stderr
    exit 2
  fi

  if [ -z "${SO_IMAGE+x}" ]; then
    echo "SO_IMAGE can't be empty" > /dev/stderr
    exit 2
  fi

  if [ -z "${SO_SCYLLACLUSTER_NODE_SERVICE_TYPE+x}" ]; then
    echo "SO_SCYLLACLUSTER_NODE_SERVICE_TYPE can't be empty" > /dev/stderr
    exit 2
  fi

  if [ -z "${SO_SCYLLACLUSTER_NODES_BROADCAST_ADDRESS_TYPE+x}" ]; then
    echo "SO_SCYLLACLUSTER_NODES_BROADCAST_ADDRESS_TYPE can't be empty" > /dev/stderr
    exit 2
  fi

  if [ -z "${SO_SCYLLACLUSTER_CLIENTS_BROADCAST_ADDRESS_TYPE+x}" ]; then
    echo "SO_SCYLLACLUSTER_CLIENTS_BROADCAST_ADDRESS_TYPE can't be empty" > /dev/stderr
    exit 2
  fi

  if [ -z "${SO_SCYLLACLUSTER_STORAGECLASS_NAME+x}" ]; then
    echo "SO_SCYLLACLUSTER_STORAGECLASS_NAME can't be empty" > /dev/stderr
    exit 2
  fi

  if [ -z "${ARTIFACTS+x}" ]; then
    echo "ARTIFACTS can't be empty" > /dev/stderr
    exit 2
  fi

  SO_SKIPPED_TESTS="${SO_SKIPPED_TESTS:-}"
  FIELD_MANAGER="${FIELD_MANAGER:-run-e2e-script}"
  SO_BUCKET_NAME="${SO_BUCKET_NAME:-}"
  SO_E2E_PARALLELISM="${SO_E2E_PARALLELISM:-0}"
  SO_E2E_TIMEOUT="${SO_E2E_TIMEOUT:-24h}"
  SO_WAIT_FOR_E2E_POD_DELETE="${SO_WAIT_FOR_E2E_POD_DELETE:-false}"

  config_file="$(realpath "$(dirname "${BASH_SOURCE[0]}")/../../../assets/config/config.yaml")"
  SCYLLADB_VERSION="${SCYLLADB_VERSION:-$(yq '.operator.scyllaDBVersion' "$config_file")}"
  SCYLLADB_MANAGER_VERSION="${SCYLLADB_MANAGER_VERSION:-$(yq '.operator.scyllaDBManagerVersion' "$config_file")}"
  SCYLLADB_MANAGER_AGENT_VERSION="${SCYLLADB_MANAGER_AGENT_VERSION:-$(yq '.operator.scyllaDBManagerAgentVersion' "$config_file")}"
  SCYLLADB_UPDATE_FROM_VERSION="${SCYLLADB_UPDATE_FROM_VERSION:-$(yq '.operatorTests.scyllaDBVersions.updateFrom' "$config_file")}"
  SCYLLADB_UPGRADE_FROM_VERSION="${SCYLLADB_UPGRADE_FROM_VERSION:-$(yq '.operatorTests.scyllaDBVersions.upgradeFrom' "$config_file")}"

  kubectl create namespace e2e --dry-run=client -o=yaml | kubectl_create -f=-
  kubectl create clusterrolebinding e2e --clusterrole=cluster-admin --serviceaccount=e2e:default --dry-run=client -o=yaml | kubectl_create -f=-
  kubectl create -n=e2e pdb my-pdb --selector='app=e2e' --min-available=1 --dry-run=client -o=yaml | kubectl_create -f=-

  # Create a Secret with the main cluster's kubeconfig.
  # If there's IN_CLUSTER_KUBECONFIG set, it will be used instead of KUBECONFIG as a source file for the secret.
  KUBECONFIG_SECRET_SOURCE="${KUBECONFIG}"
  if [[ -n "${IN_CLUSTER_KUBECONFIG+x}" ]]; then
    KUBECONFIG_SECRET_SOURCE="${IN_CLUSTER_KUBECONFIG}"
  fi

  kubectl create -n=e2e secret generic kubeconfig --from-file=kubeconfig="${KUBECONFIG_SECRET_SOURCE}" --dry-run=client -o=yaml | kubectl_create -f=-

  # Create a Secret including _all_ workers' kubeconfigs (including the main cluster's kubeconfig if present in WORKER_KUBECONFIGS).
  kubectl create -n=e2e secret generic worker-kubeconfigs ${WORKER_KUBECONFIGS[@]/#/--from-file=} --dry-run=client -o=yaml | kubectl_create -f=-
  # Build a comma-separated string following a `<cluster_identifier>=<kubeconfig_path_in_container>` format expected by `--worker-kubeconfigs` flag.
  worker_kubeconfigs_in_container_paths=$(
    res=()
    for key in "${!WORKER_KUBECONFIGS[@]}"; do
      basename="${WORKER_KUBECONFIGS[$key]##*/}"
      in_container_path="${basename/#//var/run/secrets/worker-kubeconfigs/}"
      res+=( "${key}=${in_container_path}" )
    done
    IFS=','
    echo "${res[*]}"
  )

  gcs_sa_in_container_path=""
  if [[ -n "${SO_GCS_SERVICE_ACCOUNT_CREDENTIALS_PATH+x}" ]]; then
    gcs_sa_in_container_path=/var/run/secrets/gcs-service-account-credentials/gcs-service-account.json
    kubectl create -n=e2e secret generic gcs-service-account-credentials --from-file=gcs-service-account.json="${SO_GCS_SERVICE_ACCOUNT_CREDENTIALS_PATH}" --dry-run=client -o=yaml | kubectl_create -f=-
  else
    kubectl create -n=e2e secret generic gcs-service-account-credentials --dry-run=client -o=yaml | kubectl_create -f=-
  fi

  s3_credentials_in_container_path=""
  if [[ -n "${SO_S3_CREDENTIALS_PATH+x}" ]]; then
    s3_credentials_in_container_path=/var/run/secrets/s3-credentials/credentials
    kubectl create -n=e2e secret generic s3-credentials --from-file=credentials="${SO_S3_CREDENTIALS_PATH}" --dry-run=client -o=yaml | kubectl_create -f=-
  else
    kubectl create -n=e2e secret generic s3-credentials --dry-run=client -o=yaml | kubectl_create -f=-
  fi

  # Build a comma-separated string following a `<cluster_identifier>=<bucket_name>` format expected by `--worker-object-storage-buckets` flag.
  worker_object_storage_buckets=$(
    res=()
    for key in "${!WORKER_OBJECT_STORAGE_BUCKETS[@]}"; do
      res+=( "${key}=${WORKER_OBJECT_STORAGE_BUCKETS[$key]}" )
    done
    IFS=','
    echo "${res[*]}"
  )

  # Create a Secret including workers' S3 credentials files.
  kubectl create -n=e2e secret generic worker-s3-credentials ${WORKER_S3_CREDENTIALS_PATHS[@]/#/--from-file=} --dry-run=client -o=yaml | kubectl_create -f=-
  # Build a comma-separated string following a `<cluster_identifier>=<s3_credentials_path_in_container>` format expected by `--worker-s3-credentials-file-paths` flag.
  worker_gcs_sa_in_container_paths=$(
    res=()
    for key in "${!WORKER_GCS_SERVICE_ACCOUNT_CREDENTIALS_PATHS[@]}"; do
      basename="${WORKER_GCS_SERVICE_ACCOUNT_CREDENTIALS_PATHS[$key]##*/}"
      in_container_path="/var/run/secrets/worker-gcs-service-account-credentials/${basename}"
      res+=( "${key}=${in_container_path}" )
    done
    IFS=','
    echo "${res[*]}"
  )

  # Create a Secret including workers' GCS service account credentials files.
  kubectl create -n=e2e secret generic worker-gcs-service-account-credentials ${WORKER_GCS_SERVICE_ACCOUNT_CREDENTIALS_PATHS[@]/#/--from-file=} --dry-run=client -o=yaml | kubectl_create -f=-
  # Build a comma-separated string following a `<cluster_identifier>=<gcs_service_account_path_in_container>` format expected by `--worker-gcs-service-account-key-paths` flag.
  worker_s3_credentials_in_container_paths=$(
    res=()
    for key in "${!WORKER_S3_CREDENTIALS_PATHS[@]}"; do
      basename="${WORKER_S3_CREDENTIALS_PATHS[$key]##*/}"
      in_container_path="/var/run/secrets/worker-s3-credentials/${basename}"
      res+=( "${key}=${in_container_path}" )
    done
    IFS=','
    echo "${res[*]}"
  )

  ingress_class_name='haproxy'
  ingress_custom_annotations='haproxy.org/ssl-passthrough=true,route.openshift.io/termination=passthrough'
  ingress_controller_address="$( kubectl -n=haproxy-ingress get svc haproxy-ingress --template='{{ .spec.clusterIP }}' ):9142"

  e2e_command_args=(
    "--skip=${SO_SKIPPED_TESTS}"
    "--kubeconfig=/var/run/secrets/kubeconfig"
    "--loglevel=2"
    "--color=false"
    "--artifacts-dir=/tmp/artifacts"
    "--parallelism=${SO_E2E_PARALLELISM}"
    "--timeout=${SO_E2E_TIMEOUT}"
    "--feature-gates=${SCYLLA_OPERATOR_FEATURE_GATES}"
    "--ingress-controller-address=${ingress_controller_address}"
    "--ingress-controller-ingress-class-name=${ingress_class_name}"
    "--ingress-controller-custom-annotations=${ingress_custom_annotations}"
    "--scyllacluster-node-service-type=${SO_SCYLLACLUSTER_NODE_SERVICE_TYPE}"
    "--scyllacluster-nodes-broadcast-address-type=${SO_SCYLLACLUSTER_NODES_BROADCAST_ADDRESS_TYPE}"
    "--scyllacluster-clients-broadcast-address-type=${SO_SCYLLACLUSTER_CLIENTS_BROADCAST_ADDRESS_TYPE}"
    "--scyllacluster-storageclass-name=${SO_SCYLLACLUSTER_STORAGECLASS_NAME}"
    "--scyllacluster-reactor-backend=${SO_SCYLLACLUSTER_REACTOR_BACKEND:-}"
    "--object-storage-bucket=${SO_BUCKET_NAME}"
    "--gcs-service-account-key-path=${gcs_sa_in_container_path}"
    "--s3-credentials-file-path=${s3_credentials_in_container_path}"
    "--scylladb-version=${SCYLLADB_VERSION}"
    "--scylladb-manager-version=${SCYLLADB_MANAGER_VERSION}"
    "--scylladb-manager-agent-version=${SCYLLADB_MANAGER_AGENT_VERSION}"
    "--scylladb-update-from-version=${SCYLLADB_UPDATE_FROM_VERSION}"
    "--scylladb-upgrade-from-version=${SCYLLADB_UPGRADE_FROM_VERSION}"
  )

  if [[ -n "${worker_kubeconfigs_in_container_paths}" ]]; then
    e2e_command_args+=( "--worker-kubeconfigs=${worker_kubeconfigs_in_container_paths}" )
  fi

  if [[ -n "${worker_gcs_sa_in_container_paths}" ]]; then
    e2e_command_args+=( "--worker-gcs-service-account-key-paths=${worker_gcs_sa_in_container_paths}" )
  fi

  if [[ -n "${worker_s3_credentials_in_container_paths}" ]]; then
    e2e_command_args+=( "--worker-s3-credentials-file-paths=${worker_s3_credentials_in_container_paths}" )
  fi

  if [[ -n "${worker_object_storage_buckets}" ]]; then
    e2e_command_args+=( "--worker-object-storage-buckets=${worker_object_storage_buckets}" )
  fi

  if [[ -n "${SO_FOCUS+x}" ]]; then
    e2e_command_args+=( "--focus=${SO_FOCUS}" )
  fi

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
    imagePullPolicy: Always
    volumeMounts:
    - name: artifacts
      mountPath: /tmp/artifacts
  - name: e2e
    command:
    - scylla-operator-tests
    - run
    - "${SO_SUITE}"
$(printf '    - "%s"\n' "${e2e_command_args[@]}")
    image: "${SO_IMAGE}"
    imagePullPolicy: Always
    volumeMounts:
    - name: artifacts
      mountPath: /tmp/artifacts
    - name: gcs-service-account-credentials
      mountPath: /var/run/secrets/gcs-service-account-credentials
    - name: s3-credentials
      mountPath: /var/run/secrets/s3-credentials
    - name: kubeconfig
      mountPath: /var/run/secrets/kubeconfig
      subPath: kubeconfig
      readOnly: true
    - name: worker-kubeconfigs
      mountPath: /var/run/secrets/worker-kubeconfigs
      readOnly: true
    - name: worker-gcs-service-account-credentials
      mountPath: /var/run/secrets/worker-gcs-service-account-credentials
      readOnly: true
    - name: worker-s3-credentials
      mountPath: /var/run/secrets/worker-s3-credentials
      readOnly: true
  volumes:
  - name: artifacts
    emptyDir: {}
  - name: gcs-service-account-credentials
    secret:
      secretName: gcs-service-account-credentials
  - name: s3-credentials
    secret:
      secretName: s3-credentials
  - name: kubeconfig
    secret:
      secretName: kubeconfig
      items:
      - key: kubeconfig
        path: kubeconfig
  - name: worker-kubeconfigs
    secret:
      secretName: worker-kubeconfigs
  - name : worker-gcs-service-account-credentials
    secret:
      secretName: worker-gcs-service-account-credentials
  - name: worker-s3-credentials
    secret:
      secretName: worker-s3-credentials
EOF
  kubectl -n=e2e wait --for=condition=Ready pod/e2e

  exit_code="$( wait-for-container-exit-with-logs e2e e2e e2e )"

  kubectl_cp -n=e2e e2e:/tmp/artifacts -c=wait-for-artifacts "${ARTIFACTS}"

  ls -l "${ARTIFACTS}"

  kubectl -n=e2e delete pod/e2e --wait="${SO_WAIT_FOR_E2E_POD_DELETE}"

  if [[ "${exit_code}" != "0" ]]; then
    echo "E2E tests failed"
    exit "${exit_code}"
  fi

  wait
  echo "E2E tests finished successfully"
}
