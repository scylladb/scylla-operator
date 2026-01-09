#!/usr/bin/env bash

set -euo pipefail

function load_image_into_kind() {
    local image=$1
    if ! kind load docker-image "${image}" --name "${KIND_CLUSTER_NAME}"; then
        echo "Failed to load image ${image} into KinD cluster ${KIND_CLUSTER_NAME}"
        exit 1
    fi
}

function extract_images_from_manifests() {
  local files=("$@")
  local images

  images=($(
    grep -hoE 'image:\s*["'\'']?[^"'\''[:space:]]+' "${files[@]}" 2>/dev/null \
      | awk -F'image:' '{print $2}' \
      | tr -d '"' | tr -d "'" | tr -d ' ' \
      | sort -u
  ))

  echo "${images[@]}"
}
