all: test local-build

# Image URL to use all building/pushing image targets
REPO	?= scylladb/scylla-operator
TAG		?= $(shell git describe --tags --always --abbrev=0)
IMG		?= $(REPO):$(TAG)
# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:trivialVersions=true"

.EXPORT_ALL_VARIABLES:
DOCKER_BUILDKIT		:= 1
GOVERSION			:= $(shell go version)
GOPATH				:= $(shell go env GOPATH)
KUBEBUILDER_ASSETS	:= $(GOPATH)/bin
PATH				:= $(GOPATH)/bin:$(PATH):

# Default package
PKG := ./pkg/...

# Run tests
test: fmt vet
	go test $(PKG) -coverprofile cover.out

integration-test:
	go test $(PKG) -tags integration

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
manifests:
	controller-gen $(CRD_OPTIONS) paths="$(PKG)" output:crd:dir=config/operator/crd/bases \
	rbac:roleName=manager-role output:rbac:artifacts:config=config/operator/rbac \
	webhook output:webhook:artifacts:config=config/operator/webhook

	controller-gen $(CRD_OPTIONS) paths="./pkg/controllers/manager" rbac:roleName=manager-role output:rbac:artifacts:config=config/manager/rbac
	kustomize build config/operator/default > examples/generic/operator.yaml
	kustomize build config/operator/default > examples/gke/operator.yaml
	kustomize build config/operator/default > examples/eks/operator.yaml
	kustomize build config/manager/default > examples/generic/manager.yaml
	kustomize build config/manager/default > examples/gke/manager.yaml
	kustomize build config/manager/default > examples/eks/manager.yaml

# Run go fmt against code
fmt:
	go fmt $(PKG)

# Run go vet against code
vet:
	go vet $(PKG)

# Generate code
generate:
	controller-gen object:headerFile="hack/boilerplate.go.txt" paths="$(PKG)"

# Build the docker image
.PHONY: docker-build
docker-build:
	goreleaser --skip-validate --skip-publish --rm-dist

# Push the docker image
docker-push:
	docker push ${IMG}

# Ensure dependencies
.PHONY: vendor
vendor:
	go mod vendor

# Build local-build binary
.PHONY: local-build
local-build: fmt vet vendor
	CGO_ENABLED=0 go build -trimpath -a -o bin/scylla-operator github.com/scylladb/scylla-operator/pkg/cmd

release:
	goreleaser --rm-dist
