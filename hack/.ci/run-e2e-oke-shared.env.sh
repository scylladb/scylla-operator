#!/usr/bin/env bash
# Copyright (C) 2026 ScyllaDB

set -euExo pipefail
shopt -s inherit_errexit

_oke_shared_env_parent_dir="$( dirname "${BASH_SOURCE[0]}" )"

SO_NODECONFIG_PATH="${SO_NODECONFIG_PATH=${_oke_shared_env_parent_dir}/manifests/cluster/nodeconfig-oke.yaml}"
export SO_NODECONFIG_PATH

SO_CSI_DRIVER_PATH="${SO_CSI_DRIVER_PATH=${_oke_shared_env_parent_dir}/manifests/namespaces/local-csi-driver/}"
export SO_CSI_DRIVER_PATH

# TODO: Remove object-storage entries from SO_SKIPPED_TESTS when https://scylladb.atlassian.net/browse/OPERATOR-83 is resolved.
# TODO: Remove coredumps entry from SO_SKIPPED_TESTS when https://scylladb.atlassian.net/browse/OPERATOR-116 is resolved.
SO_SKIPPED_TESTS="${SO_SKIPPED_TESTS=\
ScyllaDBManagerTask and ScyllaDBDatacenter integration with global ScyllaDB Manager should synchronise a backup task and support a manual restore procedure|\
Scylla Manager integration should register cluster, sync backup tasks and support manual restore procedure|\
Scylla Manager integration should discover cluster and sync errors for invalid tasks and invalid updates to existing tasks|\
ScyllaCluster coredumps should persist a coredump when ScyllaDB crashes with SIGABRT}"
export SO_SKIPPED_TESTS
