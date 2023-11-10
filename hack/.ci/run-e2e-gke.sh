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

function gather-artifacts {
  kubectl -n e2e run --restart=Never --image="${SO_IMAGE}" --labels='app=must-gather' --command=true must-gather -- bash -euExo pipefail -O inherit_errexit -c "function wait-for-artifacts { touch /tmp/done && until [[ -f '/tmp/exit' ]]; do sleep 1; done } && trap wait-for-artifacts EXIT && mkdir /tmp/artifacts && scylla-operator must-gather --all-resources --loglevel=2 --dest-dir=/tmp/artifacts"
  kubectl -n e2e wait --for=condition=Ready pod/must-gather

  # Setup artifacts transfer when finished and unblock the must-gather pod when done.
  (
    kubectl -n e2e exec pod/must-gather -- bash -euEo pipefail -O inherit_errexit -c "until [[ -f /tmp/done ]]; do sleep 1; done; ls -l /tmp/artifacts"
    kubectl -n e2e cp --retries=42 must-gather:/tmp/artifacts "${ARTIFACTS}/must-gather"
    ls -l "${ARTIFACTS}"
    kubectl -n e2e exec pod/must-gather -- bash -euEo pipefail -O inherit_errexit -c "touch /tmp/exit"
  ) &
  must_gather_bg_pid=$!

  kubectl -n e2e logs -f pod/must-gather
  exit_code=$( kubectl -n e2e get pods/must-gather --output='jsonpath={.status.containerStatuses[0].state.terminated.exitCode}' )
  kubectl -n e2e delete pod/must-gather --wait=false

  if [[ "${exit_code}" != "0" ]]; then
    echo "Collecting artifacts using must-gather failed"
    exit "${exit_code}"
  fi

  wait "${must_gather_bg_pid}"
}

function handle-exit {
  gather-artifacts || "Error gathering artifacts" > /dev/stderr
}

trap handle-exit EXIT

# Allow admin to use ephemeralcontainers
kubectl_create -f - <<EOF
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
kubectl_create -n default -f - <<EOF
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
kubectl -n default rollout status daemonset/sysctl

kubectl apply --server-side -f ./pkg/api/scylla/v1alpha1/scylla.scylladb.com_nodeconfigs.yaml
kubectl wait --for condition=established crd/nodeconfigs.scylla.scylladb.com

if [[ "${SO_DISABLE_NODECONFIG}" == "true" ]]; then
  echo "Skipping NodeConfig creation"
else
  kubectl_create -f ./hack/.ci/manifests/cluster/nodeconfig.yaml
fi

SCYLLA_OPERATOR_FEATURE_GATES='AllAlpha=true,AllBeta=true'
export SCYLLA_OPERATOR_FEATURE_GATES
REENTRANT=true timeout -v 10m ./hack/ci-deploy.sh "${SO_IMAGE}"
# Raise loglevel in CI.
# TODO: Replace it with ScyllaOperatorConfig field when available.
kubectl -n scylla-operator patch --field-manager="${field_manager}" deployment/scylla-operator --type=json -p='[{"op": "add", "path": "/spec/template/spec/containers/0/args/-", "value": "--loglevel=4"}]'
kubectl -n scylla-operator rollout status deployment/scylla-operator

kubectl -n scylla-manager patch --field-manager="${field_manager}" deployment/scylla-manager-controller --type=json -p='[{"op": "add", "path": "/spec/template/spec/containers/0/args/-", "value": "--loglevel=4"}]'
kubectl -n scylla-manager rollout status deployment/scylla-manager-controller

ingress_class_name='haproxy'
ingress_custom_annotations='haproxy.org/ssl-passthrough=true'
ingress_controller_address="$( kubectl -n haproxy-ingress get svc haproxy-ingress --template='{{ .spec.clusterIP }}' ):9142"

kubectl create namespace e2e --dry-run=client -o yaml | kubectl_create -f -
kubectl create clusterrolebinding e2e --clusterrole=cluster-admin --serviceaccount=e2e:default --dry-run=client -o yaml | kubectl_create -f -
kubectl create -n e2e pdb my-pdb --selector='app=e2e' --min-available=1 --dry-run=client -o yaml | kubectl_create -f -

kubectl -n e2e run --restart=Never --image="${SO_IMAGE}" --labels='app=e2e' --command=true e2e -- bash -euExo pipefail -O inherit_errexit -c "function wait-for-artifacts { touch /tmp/done && until [[ -f '/tmp/exit' ]]; do sleep 1; done } && trap wait-for-artifacts EXIT && mkdir /tmp/artifacts && scylla-operator-tests run '${SO_SUITE}' --loglevel=2 --color=false --artifacts-dir=/tmp/artifacts --feature-gates='${SCYLLA_OPERATOR_FEATURE_GATES}' --ingress-controller-address='${ingress_controller_address}' --ingress-controller-ingress-class-name='${ingress_class_name}' --ingress-controller-custom-annotations='${ingress_custom_annotations}'"
kubectl -n e2e wait --for=condition=Ready pod/e2e

# Setup artifacts transfer when finished and unblock the e2e pod when done.
(
  kubectl -n e2e exec pod/e2e -- bash -euEo pipefail -O inherit_errexit -c "until [[ -f /tmp/done ]]; do sleep 1; done; ls -l /tmp/artifacts"
  kubectl -n e2e cp --retries=42 e2e:/tmp/artifacts "${ARTIFACTS}"
  ls -l "${ARTIFACTS}"
  kubectl -n e2e exec pod/e2e -- bash -euEo pipefail -O inherit_errexit -c "touch /tmp/exit"
) &
e2e_bg_pid=$!

kubectl -n e2e logs -f pod/e2e
exit_code=$( kubectl -n e2e get pods/e2e --output='jsonpath={.status.containerStatuses[0].state.terminated.exitCode}' )
kubectl -n e2e delete pod/e2e --wait=false

wait "${e2e_bg_pid}" || ( echo "Collecting e2e artifacts failed" && exit 2 )

if [[ "${exit_code}" != "0" ]]; then
  echo "E2E tests failed"
  exit "${exit_code}"
fi

echo "E2E tests finished successfully"
