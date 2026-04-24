#!/usr/bin/env bash

# verify-e2e-suite-coverage.sh asserts that every Ginkgo spec in the e2e test
# tree is selected by at least one registered scylla-operator test suite (other
# than "all"). It guards the pure-suite-label model: a spec missing its suite
# label would be silently excluded from every CI run while still being picked
# up by the special "all" suite.

set -euEo pipefail
shopt -s inherit_errexit

repo_root="$( cd "$( dirname "${BASH_SOURCE[0]}" )/.." && pwd )"
list_script="${repo_root}/hack/list-e2e-suite-test-cases.sh"

tmp_dir="$( mktemp -d )"
trap 'rm -rf "${tmp_dir}"' EXIT

suites="$( cd "${repo_root}" && go run ./cmd/scylla-operator-tests list-suites )"
if [[ -z "${suites}" ]]; then
    echo "scylla-operator-tests list-suites produced no output" >&2
    exit 1
fi

all_list="${tmp_dir}/all.list"
union_raw="${tmp_dir}/union.raw"
union_list="${tmp_dir}/union.list"

echo "Listing test cases for suite all..." >&2
"${list_script}" all > "${all_list}"
if [[ ! -s "${all_list}" ]]; then
    echo "the \"all\" suite produced no test cases; this is unexpected" >&2
    exit 1
fi

while IFS= read -r suite; do
    [[ "${suite}" == "all" ]] && continue
    echo "Listing test cases for suite ${suite}..." >&2
    "${list_script}" "${suite}" >> "${union_raw}"
done <<< "${suites}"

sort -u "${union_raw}" -o "${union_list}"

uncovered="$( comm -23 "${all_list}" "${union_list}" )"
if [[ -n "${uncovered}" ]]; then
    echo >&2
    echo "FAIL: the following test case(s) are not selected by any non-\"all\" suite:" >&2
    printf '  %s\n' "${uncovered}" >&2
    echo >&2
    echo "Every spec must be labeled with at least one suite. See test/e2e/framework/meta.go for the suite labels." >&2
    exit 1
fi

echo "All test cases are covered by at least one registered suite." >&2
