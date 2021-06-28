# TODO: extract builder and base image into its own repo for reuse and to speed up builds
FROM registry.access.redhat.com/ubi8/ubi-minimal AS builder
ENV GOPATH=/go \
    GOROOT=/usr/local/go \
# Enable madvdontneed=1, for golang < 1.16 https://github.com/golang/go/issues/42330
    GODEBUG=madvdontneed=1
ENV PATH=$PATH:$GOROOT/bin:$GOPATH/bin
RUN set -euExo pipefail && \
    microdnf install -y make git curl tar && \
    microdnf clean all && \
    curl --fail -L https://storage.googleapis.com/golang/go1.16.4.linux-amd64.tar.gz | tar -C /usr/local -xzf -
WORKDIR /go/src/github.com/scylladb/scylla-operator
COPY . .
RUN make build --warn-undefined-variables

FROM registry.access.redhat.com/ubi8/ubi-minimal
RUN set -euExo pipefail && \
    microdnf install -y procps-ng && \
    microdnf clean all
# sidecar-injection container and existing installations use binary from root,
# we have to keep it there until we figure out how to properly upgrade them.
COPY --from=builder /go/src/github.com/scylladb/scylla-operator/scylla-operator /usr/bin/
RUN ln -s /usr/bin/scylla-operator /scylla-operator
COPY --from=builder /go/src/github.com/scylladb/scylla-operator/scylla-operator-tests /usr/bin/
ENTRYPOINT ["/usr/bin/scylla-operator"]
