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
  ARTIFACTS_DIR="${ARTIFACTS}" timeout -v 50m ./hack/ci-gather-artifacts.sh
  # Events are costly to gather, so we'll erase them for now. (e2e namespaces are collected separately)
  kubectl delete --raw /api/v1/namespaces/default/events || true
  kubectl delete --raw /api/v1/namespaces/kube-system/events || true
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

kubectl apply --server-side -f ./pkg/api/scylla/v1alpha1/scylla.scylladb.com_nodeconfigs.yaml
kubectl wait --for condition=established crd/nodeconfigs.scylla.scylladb.com

if [[ "${SO_DISABLE_NODECONFIG}" == "true" ]]; then
  echo "Skipping NodeConfig creation"
else
  kubectl_create -f ./hack/.ci/manifests/cluster/nodeconfig.yaml
  kubectl_create -n local-csi-driver -f ./hack/.ci/manifests/namespaces/local-csi-driver/
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

ingress_address="$( kubectl -n haproxy-ingress get svc haproxy-ingress --template='{{ .spec.clusterIP }}' )"

kubectl create namespace e2e --dry-run=client -o yaml | kubectl_create -f -
kubectl create clusterrolebinding e2e --clusterrole=cluster-admin --serviceaccount=e2e:default --dry-run=client -o yaml | kubectl_create -f -
kubectl create -n e2e pdb my-pdb --selector='app=e2e' --min-available=1 --dry-run=client -o yaml | kubectl_create -f -

kubectl -n e2e run --restart=Never --image="${SO_IMAGE}" --labels='app=e2e' --command=true e2e -- bash -euExo pipefail -O inherit_errexit -c "function wait-for-artifacts { touch /tmp/done && until [[ -f '/tmp/exit' ]]; do sleep 1; done } && trap wait-for-artifacts EXIT && mkdir /tmp/artifacts && scylla-operator-tests run '${SO_SUITE}' --loglevel=2 --color=false --artifacts-dir=/tmp/artifacts --feature-gates='${SCYLLA_OPERATOR_FEATURE_GATES}' --override-ingress-address='${ingress_address}'"
kubectl -n e2e wait --for=condition=Ready pod/e2e

# Setup artifacts transfer when finished and unblock the e2e pod when done.
(
  kubectl -n e2e exec pod/e2e -- bash -euEo pipefail -O inherit_errexit -c "until [[ -f /tmp/done ]]; do sleep 1; done; ls -l /tmp/artifacts"
  kubectl -n e2e cp --retries=42 e2e:/tmp/artifacts "${ARTIFACTS}"
  ls -l "${ARTIFACTS}"
  kubectl -n e2e exec pod/e2e -- bash -euEo pipefail -O inherit_errexit -c "touch /tmp/exit"
) &
bg_pid=$!

kubectl -n e2e logs -f pod/e2e
exit_code=$( kubectl -n e2e get pods/e2e --output='jsonpath={.status.containerStatuses[0].state.terminated.exitCode}' )
kubectl -n e2e delete pod/e2e --wait=false

wait "${bg_pid}" || ( echo "Collecting e2e artifacts failed" && exit 2 )

if [[ "${exit_code}" != "0" ]]; then
  echo "E2E tests failed"
  exit "${exit_code}"
fi

echo "E2E tests finished successfully"
