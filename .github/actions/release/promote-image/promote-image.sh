#!/usr/bin/env bash
#
# Copyright (C) 2026 ScyllaDB
#
# This script copies an image to a repository under the given version and rolling tags. A version tag must never move
# to a different image, so an existing one has to hold the same digest already.

set -euExo pipefail
shopt -s inherit_errexit

if [[ "$#" -lt 3 ]]; then
  echo -e "Invalid arguments.\nUsage: ${0} <digest_pinned_source_image_ref> <image_repo_ref> <version> [<rolling_tag>...]" >&2
  exit 1
fi

source_image_ref="${1}"
image_repo_ref="${2}"
version="${3}"
rolling_tags=( "${@:4}" )

if [[ "${source_image_ref}" != *@sha256:* ]]; then
  echo "Error: Source image '${source_image_ref}' isn't pinned by digest." >&2
  exit 1
fi
source_digest="${source_image_ref#*@}"

temp_dir="$( mktemp -d )"
trap 'rm -rf "${temp_dir}"' EXIT

# Tag listings can lag behind pushes, so the version tag is looked up directly. Only a missing manifest means it doesn't
# exist yet. The manifest goes through a file, since a command substitution would strip its trailing newlines and
# change the digest.
if err="$( skopeo inspect --raw "docker://${image_repo_ref}:${version}" 2>&1 > "${temp_dir}/manifest" )"; then
  digest="sha256:$( sha256sum < "${temp_dir}/manifest" | cut -d ' ' -f 1 )"
  if [[ "${digest}" != "${source_digest}" ]]; then
    echo "Error: ${image_repo_ref}:${version} already holds ${digest}, not ${source_digest}." >&2
    exit 1
  fi
elif ! grep -q 'manifest unknown' <<< "${err}"; then
  echo "${err}" >&2
  exit 1
fi

# --preserve-digests fails the copy rather than letting skopeo rewrite a manifest into a different digest.
for image_tag in "${version}" "${rolling_tags[@]}"; do
  skopeo copy --all --preserve-digests "docker://${source_image_ref}" "docker://${image_repo_ref}:${image_tag}"
done
