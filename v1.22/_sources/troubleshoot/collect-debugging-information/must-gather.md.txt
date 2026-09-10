# Collect data with must-gather

`must-gather` is a tool embedded in ScyllaDB Operator that collects diagnostic information equivalent to a debugging session, including:

- A snapshot of ScyllaDB configuration.
- A snapshot of Operator runtime state (status conditions, node service labels).
- A snapshot of Operator and ScyllaDB logs.
- A snapshot of the sequence of events (Kubernetes Events, object creation/deletion information).

## Prerequisites

All examples assume you have exported the `KUBECONFIG` environment variable pointing to a kubeconfig file on your machine.
If not, export the common default location:

```bash
export KUBECONFIG=~/.kube/config
ls -l "${KUBECONFIG}"
```

:::{note}
There can be slight deviations in the arguments for your container tool, depending on the container runtime, whether you use SELinux, or similar factors.

As an example, the need for the `Z` option on volume mounts depends on whether you use SELinux and what context is applied on your file or directory.
If you get an error mentioning `Error: lsetxattr <path>: operation not supported`, try it without the `Z` option.
:::

### Use an external authentication plugin

Check whether your kubeconfig uses an [external authentication plugin](https://kubernetes.io/docs/reference/access-authn-authz/authentication/#client-go-credential-plugins) by running:

```bash
kubectl config view --minify
```

Look for this pattern (containing the `exec` key):

```yaml
users:
- name: <user_name>
  user:
    exec:
```

If your kubeconfig does not use an external exec plugin, skip the rest of this section.

If your kubeconfig depends on external binaries, the external binary will not be available within the container to authenticate requests.
Create a dedicated ServiceAccount for must-gather and use it to run the tool:

```bash
kubectl create namespace must-gather
kubectl -n must-gather create serviceaccount must-gather
kubectl create clusterrolebinding must-gather --clusterrole=cluster-admin --serviceaccount=must-gather:must-gather
export MUST_GATHER_TOKEN
MUST_GATHER_TOKEN=$( kubectl -n must-gather create token must-gather --duration=1h )
kubeconfig=$( mktemp )
# Create a copy of the existing kubeconfig and
# replace user authentication using yq, or by adjusting the fields manually.
kubectl config view --minify --raw -o yaml | yq -e '.users[0].user = {"token": env(MUST_GATHER_TOKEN)}' > "${kubeconfig}"
KUBECONFIG="${kubeconfig}"
```

:::{note}
If you do not have `yq` installed, you can get it at https://github.com/mikefarah/yq/#install or replace the user authentication settings manually.
:::

When you are done using must-gather, remove the Kubernetes resources created for that purpose.

## Run must-gather

Run the must-gather container image, mounting your kubeconfig and a local directory for the output:

::::::{tabs}

:::::{group-tab} Docker
```bash
docker run -it --pull=always --rm \
    -v="${KUBECONFIG}:/kubeconfig:ro" \
    -v="$( pwd ):/workspace" \
    --workdir=/workspace \
    docker.io/scylladb/scylla-operator:latest must-gather \
    --kubeconfig=/kubeconfig
```
:::::

:::::{group-tab} Podman
```bash
podman run -it --pull=always --rm \
    -v="${KUBECONFIG}:/kubeconfig:ro,Z" \
    -v="$( pwd ):/workspace:Z" \
    --workdir=/workspace \
    docker.io/scylladb/scylla-operator:latest must-gather \
    --kubeconfig=/kubeconfig
```
:::::

::::::

Expected output (similar to):

```
I1029 11:40:59.708164       1 operator/gatherbase.go:119] "Created destination directory" Path="scylla-operator-must-gather-kns4cpxtnwmg"
I1029 11:40:59.708640       1 cmdutil/helpers.go:106] "Starting must-gather" GitCommit="\"2621bad4d\""
I1029 11:40:59.709003       1 operator/mustgather.go:253] "Gathering artifacts" DestDir="scylla-operator-must-gather-kns4cpxtnwmg"
I1029 11:41:06.549330       1 operator/mustgather.go:255] "Finished gathering artifacts" Duration="6.840317591s"
```

The output directory (here `scylla-operator-must-gather-kns4cpxtnwmg`) contains the collected archive.
See [must-gather contents](must-gather-contents.md) for details on navigating the archive.

## Inspect the archive for sensitive information

By default, sensitive resources (`Secrets` and `bitnami.com.SealedSecrets`) are omitted from the collection.
However, before sharing the archive, inspect it on your own to ensure nothing unexpected is included:

```bash
grep -r -l '^kind: Secret' <must-gather-directory>
```

Redact or delete sensitive information if necessary.

## Limit collection to a particular namespace

If you are running a large Kubernetes cluster with many ScyllaClusters, limit the collection to a particular namespace:

```bash
scylla-operator must-gather --namespace="<namespace>"
```

:::{note}
The `--namespace` flag affects only ScyllaClusters.
Other resources related to the Operator installation or cluster state are still collected from other namespaces.
:::

## Collect every resource in the cluster

By default, must-gather collects only a predefined subset of resources.
Request collecting every resource in the Kubernetes API if the default set is not sufficient:

```bash
scylla-operator must-gather --all-resources
```

## Include sensitive resources

Override the default behavior of omitting sensitive resources by passing the `--include-sensitive-resources` flag:

```bash
scylla-operator must-gather --all-resources --include-sensitive-resources
```

## Exclude resources

Exclude specific resources from the collection by passing the `--exclude-resource` flag:

```bash
scylla-operator must-gather --all-resources \
    --exclude-resource="LimitRange" \
    --exclude-resource="DeviceClass.resource.k8s.io"
```

:::{note}
The format for the resource is `kind` (for core resources) or `kind.group`. Examples: `LimitRange` or `DeviceClass.resource.k8s.io`.
:::

## Related pages

- [must-gather contents](must-gather-contents.md)
- [Query system tables](system-tables.md)
