#!/bin/bash
#
# Copyright (C) 2021 ScyllaDB
#
# This script deploys scylla-operator and scylla-manager.
# Usage: ${0} <operator_image_ref>

set -euxEo pipefail

function wait-for-object-creation {
    for i in {1..30}; do
        { kubectl -n "${1}" get "${2}" && break; } || sleep 1
    done
}

if [[ -z ${1+x} ]]; then
    echo "Missing operator image ref.\nUsage: ${0} <operator_image_ref>" >&2 >/dev/null
    exit 1
fi

ARTIFACTS_DIR=${ARTIFACTS_DIR:-$( mktemp -d )}
OPERATOR_IMAGE_REF=${1}

deploy_dir=${ARTIFACTS_DIR}/deploy
mkdir -p "${deploy_dir}"

cp ./examples/common/{manager,operator,cert-manager}.yaml "${deploy_dir}/"
yq eval 'select(.apiVersion=="rbac.authorization.k8s.io/v1" and .kind=="ClusterRole" and .metadata.name=="simple-cluster-member") | del(.metadata.namespace) | .metadata.name="scylla-operator-member"' examples/generic/cluster.yaml > "${deploy_dir}/scylla-operator-member.clusterrole.yaml"

for f in "${deploy_dir}"/*; do
    sed -i -E -e "s~docker.io/scylladb/scylla-operator(:|@sha256:)[^ ]*~${OPERATOR_IMAGE_REF}~" "${f}"
done

kubectl apply -f"${deploy_dir}"/cert-manager.yaml

# Wait for cert-manager
kubectl wait --for condition=established --timeout=60s crd/certificates.cert-manager.io crd/issuers.cert-manager.io
wait-for-object-creation cert-manager deployment.apps/cert-manager-webhook
kubectl -n cert-manager rollout status --timeout=5m deployment.apps/cert-manager-webhook

kubectl apply -f"${deploy_dir}"/operator.yaml
kubectl apply -f"${deploy_dir}"/scylla-operator-member.clusterrole.yaml

# Manager needs scylla CRD registered and the webhook running
kubectl wait --for condition=established crd/scyllaclusters.scylla.scylladb.com
kubectl -n scylla-operator-system rollout status --timeout=5m statefulset.apps/scylla-operator-controller-manager

# ScyllaCluster doesn't support changes and is immutable so we always have to delete/create it.
kubectl delete --ignore-not-found=true --grace-period=0 --cascade=foreground -f"${deploy_dir}"/manager.yaml
kubectl create -f"${deploy_dir}"/manager.yaml
object=$( kubectl -n scylla-manager-system get scyllacluster/scylla-manager-cluster -o json | jq '.metadata.uid = "" | .metadata.resourceVersion = "" | .metadata.generation = 0 | .spec.datacenter.racks[0].resources = {"requests":{"cpu":"10m","memory":"100Mi"},"limits":{"cpu":"200m","memory":"500Mi"}}')
kubectl delete --grace-period=0 --cascade=foreground -f <( echo "${object}" )
kubectl delete --ignore-not-found=true pvc/data-scylla-manager-cluster-manager-dc-manager-rack-0
kubectl create -f <( echo "${object}" )
kubectl patch -n scylla-manager-system deployment/scylla-manager-scylla-manager --type=strategic -p='{"spec":{"template":{"spec":{"containers":[{"name":"scylla-manager","resources":{"requests":{"cpu":"10m","memory":"100Mi"},"limits":null}}]}}}}'
# initContainers patch is broken
wait-for-object-creation scylla-manager-system statefulset.apps/scylla-manager-cluster-manager-dc-manager-rack
kubectl -n scylla-manager-system get statefulset.apps/scylla-manager-cluster-manager-dc-manager-rack -o json | jq '.spec.template.spec.initContainers[0].resources = {"requests":{"cpu":"10m","memory":"100Mi"}} | .metadata.managedFields = null' | kubectl -n scylla-manager-system apply --server-side --force-conflicts -f -

# Wait for the rest to be ready and running
kubectl -n scylla-manager-system rollout status --timeout=5m statefulset.apps/scylla-manager-controller
kubectl -n scylla-manager-system rollout status --timeout=5m statefulset.apps/scylla-manager-cluster-manager-dc-manager-rack

# Allow users to use scyllacluster CRs
kubectl apply --server-side -f - <<EOF
kind: ClusterRole
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: scylla-view
  labels:
    rbac.authorization.k8s.io/aggregate-to-admin: "true"
    rbac.authorization.k8s.io/aggregate-to-edit: "true"
    rbac.authorization.k8s.io/aggregate-to-view: "true"
rules:
- apiGroups:
  - scylla.scylladb.com
  resources:
  - scyllaclusters
  verbs:
  - get
  - list
  - watch
EOF
kubectl apply --server-side -f - <<EOF
kind: ClusterRole
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: scylla-edit
  labels:
    rbac.authorization.k8s.io/aggregate-to-admin: "true"
    rbac.authorization.k8s.io/aggregate-to-edit: "true"
rules:
- apiGroups:
  - scylla.scylladb.com
  resources:
  - scyllaclusters
  verbs:
  - create
  - patch
  - update
  - delete
  - deletecollection
EOF
