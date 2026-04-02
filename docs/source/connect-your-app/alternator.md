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

**Step 1: Look up the Alternator endpoint**
```shell
CLUSTER_NAME=scylladb
CQL_USER=cassandra

SCYLLADB_EP="$(kubectl get service/${CLUSTER_NAME}-client -o='jsonpath={.spec.clusterIP}')"
export AWS_ENDPOINT_URL_DYNAMODB="https://${SCYLLADB_EP}:8043"
```

**Step 2: Set the access key ID**
```shell
export AWS_ACCESS_KEY_ID="${CQL_USER}"
```

**Step 3: Get the secret access key**
```shell
AWS_SECRET_ACCESS_KEY="$(kubectl exec -i service/${CLUSTER_NAME}-client -c scylla -- cqlsh --user ${CQL_USER} --no-color \
  -e "SELECT salted_hash from system_auth.roles WHERE role = '${AWS_ACCESS_KEY_ID}';" \
  | sed -e 's/\r//g' | sed -e '4q;d' | sed -E -e 's/^\s+//')"
export AWS_SECRET_ACCESS_KEY
```

**Step 4: Download the TLS CA bundle**
```shell
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

### Troubleshoot

- **`AccessDeniedException`**: Confirms Alternator is reachable but credentials are wrong. Re-extract credentials per the steps above and verify they match.
- **`Could not connect to the endpoint`**: Alternator may not be enabled on the cluster. Verify `spec.alternator` is set in the ScyllaCluster and pods are running. Check that the Service port (8000 by default) is reachable.
- **`salted_hash not set`**: The credentials were created without the required `options={'timeout': 300}` parameter or were created before Alternator was enabled. Recreate the credentials via CQL.

## Multi-datacenter limitations

When using Alternator with a multi-datacenter ScyllaDB deployment (multiple `ScyllaCluster` resources connected via `externalSeeds`), the following constraints apply:

| Limitation | Detail |
|---|---|
| No built-in cross-DC routing | Alternator endpoints are per-datacenter. There is no built-in load balancer that routes DynamoDB API requests across datacenters. Connect your application to the Alternator endpoint in the datacenter closest to it. |
| No Helm-based multi-DC install | Helm charts do not support multi-DC deployments. Use manifests directly for multi-DC setups. |
| Authentication tokens are DC-local | Each `ScyllaCluster` has its own Alternator authentication credentials. If you require the same credentials across DCs, you must configure the same `alternatorWriteIsolation` and authentication settings on each cluster independently. |

See [Known issues](../reference/known-issues.md#multi-datacenter-limitations) for the complete list of multi-datacenter limitations in ScyllaDB Operator.

## Related pages

- [Connect via CQL](connect-via-cql.md) — CQL connection and authentication setup.
- [Discovery endpoint](discovery.md) — how the client Service works.
- [Security](../understand/security.md) — TLS certificate management.
