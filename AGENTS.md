# scylla-operator — Agent Guide

## What this repo is

Kubernetes operator for ScyllaDB. Written in Go. Uses **client-go informers + workqueue** — NOT controller-runtime Reconciler. All business logic lives in `pkg/`. `cmd/` contains thin `main()` entrypoints only.

## Repo layout deviations (vs Kubebuilder defaults)

| Expected path | Actual path | Notes |
|---|---|---|
| `api/` | `pkg/api/scylla/` | All CRD types here |
| `controllers/` | `pkg/controller/` | 18 controllers |
| `config/` (kustomize) | `deploy/` + `helm/` | Raw manifests + Helm chart |
| reconciler cobra | `pkg/cmd/` | All cobra/flag logic here |

`submodules/scylla-monitoring/` — full git submodule. Do not modify. `pkg/thirdparty/` — in-tree forks of external helpers. Do not replace with upstream imports.

## API groups

- `scylla.scylladb.com/v1`: **ScyllaCluster** (stable)
- `scylla.scylladb.com/v1alpha1`: ScyllaDBCluster, ScyllaDBDatacenter, ScyllaDBMonitoring, ScyllaOperatorConfig, NodeConfig, RemoteKubernetesCluster, ScyllaDBManagerTask, ScyllaDBManagerClusterRegistration, ScyllaDBDatacenterNodesStatusReport, RemoteOwner
- `cqlclient.scylla.scylladb.com/v1alpha1`: CQLConnectionConfig

Types marked `*(internal)*` are implementation details — not for external consumers.

## Core packages (import these, don't reinvent)

| Package | Use for |
|---|---|
| `pkg/naming` | ALL labels, annotation keys, name generators, constants — **always** import this |
| `pkg/controllerhelpers` | Handler wiring, claim/release, status aggregation, finalizers, WaitFor* |
| `pkg/resourceapply` | Idempotent ApplyXxx(required) — hash-based create/update for k8s resources |
| `pkg/helpers` | IP parsing, MergeMaps, ParseScyllaArguments, CreateTwoWayMergePatch |
| `pkg/pointer` | `pointer.Ptr[T](v)` — generic pointer helper |
| `pkg/scyllaclient` | HTTP client to ScyllaDB REST API |
| `pkg/remoteclient` | Multi-cluster remote client management |

## Commit message format

Conventional Commits: `<type>(<scope>): <subject>`
- Imperative, lowercase subject, ≤72 chars
- No `@mentions`, no issue-closing keywords (`Fixes #N`, `Closes #N`), no `#N` refs in message body
- Types: `feat`, `fix`, `refactor`, `test`, `docs`, `ci`, `chore`

## Import aliases (enforced by golangci-lint `importas`)

```go
corev1 "k8s.io/api/core/v1"
metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
oslices "github.com/scylladb/scylla-operator/pkg/helpers/slices"
```
Run `make lint` — it will catch wrong aliases. Full list in `.golangci.yaml` under `linters-settings.importas`.

## Build & verify commands

```bash
make build                        # build all binaries
make test-unit                    # unit tests
make verify                       # ALL CI checks — run before any PR
make lint                         # golangci-lint
make gofmt                        # gofmt check
make update-codegen               # regenerate deepcopy/client/informers/listers
make update-crds                  # regenerate CRD YAMLs (controller-gen)
make update-go-mod-replace        # update go.mod replace entries (DO NOT edit manually)
```

E2E requires podman: `KIND_EXPERIMENTAL_PROVIDER=podman CLUSTER_NAME=test make test-e2e-kind`

## Hard rules

- **Never edit generated files**: `zz_generated.*`, `clientset_generated.*`, `*_generated.go`
- **Never manually edit `go.mod` replace entries** — use `make update-go-mod-replace`
- **Never hardcode names/labels** — always use `pkg/naming`
- **Never use controller-runtime Reconcile pattern** — use informers + workqueue (see `pkg/controller/`)
- **Never raise `maxSyncDuration`** in controllers — controllers must not actively wait
- **No ginkgo Focus markers**: `FIt`, `FDescribe`, `FContext`, `FWhen` — CI will fail
- `make lint` + `make gofmt` must pass before commit

## API design conventions

Discriminated unions: `Type` field (required string) + `<Type>Options *TypeOptions omitempty`. See `API_CONVENTIONS.md` for full rules.

## Go toolchain

Go 1.25.1. Requires GNU Make ≥ 4.2.
