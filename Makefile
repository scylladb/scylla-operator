all: build

SHELL :=/bin/bash -euEo pipefail

IMAGE_TAG ?= latest
IMAGE_REF ?= docker.io/scylladb/scylla-operator:$(IMAGE_TAG)

CODEGEN_PKG ?=./vendor/k8s.io/code-generator
CODEGEN_HEADER_FILE ?=/dev/null
CODEGEN_APIS_PACKAGE ?=$(GO_PACKAGE)/pkg/api
CODEGEN_GROUPS_VERSIONS ?="scyllaclusters/v1"

GO_REQUIRED_MIN_VERSION ?=1.16

GIT_TAG ?=$(shell git describe --long --tags --abbrev=7 --match 'v[0-9]*' || echo 'v0.0.0-unknown')
GIT_COMMIT ?=$(shell git rev-parse --short "HEAD^{commit}" 2>/dev/null)
GIT_TREE_STATE ?=$(shell ( ( [ ! -d ".git/" ] || git diff --quiet ) && echo 'clean' ) || echo 'dirty')

GO ?=go
GOPATH ?=$(shell $(GO) env GOPATH)
GOOS ?=$(shell $(GO) env GOOS)
GOEXE ?=$(shell $(GO) env GOEXE)
GOFMT ?=gofmt
GOFMT_FLAGS ?=-s -l

GO_VERSION :=$(shell $(GO) version | sed -E -e 's/.*go([0-9]+.[0-9]+.[0-9]+).*/\1/')
GO_PACKAGE ?=$(shell $(GO) list -m -f '{{ .Path }}' || echo 'no_package_detected')
GO_PACKAGES ?=./...

go_packages_dirs :=$(shell $(GO) list -f '{{ .Dir }}' $(GO_PACKAGES) || echo 'no_package_dir_detected')
GO_TEST_PACKAGES ?=$(GO_PACKAGES)
GO_BUILD_PACKAGES ?=./cmd/...
GO_BUILD_PACKAGES_EXPANDED ?=$(shell $(GO) list $(GO_BUILD_PACKAGES))
go_build_binaries =$(notdir $(GO_BUILD_PACKAGES_EXPANDED))
GO_BUILD_FLAGS ?=-trimpath
GO_BUILD_BINDIR ?=
GO_LD_EXTRA_FLAGS ?=
GO_TEST_PACKAGES :=./pkg/... ./cmd/...
GO_TEST_FLAGS ?=-race
GO_TEST_COUNT ?=
GO_TEST_EXTRA_FLAGS ?=
GO_TEST_ARGS ?=
GO_TEST_EXTRA_ARGS ?=
GO_TEST_E2E_EXTRA_ARGS ?=

HELM_CHANNEL ?=latest
HELM_CHARTS ?=scylla-operator scylla-manager scylla
HELM_CHARTS_DIR ?=helm
HELM_LOCAL_REPO ?=$(HELM_CHARTS_DIR)/repo/$(HELM_CHANNEL)
HELM_APP_VERSION ?=$(IMAGE_TAG)
HELM_CHART_VERSION_SUFFIX ?=
HELM_CHART_VERSION ?=$(GIT_TAG)$(HELM_CHART_VERSION_SUFFIX)
HELM_BUCKET ?=gs://scylla-operator-charts/$(HELM_CHANNEL)
HELM_REPOSITORY ?=https://scylla-operator-charts.storage.googleapis.com/$(HELM_CHANNEL)

CONTROLLER_GEN ?=$(GO) run ./vendor/sigs.k8s.io/controller-tools/cmd/controller-gen --

define version-ldflags
-X $(1).versionFromGit="$(GIT_TAG)" \
-X $(1).commitFromGit="$(GIT_COMMIT)" \
-X $(1).gitTreeState="$(GIT_TREE_STATE)" \
-X $(1).buildDate="$(shell date -u +'%Y-%m-%dT%H:%M:%SZ')"
endef
GO_LD_FLAGS ?=-ldflags "$(call version-ldflags,$(GO_PACKAGE)/pkg/version) $(GO_LD_EXTRA_FLAGS)"

