#!/usr/bin/env bash
#
# Copyright (C) 2023 ScyllaDB
#

set -euExo pipefail
shopt -s inherit_errexit

if [ -z ${SO_SUITE+x} ]; then
  echo "SO_SUITE can't be empty" > /dev/stderr
  exit 2
fi

if [ -z ${SO_IMAGE+x} ]; then
  echo "SO_IMAGE can't be empty" > /dev/stderr
  exit 2
fi

if [ -z ${SO_SCYLLACLUSTER_NODE_SERVICE_TYPE+x} ]; then
  echo "SO_SCYLLACLUSTER_NODE_SERVICE_TYPE can't be empty" > /dev/stderr
  exit 2
fi

if [ -z ${SO_SCYLLACLUSTER_NODES_BROADCAST_ADDRESS_TYPE+x} ]; then
  echo "SO_SCYLLACLUSTER_NODES_BROADCAST_ADDRESS_TYPE can't be empty" > /dev/stderr
  exit 2
fi

if [ -z ${SO_SCYLLACLUSTER_CLIENTS_BROADCAST_ADDRESS_TYPE+x} ]; then
  echo "SO_SCYLLACLUSTER_CLIENTS_BROADCAST_ADDRESS_TYPE can't be empty" > /dev/stderr
  exit 2
fi

if [ -z ${ARTIFACTS+x} ]; then
  echo "ARTIFACTS can't be empty" > /dev/stderr
  exit 2
fi

if [ -z ${KUBECONFIG_DIR+x} ] && [ -z ${KUBECONFIG+x} ]; then
  echo "Either KUBECONFIG_DIR or KUBECONFIG must be set" > /dev/stderr
  exit 2
fi

if [ -z ${KUBECONFIG_DIR+x} ]; then
  KUBECONFIGS=("${KUBECONFIG}")
else
  KUBECONFIGS=()
  for f in $( find $(realpath "${KUBECONFIG_DIR}") -maxdepth 1 -type f ); do
    # Sanity check
    kubectl --kubeconfig="${f}" cluster-info

    KUBECONFIGS+=("${f}")
  done
fi

