#!/usr/bin/env bash
#
# Copyright (C) 2021 ScyllaDB
#
# This is a helper file for our CI to map current tag to a corresponding previous tag.

function sort_semver_decs {
    sed -e 's/-/~/' | sort -V -r | sed -e 's/~/-/'
}

function is_lower_semver {
    if [[ "$#" != 2 ]]; then
        exit 1
    fi

    lhs="${1}"
    rhs="${2}"

    if [[ "${lhs}" == "${rhs}" ]]; then
        return 1
    fi

    higher="$( echo -e "${lhs}\n${rhs}" | sort_semver_decs | head -n1 )"
    if [[ "${higher}" == "${lhs}" ]]; then
        return 1
    fi

    return 0
}

function find_previous_semver {
    if [[ "$#" != 2 ]]; then
        exit 1
    fi

    list=$( echo "${1}" | tr ' ' '\n' | sort_semver_decs )
    target="${2}"

    for v in ${list}; do
        if is_lower_semver "${v}" "${target}"; then
            echo "${v}"
            return 0
        fi
    done

    return 1
}

if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    set -euEo pipefail
    shopt -s inherit_errexit

    message_prefix="Testing helpers"

    echo "${message_prefix}..." >&2

    function err_handler {
        echo "${message_prefix}...FAILED! Error on line ${1}. Use \`bash -x ${0}\` to debug it further." >&2
        exit 1
    }
    trap 'err_handler "${LINENO}"' ERR

    if is_lower_semver 'v1.2.0-alpha.0' 'v1.2.0-alpha.0'; then false; fi
    is_lower_semver 'v1.2.0-alpha.0' 'v1.2.0-alpha.1'
    if is_lower_semver 'v1.2.0-alpha.1' 'v1.2.0-alpha.0'; then false; fi
    is_lower_semver 'v1.2.0-alpha.0' 'v1.2.0-alpha.1'
    is_lower_semver 'v1.2.0-alpha.1' 'v1.2.0-alpha.10'
    is_lower_semver 'v1.2.0-alpha.1' 'v1.2.0-beta.1'
    is_lower_semver 'v1.2.0-beta.1' 'v1.2.0-rc.1'
    is_lower_semver 'v1.2.0-alpha.1' 'v1.3.0-alpha.0'
    if is_lower_semver 'v1.3.0-alpha.0' 'v1.2.0-alpha.1'; then false; fi
    is_lower_semver 'v1.2.0-alpha.1' 'v1.2.0'
    is_lower_semver 'v1.2.0-beta.1' 'v1.2.0'
    is_lower_semver 'v1.2.0-rc.1' 'v1.2.0'
    is_lower_semver 'v1.2.0' 'v1.2.1-rc.1'

    [[ "$( find_previous_semver 'v1.0.0 v1.1.0' 'v1.2.0' )" == "v1.1.0" ]]
    [[ "$( find_previous_semver 'v1.1.0 v1.0.0' 'v1.2.0' )" == "v1.1.0" ]]

    echo "${message_prefix}..SUCCESS." >&2
fi
