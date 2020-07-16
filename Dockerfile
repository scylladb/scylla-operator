# Build the manager binary
FROM golang:1.13 as builder

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy source
COPY . .

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o scylla-operator github.com/scylladb/scylla-operator/pkg/cmd

FROM alpine:3.12
WORKDIR /
COPY --from=builder /workspace/scylla-operator .

ENTRYPOINT ["/scylla-operator"]
