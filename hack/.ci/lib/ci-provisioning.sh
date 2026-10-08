#!/usr/bin/env bash
#
# Copyright (C) 2026 ScyllaDB
#
# Helpers for the CI provisioning objects (ci.scylladb.com) the GitHub Actions workflows create.

set -euExo pipefail
shopt -s inherit_errexit

# get-provisioned-cluster-kubeconfig extracts a provisioned cluster's kubeconfig from its Secret into a file and sanity
# checks it.
# $1 - path to the CI provisioners cluster kubeconfig
# $2 - name of the Secret holding the kubeconfig
# $3 - path to write the kubeconfig to
function get-provisioned-cluster-kubeconfig {
  local provisioners_kubeconfig="${1}"
  local secret_name="${2}"
  local kubeconfig="${3}"

  mkdir -p "$( dirname "${kubeconfig}" )"
  ( umask 077; kubectl --kubeconfig="${provisioners_kubeconfig}" get secret/"${secret_name}" --template='{{ .data.kubeconfig }}' | base64 -d > "${kubeconfig}" )

  # The e2e machinery creates its own namespaces; pointing the current context at a nonexistent one keeps an
  # unqualified command from silently acting on `default`.
  kubectl --kubeconfig="${kubeconfig}" config set-context --current --namespace 'default-unexisting-namespace'

  # Sanity check. A freshly provisioned cluster's API can drop the first connections, so it gets a few attempts.
  timeout --verbose 1m bash -c 'until kubectl --kubeconfig="${0}" version -o yaml; do sleep 5; done' "${kubeconfig}"
  kubectl --kubeconfig="${kubeconfig}" config view
}
