# Development setup

This tutorial walks you through setting up a local development environment for the ScyllaDB Operator.
By the end, you will have a working Go toolchain, all required dependencies, and a local Kubernetes cluster ready for end-to-end testing.

:::{tip}
For the full contribution workflow—coding conventions, PR guidelines, and commit message format—see [CONTRIBUTING.md](https://github.com/scylladb/scylla-operator/blob/master/CONTRIBUTING.md).
:::

## Prerequisites

- **Go** — the project currently uses Go **1.25.1** (see `go.mod`).
  Install it from [go.dev](https://go.dev/dl/) or use a version manager such as [goenv](https://github.com/go-nv/goenv).
- **Git** — for cloning and managing the repository.

## Fork and clone the repository

1. [Fork the repository](https://docs.github.com/en/pull-requests/collaborating-with-pull-requests/working-with-forks/fork-a-repo) on GitHub.
2. Clone your fork locally:

   ```bash
   git clone https://github.com/<your-username>/scylla-operator.git
   cd scylla-operator
   ```

3. Add the upstream remote:

   ```bash
   git remote add upstream https://github.com/scylladb/scylla-operator.git
   ```

## Install dependencies

Most dependencies are managed by Go modules.
Some Makefile targets require additional tools to be available on your `PATH`:

:::{list-table}
:header-rows: 1
:widths: 20 50 30

* - Tool
  - Purpose
  - Installation
* - [yq](https://github.com/mikefarah/yq)
  - YAML processing in code generation and Helm pipelines.
  - `go install github.com/mikefarah/yq/v4@latest`
* - [jq](https://github.com/jqlang/jq)
  - JSON processing in build scripts.
  - Package manager (`apt`, `brew`, `dnf`, …)
* - [Helm](https://github.com/helm/helm)
  - Rendering and linting Helm charts.
  - [helm.sh/docs](https://helm.sh/docs/intro/install/)
* - [golangci-lint](https://github.com/golangci/golangci-lint)
  - Linting Go code.
  - [golangci-lint.run](https://golangci-lint.run/welcome/install/)
* - [operator-sdk](https://sdk.operatorframework.io/docs/installation/)
  - Generating OLM bundle manifests.
  - See note below.
* - [Ginkgo](https://onsi.github.io/ginkgo/#getting-started)
  - Running envtest integration tests.
  - `go install github.com/onsi/ginkgo/v2/ginkgo@latest`
* - [Podman](https://podman.io/get-started)
  - Building and running container images.
  - [podman.io/get-started](https://podman.io/get-started)
* - [Kind](https://kind.sigs.k8s.io/docs/user/quick-start/)
  - Running local Kubernetes clusters for E2E tests.
  - `go install sigs.k8s.io/kind@latest`
:::

:::{note}
Until [operator-framework/operator-sdk#6978](https://github.com/operator-framework/operator-sdk/pull/6978) is merged and released, you need to use `operator-sdk` from the PR branch.
The `quay.io/scylladb/scylla-operator-images:golang-1.25` image has it preinstalled.
:::

## Set up rootless Podman for Kind

The project uses [Kind](https://kind.sigs.k8s.io/) with rootless Podman for local E2E testing.
Before creating a cluster, configure your host according to the [Kind rootless Podman guide](https://kind.sigs.k8s.io/docs/user/rootless/).

Key steps (distribution-dependent):

1. Enable cgroup v2 delegation for your user.
2. Configure subordinate UID/GID ranges (`/etc/subuid`, `/etc/subgid`).
3. Verify rootless Podman works:

   ```bash
   podman run --rm hello-world
   ```

## Create a local Kind cluster

A Makefile target sets up a Kind cluster with a local container registry so you can push locally built images:

```bash
make kind-setup
```

This creates a cluster named `so-e2e` with a registry at `localhost:5001`.

To tear it down:

```bash
make kind-teardown
```

## Verify the setup

Run a quick build and unit test cycle to confirm everything works:

```bash
make build
make test-unit
```

If both succeed, your development environment is ready.
See [](building-and-testing.md) for the full set of Makefile targets and how to run E2E tests.
