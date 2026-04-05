# pkg/resourceapply — Agent Guide

## Purpose

Idempotent create-or-update for Kubernetes resources. All controllers use this instead of raw client calls.

## The Apply pattern

```go
// Signature (all ApplyXxx functions follow this shape):
func ApplyStatefulSet(
    ctx context.Context,
    client appsv1client.StatefulSetsGetter,
    lister appsv1listers.StatefulSetLister,
    recorder record.EventRecorder,
    required *appsv1.StatefulSet,
    options ApplyOptions,
) (*appsv1.StatefulSet, bool, error)
```

- `required` — the desired state (what you want it to look like)
- Returns `(actual, changed, error)`
- `changed == true` → resource was created or updated
- Internally: computes a hash of `required`, compares with existing, skips update if hash matches

## ApplyOptions

```go
type ApplyOptions struct {
    AllowMissingControllerRef bool  // set true only for cluster-scoped resources
    ForceOwnership            bool  // overwrite fields owned by other field managers
}
```

Default (zero value) is correct for most controller usage.

## Available Apply functions

```
ApplyConfigMap       ApplySecret          ApplyService
ApplyStatefulSet     ApplyDaemonSet       ApplyDeployment
ApplyServiceAccount  ApplyClusterRole     ApplyClusterRoleBinding
ApplyRole            ApplyRoleBinding     ApplyJob
ApplyPodDisruptionBudget                  ApplyValidatingWebhookConfiguration
```

All live in files named after the resource type (`core.go`, `apps.go`, `rbac.go`, etc.).

## Usage in sync()

```go
// Build desired state
required := &appsv1.StatefulSet{
    ObjectMeta: metav1.ObjectMeta{
        Name:      naming.StatefulSetNameForRack(rack, sc),
        Namespace: sc.Namespace,
        Labels:    naming.RackLabels(rack, sc),
        OwnerReferences: []metav1.OwnerReference{
            *metav1.NewControllerRef(sc, scyllaClusterGVK),
        },
    },
    Spec: ...,
}

// Apply — idempotent
sts, changed, err := resourceapply.ApplyStatefulSet(ctx, c.kubeClient.AppsV1(), c.statefulSetLister, c.eventRecorder, required, resourceapply.ApplyOptions{})
if err != nil {
    return err
}
```

## Hash mechanism

The package annotates applied objects with a hash of the desired spec. On the next reconcile it recomputes the hash and skips the API call if unchanged. This means:
- Never mutate the object after calling `Apply` and expect the change to persist
- Never manually set the hash annotation — it's managed internally

## Common mistakes

- Calling `client.Create` / `client.Update` directly — always use `ApplyXxx`
- Forgetting to set `OwnerReferences` on `required` before calling Apply
- Passing a mutated existing object as `required` instead of a freshly constructed desired state
- Checking `changed` to decide whether to requeue — it's informational, not a requeue signal
