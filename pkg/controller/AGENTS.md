# pkg/controller — Agent Guide

## What lives here

18 controllers, each in its own subdirectory. Every controller is a standalone package; no shared init or registration magic.

## Controller anatomy (standard pattern)

```
<name>/
  controller.go   # struct definition, New(), Run(), informer handler wiring
  sync.go         # business logic: sync(), calculateXxx(), ensureXxx()
  options.go      # CLI flags (if applicable)
```

Some controllers split `sync.go` into multiple files by resource type (`sync_statefulsets.go`, `sync_services.go`, etc.).

## Two instantiation idioms

### 1. Informer-driven (most controllers)

```go
c := &Controller{
    queue: workqueue.NewTypedRateLimitingQueueWithConfig[string](...),
}
// Wire handlers in New():
scyllaInformers.V1().ScyllaClusters().Informer().AddEventHandler(
    cache.ResourceEventHandlerFuncs{...},
)
// Run() starts workers that call c.processNextItem() → c.sync(ctx, key)
```

Key: `controllerhelpers.NewHandlers(...)` for standard add/update/delete wiring.

### 2. Observer/singleton (bootstrapbarrier, ignition, globalscylladbmanager, statusreport)

```go
observer := controllertools.NewObserver(...)
observer.GetGenericHandlers()  // returns handler funcs to wire
```

Used for controllers that watch a single object or have no queue.

## Registration

Controllers are **NOT** registered via `SetupWithManager`. They are explicitly instantiated in `pkg/cmd/operator/operator.go` and started with `go c.Run(ctx, workers)`.

## sync() contract

- Takes `(ctx context.Context, key string)` — key is `namespace/name` or just `name`
- Must be idempotent
- Returns `error` (requeueing is handled by the workqueue caller)
- Use `controllerhelpers.WaitForCacheSync(...)` at startup before processing

## maxSyncDuration

Every controller has a `maxSyncDuration` const. **Do not raise it.** Controllers must not actively wait (no `time.Sleep`, no polling loops in sync). If you need to wait, re-enqueue with a delay via the workqueue.

## Status updates

Use `controllerhelpers.SetStatusCondition` / `AggregateStatusConditions` — never build condition lists manually. Patch status via `UpdateStatus` sub-resource, not full object update.

## Claim/release pattern

Child resources (StatefulSets, Services, etc.) are "claimed" to a controller instance via label selectors + `controllerhelpers.ClaimXxx(...)`. Never directly own resources that weren't claimed.

## Adding a new controller

1. Create `pkg/controller/<name>/` with `controller.go` + `sync.go`
2. Add constructor `New(...)` that accepts informers + clients
3. Register in `pkg/cmd/operator/operator.go` (look for the block of `NewXxx(...)` calls)
4. Add to `pkg/cmd/operator/cmd.go` worker-count flags if needed
5. Wire informer cache sync in the controller's `Run()` before starting workers

## Common mistakes

- Never `time.Sleep` in `sync()` — use `workqueue.AddAfter`
- Never call `Update` on the full object to change status — always use `UpdateStatus` sub-resource
- Never hardcode label keys — import `pkg/naming`
- Observer controllers do NOT have a queue; don't add one
