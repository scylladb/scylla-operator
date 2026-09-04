# RemoteKubernetesCluster

## Introduction

The cluster-scoped `RemoteKubernetesCluster` resource provides an abstraction layer for managing resources in a remote Kubernetes cluster.

It allows users to define and interact with a separate Kubernetes environment, distinct from the cluster where the resource is created.<br />
\\\\
This is particularly useful in scenarios that require centralized management of ScyllaDB clusters distributed across multiple geographic regions.

## Example

```yaml
apiVersion: scylla.scylladb.com/v1alpha1
kind: RemoteKubernetesCluster
metadata:
    name: dev-us-east-1
spec:
  kubeconfigSecretRef:
    name: dev-us-east-1
    namespace: remotekubernetescluster-credentials
```

## Remote authorization methods

### Secret

This authorization method relies on a Secret containing credentials to a remote Kubernetes cluster in the *kubeconfig* format.<br />
\\\\
The user associated with these credentials must have the `scylladb:controller:operator-remote` ClusterRole bound.
The name and namespace of this Secret are referenced in the spec of the corresponding RemoteKubernetesCluster resource.

Example:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: dev-us-east-1
  namespace: remotekubernetescluster-credentials
type: Opaque
stringData:
  kubeconfig: |
    apiVersion: v1
    kind: Config
    clusters:
      - cluster:
          certificate-authority-data: <kube-apiserver-ca-bundle>
          server: <kube-apiserver-address>
        name: dev-us-east-1
    contexts:
      - context:
          cluster: dev-us-east-1
          user: dev-us-east-1
        name: dev-us-east-1
    current-context: dev-us-east-1
    users:
      - name: dev-us-east-1
        user:
          token: <token-having-remote-operator-cluster-role>


```

![image](resources/remotekubernetesclusters.svg)

## Status

Since the RemoteKubernetesCluster specification must authenticate with a remote Kubernetes cluster, it is important to carefully verify that the configured credentials are valid.
RemoteKubernetesCluster resources include standard aggregated conditions, which provide an easy way to confirm whether the configuration and connection were successful:

```console
$ kubectl wait --for='condition=Progressing=False' remotekubernetesclusters.scylla.scylladb.com/example
remotekubernetesclusters.scylla.scylladb.com/example condition met

$ kubectl wait --for='condition=Degraded=False' remotekubernetesclusters.scylla.scylladb.com/example
remotekubernetesclusters.scylla.scylladb.com/example condition met

$ kubectl wait --for='condition=Available=True' remotekubernetesclusters.scylla.scylladb.com/example
remotekubernetesclusters.scylla.scylladb.com/example condition met
```
