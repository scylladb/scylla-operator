# Upgrading ScyllaDB Operator

ScyllaDB Operator supports N+1 upgrades only.
That means to you can only update by 1 minor version at the time and wait for it to successfully roll out and then update
all ScyllaClusters that also run using the image that's being updated. (ScyllaDB Operator injects it as a sidecar to help run and manage ScyllaDB.)

We value the stability of our APIs and all API changes are backwards compatible.

:::{caution}
If any additional steps are required for a specific version upgrade, they will be documented in the [Upgrade steps for specific versions](#upgrade-steps-for-specific-versions) section below.
:::

## Upgrade via GitOps (kubectl)

A typical upgrade flow using GitOps (kubectl) requires re-applying the manifests using ones from the release you want to upgrade to.
Note that ScyllaDB Operator's dependencies also need to be updated to the versions compatible with the target ScyllaDB Operator version.

Please refer to the [GitOps installation instructions](./../../installation/gitops.md) for details.

## Upgrade via Helm

### Prerequisites
- Update ScyllaDB Operator dependencies to the versions compatible with the target ScyllaDB Operator version.
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

### Upgrade ScyllaDB Operator 

Replace `<release_name>` with the name of your Helm release for ScyllaDB Operator and replace `<version>` with the version number you want to install:

1. Update CRD resources. We recommend using `--server-side` flag for `kubectl apply`, if your version supports it.
   ```
   tmpdir=$( mktemp -d ) \
     && helm pull scylla-operator/scylla-operator --version <version> --untar --untardir "${tmpdir}" \
     && find "${tmpdir}"/scylla-operator/crds/ -name '*.yaml' -printf '-f=%p ' \
     | xargs kubectl apply
   ```
2. Update ScyllaDB Operator.
   ```
   helm upgrade --version <version> <release_name> scylla/scylla-operator
   ```

## Upgrade steps for specific versions

:::{caution}
The below instructions are **supplementary** to the standard upgrade procedure and don't fully replace it.
Make sure to familiarize yourself with both the standard upgrade procedure and the additional steps for the specific version you are upgrading to, if applicable, and follow them accordingly.
:::

### 1.20 to 1.21

#### Ensure ScyllaCluster repair and backup task names are RFC 1123 compliant

Before upgrading, ensure that all `ScyllaCluster` repair (`.spec.repairs[].name`) and backup (`.spec.backups[].name`) task names conform to [RFC 1123 subdomain](https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#dns-subdomain-names) requirements:
- contain no more than 253 characters,
- contain only lowercase alphanumeric characters, '-' or '.',
- start with an alphanumeric character,
- end with an alphanumeric character.

You can run the following snippet to check for such `ScyllaClusters`:
```bash
output=$(kubectl get scyllaclusters --all-namespaces -o json | jq -r '
  def is_rfc1123_subdomain_invalid:
    (length <= 253) and test("^[a-z0-9]([a-z0-9.-]{0,251}[a-z0-9])?$") | not;
  .items[] |
  {
    namespace: .metadata.namespace,
    name: .metadata.name,
    invalid_repairs: [(.spec.repairs // [] | .[].name | select(is_rfc1123_subdomain_invalid))],
    invalid_backups: [(.spec.backups // [] | .[].name | select(is_rfc1123_subdomain_invalid))]
  } |
  select((.invalid_repairs | length > 0) or (.invalid_backups | length > 0)) |
  "\(.namespace)/\(.name)\n  Invalid repairs: \(if (.invalid_repairs | length) > 0 then (.invalid_repairs | join(", ")) else "(none)" end)\n  Invalid backups: \(if (.invalid_backups | length) > 0 then (.invalid_backups | join(", ")) else "(none)" end)\n"
') && if [ -z "$output" ]; then echo "All ScyllaCluster repair and backup task names are RFC 1123 compliant."; else echo "$output"; fi
```

You should get an output similar to the following if there are any `ScyllaClusters` with invalid repair or backup task names:

```console
scylla/example
  Invalid repairs: invalid_repair
  Invalid backups: invalid_backup
```

or the following if all `ScyllaCluster` repair and backup task names are compliant:

```console
All ScyllaCluster repair and backup task names are RFC 1123 compliant.
```

:::{note}
**Why is this necessary?**
Starting with v1.20.1, ScyllaDB Operator emitted warnings for `ScyllaCluster` repair and backup task names not conforming to RFC 1123 subdomain requirements.
In v1.21, these warnings have been replaced with hard validation errors, and the operator will refuse to start if any existing `ScyllaClusters` have non-conforming task names.
:::

#### Ensure ScyllaCluster spec.version is not empty

`ScyllaCluster` `spec.version` is now a required field. Any create or update request with an empty value will be rejected by the admission webhook.
You can run the following snippet to check whether any of your existing `ScyllaClusters` have an empty `spec.version`:

```bash
kubectl get scyllaclusters --all-namespaces -o json | jq -r '
  .items[] | select((.spec.version // "") == "") |
  "\(.metadata.namespace)/\(.metadata.name)"
'
```

If the command returns no output, all your `ScyllaClusters` are unaffected. If any are listed, set their `spec.version` to a valid ScyllaDB image tag before upgrading.

:::{note}
**Why is this not a breaking change?**
A `ScyllaCluster` with an empty `spec.version` was never functional. The migration controller would fail to reconcile it, leaving the cluster in a permanently degraded state. Making the field required only prevents new broken clusters from being created.
:::

### 1.17 to 1.18

Upgrading from v1.17.x requires extra actions due to the removal of the standalone ScyllaDB Manager controller.
The controller becomes an integral part of ScyllaDB Operator. As a result, the standalone ScyllaDB Manager controller deployment and its related resources
need to be removed before upgrading ScyllaDB Operator.

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

ScyllaDB Manager Helm installation has to be upgraded before upgrading ScyllaDB Operator Helm installation, following
the standard [Helm upgrade procedure](#upgrade-via-helm). This will ensure that the ScyllaDB Manager Controller
is removed before upgrading the ScyllaDB Operator.
