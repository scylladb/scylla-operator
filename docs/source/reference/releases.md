# Releases

## Schedule

A new release is shipped approximately every 2 months. The following schedule is advisory — there are no commitments to hitting these dates.

:::{list-table}
:widths: 33 33 34
:header-rows: 1

* - Release
  - Code freeze
  - General availability
* - 1.21
  - 2026-04-13
  - 2026-04-27
:::

## Supported releases

The latest 2 releases are supported, giving everyone time to upgrade.

:::{list-table}
:widths: 33 33 34
:header-rows: 1

* - Release
  - General availability
  - Support ends
* - 1.20
  - 2026-02-23
  - Release of 1.22
* - 1.19
  - 2025-11-19
  - Release of 1.21
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

## CI/CD

[Prow](https://prow.scylla-operator.scylladb.com/) is used for CI/CD. Before any image is published, it must pass the E2E suite.

### Automated promotions

:::{list-table}
:widths: 30 15 55
:header-rows: 1

* - Git reference
  - Type
  - Container image
* - **master**
  - branch
  - docker.io/scylladb/scylla-operator:**latest**
* - **vX.Y**
  - branch
  - docker.io/scylladb/scylla-operator:**X.Y**
* - **vX.Y.Z**
  - tag
  - docker.io/scylladb/scylla-operator:**X.Y.Z**
* - **vX.Y.Z-alpha.N**
  - tag
  - docker.io/scylladb/scylla-operator:**X.Y.Z-alpha.N**
* - **vX.Y.Z-beta.N**
  - tag
  - docker.io/scylladb/scylla-operator:**X.Y.Z-beta.N**
* - **vX.Y.Z-rc.N**
  - tag
  - docker.io/scylladb/scylla-operator:**X.Y.Z-rc.N**
:::

### Generally available

GA images are not built from scratch but promoted from an existing release candidate. When a release candidate reaches the acceptable quality and QA signs it off, it is promoted to become the GA release. This ensures that the GA image has exactly the same content and SHA as the tested release candidate.

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
  - 2024.1, 2025.1, 2025.3 - 2025.4
* - ScyllaDB Manager
  - 3.7 - 3.8
* - ScyllaDB Monitoring
  - {{scyllaDBMonitoringVersion}} [^scylladb-monitoring-version]
:::

[^scylladb-monitoring-version]: ScyllaDB Operator embeds the specified version of ScyllaDB Monitoring, which includes a set of Grafana dashboards and Prometheus rules.

### Architectures

The ScyllaDB Operator image is published as a manifest list to `docker.io/scylladb/scylla-operator:X.Y.Z` containing builds for `amd64` and `aarch64`.

### Supported Kubernetes environments

The following environments are officially tested and recommended:

:::{list-table}
:widths: 50 50
:header-rows: 1

* - Platform
  - OS image
* - GKE
  - Ubuntu
* - EKS
  - Amazon Linux
:::

While the APIs generally work on any Kubernetes-conformant cluster, performance tuning and other pieces that interact with the host OS, kubelet, CRI, or kernel may hit incompatibilities on untested platforms.

:::{warning}
The following environments are known **not to work correctly** at this time.

:::{list-table}
:widths: 20 25 55
:header-rows: 1

* - Platform
  - OS image
  - Details
* - GKE
  - Container OS
  - Lack of XFS support
* - EKS
  - Bottlerocket
  - Suspected kernel/cgroups issue that breaks available memory detection for ScyllaDB
:::
:::
