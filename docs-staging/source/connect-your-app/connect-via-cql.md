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

After deployment, follow the [Creating a Custom Superuser](https://docs.scylladb.com/manual/stable/operating-scylla/security/create-superuser.html) guide in the ScyllaDB documentation to replace the default `cassandra` superuser with a dedicated role.

:::{caution}
Starting with ScyllaDB 2026.2, the default `cassandra`/`cassandra` superuser credentials are being removed. New clusters will require configuring a custom superuser via `auth_superuser_name` and `auth_superuser_salted_password` in `scylla.yaml`. See [scylladb/scylladb#27215](https://github.com/scylladb/scylladb/pull/27215) for details.
:::

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
Connected to scylla at 0.0.0.0:9042
[cqlsh 6.0.32 | Scylla 2026.1.0-0.20260309.9190d42863d4 | CQL spec 3.3.1 | Native protocol v4]
Use HELP for help.
<user>@cqlsh>
```

## Remote cqlsh with TLS

ScyllaDB Operator configures TLS certificates automatically. The encrypted CQL port `9142` works by default.

### Prepare credentials and certificates

:::{caution}
The example below simplifies credential file creation for brevity. In production, create the credentials file with a text editor to avoid passwords leaking into shell history or environment variables.
:::

**Step 1: Identify the serving CA ConfigMap**

The serving CA is stored in a ConfigMap named `<cluster-name>-local-serving-ca`:
```bash
kubectl -n scylla get configmaps | grep serving-ca
```

**Step 2: Extract the CA certificate**

```bash
kubectl -n scylla get configmap scylla-local-serving-ca \
  --template='{{ index .data "ca-bundle.crt" }}' > ca.crt
```
Replace `scylla-local-serving-ca` with the actual ConfigMap name from Step 1.

**Step 3: Extract client credentials**

```bash
kubectl -n scylla get secret scylla-local-user-admin \
  -o jsonpath='{.data.tls\.crt}' | base64 -d > client.crt
kubectl -n scylla get secret scylla-local-user-admin \
  -o jsonpath='{.data.tls\.key}' | base64 -d > client.key
```

**Step 4: Verify the extracted files**

```bash
openssl x509 -in ca.crt -noout -subject -dates
openssl x509 -in client.crt -noout -subject -dates
```

**Step 5: Create a cqlshrc file**

Set `SCYLLADB_CONFIG` to a directory containing the certificates and create a `cqlshrc` file:

```bash
export SCYLLADB_CONFIG=/path/to/scylladb-config
mkdir -p "${SCYLLADB_CONFIG}"
cp ca.crt client.crt client.key "${SCYLLADB_CONFIG}/"
cat > "${SCYLLADB_CONFIG}/cqlshrc" <<EOF
[connection]
hostname = <cluster-name>-client.<namespace>.svc
port = 9142
ssl = true

[ssl]
certfile = ${SCYLLADB_CONFIG}/ca.crt
usercert = ${SCYLLADB_CONFIG}/client.crt
userkey = ${SCYLLADB_CONFIG}/client.key
validate = true

[authentication]
username = cassandra
password = <password>
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

For more details, see [TLS and certificate management](../understand/security.md).

## Related pages

- [Discovery endpoint](discovery.md) — how the client Service works and how to expose it.
- [Alternator (DynamoDB API)](alternator.md) — connecting via the DynamoDB-compatible API.
- [Configure external access](configure-external-access.md) — connecting from outside the Kubernetes cluster.
- [Security](../understand/security.md) — TLS certificate management.
