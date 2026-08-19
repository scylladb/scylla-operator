# Gathering data with must-gather

`must-gather` is an embedded tool in Scylla Operator that helps collecting all the necessary info when something goes wrong.

The tool talks to the Kubernetes API, retrieves a predefined set of resources and saves them into a folder in your current directory.
By default, all collected Secrets are censored to avoid sending sensitive data.
That said, you can always review the archive before you attach it to an issue or your support request.

Given it needs to talk to the Kubernetes API, at the very least, you need to supply the `--kubeconfig` flag with a path to the kubeconfig file for your Kubernetes cluster, or set the `KUBECONFIG` environment variable.

## Running must-gather

There is more than one way to run `must-gather`.
Here are some examples of how you can run the tool.

### Prerequisites

All examples assume you have exported `KUBECONFIG` environment variable that points to a kubeconfig file on your machine.
If not, you can run this command to export the common default location.
Please make sure such a file exists.

```bash
export KUBECONFIG=~/.kube/config
ls -l "${KUBECONFIG}"
```

:::{note}
   There can be slight deviations in the arguments for your container tool, depending on the container runtime, whether you use SELinux or similar factors.

   As an example, the need for the `Z` option on volume mounts depends on whether you use SELinux and what context is applied on your file or directory.
   If you get an error mentioning `Error: lsetxattr <path>: operation not supported`, try it without the `Z` option.
:::

Let's also check whether your kubeconfig uses [external authentication plugin](https://kubernetes.io/docs/reference/access-authn-authz/authentication/#client-go-credential-plugins).
You can determine that by running
```bash
kubectl config view --minify
```
and checking whether it uses an external exec plugin by looking for this pattern (containing the `exec` key)
```yaml
users:
- name: <user_name>
  user:
    exec:
```
If not, you can skip the rest of this section.

In case your kubeconfig depends on external binaries, you have to take a few extra steps because the external binary won't be available within our container to authenticate the requests.

Similarly to how Pods are run within Kubernetes, we'll create a dedicated ServiceAccount for must-gather and use it to run the tool.
(When you are done using it, feel free to remove the Kubernetes resources created for that purpose.)

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
   If you don't have `yq` installed, you can get it at https://github.com/mikefarah/yq/#install or you can replace the user authentication settings manually.
:::

### Podman
```bash
podman run -it --pull=always --rm -v="${KUBECONFIG}:/kubeconfig:ro,Z" -v="$( pwd ):/workspace:Z" --workdir=/workspace docker.io/scylladb/scylla-operator:latest must-gather --kubeconfig=/kubeconfig
```

### Docker
```bash
docker run -it --pull=always --rm -v="${KUBECONFIG}:/kubeconfig:ro" -v="$( pwd ):/workspace" --workdir=/workspace docker.io/scylladb/scylla-operator:latest must-gather --kubeconfig=/kubeconfig
```

## Limiting must-gather to a particular namespace

If you are running a large Kubernetes cluster with many ScyllaClusters, it may be useful to limit the collection of ScyllaClusters to a particular namespace.
Unless you hit scale issues, we advise not to use this mode, as sometimes the ScyllaClusters affect other collected resources, like the manager or they form a multi-datacenter.

```bash
scylla-operator must-gather --namespace="<namespace_with_broken_scyllacluster>"
```

:::{note}
   The `--namespace` flag affects only `ScyllaClusters`.
   Other resources related to the operator installation or cluster state will still be collected from other namespaces.
:::

### Collecting every resource in the cluster

By default, `must-gather` collects only a predefined subset of resources.
You can also request collecting every resource in the Kubernetes API, if the default set wouldn't be enough to debug an issue.

```bash
scylla-operator must-gather --all-resources
```
