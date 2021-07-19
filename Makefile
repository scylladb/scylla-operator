all: build

SHELL :=/bin/bash -euEo pipefail

IMAGE_TAG ?=1.4
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

YQ ?=yq

HELM ?=helm
HELM_CHANNEL ?=latest
HELM_CHARTS ?=scylla-operator scylla-manager scylla
HELM_CHARTS_DIR ?=helm
HELM_LOCAL_REPO ?=$(HELM_CHARTS_DIR)/repo/$(HELM_CHANNEL)
HELM_APP_VERSION ?=$(IMAGE_TAG)
HELM_CHART_VERSION_SUFFIX ?=
HELM_CHART_VERSION ?=$(GIT_TAG)$(HELM_CHART_VERSION_SUFFIX)
HELM_BUCKET ?=gs://scylla-operator-charts/$(HELM_CHANNEL)
HELM_REPOSITORY ?=https://scylla-operator-charts.storage.googleapis.com/$(HELM_CHANNEL)
HELM_MANIFEST_CACHE_CONTROL ?=public, max-age=600

CONTROLLER_GEN ?=$(GO) run ./vendor/sigs.k8s.io/controller-tools/cmd/controller-gen --
CRD_PATH ?= pkg/api/scylla/v1/scylla.scylladb.com_scyllaclusters.yaml

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
	$(foreach file,$(1),$(call append-manifest,$(file),$(2)))
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
diff :=LC_COLLATE=C diff --no-dereference -N

# $1 - temporary directory
define restore-deps
	ln -s $(abspath ./) "$(1)"/current
	cp -R -H ./ "$(1)"/updated
	$(RM) -r "$(1)"/updated/vendor
	cd "$(1)"/updated && $(GO) mod tidy && $(GO) mod vendor && $(GO) mod verify
	cd "$(1)" && $(diff) -r {current,updated}/vendor/ > updated/deps.diff || true
endef

