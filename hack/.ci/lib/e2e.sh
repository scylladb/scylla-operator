#!/bin/bash
#
# Copyright (C) 2024 ScyllaDB
#

set -euExo pipefail
shopt -s inherit_errexit

source "$( dirname "${BASH_SOURCE[0]}" )/../../lib/bash.sh"
source "$( dirname "${BASH_SOURCE[0]}" )/../../lib/kube.sh"

if [ -z "${KUBECONFIG_DIR+x}" ]; then
  KUBECONFIGS=("${KUBECONFIG}")
else
  KUBECONFIGS=()
  for f in $( find "$( realpath "${KUBECONFIG_DIR}" )" -maxdepth 1 -type f -name '*.kubeconfig' ); do
    KUBECONFIGS+=("${f}")
  done
fi

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
  kubectl -n=gather-artifacts wait --for=condition=Ready pod/must-gather

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

  for i in "${!KUBECONFIGS[@]}"; do
    KUBECONFIG="${KUBECONFIGS[$i]}" gather-artifacts "${ARTIFACTS}/must-gather/${i}" &
    gather_artifacts_bg_pids["${i}"]=$!
  done

  for pid in "${gather_artifacts_bg_pids[@]}"; do
    wait "${pid}"
  done

  cleanup-bg-jobs "${ec}"
}

function gracefully-shutdown-e2es {
  kubectl -n e2e exec e2e -c e2e -- bash -euEo pipefail -O inherit_errexit -c 'kill -s SIGINT $(pidof scylla-operator-tests)' || true
  kubectl_cp -n=e2e e2e:/tmp/artifacts -c=wait-for-artifacts "${ARTIFACTS}" || true
}

