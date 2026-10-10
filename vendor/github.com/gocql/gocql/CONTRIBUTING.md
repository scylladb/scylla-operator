# Contributing to the ScyllaDB GoCQL Driver

**TL;DR** - this manifesto sets out the bare minimum requirements for submitting a patch to gocql.

This guide outlines the process of landing patches in gocql and the general approach to maintaining the code base.

## Background

The goal of the gocql project is to provide a stable and robust CQL driver for Go. This is a community driven project that is coordinated by a small team of core developers.

## Minimum Requirement Checklist

The following is a check list of requirements that need to be satisfied in order for us to merge your patch:

* You should raise a pull request to scylladb/gocql on Github
* The pull request has a title that clearly summarizes the purpose of the patch
* The motivation behind the patch is clearly defined in the pull request summary
* You agree that your contribution is donated to the Apache Software Foundation (appropriate copyright is on all new files)
* The patch will merge cleanly
* The test coverage does not fall
* The merge commit passes the regression test suite on GitHub Actions
* `go fmt` has been applied to the submitted code
* A correctly formatted commit message, see below

If there are any requirements that can't be reasonably satisfied, please state this either on the pull request or as part of discussion on the mailing list. Where appropriate, the core team may apply discretion and make an exception to these requirements.

## Commit Message

The commit message format should be:

```
<short description>

<reason why the change is needed>

Patch by <authors>; reviewed by <Reviewers> for #####
```

Short description should:
* Be a short sentence.
* Start with a capital letter.
* Be written in the present tense.
* Summarize what is changed, not why it is changed.

Short description should not:
* End with a period.
* Use the word Fixes . Most commits fix something.

Long description / Reason:
* Should describe why the change is needed. What is fixed by the change? Why it it was broken before? What use case does the new feature solve?
* Consider adding details of other options that you considered when implementing the change and why you made the design decisions you made.

## Beyond The Checklist

In addition to stating the hard requirements, there are a bunch of things that we consider when assessing changes to the library. These soft requirements are helpful pointers of how to get a patch landed quicker and with less fuss.

### General QA Approach

The Scylla project needs to consider the ongoing maintainability of the library at all times. Patches that look like they will introduce maintenance issues for the team will not be accepted.

Your patch will get merged quicker if you have decent test cases that provide test coverage for the new behavior you wish to introduce.

Unit tests are good, integration tests are even better. An example of a unit test is `marshal_test.go` - this tests the serialization code in isolation. `cassandra_test.go` is an integration test suite that is executed against every version of Cassandra that gocql supports as part of the CI process on Travis.

That said, the point of writing tests is to provide a safety net to catch regressions, so there is no need to go overboard with tests. Remember that the more tests you write, the more code we will have to maintain. So there's a balance to strike there.

#### Running Against Native Protocol v5

`TEST_CQL_PROTOCOL` and `TEST_COMPRESSOR` select what the integration targets negotiate; the defaults are `4` and `snappy`. Protocol v5 is never chosen automatically (`discoverProtocol` caps negotiation at v4), so the tests guarded by `session.cfg.ProtoVersion < protoVersion5` -- segmentation, compressed segments, `now_in_seconds`, keyspace override and `METADATA_CHANGED`, ten tests at the time of writing -- only run when you ask for it:

```sh
CASSANDRA_VERSION=5-LATEST TEST_CQL_PROTOCOL=5 TEST_COMPRESSOR=lz4 TEST_INTEGRATION_TIMEOUT=20m make test-integration-cassandra
```

`grep -rn 'ProtoVersion < protoVersion5' *_test.go` lists them, should that set have grown since.
Note that not every test in that list is guarded because v5 is required: `TestDurationType`
carries the guard even though the `duration` type has been served over v4 since Cassandra
3.11 and the driver gates it on nothing.

`TEST_INTEGRATION_TIMEOUT` buys headroom over the 10m default. The suite has been measured anywhere from 280s to 470s depending on machine load -- the protocol version makes little difference -- and overrunning `go test`'s `-timeout` panics with a goroutine dump rather than naming the failing test. Note that it cannot be passed through `TEST_OPTS`: `TEST_OPTS` is interpolated ahead of `-timeout` on the `go test` line, and the later flag wins.

`lz4` is not interchangeable with the default here. Snappy was removed in v5, and `ClusterConfig.Validate` rejects any compressor that does not implement `SegmentCompressor` once `ProtoVersion >= 5`, so `TEST_COMPRESSOR=snappy` fails every session in the suite; `lz4` and `no-compression` are the only two valid settings. CI runs this configuration on the Cassandra `5-LATEST` leg, plus a second `TEST_COMPRESSOR=no-compression` step scoped to `TestLargeSizeQuery` and `TestPrepareExecuteMetadataChangedFlag`, because the compressed and uncompressed segment headers are separate code paths. ScyllaDB builds its protocol extensions on top of v4 and has no v5 to negotiate, so there is no equivalent Scylla lane.

#### Releasing the lz4 Module

The `lz4` sub-module is released with the parent from one workflow dispatch, using the same
version and commit. Go still requires a separate tag with its directory prefix (`lz4/vX.Y.Z`).
The root module's currently pinned LZ4 version is written down in four places, and
`check-lz4-pin` verifies three of them:

