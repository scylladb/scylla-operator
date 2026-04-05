# pkg/controllerhelpers — Agent Guide

## Purpose

Shared infrastructure for all 18 controllers. Not business logic — plumbing. Import here instead of reimplementing.

## Key functions by category

### Handler wiring
```go
// Wire typed event handlers for an informer — generic, requires scheme + GVK
handlers, err := controllerhelpers.NewHandlers[*scyllav1.ScyllaCluster](
    queue, keyFunc, scheme, gvk, getterLister,
)
// Returns *Handlers[T] with methods: HandleAdd, HandleUpdate, HandleDelete,
// Enqueue, EnqueueAll, EnqueueOwner, EnqueueOwnerFunc, etc.
```

### Claim / release (via `pkg/controllertools`)
```go
// Claim child resources matching selector — lives in pkg/controllertools, not here
mgr := controllertools.NewControllerRefManager[*appsv1.StatefulSet](
    ctx, controller, selector, controllerGVK, controlFuncs,
)
claimed, err := mgr.ClaimObjects(statefulsets)

// Release orphaned child resources — this one IS in controllerhelpers
err := controllerhelpers.ReleaseObjects[CT, T](ctx, controller, controllerGVK, selector, control)
```

### Status conditions
```go
// Set a condition derived from an error (nil err → Available=True, non-nil → Degraded)
controllerhelpers.SetStatusConditionFromError(&conditions, err, conditionType, observedGeneration)

// Aggregate a slice of child conditions into a single parent condition
result, err := controllerhelpers.AggregateStatusConditions(childConditions, parentCondition)

// Aggregate Available/Progressing/Degraded by suffix convention
err := controllerhelpers.SetAggregatedWorkloadConditions(&conditions, generation)
```

### Finalizers
```go
// Produce a JSON merge-patch that removes/adds a finalizer (no network call — caller must Patch)
patchBytes, err := controllerhelpers.RemoveFinalizerPatch(obj, naming.MyFinalizer)
patchBytes, err := controllerhelpers.AddFinalizerPatch(obj, naming.MyFinalizer)
ok := controllerhelpers.HasFinalizer(obj, naming.MyFinalizer)
```

### WaitFor* helpers (tests only)
```go
// Generic watch-based wait — use specific wrappers below
obj, err := controllerhelpers.WaitForObjectState[*scyllav1.ScyllaCluster, *scyllav1.ScyllaClusterList](
    ctx, client, name, controllerhelpers.WaitForStateOptions{TolerateDelete: false}, conditionFn,
)

// Typed convenience wrappers (same signature pattern):
sc,  err := controllerhelpers.WaitForScyllaClusterState(ctx, client, name, opts, condFn)
sdc, err := controllerhelpers.WaitForScyllaDBDatacenterState(ctx, client, name, opts, condFn)
nc,  err := controllerhelpers.WaitForNodeConfigState(ctx, client, name, opts, condFn)
pod, err := controllerhelpers.WaitForPodState(ctx, client, name, opts, condFn)
// ...and more per resource type in wait.go
```

**WaitFor\* are test helpers** — never call them inside `sync()` or any controller path.

## Ownership / controller reference

`metav1.GetControllerOfNoCopy(obj)` (from `k8s.io/apimachinery`) — returns the ownerReference with `controller: true`. Not in this package.

When creating child resources, always set a controller reference using `metav1.NewControllerRef(owner, gvk)` before calling `resourceapply.ApplyXxx`.

## Common mistakes

- `ClaimXxx` functions don't exist here — use `pkg/controllertools.NewControllerRefManager`
- `SetStatusCondition` doesn't exist — use `SetStatusConditionFromError`
- `EnsureFinalizer`/`RemoveFinalizer` don't exist — use `AddFinalizerPatch`/`RemoveFinalizerPatch` (return bytes, require a separate Patch call)
- Don't call `WaitFor*` in production code paths — test-only
- `GetControllerRef` is `metav1.GetControllerOfNoCopy` from apimachinery, not this package
