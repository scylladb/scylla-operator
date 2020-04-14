#!/usr/bin/env bash
#
# Copyright (C) 2017 ScyllaDB
#

set -eu -o pipefail

echo "Swagger $(swagger version)"

rm -rf internal/scylla/client internal/scylla/models
swagger generate client -A scylla -T internal/templates -f scylla.json -t ./internal/scylla

rm -rf internal/scylla_v2/client internal/scylla_v2/models
swagger generate client -A scylla2 -T internal/templates -f scylla_v2.json -t ./internal/scylla_v2

rm -rf internal/agent/client internal/agent/models
swagger generate client -A agent -T internal/agent/templates -f agent.json -t ./internal/agent