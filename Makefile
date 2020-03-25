
# Image URL to use all building/pushing image targets
REPO ?= yanniszark/scylla-operator
TAG ?= $(shell git describe --tags --always)
IMG ?= $(REPO):$(TAG)
GOVERSION ?= $(go version)

.EXPORT_ALL_VARIABLES:
DOCKER_BUILDKIT = 1
GO111MODULE = off
KUBEBUILDER_ASSETS = $(CURDIR)/bin/deps
PATH := $(CURDIR)/bin/deps:$(CURDIR)/bin/deps/go/bin:$(PATH)

all: test local-build

# Run tests
test: fmt vet manifests vendor
	go test ./pkg/... ./cmd/... -coverprofile cover.out

# Build local-build binary
local-build: fmt vet vendor
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o bin/manager github.com/scylladb/scylla-operator/cmd

# Run against the configured Kubernetes cluster in ~/.kube/config
run: fmt vet vendor
	go run ./cmd operator --image="$(IMG)" --enable-admission-webhook=false

# Install CRDs into a cluster
install: manifests
	kubectl apply -f config/crds

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy: install
	kubectl apply -f config/rbac
	kustomize build config | kubectl apply -f -

# Generate manifests e.g. CRD, RBAC etc.
manifests:
	go run vendor/sigs.k8s.io/controller-tools/cmd/controller-gen/main.go all
	cd config && kustomize edit set image yanniszark/scylla-operator="$(IMG)"
	kustomize build config > examples/generic/operator.yaml
	kustomize build config > examples/gke/operator.yaml
	kustomize build config > examples/minikube/operator.yaml

# Run go fmt against code
fmt:
	go fmt ./pkg/... ./cmd/...

# Run go vet against code
vet:
	go vet ./pkg/... ./cmd/...

# Generate code
generate:
	go generate ./pkg/... ./cmd/...

# Ensure dependencies
vendor:
	dep ensure -v

# Build the docker image
docker-build: bin/deps
	GOVERSION="$(GOVERSION)" ./bin/deps/goreleaser --skip-validate --skip-publish --rm-dist

release: bin/deps
	GOVERSION="$(GOVERSION)" ./bin/deps/goreleaser --rm-dist

bin/deps:
	mkdir -p bin/deps
	hack/binary_deps.py bin/deps