verify-deps: tmp_dir:=$(shell mktemp -d)
verify-deps:
	$(call restore-deps,$(tmp_dir))
	@echo $(diff) "$(tmp_dir)"/{current,updated}/go.mod
	@     $(diff) "$(tmp_dir)"/{current,updated}/go.mod || ( echo '`go.mod` content is incorrect - did you run `go mod tidy`?' && false )
	@echo $(diff) "$(tmp_dir)"/{current,updated}/go.sum
	@     $(diff) "$(tmp_dir)"/{current,updated}/go.sum || ( echo '`go.sum` content is incorrect - did you run `go mod tidy`?' && false )
	@echo $(diff) '$(tmp_dir)'/{current,updated}/deps.diff
	@     $(diff) '$(tmp_dir)'/{current,updated}/deps.diff || ( \
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

verify-helm-lint:
	@$(foreach chart,$(HELM_CHARTS),$(call lint-helm,$(chart)))
.PHONY: verify-helm-lint

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

# $1 - target file
define generate-crd
	$(CONTROLLER_GEN) crd paths="$(GO_PACKAGES)" output:crd:stdout > '$(1)'
endef

update-crd:
	$(call generate-crd,$(CRD_PATH))
.PHONY: update-crd

verify-crd: tmp_file :=$(shell mktemp)
verify-crd:
	$(call generate-crd,$(tmp_file))
	$(diff) '$(tmp_file)' '$(CRD_PATH)' || (echo 'File $(CRD_PATH) is not up-to date. Please run `make update-crd` to update it.' && false)
.PHONY: verify-crd

# $1 - name
# $2 - chart path
# $3 - values path
# $4 - target dir
define generate-manifests-from-helm
	$(HELM) template '$(1)' '$(2)' --namespace='$(1)' --values='$(3)' --output-dir='$(4)'
	find '$(4)' -name '*.yaml' -exec sed -i -e '/^---$$/d' -e '/^# Source: /d' {} \;
endef

# $1 - Helm values file
# $2 - output_dir
# $3 - tmp_dir
define generate-operator-manifests
	$(call generate-manifests-from-helm,scylla-operator,helm/scylla-operator,$(1),$(3))

	mv '$(3)'/scylla-operator/templates/clusterrole.yaml '$(2)'/00_clusterrole.yaml
	mv '$(3)'/scylla-operator/templates/clusterrole_def.yaml '$(2)'/00_clusterrole_def.yaml
	mv '$(3)'/scylla-operator/templates/view_clusterrole.yaml '$(2)'/00_scyllacluster_clusterrole_view.yaml
	mv '$(3)'/scylla-operator/templates/edit_clusterrole.yaml '$(2)'/00_scyllacluster_clusterrole_edit.yaml
	mv '$(3)'/scylla-operator/templates/scyllacluster_member_clusterrole.yaml '$(2)'/00_scyllacluster_member_clusterrole.yaml
	mv '$(3)'/scylla-operator/templates/scyllacluster_member_clusterrole_def.yaml '$(2)'/00_scyllacluster_member_clusterrole_def.yaml

	mv '$(3)'/scylla-operator/templates/issuer.yaml '$(2)'/10_issuer.yaml
	mv '$(3)'/scylla-operator/templates/certificate.yaml '$(2)'/10_certificate.yaml
	mv '$(3)'/scylla-operator/templates/validatingwebhook.yaml '$(2)'/10_validatingwebhook.yaml
	mv '$(3)'/scylla-operator/templates/webhookserver.service.yaml '$(2)'/10_webhookserver.service.yaml
	mv '$(3)'/scylla-operator/templates/webhookserver.serviceaccount.yaml '$(2)'/10_webhookserver.serviceaccount.yaml
	mv '$(3)'/scylla-operator/templates/operator.serviceaccount.yaml '$(2)'/10_operator.serviceaccount.yaml
	mv '$(3)'/scylla-operator/templates/pdb.yaml '$(2)'/10_pdb.yaml

	mv '$(3)'/scylla-operator/templates/clusterrolebinding.yaml '$(2)'/20_clusterrolebinding.yaml

	mv '$(3)'/scylla-operator/templates/operator.deployment.yaml '$(2)'/50_operator.deployment.yaml
	mv '$(3)'/scylla-operator/templates/webhookserver.deployment.yaml '$(2)'/50_webhookserver.deployment.yaml

	@leftovers=$$( find '$(3)'/scylla-operator/ -mindepth 1 -type f ) && [[ "$${leftovers}" == "" ]] || \
	( echo -e "Internal error: Unhandled helm files: \n$${leftovers}" && false )
endef

# $1 - Helm values file
# $2 - output_dir
# $3 - tmp_dir
define generate-manager-manifests-prod
	$(call generate-manifests-from-helm,scylla-manager,helm/scylla-manager,$(1),$(3))

	mv '$(3)'/scylla-manager/templates/controller_clusterrole.yaml '$(2)'/00_controller_clusterrole.yaml
	mv '$(3)'/scylla-manager/templates/controller_clusterrole_def.yaml '$(2)'/00_controller_clusterrole_def.yaml

	mv '$(3)'/scylla-manager/templates/controller_serviceaccount.yaml '$(2)'/10_controller_serviceaccount.yaml
	mv '$(3)'/scylla-manager/templates/controller_pdb.yaml '$(2)'/10_controller_pdb.yaml
	mv '$(3)'/scylla-manager/templates/manager_service.yaml '$(2)'/10_manager_service.yaml
	mv '$(3)'/scylla-manager/templates/manager_serviceaccount.yaml '$(2)'/10_manager_serviceaccount.yaml
	mv '$(3)'/scylla-manager/templates/manager_configmap.yaml '$(2)'/10_manager_configmap.yaml
	mv '$(3)'/scylla-manager/charts/scylla/templates/serviceaccount.yaml '$(2)'/10_scyllacluster_serviceaccount.yaml

	mv '$(3)'/scylla-manager/templates/controller_clusterrolebinding.yaml '$(2)'/20_controller_clusterrolebinding.yaml
	mv '$(3)'/scylla-manager/charts/scylla/templates/rolebinding.yaml '$(2)'/20_scyllacluster_rolebinding.yaml

	mv '$(3)'/scylla-manager/charts/scylla/templates/scyllacluster.yaml '$(2)'/50_scyllacluster.yaml
	mv '$(3)'/scylla-manager/templates/controller_deployment.yaml '$(2)'/50_controller_deployment.yaml
	mv '$(3)'/scylla-manager/templates/manager_deployment.yaml '$(2)'/50_manager_deployment.yaml

	@leftovers=$$( find '$(3)'/scylla-manager/ -mindepth 1 -type f ) && [[ "$${leftovers}" == "" ]] || \
	( echo -e "Internal error: Unhandled helm files: \n$${leftovers}" && false )
endef

# $1 - output_dir
define generate-manager-manifests-dev
	cp -r deploy/manager/prod/. '$(1)'/.
	$(YQ) eval -i -P '.spec.cpuset = false | .spec.datacenter.racks[0].resources = {"limits": {"cpu": "200m", "memory": "200Mi"}, "requests": {"cpu": "10m", "memory": "100Mi"}}' '$(1)'/50_scyllacluster.yaml
endef

# $1 - chart dir
# $2 - default app version
define set-default-app-version
	$(YQ) eval -i -P '.appVersion = "$(2)"' '$(1)'/Chart.yaml
endef

update-helm-charts:
	$(call set-default-app-version,helm/scylla-operator,$(IMAGE_TAG))
	$(call set-default-app-version,helm/scylla-manager,$(IMAGE_TAG))
.PHONY: update-helm-charts

verify-helm-charts: tmp_dir:=$(shell mktemp -d)
verify-helm-charts:
	cp -r helm/scylla-{operator,manager} '$(tmp_dir)'

	$(call set-default-app-version,$(tmp_dir)/scylla-operator,$(IMAGE_TAG))
	$(diff) -r '$(tmp_dir)'/scylla-operator helm/scylla-operator

	$(call set-default-app-version,$(tmp_dir)/scylla-manager,$(IMAGE_TAG))
	$(diff) -r '$(tmp_dir)'/scylla-manager helm/scylla-manager
.PHONY: verify-helm-charts

update-deploy: tmp_dir:=$(shell mktemp -d)
update-deploy:
	$(call generate-operator-manifests,helm/deploy/operator.yaml,deploy/operator,$(tmp_dir))
	$(call generate-manager-manifests-prod,helm/deploy/manager_prod.yaml,deploy/manager/prod,$(tmp_dir))
	$(call generate-manager-manifests-dev,deploy/manager/dev)
.PHONY: update-deploy

verify-deploy: tmp_dir :=$(shell mktemp -d)
verify-deploy: tmp_dir_generate :=$(shell mktemp -d)
verify-deploy:
	mkdir -p $(tmp_dir)/{operator,manager/{prod,dev}}

	cp -r deploy/operator/. $(tmp_dir)/operator/.
	$(call generate-operator-manifests,helm/deploy/operator.yaml,$(tmp_dir)/operator,$(tmp_dir_generate))
	$(diff) -r '$(tmp_dir)'/operator deploy/operator

	cp -r deploy/manager/prod/. $(tmp_dir)/manager/prod/.
	$(call generate-manager-manifests-prod,helm/deploy/manager_prod.yaml,$(tmp_dir)/manager/prod,$(tmp_dir_generate))
	$(diff) -r '$(tmp_dir)'/manager/prod deploy/manager/prod

	$(call generate-manager-manifests-dev,$(tmp_dir)/manager/dev)
	$(diff) -r '$(tmp_dir)'/manager/dev deploy/manager/dev

.PHONY: verify-deploy

update-examples-operator:
	$(call concat-manifests,$(sort $(wildcard deploy/operator/*.yaml)),examples/common/operator.yaml)
.PHONY: update-examples-operator

verify-example-operator: tmp_file := $(shell mktemp)
verify-example-operator:
	$(call concat-manifests,$(sort $(wildcard deploy/operator/*.yaml)),$(tmp_file))
	$(diff) '$(tmp_file)' examples/common/operator.yaml || (echo 'Operator example is not up-to date. Please run `make update-examples-operator` to update it.' && false)
.PHONY: verify-example-operator

update-examples-manager:
	$(call concat-manifests,$(sort $(wildcard deploy/manager/prod/*.yaml)),examples/common/manager.yaml)
.PHONY: update-examples-manager

verify-example-manager: tmp_file :=$(shell mktemp)
verify-example-manager:
	$(call concat-manifests,$(sort $(wildcard deploy/manager/prod/*.yaml)),$(tmp_file))
	$(diff) '$(tmp_file)' examples/common/manager.yaml || (echo 'Manager example is not up-to date. Please run `make update-examples-manager` to update it.' && false)
.PHONY: verify-example-manager

update-examples: update-examples-manager update-examples-operator
.PHONY: update-examples

verify-examples: verify-example-manager verify-example-operator
.PHONY: verify-examples

verify-codegen:
	$(call run-deepcopy-gen,--verify-only)
	$(call run-client-gen,--verify-only)
	$(call run-lister-gen,--verify-only)
	$(call run-informer-gen,--verify-only)
.PHONY: verify-codegen

verify: verify-gofmt verify-codegen verify-crd verify-helm-charts verify-deploy verify-examples verify-govet verify-helm-lint
.PHONY: verify

update: update-gofmt update-codegen update-crd update-helm-charts update-deploy update-examples
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
helm-publish-dev: HELM_MANIFEST_CACHE_CONTROL :=no-cache, no-store, must-revalidate
helm-publish-dev: helm-publish
.PHONY: helm-publish-dev

# Build Helm charts and publish them in GCS repo
helm-publish:
	mkdir -p $(HELM_LOCAL_REPO)
	gsutil rsync -d $(HELM_BUCKET) $(HELM_LOCAL_REPO)

	@$(foreach chart,$(HELM_CHARTS),$(call package-helm,$(chart),$(HELM_LOCAL_REPO),$(HELM_APP_VERSION),$(HELM_CHART_VERSION)))

	helm repo index $(HELM_LOCAL_REPO) --url $(HELM_REPOSITORY) --merge $(HELM_LOCAL_REPO)/index.yaml
	gsutil rsync -d $(HELM_LOCAL_REPO) $(HELM_BUCKET)

	gsutil setmeta -h 'Content-Type:text/yaml' -h 'Cache-Control: $(HELM_MANIFEST_CACHE_CONTROL)' '$(HELM_BUCKET)/index.yaml'
.PHONY: helm-publish
