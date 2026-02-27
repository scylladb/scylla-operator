# Table of Contents

- [1.20.0 and older](#1200-and-older)

## Unreleased

### API Changes

- Extended the admission webhook to emit warnings when ScyllaCluster's backup or repair task names do not adhere to RFC 1123
  subdomain requirements. Invalid task names currently cause silent failures where the underlying ScyllaDBManagerTask objects fail to be created.
  **In the next minor release, these warnings will become validation errors that prevent ScyllaCluster creation or updates.**
  Users must update their resources to comply with the requirements.
  [#3295](https://github.com/scylladb/scylla-operator/pull/3295)

### Dependencies

- Updated ScyllaDB Monitoring (`github.com/scylladb/scylla-monitoring` git submodule) from `4.14.0` (`88dd086`) pre-release version
  to the final `4.14.0` (`9dcb579`). The update includes several Grafana dashboard improvements and bug fixes, as well as
  a patch update to the Grafana image used (`docker.io/grafana/grafana` from `12.3.2` to `12.3.3`).
  [#3282](https://github.com/scylladb/scylla-operator/pull/3282), [#3284](https://github.com/scylladb/scylla-operator/pull/3284)

## 1.20.0 and older

For those versions, the changelog information can be found in two places:
- [GitHub Releases](https://github.com/scylladb/scylla-operator/releases) for a list of pull requests grouped by category that went into a release.
- [Release Announcements in the ScyllaDB Forum](https://forum.scylladb.com/tag/operator-release/52) for an understanding-oriented summary of a release.
