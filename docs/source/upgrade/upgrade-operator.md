# Upgrade ScyllaDB Operator

Upgrade ScyllaDB Operator to a newer release to pick up bug fixes, new features, and CRD schema changes.

:::{caution}
ScyllaDB Operator supports **N+1 upgrades only** — you can only upgrade by one minor version at a time.
If you are more than one minor version behind, upgrade through each intermediate version sequentially.
:::

## Before you begin

:::{tip}
**Review the release notes** for the target version before upgrading.
Release notes describe new features, breaking changes, and any version-specific migration steps.
Find them at [ScyllaDB Operator Releases](https://github.com/scylladb/scylla-operator/releases).
:::

1. Verify the current Operator version:
   ```bash
   kubectl -n scylla-operator get deployment scylla-operator -o jsonpath='{.spec.template.spec.containers[0].image}'
   ```

2. Check the [compatibility matrix](../install-operator/prerequisites.md) to confirm the target Operator version supports your Kubernetes and ScyllaDB versions.

3. Ensure the cluster is healthy — no degraded ScyllaClusters or pending rollouts:
   ```bash
   kubectl get scyllaclusters -A -o wide
   ```

## Upgrade via Helm

### 1. Update the chart repository

```bash
helm repo update
```

### 2. Update CRDs

Helm does not update CRDs after the initial installation.
You must apply them manually before upgrading the chart:

```bash
tmpdir=$( mktemp -d ) \
  && helm pull scylla-operator/scylla-operator --version <version> --untar --untardir "${tmpdir}" \
  && find "${tmpdir}"/scylla-operator/crds/ -name '*.yaml' -printf '-f=%p ' \
  | xargs kubectl apply --server-side
```

Replace `<version>` with the target chart version.

### 3. Upgrade the chart

```bash
helm upgrade --namespace scylla-operator --version <version> <release_name> scylla-operator/scylla-operator
```

### 4. Wait for the rollout

```bash
kubectl -n scylla-operator rollout status deployment/scylla-operator
kubectl -n scylla-operator rollout status deployment/webhook-server
```

## Upgrade via GitOps / kubectl

Re-apply the operator manifest from the target release:

```bash
kubectl apply --server-side -f https://raw.githubusercontent.com/scylladb/scylla-operator/<version>/deploy/operator.yaml
```

Replace `<version>` with the target release tag (for example, `v1.19.0`).

The monolithic manifest includes CRDs, so they are updated automatically with `kubectl apply --server-side`.

Wait for the rollout:

```bash
kubectl -n scylla-operator rollout status deployment/scylla-operator
kubectl -n scylla-operator rollout status deployment/webhook-server
```

:::{note}
If you pin container images for production, update **both** the Deployment's container `image` and the `SCYLLA_OPERATOR_IMAGE` environment variable to the same reference.
A mismatch causes version skew between the Operator and the sidecar running inside ScyllaDB pods.
:::

## After upgrading

Update any external dependencies to compatible versions:

| Dependency | Notes |
|---|---|
| cert-manager | Check the [prerequisites](../install-operator/prerequisites.md) for the required version. |
| Prometheus Operator | Required only if you use `ScyllaDBMonitoring`. |
| ScyllaDB Manager | If managed via Helm, upgrade its chart as well. |

## Rollback

If the upgrade causes issues, revert to the previous version:

::::{tabs}
:::{group-tab} Helm
```bash
helm rollback <release_name> --namespace scylla-operator
```

Or upgrade explicitly to the previous version:

```bash
helm upgrade --namespace scylla-operator --version <previous_version> <release_name> scylla-operator/scylla-operator
```
:::
:::{group-tab} GitOps / kubectl
Re-apply the manifest from the previous release tag:

```bash
kubectl apply --server-side -f https://raw.githubusercontent.com/scylladb/scylla-operator/<previous_version>/deploy/operator.yaml
```
:::
::::

:::{warning}
CRD rollbacks require care.
Newer CRDs may add fields that older versions of the Operator do not recognize.
If you revert CRDs, verify that no ScyllaDB resources use newly introduced fields.
:::

## Webhook availability during upgrades

The Operator deploys a separate `webhook-server` Deployment that validates ScyllaDB custom resources.
Its `ValidatingWebhookConfiguration` uses `failurePolicy: Fail` — if no webhook-server pod is available, all create and update operations on ScyllaDB resources are rejected.

The default configuration runs two webhook-server replicas with a `PodDisruptionBudget` (`minAvailable: 1`), ensuring at least one pod remains available during a rolling update.

:::{caution}
If you set `webhookServerReplicas` to `1`, no PDB is created and webhook availability is not guaranteed during upgrades.
ScyllaDB resource mutations will fail while the single pod is restarting.
:::

## Related pages

- [Prerequisites](../install-operator/prerequisites.md) — version compatibility matrix
- [Install with Helm](../install-operator/install-with-helm.md) — initial installation procedure
- [Upgrade ScyllaDB](upgrade-scylladb.md) — upgrading the ScyllaDB version itself
- [Upgrade Kubernetes](upgrade-kubernetes.md) — upgrading the Kubernetes control plane and node pools
