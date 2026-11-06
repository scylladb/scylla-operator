# Releases

## Schedule
The following release schedule is only advisory, there are no commitments made to hitting these dates.

:::{list-table}
:widths: 33 33 34
:header-rows: 1

* - Release
  - Code freeze
  - General availability
* - 1.23
  - TBD
  - TBD
:::

## Supported releases
We support the latest 2 releases of ScyllaDB Operator to give everyone time to upgrade.

:::{list-table}
:widths: 33 33 34
:header-rows: 1

* - Release
  - General availability
  - Support ends
* - 1.22
  - 2026-09-02
  - Release of 1.24
* - 1.21
  - 2026-05-20
  - Release of 1.23
* - 1.20
  - 2026-02-23
  - 2026-09-02
* - 1.19
  - 2025-11-19
  - 2026-05-20
* - 1.18
  - 2025-08-11
  - 2026-02-23
* - 1.17
  - 2025-05-12
  - 2025-11-19
* - 1.16
  - 2025-03-03
  - 2025-08-11
* - 1.15
  - 2024-12-19
  - 2025-05-12
* - 1.14
  - 2024-09-19
  - 2025-03-03
* - 1.13
  - 2024-06-20
  - 2024-12-19
* - 1.12
  - 2024-03-28
  - 2024-09-19
* - 1.11
  - 2023-11-09
  - 2024-06-20
* - 1.10
  - 2023-08-25
  - 2024-03-28
* - 1.9
  - 2023-07-04
  - 2023-11-09
* - 1.8
  - 2023-01-25
  - 2023-08-25
* - 1.7
  - 2022-01-27
  - 2023-07-04
* - 1.6
  - 2021-12-03
  - 2023-01-25
* - 1.5
  - 2021-09-16
  - 2022-01-27
* - 1.4
  - 2021-08-10
  - 2021-12-03
* - 1.3
  - 2021-06-17
  - 2021-09-16
* - 1.2
  - 2021-05-06
  - 2021-08-10
* - 1.1
  - 2021-03-22
  - 2021-06-17
* - 1.0
  - 2021-01-21
  - 2021-05-06
:::

### Backport policy
Usually, only important bug fixes are eligible for being backported.
This may depend on the situation and assessment of the maintainers.

## Container images

Images are published to `docker.io/scylladb/scylla-operator` and `quay.io/scylladb/scylla-operator` with the following tags:

:::{list-table}
:widths: 30 70
:header-rows: 1

* - Tag
  - Points to
* - `X.Y.Z`, `X.Y.Z-alpha.N`, `X.Y.Z-beta.N`, `X.Y.Z-rc.N`
  - The release with that exact version, e.g. `1.23.0` or `1.23.0-rc.0`.
* - `X.Y`
  - The highest GA patch release of the `X.Y` minor.
* - `latest`
  - The highest GA release across all minors.
:::

## Support matrix

The support matrix table shows the version requirements for ScyllaDB Operator. Make sure to match these requirements, otherwise some functionality may not work.

:::{list-table}
:widths: 50 50
:header-rows: 1

* - Component
  - Supported versions
* - Kubernetes
  - {{supportedKubernetesVersionRange}}
* - OpenShift
  - {{supportedOpenShiftVersionRange}}
* - CRI API
  - v1
* - ScyllaDB
  - 2025.1, 2026.1 - 2026.3
* - ScyllaDB Manager
  - 3.10 - 3.12
* - ScyllaDB Monitoring
  - {{scyllaDBMonitoringVersion}} [^scylladb-monitoring-version]
:::

[^scylladb-monitoring-version]: ScyllaDB Operator embeds the specified version of ScyllaDB Monitoring, which includes a set of Grafana dashboards and Prometheus rules.

### Third-party dependencies

The following table lists the versions of third-party dependencies that ScyllaDB Operator is tested against.

:::{list-table}
:widths: 50 50
:header-rows: 1

* - Component
  - Version
* - cert-manager
  - {{certManagerVersion}}
* - Grafana
  - {{grafanaVersion}}
* - Prometheus
  - {{prometheusVersion}}
* - Prometheus Operator
  - {{prometheusOperatorVersion}}
:::

### Architectures

ScyllaDB Operator image is published as a manifest list containing image builds for `amd64` and `arm64`.

### Supported Kubernetes environments

We officially test and recommend to use the following environments:

:::{list-table}
:widths: 50 50
:header-rows: 1

* - Platform
  - OS Image
* - GKE
  - Ubuntu
* - EKS
  - Amazon Linux
* - OKE
  - Oracle Linux
:::

While our APIs generally work on any Kubernetes conformant cluster,
performance tuning and other pieces that need to interact with the host OS, kubelet, CRI, kernel, etc. might hit some incompatibilities.


:::{warning}
The following environments are known **not to work correctly** at this time.

:::{list-table}
:widths: 20 25 55
:header-rows: 1

* - Platform
  - OS Image
  - Details
* - GKE
  - Container OS
  - Lack of XFS support
* - EKS
  - Bottlerocket
  - Suspected kernel/cgroups issue that breaks available memory detection for ScyllaDB
:::
:::
