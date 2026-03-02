#!/bin/bash

set -euEo pipefail
shopt -s inherit_errexit

# This script deletes the KinD cluster specified by CLUSTER_NAME.

# Ensure all kind calls use podman.
export KIND_EXPERIMENTAL_PROVIDER=podman

if [ -z "${CLUSTER_NAME}" ]; then
  echo "CLUSTER_NAME must be set" > /dev/stderr
  exit 1
fi

kind delete cluster --name="${CLUSTER_NAME}"
