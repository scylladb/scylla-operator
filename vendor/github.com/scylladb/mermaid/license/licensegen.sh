#!/usr/bin/env bash
#
# Copyright (C) 2017 ScyllaDB
#

set -eu -o pipefail

export GO111MODULE=on

for pkg in `go list -f '{{ join .Deps  "\n"}}' $1 | cut -d/ -f1-3 | sort | uniq` ; do
    # Ignore stdlib packages
    if [[ ${pkg} =~ ^[a-z]+(/|$) ]]; then
        continue
    fi
    # Ignore subproject packages
    if [[ ${pkg} =~ github\.com/scylladb/ ]]; then
        continue
    fi
    # Ignore internal packages
    if [[ ${pkg} =~ /internal(/|$) ]]; then
        continue
    fi

    l=`license-detector -f json "../vendor/$pkg" | jq -r '.[0].matches[0].license'`
    echo -e "This product includes software from ${pkg} licensed under ${l} license."
done
