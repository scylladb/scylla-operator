FROM quay.io/scylladb/scylla-operator-images:golang-1.25 AS builder

RUN groupadd -g 1001 scylla && \
    useradd -u 1001 -g scylla -m scylla

USER scylla:scylla

WORKDIR /go/src/github.com/scylladb/scylla-operator
COPY --chown=1001:1001 . .

RUN --mount=type=cache,target=/home/scylla/.cache/go-build,uid=1001,gid=1001 \
    --mount=type=cache,target=/go/pkg/mod,uid=1001,gid=1001 \
    make build --warn-undefined-variables

FROM quay.io/scylladb/scylla-operator-images:base-ubi-9.6-minimal

LABEL org.opencontainers.image.title="Scylla Operator" \
      org.opencontainers.image.description="ScyllaDB Operator for Kubernetes" \
      org.opencontainers.image.authors="ScyllaDB Operator Team" \
      org.opencontainers.image.source="https://github.com/scylladb/scylla-operator/" \
      org.opencontainers.image.documentation="https://operator.docs.scylladb.com" \
      org.opencontainers.image.url="https://hub.docker.com/r/scylladb/scylla-operator" \
      org.opencontainers.image.vendor="ScyllaDB" \
      name="Scylla Operator" \
      maintainer="ScyllaDB Operator Team" \
      vendor="ScyllaDB" \
      version="see-image-tag" \
      release="see-image-tag" \
      summary="ScyllaDB Operator for Kubernetes" \
      description="Easily run and manage your ScyllaDB cluster on Kubernetes."

RUN microdnf install -y procps-ng && \
    microdnf clean all && \
    rm -rf /var/cache/dnf/*

COPY --from=builder /go/src/github.com/scylladb/scylla-operator/scylla-operator /usr/bin/
COPY --from=builder /go/src/github.com/scylladb/scylla-operator/scylla-operator-tests /usr/bin/

ENTRYPOINT ["/usr/bin/scylla-operator"]
