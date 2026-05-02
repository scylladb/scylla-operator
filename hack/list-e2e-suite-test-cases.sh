#!/usr/bin/env bash

# list-e2e-suite-test-cases.sh prints the test cases (Ginkgo spec FullTexts)
# selected by a given scylla-operator test suite, one per line, sorted and
# deduplicated.
#
# Usage:
#   hack/list-e2e-suite-test-cases.sh <exact-suite-name>
#
# The suite name must match an entry in pkg/cmd/tests/tests.go exactly; no
# aliases are accepted.

set -euEo pipefail
shopt -s inherit_errexit

if [[ $# -ne 1 ]]; then
    echo "Usage: $0 <exact-suite-name>" >&2
    exit 2
fi

suite="$1"

for dep in go jq; do
    if ! command -v "${dep}" >/dev/null 2>&1; then
        echo "${dep} is required but not found in PATH" >&2
        exit 2
    fi
done

repo_root="$( cd "$( dirname "${BASH_SOURCE[0]}" )/.." && pwd )"

tmp_dir="$( mktemp -d )"
trap 'rm -rf "${tmp_dir}"' EXIT

# The binary's output is captured and only replayed to stderr if the run fails,
# so a successful invocation produces only the spec list on stdout and nothing on stderr.
binary_log="${tmp_dir}/scylla-operator-tests.log"
if ! (
    cd "${repo_root}"
    go run ./cmd/scylla-operator-tests run \
        --dry-run \
        --artifacts-dir="${tmp_dir}" \
        "${suite}"
) > "${binary_log}" 2>&1; then
    echo "scylla-operator-tests dry-run for suite ${suite} failed; output:" >&2
    cat "${binary_log}" >&2
    exit 1
fi

# Extract FullText: ContainerHierarchyTexts joined by " " + " " + LeafNodeText.
jq -r '
    .[0].SpecReports[]
    | select(.LeafNodeType == "It" and .State == "passed")
    | (((.ContainerHierarchyTexts // []) + [.LeafNodeText]) | join(" "))
' "${tmp_dir}/e2e.json" \
    | sort -u
