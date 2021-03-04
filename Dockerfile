# TODO: extract builder and base image into its own repo for reuse and to speed up builds
FROM docker.io/library/ubuntu:20.04 AS builder
SHELL ["/bin/bash", "-euEo", "pipefail", "-c"]
ENV GOPATH=/go \
    GOROOT=/usr/local/go \
# Enable madvdontneed=1, for golang < 1.16 https://github.com/golang/go/issues/42330
    GODEBUG=madvdontneed=1
ENV PATH=$PATH:$GOROOT/bin:$GOPATH/bin
RUN apt-get update; \
    apt-get install -y --no-install-recommends build-essential git curl gzip ca-certificates; \
    apt-get clean; \
    curl --fail -L https://storage.googleapis.com/golang/go1.16.linux-amd64.tar.gz | tar -C /usr/local -xzf -
WORKDIR /go/src/github.com/scylladb/scylla-operator
COPY . .
RUN make build --warn-undefined-variables

FROM docker.io/library/ubuntu:20.04
SHELL ["/bin/bash", "-euEo", "pipefail", "-c"]
# sidecar-injection container and existing installations use binary from root,
# we have to keep it there until we figure out how to properly upgrade them.
COPY --from=builder /go/src/github.com/scylladb/scylla-operator/scylla-operator /usr/bin/
RUN ln -s /usr/bin/scylla-operator /scylla-operator
COPY --from=builder /go/src/github.com/scylladb/scylla-operator/scylla-operator-tests /usr/bin/
ENTRYPOINT ["/usr/bin/scylla-operator"]
