# Upgrading {{productName}}

{{productName}} supports N+1 upgrades only.
That means to you can only update by 1 minor version at the time and wait for it to successfully roll out and then update
all ScyllaClusters that also run using the image that's being updated. ({{productName}} injects it as a sidecar to help run and manage ScyllaDB.)

We value the stability of our APIs and all API changes are backwards compatible.

:::{caution}
If any additional steps are required for a specific version upgrade, they will be documented in the [Upgrade steps for specific versions](#upgrade-steps-for-specific-versions) section below.
:::

## Upgrade via GitOps (kubectl)

A typical upgrade flow using GitOps (kubectl) requires re-applying the manifests using ones from the release you want to upgrade to.
Note that {{productName}}'s dependencies also need to be updated to the versions compatible with the target {{productName}} version.

Please refer to the [GitOps installation instructions](./../../installation/gitops.md) for details.

## Upgrade via Helm

### Prerequisites
- Update {{productName}} dependencies to the versions compatible with the target {{productName}} version.
  Refer to the [Helm installation instructions](./../../installation/helm.md) for details.

- Make sure Helm chart repository is up-to-date:
  ```
  helm repo add scylla https://scylla-operator-charts.storage.googleapis.com/stable
  helm repo update
  ```

### Upgrade ScyllaDB Manager

Replace `<release_name>` with the name of your Helm release for ScyllaDB Manager and replace `<version>` with the version number you want to install:

```
helm upgrade --version <version> <release_name> scylla/scylla-manager
```

### Upgrade {{productName}} 

Replace `<release_name>` with the name of your Helm release for {{productName}} and replace `<version>` with the version number you want to install:

1. Update CRD resources. We recommend using `--server-side` flag for `kubectl apply`, if your version supports it.
   ```
   tmpdir=$( mktemp -d ) \
     && helm pull scylla-operator/scylla-operator --version <version> --untar --untardir "${tmpdir}" \
     && find "${tmpdir}"/scylla-operator/crds/ -name '*.yaml' -printf '-f=%p ' \
     | xargs kubectl apply
   ```
2. Update {{productName}}.
   ```
   helm upgrade --version <version> <release_name> scylla/scylla-operator
   ```

## Upgrade steps for specific versions

### 1.17 to 1.18

Upgrading from v1.17.x requires extra actions due to the removal of the standalone ScyllaDB Manager controller.
The controller becomes an integral part of {{productName}}. As a result, the standalone ScyllaDB Manager controller deployment and its related resources
need to be removed before upgrading {{productName}}.

{#steps-1-17-to-1-18-upgrade-via-gitops}
#### Upgrade via GitOps (kubectl)

```
kubectl delete -n scylla-manager \
    clusterrole/scylladb:controller:aggregate-to-manager-controller \
    clusterrole/scylladb:controller:manager-controller \
    poddisruptionbudgets.policy/scylla-manager-controller \
    serviceaccounts/scylla-manager-controller \
    clusterrolebindings.rbac.authorization.k8s.io/scylladb:controller:manager-controller \
    deployments.apps/scylla-manager-controller
```

{#steps-1-17-to-1-18-upgrade-via-helm}
#### Upgrade via Helm

ScyllaDB Manager Helm installation has to be upgraded before upgrading {{productName}} Helm installation, following
the standard [Helm upgrade procedure](#upgrade-via-helm). This will ensure that the ScyllaDB Manager Controller
is removed before upgrading the {{productName}}.
