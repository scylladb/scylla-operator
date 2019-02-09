
# Image URL to use all building/pushing image targets
IMG ?= yanniszark/scylla-operator:master

all: test local-build

# Run tests
test: generate fmt vet manifests
	go test ./pkg/... ./cmd/... -coverprofile cover.out

# Build local-build binary
local-build: generate fmt vet
	go build -o bin/manager github.com/scylladb/scylla-operator/cmd

# Run against the configured Kubernetes cluster in ~/.kube/config
run: generate fmt vet
	go run ./cmd operator --image="$IMG" --enable-admission-webhook=false

# Install CRDs into a cluster
install: manifests
	kubectl apply -f config/crds

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy: install
	kubectl apply -f config/rbac
	kustomize build config/default | kubectl apply -f -

# Generate manifests e.g. CRD, RBAC etc.
manifests:
	go run vendor/sigs.k8s.io/controller-tools/cmd/controller-gen/main.go all
	kustomize build config/default > config/samples/operator.yaml

# Run go fmt against code
fmt:
	go fmt ./pkg/... ./cmd/...

# Run go vet against code
vet:
	go vet ./pkg/... ./cmd/...

# Generate code
generate:
	go generate ./pkg/... ./cmd/...

# Build the docker image
docker-build: test
	docker build . -t "$IMG"
	@echo "updating kustomize image patch file for manager resource"
	sed -i'' -e 's@image: .*@image: '"${IMG}"'@' ./config/default/manager_patch.yaml

# Push the docker image
docker-push:
	docker push "$IMG"

publish: docker-build docker-push
