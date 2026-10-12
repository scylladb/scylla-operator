#!/usr/bin/env bash
#
# Copyright (C) 2026 ScyllaDB
#
# This script decides where the image to release comes from. For a tag that already exists, it reads the tested image
# from the tag. Otherwise the tag will be created on the given ref, so the ref has to be the release branch of the
# minor. It prints the results as key=value lines.

set -euExo pipefail
shopt -s inherit_errexit

if [[ "$#" -ne 5 ]]; then
  echo -e "Invalid arguments.\nUsage: ${0} <repository> <tag> <minor> <ref> <dev_image_repo_ref>" >&2
  exit 1
fi

repository="${1}"
tag="${2}"
minor="${3}"
ref="${4}"
dev_image_repo_ref="${5}"

if [[ -z "$( git ls-remote --tags --refs "https://github.com/${repository}.git" "refs/tags/${tag}" )" ]]; then
  if [[ "${ref}" != "refs/heads/v${minor}" ]]; then
    echo "Error: Tag '${tag}' has to be created on branch 'v${minor}', not '${ref}'." >&2
    exit 1
  fi

  echo "tag-exists=false"
  echo "image-ref="
  exit 0
fi

# The release writes the digest of the tested image into the tag's annotation as an `Image-Digest:` trailer. A tag
# without one wasn't made by the release, so nothing records that it passed the gate.
tag_object="$( gh api "repos/${repository}/git/ref/tags/${tag}" --jq '.object | select(.type == "tag") | .sha' )"
tag_commit=''
image_digest=''
if [[ -n "${tag_object}" ]]; then
  tag_json="$( gh api "repos/${repository}/git/tags/${tag_object}" )"
  tag_commit="$( jq -r .object.sha <<< "${tag_json}" )"
  image_digest="$( jq -r .message <<< "${tag_json}" | sed -n -e 's/^Image-Digest: //p' )"
fi
if [[ ! "${image_digest}" =~ ^sha256:[0-9a-f]{64}$ ]]; then
  echo "Error: Tag '${tag}' records no tested image, so it wasn't created by the release." >&2
  exit 1
fi
image_ref="${dev_image_repo_ref}@${image_digest}"

# The image records the revision it was built from, and the digest covers that record.
revision="$( skopeo inspect --raw "docker://${image_ref}" | jq -r '.annotations["org.opencontainers.image.revision"] // ""' )"
if [[ "${revision}" != "${tag_commit}" ]]; then
  echo "Error: Tag '${tag}' points at ${tag_commit}, but its image was built from '${revision}'." >&2
  exit 1
fi

echo "tag-exists=true"
echo "image-ref=${image_ref}"
