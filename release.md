# Release procedures

## Creating a release branch

### Mark `master` as the next release
Moving `master` to the next release is done by creating a new `vX.Y+1.Z-alpha.0` tag for the next version. It can be either just the `git` tag or a proper `alpha` release, but given the very little difference the `beta` of the previous release, it's usability is limited to worth the effort.
```
git fetch git@github.com:scylladb/scylla-operator.git
git tag -s -a vX.Y+1.Z-alpha.0 "${revision}" -m "vX.Y+1.Z-alpha.0"
git push git@github.com:scylladb/scylla-operator.git vX.Y+1.Z-alpha.0
```

### Creating a new release branch
```
git push git@github.com:scylladb/scylla-operator.git vX.Y+1.Z-alpha.0^{}:refs/heads/vX.Y
```

### Updating the release branch
1. On the new release branch, change the default value of the image tag variable in Makefile. Set only major and minor part of version without the 'v' prefix.
   ```
   IMAGE_TAG ?=X.Y
   ```

1. Commit the change.
   ```
   git commit -a -m "Pin tags to X.Y"
   ```

1. Update generated code (manifests, examples, chart defaults, ...).
   ```
   make update
   ```

1. Commit changes and create a PR with `vX.Y` as base branch.
   ```
   git commit -a -m "Updated generated"
   ```


### Enable building docs for the new release branch
1. Enable building docs from `vX.Y` branch by adding an entry to `docs/source/conf.py`.
   ```
   BRANCHES = ['master', 'v1.11', 'v1.12', 'vX.Y']
   ```

1. Set new version as unstable:
   ```
   UNSTABLE_VERSIONS = ['master', 'vX.Y']
   ```

1. Send the PR to `master` branch.

### Finalize 
When the PRs are merged, publish `vX.Y.0-beta.0` pre-release and ask QA to run smoke test using it.

## Creating releases
All the releases and pre-releases are a promotion of an existing image built by our CI.

### Alpha
`alpha` is a testing release that is done on demand, usually from a `master` branch.

It is created as a promotion of an image created by a postsubmit from the associated branch. 

### Beta
`beta` is a testing release that comes from a release branch.

It is created as a promotion of an image created by a postsubmit from the associated branch.

### RC
`rc` (release candidate) is the attempt at GA release.
It is intended to go through extended testing and become the GA image, if successful. 

It is created as a promotion of an image created by a postsubmit from the associated branch.

### GA
GA release is our final release to be published.

It is created as a promotion of an `rc` image tagged earlier, if it passes the tests.

## Promoting releases and pre-releases

### Tagging the image
First, you need to promote an existing image with your new tag. We use both `docker.io` and `quay.io` for resiliency and different features like image scanning. (Image synchronization may get automated in the future.)

#### From `master` branch
```
skopeo copy --all docker://quay.io/scylladb/scylla-operator:latest-YYYY-MM-DD-HHMMSS docker://quay.io/scylladb/scylla-operator:X.Y.0-alpha.I
```
```
skopeo copy --all docker://quay.io/scylladb/scylla-operator:X.Y.0-alpha.I docker://docker.io/scylladb/scylla-operator:X.Y.0-alpha.I
```

#### From release branch
```
skopeo copy --all docker://quay.io/scylladb/scylla-operator:X.Y-YYYY-MM-DD-HHMMSS docker://quay.io/scylladb/scylla-operator:X.Y.Z-beta.I
```
```
skopeo copy --all docker://quay.io/scylladb/scylla-operator:X.Y.Z-beta.I docker://docker.io/scylladb/scylla-operator:X.Y.0-beta.I
```

#### From a pre-release
```
skopeo copy --all docker://quay.io/scylladb/scylla-operator:X.Y.0-rc.I docker://quay.io/scylladb/scylla-operator:X.Y.0
```
```
skopeo copy --all docker://quay.io/scylladb/scylla-operator:X.Y.0 docker://docker.io/scylladb/scylla-operator:X.Y.0
```

### Creating matching `git` tag
When the new image is tagged, you should create a matching `git` tag that this image was built from. (This may get automated in the future.)
```
revision="$( skopeo inspect '--format={{ index .Labels "org.opencontainers.image.revision" }}' docker://docker.io/scylladb/scylla-operator:X.Y.Z-alpha.I )"
```
```
git fetch git@github.com:scylladb/scylla-operator.git
git tag -s -a vX.Y.Z-alpha.I "${revision}" -m "vX.Y.Z-alpha.I"
git push git@github.com:scylladb/scylla-operator.git vX.Y.Z-alpha.I
```
CI will automatically create a new release in GitHub and publish the [release notes](#release-notes) there, and it will automatically publish Helm charts.

## Release announcements
A new release should be published on following channels. This is usually done only for RC and GA releases.
- `#scylla-operator` channel in ScyllaDB-Users Slack
- users mailing list (https://groups.google.com/g/scylladb-users)

## Release notes
1. Release notes are now published by CI for every release and beta+rc prereleases. The release notes contain changes since the last corresponding release in the same category, according to this table  

   | Current        | Previous      |
   | :------------- | :------------ |
   | v1.2.0-beta.0  | v1.1.0        |
   | v1.2.0-beta.1  | v1.2.0-beta.0 |
   | v1.2.0-rc.0    | v1.1.0        |
   | v1.2.0-rc.1    | v1.2.0-rc.0   |
   | v1.1.2         | v1.1.1        |
   | v1.2.0         | v1.1.0        |
