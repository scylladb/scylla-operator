# helm/

Three Helm charts for deploying the operator ecosystem. **CRDs and deploy manifests are generated** — never edit them manually.

## Charts

| Chart | Purpose | Key content |
|---|---|---|
| `helm/scylla-operator/` | Operator + webhooks + RBAC + CRDs | Operator Deployment, webhookserver, ClusterRoles, cert-manager resources, CRDs in `crds/` |
| `helm/scylla/` | ScyllaCluster CR deployment | Wraps a `ScyllaCluster` CR and optional ServiceMonitor |
| `helm/scylla-manager/` | Scylla Manager deployment | Manager Deployment + ConfigMap; depends on `scylla` subchart |

`scylla-manager` depends on `scylla` via `file://../scylla` — the embedded subchart lives at `helm/scylla-manager/charts/scylla/`.

## Key values (scylla-operator)

```yaml
replicas: 2                     # 1 disables HA and PDB creation
image:
  repository: scylladb
  tag: ""                       # defaults to chart appVersion
logLevel: 2
additionalArgs: []
resources:
  requests: { cpu: 100m, memory: 20Mi }

# Webhook server (separate Deployment)
webhookServerReplicas: 2
webhookServerResources: ...

# TLS
webhook:
  createSelfSignedCertificate: true
  certificateSecretName: ""     # override generated name

# RBAC
serviceAccount:
  create: true
  name: ""
  annotations: {}
```

Version placeholders: `Chart.yaml` versions are `0.0.0` / `latest` — overwritten by `make update-helm-charts` during release. Do not rely on `Chart.yaml` for version info; check `assets/config/config.yaml`.

## Template conventions

- Use `_helpers.tpl` for all naming: `include "scylla-operator.fullname" .`, `include "scylla-operator.serviceAccountName" .`, etc. **Never hardcode release names.**
- Labels follow `app.kubernetes.io/` conventions — use the `include "scylla-operator.labels"` helper.
- RBAC split: `*.clusterrole.yaml` = the ClusterRole resource; `*.clusterrole_def.yaml` = aggregate/docs variant. Follow the existing pattern when adding new roles.
- OpenShift variants use a `_openshift.yaml` / `_def_openshift.yaml` suffix.
- CRDs live in `helm/scylla-operator/crds/` — Helm installs these before all templates. They are **not templated**.

## CRDs — never edit manually

CRDs are generated from Go API types in `pkg/api/`:

```bash
make update-crds          # regenerate CRDs from pkg/api using controller-gen
make verify-crds          # verify committed CRDs match generated output (run in CI)
```

After `update-crds`, commit the updated files under `helm/scylla-operator/crds/`.

## Helm value schemas — also generated

`values.schema.json` files are derived from the CRD OpenAPI schemas:

```bash
make update-helm-schemas  # regenerate from CRDs
make verify-helm-schemas  # CI check
```

## deploy/ manifests — also generated

`deploy/` YAML files are produced by running `helm template` on the charts:

```bash
make update-deploy        # regenerate deploy/ from helm template
make verify-deploy        # CI check
```

The Makefile maps specific helm template outputs to numbered deploy manifest filenames (`00_`, `10_`, `50_` prefix scheme).

## Full update workflow

**After changing a Go API type:**
```bash
make update-crds
make update-helm-schemas
make update-deploy
make verify
```

**After changing chart templates or values:**
```bash
helm lint helm/scylla-operator    # local syntax check
make update-deploy
make verify-deploy
```

**For a release:**
```bash
make update-helm-charts           # bump appVersion from assets/config/config.yaml
make helm-build                   # package charts
make helm-publish                 # publish to GCS
```

## Gotchas

| Situation | What to do |
|---|---|
| Changed a CRD type | `make update-crds && make update-helm-schemas && make update-deploy` |
| Added a new value | Update `values.yaml` and `values.schema.json` (via `make update-helm-schemas`) |
| Changed deploy manifests | `make update-deploy` — never edit `deploy/` by hand |
| scylla-manager subchart out of date | Run `helm dependency update helm/scylla-manager` |
| CI fails on verify-crds/verify-deploy | Regenerate the artifacts and commit them |

## CI deploy scripts

- `hack/ci-deploy.sh` — installs third-party components (cert-manager, prometheus-operator) then applies operator manifests. Waits for `crd/... condition=established` before applying manager.
- `hack/ci-deploy-release.sh` — fetches version info from image labels; used for release deployments.
