# Releases

## Schedule
We are aiming to ship a new release approximately every 6 weeks. The following release schedule is only advisory, there are no commitments made to hitting these dates.

| Release | Code freeze | General availability |
| :-----: | :---------: | :------------------: |
| 1.4     | 2021-07-01  | 2021-07-29           |

## Supported releases
We support the latest 2 releases of the operator to give everyone time to upgrade.

| Release | General availability      | Support ends   |
| :-----: | :-----------------------: | :------------: |
| 1.3     | 2021-06-17                | Release of 1.5 |
| 1.2     | 2021-05-06                | Release of 1.4 |
| 1.1     | 2021-03-22                | 2021-06-17     |
| 1.0     | 2021-01-21                | 2021-05-06     |

### Backport policy
Usually, only important bug fixes are eligible for being backported.
This may depend on the situation and assessment of the maintainers.

## CI/CD
We use [GitHub actions](https://github.com/scylladb/scylla-operator/actions/workflows/go.yaml?query=branch%3Amaster+event%3Apush) for our CI/CD. Every merge to a supported branch, or a creation of a tag will automatically trigger a job to build, test and publish the container image and other artifacts like helm charts. Before we publish any image, it must pass the e2e suite.

### Automated promotions

| Git reference      | Type   | Container image                                      |
| :----------------: | :----: | :--------------------------------------------------: |
| **master**         | branch | docker.io/scylladb/scylla-operator:**latest**        |
| **vX.Y**           | branch | docker.io/scylladb/scylla-operator:**X.Y**           |
| **vX.Y.Z**         | tag    | docker.io/scylladb/scylla-operator:**X.Y.Z**         |
| **vX.Y.Z-alpha.N** | tag    | docker.io/scylladb/scylla-operator:**X.Y.Z-alpha.N** |
| **vX.Y.Z-beta.N**  | tag    | docker.io/scylladb/scylla-operator:**X.Y.Z-beta.N**  |
| **vX.Y.Z-rc.N**    | tag    | docker.io/scylladb/scylla-operator:**X.Y.Z-rc.N**    |

### Generally available
GA images aren't build from scratch but rather promoted from an existing release candidates. When we decide a release candidate has the acceptable quality and QA sings it off, the release candidate is promoted to become the GA release. This makes sure the image has exactly the same content and SHA as the tested release candidate.

## Support matrix

Support matrix table shows the version requirements for a particular **scylla-operator** version. Be sure to match these requirements, otherwise some functionality will not work.

|                    | v1.4        | v1.3        | v1.2        | v1.1        | v1.0       |
| :----------------: | :---------: | :---------: | :---------: | :---------: | :--------: |
| Kubernetes         | `>=1.19.10` | `>=1.19`    | `>=1.19`    | `>=1.11`    | `>=1.11`   |
| Scylla OS          | `>=4.3`     | `>=4.2`     | `>=4.2`     | `>=4.0`     | `>=4.0`    |
| Scylla Enterprise  | `>=2021.1`  | `>=2020.1`  | `>=2020.1`  | `>=2020.1`  | `>=2020.1` |
| Scylla Manager     | `>=2.2`     | `>=2.2`     | `>=2.2`     | `>=2.2`     | `>=2.2`    |
| Scylla Monitoring  | `>=1.0`     | `>=1.0`     | `>=1.0`     | `>=1.0`     | `>=1.0`    |