function apply-e2e-workarounds {
  if [ -z "${SO_IMAGE+x}" ]; then
    echo "SO_IMAGE can't be empty" > /dev/stderr
    exit 2
  fi

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

  # FIXME: remove the workaround once https://github.com/scylladb/scylla-operator/issues/749 is done
  kubectl_create -n=default -f=- <<EOF
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: sysctl
spec:
  selector:
    matchLabels:
      app.kubernetes.io/name: sysctl
  template:
    metadata:
      labels:
        app.kubernetes.io/name: sysctl
    spec:
      containers:
      - name: sysctl
        securityContext:
          privileged: true
        image: "${SO_IMAGE}"
        imagePullPolicy: IfNotPresent
        command:
        - /usr/bin/bash
        - -euExo
        - pipefail
        - -O
        - inherit_errexit
        - -c
        args:
        - |
          sysctl fs.aio-max-nr=0xffffffff

          sleep infinity &
          wait
      nodeSelector:
        scylla.scylladb.com/node-type: scylla
      tolerations:
      - effect: NoSchedule
        key: scylla-operator.scylladb.com/dedicated
        operator: Equal
        value: scyllaclusters
EOF
  kubectl -n=default rollout status daemonset/sysctl
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

  config_file="$(realpath "$(dirname "${BASH_SOURCE[0]}")/../../../assets/config/config.yaml")"
  SCYLLADB_VERSION="${SCYLLADB_VERSION:-$(yq '.operator.scyllaDBVersion' "$config_file")}"
  SCYLLADB_MANAGER_VERSION="${SCYLLADB_MANAGER_VERSION:-$(yq '.operator.scyllaDBManagerVersion' "$config_file")}"
  SCYLLADB_MANAGER_AGENT_VERSION="${SCYLLADB_MANAGER_AGENT_VERSION:-$(yq '.operator.scyllaDBManagerAgentVersion' "$config_file")}"
  SCYLLADB_UPDATE_FROM_VERSION="${SCYLLADB_UPDATE_FROM_VERSION:-$(yq '.operatorTests.scyllaDBVersions.updateFrom' "$config_file")}"
  SCYLLADB_UPGRADE_FROM_VERSION="${SCYLLADB_UPGRADE_FROM_VERSION:-$(yq '.operatorTests.scyllaDBVersions.upgradeFrom' "$config_file")}"

  kubectl create namespace e2e --dry-run=client -o=yaml | kubectl_create -f=-
  kubectl create clusterrolebinding e2e --clusterrole=cluster-admin --serviceaccount=e2e:default --dry-run=client -o=yaml | kubectl_create -f=-
  kubectl create -n=e2e pdb my-pdb --selector='app=e2e' --min-available=1 --dry-run=client -o=yaml | kubectl_create -f=-

  kubectl create -n=e2e secret generic kubeconfigs ${KUBECONFIGS[@]/#/--from-file=} --dry-run=client -o=yaml | kubectl_create -f=-
  kubeconfigs_in_container_path=$( IFS=','; basenames=( "${KUBECONFIGS[@]##*/}" ) && in_container_paths="${basenames[@]/#//var/run/secrets/kubeconfigs/}" && echo "${in_container_paths[*]}" )

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

  ingress_class_name='haproxy'
  ingress_custom_annotations='haproxy.org/ssl-passthrough=true,route.openshift.io/termination=passthrough'
  ingress_controller_address="$( kubectl -n=haproxy-ingress get svc haproxy-ingress --template='{{ .spec.clusterIP }}' ):9142"

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
    - "--skip=${SO_SKIPPED_TESTS}"
    - "--kubeconfig=${kubeconfigs_in_container_path}"
    - --loglevel=2
    - --color=false
    - --artifacts-dir=/tmp/artifacts
    - "--parallelism=${SO_E2E_PARALLELISM}"
    - "--timeout=${SO_E2E_TIMEOUT}"
    - "--feature-gates=${SCYLLA_OPERATOR_FEATURE_GATES}"
    - "--ingress-controller-address=${ingress_controller_address}"
    - "--ingress-controller-ingress-class-name=${ingress_class_name}"
    - "--ingress-controller-custom-annotations=${ingress_custom_annotations}"
    - "--scyllacluster-node-service-type=${SO_SCYLLACLUSTER_NODE_SERVICE_TYPE}"
    - "--scyllacluster-nodes-broadcast-address-type=${SO_SCYLLACLUSTER_NODES_BROADCAST_ADDRESS_TYPE}"
    - "--scyllacluster-clients-broadcast-address-type=${SO_SCYLLACLUSTER_CLIENTS_BROADCAST_ADDRESS_TYPE}"
    - "--scyllacluster-storageclass-name=${SO_SCYLLACLUSTER_STORAGECLASS_NAME}"
    - "--object-storage-bucket=${SO_BUCKET_NAME}"
    - "--gcs-service-account-key-path=${gcs_sa_in_container_path}"
    - "--s3-credentials-file-path=${s3_credentials_in_container_path}"
    - "--scylladb-version=${SCYLLADB_VERSION}"
    - "--scylladb-manager-version=${SCYLLADB_MANAGER_VERSION}"
    - "--scylladb-manager-agent-version=${SCYLLADB_MANAGER_AGENT_VERSION}"
    - "--scylladb-update-from-version=${SCYLLADB_UPDATE_FROM_VERSION}"
    - "--scylladb-upgrade-from-version=${SCYLLADB_UPGRADE_FROM_VERSION}"
    image: "${SO_IMAGE}"
    imagePullPolicy: Always
    volumeMounts:
    - name: artifacts
      mountPath: /tmp/artifacts
    - name: gcs-service-account-credentials
      mountPath: /var/run/secrets/gcs-service-account-credentials
    - name: s3-credentials
      mountPath: /var/run/secrets/s3-credentials
    - name: kubeconfigs
      mountPath: /var/run/secrets/kubeconfigs
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
  - name: kubeconfigs
    secret:
      secretName: kubeconfigs
EOF
  kubectl -n=e2e wait --for=condition=Ready pod/e2e

  exit_code="$( wait-for-container-exit-with-logs e2e e2e e2e )"

  kubectl_cp -n=e2e e2e:/tmp/artifacts -c=wait-for-artifacts "${ARTIFACTS}"

  ls -l "${ARTIFACTS}"

  kubectl -n=e2e delete pod/e2e --wait=false

  if [[ "${exit_code}" != "0" ]]; then
    echo "E2E tests failed"
    exit "${exit_code}"
  fi

  wait
  echo "E2E tests finished successfully"
}
