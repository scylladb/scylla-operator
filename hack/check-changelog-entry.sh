#!/usr/bin/env bash

set -euo pipefail

if [[ $# -ne 2 ]]; then
    echo "Usage: $0 <ORG/REPO> <PR_NUMBER>" >&2
    exit 1
fi

REPO="$1"
PR_NUMBER="$2"

if grep -q "\[#${PR_NUMBER}\](https://github.com/${REPO}/pull/${PR_NUMBER})" CHANGELOG.md; then
    echo "Found changelog entry for PR #${PR_NUMBER}."
    exit 0
fi

echo "ERROR: Missing changelog entry for PR #${PR_NUMBER}." >&2
exit 1
