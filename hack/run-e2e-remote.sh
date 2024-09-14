#!/usr/bin/env bash
#
# Copyright (C) 2024 ScyllaDB
#

set -euExo pipefail
shopt -s inherit_errexit

source "$( dirname "${BASH_SOURCE[0]}" )/.ci/lib/e2e.sh"

KUBECONFIG="${KUBECONFIGS[0]}" apply-e2e-workarounds
KUBECONFIG="${KUBECONFIGS[0]}" run-e2e
