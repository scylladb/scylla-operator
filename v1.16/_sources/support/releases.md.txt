# Releases

## Schedule
We are aiming to ship a new release approximately every 2 months. The following release schedule is only advisory, there are no commitments made to hitting these dates.

:::{table}
| Release | Code freeze | General availability |
|:-------:|:-----------:|:--------------------:|
|  1.16   | 2025-02-24  |      2025-03-03      |
:::

## Supported releases
We support the latest 2 releases of the operator to give everyone time to upgrade.

:::{table}
| Release | General availability |  Support ends   |
|:-------:|:--------------------:|:---------------:|
|  1.15   |      2024-12-19      | Release of 1.17 |
|  1.14   |      2024-09-19      | Release of 1.16 |
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
We use [GitHub actions](https://github.com/scylladb/scylla-operator/actions/workflows/go.yaml?query=branch%3Amaster+event%3Apush) for our CI/CD. Every merge to a supported branch, or a creation of a tag will automatically trigger a job to build, test and publish the container image and other artifacts like helm charts. Before we publish any image, it must pass the e2e suite.

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
GA images aren't build from scratch but rather promoted from an existing release candidates. When we decide a release candidate has the acceptable quality and QA sings it off, the release candidate is promoted to become the GA release. This makes sure the image has exactly the same content and SHA as the tested release candidate.

## Support matrix

Support matrix table shows the version requirements for a particular **scylla-operator** version. Be sure to match these requirements, otherwise some functionality will not work.

:::{table}
| Component         | master                 | v1.15                 | v1.14                   |
|:-----------------:|:----------------------:|:---------------------:|:-----------------------:|
| Kubernetes        | `>=1.21`               | `>=1.21`               | `>=1.21`               |
| CRI API           | `v1`                   | `v1`                   | `v1`                   |
| Scylla OS         | `>=6.0 && <=6.2`       | `>=6.0 && <=6.2`       | `>=5.4 && <=6.1`       |
| Scylla Enterprise | `>=2023.1 && <=2024.2` | `>=2023.1 && <=2024.2` | `>=2023.1 && <=2024.1` |
| Scylla Manager    | `>=3.3.3 && <3.4`      | `>=3.3.3 && <3.4`      | `>=3.3.0 && <3.4`      |
| Scylla Monitoring | `(CRD)`                | `(CRD)`                | `(CRD)`                |
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
| GKE              | Container OS |         |
| EKS              | Bottlerocket | Suspected kernel/cgroups issue that breaks available memory detection for ScyllaDB |
:::
:::
