#!/usr/bin/env bash
#
# Copyright (C) 2023 ScyllaDB
#

set -euExo pipefail
shopt -s inherit_errexit

trap 'kill $( jobs -p ); exit 0' EXIT

if [ -z "${ARTIFACTS+x}" ]; then
  echo "ARTIFACTS can't be empty" > /dev/stderr
  exit 2
fi

source "$( dirname "${BASH_SOURCE[0]}" )/../lib/kube.sh"
source "$( dirname "${BASH_SOURCE[0]}" )/lib/e2e.sh"
source "$( dirname "${BASH_SOURCE[0]}" )/run-e2e-shared.env.sh"
parent_dir="$( dirname "${BASH_SOURCE[0]}" )"

trap gather-artifacts-on-exit EXIT
trap gracefully-shutdown-e2es INT

# Test cases including $test_disable_tag in their name will be skipped.
# TODO: Get rid of this tagging method in favor of defined test suites, and mapping
# specific test suites to specific runtime configurations.
test_disable_tag="TESTCASE_DISABLED_ON_OPENSHIFT"
SO_SKIPPED_TESTS="${SO_SKIPPED_TESTS:-$test_disable_tag}"
export SO_SKIPPED_TESTS

SO_NODECONFIG_PATH="${SO_NODECONFIG_PATH=${parent_dir}/manifests/cluster/nodeconfig-openshift-aws.yaml}"
export SO_NODECONFIG_PATH

SO_CSI_DRIVER_PATH="${SO_CSI_DRIVER_PATH=${parent_dir}/manifests/namespaces/local-csi-driver/}"
export SO_CSI_DRIVER_PATH

# TODO: When https://github.com/scylladb/scylla-operator/issues/2490 is completed,
# we should make sure we have all required CRDs in the OpenShift cluster.
SO_DISABLE_PROMETHEUS_OPERATOR="${SO_DISABLE_PROMETHEUS_OPERATOR:-true}"
export SO_DISABLE_PROMETHEUS_OPERATOR

SO_ENABLE_OPENSHIFT_USER_WORKLOAD_MONITORING="${SO_ENABLE_OPENSHIFT_USER_WORKLOAD_MONITORING:-true}"
export SO_ENABLE_OPENSHIFT_USER_WORKLOAD_MONITORING

run-deploy-script-in-all-clusters "${parent_dir}/../ci-deploy.sh"

apply-e2e-workarounds-in-all-clusters
run-e2e
