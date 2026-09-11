# Controllers deciding from caches that have not observed their own writes (OPERATOR-392)

Section 9 records the team's decision. This document closes the discovery work of
[OPERATOR-393](https://scylladb.atlassian.net/browse/OPERATOR-393) (how the ScyllaDBDatacenter controller gets
read-your-writes) and [OPERATOR-397](https://scylladb.atlassian.net/browse/OPERATOR-397) (the controller-runtime
migration steps and challenges). It compares two proofs of concept:

- **the client-go PoC**, [PR #3650](https://github.com/scylladb/scylla-operator/pull/3650), pinned at
  [`02390041f`](https://github.com/czeslavo/scylla-operator/tree/02390041f): an in-house consistency layer over
  client-go informers;
- **the controller-runtime PoC**, [PR #3652](https://github.com/scylladb/scylla-operator/pull/3652), pinned at
  [`f334d7813`](https://github.com/czeslavo/scylla-operator/tree/f334d7813): a migration to controller-runtime and its
  read-your-writes client.

Conventions used throughout:

- **Approach A** is the client-go layer and **approach B** is controller-runtime. They are the two mechanisms.
- **Options 1, 2 and 3** are the three ways to ship (section 2). Options 1 and 2 are approach B at two scopes; option 3
  is approach A.
- **Milestone 1** is approach B for the ScyllaDBDatacenter controller only; **milestone 2** is approach B for the whole
  operator.
- **Legacy controllers** are the controllers still on our hand-written client-go machinery.
- **Multi-DC** means the automated multi-DC controllers, ScyllaDBCluster and RemoteKubernetesCluster. The document
  assumes they are removed rather than migrated; 4.4 says why and what that removes.
- **e2e** is the end-to-end test suites that run on CI.
- Line counts are non-test Go lines, excluding `vendor/`, `go.mod` and `go.sum`.
- Code links point at the pinned commit of the PoC they discuss, or at master
  [`5d041a933`](https://github.com/scylladb/scylla-operator/tree/5d041a933) for the code both PoCs replace.

---

## 1. The problem

[OPERATOR-392](https://scylladb.atlassian.net/browse/OPERATOR-392): the ScyllaDBDatacenter controller makes scale
decisions, publishes status and sequences decommissions from cached state that does not yet reflect its own writes.
Informer caches give no read-your-writes guarantee. Two bugs in the epic are instances of this hazard:

- [OPERATOR-369](https://scylladb.atlassian.net/browse/OPERATOR-369): a sync that runs between the controller stamping
  a `scylla/decommissioned` label on a member Service and the Service informer delivering it can miss a decommission
  the controller itself just requested, and scale the rack up over the leaving node.
- [OPERATOR-376](https://scylladb.atlassian.net/browse/OPERATOR-376): a sync that has not yet observed a StatefulSet it
  applied recomputes rack status from the pre-apply object, whose `observedGeneration` matches its `generation`, and so
  reports a rack as rolled out for a rollout that has not started.

Today's mitigation for the second is a fixed sleep after every applied StatefulSet change: three call sites in
[`sync_statefulsets.go`](https://github.com/scylladb/scylla-operator/blob/5d041a933/pkg/controller/scylladbdatacenter/sync_statefulsets.go#L883)
([and](https://github.com/scylladb/scylla-operator/blob/5d041a933/pkg/controller/scylladbdatacenter/sync_statefulsets.go#L1118)
[two](https://github.com/scylladb/scylla-operator/blob/5d041a933/pkg/controller/scylladbdatacenter/sync_statefulsets.go#L1296)
more), 10 s by default
([`controller.go:93`](https://github.com/scylladb/scylla-operator/blob/5d041a933/pkg/controller/scylladbdatacenter/controller.go#L93)).
It is a guess rather than a check, it holds a worker for 10 s per apply, and it pays the full delay even when the cache
was already current. **Both PoCs delete these three cache-propagation sleeps.** Neither touches the unrelated 10 s sleep
in the node-replace path, which compensates for the StatefulSet controller not reconciling PVCs
([`sync_services.go:460`](https://github.com/scylladb/scylla-operator/blob/5d041a933/pkg/controller/scylladbdatacenter/sync_services.go#L460)).

## 2. The options

1. **Milestone 1 now, milestone 2 later.** The ScyllaDBDatacenter controller moves to controller-runtime; the other
   controllers stay legacy and share its cache through a bridge (4.2). The rest of the operator is separate, later work.
2. **Milestone 2 now.** The whole operator moves to controller-runtime in one go: 16 controllers across five binaries.
3. **The client-go layer.** We keep our controller machinery and add an in-house consistency layer that the
   ScyllaDBDatacenter controller reads and writes through.

Section 8 spells out what each option commits us to; section 9 lists the decisions.

## 3. Approach A: the client-go layer

PoC: [PR #3650](https://github.com/scylladb/scylla-operator/pull/3650), seven commits. No dependency bump, no vendored
change.

### 3.1 Implementation

A new package, [`pkg/cacheconsistency`](https://github.com/czeslavo/scylla-operator/blob/02390041f/pkg/cacheconsistency),
1030 lines in three layers:

- a [`ConsistencyStore`](https://github.com/czeslavo/scylla-operator/blob/02390041f/pkg/cacheconsistency/consistencystore.go#L21)
  per controller (113 lines): a map from group-version-kind (GVK) to one handler per informer, resolved through the
  operator's scheme;
- a [`consistencyHandler`](https://github.com/czeslavo/scylla-operator/blob/02390041f/pkg/cacheconsistency/consistencyhandler.go#L81)
  per kind (382 lines), registered as a plain event handler on the informer. It holds the highest resource version it
  has seen, the per-object watermark each write must reach (keyed by namespace and name), and the deletes it is waiting
  on (keyed by UID);
- **recording clients** (535 lines across
  [`recordingclient.go`](https://github.com/czeslavo/scylla-operator/blob/02390041f/pkg/cacheconsistency/recordingclient.go),
  [`kubeclient.go`](https://github.com/czeslavo/scylla-operator/blob/02390041f/pkg/cacheconsistency/kubeclient.go) and
  [`scyllaclient.go`](https://github.com/czeslavo/scylla-operator/blob/02390041f/pkg/cacheconsistency/scyllaclient.go))
  that wrap the typed clientsets and report each write to the consistency store.

The handler learns what the informer has observed from the **maximum of two sources**: the informer store's own
watermark, `LastStoreSyncResourceVersion()`, which client-go fills only when the `AtomicFIFO` feature gate is on (beta
and on by default since client-go 1.36), and its own event stream. It also polls every 100 ms, because bookmarks and
relists advance the informer store's watermark without producing handler events
([`consistencyhandler.go:150`](https://github.com/czeslavo/scylla-operator/blob/02390041f/pkg/cacheconsistency/consistencyhandler.go#L150)).

Recording covers create, update, patch (including subresource patches), delete, status update, apply and apply-status,
plus two per-kind extras: evictions are recorded as deletes, and a StatefulSet scale update is recorded against the
StatefulSet, because the Scale response carries its resource version
([`kubeclient.go:213`](https://github.com/czeslavo/scylla-operator/blob/02390041f/pkg/cacheconsistency/kubeclient.go#L213)).
`DeleteCollection` passes through unrecorded, deliberately, because the operator does not use it.

### 3.2 Where the wait for the controller's own writes happens

Both of the controller's clients are wrapped **in place**, so no call site changed
([`controller.go:211`](https://github.com/czeslavo/scylla-operator/blob/02390041f/pkg/controller/scylladbdatacenter/controller.go#L211)),
and 12 kinds are registered against the controller's informers in one table
([`controller.go:202`](https://github.com/czeslavo/scylla-operator/blob/02390041f/pkg/controller/scylladbdatacenter/controller.go#L202)).
Reads are not intercepted. Instead the sync calls `WaitReady` **once, at the top**, before its first lister read, under
a one-minute timeout
([`sync.go:38`](https://github.com/czeslavo/scylla-operator/blob/02390041f/pkg/controller/scylladbdatacenter/sync.go#L38)).
That is the whole barrier: after it returns, every lister read in that sync is consistent with everything this
controller wrote.

Two consequences follow from gating there, and they are the crux of the comparison in section 5:

- **The barrier is global, not per read.** One `WaitReady` covers every registered kind and every pending key, so a
  write still in flight for one ScyllaDBDatacenter delays the sync of every other one, on every worker.
- **Writes made later inside a sync are not waited on within that sync.** The sync reads its state up front, so this is
  a design property rather than a defect, but it is a property the code must keep honouring.

The one adjacent refactor is
[`PruneObjects`](https://github.com/czeslavo/scylla-operator/blob/02390041f/pkg/controllerhelpers/prune.go#L35), which
now returns the objects it deleted, so that the five hand-written prune loops it replaced can still emit one progressing
condition per deleted object. It also fixes a pre-existing wart: those loops reported the condition before the delete
and appended a nil error.

### 3.3 What it does not cover

- **Objects the controller did not write** are outside the model by construction. The nodes status reports written by
  the statusreport controller and the ScyllaOperatorConfig are read but never registered.
- **A delete of an object the informer store does not hold records nothing.** The UID is taken from the informer store
  rather than from the delete preconditions, and the lookup returns silently when the object is absent
  ([`consistencyhandler.go:202`](https://github.com/czeslavo/scylla-operator/blob/02390041f/pkg/cacheconsistency/consistencyhandler.go#L202)).
  The prune sites feed their objects from listers, so the object is there; ad-hoc deletes are the hole. The prune path
  already sets the UID precondition, so this is a fixable gap rather than a design limit.
- **No per-call opt-out.** A kind is either registered or it is not.

### 3.4 Extending it to other controllers

Per controller, the mechanical part: a consistency store, a registration table for every kind it both writes and reads
from a cache, `HasSynced` in `cachesToSync`, the client wrapping, and a `WaitReady` at the top of its sync.

The part that is not mechanical is the recording clients. About 290 of the 1030 lines are hand-written per group and
per kind, and the kinds the operator writes elsewhere have none yet: DaemonSets, Deployments, Endpoints, Namespaces,
Nodes, PersistentVolumeClaims, Roles, ClusterRoles, ClusterRoleBindings, `scylla.v1` ScyllaClusters, NodeConfigs,
ScyllaDBManagerClusterRegistrations and ScyllaDBManagerTasks, plus the Prometheus-operator client the monitoring
controller uses.

So the scope statement is: this approach fixes the ScyllaDBDatacenter controller now, and every further controller that
needs the guarantee is a small piece of per-controller work plus, for most of them, new per-kind recording clients.

## 4. Approach B: controller-runtime

PoC: [PR #3652](https://github.com/scylladb/scylla-operator/pull/3652), 29 commits on top of master. Milestone 1 is the
first six commits, up to [`a60e50ee2`](https://github.com/czeslavo/scylla-operator/commit/a60e50ee2); milestone 2 is
the rest.

### 4.1 The guarantee

The mechanism is upstream's, enabled by one flag:
[`client.CacheOptions.EnableReadYourWritesConsistency`](https://github.com/kubernetes-sigs/controller-runtime/blob/v0.25.0/pkg/client/client.go#L97).
Upstream documents it as experimental, in those words: a form of it will be kept, but how it works and how it is
configured may change. The flag is set in one place, the operator's manager
([`manager.go:86`](https://github.com/czeslavo/scylla-operator/blob/f334d7813/pkg/controllermanager/manager.go#L86)).

The read-your-writes client keeps, per (GVK, representation, key): a write barrier for writes in flight, the highest
resource version it has written, and the deletes it is waiting to see. It learns what the cache has observed from an
event handler it registers on the informer, which it builds on first use and waits for to sync. Then:

- a `Get` waits until the informer has observed at least the resource version this client last wrote **for that key**,
  and until every pending delete for that key has been observed
  ([`consistency.go:122`](https://github.com/kubernetes-sigs/controller-runtime/blob/v0.25.0/pkg/client/consistency.go#L122));
- a `List` is a **kind-wide** barrier, and the bookkeeping is per client, not per controller: it waits for every write
  of that kind still in flight through the client and for the highest resource version any caller recorded for that
  kind ([`consistency.go:150`](https://github.com/kubernetes-sigs/controller-runtime/blob/v0.25.0/pkg/client/consistency.go#L150)).
  All 12 controllers of the operator binary share one client, so an SDC reconcile listing Pods waits for Pod writes
  made by any controller in the process. For completed writes that costs the informer lag between the write it cares
  about and the newest one, since there is one informer per kind either way; for a write still in flight it costs that
  request's latency, whichever controller issued it;
- `Create`, `Update`, `Patch`, `Apply` and every subresource write record their resource version; `Delete` registers the
  pending delete *before* issuing the request, and prefers the response's resource version when the object survives
  behind a finalizer
  ([`consistency.go:335`](https://github.com/kubernetes-sigs/controller-runtime/blob/v0.25.0/pkg/client/consistency.go#L335)).

The representation is part of the key, so a typed write does not block an unstructured read of the same object. There is
a per-call opt-out, `client.DisableReadYourWritesConsistency`.

### 4.2 Milestone 1: the ScyllaDBDatacenter controller

Controller-runtime does not slot into this codebase for free: the controllers call `resourceapply` and
`controllerhelpers` functions that take client-go typed clients and listers. Milestone 1 adds the glue that bridges that
gap and rewires one controller.

**The manager package**
([`pkg/controllermanager`](https://github.com/czeslavo/scylla-operator/blob/f334d7813/pkg/controllermanager), 152 + 546
+ 80 lines at milestone 1). `manager.go` builds one manager with the operator's scheme, the resync period moved onto the
cache, the read-your-writes client, and every parity decision from 4.5
([`manager.go:71`](https://github.com/czeslavo/scylla-operator/blob/f334d7813/pkg/controllermanager/manager.go#L71)).
`controllers.go` is where all controller wiring now lives, moved out of `pkg/cmd/operator/operator.go`; at milestone 1
it carries the legacy wiring unchanged, which is why it is large. At runtime the change is one cache instead of four
informer factories.

`informers.go` is **the bridge**, and it is what makes a gradual path possible:

```go
// InformerFor returns the shared informer for obj from the cache, wrapped together with a typed lister built
// over the informer's indexer.
func InformerFor[L any](ctx context.Context, c cache.Cache, obj client.Object, newLister func(toolscache.Indexer) L) (Informer[L], error) {
	informer, err := c.GetInformer(ctx, obj)
	if err != nil {
		return Informer[L]{}, fmt.Errorf("can't get informer for %T: %w", obj, err)
	}

	// controller-runtime's cache is built on client-go shared index informers and hands out the concrete
	// informer; the typed listers need its indexer.
	sharedIndexInformer, ok := informer.(toolscache.SharedIndexInformer)
	if !ok {
		return Informer[L]{}, fmt.Errorf("informer for %T is %T, not a client-go SharedIndexInformer", obj, informer)
	}

	return Informer[L]{
		informer: sharedIndexInformer,
		lister:   newLister(sharedIndexInformer.GetIndexer()),
	}, nil
}
```

The result satisfies the two-method shape (`Informer()`, `Lister()`) that every generated typed informer has, so the
17 legacy constructors take it without knowing anything changed. Consuming it is one line per kind, 31 times
([`controllers.go:61`](https://github.com/czeslavo/scylla-operator/blob/a60e50ee2/pkg/controllermanager/controllers.go#L61)):

```go
pods := informerFor(f, &corev1.Pod{}, corev1listers.NewPodLister)
services := informerFor(f, &corev1.Service{}, corev1listers.NewServiceLister)
statefulSets := informerFor(f, &appsv1.StatefulSet{}, appsv1listers.NewStatefulSetLister)
```

So there is **one** cache and one watch per kind, not two, which was the main worry about a gradual path. Two things to
know about the bridge. It works only on a cluster-wide cache: as soon as `cache.Options.DefaultNamespaces` is set,
controller-runtime returns its own multi-namespace informer type, which is not a client-go `SharedIndexInformer` and
cannot back a typed lister. The operator binary's cache is cluster-wide, so this is a constraint to remember rather than
a blocker. And the bridge is throwaway by design: it is deleted in milestone 2 once the last legacy controller is gone.

**The client adapters**
([`pkg/ctrlclient`](https://github.com/czeslavo/scylla-operator/blob/f334d7813/pkg/ctrlclient), 240 + 94 + 56 lines).
Rather than rewrite `resourceapply.ApplyService` and friends, the adapters hand them a `client.Client` in the
function-shaped interfaces they already accept, generically over the kind
([`ctrlclient.go:34`](https://github.com/czeslavo/scylla-operator/blob/f334d7813/pkg/ctrlclient/ctrlclient.go#L34)).
This is what keeps the pure functions and their unit tests untouched. At a call site the change is mechanical. Before:

```go
_, changed, err := resourceapply.ApplyService(ctx, sdcc.kubeClient.CoreV1(), sdcc.serviceLister, sdcc.eventRecorder, svc, resourceapply.ApplyOptions{})
```

After:

```go
_, changed, err := resourceapply.ApplyServiceWithControl(ctx, ctrlclient.ApplyControl[corev1.Service](ctx, sdcc.client, sdc.Namespace), sdcc.eventRecorder, svc, resourceapply.ApplyOptions{})
```

The one part that is not generic is the client-backed listers
([`listers.go`](https://github.com/czeslavo/scylla-operator/blob/f334d7813/pkg/ctrlclient/listers.go)), for the pure
functions whose signature takes a `corev1listers.PodLister` or `SecretLister`. Each is about 30 lines; milestone 1 needs
two. The adapters also carry the field-by-field options conversion from 4.5, about 45 lines.

**kubecrypto** (+117 / −37). The certificate manager took a typed client plus a lister per kind; it now takes one
five-method `ObjectControl[T]` per kind
([`certmanager.go:117`](https://github.com/czeslavo/scylla-operator/blob/f334d7813/pkg/kubecrypto/certmanager.go#L117)),
which `ctrlclient.ObjectControl` implements generically in 56 lines. The typed-client implementation stays, so
unmigrated callers are unaffected.

**The controller as a reconciler.** This is where the boilerplate goes. The old constructor took 15 informers and built
13 listers, 13 `cachesToSync` entries and a workqueue
([master `controller.go:106`](https://github.com/scylladb/scylla-operator/blob/5d041a933/pkg/controller/scylladbdatacenter/controller.go#L106));
then, per watched kind, three handlers of this shape, 39 in total:

```go
func (sdcc *Controller) addService(obj interface{}) {
	sdcc.handlers.HandleAdd(
		obj.(*corev1.Service),
		sdcc.handlers.EnqueueOwner,
	)
}

func (sdcc *Controller) updateService(old, cur interface{}) {
	sdcc.handlers.HandleUpdate(
		old.(*corev1.Service),
		cur.(*corev1.Service),
		sdcc.handlers.EnqueueOwner,
		sdcc.deleteService,
	)
}

func (sdcc *Controller) deleteService(obj interface{}) {
	sdcc.handlers.HandleDelete(
		obj,
		sdcc.handlers.EnqueueOwner,
	)
}
```

plus a copy of the queue loop that every legacy controller carries. All of that becomes one declaration
([`controller.go:85`](https://github.com/czeslavo/scylla-operator/blob/f334d7813/pkg/controller/scylladbdatacenter/controller.go#L85)):

```go
func (sdcc *Controller) SetupWithManager(mgr ctrlmanager.Manager, options controller.Options) error {
	cache := mgr.GetCache()

	return ctrlbuilder.ControllerManagedBy(mgr).
		Named(controllerRuntimeName).
		For(&scyllav1alpha1.ScyllaDBDatacenter{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.ServiceAccount{}).
		Owns(&rbacv1.RoleBinding{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&policyv1.PodDisruptionBudget{}).
		Owns(&networkingv1.Ingress{}).
		Owns(&batchv1.Job{}).
		Owns(&scyllav1alpha1.ScyllaDBDatacenterNodesStatusReport{}).
		Watches(&corev1.Secret{}, handler.EnqueueRequestsFromMapFunc(mapSecretToOwnerOrAgentAuthTokenOverrideReferrers(cache))).
		Watches(&corev1.Pod{}, handler.EnqueueRequestsFromMapFunc(mapPodToOwnerThroughStatefulSet(cache))).
		Watches(&scyllav1alpha1.ScyllaOperatorConfig{}, handler.EnqueueRequestsFromMapFunc(mapToAllScyllaDBDatacenters(cache))).
		WithOptions(options).
		Complete(sdcc)
}
```

The three `Watches` need map functions, the one place hand-written enqueue logic survives: the old
`enqueueOwnerThroughStatefulSetOwner` and friends, reshaped to return requests instead of pushing to a queue. The old
`sync` becomes `Reconcile`; the three cache-propagation sleeps are deleted; polling becomes a requeue accumulator. Five
reads stay live against the API server, each guarding an irreversible or expensive step: adoption, the Service before a
PVC deletion, a Pod-readiness re-check, the upgrade ConfigMap, and the StatefulSet partition. The controller file goes
from 734 lines to 242.

**The other 17 controllers keep working** unchanged, taking their informers from the new cache through the bridge.

| milestone 1 change | added | removed | replaces |
|---|---|---|---|
| `pkg/controllermanager/manager.go` | 152 | | four informer factories and their start/sync code in `operator.go` |
| `pkg/controllermanager/informers.go` (the bridge) | 80 | | nothing; it exists only until milestone 2 |
| `pkg/controllermanager/controllers.go` | 546 | | the wiring in `operator.go`, moved, not rewritten |
| `pkg/cmd/operator` | 42 | 587 | the wiring that moved to `controllers.go` |
| `pkg/ctrlclient` (adapters, 2 listers, object control, options) | 390 | | nothing directly; lets `resourceapply` and `controllerhelpers` stay as they are |
| `pkg/kubecrypto` object controls | 117 | 37 | the client-plus-lister pairs the certificate manager took |
| ScyllaDBDatacenter controller (about 125 of the additions are `SetupWithManager` and the 3 map functions) | 287 | 804 | 39 handlers, the queue loop, 15 constructor arguments, 13 `cachesToSync` entries |
| **total** | **1614** | **1428** | |

The shape of that net, +186, is the point: milestone 1 is not "add a consistency layer", it is "replace the
ScyllaDBDatacenter controller's plumbing", which is why the removals nearly cancel the additions.

### 4.3 Milestone 2: the rest of the operator

Fifteen legacy controllers move the same way, one commit each: the nine remaining controllers of the operator binary
and the six controllers of the four side binaries (bootstrap barrier, ignition, sidecar and node setup). The two
multi-DC controllers are removed, not migrated (4.4). At the end `Reconcile` is the only entry point and there is no
error classification of our own (`controllertools.NonRetriable` becomes `reconcile.TerminalError`).

Milestone 2 adds three pieces of shared glue and deletes one.

**The bridge is deleted** (−80), and `controllers.go` drops from 546 to 184 lines: with every controller a reconciler,
registration is a constructor call and a `SetupWithManager` each.

**Reconciler helpers**
([`pkg/controllertools/reconciler.go`](https://github.com/czeslavo/scylla-operator/blob/f334d7813/pkg/controllertools/reconciler.go),
96 lines). Three small things controller-runtime does not have and our controllers need: `EnqueueSingleton`, for
observers that reconcile a singleton rather than an object; `Requeue`, which accumulates the shortest of the delays a
sync used to pass to `queue.AddAfter`; and `Trigger` over a `source.Channel`, with `PeriodicTrigger` as a manager
runnable on top, for anything outside the watches that needs to wake a controller, a timer or a test. The observer
abstraction they replace was 159 lines.

The PoC branch also carries 541 lines of glue that exist only for its multi-DC port, a remote cluster set and eight
remote-kind listers. They go with the controllers they serve and are not counted anywhere below.

**Per controller.** Each gets a `SetupWithManager` like the one in 4.2, between 8 and 45 lines, plus a map function for
every watched kind that is not owned. The side binaries declare what they watch as cache options instead of per-informer
tweaks, for instance the bootstrap barrier
([`controller.go:66`](https://github.com/czeslavo/scylla-operator/blob/f334d7813/pkg/controller/bootstrapbarrier/controller.go#L66)):

```go
func CacheOptions(namespace, serviceName, selectorLabelValue string) cache.Options {
	return cache.Options{
		DefaultNamespaces: map[string]cache.Config{
			namespace: {},
		},
		ByObject: map[client.Object]cache.ByObject{
			&corev1.Service{}: {
				Field: fields.OneTermEqualSelector("metadata.name", serviceName),
			},
			&scyllav1alpha1.ScyllaDBDatacenterNodesStatusReport{}: {
				Label: labels.SelectorFromSet(labels.Set{
					naming.ScyllaDBDatacenterNodesStatusReportSelectorLabel: selectorLabelValue,
				}),
			},
		},
	}
}
```

and the controllers with a custom retry policy or a sync deadline say so in `controller.Options` (`RateLimiter`,
`ReconciliationTimeout`) rather than in a hand-built workqueue, for instance the sidecar
([`controller.go:111`](https://github.com/czeslavo/scylla-operator/blob/f334d7813/pkg/controller/sidecar/controller.go#L111)).

Controller files, before and after, for the ones with the most to lose:

| controller | `controller.go` before | after | map functions |
|---|---|---|---|
| scylladbdatacenter | 734 | 242 | 3 |
| scylladbmonitoring | 673 | 156 | 2 |
| scyllacluster | 631 | 136 | 1 |
| nodetune | 539 | 249 | 0 |
| nodeconfig | 527 | 143 | 1 |
| scylladbmanagerclusterregistration | 440 | 216 | 4 |
| globalscylladbmanager | 385 | 77 | 0 |

Two grow slightly, statusreport (160 to 194) and ignition (248 to 259), because their cache options and trigger wiring
are more lines than the two informers they replaced.

### 4.4 The multi-DC controllers are removed, not migrated

The automated multi-DC controllers, ScyllaDBCluster and RemoteKubernetesCluster, are out of scope for both milestones.
Their CRDs are `v1alpha1`, the user documentation
[says outright](https://github.com/scylladb/scylla-operator/blob/5d041a933/docs/source/understand/index.md#L129) that
there is no plan to make them generally available and that they may be removed, and the documented way to run a
multi-datacenter cluster is the manual one, ScyllaClusters with `externalSeeds`, which does not involve them.

Removing them takes out about 4300 lines of controller code, `pkg/remoteclient` (595 lines), the ScyllaDBCluster,
RemoteKubernetesCluster and RemoteOwner CRDs, and their e2e specs. For the migration it means 16 controllers instead of
18, no per-remote-cluster caches, and 541 fewer lines of glue (4.3).

For the record, the PoC did port them, through one `cluster.Cluster` per RemoteKubernetesCluster with watches attached
as clusters appear, and the multi-DC e2e jobs passed. It was the least comfortable part of the work: controller-runtime
cannot remove a watch once attached, so a cluster replaced while a watch was still syncing keeps its stopped cache
alive; a cluster is reported ready before its cache has synced; the upstream answer, `sigs.k8s.io/multicluster-runtime`,
is itself experimental; and none of it has envtest coverage. The branch keeps that code as evidence that the port is
possible should the product decision change.

### 4.5 Preserving existing behaviour

Moving onto someone else's runtime means its defaults arrive with it, and several are not ours. The PoC set or restored
the following deliberately, so that swapping the binary changes nothing an operator can observe:

- **Leader election stays ours.** The manager's own election is off and `mgr.Start` runs inside the existing
  `leaderelection.Run`, so the lease keeps its name (`scylla-operator-lock`) and identity format, and standby replicas
  still do not start caches
  ([`manager.go:97`](https://github.com/czeslavo/scylla-operator/blob/f334d7813/pkg/controllermanager/manager.go#L97)).
- **No new listeners.** The manager would serve metrics and health probes by default; both are bound to `"0"`, and
  metrics are opt-in behind a new `--metrics-bind-address` flag.
- **One logger.** controller-runtime's cache and reflectors log through a package-level logger, not the manager's.
  Leaving it unset drops their logs and makes long-lived processes print a `log.SetLogger(...) was never called`
  warning with a stack trace. It is set once per process to klog
  ([`manager.go`](https://github.com/czeslavo/scylla-operator/blob/f334d7813/pkg/controllermanager/manager.go)), so all
  existing log flags keep applying.
- **Options survive the translation.** controller-runtime's option structs treat `Raw` as a base only and overwrite its
  fields with their own, zero values included. Passing metav1 options through `Raw` silently dropped the Orphan
  propagation of a StatefulSet recreate and every prune's UID precondition (6.2). Options are now converted field by
  field
  ([`ctrlclient.go:194`](https://github.com/czeslavo/scylla-operator/blob/f334d7813/pkg/ctrlclient/ctrlclient.go#L194)).
- **Log levels.** Waits that our queue loops logged at V(2) surface as reconciler errors once `Reconcile` returns them,
  which made the orphaned-PV controller's PVC waits look like failures. It now returns a requeue instead of an error.
- **Watch filters.** `Owns` and map functions do not carry the filters our hand-written handlers had. The node-config
  Pod controller needed its ScyllaDB-Pod check restored in its Node map function, which the label selector alone did not
  cover.
- **One write opted out of the read-your-writes client**, the Pod eviction (6.2).
- **Live reads stay live.** The five API-server reads listed in 4.2 still go through the API reader.

The first three are one-time decisions in the manager package. The rest are the class of thing to expect per controller:
small differences between a declarative watch and a hand-written handler, each cheap once noticed, and noticed by
comparing behaviour rather than by reading code.

### 4.6 Control over informers

controller-runtime is more opinionated about creating informers than the client-go approach. `For`, `Owns` and
`Watches` in a controller's `SetupWithManager` each declare one, and a `Get` or `List` through the cache-backed client
on a kind that has no informer yet creates one on the fly, scoped by the cache's defaults.

**Filtering.** Every informer tweak on master has a cache option:

| on master | in the PoC |
|---|---|
| `informers.WithNamespace(ns)` on a factory (sidecar, ignition, bootstrap barrier, status report, node setup) | `cache.Options.DefaultNamespaces: {ns: {}}` |
| `WithTweakListOptions` setting `metadata.name=<svc>` (sidecar, bootstrap barrier, ignition, probe server) | `ByObject[&corev1.Service{}].Field: fields.OneTermEqualSelector("metadata.name", svc)` |
| `WithTweakListOptions` setting a label selector (bootstrap barrier's status reports, ignition's ConfigMaps) | `ByObject[...].Label` |
| `WithTweakListOptions` setting `spec.nodeName=<node>` across all namespaces (node setup) | `ByObject[&corev1.Pod{}].Namespaces: {cache.AllNamespaces: {FieldSelector: ...}}` |
| cluster-scoped kinds inside a namespaced factory | served cluster-wide regardless of `DefaultNamespaces` |
| no equivalent | `ByObject[...].Transform`, used to inject informer lag in envtest |

Two examples are in 4.3, for the bootstrap barrier and node tune. Filtering itself is not where controller-runtime is
rigid.

**One informer per kind per cache.** This is the real constraint. A cache holds one informer per kind and
representation, so two different selectors on the same kind in one process cannot both exist. The choice is to widen
the selector and filter client-side, or to run a second cache. The PoC hit it twice:

- Node setup had two Pod factories with disjoint selectors, ScyllaDB Pods on the node across all namespaces
  ([`nodesetupdaemon.go:197`](https://github.com/scylladb/scylla-operator/blob/5d041a933/pkg/cmd/operator/nodesetupdaemon.go#L197)) and the daemon's own Pod by namespace and
  name ([`:203`](https://github.com/scylladb/scylla-operator/blob/5d041a933/pkg/cmd/operator/nodesetupdaemon.go#L203)). They became one informer on `spec.nodeName` alone
  ([`controller.go:122`](https://github.com/czeslavo/scylla-operator/blob/f334d7813/pkg/controller/nodetune/controller.go#L122)) with the ScyllaDB label applied as a
  predicate ([`controller.go:150`](https://github.com/czeslavo/scylla-operator/blob/f334d7813/pkg/controller/nodetune/controller.go#L150)). The daemon now caches every
  Pod on its node rather than two filtered sets. On a ScyllaDB node that is a handful of Pods, but it is a change.
- The operator binary had a dedicated ScyllaOperatorConfig factory filtered to `metadata.name=cluster`
  ([`operator.go:320`](https://github.com/scylladb/scylla-operator/blob/5d041a933/pkg/cmd/operator/operator.go#L320)). It was dropped for the shared unfiltered informer plus
  a name predicate ([`controller.go:75`](https://github.com/czeslavo/scylla-operator/blob/f334d7813/pkg/controller/scyllaoperatorconfig/controller.go#L75)). Harmless,
  the kind is a singleton, but the same shape.

### 4.7 What we own at the end

| shared glue, whole operator | lines |
|---|---|
| `pkg/ctrlclient` (adapters, 2 listers, object control, options) | 390 |
| `pkg/controllermanager` | 326 |
| `pkg/controllertools` reconciler helpers | 96 |
| **total** | **812** |

That is the code that exists only because we run on controller-runtime, against 6809 lines of controller boilerplate
deleted. Of the 812, about 100 is per-kind lister glue that grows by 30 lines for each further kind a lister-taking
function needs; the rest is fixed. Newly upstream's, and no longer ours: the workqueue and its rate limiting, the watch
and event plumbing, and the consistency implementation itself.

**What stays ours on purpose: `pkg/resourceapply`.** The 59 `Apply*` functions and their 67 call sites are untouched by
either milestone; the adapters in `pkg/ctrlclient` exist precisely so that they can be. It is a fair question why a
migration that deletes 6809 lines of boilerplate keeps this layer, since controller-runtime's client has `Apply` and
server-side apply would remove most of it. The answer is that our apply is not a thin wrapper over `Update`. It carries
behaviour that server-side apply does not provide and that the operator relies on today
([`helpers.go:240`](https://github.com/czeslavo/scylla-operator/blob/f334d7813/pkg/resourceapply/helpers.go#L240)):

- a hash of the required object in the `managed-hash` annotation, so an unchanged object costs no write at all
  ([`helpers.go:306`](https://github.com/czeslavo/scylla-operator/blob/f334d7813/pkg/resourceapply/helpers.go#L306));
- an ownership check against the controller reference, with `ForceOwnership` and `AllowMissingControllerRef` as the
  two deliberate exceptions;
- per-kind recreate logic, deleting and re-creating instead of updating when an immutable field changed, which
  server-side apply will not do;
- per-kind preservation of server-assigned fields, such as a Service's `ClusterIP`, which is what the typed wrappers are
  for;
- the create, update and delete events the controllers emit.

Replacing it with server-side apply is a change of write semantics, from hash comparison to field managers, across every
object the operator manages, including objects that already exist in users' clusters with `managed-hash` annotations
and `Update`-style managed fields. That is a behaviour change on live clusters, the one thing both milestones are
designed not to be, and it does not depend on the runtime: the typed clientsets have had `Apply` for years, so it was
equally possible on master and is equally possible after milestone 2. It is a third step, worth its own proposal, and
the migration neither requires it nor makes it harder.

## 5. Comparison

| | A: the client-go layer | B: controller-runtime |
|---|---|---|
| Where the guarantee is enforced | one barrier at the top of each sync, over everything that controller wrote | at each read: a `Get` waits for that one object; a `List` waits for every write of that kind made through the shared client, by any controller in the process, including writes still in flight |
| Granularity, and what it couples | per controller, all kinds, once per sync: a pending write for one ScyllaDBDatacenter delays the sync of every other one, on every worker | per kind, process-wide, per list call: a reconcile that lists Pods waits for Pod writes by any controller; an SDC reconcile makes about a dozen such lists. For completed writes the cost is informer lag, for in-flight writes it is that request's latency |
| Bound on a wait that never completes | 1 minute per sync, then the key requeues | the request context only; `ReconciliationTimeout` has no default and six controllers, including ScyllaDBDatacenter, set none today; a write to an object the cache does not watch is the concrete way such a wait arises (7.2) |
| Per-call opt-out | none; a kind is registered or it is not | yes, on every verb |
| Who owns the concurrency-critical code | us: 1030 lines of watermarks, pending deletes, locking and wake-ups | upstream, marked experimental |
| Code we maintain (gross added / net) | 1151 / **+999** | milestone 1: 1614 / **+186**; whole operator: 3077 / **−3732** |
| What the PoC delivers | the ScyllaDBDatacenter controller | milestone 1: the same one controller; milestone 2: 16 controllers across five binaries |
| Work to give another controller the guarantee | a consistency store, a registration table, client wrapping and a `WaitReady`, plus new recording clients for about 13 kinds and the monitoring client | none: the client every reconciler is handed is already consistent |
| Control over what is watched | full: one factory per need, any selector | declared per controller via `For`/`Owns`/`Watches` and per cache via options, with an equivalent for every tweak we use; one informer per kind per cache, so two selectors on one kind need a second cache (4.6) |
| Effect on the rest of the codebase | none; the hand-written controller machinery stays as it is | workqueues, rate limiting, watch plumbing, metrics and the wiring idiom become upstream's; so do our log-level and error-reporting conventions |
| Boilerplate | stays as it is; the common parts could be extracted by refactoring the controllers ourselves, a significant effort that would reproduce abstractions controller-runtime already provides | 6809 lines of controller boilerplate removed across the operator; the ScyllaDBDatacenter controller's 39 event handlers become one `SetupWithManager` |

### 5.1 How the guarantee is obtained

Both mechanisms are bookkeeping over the same idea, and their internals are near-identical: a per-key minimum resource
version taken from the write's response, a monotonic observed version advanced by informer events, pending deletes
tracked by UID, and a wake-up channel. The difference that matters is not the algorithm but **how the guarantee is
obtained**.

In approach A the guarantee comes from remembering to route a write through a recording client and to register the kind.
Both hold today because the controller has exactly two clients and one registration table. When someone adds a write
later, or reads a kind nobody registered, the object silently loses the guarantee and we are back to the class of bug we
are fixing, with no signal.

In approach B the guarantee is a property of the client the reconciler is handed, so a new write site gets it whether
the author thought about it or not. There is no registration table, but there is a contract of the same shape: the
cache must watch every object the client writes, and in a filtered cache that is decided by the cache options rather
than by a `Register` call. Neither approach checks it (7.1, 7.2).

Neither approach covers a write made through a plain typed clientset, and neither covers objects written by another
controller. That limitation is shared and unavoidable at this layer.

### 5.2 Ownership

Approach A means we own the concurrency-critical code. It is well-tested, its assumptions are documented at the top of
the handler, and it is ours to fix and to extend. It is also 1030 lines of lock-holding, watermark-comparing code in a
repository where nothing else looks like it, and it needs per-kind glue every time it grows.

Approach B means upstream owns it, experimental label and the bug we found (6.2) included. The upside is the one Michal
has raised: the workqueue, the rate limiting, the watch plumbing and the wiring idiom stop being ours, 16 controllers
stop carrying three event-handler functions per watched kind, and a new controller looks like every controller-runtime
controller anyone has ever seen.

## 6. Evidence

Read this section with one asymmetry in mind: because the controller-runtime PoC changes the whole runtime, it was also
put through a log comparison against master (6.3) and a resource measurement (6.4). The client-go PoC was not. Where a
subsection says *not measured*, it means exactly that, and what follows is reasoning, labelled as such.

### 6.1 The stale-cache scenarios in envtest

Both PoCs carry envtest specs that inject informer lag and assert the controller does not act on the pre-write state.
This is the direct evidence that either mechanism fixes OPERATOR-369 and OPERATOR-376.

- client-go PoC:
  [`scylladbdatacenter_controller_cacheconsistency_test.go`](https://github.com/czeslavo/scylla-operator/blob/02390041f/test/envtest/controllers/scylladbdatacenter_controller_cacheconsistency_test.go)
  (191 lines) plus a suite that verifies the client-go layer's own guarantees against a real API server,
  [`test/envtest/cacheconsistency/recordingclient_test.go`](https://github.com/czeslavo/scylla-operator/blob/02390041f/test/envtest/cacheconsistency/recordingclient_test.go)
  (383 lines).
- controller-runtime PoC:
  [`scylladbdatacenter_controller_cacheconsistency_test.go`](https://github.com/czeslavo/scylla-operator/blob/f334d7813/test/envtest/controllers/scylladbdatacenter_controller_cacheconsistency_test.go),
  lag injected through a cache transform
  ([`withInformerLag`](https://github.com/czeslavo/scylla-operator/blob/f334d7813/test/envtest/controllers/scylladbdatacenter_controller_cacheconsistency_test.go#L41)).

Test strategy is not a differentiator: the same scenarios are expressible either way, and the safety net from
[OPERATOR-399](https://scylladb.atlassian.net/browse/OPERATOR-399) applies to both.

### 6.2 What the e2e suites found

Both PoCs were exercised by the full end-to-end (e2e) suites on CI, and both are green in their current shape. What is
worth recording is what the runs surfaced along the way.

The client-go PoC produced no functional failure.

The controller-runtime PoC's runs produced three real problems, all invisible to every other gate we have:

1. **Delete options lost their preconditions.** controller-runtime's option structs overwrite the `Raw` options they
   are given with their own typed fields, zero values included, so a StatefulSet recreate lost its Orphan propagation
   and every prune lost its UID precondition, and the garbage collector deleted the Pods. Fixed by converting options
   field by field
   ([`ctrlclient.go:194`](https://github.com/czeslavo/scylla-operator/blob/f334d7813/pkg/ctrlclient/ctrlclient.go#L194)),
   with a unit test.
2. **Pod eviction through the read-your-writes client failed after the eviction had executed.** The client takes the
   resource version off the object it was handed after a write; an eviction's response is a `Status`, not the Pod, so
   the parse failed and the reconcile returned an error for a write that had succeeded. The retry then evicted the
   replacement Pod, and the node-replace specs hung. Reported upstream as
   [controller-runtime#3590](https://github.com/kubernetes-sigs/controller-runtime/issues/3590) with a minimal
   reproduction; worked around by opting that one call out
   ([`sync_services.go:489`](https://github.com/czeslavo/scylla-operator/blob/f334d7813/pkg/controller/scylladbdatacenter/sync_services.go#L489)),
   the only opt-out in the tree.
3. **Building a cache performs discovery at construction.** In the PoC's multi-DC port, a RemoteKubernetesCluster with
   a broken kubeconfig failed to construct its cache and reported `Degraded` instead of `Available=False`, where the
   legacy typed clients constructed offline. That code goes with multi-DC (4.4), but the property is
   controller-runtime's: a cache built against an API server it cannot reach fails at construction rather than at first
   use.

The point to carry forward is where they were caught: envtest cannot see any of them, because it has no garbage
collector and no spec exercises eviction, and neither can unit tests or lint. A migration of this size leans on e2e
more than our usual changes do, and the areas with the thinnest envtest coverage, ignition and the node daemons, are
exactly where that matters.

A green e2e run is also not a guarantee. The suites assert outcomes, not behaviour along the way: a controller that
re-evicts a Pod on every retry, or one that reconciles ten times where it used to reconcile once, passes as long as the
cluster ends up in the right state within the timeout. The problems above were caught because they happened to make a
spec time out, and the differences in 6.3 were not caught by e2e at all; they were found by reading the logs. Swapping
the runtime under 16 controllers will produce more differences of that kind than any test suite we have is designed to
notice. Risk B.5 proposes how to close that gap.

### 6.3 Logs against master

The operator logs of the controller-runtime PoC's e2e runs were compared line by line, as normalised templates with
rates per covered minute, against the same jobs on a green run of master plus the Kubernetes bump
([PR #3653](https://github.com/scylladb/scylla-operator/pull/3653)) from the same day. Only the last 10 MB of each
leader log survives rotation, so the window is the final 70 to 80 minutes of each run.

Nothing pointed at a correctness regression. Four differences were worth fixing and were fixed: three are the logger,
log-level and watch-filter items in 4.5, and the fourth was a log message naming the wrong kind. Two differences were
left as they are: cheap controllers take 2 to 5 times longer per reconcile (still hundreds of microseconds; heavy
controllers unchanged), and the single-key global-manager controller retries faster during namespace termination, so it
prints roughly twice the error lines for the same underlying condition. Otherwise: no panics, no cache-sync timeouts, no
throttling, no leader-election anomalies, startup to all workers under 0.4 s, and a total error rate slightly *below*
master. Conflict lines dropped from 1003 to 15, which is the read-your-writes client doing its job.

Not measured for the client-go PoC. Reasoning: it does not change the controller machinery, the workqueues or the log
sites outside the ScyllaDBDatacenter controller, so a comparable log delta would be small by construction.

### 6.4 CPU and memory

Measured for the controller-runtime PoC only, on a freshly recreated 3-worker kind cluster: master (`5d041a933`) and the
PoC (`f334d7813`) deployed in turn with identical operator flags, then the same seven kind-fast specs (scaling out and
in, node replace, rolling restart, resource changes, parallel-ops toggle, cleanup after provisioning) at parallelism 2.
All seven passed in both runs, about 25 minutes each. Sampled every 10 s from the kind nodes.

Leader operator container:

| | master | controller-runtime PoC |
|---|---|---|
| CPU mean [cores] | 0.022 | 0.025 |
| CPU p95 [cores] | 0.087 | 0.093 |
| CPU total over the run [cpu-s] | 33.7 | 36.6 |
| memory mean [MiB] | 49.4 | 55.7 |
| memory max [MiB] | 57.7 | 65.2 |
| memory at end [MiB] | 49.4 | 53.9 |

The standby replica sits at about 12 MiB and no measurable CPU in both; the webhook servers are unchanged. One run per
variant on a laptop, so anything under roughly 10 % is inside the noise; the direction, a few MiB and a few percent CPU,
is consistent with per-cache bookkeeping and deep copies on cache reads. Memory returns to its pre-run level in both, so
there is no leak signal.

Not measured for the client-go PoC. Reasoning: it adds no cache and no vendored runtime, so its steady-state footprint
should be indistinguishable from master apart from the consistency store's own bookkeeping.

### 6.5 Code size

Gross additions and net delta both matter: the net is what happens to the repo, the gross is what we have to read and
maintain.

| | added | removed | net |
|---|---|---|---|
| client-go PoC | 1151 | 152 | **+999** |
| controller-runtime, milestone 1 only | 1614 | 1428 | **+186** |
| controller-runtime, whole operator | 3077 | 6809 | **−3732** |

The client-go PoC's additions are `pkg/cacheconsistency` (1030 lines, broken down in 3.1), plus 110 added and 151
removed inside the ScyllaDBDatacenter controller and 11 added in `controllerhelpers`.

The controller-runtime PoC's additions are broken down in 4.2 (milestone 1) and 4.7 (whole operator). Every controller
shrinks; the largest deletions are scylladbmonitoring −878, scylladbdatacenter −831, scyllacluster −662 and nodeconfig
−553. The whole-operator figures exclude the multi-DC controllers and their glue (4.4).

### 6.6 API calls

Counted from kube-apiserver audit logging, one Metadata-level event per request, aggregated per service account and
verb over an e2e run. Both variants ran on one freshly created kind cluster with audit logging enabled, master first and
the PoC second, on the same seven specs as 6.4, and the shared log is sliced by each run's e2e window. Windows differ
in length only because the PoC finished the same specs sooner, so totals, not per-minute rates, are the fair comparison.
Both runs passed 7/7.

**Operator service account.** The totals are the same work:

| | master | PoC |
|---|---|---|
| e2e window [min] | 27.0 | 22.9 |
| requests, total | 2889 | 2847 |
| successful (2xx) | 2412 | 2567 |
| `409 Conflict` | **152** | **3** |
| `404 Not Found` on GET | **88** | **11** |
| `403` create into a terminating namespace | 237 | 266 |
| GET (any) | 178 | 93 |
| WATCH | 113 | 102 |

The successful writes match almost exactly, which is the sanity check that the two operators did the same job: 56 and
56 ConfigMap creates, 42 and 42 Secret creates, 23 and 23 Service creates, 274 and 275 events. Three rows carry the
signal:

- **Conflicts fall from 152 to 3.** On master, 105 of them are `update scylladbdatacenters/status` and 41 are
  `update scyllaclusters/status`, all "the object has been modified": a sync computing status from a cache that has
  not yet seen the previous sync's status write. That is the stale-cache class of bug, counted in API calls, and it is
  the same drop the log comparison in 6.3 saw (1003 conflict log lines to 15). The PoC's three conflicts are on the
  manager cluster registration, written from two sides.
- **Live GETs that return 404 fall from 88 to 11.** On master, 55 are the certificate manager looking up CA and serving
  Secrets before creating them and 17 are the matching ConfigMaps: the "read live when the cache says it is missing"
  pattern, paying an API round trip per certificate object per sync until the object exists. In the PoC those reads go
  through the read-your-writes client, which already knows what it wrote.
- **The `403` creates are teardown noise, present in both.** Every one is "unable to create new content in namespace
  because it is being terminated": the controller keeps re-creating owned objects until the ScyllaDBDatacenter itself is
  gone. Which kinds happen to be mid-creation when the namespace goes differs between runs (jobs 0 and 22, services 45
  and 11), the total does not meaningfully.

**Cluster member service accounts** (sidecar, ignition, bootstrap barrier, status probe): 376 requests on master, 480
on the PoC. The extra 110 are 55 `GET /api` and 55 `GET /apis`, two discovery requests per side-binary process start,
made by controller-runtime's REST mapper when the manager is built. The typed clients on master never did discovery.
Every other row is identical between the variants (110 and 108 Pod patches, 74 and 74 Service watches, 19 and 19 Pod
lists). Two requests per Pod start is a real but small cost; a static REST mapper for the handful of kinds the side
binaries watch would remove it.

## 7. Risks

### 7.1 Approach A

1. **We own a concurrency-critical component with no upstream.** 1030 lines whose correctness rests on
   resource-version monotonicity, informer event ordering and lock discipline. It is tested, including against a real
   API server, but every future change to it is ours to get right.
2. **Correctness depends on discipline, not on the type system.** Every write must go through a recording client and
   every kind read from a cache must be registered; `Register` checks neither. A third condition, that the informer
   watches every object written through the client, in namespace and selector terms, is shared with approach B, where
   the cache options play the role of the registration table (7.2). In A a violation means the sync waits its one
   minute for a resource version the informer will never deliver on its own, then fails and requeues; the store
   watermark also advances on watch bookmarks, so the wait usually ends sooner.
3. **Growth is per kind.** Extending it means new recording clients for about 13 more kinds plus the monitoring client
   (3.4).
4. **It probably closes the controller-runtime door.** We will not get a better opportunity than this one: a
   concrete bug that the migration fixes, a working port of the whole operator to compare against, and a moment before
   2.0 when behaviour changes are acceptable. Choosing the layer means the hand-written machinery and its boilerplate
   stay, and the migration is unlikely to be picked up again on its own merits.

### 7.2 Approach B

1. **The API is experimental, in upstream's own words.** Both the mechanism and its configuration may change. Our blast
   radius is small, two call sites set the flag, but a breaking change upstream lands in a version we also need for
   Kubernetes compatibility.
2. **We have already found one bug in it.** The eviction case (6.2) is fixed locally with the per-call opt-out and
   reported upstream. Nothing points out which other subresource writes are affected; we found this one because e2e
   broke.
3. **Consistent reads have no deadline of their own,** and `ReconciliationTimeout` has no built-in default. Six
   controllers, including ScyllaDBDatacenter, set none today. A read waiting on a cache that never catches up blocks
   that reconcile until the manager stops. The next item is the concrete way such a wait arises. This needs a
   deliberate default before anything ships.
4. **A write to an object the cache does not watch stalls the next read of that kind.** The consistent client records
   the resource version of every write it makes, with no check that the cache's selector or namespace covers the
   object, and the resource version it considers observed advances only on events the informer delivers
   ([`SetMinimumRV`](https://github.com/kubernetes-sigs/controller-runtime/blob/v0.25.0/pkg/client/internal/consistencyhandler/consistencyhandler.go#L109),
   [`OnUpdate`](https://github.com/kubernetes-sigs/controller-runtime/blob/v0.25.0/pkg/client/internal/consistencyhandler/consistencyhandler.go#L296)).
   So a binary on a filtered cache that writes an object outside its filter makes the next `Get` of that key, and the
   next `List` of that kind, wait until some watched object of the same kind happens to change, since resource
   versions are shared across the store, or until the reconciliation's context ends. The operator binary's cache is
   unfiltered, so this cannot happen there. It can in the side binaries, whose caches select Services and Pods by name.
   No such write exists today: the sidecar writes only its own Service and Pod, and the status reporter still patches
   its Pod through the typed client. It is a contract, and nothing enforces it. Mitigation the PoC should adopt: a
   default `ReconciliationTimeout` set in the manager package for every controller, so no wait outlives a reconcile,
   and read-your-writes enabled only where the cache is unfiltered, which is the operator binary; the side binaries
   have no stale-cache bug to fix and keep the plain cache-backed client.
5. **Verification leans on e2e.** All three problems found so far (6.2) were found by the e2e suites and are
   structurally invisible to envtest. Ignition, node setup and node tune have no envtest coverage, so e2e remains the
   only gate for that slice of milestone 2, and a single red run costs a day of
   turnaround. Two mitigations:
   - Each controller's migration is one commit that touches only that controller, which is the natural moment to give
     it the envtest coverage it lacks, the way the ScyllaDBDatacenter controller got its safety net
     ([OPERATOR-399](https://scylladb.atlassian.net/browse/OPERATOR-399)) ahead of its port. Done that way, the coverage
     gap closes as the migration proceeds rather than after it.
   - Before anything ships, a round of manual, agent-assisted testing on live clusters: run the realistic scenarios
     (rollout, scale in both directions, node replace, rolling restart, upgrade, multi-rack, and the side-binary paths
     on kind and on GKE) with the old and the new operator side by side, and have an agent collect and diff what the
     suites do not assert on: operator and sidecar logs normalised against the baseline, reconcile counts and durations
     per controller, API request rates, event streams, and resource usage. That is how 6.3 and 6.4 were produced for
     this document, and both surfaced things the suites had passed over. It costs a few hours per round and turns "e2e
     is green" into "e2e is green and the runtime behaves like the old one where we looked", which is the honest ceiling
     for a change of this shape.
6. **A partial migration can become permanent.** If we take option 1 and milestone 2 slips, the tree keeps two
   controller idioms and a bridge whose only purpose is to be deleted. The bridge is stable enough to sit there
   indefinitely, which is a comfort and a risk in equal measure.

## 8. What each option commits us to

### Option 1: milestone 1 now, milestone 2 later

Ships the manager package, the client adapters and the ScyllaDBDatacenter controller as a reconciler (4.2). The other
17 controllers stay legacy, taking their informers from the controller-runtime cache through the bridge, so there is
one cache and one watch per kind. Until milestone 2 the tree carries two controller idioms, which is a readability cost
while it lasts, and the bridge's cluster-wide-cache constraint has to be remembered.

Leaves open: whether milestone 2 ever happens (risk B.6).

### Option 2: milestone 2 now

Ships everything (4.3): 16 controllers across five binaries, the side binaries on namespaced and field-filtered caches.
The boilerplate is gone now rather than later, the tree has one idiom, and the bridge never needs to exist.

The cost is concentrated in verification (risk B.5): the controllers without envtest coverage are gated by e2e alone.

### Option 3: the client-go layer

Ships `pkg/cacheconsistency` and the ScyllaDBDatacenter controller reading and writing through it (section 3). No new
runtime, no vendored code, nothing changes for the other 17 controllers, and the blast radius is the smallest of the
three by a wide margin: one package plus one controller.

It answers nothing about the boilerplate or the standardisation question (5.2). Those stay open, and the operator keeps
its hand-written controller machinery. If another controller needs the guarantee later, the client-go layer is there,
but wiring it in is per-controller work each time (3.4).

## 9. Decision

The team chose approach B, controller-runtime, in the shape of option 1 with milestone 2 as a stretch goal:

- **Milestone 1 in sprint 12.** The ScyllaDBDatacenter controller moves to controller-runtime's read-your-writes
  client, closing [OPERATOR-369](https://scylladb.atlassian.net/browse/OPERATOR-369) and
  [OPERATOR-376](https://scylladb.atlassian.net/browse/OPERATOR-376) under
  [OPERATOR-392](https://scylladb.atlassian.net/browse/OPERATOR-392); the other controllers stay legacy and share the
  cache through the bridge (4.2).
- **Milestone 2 as a stretch goal in the same sprint, if time allows,** tracked under
  [OPERATOR-414](https://scylladb.atlassian.net/browse/OPERATOR-414), operator observability. The two are linked: the
  controller metrics that epic wants, reconciles per controller, workqueue depth, REST client requests, come from
  controller-runtime, so each controller has to be migrated before it can expose them.
- **The automated multi-DC controllers are removed, not migrated** (4.4).

Section 8 describes what each option commits us to; the risks in 7.2 and the mitigations named there apply.
