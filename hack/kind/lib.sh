#!/bin/bash
#
# Copyright (C) 2026 ScyllaDB

# Helper functions for KIND-based E2E testing

shopt -s inherit_errexit

# Local registry backing KIND clusters. Set up by hack/kind/cluster-setup.sh and shared with the build helpers below.
KIND_REGISTRY_NAME="kind-registry"
KIND_REGISTRY_PORT="${KIND_REGISTRY_PORT:-5001}"
KIND_REGISTRY_HOST="localhost"

# build-and-push-operator-image builds the operator image and pushes it to the local registry, storing (and exporting)
# the reference in the variable named by $2. No-op if that variable is already set.
# Usage: build-and-push-operator-image <root_dir> <image_var_name>
function build-and-push-operator-image {
  local root_dir="${1:?Missing root dir}"
  local -n image_ref="${2:?Missing image variable name}"

  if [ -n "${image_ref:-}" ]; then
    echo "Using existing operator image: ${image_ref}"
    return
  fi

  local tag_ref="${KIND_REGISTRY_HOST}:${KIND_REGISTRY_PORT}/scylladb/scylla-operator:latest"

  echo "Building operator image: ${tag_ref}"
  podman build --format docker -t "${tag_ref}" -f "${root_dir}/Dockerfile" "${root_dir}"

  # Push the image to the local registry. Use --tls-verify=false as we're running local registry without TLS.
  echo "Pushing operator image to local registry: ${tag_ref}"
  local digestfile
  digestfile="$( mktemp )"
  podman push --tls-verify=false --digestfile="${digestfile}" "${tag_ref}"

  # Deploy by digest: the ref changes with image content, so the kubelet's IfNotPresent cache never serves
  # a stale image on a reused cluster, while unchanged images stay cached (no pull-policy changes needed).
  image_ref="${KIND_REGISTRY_HOST}:${KIND_REGISTRY_PORT}/scylladb/scylla-operator@$( cat "${digestfile}" )"
  rm -f "${digestfile}"
  export "${2}"
  echo "Operator image reference: ${image_ref}"
}
