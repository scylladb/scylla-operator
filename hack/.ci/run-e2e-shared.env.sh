#!/bin/bash
#
# Copyright (C) 2025 ScyllaDB
#

set -euExo pipefail
shopt -s inherit_errexit

crypto_key_buffer_size_multiplier=1
if [[ -n "${SO_E2E_PARALLELISM:-}" && "${SO_E2E_PARALLELISM}" -ne 0 ]]; then
  crypto_key_buffer_size_multiplier="${SO_E2E_PARALLELISM}"
fi

SO_CRYPTO_KEY_SIZE="${SO_CRYPTO_KEY_SIZE:-2048}"
export SO_CRYPTO_KEY_SIZE
SO_CRYPTO_KEY_BUFFER_SIZE_MIN="${SO_CRYPTO_KEY_BUFFER_SIZE_MIN:-$(( 6 * "${crypto_key_buffer_size_multiplier}" ))}"
export SO_CRYPTO_KEY_BUFFER_SIZE_MIN
SO_CRYPTO_KEY_BUFFER_SIZE_MAX="${SO_CRYPTO_KEY_BUFFER_SIZE_MAX:-$(( 10 * "${crypto_key_buffer_size_multiplier}" ))}"
export SO_CRYPTO_KEY_BUFFER_SIZE_MAX

SCYLLA_OPERATOR_FEATURE_GATES="${SCYLLA_OPERATOR_FEATURE_GATES:-AllAlpha=true,AllBeta=true}"
export SCYLLA_OPERATOR_FEATURE_GATES

SO_SCYLLACLUSTER_STORAGECLASS_NAME="${SO_SCYLLACLUSTER_STORAGECLASS_NAME=scylladb-local-xfs}"
export SO_SCYLLACLUSTER_STORAGECLASS_NAME

