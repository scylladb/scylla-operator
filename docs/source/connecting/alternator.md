# Alternator (DynamoDB API)

This page explains how to enable and use ScyllaDB's Alternator, a DynamoDB-compatible API, on Kubernetes.

## Enable Alternator

Add the `alternator` section to your ScyllaCluster spec:

```yaml
apiVersion: scylla.scylladb.com/v1
kind: ScyllaCluster
metadata:
  name: scylladb
spec:
  alternator: {}
  # ... rest of the spec
```

This enables the Alternator API with HTTPS on port `8043` and authorization enabled by default.

### Configuration options

| Field | Description | Default |
|-------|-------------|---------|
| `writeIsolation` | Write isolation level for Alternator operations. | `""` (ScyllaDB default) |
| `insecureEnableHTTP` | Also serve Alternator on the unencrypted HTTP port. | `false` |
| `insecureDisableAuthorization` | Disable Alternator authorization. | `false` |
| `servingCertificate` | TLS certificate configuration (`OperatorManaged` or `UserManaged`). | `OperatorManaged` |

:::{note}
Unlike CQL clients, Alternator clients do not need to connect to every ScyllaDB node directly or discover individual node IP addresses. The Alternator protocol is HTTP-based, so you can also expose it through an Ingress or other HTTP networking concepts.
:::

## Obtain credentials

Alternator uses the CQL `salted_hash` from `system_auth.roles` as the AWS secret access key. The access key ID is the CQL username.

:::{caution}
The `salted_hash` is only available when CQL password authentication is enabled. Always configure authentication before using Alternator.
:::

```shell
CLUSTER_NAME=scylladb
CQL_USER=cassandra

kubectl exec -it service/${CLUSTER_NAME}-client -c scylla -- cqlsh --user ${CQL_USER} \
  -e "SELECT salted_hash FROM system_auth.roles WHERE role = '${CQL_USER}'"
```

## Connect with AWS CLI

Set up the environment variables and TLS CA bundle:

```shell
CLUSTER_NAME=scylladb
CQL_USER=cassandra

SCYLLADB_EP="$(kubectl get service/${CLUSTER_NAME}-client -o='jsonpath={.spec.clusterIP}')"
export AWS_ENDPOINT_URL_DYNAMODB="https://${SCYLLADB_EP}:8043"
export AWS_ACCESS_KEY_ID="${CQL_USER}"

AWS_SECRET_ACCESS_KEY="$(kubectl exec -i service/${CLUSTER_NAME}-client -c scylla -- cqlsh --user ${CQL_USER} --no-color \
  -e "SELECT salted_hash from system_auth.roles WHERE role = '${AWS_ACCESS_KEY_ID}';" \
  | sed -e 's/\r//g' | sed -e '4q;d' | sed -E -e 's/^\s+//')"
export AWS_SECRET_ACCESS_KEY

AWS_CA_BUNDLE="$(mktemp)"
export AWS_CA_BUNDLE
kubectl get configmap/${CLUSTER_NAME}-alternator-local-serving-ca \
  --template='{{ index .data "ca-bundle.crt" }}' > "${AWS_CA_BUNDLE}"
```

Now use the `aws dynamodb` CLI normally:

```shell
aws dynamodb create-table \
  --table-name MusicCollection \
  --attribute-definitions AttributeName=Artist,AttributeType=S AttributeName=SongTitle,AttributeType=S \
  --key-schema AttributeName=Artist,KeyType=HASH AttributeName=SongTitle,KeyType=RANGE \
  --provisioned-throughput ReadCapacityUnits=5,WriteCapacityUnits=5
```

```shell
aws dynamodb list-tables
```

```
TABLENAMES      MusicCollection
```

## TLS certificate resources

The Operator creates these resources for Alternator:

| Resource | Name | Contents |
|----------|------|----------|
| Serving CA | `configmap/<cluster-name>-alternator-local-serving-ca` | `ca-bundle.crt` — CA to validate Alternator HTTPS. |

## Related pages

- [Connecting via CQL](cql.md) — CQL connection and authentication setup.
- [Discovery endpoint](discovery.md) — how the client Service works.
- [Security](../architecture/security.md) — TLS certificate management.
