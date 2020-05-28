all: test local-build

# Image URL to use all building/pushing image targets
REPO	?= scylladb/scylla-operator
TAG		?= $(shell git describe --tags --always)
IMG		?= $(REPO):$(TAG)

.EXPORT_ALL_VARIABLES:
DOCKER_BUILDKIT		:= 1
GO111MODULE			:= off
KUBEBUILDER_ASSETS	:= $(CURDIR)/bin/deps
PATH				:= $(CURDIR)/bin/deps:$(CURDIR)/bin/deps/go/bin:$(PATH)
GOROOT				:= $(CURDIR)/bin/deps/go
GOVERSION			:= $(shell go version)

# Run tests
.PHONY: test
test: fmt vet vendor
	go test ./pkg/... ./cmd/... -coverprofile cover.out

# Build local-build binary
.PHONY: local-build
local-build: fmt vet vendor
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o bin/manager github.com/scylladb/scylla-operator/cmd

# Run against the configured Kubernetes cluster in ~/.kube/config
.PHONY: run
run: fmt vet vendor
	go run ./cmd operator --image="$(IMG)" --enable-admission-webhook=false

# Install CRDs into a cluster
.PHONY: install
install: manifests
	kubectl apply -f config/crds

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
.PHONY: deploy
deploy: install
	kubectl apply -f config/rbac
	kustomize build config | kubectl apply -f -

# Generate manifests e.g. CRD, RBAC etc.
.PHONY: manifests
manifests: bin/deps
	go run vendor/sigs.k8s.io/controller-tools/cmd/controller-gen/main.go all
	cd config && kustomize edit set image scylladb/scylla-operator="$(IMG)"
	kustomize build config > examples/generic/operator.yaml
	kustomize build config > examples/gke/operator.yaml

# Run go fmt against code
.PHONY: fmt
fmt: bin/deps
	go fmt ./pkg/... ./cmd/...

# Run go vet against code
.PHONY: vet
vet: bin/deps
	go vet ./pkg/... ./cmd/...

# Generate code
.PHONY: generate
generate: bin/deps
	go generate ./pkg/... ./cmd/...

# Ensure dependencies
.PHONY: vendor
vendor:
	dep ensure -v

# Build the docker image
.PHONY: docker-build
docker-build: bin/deps
	goreleaser --skip-validate --skip-publish --rm-dist

release: bin/deps
	goreleaser --rm-dist

bin/deps: hack/binary_deps.py
	mkdir -p bin/deps
	hack/binary_deps.py bin/deps
