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

Reference this ConfigMap from your ScyllaCluster via `scyllaConfig` on each rack:

```yaml
apiVersion: scylla.scylladb.com/v1
kind: ScyllaCluster
metadata:
  name: scylladb
spec:
  datacenter:
    racks:
    - name: us-east-1a
      scyllaConfig: scylladb-config
      # ...
```

After deployment, follow the [Creating a Custom Superuser](https://docs.scylladb.com/manual/stable/operating-scylla/security/create-superuser.html) guide in the ScyllaDB documentation to replace the default `cassandra` superuser with a dedicated role.

## Embedded cqlsh

Every ScyllaDB Pod includes a built-in `cqlsh`. This is the simplest way to run queries:

Any node (via Service)

```shell
kubectl exec -it service/<cluster-name>-client -c scylla -- cqlsh -u <user>
```

Specific node

```shell
kubectl exec -it pod/<cluster-name>-<datacenter>-<rack>-<ordinal> -c scylla -- cqlsh -u <user>
```

```default
Password:
Connected to scylla at 0.0.0.0:9042
[cqlsh 6.0.32 | Scylla 2026.1.0-0.20260309.9190d42863d4 | CQL spec 3.3.1 | Native protocol v4]
Use HELP for help.
<user>@cqlsh>
```

## Remote cqlsh with TLS

ScyllaDB Operator configures TLS certificates automatically. The encrypted CQL port `9142` works by default.

### Prepare credentials and certificates

The Operator automatically creates TLS resources for each ScyllaCluster. For programmatic access using the Go driver with clusters that have `dnsDomains` configured, use the pre-built connection bundle from `secret/<cluster-name>-local-cql-connection-configs-admin` directly (see [Security](https://operator.docs.scylladb.com/master/understand/security.md) for details).

For `cqlsh`, extract the certificates and create a `cqlshrc` file:

**Step 1: Extract certificates**

```bash
kubectl -n scylla get configmap <cluster-name>-local-serving-ca \
  --template='{{ index .data "ca-bundle.crt" }}' > ca.crt
kubectl -n scylla get secret <cluster-name>-local-user-admin \
  -o jsonpath='{.data.tls\.crt}' | base64 -d > client.crt
kubectl -n scylla get secret <cluster-name>-local-user-admin \
  -o jsonpath='{.data.tls\.key}' | base64 -d > client.key
```

**Step 2: Create a cqlshrc file**

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

Native

```shell
cqlsh --cqlshrc="${SCYLLADB_CONFIG}/cqlshrc"
```

Podman

```shell
podman run -it --rm --entrypoint=cqlsh \
  -v="${SCYLLADB_CONFIG}:${SCYLLADB_CONFIG}:ro,Z" \
  -v="${SCYLLADB_CONFIG}/cqlshrc:/root/.cassandra/cqlshrc:ro,Z" \
  docker.io/scylladb/scylla:2026.2.4
```

Docker

```shell
docker run -it --rm --entrypoint=cqlsh \
  -v="${SCYLLADB_CONFIG}:${SCYLLADB_CONFIG}:ro" \
  -v="${SCYLLADB_CONFIG}/cqlshrc:/root/.cassandra/cqlshrc:ro" \
  docker.io/scylladb/scylla:2026.2.4
```

## Driver configuration tips

When using a ScyllaDB or Cassandra driver in your application:

| Setting          | Recommended value                                                      | Why                                                                                              |
|------------------|------------------------------------------------------------------------|--------------------------------------------------------------------------------------------------|
| Contact points   | `<cluster-name>-client.<namespace>.svc` (DNS) or the Service ClusterIP | Use the discovery Service, not individual Pod IPs. The driver discovers all nodes automatically. |
| Local datacenter | Your `datacenter.name` value (e.g., `us-east-1`)                       | Required for `DCAwareRoundRobinPolicy`. Prevents cross-DC queries.                               |
| Load balancing   | Token-aware + DC-aware round robin                                     | Sends queries directly to the replica owning the partition.                                      |
| TLS              | Enabled, with CA verification                                          | Use the serving CA from `configmap/<cluster-name>-local-serving-ca`.                             |
| Reconnection     | Exponential backoff                                                    | Handles node restarts during rolling updates.                                                    |

## Related pages

- [Discovery endpoint](https://operator.docs.scylladb.com/master/connect-your-app/discovery.md) — how the client Service works and how to expose it.
- [Alternator (DynamoDB API)](https://operator.docs.scylladb.com/master/connect-your-app/alternator.md) — connecting via the DynamoDB-compatible API.
- [Configure external access](https://operator.docs.scylladb.com/master/deploy-scylladb/set-up-networking/configure-external-access.md) — connecting from outside the Kubernetes cluster.
- [Security](https://operator.docs.scylladb.com/master/understand/security.md) — TLS certificate management.
