# Connect via CQL

This page explains how to connect to a ScyllaDB cluster running on Kubernetes using CQL (Cassandra Query Language).

## Authentication setup

For security, always enable authentication and authorization. Create a ConfigMap with the ScyllaDB configuration before deploying your cluster:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: scylladb-config
data:
  scylla.yaml: |
    authenticator: PasswordAuthenticator
    authorizer: CassandraAuthorizer
```

Reference this ConfigMap from your ScyllaCluster via `scyllaConfig` on each rack.

## Embedded cqlsh

Every ScyllaDB pod includes a built-in `cqlsh`. This is the simplest way to run queries:

::::{tabs}
:::{group-tab} Any node (via Service)
```shell
kubectl exec -it service/<cluster-name>-client -c scylla -- cqlsh -u <user>
```
:::

:::{group-tab} Specific node
```shell
kubectl exec -it pod/<cluster-name>-<datacenter>-<rack>-<ordinal> -c scylla -- cqlsh -u <user>
```
:::
::::

```
Password:
Connected to scylla at 127.0.0.1:9042
[cqlsh 6.2.0 | Scylla 2025.4.2 | CQL spec 3.3.1 | Native protocol v4]
Use HELP for help.
<user>@cqlsh>
```

## Remote cqlsh with TLS

ScyllaDB Operator configures TLS certificates automatically. The encrypted CQL port `9142` works by default.

:::{caution}
In future releases, the unencrypted CQL port `9042` will be disabled by default unless explicitly opted in. Always use the TLS port `9142` for remote connections.
:::

### Prepare credentials and certificates

:::{caution}
The example below simplifies credential file creation for brevity. In production, create the credentials file with a text editor to avoid passwords leaking into shell history or environment variables.
:::

```shell
SCYLLADB_CONFIG="$(mktemp -d)"
CLUSTER_NAME=scylladb

# Create credentials file
cat <<EOF > "${SCYLLADB_CONFIG}/credentials"
[PlainTextAuthProvider]
username = <your_username>
password = <your_password>
EOF
chmod 600 "${SCYLLADB_CONFIG}/credentials"

# Get the discovery endpoint
SCYLLADB_DISCOVERY_EP="$(kubectl get service/${CLUSTER_NAME}-client -o='jsonpath={.spec.clusterIP}')"

# Extract TLS certificates
kubectl get configmap/${CLUSTER_NAME}-local-serving-ca \
  -o='jsonpath={.data.ca-bundle\.crt}' > "${SCYLLADB_CONFIG}/serving-ca-bundle.crt"
kubectl get secret/${CLUSTER_NAME}-local-user-admin \
  -o='jsonpath={.data.tls\.crt}' | base64 -d > "${SCYLLADB_CONFIG}/admin.crt"
kubectl get secret/${CLUSTER_NAME}-local-user-admin \
  -o='jsonpath={.data.tls\.key}' | base64 -d > "${SCYLLADB_CONFIG}/admin.key"

# Create cqlshrc
cat <<EOF > "${SCYLLADB_CONFIG}/cqlshrc"
[authentication]
credentials = ${SCYLLADB_CONFIG}/credentials
[connection]
hostname = ${SCYLLADB_DISCOVERY_EP}
port = 9142
ssl = true
factory = cqlshlib.ssl.ssl_transport_factory
[ssl]
validate = true
certfile = ${SCYLLADB_CONFIG}/serving-ca-bundle.crt
usercert = ${SCYLLADB_CONFIG}/admin.crt
userkey = ${SCYLLADB_CONFIG}/admin.key
EOF
```

### Connect

::::{tabs}
:::{group-tab} Native
```shell
cqlsh --cqlshrc="${SCYLLADB_CONFIG}/cqlshrc"
```
:::

:::{group-tab} Podman
```shell
podman run -it --rm --entrypoint=cqlsh \
  -v="${SCYLLADB_CONFIG}:${SCYLLADB_CONFIG}:ro,Z" \
  -v="${SCYLLADB_CONFIG}/cqlshrc:/root/.cassandra/cqlshrc:ro,Z" \
  docker.io/scylladb/scylla:2025.4.2
```
:::

:::{group-tab} Docker
```shell
docker run -it --rm --entrypoint=cqlsh \
  -v="${SCYLLADB_CONFIG}:${SCYLLADB_CONFIG}:ro" \
  -v="${SCYLLADB_CONFIG}/cqlshrc:/root/.cassandra/cqlshrc:ro" \
  docker.io/scylladb/scylla:2025.4.2
```
:::
::::

## Driver configuration tips

When using a ScyllaDB or Cassandra driver in your application:

| Setting | Recommended value | Why |
|---------|-------------------|-----|
| Contact points | `<cluster-name>-client.<namespace>.svc` (DNS) or the Service ClusterIP | Use the discovery Service, not individual pod IPs. The driver discovers all nodes automatically. |
| Local datacenter | Your `datacenter.name` value (e.g., `us-east-1`) | Required for `DCAwareRoundRobinPolicy`. Prevents cross-DC queries. |
| Load balancing | Token-aware + DC-aware round robin | Sends queries directly to the replica owning the partition. |
| TLS | Enabled, with CA verification | Use the serving CA from `configmap/<cluster-name>-local-serving-ca`. |
| Reconnection | Exponential backoff | Handles node restarts during rolling updates. |

## TLS certificate resources

The Operator creates these resources automatically:

| Resource | Name | Contents |
|----------|------|----------|
| Serving CA | `configmap/<cluster-name>-local-serving-ca` | `ca-bundle.crt` — CA to validate server certificates. |
| Admin client cert | `secret/<cluster-name>-local-user-admin` | `tls.crt`, `tls.key` — client certificate for mTLS. |

To use your own TLS certificates instead of operator-managed ones, set `servingCertificate.type: UserManaged` on the ScyllaCluster. See [TLS and certificate management](../understand/security.md).

## Related pages

- [Discovery endpoint](discovery.md) — how the client Service works and how to expose it.
- [Alternator (DynamoDB API)](alternator.md) — connecting via the DynamoDB-compatible API.
- [Configure external access](configure-external-access.md) — connecting from outside the Kubernetes cluster.
- [Security](../understand/security.md) — TLS certificate management.