# TODO: look into how to make these local to the targets
export DOCKER_BUILDKIT :=1
export GOVERSION :=$(shell go version)
export KUBEBUILDER_ASSETS :=$(GOPATH)/bin
export PATH :=$(GOPATH)/bin:$(PATH):

# $1 - required version
# $2 - current version
define is_equal_or_higher_version
$(strip $(filter $(2),$(firstword $(shell printf '%s\n%s' '$(1)' '$(2)' | sort -V -r -b))))
endef

# $1 - program name
# $2 - required version variable name
# $3 - current version string
define require_minimal_version
$(if $($(2)),\
$(if $(strip $(call is_equal_or_higher_version,$($(2)),$(3))),,$(error `$(1)` is required with minimal version "$($(2))", detected version "$(3)". You can override this check by using `make $(2):=`)),\
)
endef

ifneq "$(GO_REQUIRED_MIN_VERSION)" ""
$(call require_minimal_version,$(GO),GO_REQUIRED_MIN_VERSION,$(GO_VERSION))
endif

# $1 - package name
define build-package
	$(if $(GO_BUILD_BINDIR),mkdir -p '$(GO_BUILD_BINDIR)',)
	$(strip CGO_ENABLED=0 $(GO) build $(GO_BUILD_FLAGS) $(GO_LD_FLAGS) \
		$(if $(GO_BUILD_BINDIR),-o '$(GO_BUILD_BINDIR)/$(notdir $(1))$(GOEXE)',) \
	$(1))

endef

# $1 - chart name
# $2 - destination dir
# $3 - app version
# $4 - chart version
define package-helm
	echo 'Preparing $(1) Helm Chart'
	helm package '$(HELM_CHARTS_DIR)/$(1)' --destination '$(2)' --app-version '$(3)' --version '$(4)'

endef

# $1 - chart name
define lint-helm
	helm lint helm/$(1)

endef

# $1 - manifest
# $2 - output
define append-manifest
	echo -e '\n---' | cat '$(1)' - >> '$(2)'

endef

# $1 - manifest files list
# $2 - output
define concat-manifests
	true > '$(2)'
	$(foreach file,$(1), $(call append-manifest,$(file),$(2)))
endef

# We need to build each package separately so go build creates appropriate binaries
build:
	$(if $(strip $(GO_BUILD_PACKAGES_EXPANDED)),,$(error no packages to build: GO_BUILD_PACKAGES_EXPANDED var is empty))
	$(foreach package,$(GO_BUILD_PACKAGES_EXPANDED),$(call build-package,$(package)))
.PHONY: build

clean:
	$(RM) $(go_build_binaries)
.PHONY: clean

verify-govet:
	$(GO) vet $(GO_PACKAGES)
.PHONY: verify-govet

verify-gofmt:
	$(info Running $(GOFMT) $(GOFMT_FLAGS))
	@output=$$( $(GOFMT) $(GOFMT_FLAGS) $(go_packages_dirs) ); \
	if [ -n "$${output}" ]; then \
		echo "$@ failed - please run \`make update-gofmt\` to fix following files:"; \
		echo "$${output}"; \
		exit 1; \
	fi;
.PHONY: verify-gofmt

update-gofmt:
	$(info Running $(GOFMT) $(GOFMT_FLAGS) -w)
	@$(GOFMT) $(GOFMT_FLAGS) -w $(go_packages_dirs)
.PHONY: update-gofmt

# We need to force localle so different envs sort files the same way for recursive traversals
deps_diff :=LC_COLLATE=C diff --no-dereference -N

# $1 - temporary directory
define restore-deps
	ln -s $(abspath ./) "$(1)"/current
	cp -R -H ./ "$(1)"/updated
	$(RM) -r "$(1)"/updated/vendor
	cd "$(1)"/updated && $(GO) mod tidy && $(GO) mod vendor && $(GO) mod verify
	cd "$(1)" && $(deps_diff) -r {current,updated}/vendor/ > updated/deps.diff || true
endef

