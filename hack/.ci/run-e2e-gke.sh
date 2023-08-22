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

field_manager=hack

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

kubectl_create -f - << EOF
apiVersion: scylla.scylladb.com/v1alpha1
kind: NodeConfig
metadata:
  name: cluster
spec:
  # TODO: make local disk setup create a disk img on a filesystem so we don't need the SSD.
  localDiskSetup:
    filesystems:
    - device: /dev/nvme0n1
      type: xfs
    mounts:
    - device: /dev/nvme0n1
      mountPoint: /mnt/persistent-volumes
      unsupportedOptions:
      - prjquota
  placement:
    nodeSelector:
      scylla.scylladb.com/node-type: scylla
    tolerations:
    - effect: NoSchedule
      key: role
      operator: Equal
      value: scylla-clusters
EOF

kubectl_create -n local-csi-driver -f=https://raw.githubusercontent.com/scylladb/k8s-local-volume-provisioner/dcfceb52d9122192e50236463060cb5151d90dc4/deploy/kubernetes/local-csi-driver/{00_namespace,10_csidriver,10_driver_serviceaccount,10_provisioner_clusterrole,20_provisioner_clusterrolebinding,50_daemonset}.yaml
kubectl -n local-csi-driver patch --field-manager="${field_manager}" daemonset/local-csi-driver --type=json -p='[{"op": "add", "path": "/spec/template/spec/nodeSelector/pool", "value":"workers"}]'
# FIXME: The storage class should live alongside the manifests.
kubectl_create -n local-csi-driver -f https://raw.githubusercontent.com/scylladb/k8s-local-volume-provisioner/dcfceb52d9122192e50236463060cb5151d90dc4/example/storageclass_xfs.yaml
# TODO: Our tests shouldn't require a default storage class but rather have an option to specify which one to use
kubectl patch --field-manager="${field_manager}" storageclass scylladb-local-xfs -p '{"metadata": {"annotations":{"storageclass.kubernetes.io/is-default-class":"true"}}}'

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

kubectl -n e2e run --restart=Never --image="${SO_IMAGE}" --labels='app=e2e' --command=true e2e -- bash -euExo pipefail -O inherit_errexit -c "mkdir /tmp/artifacts && scylla-operator-tests run '${SO_SUITE}' --parallelism=60 --loglevel=2 --color=false --artifacts-dir=/tmp/artifacts --feature-gates='${SCYLLA_OPERATOR_FEATURE_GATES}' --override-ingress-address='${ingress_address}' && touch /tmp/done && until [[ -f '/tmp/exit' ]]; do sleep 1; done"
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

wait "${bg_pid}"

if [[ "${exit_code}" != "0" ]]; then
  exit "${exit_code}"
fi
