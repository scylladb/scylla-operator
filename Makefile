all: test local-build

# Image URL to use all building/pushing image targets
REPO	?= scylladb/scylla-operator
TAG		?= $(shell git describe --tags --always --abbrev=0)
IMG		?= $(REPO):$(TAG)
# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:trivialVersions=true"

.EXPORT_ALL_VARIABLES:
DOCKER_BUILDKIT		:= 1
KUBEBUILDER_ASSETS	:= $(CURDIR)/bin/deps
PATH				:= $(CURDIR)/bin/deps:$(PATH):
PATH				:= $(CURDIR)/bin/deps/go/bin:$(PATH):
GOROOT				:= $(CURDIR)/bin/deps/go
GOVERSION			:= $(shell go version)

# Default package
PKG := ./pkg/...

# Run tests
test: generate fmt vet manifests
	go test $(PKG) -coverprofile cover.out

# Run against the configured Kubernetes cluster in ~/.kube/config
run: generate fmt vet manifests
	go run ./cmd operator --image="$(IMG)" --enable-admission-webhook=false

# Install CRDs into a cluster
install: manifests cert-manager
	kustomize build config/operator/crd | kubectl apply -f -

# Uninstall CRDs from a cluster
uninstall: manifests
	kustomize build config/operator/crd | kubectl delete -f -
	kubectl delete -f examples/generic/cert-manager.yaml

cert-manager:
	cat config/operator/certmanager/cert-manager.yaml > examples/generic/cert-manager.yaml
	cat config/operator/certmanager/cert-manager.yaml > examples/gke/cert-manager.yaml
	cat config/operator/certmanager/cert-manager.yaml > examples/eks/cert-manager.yaml
	kubectl apply -f examples/generic/cert-manager.yaml
	kubectl -n cert-manager wait --for=condition=ready pod -l app=cert-manager --timeout=60s
	kubectl -n cert-manager wait --for=condition=ready pod -l app=cainjector --timeout=60s
	kubectl -n cert-manager wait --for=condition=ready pod -l app=webhook --timeout=60s

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy: manifests cert-manager
	cd config/operator/operator && kustomize edit set image controller=${IMG}
	kustomize build config/operator/default | kubectl apply -f -

# Generate manifests e.g. CRD, RBAC etc.
manifests: bin/deps controller-gen
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=manager-role webhook paths="$(PKG)" output:crd:artifacts:config=config/operator/crd/bases output:rbac:artifacts:config=config/operator/rbac/bases
	kustomize build config/operator/default > examples/generic/operator.yaml
	kustomize build config/operator/default > examples/gke/operator.yaml
	kustomize build config/operator/default > examples/eks/operator.yaml
	kustomize build config/manager/default > examples/generic/manager.yaml
	kustomize build config/manager/default > examples/gke/manager.yaml
	kustomize build config/manager/default > examples/eks/manager.yaml

# Run go fmt against code
fmt: bin/deps
	go fmt $(PKG)

# Run go vet against code
vet: bin/deps
	go vet $(PKG)

# Generate code
generate: controller-gen
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="$(PKG)"

# Build the docker image
.PHONY: docker-build
docker-build: bin/deps
	goreleaser --skip-validate --skip-publish --rm-dist

# Push the docker image
docker-push:
	docker push ${IMG}

# Ensure dependencies
.PHONY: vendor
vendor: bin/deps
	go mod vendor

# Build local-build binary
.PHONY: local-build
local-build: fmt vet vendor
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o bin/scylla-operator github.com/scylladb/scylla-operator/pkg/cmd

# find or download controller-gen
# download controller-gen if necessary
controller-gen: bin/deps
CONTROLLER_GEN=bin/deps/controller-gen

release: bin/deps
	goreleaser --rm-dist

bin/deps: hack/binary_deps.py
	mkdir -p bin/deps
	hack/binary_deps.py bin/deps
