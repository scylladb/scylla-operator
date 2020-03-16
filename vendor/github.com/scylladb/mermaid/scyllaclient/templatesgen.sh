#!/usr/bin/env bash
#
# Copyright (C) 2017 ScyllaDB
#

set -eu -o pipefail

BASE_URL="https://raw.githubusercontent.com/go-swagger/go-swagger/master/generator/templates"

curl -Ss -o "internal/templates/client.gotmpl" "${BASE_URL}/client/client.gotmpl"
