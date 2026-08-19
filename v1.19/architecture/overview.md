# Overview

## Foreword

Scylla Operator is a set of controllers and API extensions that need to be installed in your cluster.
The Kubernetes API is extended using [CustomResourceDefinitions (CRDs)](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/) and [dynamic admission webhooks](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/) to provide new resources ([API reference](https://operator.docs.scylladb.com/v1.19/reference/api/index.md)).
These resources are reconciled by controllers embedded within the Scylla Operator deployment.

ScyllaDB is a stateful application and Scylla Operator requires you to have a storage provisioner installed in your cluster.
To achieve the best performance, we recommend using a storage provisioner based on local NVMEs.
You can learn more about different setups in [a dedicated storage section](https://operator.docs.scylladb.com/v1.19/architecture/storage/overview.md).

## Components

Scylla Operator deployment consists of several components that need to be installed / present in you Kubernetes cluster.
By design, some of the components need elevated permissions, but they are only accessible to the administrators.

### Cluster scoped

![image](architecture/components-cluster_scoped.svg)

### Namespaced

![image](architecture/components-namespaced.svg)
