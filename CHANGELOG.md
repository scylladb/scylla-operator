# Table of Contents

- [1.21.1](#1211)
- [1.20.3](#1203)
- [1.21.0](#1210)
- [1.20.2](#1202)
- [1.20.1](#1201)
- [1.19.2](#1192)
- [Before 2026-03-11](#versions-released-before-2026-03-11)

## Unreleased

### Bug fixes

- Fixed `ScyllaDBDatacenter` rollouts getting stuck when a new rack was inserted before existing racks while an existing 
  rack was still progressing. The controller now waits for all existing `StatefulSets` to roll out before a new
  `StatefulSet` is created.
  [#3513](https://github.com/scylladb/scylla-operator/pull/3513)
- Fixed the sidecar returning spurious errors during node decommission. `IsDecommissioning()` in the ScyllaDB API
  wrapper incorrectly compared against `OperationalModeDecommissioned` instead of `OperationalModeDecommissioning`,
  which resulted in an unhandled error being logged.
  [#3538](https://github.com/scylladb/scylla-operator/pull/3538)
- Fixed nodes being skipped by cleanup after the token ring changed. The operator seeded a node's last cleaned up token
  ring hash with whatever ring state its sidecar sampled first, which exempted that node from cleanup of the ring it
  first observed. Which nodes ended up exempt depended on the race between node joins and the time for sidecar to propagate the token ring hash.
  Every node is now cleaned up once the ring changes. Note that this increases the number of cleanup jobs created at once,
  from N-1 to N for an N node cluster.
  [#3574](https://github.com/scylladb/scylla-operator/pull/3574)
- Fixed the `ScyllaDBDatacenter` controller discarding failures to sync `ScyllaDBDatacenterNodesStatusReport` objects.
  The error was dropped instead of being aggregated into the error returned by the reconciliation, so a failed sync was
  reported as successful and the key was forgotten rather than requeued, losing the retry with backoff. The degraded
  condition was reported correctly, so this was not visible in the resource status.
  [#3587](https://github.com/scylladb/scylla-operator/pull/3587)

### Features & Enhancements
- Added an optional `bootstrapPolicy` field to `ScyllaCluster.spec` and `ScyllaDBDatacenter.spec`, accepting `Sequential`
  or `Parallel`. `Parallel` requires ScyllaDB 2026.2 or later, declared with a semver-parseable image tag.
  [#3568](https://github.com/scylladb/scylla-operator/pull/3568)
- The webhook server now serves a mutating admission webhook that applies API defaults to `ScyllaClusters` and
  `ScyllaDBDatacenters` on creation. Matching `MutatingWebhookConfiguration` is added to the operator's manifests.
  [#3579](https://github.com/scylladb/scylla-operator/pull/3579)
- ScyllaDB nodes can now bootstrap in parallel. With `bootstrapPolicy: Parallel`, the nodes of a rack are started
  without each one waiting for the previous ordinal to become ready, and all missing racks are created in a single sync
  rather than one per requeue, which cuts the time to bring up a cluster. Creating them still waits for any in-flight
  scaling, update or upgrade of the existing racks to settle. Newly created clusters use `Parallel` by default, while
  existing ones stay on `Sequential` and can opt in. The policy can be changed on a running cluster without disruptions
  to existing nodes. Parallel bootstrap isn't available for datacenters managed by `ScyllaDBCluster`, which always
  bootstraps sequentially.
  [#3578](https://github.com/scylladb/scylla-operator/pull/3578), [#3588](https://github.com/scylladb/scylla-operator/pull/3588)

### Other changes

- ScyllaDB Operator now runs `scylladb-node-exporter` as a dedicated sidecar container for ScyllaDB clusters running
  version 2026.3 or later, since these versions no longer bundle node-exporter. The node-exporter container image is configurable via
  `ScyllaOperatorConfig.spec.scyllaDBNodeExporterImage`.
  [#3482](https://github.com/scylladb/scylla-operator/pull/3482)
- The bootstrap barrier now only takes into account nodes that have already joined the ScyllaDB cluster and own tokens
  in it. `ScyllaDBDatacenterNodesStatusReport` reflects the same: a node appears in it only once it's a member of the
  ScyllaDB cluster, so the absence of an entry no longer implies the corresponding node is missing or unhealthy.
  Previously every node expected to exist in the Kubernetes state was required to be reported and UP, including nodes
  that hadn't joined yet and couldn't report their status.
  [#3530](https://github.com/scylladb/scylla-operator/pull/3530)
- Restored the multi-datacenter documentation, which was dropped by the docs restructuring in 1.21. The guide for
  deploying a multi-datacenter ScyllaDB cluster, and the guides for preparing interconnected EKS and GKE clusters to
  run it on, are available again under Deploy ScyllaDB and Provision infrastructure respectively.
  [#3564](https://github.com/scylladb/scylla-operator/pull/3564)
- Corrected the API reference for `ScyllaCluster.spec.datacenter.racks[].scyllaAgentConfig`. Its description
  previously referred to a ConfigMap, but the operator has always resolved this field to a Secret, since the
  Scylla Manager Agent config may contain a sensitive auth token. Only the field description and generated
  reference were updated — the expected resource type and runtime behavior are unchanged.
  [#3534](https://github.com/scylladb/scylla-operator/pull/3534)

## [1.21.1](https://github.com/scylladb/scylla-operator/releases/tag/v1.21.1)

Release date: 2026-08-11

### Highlights

- 🐛 Fixed `ScyllaDBDatacenter` rollouts getting stuck when a new rack was inserted before existing racks while an existing rack was still progressing.
- 🐛 Fixed the sidecar returning spurious errors during node decommission.

### Bug Fixes

- Fixed `ScyllaDBDatacenter` rollouts getting stuck when a new rack was inserted before existing racks while an existing
  rack was still progressing. The controller now waits for all existing `StatefulSets` to roll out before a new
  `StatefulSet` is created.
  [#3549](https://github.com/scylladb/scylla-operator/pull/3549)
- Fixed the sidecar returning spurious errors during node decommission. `IsDecommissioning()` in the ScyllaDB API
  wrapper incorrectly compared against `OperationalModeDecommissioned` instead of `OperationalModeDecommissioning`,
  which resulted in an unhandled error being logged.
  [#3548](https://github.com/scylladb/scylla-operator/pull/3548)

### Dependencies

- Updated `k8s.io/*` modules from `v0.36.1` to `v0.36.3`, picking up the latest patch fixes of the Kubernetes 1.36 client
  libraries the Operator uses to talk to the Kubernetes API.
  [#3551](https://github.com/scylladb/scylla-operator/pull/3551)
- Updated `github.com/prometheus-operator/prometheus-operator/pkg/client` from `v0.86.2` to `v0.93.0` and
  `github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring` from `v0.91.0` to `v0.93.0`, realigning the two
  modules on a single Prometheus Operator version. These define and access the `Prometheus` and `ServiceMonitor` resources
  that the `ScyllaDBMonitoring` controller manages.
  [#3566](https://github.com/scylladb/scylla-operator/pull/3566)
- Updated the ScyllaDB Manager client modules (`github.com/scylladb/scylla-manager/v3/*`), used to communicate with
  ScyllaDB Manager, to their `2026-08-06` revision.
  [#3551](https://github.com/scylladb/scylla-operator/pull/3551), [#3566](https://github.com/scylladb/scylla-operator/pull/3566)
- Updated `github.com/prometheus/client_golang` from `v1.23.2` to `v1.24.1`, the library exposing the Operator's own metrics.
  [#3566](https://github.com/scylladb/scylla-operator/pull/3566)
- Updated `github.com/grafana/grafana-openapi-client-go`, used by the `ScyllaDBMonitoring` controller to configure Grafana,
  to its `2026-07-24` revision.
  [#3566](https://github.com/scylladb/scylla-operator/pull/3566)

<details><summary>Other dependencies updates</summary>

- `github.com/aws/aws-sdk-go-v2` from `v1.41.7` to `v1.43.4`, along with the rest of the AWS SDK modules and
  `github.com/aws/smithy-go` from `v1.25.1` to `v1.27.6`.
  [#3551](https://github.com/scylladb/scylla-operator/pull/3551), [#3566](https://github.com/scylladb/scylla-operator/pull/3566)
- `google.golang.org/grpc` from `v1.81.1` to `v1.83.0`.
  [#3566](https://github.com/scylladb/scylla-operator/pull/3566)
- `golang.org/x/sys` from `v0.44.0` to `v0.47.0`.
  [#3551](https://github.com/scylladb/scylla-operator/pull/3551)
- `github.com/go-openapi/runtime` from `v0.31.0` to `v0.33.0` and `github.com/go-openapi/strfmt` from `v0.26.2` to `v0.27.0`.
  [#3551](https://github.com/scylladb/scylla-operator/pull/3551), [#3566](https://github.com/scylladb/scylla-operator/pull/3566)
- `github.com/magiconair/properties` from `v1.8.10` to `v1.18.11`.
  [#3566](https://github.com/scylladb/scylla-operator/pull/3566)
- `github.com/onsi/ginkgo/v2` from `v2.29.0` to `v2.32.0` and `github.com/onsi/gomega` from `v1.41.0` to `v1.42.1`.
  [#3566](https://github.com/scylladb/scylla-operator/pull/3566)
- `go` directive in `go.mod` from `1.26.0` to `1.26.3`.
  [#3551](https://github.com/scylladb/scylla-operator/pull/3551)

</details>

## [1.20.3](https://github.com/scylladb/scylla-operator/releases/tag/v1.20.3)

Release date: 2026-08-11

### Highlights

- 🐛 Fixed the sidecar returning spurious errors during node decommission.
- ⬆️ Updated the Kubernetes client libraries from `v0.35.3` to `v0.36.3` (Kubernetes 1.36).

### Bug Fixes

- Fixed the sidecar returning spurious errors during node decommission. `IsDecommissioning()` in the ScyllaDB API
  wrapper incorrectly compared against `OperationalModeDecommissioned` instead of `OperationalModeDecommissioning`,
  which resulted in an unhandled error being logged.
  [#3547](https://github.com/scylladb/scylla-operator/pull/3547)
- Fixed a garbled generation number in the error reported by the `NodeConfig` controller when a node condition is missing
  for the observed generation. The generation was formatted with `%q` instead of `%d`, rendering it as
  `%!q(int64=...)`.
  [#3550](https://github.com/scylladb/scylla-operator/pull/3550)

### Dependencies

- Updated `k8s.io/*` modules from `v0.35.3` to `v0.36.3`, moving the Kubernetes client libraries the Operator uses to talk
  to the Kubernetes API from Kubernetes 1.35 to 1.36.
  [#3550](https://github.com/scylladb/scylla-operator/pull/3550), [#3565](https://github.com/scylladb/scylla-operator/pull/3565)
- Updated `github.com/prometheus-operator/prometheus-operator/pkg/client` and
  `github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring` from `v0.89.0` to `v0.93.0`. These define and
  access the `Prometheus` and `ServiceMonitor` resources that the `ScyllaDBMonitoring` controller manages.
  [#3565](https://github.com/scylladb/scylla-operator/pull/3565)
- Updated the ScyllaDB Manager client modules (`github.com/scylladb/scylla-manager/v3/*`), used to communicate with
  ScyllaDB Manager, to their `2026-08-06` revision.
  [#3550](https://github.com/scylladb/scylla-operator/pull/3550), [#3565](https://github.com/scylladb/scylla-operator/pull/3565)
- Updated `github.com/prometheus/client_golang` from `v1.23.2` to `v1.24.1`, the library exposing the Operator's own metrics.
  [#3565](https://github.com/scylladb/scylla-operator/pull/3565)
- Updated `github.com/grafana/grafana-openapi-client-go`, used by the `ScyllaDBMonitoring` controller to configure Grafana,
  to its `2026-07-24` revision.
  [#3565](https://github.com/scylladb/scylla-operator/pull/3565)
- Raised the `go` directive in `go.mod` from `1.25.1` to `1.26.3`. Building the Operator from source, or importing its Go
  modules, now requires a Go 1.26 toolchain. The builder image has been on `golang-1.26` since 1.20.1.
  [#3550](https://github.com/scylladb/scylla-operator/pull/3550)

<details><summary>Other dependencies updates</summary>

- `github.com/aws/aws-sdk-go-v2` from `v1.41.4` to `v1.43.4`, along with the rest of the AWS SDK modules and
  `github.com/aws/smithy-go` from `v1.24.2` to `v1.27.6`.
  [#3550](https://github.com/scylladb/scylla-operator/pull/3550), [#3565](https://github.com/scylladb/scylla-operator/pull/3565)
- `google.golang.org/grpc` from `v1.79.3` to `v1.83.0`.
  [#3565](https://github.com/scylladb/scylla-operator/pull/3565)
- `golang.org/x/sys` from `v0.42.0` to `v0.47.0`.
  [#3550](https://github.com/scylladb/scylla-operator/pull/3550)
- `github.com/go-git/go-git/v5` from `v5.17.0` to `v5.19.2`.
  [#3550](https://github.com/scylladb/scylla-operator/pull/3550), [#3565](https://github.com/scylladb/scylla-operator/pull/3565)
- `github.com/go-openapi/runtime` from `v0.29.3` to `v0.33.0` and `github.com/go-openapi/strfmt` from `v0.26.1` to `v0.27.0`.
  [#3550](https://github.com/scylladb/scylla-operator/pull/3550), [#3565](https://github.com/scylladb/scylla-operator/pull/3565)
- `github.com/magiconair/properties` from `v1.8.10` to `v1.18.11`.
  [#3565](https://github.com/scylladb/scylla-operator/pull/3565)
- `go.uber.org/config` from `v1.4.0` to `v1.4.1`.
  [#3550](https://github.com/scylladb/scylla-operator/pull/3550)
- `github.com/onsi/ginkgo/v2` from `v2.28.1` to `v2.32.0` and `github.com/onsi/gomega` from `v1.39.1` to `v1.42.1`.
  [#3550](https://github.com/scylladb/scylla-operator/pull/3550), [#3565](https://github.com/scylladb/scylla-operator/pull/3565)
- `sigs.k8s.io/controller-runtime` from `v0.23.3` to `v0.24.1`.
  [#3565](https://github.com/scylladb/scylla-operator/pull/3565)

</details>

## [1.21.0](https://github.com/scylladb/scylla-operator/releases/tag/v1.21.0)

Release date: 2026-05-20

### Highlights

- Oracle Kubernetes Engine (OKE) is now a supported platform. Refer to the [OKE reference deployment](https://operator.docs.scylladb.com/v1.21/deploy-scylladb/reference-deployments/reference-deployment-oke.html) in documentation.
- Added ECDSA as an alternative to RSA for TLS certificate key generation.
- Prometheus Operator is now an optional dependency; setups without its CRDs are fully supported.
- ⚠️  `ScyllaCluster` backup/repair task names not conforming to RFC 1123 are now rejected (previously warned).
- 🐛 Several bug fixes including identity service selector, monitoring status conditions, and `must-gather` resilience.

### Upgrade requirements
  
Please refer to the [1.20 to 1.21 upgrade guide](https://operator.docs.scylladb.com/v1.21/upgrade/upgrade-operator.html#to-1-21).

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
- `--crypto-key-size` flag is now deprecated and remains accepted as a one-to-one alias for `--crypto-rsa-key-size`. It does not affect ECDSA key size.
  Use `--crypto-rsa-key-size` to configure RSA key size and `--crypto-ecdsa-key-size` to configure ECDSA curve bit-size.
### Features & Enhancements

- Prometheus Operator is now an optional dependency. Setups without Prometheus Operator CRDs (`monitoring.coreos.com/v1`) are
  fully supported. The Operator detects these CRDs at startup and enables the `ScyllaDBMonitoring` controller only when they are present.
  If the CRDs are not found, an informational log message is printed.
  Refer to the [monitoring setup guide](https://operator.docs.scylladb.com/stable/management/monitoring/setup.html#requirements) for detailed instructions.
  [#3386](https://github.com/scylladb/scylla-operator/pull/3386)
- Added ECDSA as an alternative to RSA for TLS certificate key generation. Opt in via `--crypto-key-type=ECDSA` flag and configure curve bit-size with `--crypto-ecdsa-key-size=256|384|521` (default: 384).
  RSA key size is now configured with `--crypto-rsa-key-size` (default: 4096). The previous `--crypto-key-size` flag is deprecated and remains accepted as a one-to-one alias for `--crypto-rsa-key-size`; it does not affect the ECDSA key size.
  RSA remains the default key type.
  [#3401](https://github.com/scylladb/scylla-operator/pull/3401)

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
