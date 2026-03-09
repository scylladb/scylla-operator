#!/usr/bin/env bash

# This scripts runs renovate in dry-run mode against the local config.
# It can be used to check if changes to the config result in the expected updates.

# Note: Renovate CLI output is quote verbose, so it's recommended to redirect it to a file for easier analysis.

set -euxEo pipefail
shopt -s inherit_errexit

npx --yes renovate --platform=local --dry-run=full --print-config=true
