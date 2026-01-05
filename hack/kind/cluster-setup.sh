#!/bin/bash

set -euEo pipefail
shopt -s inherit_errexit

# This script makes sure that a KinD cluster is set up. If a cluster already exists, it will be reused unless the
# RECREATE environment variable is set to "true".
# Additionally, it sets up a local container registry and connects it to the KinD cluster so that images can be pushed to
# it efficiently.

readonly parent_dir="$( dirname "${BASH_SOURCE[0]}" )"

if [ -z "${CLUSTER_NAME}" ]; then
  echo "CLUSTER_NAME must be set" > /dev/stderr
  exit 1
fi

# Ensure KinD cluster exists.
if [ "${RECREATE:-false}" == "true" ]; then
    kind delete cluster --name="${CLUSTER_NAME}" || true
fi

if ! kind get clusters | grep -q "^${CLUSTER_NAME}$"; then
    kind create cluster --name="${CLUSTER_NAME}" --config="${parent_dir}/cluster-config.yaml"
else
    echo "Reusing existing KinD cluster: ${CLUSTER_NAME}"
fi

# Set up a local registry for the KinD cluster following https://kind.sigs.k8s.io/docs/user/local-registry/.
reg_name='kind-registry'
reg_port='5001'
if [ "$(podman inspect -f '{{.State.Running}}' "${reg_name}" 2>/dev/null || true)" != 'true' ]; then
  podman run \
    -d --restart=always -p "127.0.0.1:${reg_port}:5000" --network bridge --name "${reg_name}" \
    registry:2
fi

REGISTRY_DIR="/etc/containerd/certs.d/localhost:${reg_port}"
for node in $(kind get nodes --name="${CLUSTER_NAME}"); do
  echo "Configuring node ${node} to use local registry at localhost:${reg_port}"
  podman exec "${node}" mkdir -p "${REGISTRY_DIR}"
  cat <<EOF | podman exec -i "${node}" cp /dev/stdin "${REGISTRY_DIR}/hosts.toml"
[host."http://${reg_name}:5000"]
EOF
done

if [ "$(podman inspect -f='{{json .NetworkSettings.Networks.kind}}' "${reg_name}")" = 'null' ]; then
  podman network connect "kind" "${reg_name}"
fi

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
