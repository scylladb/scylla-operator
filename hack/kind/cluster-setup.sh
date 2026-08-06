#!/bin/bash

set -euEo pipefail
shopt -s inherit_errexit

# This script makes sure that a KinD cluster is set up. If a cluster already exists, it will be reused unless the
# RECREATE environment variable is set to "true".
# Additionally, it sets up a local container registry and connects it to the KinD cluster so that images can be pushed to
# it and be available inside the KinD cluster.

readonly repo_root="$( dirname "${BASH_SOURCE[0]}" )/../.."

source "${repo_root}/hack/kind/lib.sh"

# Ensure all kind calls use podman.
export KIND_EXPERIMENTAL_PROVIDER=podman

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

# Generate containerd registry configuration (mounted into KinD nodes via cluster-config.yaml).
internal_reg_port=5000
containerd_reg_dir="${repo_root}/hack/kind/containerd-registries/localhost:${KIND_REGISTRY_PORT}"
mkdir -p "${containerd_reg_dir}"
cat > "${containerd_reg_dir}/hosts.toml" <<EOF
[host."http://${KIND_REGISTRY_NAME}:${internal_reg_port}"]
EOF

# Ensure KinD cluster exists.
if ! kind get clusters | grep -q "^${CLUSTER_NAME}$"; then
    KIND_CREATE_CMD=(kind create cluster --name="${CLUSTER_NAME}" --config="${repo_root}/hack/kind/cluster-config.yaml" --retain)

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
if [ "$(podman inspect -f '{{.State.Running}}' "${KIND_REGISTRY_NAME}" 2>/dev/null || true)" != 'true' ]; then
  podman run \
    -d --restart=always -p "127.0.0.1:${KIND_REGISTRY_PORT}:${internal_reg_port}" --replace --network bridge --name "${KIND_REGISTRY_NAME}" \
    registry:2
fi

# Connect registry to KinD network.
if [ "$(podman inspect -f='{{json .NetworkSettings.Networks.kind}}' "${KIND_REGISTRY_NAME}")" = 'null' ]; then
  podman network connect "kind" "${KIND_REGISTRY_NAME}"
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
    host: "${KIND_REGISTRY_HOST}:${KIND_REGISTRY_PORT}"
    help: "https://kind.sigs.k8s.io/docs/user/local-registry/"
EOF
rm "${temp_kubeconfig}"
