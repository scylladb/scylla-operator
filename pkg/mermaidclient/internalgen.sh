#!/usr/bin/env bash
#
# Copyright (C) 2017 ScyllaDB
#

set -eu -o pipefail

echo "Swagger $(swagger version)"

rm -rf internal/client internal/models
swagger generate client -A mermaid -f mermaid.json -t ./internal
