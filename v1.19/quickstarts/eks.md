# Deploying ScyllaDB on EKS

This is a quickstart guide to help you set up a basic EKS cluster quickly with local NVMes and solid performance.

This is by no means a complete guide, and you should always consult your provider’s documentation.

## Prerequisites

In this guide we’ll be using `eksctl` to set up the cluster, and you’ll need `kubectl` to talk to it.

If you don’t have those already, or are not available through your package manager, you can try these links to learn more about installing them:

- [eksctl](https://docs.aws.amazon.com/eks/latest/userguide/getting-started-eksctl.html)
- [kubectl](https://kubernetes.io/docs/tasks/tools/#kubectl)

## Creating an EKS cluster

First, let’s create a declarative config to used with eksctl

```shell
curl --fail --retry 5 --retry-all-errors -o 'clusterconfig.eksctl.yaml' -L https://raw.githubusercontent.com/scylladb/scylla-operator/v1.19/examples/eks/clusterconfig.eksctl.yaml
```

With the config ready, we can easily create an EKS cluster by running

```bash
eksctl create cluster -f=clusterconfig.eksctl.yaml
```

## Deploying Scylla Operator

To deploy Scylla Operator follow the [installation guide](https://operator.docs.scylladb.com/v1.19/installation/overview.md).

## Creating ScyllaDB

To deploy a ScyllaDB cluster please head to [our dedicated section on the topic](https://operator.docs.scylladb.com/v1.19/resources/scyllaclusters/basics.md).

## Accessing ScyllaDB

We also hve a whole section dedicated to [how you can access the ScyllaDB cluster you’ve just created](https://operator.docs.scylladb.com/v1.19/resources/scyllaclusters/clients/index.md).

## Deleting the EKS cluster

Once you are done, you can delete the EKS cluster using the following command:

```default
eksctl delete cluster -f=clusterconfig.eksctl.yaml
```
