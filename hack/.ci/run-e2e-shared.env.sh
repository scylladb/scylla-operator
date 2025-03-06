#!/bin/bash
#
# Copyright (C) 2025 ScyllaDB
#

set -euExo pipefail
shopt -s inherit_errexit

SO_SCYLLA_OPERATOR_REPLICAS="${SO_SCYLLA_OPERATOR_REPLICAS:-1}"
export SO_SCYLLA_OPERATOR_REPLICAS
SO_SCYLLA_OPERATOR_RESOURCE_REQUESTS_CPU="${SO_SCYLLA_OPERATOR_RESOURCE_REQUEST_CPU:-4}"
export SO_SCYLLA_OPERATOR_RESOURCE_REQUESTS_CPU
SO_SCYLLA_OPERATOR_RESOURCE_REQUESTS_MEMORY="${SO_SCYLLA_OPERATOR_RESOURCE_REQUESTS_MEMORY:-2Gi}"
export SO_SCYLLA_OPERATOR_RESOURCE_REQUESTS_MEMORY
SO_SCYLLA_OPERATOR_RESOURCE_LIMITS_CPU="${SO_SCYLLA_OPERATOR_RESOURCE_REQUEST_CPU:-4}"
export SO_SCYLLA_OPERATOR_RESOURCE_LIMITS_CPU
SO_SCYLLA_OPERATOR_RESOURCE_LIMITS_MEMORY="${SO_SCYLLA_OPERATOR_RESOURCE_LIMITS_MEMORY:-2Gi}"
export SO_SCYLLA_OPERATOR_RESOURCE_LIMITS_MEMORY

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
