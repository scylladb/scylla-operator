#!/usr/bin/env bash
#
# Copyright (C) 2026 ScyllaDB
#
# This script runs the preflight checks of an image against its Red Hat certification component and, with SUBMIT, submits
# the results. Preflight checks every platform of a manifest list and reports a failed check in the results, not the
# exit code.
# Pyxis rejects the results for a digest that's already certified, but still adds the tag to it, so a rejected
# submission counts as a success once the certified image of that digest has the tag in Pyxis.
#
# Usage: certify-image.sh <image_repo_ref> <image_tag> <component_id>
# Environment:
#   PFLT_PYXIS_API_TOKEN  required, Red Hat Container API key, read by preflight. Without SUBMIT it only selects the
#                         component's policy, which the default one differs from.
#   SUBMIT                optional, "true" submits the results, defaults to "false"

set -euExo pipefail
shopt -s inherit_errexit

if [[ "$#" -ne 3 ]]; then
  echo -e "Invalid arguments.\nUsage: ${0} <image_repo_ref> <image_tag> <component_id>" >&2
  exit 1
fi

image_repo_ref="${1}"
image_tag="${2}"
component_id="${3}"
submit="${SUBMIT:-false}"

# The token is only ever expanded with the trace off, so the check expands it to a placeholder.
if [[ -z "${PFLT_PYXIS_API_TOKEN:+set}" ]]; then
  echo "Error: PFLT_PYXIS_API_TOKEN must not be empty." >&2
  exit 1
fi

temp_dir="$( mktemp -d )"
trap 'rm -rf "${temp_dir}"' EXIT

export PFLT_ARTIFACTS="${temp_dir}"
preflight_exit=0
preflight check container "${image_repo_ref}:${image_tag}" --certification-component-id="${component_id}" --submit="${submit}" || preflight_exit="$?"

for results in "${PFLT_ARTIFACTS}"/*/results.json; do
  if [[ "$( jq '.passed' "${results}" )" != "true" ]]; then
    echo "Error: Preflight checks haven't passed on $( basename "$( dirname "${results}" )" ): $( jq -r '[.results.failed[].name] | join(", ")' "${results}" )." >&2
    exit 1
  fi
done

if [[ "${preflight_exit}" -eq 0 || "${submit}" != "true" ]]; then
  exit "${preflight_exit}"
fi

# The manifest goes through a file, since a command substitution would strip its trailing newlines and change the digest.
skopeo inspect --raw "docker://${image_repo_ref}:${image_tag}" > "${temp_dir}/manifest"
digest="sha256:$( sha256sum < "${temp_dir}/manifest" | cut -d ' ' -f 1 )"

{ set +x; } 2> /dev/null
total="$( curl --fail --silent --show-error --get \
  --header "X-API-KEY: ${PFLT_PYXIS_API_TOKEN}" \
  --data-urlencode "filter=certified==true;repositories=em=(manifest_list_digest==\"${digest}\";tags=em=(name==\"${image_tag}\"))" \
  --data-urlencode 'include=total' \
  "https://catalog.redhat.com/api/containers/v1/repositories/registry/${image_repo_ref%%/*}/repository/${image_repo_ref#*/}/images" \
  | jq -e '.total' )"
set -x
if [[ "${total}" -eq 0 ]]; then
  exit 1
fi
echo "::notice::${digest} was already certified, preflight added the ${image_tag} tag to it."
