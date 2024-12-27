#!/usr/bin/env bash
#
# Copyright (C) 2024 ScyllaDB
#

set -euExo pipefail
shopt -s inherit_errexit
shopt -s extglob

if [ -z "${ARTIFACTS+x}" ]; then
  echo "ARTIFACTS can't be empty" > /dev/stderr
  exit 2
fi

source "$( dirname "${BASH_SOURCE[0]}" )/lib/e2e.sh"
parent_dir="$( dirname "${BASH_SOURCE[0]}" )"

trap gather-artifacts-on-exit EXIT

SO_NODECONFIG_PATH="${SO_NODECONFIG_PATH=${parent_dir}/manifests/cluster/nodeconfig.yaml}"
export SO_NODECONFIG_PATH

# We need to strip the tag because skopeo doesn't support references with both a tag and a digest.
SO_IMAGE="${SO_IMAGE/:*([^:\/])@/@}"
echo "${SO_IMAGE}"
ARTIFACTS_DEPLOY_DIR="${ARTIFACTS}/deploy" timeout --foreground -v 10m "${parent_dir}/../ci-deploy-release.sh" "${SO_IMAGE}"
