# Release procedure

## Prerequisites
1. Stop merges to `master` branch except for the PR in the next section.

## Creating a new release branch
1. Create a release branch on the upstream.
    ```
    git fetch upstream master
    git push upstream upstream/master:refs/heads/vX.Y
    ```

1. Set branch protection rules on `vX.Y` branch in GitHub repository settings.

## Updating the release branch
1. On the release branch, change the default value of the image tag variable in Makefile. Set only major and minor part of version without the 'v' prefix.
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
   git commit -a -m "Updated generated code"
   ```

1. Once the PR is merged, tag the merge commit as `vX.Y.Z-beta.0` using an **annotated** tag.
   ```
   git fetch upstream vX.Y
   git tag -a vX.Y.Z-beta.0 upstream/vX.Y -m "vX.Y.Z-beta.0"
   git push upstream vX.Y.Z-beta.0
   ```

## Updating `master` branch for the next release
1. Enable building docs from `vX.Y` branch by adding an entry to `docs/source/conf.py`.
   ```
   BRANCHES = ['master', 'v0.3', 'v1.0', 'v1.1', 'vX.Y']
   ```

1. Send the PR to `master` branch.

1. When merged, create an **annotated** tag `vX.Y+1.0-alpha.0` for the next release from the merge commit.
   ```
   git fetch upstream master
   git tag -a vX.Y+1.0-alpha.0 upstream/master -m 'vX.Y+1.0-alpha.0'
   git push upstream vX.Y+1.0-alpha.0
   ```

1. Open `master` branch for merging.

## Publishing a release candidate
1. For `Z=0`, the release candidate should be tagged only when all issues in the GitHub milestone are closed and fixed in the release branch.

1. Tag the HEAD of the release branch using an **annotated** tag.
   ```
   git fetch upstream vX.Y
   git tag -a vX.Y.Z-rc.I upstream/vX.Y -m "vX.Y.Z-rc.I"
   git push upstream vX.Y.Z-rc.I
   ```
   CI will automatically create a new release in GitHub and publish the [release notes](#release-notes) there.

1. Announce the new RC with the link to the GitHub release on:
   - `#scylla-operator` channel in ScyllaDB-Users Slack
   - users mailing list (https://groups.google.com/g/scylladb-users)

## Publishing a final release

1. Tag the final release from the last RC that was approved by QA team using an **annotated** tag:
   ```
   git tag -a vX.Y.Z tags/vX.Y.Z-rc.I^{} -m 'vX.Y.Z'
   git push upstream vX.Y.Z
   ```
   CI will automatically create a new release in GitHub and publish the [release notes](#release-notes) there.

1. Promote the container image from the latest RC approved by QA team.
   - `skopeo`
      ```
      skopeo copy docker://docker.io/scylladb/scylla-operator:X.Y.Z-rc.I docker://docker.io/scylladb/scylla-operator:X.Y.Z
      ```
   
   - `docker`
      ```
      docker pull docker.io/scylladb/scylla-operator:X.Y.Z-rc.I
      docker tag docker.io/scylladb/scylla-operator:X.Y.Z-rc.I docker.io/scylladb/scylla-operator:X.Y.Z
      docker push docker.io/scylladb/scylla-operator:X.Y.Z
      ```

1. Publish the Helm charts.
    ```
    git checkout vX.Y.Z
    gcloud auth login
    make helm-publish HELM_CHANNEL=stable HELM_APP_VERSION=X.Y.Z HELM_CHART_VERSION=vX.Y.Z
    ```

1. Ask QA to smoke test vX.Y.Z helm charts.

1. Mark docs as latest in `docs/source/conf.py` in the master branch:
   ```
   smv_latest_version = 'vX.Y'
   ```
1. Submit a PR using `master` as target branch.

1. (optional) Update the release schedule in `docs/source/release.md`.

1. Submit a PR using `master` as target branch.

1. Wait for QA to give you the green light.

1. Announce the new release with the link to the GitHub release on:
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
