#!/usr/bin/env bash
#
# Copyright (C) 2026 ScyllaDB
#
# Helpers for the CI provisioning objects (ci.scylladb.com) the GitHub Actions workflows create. They run against the
# CI provisioners cluster, so KUBECONFIG has to point at it.

set -euExo pipefail
shopt -s inherit_errexit

# get-provisioned-cluster-kubeconfig extracts a provisioned cluster's kubeconfig from its Secret into a file and sanity
# checks it.
# $1 - name of the Secret holding the kubeconfig
# $2 - path to write the kubeconfig to
function get-provisioned-cluster-kubeconfig {
  local secret_name="${1}"
  local kubeconfig="${2}"

  mkdir -p "$( dirname "${kubeconfig}" )"
  ( umask 077; kubectl get secret/"${secret_name}" --template='{{ .data.kubeconfig }}' | base64 -d > "${kubeconfig}" )

  # The e2e machinery creates its own namespaces; pointing the current context at a nonexistent one keeps an
  # unqualified command from silently acting on `default`.
  kubectl --kubeconfig="${kubeconfig}" config set-context --current --namespace 'default-unexisting-namespace'

  # Sanity check.
  kubectl --kubeconfig="${kubeconfig}" version -o yaml
  kubectl --kubeconfig="${kubeconfig}" config view
}

# provision-storage-bucket creates a StorageBucket from the manifest on stdin, waits for it and extracts its
# credentials into a file. The backend-specific part is the manifest, so the caller owns it. The bucket name is in the
# object's status.bucketName afterwards.
# $1 - name of the StorageBucket object, as in the manifest
# $2 - path to write the credentials file to
function provision-storage-bucket {
  local object_name="${1}"
  local credentials="${2}"

  kubectl create -f -

  timeout -v 5m bash -c "until kubectl wait --for=condition=Degraded=False storagebucket/'${object_name}' --timeout=5m && kubectl wait --for=condition=Progressing=False storagebucket/'${object_name}' --timeout=5m; do sleep 1; done"

  # Every backend writes exactly one key into the credentials Secret, under a backend-specific name, so this takes
  # whatever is there rather than knowing the name.
  local secret_name
  secret_name="$( kubectl get storagebucket/"${object_name}" --template='{{ .spec.credentialsSecret.name }}' )"
  mkdir -p "$( dirname "${credentials}" )"
  ( umask 077; kubectl get secret/"${secret_name}" --template='{{ range .data }}{{ . }}{{ end }}' | base64 -d > "${credentials}" )
}
