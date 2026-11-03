#!/usr/bin/env bash
#
# Copyright (C) 2026 ScyllaDB
#
# This script merges the `auths` of a containers-auth.json(5) file into `config.json` in DOCKER_CONFIG, or in
# `~/.docker` when it's unset, and prints the path of that file. Other fields of the file, like `credHelpers`, are
# ignored.
#
# Usage: write-registry-auth.sh
# Environment:
#   AUTH_JSON      required, contents of a containers-auth.json(5) file.
#   DOCKER_CONFIG  optional, directory of the config, defaults to `~/.docker`.

# No xtrace, the auth is the credential.
set -euEo pipefail
shopt -s inherit_errexit

if [[ "$#" -ne 0 ]]; then
  echo -e "Invalid arguments.\nUsage: ${0}" >&2
  exit 1
fi

if [[ -z "${AUTH_JSON//[[:space:]]/}" ]]; then
  echo '::error::The registry auth is empty. GitHub does not pass secrets to runs of pull requests from forks. If this is a pull request from a fork, this is expected and needs no action from you: a maintainer will run the e2e suites for it.' >&2
  exit 1
fi

if ! jq -ne 'env.AUTH_JSON | fromjson | .auths | objects | length > 0' &> /dev/null; then
  echo '::error::The registry auth is not a containers-auth.json(5) file with a non-empty "auths" object.' >&2
  exit 1
fi

# containers-auth.json(5) shares the "auths" format with docker's config.json, so buildx picks it up as is.
umask 077
config_dir="${DOCKER_CONFIG:-${HOME}/.docker}"
mkdir -p "${config_dir}"
config="${config_dir}/config.json"
if [[ ! -f "${config}" ]]; then
  echo '{}' > "${config}"
fi
jq '.auths += (env.AUTH_JSON | fromjson | .auths)' "${config}" > "${config}.tmp"
mv "${config}.tmp" "${config}"

echo "${config}"
