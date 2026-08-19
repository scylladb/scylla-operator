# Upgrade

Scylla Operator supports N+1 upgrades only.
That means to you can only update by 1 minor version at the time and wait for it to successfully roll out and then update
all ScyllaClusters that also run using the image that’s being updated. (Scylla Operator injects it as a sidecar to help run and manage ScyllaDB.)

We value the stability of our APIs and all API changes are backwards compatible.

## Upgrade via GitOps (kubectl)

A typical upgrade flow using GitOps (kubectl) requires re-applying the manifests using ones from the release you want to upgrade to.
Please refer to the [GitOps installation instructions](https://operator.docs.scylladb.com/v1.18/installation/gitops.md) for details.

## Upgrade via Helm

### Pre-requisites

- Make sure Helm chart repository is up-to-date:
  ```default
  helm repo add scylla https://scylla-operator-charts.storage.googleapis.com/stable
  helm repo update
  ```

### Upgrade ScyllaDB Manager

Replace `<release_name>` with the name of your Helm release for ScyllaDB Manager and replace `<version>` with the version number you want to install:

```default
helm upgrade --version <version> <release_name> scylla/scylla-manager
```

### Upgrade Scylla Operator

Replace `<release_name>` with the name of your Helm release for Scylla Operator and replace `<version>` with the version number you want to install:

1. Update CRD resources. We recommend using `--server-side` flag for `kubectl apply`, if your version supports it.
   ```default
   tmpdir=$( mktemp -d ) \
     && helm pull scylla-operator/scylla-operator --version <version> --untar --untardir "${tmpdir}" \
     && find "${tmpdir}"/scylla-operator/crds/ -name '*.yaml' -printf '-f=%p ' \
     | xargs kubectl apply
   ```
2. Update Scylla Operator.
   ```default
   helm upgrade --version <version> <release_name> scylla/scylla-operator
   ```

## Upgrade steps for specific versions

### 1.17 to 1.18

Upgrading from v1.17.x requires extra actions due to the removal of the standalone ScyllaDB Manager controller.
The controller becomes an integral part of Scylla Operator. As a result, the standalone ScyllaDB Manager controller deployment and its related resources
need to be removed before upgrading Scylla Operator.

#### Upgrade via GitOps (kubectl)

```default
kubectl delete -n scylla-manager \
    clusterrole/scylladb:controller:aggregate-to-manager-controller \
    clusterrole/scylladb:controller:manager-controller \
    poddisruptionbudgets.policy/scylla-manager-controller \
    serviceaccounts/scylla-manager-controller \
    clusterrolebindings.rbac.authorization.k8s.io/scylladb:controller:manager-controller \
    deployments.apps/scylla-manager-controller
```

#### Upgrade via Helm

ScyllaDB Manager Helm installation has to be upgraded before upgrading Scylla Operator Helm installation, following
the standard [Helm upgrade procedure](). This will ensure that the ScyllaDB Manager Controller
is removed before upgrading the Scylla Operator.
