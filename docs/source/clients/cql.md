# Using CQL

`cqlsh` is the CQL shell for ScyllaDB. You can learn more about it in [ScyllaDB documentation](https://opensource.docs.scylladb.com/stable/cql/cqlsh.html).

## Authentication and Authorization

For security reasons, you should always enable Authentication and Authorization.
At this point, this needs to be done manually in ScyllaDB Config.
You can find an example configuration bellow:

:::{code-block} yaml
   :emphasize-lines: 7-8

apiVersion: v1
kind: ConfigMap
metadata:
  name: scylla-config
data:
  scylla.yaml: |
    authenticator: PasswordAuthenticator
    authorizer: CassandraAuthorizer
:::

## Embedded cqlsh (aka localhost)

Every ScyllaDB node has an integrated `cqlsh` available. Here is an example of how it can be used:

::::{tab-set}
:::{tab-item} Any ScyllaDB Node
```bash
kubectl exec -it service/<sc-name>-client -c scylla -- cqlsh -u <user>
```
:::
:::{tab-item} Specific ScyllaDB Node
```bash
kubectl exec -it pod/<sc-name>-<datacenter>-<node-index> -c scylla -- cqlsh -u <user>
```
:::
::::
```text
Password: 
Connected to scylla at 127.0.0.1:9042
[cqlsh 6.2.0 | Scylla 5.4.0-0.20231205.58a89e7a4231 | CQL spec 3.3.1 | Native protocol v4]
Use HELP for help.
<user>@cqlsh>
```

## Remote cqlsh

This paragraph shows how to use `cqlsh` to remotely connect to a ScyllaDB node.
It is strongly recommended to access CQL over TLS connections on port `9142` instead of unencrypted `9042`.
Note that Scylla Operator sets up TLS certificates by default and makes them accessible in the Kubernetes API,
so the encrypted port `9142` works by default.

:::{caution}
In future releases the unencrypted port `9042` will be disabled by default, unless explicitly opted-in.  
:::

:::{caution}
To avoid unnecessary complexity, the following example simplifies how the credentials file is created.
Please create the credentials file with your text editor and avoid your password leaking into your bash history or environment variables.
To store the configuration permanently, please adjust `SCYLLADB_CONFIG` variable to an empty folder of your choice. 
:::

:::{note}
This example uses Service ClusterIP to connect to the ScyllaDB cluster. If you have configured the networking options differently,
you may need to adjust this endpoint. Please refer to [discovery documentation page](./discovery.md).
:::

```bash
SCYLLADB_CONFIG="$( mktemp -d )" 

cat <<EOF > "${SCYLLADB_CONFIG}/credentials"
[PlainTextAuthProvider]
username = <your_username>
password = <your_password>
EOF
chmod 600 "${SCYLLADB_CONFIG}/credentials"

SCYLLADB_DISCOVERY_EP="$( kubectl get service/<sc-name>-client -o='jsonpath={.spec.clusterIP}' )"
kubectl get configmap/<sc-name>-local-serving-ca -o='jsonpath={.data.ca-bundle\.crt}' > "${SCYLLADB_CONFIG}/serving-ca-bundle.crt"
kubectl get secret/<sc-name>-local-user-admin -o='jsonpath={.data.tls\.crt}' | base64 -d > "${SCYLLADB_CONFIG}/admin.crt"
kubectl get secret/<sc-name>-local-user-admin -o='jsonpath={.data.tls\.key}' | base64 -d > "${SCYLLADB_CONFIG}/admin.key"

cat <<EOF > "${SCYLLADB_CONFIG}/cqlshrc"
[authentication]
credentials = ${SCYLLADB_CONFIG}/credentials
[connection]
hostname = ${SCYLLADB_DISCOVERY_EP}
port = 9142
ssl=true
factory = cqlshlib.ssl.ssl_transport_factory
[ssl]
validate=true
certfile=${SCYLLADB_CONFIG}/serving-ca-bundle.crt
usercert=${SCYLLADB_CONFIG}/admin.crt
userkey=${SCYLLADB_CONFIG}/admin.key
EOF
```

::::{tab-set}
:::{tab-item} Native
```bash
cqlsh --cqlshrc="${SCYLLADB_CONFIG}/cqlshrc"
```
:::
:::{tab-item} Podman
```bash
podman run -it --rm --entrypoint=cqlsh \
-v="${SCYLLADB_CONFIG}:${SCYLLADB_CONFIG}:ro,Z" \
-v="${SCYLLADB_CONFIG}/cqlshrc:/root/.cassandra/cqlshrc:ro,Z" \
docker.io/scylladb/scylla:5.4.3
```
:::
:::{tab-item} Docker
```bash
docker run -it --rm --entrypoint=cqlsh \
-v="${SCYLLADB_CONFIG}:${SCYLLADB_CONFIG}:ro" \
-v="${SCYLLADB_CONFIG}/cqlshrc:/root/.cassandra/cqlshrc:ro" \
docker.io/scylladb/scylla:5.4.3
```
:::
::::
```text
Connected to scylla at <CLUSTER_IP>:9142
[cqlsh 6.2.0 | Scylla 5.4.0-0.20231205.58a89e7a4231 | CQL spec 3.3.1 | Native protocol v4]
Use HELP for help.
<your_username>@cqlsh> 
```
