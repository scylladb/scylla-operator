#!/usr/bin/env bash
#
# Copyright (C) 2024 ScyllaDB
#

set -euExo pipefail
shopt -s inherit_errexit

FIELD_MANAGER="${FIELD_MANAGER:-so-default}"

function kubectl_apply {
  kubectl apply --server-side=true --field-manager="${FIELD_MANAGER}" --force-conflicts "$@"
}

function kubectl_create {
    if [[ -z "${REENTRANT+x}" ]]; then
        # In an actual CI run we have to enforce that no two objects have the same name.
        kubectl create --field-manager="${FIELD_MANAGER}" "$@"
    else
        # For development iterations we want to update the objects.
        kubectl_apply "$@"
    fi
}

function kubectl_cp {
  for i in {1..60}; do
    kubectl cp "$@" && break || echo "Attempt $i to kubectl copy failed, retrying"
  done
}

# kubectl_retry runs kubectl with the given arguments until it succeeds or the timeout expires. It's meant for idempotent
# commands, like waits, so that a dropped connection to the API server doesn't fail the caller.
# $1 - timeout
# $2... - kubectl arguments
function kubectl_retry {
  local timeout="${1}"
  shift
  timeout --verbose "${timeout}" bash -c 'until kubectl "$@"; do sleep 5; done' bash "$@"
}

# wait-for-object-creation awaits the creation of a Kubernetes object in a given namespace.
# $1 - namespace
# $2 - object kind/name (e.g., pod/name)
# $3 - timeout (optional, defaults to 60s)
function wait-for-object-creation {
  local timeout="${3:-60s}"
  kubectl_retry "${timeout}" wait --timeout="${timeout}" --for=create -n="${1}" "${2}"
}

# wait-for-scyllacluster-rollout waits until a ScyllaCluster is fully rolled out and available.
# $1 - namespace
# $2 - ScyllaCluster name
# $3 - timeout (optional, defaults to 10m)
function wait-for-scyllacluster-rollout {
  local timeout="${3:-10m}"
  kubectl -n="${1}" wait --timeout="${timeout}" --for='condition=Progressing=False' scyllaclusters.scylla.scylladb.com/"${2}"
  kubectl -n="${1}" wait --timeout="${timeout}" --for='condition=Degraded=False' scyllaclusters.scylla.scylladb.com/"${2}"
  kubectl -n="${1}" wait --timeout="${timeout}" --for='condition=Available=True' scyllaclusters.scylla.scylladb.com/"${2}"
}

# $1 - namespace
# $2 - pod name
# $3 - container name
function wait-for-container-exit-with-logs {
  exit_code=""
  while [[ "${exit_code}" == "" ]]; do
    kubectl -n="${1}" logs -f pod/"${2}" -c="${3}" > /dev/stderr || echo "kubectl logs failed before pod has finished, retrying..." > /dev/stderr
    exit_code="$( kubectl -n="${1}" get pods/"${2}" --template='{{ range .status.containerStatuses }}{{ if and (eq .name "'"${3}"'") (ne .state.terminated.exitCode nil) }}{{ .state.terminated.exitCode }}{{ end }}{{ end }}' )"
  done
  echo -n "${exit_code}"
}