- `go.mod` -- `require github.com/scylladb/gocql/lz4 vX.Y.Z`. It is inert locally, because the
  `replace` directive beside it redirects to `./lz4`, but consumers see the require and not
  the replace.
- `README.md` section 5.4 -- the `github.com/scylladb/gocql/lz4 vX.Y.Z` the prose quotes, and
  the `lz4/vX.Y.Z` tag reference below it. The third occurrence, the bare version at the end of
  the section, is deliberately left unchecked: it is indistinguishable from the parent module's
  own version, which sits a few lines above it and can differ from the current LZ4 pin.

That neighbouring version is a release obligation of its own, and it belongs to the **parent**
module rather than to this one: section 5.4 opens by telling consumers to pin
`replace github.com/gocql/gocql => github.com/scylladb/gocql vX.Y.Z`. Nothing checks it --
`check-lz4-pin` anchors both of its greps on the lz4 module path precisely because a bare version
is ambiguous -- so bump it by hand at each **parent** release. Left behind, it hands consumers a
pin older than the section introducing it, which then describes a `go.mod` that release does not
have.

One gap remains: the Build workflow's `paths-ignore` skips `*.md`, so a pull request that
edits **only** README.md never triggers it and `check-lz4-pin` never runs on the change it
guards. Closing it would mean running the full integration matrix on documentation-only pull
requests, which costs more than the gap does; release preparation updates the root replacement
version in README.md and must be reviewed even if that is the only changed file.

Publish both tags first, then bump the pin -- the order matters. A pin that lags its newest tag
is only a warning, because a `require` is a lower bound: it still resolves, and the README's
`replace` selects the version a consumer builds against. The window between publishing
`lz4/vX.Y.Z` and landing the bump is therefore green. The reverse order is not: a pin naming a
version that was never tagged resolves for nobody, and `check-lz4-pin` fails hard on it. That is
also why the check tests whether the tag exists rather than whether the pin sorts newest -- an
untagged version can sit between two tags and still compare as older than the newest one.

Renovate is deliberately disabled for `github.com/scylladb/gocql/lz4` (see `renovate.json`)
because the version has to follow the release tag rather than a bot.

#### Measuring Code Coverage

`make test-unit-coverage` runs the unit suite (root module and the `lz4` submodule) instrumented for coverage. To include the integration suite too, start a cluster and run its coverage target as well, e.g.:

```sh
make scylla-start
make test-integration-scylla-coverage
```

To also measure the `ccm`-tagged tests (`internal/ccm`, exercised by `make ccm-test`), run `make ccm-test-coverage` too -- it targets that package directly rather than going through `test-integration-scylla`, since `internal/ccm`'s tests don't accept the cluster-connection flags (`-distribution`, `-cluster`, ...) that target passes.

All of these accumulate into the same coverage data directory (`.coverage/data` by default, override with `COVERAGE_DIR`), so unit and integration runs combine into one picture. Once you've run whichever combination you want measured, generate the report:

```sh
make coverage-report
```

This prints a per-package percentage and writes `coverage-root.html`/`coverage-lz4.html` (open either in a browser for a line-by-line view) plus the underlying `.out` profiles. `lz4` is a separate Go module from the root one, so its report is generated and rendered separately -- `go tool cover` resolves source files against the module rooted at the current directory, and can't do that for two modules from one profile.

`make clean-coverage` removes the coverage data directory and generated reports.

CI runs the same targets in `.github/workflows/coverage.yml` and uploads the resulting
`coverage-root.out`/`coverage-lz4.out` profiles to [Codecov](https://codecov.io/gh/scylladb/gocql),
one upload per Go module under the `root` and `lz4` flags. Codecov then comments the delta on the
pull request and reports it as a check. Both of its statuses are `informational` in `codecov.yml`,
so a drop annotates the pull request but never blocks merging it -- the integration lanes need a
live ScyllaDB cluster, and a lane that fails to start one moves the number for reasons unrelated to
the diff. The workflow is still skippable: add the `disable-coverage-tests` label to a pull request
and no run, and so no upload, happens. Codecov then has no report for that commit at all -- the
`carryforward` flags in `codecov.yml` do not apply, since carryforward only fills in flags missing
from a commit that uploaded *something* -- and comparisons against it simply walk back to the
nearest ancestor that does have a report.

### Sign Off Procedure

Generally speaking, a pull request can get merged by any one of the project's committers. If your change is minor, chances are that one team member will just go ahead and merge it there and then. As stated earlier, suitable test coverage will increase the likelihood that a single reviewer will assess and merge your change. If your change has no test coverage, or looks like it may have wider implications for the health and stability of the library, the reviewer may elect to refer the change to another team member to achieve consensus before proceeding. Therefore, the tighter and cleaner your patch is, the quicker it will go through the review process.

### Supported Features

gocql is a low level wire driver for Cassandra CQL. By and large, we would like to keep the functional scope of the library as narrow as possible. We think that gocql should be tight and focused, and we will be naturally skeptical of things that could just as easily be implemented in a higher layer. Inevitably you will come across something that could be implemented in a higher layer, save for a minor change to the core API. In this instance, please strike up a conversation in the Cassandra community. Chances are we will understand what you are trying to achieve and will try to accommodate this in a maintainable way.
