all: build

SHELL :=/bin/bash -euEo pipefail -O inherit_errexit

comma :=,

IMAGE_TAG ?= latest
IMAGE_REF ?= docker.io/scylladb/scylla-operator:$(IMAGE_TAG)

MAKE_REQUIRED_MIN_VERSION:=4.2 # for SHELLSTATUS
GO_REQUIRED_MIN_VERSION ?=1.18

GIT_TAG ?=$(shell git describe --long --tags --abbrev=7 --match 'v[0-9]*')$(if $(filter $(.SHELLSTATUS),0),,$(error git describe failed))
GIT_TAG_SHORT ?=$(shell git describe --tags --abbrev=7 --match 'v[0-9]*')$(if $(filter $(.SHELLSTATUS),0),,$(error git describe failed))
GIT_COMMIT ?=$(shell git rev-parse --short "HEAD^{commit}" 2>/dev/null)$(if $(filter $(.SHELLSTATUS),0),,$(error git rev-parse failed))
GIT_TREE_STATE ?=$(shell ( ( [ ! -d ".git/" ] || git diff --quiet ) && echo 'clean' ) || echo 'dirty')

GO ?=go
GO_MODULE ?=$(shell $(GO) list -m)$(if $(filter $(.SHELLSTATUS),0),,$(error failed to list go module name))
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

JQ ?=jq
YQ ?=yq -e
GSUTIL ?=gsutil -m -q

CODEGEN_PKG ?=./vendor/k8s.io/code-generator
CODEGEN_HEADER_FILE ?=/dev/null

