#!/usr/bin/env bash

set -euEo pipefail
shopt -s inherit_errexit

# This script makes sure that a KinD cluster is set up. If a cluster already exists, it will be reused unless the
# RECREATE environment variable is set to "true".
# Additionally, it sets up a local container registry and connects it to the KinD cluster so that images can be pushed to
# it and be available inside the KinD cluster.

readonly parent_dir="$( dirname "${BASH_SOURCE[0]}" )"

if [ -z "${CLUSTER_NAME}" ]; then
  echo "CLUSTER_NAME must be set" > /dev/stderr
  exit 1
fi

# If RECREATE is set to "true", delete any existing KinD cluster and Podman network.
if [ "${RECREATE:-false}" == "true" ]; then
    kind delete cluster --name="${CLUSTER_NAME}" || true
    podman network rm -f kind || true
fi

# Ensure there's a `kind` IPv4 network.
if ! podman network inspect kind >/dev/null 2>&1; then
  echo "Creating kind IPv4-only network..."
  podman network create kind
fi

# Ensure KinD cluster exists.
if ! kind get clusters | grep -q "^${CLUSTER_NAME}$"; then
    KIND_CREATE_CMD=(kind create cluster --name="${CLUSTER_NAME}" --config="${parent_dir}/cluster-config.yaml" --retain)

    # Ensure kind uses podman.
    export KIND_EXPERIMENTAL_PROVIDER=podman

    # As we rely on rootless Podman, we need to delegate cgroup management to the user systemd instance (this is implicitly
    # done on systems with systemd >= 252, but needs to be explicit on older systems).
    # See https://kind.sigs.k8s.io/docs/user/rootless/ for more details.

    # Check if systemd is the init system.
    if [ -d /run/systemd/system ] && command -v systemd-run >/dev/null 2>&1; then
        echo "Systemd detected. Delegating cgroups via systemd-run."
        systemd-run --scope --user -p "Delegate=yes" "${KIND_CREATE_CMD[@]}"
    else
        # Most likely in a container with no systemd (e.g., in CI).
        echo "No systemd detected. Running kind directly."
        "${KIND_CREATE_CMD[@]}"
    fi
else
    echo "Reusing existing KinD cluster: ${CLUSTER_NAME}"
fi

# Set up a local registry for the KinD cluster following https://kind.sigs.k8s.io/docs/user/local-registry/.
reg_name='kind-registry'
reg_port='5001'
if [ "$(podman inspect -f '{{.State.Running}}' "${reg_name}" 2>/dev/null || true)" != 'true' ]; then
  podman run \
    -d --restart=always -p "127.0.0.1:${reg_port}:5000" --replace --network bridge --name "${reg_name}" \
    registry:2
fi

# Connect registry to KinD network.
if [ "$(podman inspect -f='{{json .NetworkSettings.Networks.kind}}' "${reg_name}")" = 'null' ]; then
  podman network connect "kind" "${reg_name}"
fi

# Inform KinD cluster about the local registry.
temp_kubeconfig="$(mktemp)"
kind get kubeconfig --name="${CLUSTER_NAME}" > "${temp_kubeconfig}"
KUBECONFIG="${temp_kubeconfig}" cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: ConfigMap
metadata:
  name: local-registry-hosting
  namespace: kube-public
data:
  localRegistryHosting.v1: |
    host: "localhost:${reg_port}"
    help: "https://kind.sigs.k8s.io/docs/user/local-registry/"
EOF
rm "${temp_kubeconfig}"
