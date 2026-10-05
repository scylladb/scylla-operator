# Supervisord Removal Readiness

What has to change in scylla-operator to run the ScyllaDB image that drops supervisord, while keeping today's supervisord-based images working.

- Upstream change: [scylladb/scylladb#31914](https://github.com/scylladb/scylladb/pull/31914)
- PoC PR: [scylladb/scylla-operator#3710](https://github.com/scylladb/scylla-operator/pull/3710)
- Image: `docker.io/scylladb/scylladb-ci:2026.4.0-dev-0.20260927.c2cd59098c0f`

## 1. Objective

We assume scylladb/scylladb#31914 ships in **ScyllaDB 2026.4.0**.
The goal is to find what in scylla-operator blocks supporting that image, while ScyllaDB 2026.3 and older images, which still run supervisord, keep working unchanged.
Both kinds of image will exist side by side, including inside one cluster during an upgrade.

In the new image, `/docker-entrypoint.py` sets up the node and then execs `/opt/scylladb/supervisor/scylla-server.sh`, which in turn execs `/usr/bin/scylla`.
supervisord, `supervisorctl`, their config and `scylla-housekeeping` are gone.
The operator's sidecar is still PID 1 in the `scylla` container, so scylla is now its direct child.

Old image (`scylla:2026.3.1`), observed in `/proc`:

```
1   scylla-operator sidecar
└─ 47  python3 /docker-entrypoint.py
   └─ 73  supervisord -c /etc/supervisord.conf
      ├─ 74  /usr/bin/scylla --smp 1 …
      └─ 75  scylla-housekeeping-service.sh
```

New image (scylladb-ci PR build), observed in `/proc`:

```
1   scylla-operator sidecar
└─ 47  /usr/bin/scylla --smp 1 …
       (entrypoint exec'd into scylla)

✕ supervisord, supervisorctl, :9001
✕ scylla-housekeeping
```

How we tested:
the draft PoC PR points `assets/config/config.yaml` at the CI image, so every e2e suite uses it.
We ran the kind and GKE e2e suites on it.
In addition, two 3-node clusters on a local kind cluster, one per image, were put through the same failure scenarios for a side-by-side comparison.

## 2. Scenarios

### CI e2e on the new image

Every suite ran on the same commit.
Kind and GKE share the same failures; GKE adds the iotune spec, which isn't in any kind suite.

Legend: ✕ fails, n/a not in that suite.

| Spec | kind fast | kind topology | GKE parallel | GKE clusterip | GKE serial | Cause |
|---|---|---|---|---|---|---|
| Horizontal scaling in, when the node has been drained in maintenance mode | ✕ | ✕ | ✕ | ✕ | n/a | R1 |
| Listens only on secure ports | ✕ | n/a | ✕ | ✕ | n/a | R4 |
| Should scale-up vertically after SMP change | ✕ | n/a | ✕ | ✕ | n/a | R3 |
| Should skip iotune on restart when io properties are cached | n/a | n/a | ✕ | ✕ | n/a | R3 |

Job logs:

- [kind fast](https://prow.scylla-operator.scylladb.com/view/gs/scylla-operator-prow/pr-logs/pull/scylladb_scylla-operator/3704/pull-scylla-operator-master-e2e-kind-fast/2105622923686449152)
- [kind topology](https://prow.scylla-operator.scylladb.com/view/gs/scylla-operator-prow/pr-logs/pull/scylladb_scylla-operator/3704/pull-scylla-operator-master-e2e-kind-cluster-topology-sequential/2105622923950690304)
- [GKE parallel](https://prow.scylla-operator.scylladb.com/view/gs/scylla-operator-prow/pr-logs/pull/scylladb_scylla-operator/3704/pull-scylla-operator-master-e2e-gke-parallel/2105622922914697216)
- [GKE clusterip](https://prow.scylla-operator.scylladb.com/view/gs/scylla-operator-prow/pr-logs/pull/scylladb_scylla-operator/3704/pull-scylla-operator-master-e2e-gke-parallel-clusterip/2105622923015360512)
- [GKE serial](https://prow.scylla-operator.scylladb.com/view/gs/scylla-operator-prow/pr-logs/pull/scylladb_scylla-operator/3704/pull-scylla-operator-master-e2e-gke-serial/2105622923095052288)

Everything else passed, including graceful termination, rolling restarts, every other scale-in/out variant, TLS certificates, Manager backup and restore, and the update and upgrade specs from 2026.3.0 and 2026.2.6.

### Manual scenarios on kind

Two 3-node developer-mode clusters ran next to each other on kind (Kubernetes 1.36.1): one on the new image and one on `scylla:2026.3.1`.
Each scenario was applied to both clusters at the same moment.
The time shown runs from the fault until the scylla container is ready again.

| Scenario | New image | Old image (supervisord) |
|---|---|---|
| kill -9 scylla | 144 s | 7 s |
| SIGTERM, clean exit | 150 s | 153 s |
| Startup always fails | 463 s | 467 s |
| OOM in container | 13 s | 13 s |
| Graceful pod delete | 16 s | 17 s |

For "Startup always fails" the time is when the container was first restarted.
Scylla never became ready, because its config was broken.
For "Graceful pod delete" the time is how long the deletion took.

| Scenario | What we did | New image | Old image | Verdict |
|---|---|---|---|---|
| Scylla crash | `kill -9` on the scylla PID | The sidecar keeps running and scylla stays dead. The liveness probe restarts the container after 128 s; ready at 144 s. | supervisord restarts scylla inside the container; ready at 7 s. | **Regression** |
| Clean exit | `kill -TERM` on the scylla PID | Liveness restart, ready at 150 s. | supervisord treats exit 0 as expected and doesn't restart; liveness restart, ready at 153 s. | Same |
| Startup crash loop | Rack `scyllaConfig` with `commitlog_directory: /proc/forbidden` | Scylla exits with `Startup failed`. The container stays Running until the startup probe gives up (~7.7 min), then repeats. | supervisord retries a few times and gives up; the same ~7.8 min. | Same |
| OOM | `tail /dev/zero` inside the scylla container (1 Gi limit) | Whole container `OOMKilled` (137); ready at 13 s. | Same. | Same |
| Drained node scale-in | Covered by the e2e spec on every platform | Sidecar error loop: `exec: "supervisorctl": executable file not found in $PATH` | Passes on master. | **Regression** |
| Graceful pod delete | `kubectl delete pod` | preStop drain, then SIGTERM straight to scylla. `shutdown complete`, exit 0; 16 s. | Same, plus supervisord stops housekeeping first; 17 s. | Same |
| Restart after a crash | Startup of the container that liveness restarted | The ignition wait is skipped. The node rejoins as UN with its host ID unchanged. | n/a (no container restart) | OK |
| Update in place | 2026.3.1 → new image on a running cluster, with `kill -9` on a pod still on the old image halfway through | Completed. All UN, host IDs unchanged, upgrade hooks ran. One transient startup failure (`raft_operation_timeout_error … read_barrier timed out`) left scylla down for ~2 min until liveness restarted the container. | The old-image pod recovered in 7 s halfway through the update. | Works |

## 3. Regressions and mitigations

Production code depends on supervisord in exactly one place, `sync.go:51`.
The rest are a gap in how the sidecar watches its child and assumptions inside the e2e tests.

### R1. Scale-in of a drained node never completes (blocks support)

**Where:**
[`pkg/controller/sidecar/sync.go#L48-L56`](https://github.com/scylladb/scylla-operator/blob/5089b69654694985a67991fb30f501fe1c27d53e/pkg/controller/sidecar/sync.go#L48-L56).
When a node about to be decommissioned is in `DRAINED` mode, the sidecar runs `supervisorctl restart scylla` so that the node can be decommissioned.

**Impact:**
A user who drained a node (for example in maintenance mode) and then scaled the rack in gets a scale-in that never finishes.
The sidecar retries forever with `can't restart scylla node: exec: "supervisorctl": executable file not found in $PATH`.

**Mitigation:**

- Restart scylla by sending SIGTERM to the sidecar's child process.
  With R2 fixed, the sidecar then exits and the kubelet restarts the container.
  `ignition.done` survives a container restart, so the restart is quick (see "Restart after a crash" in the scenarios).
- This works the same on both images, so no detection is needed.
  On old images the child is the entrypoint, which forwards SIGTERM to supervisord; supervisord stops scylla and exits, and the entrypoint returns.
  The drained-node spec passes with this on both images (section 7).
- The difference for old images: the container restarts instead of just the scylla process, so the restart count of the leaving node goes up by one.

### R2. Crash recovery takes 2+ minutes and the crash is hidden (regression)

**Where:**
[`pkg/cmd/operator/sidecar/sidecar.go#L298-L331`](https://github.com/scylladb/scylla-operator/blob/5089b69654694985a67991fb30f501fe1c27d53e/pkg/cmd/operator/sidecar/sidecar.go#L298-L331).
The sidecar starts the entrypoint and then blocks only on `ctx.Done()`.
It never notices its child exiting.
With supervisord in between, a crashed scylla came back in seconds.
Now nothing restarts it until [liveness](https://github.com/scylladb/scylla-operator/blob/5089b69654694985a67991fb30f501fe1c27d53e/pkg/controller/scylladbdatacenter/resource.go#L922-L932) fails 12 × 10 s.

**Impact:**
A node is down 144 s instead of 7 s after each crash or transient startup failure.
That happened for real during the update test (see "Update in place" in the scenarios).
The kubelet then records the restart as exit 0 `Completed` (U2), so the pod status never shows that scylla crashed.

**Mitigation:**

- Make the sidecar wait for its child in the background and exit with the child's exit code.
  The kubelet then restarts the container straight away, applies its CrashLoopBackOff backoff to repeated failures, and shows the real exit code in `lastState`.
- On old images the child is the entrypoint, which exits only if supervisord dies, so supervisord keeps restarting scylla as today.
  Old images also gain from it: after a clean scylla exit they currently wait ~2.5 min too.
- Having the sidecar restart scylla itself would rebuild supervisord in Go and keep crashes invisible.
  We don't recommend it.
- Exit codes are only accurate for failures of scylla itself on the new image.
  Both entrypoints swallow exceptions in their own setup and exit 0, and on old images the code is the entrypoint's, not scylla's.

### R3. E2E helper can't find the entrypoint process (test only)

**Where:**
[`GetScyllaDBDockerEntrypointCommand`](https://github.com/scylladb/scylla-operator/blob/5089b69654694985a67991fb30f501fe1c27d53e/test/e2e/utils/helpers.go#L923-L940) runs `pgrep -af docker-entrypoint.py`.
It is used by [`assertPodsSMPEquals`](https://github.com/scylladb/scylla-operator/blob/5089b69654694985a67991fb30f501fe1c27d53e/test/e2e/set/scyllacluster/scyllacluster_scaling.go#L312-L319) and [`IsIOTuneRequested`](https://github.com/scylladb/scylla-operator/blob/5089b69654694985a67991fb30f501fe1c27d53e/test/e2e/set/scyllacluster/scyllacluster_iotune.go#L28-L49), which rely on the entrypoint staying alive for the whole container lifetime.

**Impact:**
The SMP spec fails on kind and GKE, and the iotune spec fails on GKE.
The iotune spec isn't in any kind suite.

**Mitigation:**

- For SMP and developer mode, read scylla's own command line (`pgrep -a -x scylla`).
  `/usr/bin/scylla` runs in both images, launched by the same `scylla-server.sh`, with the same argument format: `--smp 1` rather than the entrypoint's `--smp=1`.
- For iotune, scylla's command line can't tell the two cases apart.
  `--io-setup=0` is consumed by the entrypoint, and `--io-properties-file` is on scylla's command line either way: the operator passes it when iotune is skipped, and iotune adds it through `SEASTAR_IO` when it runs.
  Instead, read `SCYLLA_DOCKER_ARGS` from `/etc/scylla.d/docker.conf`, which both entrypoints write.
  It contains `--io-properties-file` only when the operator passed it, that is when iotune was skipped.
- The iotune check can't run on kind (AIO isn't supported on kind's storage), so it is still to be confirmed on GKE.

### R4. Listen test requires supervisord's port (test only)

**Where:**
[`scyllacluster_listen.go#L142-L147`](https://github.com/scylladb/scylla-operator/blob/5089b69654694985a67991fb30f501fe1c27d53e/test/e2e/set/scyllacluster/scyllacluster_listen.go#L142-L147) lists `127.0.0.1:9001` in a `ConsistOf`.
The entry already carries a FIXME for [#1769](https://github.com/scylladb/scylla-operator/issues/1769).

**Impact:**
The spec fails on kind and GKE; 9001 is the only element missing.
For new images this is good news, because the insecure port is gone.

**Mitigation:**
Require 9001 only when the image ships supervisord (`/usr/bin/supervisord` exists in the scylla container), so the check stays exact for both images.
Close #1769 once the operator's minimum supported ScyllaDB version no longer ships supervisord.

## 4. Unexpected findings

**U1. supervisord never restarted a clean exit either.**
With `autorestart=unexpected`, exit 0 counts as expected, so an old-image node also waits ~2.5 min for liveness after scylla exits cleanly.
Fixing R2 shortens this for both images.

**U2. Liveness restarts look like successful exits.**
On SIGTERM the sidecar stops its child and returns 0.
A liveness kill therefore shows in `lastState` as `exitCode: 0, reason: Completed`, so you can't tell from the pod status that scylla had crashed.
This was observed on the new image, and the old image goes through the same code.

**U3. Crash loops never reach CrashLoopBackOff.**
On both images, a scylla that fails on every start sits in a `Running` container for ~7.5 min until the startup probe (40 × 10 s) gives up, and then the cycle repeats.
Fixing R2 would turn this into a standard CrashLoopBackOff with the real exit code.

**U4. scylla-housekeeping no longer runs.**
scylla-housekeeping was a version check: once at start-up and then daily, it contacted ScyllaDB's servers with a per-node UUID to check whether a newer release was available.
It was removed from all packaging on master by [scylladb/scylladb#31137](https://github.com/scylladb/scylladb/pull/31137) (commit [`2f5d5be9e9b2`](https://github.com/scylladb/scylladb/commit/2f5d5be9e9b2), "remove scylla-housekeeping telemetry/version-check"), separately from #31914, so it ships in 2026.4.0 either way.
It has no operator impact: nothing in the operator references it, and pods simply stop making that outbound call.

## 5. Recommended actions

The order follows the dependencies: the R1 fix relies on the R2 fix, and the CI changes should land before config.yaml moves to 2026.4.0.
Actions 1–3 are implemented in the PoC (section 7), which serves as the reference for the real work.

1. **Make the sidecar exit when its scylla child exits, with the child's exit code.**
   Fixes R2 and also improves U1–U3 for both images.
   Add an e2e spec that kills the sidecar's child and expects the container to restart well within the liveness window, with exit code 137.
2. **Replace the hard dependency on `supervisorctl` in the DRAINED decommission path.**
   Send SIGTERM to the sidecar's child and let step 1 restart the container, on both images.
   The existing drained-node spec covers both image styles once CI runs both.
3. **Adapt the e2e tests to both images.**
   Read SMP and developer mode from the scylla process and the iotune decision from `SCYLLA_DOCKER_ARGS`, and require 127.0.0.1:9001 in the listen test only for images with supervisord.
   Confirm the iotune check on GKE, and keep tracking #1769 until no supported ScyllaDB version ships supervisord.
4. **Keep CI coverage for both image styles.**
   Once the default moves to 2026.4.0, keep at least one full suite (for example kind fast) on a supervisord image through `--scylladb-image-ref`.
   The update and upgrade specs (`updateFrom` / `upgradeFrom`) already exercise old → new transitions.

## 6. Upgrade path

The expected order is: upgrade ScyllaDB Operator to a release with the adaptations first, then upgrade ScyllaDB to 2026.4.0.
This matches the existing [ScyllaDB upgrade guide](https://operator.docs.scylladb.com/stable/upgrade/upgrade-scylladb.html), which requires the target ScyllaDB version to be in the [support matrix](https://operator.docs.scylladb.com/stable/reference/releases.html#support-matrix) of the running Operator.
2026.4.0 should only be listed for Operator releases that include actions 1 and 2.

### Upgrading the Operator

- The new Operator image changes the sidecar injected into every ScyllaDB Pod, so every cluster goes through a rolling restart, one node at a time.
  This is what every Operator upgrade does today; there is no API change and no user action needed.
- While the restart is in progress, Pods that haven't been restarted keep the old sidecar.
  Each sidecar only acts on its own container, so mixing old and new sidecars is safe.
- Clusters on supervisord-based images keep working as before, with two differences:
  - If the entrypoint or supervisord itself dies, the container is restarted straight away instead of waiting for the probes.
    A plain scylla crash is still restarted by supervisord inside the container.
  - Decommissioning a drained node restarts the container instead of only the scylla process, so that node's restart count goes up by one.
- This was verified on kind for clusters on both images (section 7).

### Upgrading ScyllaDB to 2026.4.0

- It's a regular minor version upgrade: users change `spec.version` and the Operator rolls the cluster with its usual upgrade procedure.
  As with any minor upgrade, it has to start from 2026.3.
- During the rollout the cluster runs both image flavours side by side.
  This worked in the "Update in place" scenario: all nodes ended UN with unchanged host IDs.
- Visible changes after the upgrade:
  - The `scylla` container no longer has supervisord or `supervisorctl`, and nothing listens on 127.0.0.1:9001.
    The Operator docs never told users to rely on either.
  - scylla-housekeeping no longer runs (U4).
  - A scylla crash now restarts the container, with scylla's exit code in the Pod status and CrashLoopBackOff for repeated failures.

### Running 2026.4.0 with an older Operator

If ScyllaDB is upgraded before the Operator, or with an Operator release without the adaptations, the cluster runs, but R1 and R2 apply:
scale-in of a drained node never completes, and a crashed scylla stays down until the liveness probe restarts the container.
Upgrading the Operator fixes both, because the rolling restart injects the new sidecar.
A scale-in that is already stuck is expected to complete once the leaving node's Pod runs the new sidecar, but that wasn't tested.

## 7. PoC

The PoC is [scylladb/scylla-operator#3710](https://github.com/scylladb/scylla-operator/pull/3710).
It implements actions 1–3.

Verification on kind:

| Spec | New image | 2026.3.1 |
|---|---|---|
| Horizontal scaling in, when the node has been drained in maintenance mode | pass | pass |
| Horizontal scaling in | pass | pass |
| Should scale-up vertically after SMP change | pass | pass |
| Listens only on secure ports | pass | pass |
| Graceful termination (both specs) | pass | pass |
| New crash-restart spec | pass | pass |

Operator upgrade: clusters on both images were created by the upstream master operator (`scylla-operator:latest`, `84d1d9544`), then the operator was switched to the PoC build.
Every ScyllaDB pod was recreated with the new sidecar, which is the rolling restart master already does when the operator image changes.
Both clusters ended Available with no container restarts.
Killing scylla's process on the upgraded pods then restarted the container within ~4 s, ready again in 7–13 s, recorded as `exitCode: 137, reason: Error`.
Before the PoC, the new image took 144 s and recorded `exitCode: 0, reason: Completed`.

## 8. Outlook: splitting the sidecar from the scylla container

Removing supervisord is the right direction strategically.
Our long-term goal is to move the operator's sidecar out of the `scylla` container, so that the container runs ScyllaDB alone and the kubelet supervises it directly.
That was never possible with supervisord in the image, because supervisord hid crashes from the kubelet and the sidecar needed `supervisorctl` in the same container to restart scylla.
With #31914, scylla can be the container's main process, which is the precondition for the split.
Making the sidecar exit with its child (action 1) already moves that way, because the container's lifecycle then equals scylla's.
The PoC's drained-node restart still relies on the sidecar being scylla's parent, so a split would need another restart mechanism.
The split itself is out of scope here and would only be possible for supervisord-free images.
