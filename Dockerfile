FROM alpine:3.12
WORKDIR /
COPY scylla-operator .
ENTRYPOINT ["/scylla-operator"]