verify-deps: tmp_dir:=$(shell mktemp -d)
verify-deps:
	$(call restore-deps,$(tmp_dir))
	@echo $(deps_diff) "$(tmp_dir)"/{current,updated}/go.mod
	@     $(deps_diff) "$(tmp_dir)"/{current,updated}/go.mod || ( echo '`go.mod` content is incorrect - did you run `go mod tidy`?' && false )
	@echo $(deps_diff) "$(tmp_dir)"/{current,updated}/go.sum
	@     $(deps_diff) "$(tmp_dir)"/{current,updated}/go.sum || ( echo '`go.sum` content is incorrect - did you run `go mod tidy`?' && false )
	@echo $(deps_diff) '$(tmp_dir)'/{current,updated}/deps.diff
	@     $(deps_diff) '$(tmp_dir)'/{current,updated}/deps.diff || ( \
		echo "ERROR: Content of 'vendor/' directory doesn't match 'go.mod' configuration and the overrides in 'deps.diff'!" && \
		echo 'Did you run `go mod vendor`?' && \
		echo "If this is an intentional change (a carry patch) please update the 'deps.diff' using 'make update-deps-overrides'." && \
		false \
	)
.PHONY: verify-deps

update-deps-overrides: tmp_dir:=$(shell mktemp -d)
update-deps-overrides:
	$(call restore-deps,$(tmp_dir))
	cp "$(tmp_dir)"/{updated,current}/deps.diff
.PHONY: update-deps-overrides

verify-helm:
	@$(foreach chart,$(HELM_CHARTS),$(call lint-helm,$(chart)))
.PHONY: verify-helm

define run-codegen
	GOPATH=$(GOPATH) $(GO) run "$(CODEGEN_PKG)/cmd/$(1)" --go-header-file='$(CODEGEN_HEADER_FILE)' $(2)

endef

define run-deepcopy-gen
	$(call run-codegen,deepcopy-gen,--input-dirs='github.com/scylladb/scylla-operator/pkg/api/scylla/v1' --output-file-base='zz_generated.deepcopy' --bounding-dirs='github.com/scylladb/scylla-operator/pkg/api/' $(1))

endef

define run-client-gen
	$(call run-codegen,client-gen,--clientset-name=versioned --input-base="./" --input='github.com/scylladb/scylla-operator/pkg/api/scylla/v1' --output-package='github.com/scylladb/scylla-operator/pkg/client/scylla/clientset' $(1))

endef

define run-lister-gen
	$(call run-codegen,lister-gen,--input-dirs='github.com/scylladb/scylla-operator/pkg/api/scylla/v1' --output-package='github.com/scylladb/scylla-operator/pkg/client/scylla/listers' $(1))

endef

define run-informer-gen
	$(call run-codegen,informer-gen,--input-dirs='github.com/scylladb/scylla-operator/pkg/api/scylla/v1' --output-package='github.com/scylladb/scylla-operator/pkg/client/scylla/informers' --versioned-clientset-package "github.com/scylladb/scylla-operator/pkg/client/scylla/clientset/versioned" --listers-package="github.com/scylladb/scylla-operator/pkg/client/scylla/listers" $(1))

endef

update-codegen:
	$(call run-deepcopy-gen,)
	$(call run-client-gen,)
	$(call run-lister-gen,)
	$(call run-informer-gen,)
.PHONY: update-codegen

update-crd:
	$(CONTROLLER_GEN) crd paths="$(GO_PACKAGES)" output:crd:stdout > ./deploy/operator/00_scylla.scylladb.com_scyllaclusters.yaml
.PHONY: update-crd

