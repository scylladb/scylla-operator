# Releases

## Schedule
We are aiming to ship a new release approximately every 6 weeks. The following release schedule is only advisory, there are no commitments made to hitting these dates.

| Release | Code freeze | General availability |
| :-----: | :---------: | :------------------: |
| 1.2     | 2021-04-15  | 2021-05-06           |

## Supported releases
We support the latest 2 releases of the operator to give everyone time to upgrade.

| Release | General availability      | Support ends   |
| :-----: | :-----------------------: | :------------: |
| 1.1     | 2021-03-22                | Release of 1.3 |
| 1.0     | 2021-01-21                | Release of 1.2 |

### Backport policy
Usually, only important bug fixes are eligible for being backported.
This may depend on the situation and assessment of the maintainers.

## CI/CD
We use [GitHub actions](https://github.com/scylladb/scylla-operator/actions/workflows/go.yaml?query=branch%3Amaster+event%3Apush) for our CI/CD. Every merge to a supported branch, or a creation of a tag will automatically trigger a job to build, test and publish the container image and other artifacts like helm charts. Before we publish any image, it must pass the e2e suite.

### Automated promotions

| Git reference   | Type   | Container image                                   |
| :-------------: | :----: | :-----------------------------------------------: |
| **master**      | branch | docker.io/scylladb/scylla-operator:**latest**     |
| **v1.1**        | branch | docker.io/scylladb/scylla-operator:**1.1**        |
| **v1.1.0-rc.5** | tag    | docker.io/scylladb/scylla-operator:**1.1.0-rc.5** |

### Generally available
GA images aren't build from scratch but rather promoted from an existing release candidates. When we decide a release candidate has the acceptable quality and QA sings it off, the release candidate is promoted to become the GA release. This makes sure the image has exactly the same content and SHA as the tested release candidate.
