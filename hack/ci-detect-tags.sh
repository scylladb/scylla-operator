#!/bin/bash
#
# Copyright (C) 2021 ScyllaDB
#
# This script will set appropriate tag variables in the CI environment.
# Multiple tags can point to one commit which makes `git describe` ambiguous
# for the purposes of promotion. Like when vX.Y.Z-beta.0 and vX.Y.Z-rc.0 point
# to the same commit. Git describe could return any of those and even if it would
# return the newest, the job order is not determined either. And jobs can be rerun
# which would make the beta job promoting to rc.0.
set -euExo pipefail
shopt -s inherit_errexit

parent_dir=$( dirname "${BASH_SOURCE[0]}" )
source "${parent_dir}/lib/tag-from-gh-ref.sh"

tag=$( tag_from_gh_ref "${GITHUB_REF}" )
git_sha=$( git rev-parse --short=7 "HEAD^{commit}" )

echo "GIT_TAG=${tag}-0-g${git_sha}" | tee -a ${GITHUB_ENV}
echo "GIT_TAG_SHORT=${tag}" | tee -a ${GITHUB_ENV}