api_groups :=$(patsubst %/,%,$(wildcard ./pkg/api/*/))
external_api_groups :=$(patsubst %/.,%,$(wildcard ./pkg/externalapi/*/.))
nonrest_api_groups :=$(patsubst %/.,%,$(wildcard ./pkg/scylla/api/*/.))

api_package_dirs :=$(api_groups) $(external_api_groups)
api_packages =$(call expand_go_packages_with_spaces,$(addsuffix /...,$(api_package_dirs)))

HELM ?=helm
HELM_CHANNEL ?=latest
HELM_CHARTS ?=scylla-operator scylla-manager scylla
HELM_CHARTS_DIR ?=helm
HELM_BUILD_DIR ?=$(HELM_CHARTS_DIR)/build
HELM_LOCAL_REPO ?=$(HELM_CHARTS_DIR)/repo/$(HELM_CHANNEL)
HELM_APP_VERSION ?=$(IMAGE_TAG)
HELM_CHART_VERSION_SUFFIX ?=
HELM_CHART_VERSION ?=$(GIT_TAG_SHORT)$(HELM_CHART_VERSION_SUFFIX)
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
GO_LD_FLAGS ?=-ldflags '$(strip $(call version-ldflags,$(GO_PACKAGE)/pkg/version) $(GO_LD_EXTRA_FLAGS))'

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

ifneq "$(MAKE_REQUIRED_MIN_VERSION)" ""
$(call require_minimal_version,make,MAKE_REQUIRED_MIN_VERSION,$(MAKE_VERSION))
endif

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

# We need to force locale so different envs sort files the same way for recursive traversals
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

# $1 - api packages
# $2 - extra args
define run-deepcopy-gen
	$(call run-codegen,deepcopy-gen,--input-dirs='$(1)' --output-file-base='zz_generated.deepcopy' $(2))

endef

# $1 - group
# $2 - api packages
# $3 - client dir
# $4 - extra args
define run-client-gen
	$(call run-codegen,client-gen,--clientset-name=versioned --input-base='./' --input='$(2)' --output-package='$(GO_PACKAGE)/$(3)/$(1)/clientset' $(4))

endef

# $1 - group
# $2 - api packages
# $3 - client dir
# $4 - extra args
define run-lister-gen
	$(call run-codegen,lister-gen,--input-dirs='$(2)' --output-package='$(GO_PACKAGE)/$(3)/$(1)/listers' $(4))

endef

# $1 - group
# $2 - api packages
# $3 - client dir
# $4 - extra args
define run-informer-gen
	$(call run-codegen,informer-gen,--input-dirs='$(2)' --output-package='$(GO_PACKAGE)/$(3)/$(1)/informers' --versioned-clientset-package '$(GO_PACKAGE)/$(3)/$(1)/clientset/versioned' --listers-package='$(GO_PACKAGE)/$(3)/$(1)/listers' $(4))

endef

# $1 - packages
expand_go_packages_to_json=$(shell $(GO) list -json $(1) | $(JQ) -sr 'map(select(.Name | test("^v[0-9]+.*$$"))) | reduce .[] as $$item ([]; . + [$$item.ImportPath])')$(if $(filter $(.SHELLSTATUS),0),,$(error failed to expand packages to json: $(1)))

# $1 - packages
expand_go_packages_with_commas=$(shell echo '$(call expand_go_packages_to_json,$(1))' | $(JQ) -r '. | join(",")')$(if $(filter $(.SHELLSTATUS),0),,$(error failed to expand packages with commas: $(1)))

# $1 - packages
expand_go_packages_with_spaces=$(shell echo '$(call expand_go_packages_to_json,$(1))' | $(JQ) -r '. | join(" ")')$(if $(filter $(.SHELLSTATUS),0),,$(error failed to expand packages with spaces: $(1)))

# $1 - group
# $2 - api packages
# $3 - client dir
# $4 - extra args
define run-client-generators
	$(call run-client-gen,$(1),$(2),$(3),$(4))
	$(call run-lister-gen,$(1),$(2),$(3),$(4))
	$(call run-informer-gen,$(1),$(2),$(3),$(4))

endef

# $1 - extra args
define run-update-codegen
	$(call run-deepcopy-gen,$(call expand_go_packages_with_commas,$(addsuffix /...,$(api_groups)) $(addsuffix /...,$(nonrest_api_groups)) $(addsuffix /...,$(external_api_groups))),$(1))
	$(foreach group,$(api_groups),$(call run-client-generators,$(notdir $(group)),$(call expand_go_packages_with_commas,$(group)/...),pkg/client,$(1)))
	$(foreach group,$(external_api_groups),$(call run-client-generators,$(notdir $(group)),$(call expand_go_packages_with_commas,$(group)/...),pkg/externalclient,$(1)))

endef

update-codegen:
	$(call run-update-codegen,)
.PHONY: update-codegen

verify-codegen:
	$(call run-update-codegen,--verify-only)
.PHONY: verify-codegen

# $1 - api package
# $2 - output dir
# We need to cleanup `---` in the yaml output manually because it shouldn't be there and it breaks opm.
define run-crd-gen
	$(CONTROLLER_GEN) crd paths='$(1)' output:dir='$(2)'
	find '$(2)' -mindepth 1 -maxdepth 1 -type f -name '*.yaml' -exec $(YQ) -i eval '.' {} \;

endef

# $1 - dir prefix
define generate-crds
	$(foreach p,$(api_packages),$(call run-crd-gen,$(subst $(GO_MODULE)/,,./$(p)),$(1)$(subst $(GO_MODULE)/,,./$(p))))

endef

update-crds:
	$(call generate-crds,)
.PHONY: update-crds

verify-crds: tmp_dir :=$(shell mktemp -d)
verify-crds:
	mkdir '$(tmp_dir)'/{original,generated}

	find $(api_package_dirs) -type f -name '*.yaml' -exec cp --parent {} '$(tmp_dir)'/original \;

	$(call generate-crds,$(tmp_dir)/generated/)

	$(diff) -r '$(tmp_dir)'/{original,generated} || (echo 'CRD definitions are not up-to date. Please run `make update-crds` to update it or manually remove the ones that should no longer be generated.' && false)
.PHONY: verify-crds

# $1 - target file
define generate-helm-schema-scylla
	$(YQ) eval -j '{"$$schema": "http://json-schema.org/schema#"} * (.spec.versions[] | select(.name == "v1") | \
	 .schema.openAPIV3Schema.properties.spec) | \
	 .properties.racks=.properties.datacenter.properties.racks | \
	 .properties.datacenter={"type": "string"}' $(CRD_PATH) > '$(1)'
endef

# $1 - Scylla schema
# $2 - target file
define generate-helm-schema-scylla-manager
	$(YQ) eval-all -j 'select(fi==0).properties.scylla = ( \
		select(fi==1) | del(.$$schema) \
	) | select(fi==0)' '$(HELM_CHARTS_DIR)/scylla-manager/values.schema.template.json' '$(1)' > '$(2)'
endef

update-helm-schemas:
	$(call generate-helm-schema-scylla,'$(HELM_CHARTS_DIR)/scylla/values.schema.json')
	$(call generate-helm-schema-scylla-manager,'$(HELM_CHARTS_DIR)/scylla/values.schema.json','$(HELM_CHARTS_DIR)/scylla-manager/values.schema.json')
.PHONY: update-helm-schemas

verify-helm-schemas: tmp_dir:=$(shell mktemp -d)
verify-helm-schemas:
	mkdir -p $(tmp_dir)/{scylla,scylla-manager}

	$(call generate-helm-schema-scylla,'$(tmp_dir)/scylla/values.schema.json')
	$(diff) '$(tmp_dir)/scylla/values.schema.json' '$(HELM_CHARTS_DIR)/scylla/values.schema.json'

	$(call generate-helm-schema-scylla-manager,'$(HELM_CHARTS_DIR)/scylla/values.schema.json','$(tmp_dir)/scylla-manager/values.schema.json')
	$(diff) '$(tmp_dir)/scylla-manager/values.schema.json' '$(HELM_CHARTS_DIR)/scylla-manager/values.schema.json'
.PHONY: verify-helm-schemas

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
	mv '$(3)'/scylla-operator/templates/operator.pdb.yaml '$(2)'/10_operator.pdb.yaml
	mv '$(3)'/scylla-operator/templates/webhookserver.pdb.yaml '$(2)'/10_webhookserver.pdb.yaml

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

	mv '$(3)'/scylla-manager/templates/controller_clusterrolebinding.yaml '$(2)'/20_controller_clusterrolebinding.yaml

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

# $1 - file path
# $2 - container name
# $3 - target image ref
define replace-template-container-image-ref
	$(YQ) eval -i '(.spec.template.spec.containers[]|select(.name == "$(2)").image) |= "$(3)"' $(1)
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

update-example-xfs-formatter:
	$(call replace-template-container-image-ref,examples/gke/xfs-formatter-daemonset.yaml,xfs-formatter,$(IMAGE_REF))
.PHONY: update-example-xfs-formatter

verify-example-xfs-formatter: tmp_dir :=$(shell mktemp -d)
verify-example-xfs-formatter:
	cp examples/gke/xfs-formatter-daemonset.yaml $(tmp_dir)/xfs-formatter-daemonset.yaml
	$(call replace-template-container-image-ref,$(tmp_dir)/xfs-formatter-daemonset.yaml,xfs-formatter,$(IMAGE_REF))
	$(diff) '$(tmp_dir)/xfs-formatter-daemonset.yaml' examples/gke/xfs-formatter-daemonset.yaml || (echo 'xfs-formatter example is not up-to date. Please run `make update-example-xfs-formatter` to update it.' && false)
.PHONY: verify-example-xfs-formatter

update-examples: update-examples-manager update-examples-operator update-example-xfs-formatter
.PHONY: update-examples

verify-examples: verify-example-manager verify-example-operator verify-example-xfs-formatter
.PHONY: verify-examples

verify-links:
	@set -euEo pipefail; broken_links=( $$( find . -type l ! -exec test -e {} \; -print ) ); \
	if [[ -n "$${broken_links[@]}" ]]; then \
		echo "The following links are broken:" > /dev/stderr; \
		ls -l --color=auto $${broken_links[@]}; \
		exit 1; \
	fi;
.PHONY: verify-links

verify: verify-gofmt verify-codegen verify-crds verify-helm-schemas verify-helm-charts verify-deploy verify-examples verify-govet verify-helm-lint verify-links
.PHONY: verify

update: update-gofmt update-codegen update-crds update-helm-schemas update-helm-charts update-deploy update-examples
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

test-scripts:
	./hack/lib/tag-from-gh-ref.sh
	./hack/test/gke/xfs-formatter/tune2fs_test.sh
.PHONY: test-scripts

test: test-unit test-scripts
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

helm-build:
	$(foreach chart,$(HELM_CHARTS),$(call package-helm,$(chart),$(HELM_BUILD_DIR),$(HELM_APP_VERSION),$(HELM_CHART_VERSION)))
.PHONY: helm-build

# Build Helm charts and publish them in Development GCS repo
helm-publish-dev: HELM_REPOSITORY=https://scylla-operator-charts-dev.storage.googleapis.com/$(HELM_CHANNEL)
helm-publish-dev: HELM_BUCKET=gs://scylla-operator-charts-dev/$(HELM_CHANNEL)
helm-publish-dev: HELM_MANIFEST_CACHE_CONTROL :=no-cache, no-store, must-revalidate
helm-publish-dev: helm-publish
.PHONY: helm-publish-dev

# Build Helm charts and publish them in GCS repo
helm-publish:
	mkdir -p '$(HELM_LOCAL_REPO)'
	$(GSUTIL) rsync -d '$(HELM_BUCKET)' '$(HELM_LOCAL_REPO)'

	$(foreach chart,$(HELM_CHARTS),$(call package-helm,$(chart),$(HELM_LOCAL_REPO),$(HELM_APP_VERSION),$(HELM_CHART_VERSION)))

	helm repo index '$(HELM_LOCAL_REPO)' --url '$(HELM_REPOSITORY)' --merge '$(HELM_LOCAL_REPO)/index.yaml'
	$(GSUTIL) rsync -d '$(HELM_LOCAL_REPO)' '$(HELM_BUCKET)'

	$(GSUTIL) setmeta -h 'Content-Type:text/yaml' -h 'Cache-Control: $(HELM_MANIFEST_CACHE_CONTROL)' '$(HELM_BUCKET)/index.yaml'
.PHONY: helm-publish
