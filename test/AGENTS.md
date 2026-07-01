# test — Agent Guide

## Three distinct test tiers

| Tier | Location | Build tag | Runner | When to use |
|---|---|---|---|---|
| Unit | `pkg/**/*_test.go` | none | `go test ./...` | Pure logic, no k8s API |
| Envtest | `test/envtest/` | `envtest` | `go test -tags envtest` | Controller logic with real API server |
| E2E | `test/e2e/` | none | custom runner binary | Full cluster scenarios |

## Unit tests

- Colocated with source in `pkg/`
- Standard Go testing, table-driven (`[]struct{ name, input, expected }`)
- No Ginkgo — plain `t.Run` / `t.Error`
- Run: `make test-unit` or `go test ./pkg/...`

## Envtest tests (`test/envtest/`)

```
test/envtest/
  controllers/    # one package per controller under test
  setup.go        # shared env setup: NewEnvTestSuite, StartEnv, StopEnv
```

- Build tag: `//go:build envtest` in every file
- Ginkgo v2 + Gomega; suite entry in `*_test.go` via `RunSpecs`
- Uses `sigs.k8s.io/controller-runtime/pkg/envtest` for API server
- Run: `go test -tags envtest ./test/envtest/controllers/...`
- Shared fixtures in `test/envtest/setup.go` — reuse `NewEnvTestSuite`, don't re-implement env setup

## E2E tests (`test/e2e/`)

```
test/e2e/
  set/          # test sets, each a plain .go package
  include.go    # blank imports that register all test sets
  framework/    # test framework helpers (Framework, ScyllaCluster helpers, etc.)
```

- Tests are **registered via side-effect imports** in `include.go`, not discovered automatically
- NOT run via `go test` directly — use the custom binary: `go run ./cmd/scylla-operator-tests run all`
- Kind cluster setup: `KIND_EXPERIMENTAL_PROVIDER=podman CLUSTER_NAME=test make test-e2e-kind`
- Requires a live cluster with the operator deployed

## Framework (`test/e2e/framework/`)

```go
f := framework.NewFramework(ctx, "my-test")
// f.Namespace() — creates a unique test namespace, auto-deleted on cleanup
sc := f.NewScyllaCluster(ctx, ...)

// Waiting uses controllerhelpers.WaitForXxxState — NOT framework methods:
sc, err = controllerhelpers.WaitForScyllaClusterState(
    ctx, f.ScyllaClient().ScyllaV1().ScyllaClusters(sc.Namespace), sc.Name,
    controllerhelpers.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut,
)
```

Key helpers: `f.ScyllaClient()` (returns `*scyllaclientset.Clientset`). Wait helpers are in `pkg/controllerhelpers` (`WaitForScyllaClusterState`, `WaitForScyllaDBDatacenterState`, `WaitForNodeConfigState`, etc.), not in the framework package.

## Hard rules

- **No Ginkgo Focus markers**: `FIt`, `FDescribe`, `FContext`, `FWhen` — CI will fail immediately
- New e2e test sets must be blank-imported in `test/e2e/include.go`
- Envtest files must carry `//go:build envtest` tag
- Don't add cluster-level assertions in unit tests — use envtest or e2e
- `WaitFor*` helpers from `pkg/controllerhelpers` are for tests — never call in production sync()
