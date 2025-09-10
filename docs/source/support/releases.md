# Releases

## Schedule
We are aiming to ship a new release approximately every 2 months. The following release schedule is only advisory, there are no commitments made to hitting these dates.

:::{table}
| Release | Code freeze | General availability |
|:-------:|:-----------:|:--------------------:|
|  1.19   | 2025-10-27  |      2025-11-05      |
:::

## Supported releases
We support the latest 2 releases of Scylla Operator to give everyone time to upgrade.

:::{table}
| Release | General availability |  Support ends   |
|:-------:|:--------------------:|:---------------:|
|  1.18   |      2025-08-11      | Release of 1.20 |
|  1.17   |      2025-05-12      | Release of 1.19 |
|  1.16   |      2025-03-03      |   2025-08-11    |
|  1.15   |      2024-12-19      |   2025-05-12    |
|  1.14   |      2024-09-19      |   2025-03-03    |
|  1.13   |      2024-06-20      |   2024-12-19    |
|  1.12   |      2024-03-28      |   2024-09-19    |
|  1.11   |      2023-11-09      |   2024-06-20    |
|  1.10   |      2023-08-25      |   2024-03-28    |
|   1.9   |      2023-07-04      |   2023-11-09    |
|   1.8   |      2023-01-25      |   2023-08-25    |
|   1.7   |      2022-01-27      |   2023-07-04    |
|   1.6   |      2021-12-03      |   2023-01-25    |
|   1.5   |      2021-09-16      |   2022-01-27    |
|   1.4   |      2021-08-10      |   2021-12-03    |
|   1.3   |      2021-06-17      |   2021-09-16    |
|   1.2   |      2021-05-06      |   2021-08-10    |
|   1.1   |      2021-03-22      |   2021-06-17    |
|   1.0   |      2021-01-21      |   2021-05-06    |
:::

### Backport policy
Usually, only important bug fixes are eligible for being backported.
This may depend on the situation and assessment of the maintainers.

## CI/CD
We use [Prow](https://prow.scylla-operator.scylladb.com/) for our CI/CD. Before we publish any image, it must pass the E2E suite.

### Automated promotions

:::{table}
| Git reference      | Type   | Container image                                      |
| :----------------: | :----: | :--------------------------------------------------: |
| **master**         | branch | docker.io/scylladb/scylla-operator:**latest**        |
| **vX.Y**           | branch | docker.io/scylladb/scylla-operator:**X.Y**           |
| **vX.Y.Z**         | tag    | docker.io/scylladb/scylla-operator:**X.Y.Z**         |
| **vX.Y.Z-alpha.N** | tag    | docker.io/scylladb/scylla-operator:**X.Y.Z-alpha.N** |
| **vX.Y.Z-beta.N**  | tag    | docker.io/scylladb/scylla-operator:**X.Y.Z-beta.N**  |
| **vX.Y.Z-rc.N**    | tag    | docker.io/scylladb/scylla-operator:**X.Y.Z-rc.N**    |
:::

### Generally available
GA images aren't built from scratch but rather promoted from an existing release candidate. When we decide a release candidate has the acceptable quality and QA signs it off, the release candidate is promoted to become the GA release. This ensures that the GA image has exactly the same content and SHA as the tested release candidate.

## Support matrix

The support matrix table shows version requirements for a particular **Scylla Operator** version. Be sure to match these requirements, otherwise some functionality will not work.

:::{table}
| Component         | master                       | v1.18                        | v1.17                                 |
|:-----------------:|:----------------------------:|:----------------------------:|:-------------------------------------:|
| Kubernetes        | `>=1.30 && <=1.33`           | `>=1.30 && <=1.33`           | `>=1.25 && <=1.31`                    |
| CRI API           | `v1`                         | `v1`                         | `v1`                                  |
| ScyllaDB          | `2024.1 ... 2025.1`          | `2024.1 ... 2025.1`          | `>=6.0`, `2023.1 ... 2025.1`          |
| Scylla Manager    | `>=3.5.0 && <=3.5.1`         | `>=3.5.0 && <=3.5.1`         | `>=3.3.3 && <=3.5`                    |
| Scylla Monitoring | `(CRD)`                      | `(CRD)`                      | `(CRD)`                               |
:::

### Architectures

{{productName}} image is published as a manifest list to `docker.io/scylladb/scylla-operator:X.Y.Z` containing image build for `amd64` and `aarch64`.

### Supported Kubernetes platforms

We officially test and recommend to use the following platforms:

:::{table}
| Platform         | OS Image     |
|:-----------------|:-------------|
| GKE              | Ubuntu       |
| EKS              | Amazon Linux |
:::

While our APIs generally work on any Kubernetes conformant cluster,
performance tuning and other pieces that need to interact with the host OS, kubelet, CRI, kernel, etc. might hit some incompatibilities.


:::{warning}
The following platforms are known **not to work correctly** at this time.

:::{table}
| Platform         | OS Image     | Details |
|:-----------------|:-------------| :------ |
| GKE              | Container OS | Lack of XFS support |
| EKS              | Bottlerocket | Suspected kernel/cgroups issue that breaks available memory detection for ScyllaDB |
:::
:::