update-dev-deploy:
	mkdir -p deploy/manager/dev
	cp deploy/manager/*.yaml deploy/manager/dev/
	yq eval 'select(.apiVersion=="scylla.scylladb.com/v1" and .kind=="ScyllaCluster" and .metadata.name=="scylla-manager-cluster") | .spec.cpuset=false | .spec.datacenter.racks[0].resources.limits.cpu="200m" | .spec.datacenter.racks[0].resources.limits.memory="200Mi" | .spec.datacenter.racks[0].resources.requests.cpu="10m" | .spec.datacenter.racks[0].resources.requests.memory="100Mi"' deploy/manager/10_scylla_cluster.yaml > deploy/manager/dev/10_scylla_cluster.yaml
.PHONY: update-dev-deploy

verify-dev-deploy:
	@$(MAKE) update-dev-deploy
	@git diff --quiet deploy/manager/dev || echo 'Development deployment dir was not updated, make sure to regenerate it' && false
.PHONY: verify-dev-deploy

update-example-operator:
	$(call concat-manifests,$(sort $(wildcard deploy/operator/*.yaml)),examples/common/operator.yaml)
.PHONY: update-example-operator

update-example-manager:
	$(call concat-manifests,$(sort $(wildcard deploy/manager/*.yaml)),examples/common/manager.yaml)
.PHONY: update-example-manager

update-examples: update-example-manager update-example-operator
.PHONY: update-examples

verify-codegen:
	$(call run-deepcopy-gen,--verify-only)
	$(call run-client-gen,--verify-only)
	$(call run-lister-gen,--verify-only)
	$(call run-informer-gen,--verify-only)
.PHONY: verify-codegen

verify: verify-govet verify-gofmt verify-helm verify-codegen verify-dev-deploy
.PHONY: verify

update: update-gofmt update-codegen update-crd update-examples update-dev-deploy
.PHONY: update

test-unit:
	$(GO) test $(GO_TEST_COUNT) $(GO_TEST_FLAGS) $(GO_TEST_EXTRA_FLAGS) $(GO_TEST_PACKAGES) $(if $(GO_TEST_ARGS)$(GO_TEST_EXTRA_ARGS),-args $(GO_TEST_ARGS) $(GO_TEST_EXTRA_ARGS))
.PHONY: test-unit

test-integration: GO_TEST_PACKAGES :=./test/integration/...
test-integration: GO_TEST_COUNT :=-count=1
test-integration: GO_TEST_FLAGS += -p=1 -timeout 30m -v
test-integration: GO_TEST_ARGS += -ginkgo.progress
test-integration: test-unit
.PHONY: test-integration

test-e2e:
	$(GO) run ./cmd/scylla-operator-tests run $(GO_TEST_E2E_EXTRA_ARGS)
.PHONY: test-e2e

test-version-script:
	./hack/lib/tag-from-gh-ref.sh
.PHONY: test-version-script

test: test-unit test-version-script
.PHONY: test

help:
	$(info The following make targets are available:)
	@$(MAKE) -f $(firstword $(MAKEFILE_LIST)) --print-data-base --question no-such-target 2>&1 | grep -v 'no-such-target' | \
	grep -v -e '^no-such-target' -e '^makefile' | \
	awk '/^[^.%][-A-Za-z0-9_]*:/	{ print substr($$1, 1, length($$1)-1) }' | sort -u
.PHONY: help

cert-manager:
	kubectl apply -f examples/common/cert-manager.yaml
	kubectl -n cert-manager wait --for=condition=ready pod -l app=cert-manager --timeout=60s
	kubectl -n cert-manager wait --for=condition=ready pod -l app=cainjector --timeout=60s
	kubectl -n cert-manager wait --for=condition=ready pod -l app=webhook --timeout=60s
.PHONY: cert-manager

image:
	docker build . -t $(IMAGE_REF)
.PHONY: image

# Build Helm charts and publish them in Development GCS repo
helm-publish-dev: HELM_REPOSITORY=https://scylla-operator-charts-dev.storage.googleapis.com/$(HELM_CHANNEL)
helm-publish-dev: HELM_BUCKET=gs://scylla-operator-charts-dev/$(HELM_CHANNEL)
helm-publish-dev: helm-publish
.PHONY: helm-publish-dev

# Build Helm charts and publish them in GCS repo
helm-publish:
	mkdir -p $(HELM_LOCAL_REPO)
	gsutil rsync -d $(HELM_BUCKET) $(HELM_LOCAL_REPO)

	@$(foreach chart,$(HELM_CHARTS),$(call package-helm,$(chart),$(HELM_LOCAL_REPO),$(HELM_APP_VERSION),$(HELM_CHART_VERSION)))

	helm repo index $(HELM_LOCAL_REPO) --url $(HELM_REPOSITORY) --merge $(HELM_LOCAL_REPO)/index.yaml
	gsutil rsync -d $(HELM_LOCAL_REPO) $(HELM_BUCKET)
.PHONY: helm-publish
