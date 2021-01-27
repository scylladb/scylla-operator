# Scylla Charts

Helm Charts allows for customization and deployments of Scylla, Scylla Manager
and Scylla Operator on Kubernetes environments.

## Prerequisites

- Kubernetes 1.16+
- Helm 3+

## Usage

[Helm](https://helm.sh) must be installed to use the charts.
Please refer to Helm's [documentation](https://helm.sh/docs/) to get started.

Once Helm is set up properly, add the repo as follows:

```console
helm repo add scylla-operator https://storage.googleapis.com/scylla-operator-charts
```

You can then run `helm search repo scylla-operator` to see the charts.

## Install Chart

```console
# Install Scylla Operator
$ helm install [NAME] scylla-operator/scylla-operator --create-namespace --namespace scylla-operator

# Install Scylla Manager
$ helm install [NAME] scylla-operator/scylla-manager --create-namespace --namespace scylla-manager

# Install Scylla 
$ helm install [NAME] scylla-operator/scylla --create-namespace --namespace scylla

```
