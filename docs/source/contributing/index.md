# Contributing to ScyllaDB Operator

ScyllaDB Operator is an open-source project. Contributions of all kinds are welcome — bug reports, documentation improvements, feature requests, and code.

## Getting started

Before contributing, read the [CONTRIBUTING.md](https://github.com/scylladb/scylla-operator/blob/master/CONTRIBUTING.md) file in the repository root.
It covers:

- Setting up the development environment (Go, `kind`, and required tools).
- Building the Operator binary and image.
- Running the end-to-end and unit test suites.
- The contribution workflow (fork → branch → PR).
- Coding conventions and commit message guidelines.

## Quick start

Clone the repository and build the Operator:

```bash
git clone https://github.com/scylladb/scylla-operator.git
cd scylla-operator
make build
```

Run unit tests:

```bash
make test
```

Run the linter:

```bash
make lint
```

Deploy to a local `kind` cluster for development:

```bash
make deploy
```

## Deploying a custom build

To build and push a custom Operator image to your own registry:

```bash
IMAGE=<your-registry>/scylla-operator:<tag> make build-image
IMAGE=<your-registry>/scylla-operator:<tag> make push-image
```

Then update the Operator deployment to use your image:

```bash
kubectl -n scylla-operator set image deployment/scylla-operator \
  scylla-operator=<your-registry>/scylla-operator:<tag>
```

:::{warning}
Custom builds are intended for development and testing only.
ScyllaDB Support does not cover clusters running custom Operator images.
:::

## Reporting issues

Report issues on the [GitHub issue tracker](https://github.com/scylladb/scylla-operator/issues).

Before opening a new issue:
1. Search existing issues to avoid duplicates.
2. Include the Operator version (`kubectl -n scylla-operator get deployment scylla-operator -o jsonpath='{.spec.template.spec.containers[0].image}'`).
3. Attach a [must-gather](../troubleshoot/collect-debugging-information/must-gather.md) archive if the issue is a runtime problem.

## Related pages

- [CONTRIBUTING.md](https://github.com/scylladb/scylla-operator/blob/master/CONTRIBUTING.md) — full contributor guide
- [GitHub repository](https://github.com/scylladb/scylla-operator)
