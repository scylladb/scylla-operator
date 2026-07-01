# pkg/api — Agent Guide

## Layout

```
pkg/api/scylla/
  v1/                 # stable API — ScyllaCluster only
    types_cluster.go  # ScyllaCluster, ScyllaClusterSpec, ScyllaClusterStatus
    zz_generated.*    # DO NOT EDIT
  v1alpha1/           # all other CRDs + internal types
    types_*.go        # one file per CRD
    zz_generated.*    # DO NOT EDIT
cqlclient/scylla.scylladb.com/
  v1alpha1/
    types_cqlconnectionconfig.go
```

## CRD inventory

| Group | Version | Kind | Stability |
|---|---|---|---|
| `scylla.scylladb.com` | `v1` | ScyllaCluster | stable |
| `scylla.scylladb.com` | `v1alpha1` | ScyllaDBCluster | external |
| `scylla.scylladb.com` | `v1alpha1` | ScyllaDBDatacenter | external |
| `scylla.scylladb.com` | `v1alpha1` | ScyllaDBMonitoring | external |
| `scylla.scylladb.com` | `v1alpha1` | ScyllaOperatorConfig | external |
| `scylla.scylladb.com` | `v1alpha1` | NodeConfig | external |
| `scylla.scylladb.com` | `v1alpha1` | RemoteKubernetesCluster | external |
| `scylla.scylladb.com` | `v1alpha1` | ScyllaDBManagerTask | external |
| `scylla.scylladb.com` | `v1alpha1` | ScyllaDBManagerClusterRegistration | internal |
| `scylla.scylladb.com` | `v1alpha1` | ScyllaDBDatacenterNodesStatusReport | internal |
| `scylla.scylladb.com` | `v1alpha1` | RemoteOwner | internal |
| `cqlclient.scylla.scylladb.com` | `v1alpha1` | CQLConnectionConfig | external |

**Internal types** are implementation details between operator components — not for external consumers or documentation.

## API design conventions (enforced)

See `API_CONVENTIONS.md` at repo root for full rules. Key points:

**Discriminated unions:**
```go
// CORRECT
Type     StorageType  `json:"type"`
LocalOptions *LocalStorageOptions `json:"localOptions,omitempty"`

// WRONG — anonymous embedded struct, missing Type discriminator
Options struct { ... } `json:"options"`
```

**Optional fields:** always `omitempty` + pointer (`*T`) for structs, plain `omitempty` for scalars with meaningful zero values.

**Status conditions:** use `metav1.Condition` slice with standard `Type`, `Status`, `Reason`, `Message` fields. Reason must be CamelCase, no spaces.

**Defaulting/validation:** in `pkg/api/scylla/<version>/validation/` and `*_webhook.go`. Never inline validation in controllers.

## Generated code

After any type change, run:
```bash
make update-codegen   # deepcopy, client, informers, listers
make update-crds      # CRD YAML manifests (controller-gen)
```

Files named `zz_generated.*` or `*_generated.go` are **never edited by hand**.

## Import aliases

```go
scyllav1       "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
```

These are enforced by golangci-lint `importas`. Wrong aliases = lint failure.

## Adding a new CRD

1. Create `types_<name>.go` in the appropriate version directory
2. Add `+kubebuilder:object:root=true` and required markers
3. Register in `register.go` (add to `SchemeBuilder.Register(...)`)
4. Run `make update-codegen && make update-crds`
5. Add validation in `pkg/api/scylla/<version>/validation/`
6. Never add business logic to type files
