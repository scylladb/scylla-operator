#!/bin/bash
#
# Copyright (C) 2021 ScyllaDB
#
# This is a helper file for our CI to map branch names and tags into image tags.

function tag_from_gh_ref {
    tmp_file=$( mktemp )
    echo "${1}" > "${tmp_file}"
    (
        # master
        sed -i -E -e '/^master$/,${s//latest/;b};$q1' "${tmp_file}" || \
        # branch name
        sed -i -E -e '/^v([0-9]+\.[0-9]+)$/,${s//\1/;b};$q1' "${tmp_file}" || \
        # tag name
        sed -i -E -e '/^v([0-9]+\.[0-9]+\.[0-9]+-(alpha|beta|rc)\.[0-9]+)$/,${s//\1/;b};$q1' "${tmp_file}" || \
        false
    ) && cat "${tmp_file}"
}

if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    set -euEo pipefail

    message_prefix="Testing mapping branch names and tags into image tags"

    echo "${message_prefix}..." >&2

    function err_handler {
        echo "${message_prefix}...FAILED! Error on line ${1}. Use \`bash -x ${0}\` to debug it further." >&2
    }
    trap 'err_handler "${LINENO}"' ERR

    ( (tag_from_gh_ref '' && exit 1) || true )
    ( (tag_from_gh_ref 'foo' && exit 1) || true )
    ( (tag_from_gh_ref 'masters' && exit 1) || true )
    # Fail on final tags, those shouldn't be rebuild by CI but instead promoted from a previous tested image.
    ( (tag_from_gh_ref 'v1.0.0' && exit 1) || true )
    ( (tag_from_gh_ref 'v1.0.0-alpha' && exit 1) || true )
    ( (tag_from_gh_ref 'v1.0.0-alpha.' && exit 1) || true )
    ( (tag_from_gh_ref 'v1.0.0-alpha0' && exit 1) || true )
    ( (tag_from_gh_ref 'v1.0.0.0' && exit 1) || true )
    [[ "$( tag_from_gh_ref 'master' )" == 'latest' ]]
    [[ "$( tag_from_gh_ref 'v1.0' )" == '1.0' ]]
    [[ "$( tag_from_gh_ref 'v1.0.0-alpha.0' )" == '1.0.0-alpha.0' ]]
    [[ "$( tag_from_gh_ref 'v1.0.0-beta.0' )" == '1.0.0-beta.0' ]]
    [[ "$( tag_from_gh_ref 'v1.0.0-rc.0' )" == '1.0.0-rc.0' ]]

    echo "${message_prefix}..SUCCESS." >&2
fi;
