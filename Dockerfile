FROM quay.io/scylladb/scylla-operator-images:golang-1.20 AS builder
WORKDIR /go/src/github.com/scylladb/scylla-operator
COPY . .
RUN make build --warn-undefined-variables

FROM quay.io/scylladb/scylla-operator-images:base-ubuntu-22.04
# sidecar-injection container and existing installations use binary from root,
# we have to keep it there until we figure out how to properly upgrade them.
COPY --from=builder /go/src/github.com/scylladb/scylla-operator/scylla-operator /usr/bin/
RUN ln -s /usr/bin/scylla-operator /scylla-operator
COPY --from=builder /go/src/github.com/scylladb/scylla-operator/scylla-operator-tests /usr/bin/
COPY ./hack/gke /usr/local/lib/scylla-operator/gke
ENTRYPOINT ["/usr/bin/scylla-operator"]
