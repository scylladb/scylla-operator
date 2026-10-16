#!/usr/bin/env bash
#
# Copyright (C) 2026 ScyllaDB
#
# This script deploys a ScyllaCluster using the Helm chart, waits for it to roll out
# and verifies CQL connectivity. It expects scylla-operator to be already deployed.

set -euxEo pipefail
shopt -s inherit_errexit

source "$( dirname "${BASH_SOURCE[0]}" )/lib/kube.sh"

source_root="$( realpath "$( dirname "${BASH_SOURCE[0]}" )/.." )"

helm install scylla "${source_root}/helm/scylla" \
  --create-namespace \
  --namespace=scylla \
  --values=- <<EOF
developerMode: true
scyllaArgs: "--reactor-backend=io_uring"
exposeOptions:
  nodeService:
    type: Headless
  broadcastOptions:
    nodes:
      type: PodIP
    clients:
      type: PodIP
racks:
- name: us-east-1a
  members: 1
  storage:
    storageClassName: standard
    capacity: 1Gi
  resources:
    requests:
      cpu: 10m
      memory: 100Mi
    limits:
      cpu: 1
      memory: 1Gi
EOF

wait-for-scyllacluster-rollout scylla scylla

pod="$( kubectl -n=scylla get pods -l='scylla/cluster=scylla' -o=jsonpath='{.items[0].metadata.name}' )"
kubectl -n=scylla exec "${pod}" -c=scylla -- cqlsh localhost -e 'SELECT key, cluster_name, data_center FROM system.local'
