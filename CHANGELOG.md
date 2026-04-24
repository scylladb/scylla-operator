# Table of Contents

- [1.20.2](#1202)
- [1.20.1](#1201)
- [1.19.2](#1192)
- [Before 2026-03-11](#versions-released-before-2026-03-11)

## Unreleased

### Highlights

### Upgrade requirements
  
Please refer to the [1.20 to 1.21 upgrade guide](https://operator.docs.scylladb.com/stable/management/upgrading/upgrade/#to-1-21).

### Deprecations

- `ScyllaCluster` backup and repair task names not conforming to RFC 1123 subdomain requirements (e.g. containing underscores `_`)
  are now rejected on object creation or update. The operator will refuse to start if any existing `ScyllaClusters` have non-conforming task names.
  [#3326](https://github.com/scylladb/scylla-operator/pull/3326)
- `ScyllaCluster` `spec.exposeOptions.cql` and `ScyllaDBDatacenter` `spec.exposeOptions.cql` are deprecated and will be removed
  in a future release, along with operator support for exposing CQL over an SNI proxy. The admission webhook emits a warning
  when these fields are set.
  [#3410](https://github.com/scylladb/scylla-operator/pull/3410)
- `ScyllaDBMonitoring` `spec.type` value `SaaS` is deprecated and will be removed in a future release. The admission webhook
  emits a warning when `SaaS` is explicitly set. The default value of `spec.type` changed from `SaaS` to `Platform`; existing
  objects that omit `spec.type` will render `Platform` dashboards after the upgrade.
  [#3410](https://github.com/scylladb/scylla-operator/pull/3410)

### Features & Enhancements

- Prometheus Operator is now an optional dependency. Setups without Prometheus Operator CRDs (`monitoring.coreos.com/v1`) are
  fully supported. The Operator detects these CRDs at startup and enables the `ScyllaDBMonitoring` controller only when they are present.
  If the CRDs are not found, an informational log message is printed.
  Refer to the [monitoring setup guide](https://operator.docs.scylladb.com/stable/management/monitoring/setup.html#requirements) for detailed instructions.
  [#3386](https://github.com/scylladb/scylla-operator/pull/3386)

### Bug fixes

- Fixed [#2990](https://github.com/scylladb/scylla-operator/issues/2990): set monitoring scrape intervals to: 5s in
  `ServiceMonitor`, and 30s in Grafana. This overrides the global scrape interval set in Prometheus.
  [#3293](https://github.com/scylladb/scylla-operator/pull/3293)
- Fixed [#2778](https://github.com/scylladb/scylla-operator/issues/2778): `ScyllaDBDatacenter` controller now preserves
  `volumeClaimTemplates` labels and annotations from the existing `StatefulSet` instead of recomputing them, preventing
  immutable field update errors when `.spec.rackTemplate` is set on an existing `ScyllaDBDatacenter`.
  [#3309](https://github.com/scylladb/scylla-operator/pull/3309)
- Fixed [#3007](https://github.com/scylladb/scylla-operator/issues/3007): `ScyllaDBMonitoring` controller now properly 
  sets the aggregated `Available` and `Progressing` status conditions by inspecting state of the underlying Grafana `Deployment` and `Prometheus` CR.
  [#3347](https://github.com/scylladb/scylla-operator/pull/3347)
- Grafana `Deployment`'s volume name changed to the sanitized dashboard name. This prevents volume name rejections when the `ScyllaDBMonitoring` name is too long (> 19 characters).
  [#3363](https://github.com/scylladb/scylla-operator/pull/3363)
- `must-gather` resource collection now tolerates partial API discovery failures (e.g., when aggregated API servers like `metrics.k8s.io` are transiently unavailable) instead of failing entirely.
  [#3396](https://github.com/scylladb/scylla-operator/pull/3396)
- Fixed [#3302](https://github.com/scylladb/scylla-operator/issues/3302): `ScyllaCluster` `spec.version` is now a required field. Previously, an empty value was accepted but caused a silent failure in the migration controller.
  [#3385](https://github.com/scylladb/scylla-operator/pull/3385)
- Fixed [#3407](https://github.com/scylladb/scylla-operator/issues/3407): `ScyllaDBDatacenter` identity service selector now includes the `scylla-operator.scylladb.com/pod-type: scylladb-node` label, preventing cleanup job pods from being matched by the service and causing client connection failures.
  [#3409](https://github.com/scylladb/scylla-operator/pull/3409)

### Dependencies

## [1.20.2](https://github.com/scylladb/scylla-operator/releases/tag/v1.20.2)

Release date: 2026-03-25

### Highlights

- Updated default ScyllaDB version to `2026.1.0` and ScyllaDB Manager to `3.9.0`.
- 🐛 `Pod` annotation "internal.scylla.scylladb.com/scylladb-node-status-report" and `ScyllaDBDatacenterNodesStatusReport` objects now use stable ordering of entries,
  preventing random reordering and frequent updates resulting in unstable `ScyllaCluster`/`ScyllaDBDatacenter` status conditions.

### Bug Fixes

- Fixed [#3337](https://github.com/scylladb/scylla-operator/issues/3337):
  `Pod` annotation "internal.scylla.scylladb.com/scylladb-node-status-report" and `ScyllaDBDatacenterNodesStatusReport` objects now use stable ordering of entries,
  preventing random reordering and frequent updates resulting in unstable `ScyllaCluster`/`ScyllaDBDatacenter` status conditions.
  [#3359](https://github.com/scylladb/scylla-operator/pull/3359)

### Dependencies

- Updated default ScyllaDB version from `2025.4.3` to `2026.1.0` and `scyllaDBUtilsImage` from `docker.io/scylladb/scylla:2025.1.9` to `docker.io/scylladb/scylla:2026.1.0`.
  [#3344](https://github.com/scylladb/scylla-operator/pull/3344)
- Updated default ScyllaDB Manager version from `3.8.0` to `3.9.0`.
  [#3351](https://github.com/scylladb/scylla-operator/pull/3351)
- Minor go module dependencies updates.
  [#3357](https://github.com/scylladb/scylla-operator/pull/3357)

## [1.19.2](https://github.com/scylladb/scylla-operator/releases/tag/v1.19.2)

Release date: 2026-03-19

### Highlights

- 🐛 `Pod` annotation "internal.scylla.scylladb.com/scylladb-node-status-report" and `ScyllaDBDatacenterNodesStatusReport` objects now use stable ordering of entries,
  preventing random reordering and frequent updates resulting in unstable `ScyllaCluster`/`ScyllaDBDatacenter` status conditions.
- 🐛 Fixed `ScyllaCluster` status conditions not properly surfacing errors from child resources (e.g., `ScyllaDBManagerTask` apply failures)
  and misreporting observed generation after certain spec changes (e.g., `.spec.sysctls`), which could make the `Progressing`, `Degraded`, and `Available` conditions unreliable.
- ⚠️ Admission webhook now warns when `ScyllaCluster` backup or repair task names don't comply with RFC 1123 (e.g., containing underscores `_`) - **these will become errors in the next minor release (1.21)**.

### Deprecations

- `ScyllaCluster.spec.backup.tasks` and `ScyllaCluster.spec.repair.tasks` task names not compliant with RFC 1123 subdomain requirements (e.g. containing underscores `_`)
  will be rejected on object creation/update in the next minor release (1.21).

### Features & Enhancements

- Extended the admission webhook to emit warnings when `ScyllaCluster`'s backup or repair task names do not adhere to RFC 1123
  subdomain requirements (e.g. contain underscores `_`). Invalid task names currently cause silent failures where the underlying `ScyllaDBManagerTask` objects fail to be created.
  **In the next minor release (1.21), these warnings will become validation errors that prevent `ScyllaCluster` creation or updates.**
  Users must update their resources to comply with the requirements.
  [#3348](https://github.com/scylladb/scylla-operator/pull/3348)

### Bug Fixes

- Fixed [#3337](https://github.com/scylladb/scylla-operator/issues/3337):
  `Pod` annotation "internal.scylla.scylladb.com/scylladb-node-status-report" and `ScyllaDBDatacenterNodesStatusReport` objects now use stable ordering of entries,
  preventing random reordering and frequent updates resulting in unstable `ScyllaCluster`/`ScyllaDBDatacenter` status conditions.
  [#3360](https://github.com/scylladb/scylla-operator/pull/3360)
- `ScyllaCluster`'s translation controller now combines `ScyllaDBDatacenter` status conditions with its own controller partial conditions when aggregating `ScyllaCluster`'s status conditions,
  and correctly offsets their observed generations by the generation skew between the two resources.
  [#3352](https://github.com/scylladb/scylla-operator/pull/3352)

### Dependencies

- Updated base image from `quay.io/scylladb/scylla-operator-images:base-ubi-9.6-minimal` to `quay.io/scylladb/scylla-operator-images:base-ubi-9.7-minimal`.
  [#3353](https://github.com/scylladb/scylla-operator/pull/3353)
- Bumped builder image from `quay.io/scylladb/scylla-operator-images:golang-1.25` to `quay.io/scylladb/scylla-operator-images:golang-1.26`.
  [#3346](https://github.com/scylladb/scylla-operator/pull/3346)
- Updated `controller-gen` from `v0.18.0` to `v0.20.0`, along with `k8s.io/*` modules from `v0.34.3` to `v0.35.3`.
  [#3358](https://github.com/scylladb/scylla-operator/pull/3358)
- Minor go module dependencies updates. 
  [#3356](https://github.com/scylladb/scylla-operator/pull/3356)

## [1.20.1](https://github.com/scylladb/scylla-operator/releases/tag/v1.20.1)

Release date: 2026-03-11

### Highlights

- 🐛 Fixed `ScyllaCluster` status conditions not properly surfacing errors from child resources (e.g., `ScyllaDBManagerTask` apply failures) 
  and misreporting observed generation after certain spec changes (e.g., `.spec.sysctls`), which could make the `Progressing`, `Degraded`, and `Available` conditions unreliable.
- ⚠️ Admission webhook now warns when `ScyllaCluster` backup or repair task names don't comply with RFC 1123 (e.g., containing underscores `_`) - **these will become errors in the next minor release**.
- 📊 Updated ScyllaDB Monitoring to `4.14.2` with dashboard improvements and Grafana `12.3.3`.

### Deprecations

- `ScyllaCluster.spec.backup.tasks` and `ScyllaCluster.spec.repair.tasks` task names not compliant with RFC 1123 subdomain requirements (e.g. containing underscores `_`)
  will be rejected on object creation/update in the next minor release.

### Features & Enhancements 

- Extended the admission webhook to emit warnings when `ScyllaCluster`'s backup or repair task names do not adhere to RFC 1123
  subdomain requirements (e.g. contain underscores `_`). Invalid task names currently cause silent failures where the underlying `ScyllaDBManagerTask` objects fail to be created.
  **In the next minor release, these warnings will become validation errors that prevent `ScyllaCluster` creation or updates.**
  Users must update their resources to comply with the requirements.
  [#3298](https://github.com/scylladb/scylla-operator/pull/3298)

### Bug Fixes

- `ScyllaCluster`'s translation controller now combines `ScyllaDBDatacenter` status conditions with its own controller partial conditions when aggregating `ScyllaCluster`'s status conditions,
  and correctly offsets their observed generations by the generation skew between the two resources.
  [#3311](https://github.com/scylladb/scylla-operator/pull/3311) [#3321](https://github.com/scylladb/scylla-operator/pull/3321)

### Dependencies

- Updated ScyllaDB Monitoring (`github.com/scylladb/scylla-monitoring` git submodule) from `4.14.0` pre-release (`88dd086`) to `4.14.2`.
  The update includes several Grafana dashboard improvements and bug fixes, as well as a patch update to the Grafana image used (`docker.io/grafana/grafana` from `12.3.2` to `12.3.3`).
  [#3284](https://github.com/scylladb/scylla-operator/pull/3284), [#3314](https://github.com/scylladb/scylla-operator/pull/3314)
- Bumped builder image from `quay.io/scylladb/scylla-operator-images:golang-1.25` to `quay.io/scylladb/scylla-operator-images:golang-1.26`.
  [#3316](https://github.com/scylladb/scylla-operator/pull/3316)
- Minor go module dependencies updates. [#3317](https://github.com/scylladb/scylla-operator/pull/3317)

## Versions released before 2026-03-11

For versions released before 2026-03-11, the changelog information can be found in two places:
- [GitHub Releases](https://github.com/scylladb/scylla-operator/releases) for a list of pull requests grouped by category that went into a release.
- [Release Announcements in the ScyllaDB Forum](https://forum.scylladb.com/tag/operator-release/52) for an understanding-oriented summary of a release.
