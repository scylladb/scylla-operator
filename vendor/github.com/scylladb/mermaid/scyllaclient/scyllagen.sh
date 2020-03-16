#!/usr/bin/env bash
#
# Copyright (C) 2017 ScyllaDB
#

set -eu -o pipefail

if [[ -z ${SCYLLA_API_DOC} ]]; then
    echo "set SCYLLA_API_DOC env variable pointing to Scylla api/api-doc folder"
    exit 1
fi

jq -M -s 'reduce .[] as $x (.[0]; .apis += $x.apis? | .models += $x.models) | .resourcePath="/"' ${SCYLLA_API_DOC}/*.json \
| sed -e 's/#\/utils\///g' \
| sed -e 's/"bool"/"boolean"/g' \
| sed -e 's/"int"/"integer"/g' \
> scylla_12.json

echo "Go to https://www.apimatic.io/transformer transform '$(pwd)/scylla_12.json' to scylla.json"
