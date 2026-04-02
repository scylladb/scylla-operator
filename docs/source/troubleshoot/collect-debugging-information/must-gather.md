# Collect data with must-gather

`must-gather` is an embedded tool in ScyllaDB Operator that collects Kubernetes resources, pod logs, and ScyllaDB diagnostic data into an archive.
Attach this archive to support tickets to provide a complete snapshot of your cluster state.

By default, sensitive resources (`Secrets` and `bitnami.com.SealedSecrets`) are omitted.

## Prerequisites

Export the `KUBECONFIG` environment variable pointing to your kubeconfig file:

```bash
export KUBECONFIG=~/.kube/config
```

### External authentication plugins

If your kubeconfig uses an [external exec credential plugin](https://kubernetes.io/docs/reference/access-authn-authz/authentication/#client-go-credential-plugins), the plugin binary is not available inside the must-gather container.
Create a ServiceAccount token instead:

```bash
kubectl create namespace must-gather
kubectl -n must-gather create serviceaccount must-gather
kubectl create clusterrolebinding must-gather \
  --clusterrole=cluster-admin \
  --serviceaccount=must-gather:must-gather

MUST_GATHER_TOKEN=$(kubectl -n must-gather create token must-gather --duration=1h)

# Create a kubeconfig copy with the token
kubeconfig=$(mktemp)
kubectl config view --minify --raw -o yaml | \
  yq -e '.users[0].user = {"token": env(MUST_GATHER_TOKEN)}' > "${kubeconfig}"
KUBECONFIG="${kubeconfig}"
```

## Running must-gather

::::{tabs}
:::{group-tab} Podman
```bash
podman run -it --pull=always --rm \
  -v="${KUBECONFIG}:/kubeconfig:ro,Z" \
  -v="$(pwd):/workspace:Z" \
  --workdir=/workspace \
  docker.io/scylladb/scylla-operator:latest \
  must-gather --kubeconfig=/kubeconfig
```
:::
:::{group-tab} Docker
```bash
docker run -it --pull=always --rm \
  -v="${KUBECONFIG}:/kubeconfig:ro" \
  -v="$(pwd):/workspace" \
  --workdir=/workspace \
  docker.io/scylladb/scylla-operator:latest \
  must-gather --kubeconfig=/kubeconfig
```
:::
:::{group-tab} minikube
Minikube's kubeconfig points the API server at `127.0.0.1`, which is not reachable from inside a container.
Use `--network=host` so the container shares the host's network namespace:

```bash
docker run -it --pull=always --rm --network=host \
  -v="${KUBECONFIG}:/kubeconfig:ro" \
  -v="$(pwd):/workspace" \
  --workdir=/workspace \
  docker.io/scylladb/scylla-operator:latest \
  must-gather --kubeconfig=/kubeconfig
```
:::
:::{group-tab} crictl

On nodes where neither `docker` nor `podman` is available (common on production bare-metal nodes using containerd), use `crictl`:

```bash
crictl pull docker.io/scylladb/scylla-operator:latest
crictl run \
  --pull=always \
  --rm \
  -v "${HOME}/.kube/config:/root/.kube/config:ro" \
  -v "$(pwd)/must-gather.tar.gz:/must-gather.tar.gz" \
  docker.io/scylladb/scylla-operator:latest \
  must-gather \
  --all-resources \
  --dest=/must-gather.tar.gz
```

`crictl` communicates directly with the container runtime socket. Ensure the socket path is reachable (typically `/run/containerd/containerd.sock` or `/run/crio/crio.sock`) and that you have permission to access it.
:::
:::{group-tab} kubectl (no container runtime)

If no container runtime is available locally, run must-gather as a Kubernetes Job inside the cluster.
This requires RBAC permissions to create Jobs and access cluster resources.

First, apply the necessary ClusterRole:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: scylla-must-gather
rules:
- apiGroups: ["*"]
  resources: ["*"]
  verbs: ["get", "list", "watch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: scylla-must-gather
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: scylla-must-gather
subjects:
- kind: ServiceAccount
  name: scylla-must-gather
  namespace: default
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: scylla-must-gather
  namespace: default
```

Then run the must-gather pod:

```bash
kubectl run must-gather \
  --image=docker.io/scylladb/scylla-operator:latest \
  --restart=Never \
  --serviceaccount=scylla-must-gather \
  -- must-gather --all-resources --dest=/tmp/must-gather.tar.gz
```

After the pod completes, copy the archive:

```bash
kubectl cp default/must-gather:/tmp/must-gather.tar.gz ./must-gather.tar.gz
kubectl delete pod must-gather
kubectl delete clusterrolebinding scylla-must-gather
kubectl delete clusterrole scylla-must-gather
kubectl delete serviceaccount -n default scylla-must-gather
```
:::
::::

:::{tip}
If you get `Error: lsetxattr <path>: operation not supported` with Podman, remove the `Z` option from the volume mounts.
This depends on your SELinux configuration.
:::

## Options

### Limit to a namespace

Limit ScyllaCluster collection to a specific namespace:

```bash
scylla-operator must-gather --namespace="<namespace>"
```

:::{note}
The `--namespace` flag only affects ScyllaCluster resources.
Operator installation resources and cluster-wide resources are still collected from other namespaces.
:::

### Collect all resources

Collect every resource in the Kubernetes API (not just the default subset):

```bash
scylla-operator must-gather --all-resources
```

### Include sensitive resources

Include `Secrets` and `SealedSecrets` (not recommended unless needed for investigation):

```bash
scylla-operator must-gather --all-resources --include-sensitive-resources
```

### Exclude specific resources

Exclude resources you do not want in the archive:

```bash
scylla-operator must-gather --all-resources \
  --exclude-resource="LimitRange" \
  --exclude-resource="DeviceClass.resource.k8s.io"
```

The format is `Kind` for core resources or `Kind.group` for others.

## Output

The tool creates a timestamped directory in the current working directory containing:
- Kubernetes resource YAMLs organized by namespace.
- Pod logs (current, previous, and terminated containers).
- Per-node diagnostic files for ScyllaDB pods.

See [must-gather contents](must-gather-contents.md) for a detailed description of the output structure.

## Related pages

- [must-gather contents](must-gather-contents.md)
- [System tables](system-tables.md)
- [Diagnostic flowchart](../diagnostic-flowchart.md)
