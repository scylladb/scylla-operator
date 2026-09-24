#!/usr/bin/env bash
#
# Copyright (C) 2026 ScyllaDB
#
# This script deploys scylla-operator and its dependencies using the Helm chart.
# Usage: ${0} <image_repository> <image_tag>

set -euxEo pipefail
shopt -s inherit_errexit

source "$( dirname "${BASH_SOURCE[0]}" )/lib/bash.sh"
source "$( dirname "${BASH_SOURCE[0]}" )/lib/kube.sh"
source "$( dirname "${BASH_SOURCE[0]}" )/lib/install.sh"

if [[ "$#" -ne 2 ]]; then
  echo -e "Missing arguments.\nUsage: ${0} <image_repository> <image_tag>" > /dev/stderr
  exit 1
fi

image_repository="${1}"
image_tag="${2}"

source_root="$( realpath "$( dirname "${BASH_SOURCE[0]}" )/.." )"

deploy-cert-manager "${source_root}"

helm upgrade --install scylla-operator "${source_root}/helm/scylla-operator" \
  --create-namespace \
  --namespace=scylla-operator \
  --set=image.repository="${image_repository}" \
  --set=image.tag="${image_tag}" \
  --set=replicas=1 \
  --set=webhookServerReplicas=1

wait-for-scylla-operator-rollout