if [ ${#KUBECONFIGS[@]} -lt 1 ]; then
  echo "At least one kubeconfig is required" > /dev/stderr
  exit 2
fi

SO_DISABLE_NODECONFIG=${SO_DISABLE_NODECONFIG:-false}

field_manager=run-e2e-script

function kubectl_create {
    if [[ -z ${REENTRANT+x} ]]; then
        # In an actual CI run we have to enforce that no two objects have the same name.
        kubectl create --field-manager="${field_manager}" "$@"
    else
        # For development iterations we want to update the objects.
        kubectl apply --server-side=true --field-manager="${field_manager}" --force-conflicts "$@"
    fi
}

# $1 - kubeconfig path
function gather-artifacts {
  local kubeconfig
  kubeconfig="${1}"

  kubectl --kubeconfig="${kubeconfig}" -n e2e run --restart=Never --image="${SO_IMAGE}" --labels='app=must-gather' --command=true must-gather -- bash -euExo pipefail -O inherit_errexit -c "function wait-for-artifacts { touch /tmp/done && until [[ -f '/tmp/exit' ]]; do sleep 1; done } && trap wait-for-artifacts EXIT && mkdir /tmp/artifacts && scylla-operator must-gather --all-resources --loglevel=2 --dest-dir=/tmp/artifacts"
  kubectl --kubeconfig="${kubeconfig}" -n e2e wait --for=condition=Ready pod/must-gather

  # Setup artifacts transfer when finished and unblock the must-gather pod when done.
  (
    function unblock-must-gather-pod {
      kubectl --kubeconfig="${kubeconfig}" -n e2e exec pod/must-gather -- bash -euEo pipefail -O inherit_errexit -c "touch /tmp/exit"
    }
    trap unblock-must-gather-pod EXIT

    kubectl --kubeconfig="${kubeconfig}" -n e2e exec pod/must-gather -- bash -euEo pipefail -O inherit_errexit -c "until [[ -f /tmp/done ]]; do sleep 1; done; ls -l /tmp/artifacts"
    # TODO: does the path make sense? Cluster name isn't particularly helpful.
    kubectl --kubeconfig="${kubeconfig}" -n e2e cp --retries=42 must-gather:/tmp/artifacts "${ARTIFACTS}/must-gather/$( kubectl --kubeconfig="${kubeconfig}" config current-context )"
    ls -l "${ARTIFACTS}"
  ) &
  must_gather_bg_pid=$!

  kubectl --kubeconfig="${kubeconfig}" -n e2e logs -f pod/must-gather
  exit_code=$( kubectl --kubeconfig="${kubeconfig}" -n e2e get pods/must-gather --output='jsonpath={.status.containerStatuses[0].state.terminated.exitCode}' )
  kubectl --kubeconfig="${kubeconfig}" -n e2e delete pod/must-gather --wait=false

  if [[ "${exit_code}" != "0" ]]; then
    echo "Collecting artifacts using must-gather failed"
    exit "${exit_code}"
  fi

  wait "${must_gather_bg_pid}"
}

function handle-exit {
  for i in ${!KUBECONFIGS[@]}; do
    gather-artifacts "${KUBECONFIGS[$i]}" &
    gather_artifacts_bg_pids["${i}"]=$!
  done

  gather_artifacts_failed=0
  for pid in ${gather_artifacts_bg_pids[*]}; do
    wait "${pid}" || gather_artifacts_failed=1
  done

  if [ ${gather_artifacts_failed} -eq 1 ]; then
    echo "Error gathering artifacts" > /dev/stderr
  fi

  echo "exited"
}

trap handle-exit EXIT

# $1 - kubeconfig path
function setup {
  # Allow admin to use ephemeralcontainers
  kubectl_create --kubeconfig="${1}" -f - <<EOF
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
  kubectl_create --kubeconfig="${1}" -n default -f - <<EOF
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
EOF
  kubectl --kubeconfig="${1}" -n default rollout status daemonset/sysctl

  kubectl --kubeconfig="${1}" apply --server-side -f ./pkg/api/scylla/v1alpha1/scylla.scylladb.com_nodeconfigs.yaml
  kubectl --kubeconfig="${1}" wait --for condition=established crd/nodeconfigs.scylla.scylladb.com

  if [[ "${SO_DISABLE_NODECONFIG}" == "true" ]]; then
    echo "Skipping NodeConfig creation"
  else
    kubectl_create --kubeconfig="${1}" -f ./hack/.ci/manifests/cluster/nodeconfig.yaml
    kubectl_create --kubeconfig="${1}" -n local-csi-driver -f ./hack/.ci/manifests/namespaces/local-csi-driver/
  fi

  kubectl create namespace e2e --dry-run=client -o yaml | kubectl_create --kubeconfig="${1}" -f -
  kubectl create clusterrolebinding e2e --clusterrole=cluster-admin --serviceaccount=e2e:default --dry-run=client -o yaml | kubectl_create --kubeconfig="${1}" -f -

  SCYLLA_OPERATOR_FEATURE_GATES='AllAlpha=true,AllBeta=true'
  export SCYLLA_OPERATOR_FEATURE_GATES
  REENTRANT=true KUBECONFIG="${1}" timeout -v 10m ./hack/ci-deploy.sh "${SO_IMAGE}"
  # Raise loglevel in CI.
  # TODO: Replace it with ScyllaOperatorConfig field when available.
  kubectl --kubeconfig="${1}" -n scylla-operator patch --field-manager="${field_manager}" deployment/scylla-operator --type=json -p='[{"op": "add", "path": "/spec/template/spec/containers/0/args/-", "value": "--loglevel=4"}]'
  kubectl --kubeconfig="${1}" -n scylla-operator rollout status deployment/scylla-operator

  kubectl --kubeconfig="${1}" -n scylla-manager patch --field-manager="${field_manager}" deployment/scylla-manager-controller --type=json -p='[{"op": "add", "path": "/spec/template/spec/containers/0/args/-", "value": "--loglevel=4"}]'
  kubectl --kubeconfig="${1}" -n scylla-manager rollout status deployment/scylla-manager-controller

  kubectl create -n e2e pdb my-pdb --selector='app=e2e' --min-available=1 --dry-run=client -o yaml | kubectl_create --kubeconfig="${1}" -f -
}

for i in ${!KUBECONFIGS[@]}; do
  setup "${KUBECONFIGS[$i]}" &
  setup_bg_pids["${i}"]=$!
done

setup_failed=0
for pid in ${setup_bg_pids[*]}; do
  wait "${pid}" || setup_failed=1
done

if [ ${setup_failed} -eq 1 ]; then
  echo "Error setting up clusters" > /dev/stderr
  exit 2
fi

feature_gates='AllAlpha=true,AllBeta=true'
ingress_class_name='haproxy'
ingress_custom_annotations='haproxy.org/ssl-passthrough=true'
ingress_controller_address="$( kubectl --kubeconfig="${KUBECONFIGS[0]}" -n haproxy-ingress get svc haproxy-ingress --template='{{ .spec.clusterIP }}' ):9142"

kubectl_create --kubeconfig="${KUBECONFIGS[0]}" -n e2e configmap e2e $( IFS=" "; echo "${KUBECONFIGS[@]/#/--from-file=}" )
kubectl_create --kubeconfig="${KUBECONFIGS[0]}" -n e2e -f - <<EOF
  apiVersion: v1
  kind: Pod
  metadata:
    labels:
      app: e2e
    name: e2e
  spec:
    containers:
    - name: e2e
      image: "${SO_IMAGE}"
      command:
      - bash
      - -euExo
      - pipefail
      - -O
      - inherit_errexit
      - -c
      - |
        function wait-for-artifacts {
          touch /tmp/done && until [[ -f '/tmp/exit' ]]; do sleep 1; done
        }
        trap wait-for-artifacts EXIT

        mkdir /tmp/artifacts
        scylla-operator-tests run '${SO_SUITE}' $( IFS=" "; kubeconfig_basenames=( "${KUBECONFIGS[@]##*/}" ) && echo "${kubeconfig_basenames[@]/#/--kubeconfig=/tmp/kubeconfigs/}" ) --loglevel=2 --color=false --artifacts-dir=/tmp/artifacts --feature-gates='${feature_gates}' --ingress-controller-address='${ingress_controller_address}' --ingress-controller-ingress-class-name='${ingress_class_name}' --ingress-controller-custom-annotations='${ingress_custom_annotations}' kubectl -n e2e run --restart=Never --image="${SO_IMAGE}" --labels='app=e2e' --command=true e2e -- bash -euExo pipefail -O inherit_errexit -c "function wait-for-artifacts { touch /tmp/done && until [[ -f '/tmp/exit' ]]; do sleep 1; done } && trap wait-for-artifacts EXIT && mkdir /tmp/artifacts && scylla-operator-tests run '${SO_SUITE}' --loglevel=2 --color=false --artifacts-dir=/tmp/artifacts --feature-gates='${SCYLLA_OPERATOR_FEATURE_GATES}' --ingress-controller-address='${ingress_controller_address}' --ingress-controller-ingress-class-name='${ingress_class_name}' --ingress-controller-custom-annotations='${ingress_custom_annotations}' --scyllacluster-node-service-type='${SO_SCYLLACLUSTER_NODE_SERVICE_TYPE}' --scyllacluster-nodes-broadcast-address-type='${SO_SCYLLACLUSTER_NODES_BROADCAST_ADDRESS_TYPE}' --scyllacluster-clients-broadcast-address-type='${SO_SCYLLACLUSTER_CLIENTS_BROADCAST_ADDRESS_TYPE}'"

      volumeMounts:
      - name: kubeconfigs
        mountPath: /tmp/kubeconfigs
    restartPolicy: Never
    volumes:
    - name: kubeconfigs
      configMap:
        name: e2e
EOF
kubectl --kubeconfig="${KUBECONFIGS[0]}" -n e2e wait --for=condition=Ready pod/e2e

# Setup artifacts transfer when finished and unblock the e2e pod when done.
(
  function unblock-e2e-pod {
    kubectl --kubeconfig="${KUBECONFIGS[0]}" -n e2e exec pod/e2e -- bash -euEo pipefail -O inherit_errexit -c "touch /tmp/exit"
  }
  trap unblock-e2e-pod EXIT

  kubectl --kubeconfig="${KUBECONFIGS[0]}" -n e2e exec pod/e2e -- bash -euEo pipefail -O inherit_errexit -c "until [[ -f /tmp/done ]]; do sleep 1; done; ls -l /tmp/artifacts"
  kubectl --kubeconfig="${KUBECONFIGS[0]}" -n e2e cp --retries=42 e2e:/tmp/artifacts "${ARTIFACTS}"
  ls -l "${ARTIFACTS}"
) &
e2e_bg_pid=$!

kubectl --kubeconfig="${KUBECONFIGS[0]}" -n e2e logs -f pod/e2e
exit_code=$( kubectl --kubeconfig="${KUBECONFIGS[0]}" -n e2e get pods/e2e --output='jsonpath={.status.containerStatuses[0].state.terminated.exitCode}' )
kubectl --kubeconfig="${KUBECONFIGS[0]}" -n e2e delete pod/e2e --wait=false

wait "${e2e_bg_pid}" || ( echo "Collecting e2e artifacts failed" && exit 2 )

if [[ "${exit_code}" != "0" ]]; then
  echo "E2E tests failed"
  exit "${exit_code}"
fi

echo "E2E tests finished successfully"
