# pkg/naming — Agent Guide

## Golden rule

**Every label key, annotation key, resource name, and constant that appears in more than one file must come from this package.** Never hardcode strings that could live here.

## What's here

| File | Contents |
|---|---|
| `names.go` | Name generators: `StatefulSetNameForRack`, `ServiceNameForPod`, `HeadlessServiceNameForCluster`, `ConfigMapNameForRack`, `SecretNameForAgent`, etc. |
| `labels.go` | Label key constants (`OwnerUIDLabel`, `ClusterNameLabel`, `DatacenterNameLabel`, `RackNameLabel`, …) and label-set builders (`ClusterLabels`, `RackLabels`, `PodLabels`, …) |
| `annotations.go` | Annotation key constants |
| `constants.go` | Port names/numbers, container names, file paths, feature flag names, finalizer strings |
| `selectors.go` | `LabelSelectorForRack`, `LabelSelectorForCluster`, etc. |

## Usage pattern

```go
import "github.com/scylladb/scylla-operator/pkg/naming"

// Name generation
stsName := naming.StatefulSetNameForRack(rack, sc)
svcName := naming.HeadlessServiceNameForCluster(sc)

// Label keys
labels := naming.ClusterLabels(sc)
labels[naming.RackNameLabel] = rack.Name

// Constants
containerName := naming.ScyllaContainerName  // "scylla"
port := naming.CQLPort                       // 9042
finalizer := naming.ScyllaFinalizer
```

## Adding new constants / generators

1. Choose the right file (`names.go` for generators, `constants.go` for literals, `labels.go` for label keys)
2. Follow the existing naming convention: exported, no abbreviations, self-documenting
3. If a name depends on a CR (e.g., `StatefulSetNameForRack(rack, sc)`), add a function — don't inline templates elsewhere
4. After adding, verify all callers compile: `make build`

## Common mistakes

- Hardcoding `"scylla"` instead of `naming.ScyllaContainerName`
- Hardcoding `"scylla/rack"` instead of `naming.RackNameLabel`
- Building label maps without `naming.ClusterLabels(sc)` as base
- Duplicating a name generator in a controller instead of adding it here
