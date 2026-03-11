# Table of Contents

- [1.20.1](#1201)
- [1.20.0 and older](#1200-and-older)

## Unreleased

### Highlights

### Upgrade requirements

### Deprecations

### Features & Enhancements

### Bug fixes

- Fixed [#2990](https://github.com/scylladb/scylla-operator/issues/2990): set monitoring scrape intervals to: 5s in
  `ServiceMonitor`, and 30s in Grafana. This overrides the global scrape interval set in Prometheus.
  [#3293](https://github.com/scylladb/scylla-operator/pull/3293)
- Fixed [#2778](https://github.com/scylladb/scylla-operator/issues/2778): `ScyllaDBDatacenter` controller now preserves
  `volumeClaimTemplates` labels and annotations from the existing `StatefulSet` instead of recomputing them, preventing
  immutable field update errors when `.spec.rackTemplate` is set on an existing `ScyllaDBDatacenter`.
  [#3309](https://github.com/scylladb/scylla-operator/pull/3309)

### Dependencies

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

## 1.20.0 and older

For those versions, the changelog information can be found in two places:
- [GitHub Releases](https://github.com/scylladb/scylla-operator/releases) for a list of pull requests grouped by category that went into a release.
- [Release Announcements in the ScyllaDB Forum](https://forum.scylladb.com/tag/operator-release/52) for an understanding-oriented summary of a release.
