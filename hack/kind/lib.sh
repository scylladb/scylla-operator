#!/bin/bash
#
# Copyright (C) 2026 ScyllaDB

# Helper functions for KIND-based E2E testing

shopt -s inherit_errexit

# build-and-push-operator-image builds the operator image and pushes it to the local registry if SO_IMAGE is not set.
# It sets and exports SO_IMAGE with the built image reference.
function build-and-push-operator-image {
  if [ -z "${1:-}" ]; then
    echo "Missing parent directory argument.\nUsage: ${FUNCNAME[0]} <repo_root>" > /dev/stderr
    exit 2
  fi

  local repo_root="${1}"

  # If SO_IMAGE is not set, build the image.
  if [ -z "${SO_IMAGE:-}" ]; then
    SO_IMAGE="localhost:5001/scylladb/scylla-operator:e2e-$( date +%Y%m%d%H%M%S )"
    export SO_IMAGE

    echo "Building operator image: ${SO_IMAGE}"
    podman build --format docker -t "${SO_IMAGE}" -f "${repo_root}/Dockerfile" "${repo_root}"

    # Push the image to the local registry. Use --tls-verify=false as we're running local registry without TLS.
    echo "Pushing operator image to local registry: ${SO_IMAGE}"
    podman push --tls-verify=false "${SO_IMAGE}"
  else
    echo "Using existing operator image: ${SO_IMAGE}"
  fi
}
