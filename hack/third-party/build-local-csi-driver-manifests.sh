#!/usr/bin/env bash
#
# Copyright (C) 2026 ScyllaDB
#
# This script builds the ScyllaDB Local CSI Driver manifests with a specified version from the manifests
# shipped upstream in https://github.com/scylladb/local-csi-driver/tree/master/deploy/kubernetes/local-csi-driver
# and applies the deliberate divergences this repository maintains on top of them.

set -euxEo pipefail
shopt -s inherit_errexit

source "$( dirname "${BASH_SOURCE[0]}" )/../lib/assets.sh"

if [[ -n "${1+x}" ]]; then
    target_dir="${1}"
else
    printf 'Missing target directory.\nUsage: %s <target-dir>\n' "$0" >&2
    exit 1
fi

# The volumes directory this repository uses. It has to match the mountPoint of the filesystem that NodeConfig
# provisions, which differs from the upstream default.
readonly volumes_dir="/var/lib/persistent-volumes"
readonly upstream_volumes_dir="/mnt/persistent-volumes"

# The driver only runs on nodes tuned by NodeConfig.
readonly node_type_label="scylla.scylladb.com/node-type"
readonly node_type="scylla"

# This repository splits the upstream provisioner ClusterRole into an aggregated one, so that platform specific
# permissions can be attached to it without patching the definition.
readonly aggregated_clusterrole_name="scylladb:csi-external-provisioner"
readonly aggregate_label="rbac.operator.scylladb.com/aggregate-to-csi-external-provisioner"

function get-url() {
    local ref="${1}"
    local file="${2}"
    echo "https://raw.githubusercontent.com/scylladb/local-csi-driver/refs/tags/${ref}/${file}"
}

# version holds a tag optionally suffixed with a digest, e.g. "1.0.0-rc.3@sha256:a802...". The upstream Git tag
# that ships the matching manifests is the tag part prefixed with "v".
version=$( get-config ".thirdParty.localCSIDriver.version" )
tag="${version%%@*}"
ref="v${tag}"

tmp_dir=$( mktemp -d )
trap 'rm -rf "${tmp_dir}"' EXIT

for file in '00_namespace.yaml' '10_csidriver.yaml' '10_driver_serviceaccount.yaml' '10_provisioner_clusterrole.yaml' '20_provisioner_clusterrolebinding.yaml' '50_daemonset.yaml'; do
    curl --fail --retry 5 --retry-all-errors -L "$( get-url "${ref}" "deploy/kubernetes/local-csi-driver/${file}" )" -o "${tmp_dir}/${file}"
done

curl --fail --retry 5 --retry-all-errors -L "$( get-url "${ref}" 'example/storageclass_xfs.yaml' )" -o "${tmp_dir}/storageclass_xfs.yaml"

mkdir -p "${target_dir}"

# Taken over verbatim.
yq '.' "${tmp_dir}/00_namespace.yaml" > "${target_dir}/00_namespace.yaml"
yq '.' "${tmp_dir}/10_csidriver.yaml" > "${target_dir}/10_csidriver.yaml"
yq '.' "${tmp_dir}/10_driver_serviceaccount.yaml" > "${target_dir}/10_serviceaccount.yaml"
yq '.' "${tmp_dir}/20_provisioner_clusterrolebinding.yaml" > "${target_dir}/20_clusterrolebinding.yaml"
yq '.' "${tmp_dir}/storageclass_xfs.yaml" > "${target_dir}/00_scylladb-local-xfs.storageclass.yaml"

# The upstream provisioner ClusterRole becomes the aggregated definition holding the platform independent rules.
yq " \
    .metadata.name = \"scylladb:aggregate-to-csi-external-provisioner\" | \
    .metadata.labels.\"${aggregate_label}\" = \"true\" \
    " "${tmp_dir}/10_provisioner_clusterrole.yaml" > "${target_dir}/00_clusterrole_def.yaml"

# The aggregating ClusterRole the ServiceAccount is actually bound to. It has no upstream counterpart.
cat > "${target_dir}/00_clusterrole.yaml" << EOF
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: ${aggregated_clusterrole_name}
aggregationRule:
  clusterRoleSelectors:
  - matchLabels:
      ${aggregate_label}: "true"
EOF

# Permissions required on OpenShift only. They have no upstream counterpart.
cat > "${target_dir}/00_clusterrole_def_openshift.yaml" << EOF
kind: ClusterRole
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: scylladb:aggregate-to-csi-external-provisioner-openshift
  labels:
    ${aggregate_label}: "true"
rules:
- apiGroups:
  - security.openshift.io
  resourceNames:
  - privileged
  resources:
  - securitycontextconstraints
  verbs:
  - use
EOF

# The DaemonSet pins the driver image, is restricted to NodeConfig tuned nodes and uses this repository's
# volumes directory in the driver flag, the mount and the host path backing it.
yq " \
    ( .spec.template.spec.containers[] | select(.name == \"local-csi-driver\") | .image ) = \"docker.io/scylladb/local-csi-driver:${version}\" | \
    .spec.template.spec.nodeSelector.\"${node_type_label}\" = \"${node_type}\" | \
    ( .. | select(tag == \"!!str\") ) |= sub(\"${upstream_volumes_dir}\"; \"${volumes_dir}\") \
    " "${tmp_dir}/50_daemonset.yaml" |
    # Upstream annotates the sidecar images for its own Renovate. Here the only managed version is the one in
    # config.yaml and no manager reads this file, so keeping the annotations would claim an automation that
    # does not exist.
    grep -v -E '^[[:space:]]*# renovate:' > "${target_dir}/50_daemonset.yaml"

# Every transform above is a no-op if upstream renames what it matches on, which would silently ship
# upstream's defaults. Assert the results instead of trusting them.
function assert-equal() {
    local what="${1}"
    local expected="${2}"
    local actual="${3}"

    if [[ "${expected}" != "${actual}" ]]; then
        printf 'Manifest transform failed: %s is %q, expected %q. Did the upstream manifests change?\n' "${what}" "${actual}" "${expected}" >&2
        exit 1
    fi
}

assert-equal 'the driver image' \
    "docker.io/scylladb/local-csi-driver:${version}" \
    "$( yq '.spec.template.spec.containers[] | select(.name == "local-csi-driver") | .image' "${target_dir}/50_daemonset.yaml" )"

assert-equal 'the node type node selector' \
    "${node_type}" \
    "$( yq ".spec.template.spec.nodeSelector.\"${node_type_label}\"" "${target_dir}/50_daemonset.yaml" )"

assert-equal 'the number of remaining upstream volumes dir references' \
    '0' \
    "$( grep -c -F "${upstream_volumes_dir}" "${target_dir}/50_daemonset.yaml" || true )"

assert-equal 'the number of volumes dir references' \
    '3' \
    "$( grep -c -F "${volumes_dir}" "${target_dir}/50_daemonset.yaml" )"

assert-equal 'the number of remaining Renovate annotations' \
    '0' \
    "$( grep -c -E '^[[:space:]]*# renovate:' "${target_dir}/50_daemonset.yaml" || true )"

# The upstream ClusterRoleBinding is taken over verbatim, so the ClusterRole this repository aggregates
# into has to keep the name that binding refers to.
assert-equal 'the aggregated ClusterRole name the ClusterRoleBinding refers to' \
    "${aggregated_clusterrole_name}" \
    "$( yq '.roleRef.name' "${target_dir}/20_clusterrolebinding.yaml" )"
