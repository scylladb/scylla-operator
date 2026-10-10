# Releasing gocql

Releases are published only by the manually dispatched **Release** workflow. Do not create release tags or GitHub Releases by hand. A Go module becomes public when its tag is pushed, so an incorrectly tagged version cannot be unpublished.

See [`ci/release/README.md`](ci/release/README.md) for the release controller's high-level logic, state machine, and trust boundaries.

## One-time repository setup

Create GitHub Actions environment `release`. Configure no required reviewer. Limit deployment branches and tags to `master`. Add:

- Secret `GPG_PRIVATE_KEY`: armored private key matching `ci/release/release-signing-key.asc`
- Secret `GPG_PASSPHRASE`: promoter-key passphrase

The publish job grants its built-in `GITHUB_TOKEN` `contents: write` and `issues: read`. Keep the repository's Actions settings and tag rules compatible with that token; a tag ruleset that blocks GitHub Actions from creating release tags will stop publication.

Committed trusted fingerprint: `DC4D ED58 7433 F319 EEE1 EB74 5BD1 EAD2 57F2 1B89`. Key rotation must update public-key file and fingerprint in reviewed PR before environment secret changes.

## Prepare candidate

1. Complete content-readiness checklist [#1068](https://github.com/scylladb/gocql/issues/1068). Workflow does not replace it.
2. Resolve every open `release-blocker`. Workflow checks before CI and immediately before publication. API/parsing errors stop release.
3. Merge release changes to `master`.
4. Update concrete root replacement in README.md to candidate `v1.x.y`; workflow requires match. The same version is released for root and LZ4.
5. If the tip commit changes `.github/workflows/`, merge a release-preparation commit that changes only non-workflow files before tagging. GitHub rejected the `v1.20.0` tag push when its target was a workflow-changing commit, even with `contents: write`; Java Driver's release tags point to a release commit without workflow changes.
6. Choose target `master` or a full 40-character SHA reachable from `master`. `master` is fetched and resolved once during preflight; every later job uses that immutable SHA. Other branches, abbreviated SHAs, and non-ancestors are rejected.

Release-control code and trusted public-key material come from the workflow revision on `master`, not from the candidate commit. The controller is built once and passed to later jobs as a short-lived workflow artifact. This permits releasing an older reachable commit without trusting or requiring release scripts in that commit.

Version input: bare canonical v1 SemVer, e.g. `1.20.0` or `1.20.0-rc.1`. No leading `v`. v2+, build metadata, leading zeroes, unsafe tag characters rejected. Both modules remain v1 paths without `/v2`; major release needs separate path/workflow change.

Published Go versions and source commits are immutable. Proxies/checksum databases cache tags immediately. Never move, replace, delete published tag. Correct with new version; add `retract` later if needed.

## Validate

Open **Actions → Release → Run workflow**, select `master`, enter:

- `version`: bare candidate for both modules
- `target`: `master` or a full SHA
- `mode`: `validate`

Validation performs target, both module, README, blocker, recovery-state, and full Build gates (amd64, arm64, ScyllaDB, Cassandra). It never enters `release` environment, receives no signing credentials, creates no tag/Release. Run summary shows requested target, resolved SHA, both tags, release types, Latest behavior, and recovery actions. Confirm resolved SHA appears in every checkout.

Mappings:

- `root`: module `github.com/gocql/gocql`, tag/title `v<version>`.
- `lz4`: module `github.com/scylladb/gocql/lz4`, tag `lz4/v<version>`, title `lz4 v<version>`.

Gate test: temporary open `release-blocker` issue must stop validation. Remove label/close issue afterward; never bypass.

## Publish

Dispatch again from `master` with the same version and set `mode: publish`. To reproduce a validated candidate after `master` moves, copy resolved SHA from validation summary into `target`; do not enter `master`. Serialized workflow reruns every check and full Build matrix before entering `release` environment.

Actions run names include mode, shared version, and requested target, making validation and publication runs distinguishable in history.

The publish command uses GitHub's generated notes, which read `.github/release.yml` and omit PRs labeled `omit-from-release-notes`. The merge workflow adds that label when every changed file is documentation, tests, CI, or workflow configuration. Mixed PRs remain in the notes. To recheck an older merged PR, manually run **Label support-only release changes** with its PR number.

Equivalent CLI dispatches reduce form-entry mistakes:

```sh
gh workflow run release.yml --ref master \
  -f version=1.20.0 -f target=master \
  -f mode=validate

# Copy resolved SHA from validation summary.
TARGET_SHA=0123456789abcdef0123456789abcdef01234567
gh workflow run release.yml --ref master \
  -f version=1.20.0 -f target="$TARGET_SHA" \
  -f mode=publish
```

Production uses its built-in `GITHUB_TOKEN`, imports promoter key, checks primary fingerprint, then publishes and verifies LZ4 before root. Both signed annotated tags point to the validated SHA. Each GitHub Release uses generated notes from its module's preceding tag and `--verify-tag`. Stable root releases become Latest. Root prereleases and all LZ4 releases use `latest=false`.

Verify:

```sh
git fetch --tags origin
git cat-file -t v1.20.0
git rev-list -n 1 v1.20.0
git tag --verify v1.20.0
```

Repeat for `lz4/v1.20.0`. Object type must be `tag`; resolved commit must match requested SHA; signature must identify committed fingerprint.

After the root tag exists, add it to `TAGS` in `docs/source/conf.py` and set
`LATEST_VERSION` to it so the docs version selector and `/stable` point at the
new release. Run `make -C docs test` and `make -C docs multiversion` before
publishing the docs update. Do this after publication: the multiversion build
cannot include a tag that does not exist yet.

## Retries and partial publication

Rerun identical inputs after transient failure. Each module independently resumes from its verified state; LZ4 completes before root starts:

- No tag/Release: create signed tag, push, create Release.
- Correct signed tag at exact SHA, no Release: create Release only.
- Correct tag and matching Release: verify and succeed without mutation.

Workflow fails closed for Release without tag, wrong target, lightweight/untrusted/unverified tag, conflicting title/prerelease metadata, a new release with wrong Latest behavior, or Git/GitHub/parsing failure. A historical stable root release remains valid after a newer stable release supersedes it as Latest. Never repair by moving/deleting tag. Investigate; if public state may exist, issue new version.

Release-control jobs time out after 20 minutes, build jobs after 45 minutes, and integration jobs after 120 minutes. A stuck run therefore cannot hold the globally serialized release queue indefinitely.

## LZ4 pin follow-up

The root release keeps its existing, resolvable LZ4 requirement. Its local `replace` makes the Build matrix test the LZ4 source being released, but consumers still resolve the version pinned in the root `go.mod`. Check compatibility with that pinned version before publication. After both releases succeed, open a PR updating root `go.mod` and every LZ4 README version, then run `make fix-go-mod-drift` and `make check`. The new tag must exist before the pin changes; consumers cannot resolve an untagged version. The next root release will carry the updated pin.

## Break glass

Ruleset change/disable is production-impacting break glass, never release shortcut. Require explicit repository-owner approval in public tracking issue recording reason, actor, tag patterns, time window. Preserve repository/organization audit logs. Restore ruleset and rerun both maintainer probes immediately. Document every remote mutation and final tag/Release state. Blocker gates, immutable-tag rule, signature and SHA checks still apply.
