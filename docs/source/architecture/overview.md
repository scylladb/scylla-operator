# Overview

## Foreword

{{productName}} is a set of controllers and API extensions that need to be installed in your cluster.
The Kubernetes API is extended using [CustomResourceDefinitions (CRDs)](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/) and [dynamic admission webhooks](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/) to provide new resources ([API reference](../api-reference/index.rst)).
These resources are reconciled by controllers embedded within the {{productName}} deployment.

ScyllaDB is a stateful application and {{productName}} requires you to have a storage provisioner installed in your cluster.
To achieve the best performance, we recommend using a storage provisioner based on local NVMEs.
You can learn more about different setups in [a dedicated storage section](./storage/overview.md).

## Components

{{productName}} deployment consists of several components that need to be installed / present in you Kubernetes cluster.
By design, some of the components need elevated permissions, but they are only accessible to the administrators.


### Cluster scoped
```{image} ./components-cluster_scoped.svg
:name: components-cluster-scoped
:align: center
:scale: 75%
```

### Namespaced
```{image} ./components-namespaced.svg
:name: components-namespaced
:align: center
:scale: 75%
```
