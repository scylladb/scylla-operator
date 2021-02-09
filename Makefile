all: build

SHELL :=/bin/bash -euEo pipefail

IMAGE_REF		?= docker.io/scylladb/scylla-operator:latest

# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS 	?= "crd:trivialVersions=true"
# Helm repo parameters
HELM_BUCKET 	?= "gs://scylla-operator-charts"
HELM_LOCAL_REPO	?= "helm/repo"
HELM_REPOSITORY	?= "https://scylla-operator-charts.storage.googleapis.com"

GO_REQUIRED_MIN_VERSION ?=1.15.7

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
GO_BUILD_PACKAGES ?=./pkg/cmd/...
GO_BUILD_PACKAGES_EXPANDED ?=$(shell $(GO) list $(GO_BUILD_PACKAGES))
go_build_binaries =$(notdir $(GO_BUILD_PACKAGES_EXPANDED))
GO_BUILD_FLAGS ?=-trimpath
GO_BUILD_BINDIR ?=
GO_LD_EXTRAFLAGS ?=
GO_TEST_PACKAGES :=./pkg/... # ./cmd/...
GO_TEST_FLAGS ?=-race
GO_TEST_ARGS ?=


define version-ldflags
-X $(1).versionFromGit="$(GIT_TAG)" \
-X $(1).commitFromGit="$(GIT_COMMIT)" \
-X $(1).gitTreeState="$(GIT_TREE_STATE)" \
-X $(1).buildDate="$(shell date -u +'%Y-%m-%dT%H:%M:%SZ')"
endef
GO_LD_FLAGS ?=-ldflags "$(call version-ldflags,$(GO_PACKAGE)/pkg/version) $(GO_LD_EXTRAFLAGS)"

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

# We need to build each package separately so go build creates appropriate binaries
build:
	$(if $(strip $(GO_BUILD_PACKAGES_EXPANDED)),,$(error no packages to build: GO_BUILD_PACKAGES_EXPANDED var is empty))
	$(foreach package,$(GO_BUILD_PACKAGES_EXPANDED),$(call build-package,$(package)))
.PHONY: build

clean:
	$(RM) $(go_build_binaries)
.PHONY: clean

verify-govet:
	$(GO) vet $(GO_MOD_FLAGS) $(GO_PACKAGES)
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

verify: verify-govet verify-gofmt
.PHONY: verify

update: update-gofmt
.PHONY: update

test-unit:
	$(GO) test $(GO_TEST_FLAGS) $(GO_TEST_PACKAGES) $(if $(GO_TEST_ARGS),-args $(GO_TEST_ARGS))
.PHONY: test-unit

test-integration: GO_TEST_PACKAGES :=./test/integration/...
test-integration: GO_TEST_FLAGS += -count=1 -p=1 -timeout 30m
test-integration: test-unit
.PHONY: test-integration

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

# Install CRDs into a cluster
install: manifests cert-manager
	kustomize build config/operator/crd | kubectl apply -f -
.PHONY: install

# Uninstall CRDs from a cluster
uninstall: manifests
	kustomize build config/operator/crd | kubectl delete -f -
	kubectl delete -f examples/common/cert-manager.yaml
.PHONY: uninstall

cert-manager:
	cat config/operator/certmanager/cert-manager.yaml > examples/common/cert-manager.yaml
	kubectl apply -f examples/common/cert-manager.yaml
	kubectl -n cert-manager wait --for=condition=ready pod -l app=cert-manager --timeout=60s
	kubectl -n cert-manager wait --for=condition=ready pod -l app=cainjector --timeout=60s
	kubectl -n cert-manager wait --for=condition=ready pod -l app=webhook --timeout=60s
.PHONY: cert-manager

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy: manifests cert-manager
	kubectl apply -f examples/common/operator.yaml
.PHONY: deploy

# Generate manifests e.g. CRD, RBAC etc.
manifests:
	cd config/operator/operator && kustomize edit set image controller=$(IMAGE_REF)
	cd config/manager/manager && kustomize edit set image operator=$(IMAGE_REF)

	controller-gen $(CRD_OPTIONS) paths="$(GO_PACKAGES)" output:crd:dir=config/operator/crd/bases \
	rbac:roleName=manager-role output:rbac:artifacts:config=config/operator/rbac \
	webhook output:webhook:artifacts:config=config/operator/webhook

	controller-gen $(CRD_OPTIONS) paths="./pkg/controllers/manager" rbac:roleName=manager-role output:rbac:artifacts:config=config/manager/rbac
	kustomize build config/operator/default > examples/common/operator.yaml
	kustomize build config/manager/default > examples/common/manager.yaml
.PHONY: manifests

latest:
	docker build . -t $(IMAGE_REF)
.PHONY: latest

# Generate code
generate:
	controller-gen object:headerFile="hack/boilerplate.go.txt" paths="$(GO_PACKAGES)"
.PHONY: generate

# Build Helm charts and publish them in GCS repo
helm-release:
	mkdir -p $(HELM_LOCAL_REPO)
	gsutil rsync -d $(HELM_BUCKET) $(HELM_LOCAL_REPO)
	helm package helm/scylla-operator helm/scylla helm/scylla-manager -d $(HELM_LOCAL_REPO)
	helm repo index $(HELM_LOCAL_REPO) --url $(HELM_REPOSITORY) --merge $(HELM_LOCAL_REPO)/index.yaml
	gsutil rsync -d $(HELM_LOCAL_REPO) $(HELM_BUCKET)
.PHONY: helm-release
