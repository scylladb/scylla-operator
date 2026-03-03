# RemoteKubernetesCluster

## Introduction

The cluster-scoped `RemoteKubernetesCluster` resource provides an abstraction layer for managing resources in a remote Kubernetes cluster.

It allows users to define and interact with a separate Kubernetes environment, distinct from the cluster where the resource is created.  
This is particularly useful in scenarios that require centralized management of ScyllaDB clusters distributed across multiple geographic regions.

## Example

```{literalinclude} ../../../examples/multi-dc/cluster-wide-resources/00_dev-us-east-1.remotekubernetescluster.yaml
:language: yaml
:linenos:
```

## Remote authorization methods

### Secret

This authorization method relies on a Secret containing credentials to a remote Kubernetes cluster in the _kubeconfig_ format.  
The user associated with these credentials must have the `scylladb:controller:operator-remote` ClusterRole bound.
The name and namespace of this Secret are referenced in the spec of the corresponding RemoteKubernetesCluster resource.

Example:

::::{caution}
While token-based authentication is the easiest to configure, the use of long-lived tokens is discouraged due to security risks.  
For improved security, consider using alternative authentication methods outlined in the Kubernetes documentation:  
[https://kubernetes.io/docs/reference/access-authn-authz/authentication/](https://kubernetes.io/docs/reference/access-authn-authz/authentication/)

Currently, `RemoteKubernetesCluster` requires unconditional access to all `Secrets` and `ConfigMaps` across all namespaces in the remote cluster.
Please note the [security implications](https://github.com/scylladb/scylla-operator/issues/2577) of this limitation.
::::

```{literalinclude} ../../../examples/multi-dc/namespaces/remotekubernetescluster-credentials/10_dev-us-east-1.secret.yaml
:language: yaml
:linenos:
```

```{image} remotekubernetesclusters.svg
``` 


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

