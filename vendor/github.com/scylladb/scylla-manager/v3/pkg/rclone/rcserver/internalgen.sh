#!/usr/bin/env bash
#
# Copyright (C) 2017 ScyllaDB
#

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )
cd "${SCRIPT_DIR}" || exit 1
rm -f internal/rclone_supported_calls.go
jq '.paths | [keys[] | select(. | startswith("/rclone")) | sub("^/rclone/"; "")]' $(git rev-parse --show-toplevel)/swagger/agent.json | \
  go run internal/templates/jsontemplate.go internal/templates/rclone_supported_calls.gotmpl > \
  internal/rclone_supported_calls.go