# Using Alternator (DynamoDB)

Alternator is a DynamoDB compatible API provided by ScyllaDB.
You can enable it on your ScyllaClusters by adding this section:
```yaml
spec:
  alternator: {}
```
While this is enough to turn it on, there are more options available.
Please refer to our [API documentation](<api-scylla.scylladb.com-scyllaclusters-v1-.spec.alternator>) for details.

:::{note}
Contrary to CQL clients, Alternator clients don't need to connect to every ScyllaDB node directly, nor discover the ScyllaDB node IP addresses.
Alternator protocol is based on HTTP and you can also expose the service "manually" with other networking concepts like Ingresses.
:::

## Credentials

Scylla Operator enables Alternator authorization by default. 
Here is a quick example of how to get the token for accessing Alternator API.
To find out more, please refer to [ScyllaDB Alternator documentation](https://opensource.docs.scylladb.com/stable/alternator/compatibility.html#authorization).

:::{caution}
The `salted_hash` is only present if password authentication for CQL is set up.

Always make sure your clusters are configured to use Authentication and Authorization. 
:::

:::{tip}
You can find a quick example that enables Authentication and Authorization [here](./cql.md#authentication-and-authorization).
:::

```bash
kubectl exec -it service/<sc-name>-client -c scylla -- cqlsh --user <cql_user> \
-e "SELECT salted_hash FROM system_auth.roles WHERE role = '<cql_user>'"
```

## AWS CLI

This paragraph shows how to use `aws dynamodb` cli to remotely connect to ScyllaDB Alternator API.

:::{note}
This example uses Service ClusterIP to connect to the ScyllaDB cluster. If you have configured networking options differently,
or are using additional networking concepts like Ingresses, this address will need to be adjusted.
:::
:::{caution}
At the time of writing this document `kubectl exec -i` echoes passwords into the terminal.
It can be avoided by manually running `kubectl exec -it` and copying the output into a file / variable.
Because using `kubectl exec` with `-t` option merges standard and error outputs, we can't use it in the scripts bellow.

See <https://github.com/kubernetes/kubernetes/issues/123913> for more details.
:::

```bash
SCYLLACLUSTER_NAME=scylladb
CQL_USER=cassandra
```
```bash
SCYLLADB_EP="$( kubectl get "service/${SCYLLACLUSTER_NAME}-client" -o='jsonpath={.spec.clusterIP}' )"
AWS_ENDPOINT_URL_DYNAMODB="https://${SCYLLADB_EP}:8043"
export AWS_ENDPOINT_URL_DYNAMODB

AWS_ACCESS_KEY_ID="${CQL_USER}"
export AWS_ACCESS_KEY_ID
AWS_SECRET_ACCESS_KEY="$( kubectl exec -i "service/${SCYLLACLUSTER_NAME}-client" -c scylla -- cqlsh --user ${CQL_USER} --no-color \
-e "SELECT salted_hash from system_auth.roles WHERE role = '${AWS_ACCESS_KEY_ID}';" \
| sed -e 's/\r//g' | sed -e '4q;d' | sed -E -e 's/^\s+//' )"
export AWS_SECRET_ACCESS_KEY

AWS_CA_BUNDLE="$( mktemp )"
export AWS_CA_BUNDLE
kubectl get "configmap/${SCYLLACLUSTER_NAME}-alternator-local-serving-ca" --template='{{ index .data "ca-bundle.crt" }}' > "${AWS_CA_BUNDLE}"
```

Now we can use `aws dynamodb` cli without modifications.

```bash
aws dynamodb create-table --table-name MusicCollection --attribute-definitions AttributeName=Artist,AttributeType=S AttributeName=SongTitle,AttributeType=S --key-schema AttributeName=Artist,KeyType=HASH AttributeName=SongTitle,KeyType=RANGE --provisioned-throughput ReadCapacityUnits=5,WriteCapacityUnits=5
```
```text
TABLEDESCRIPTION        2024-03-01T16:35:41+01:00       5c8aae70-d7e1-11ee-a99e-6f31aaf1d6d3    MusicCollection ACTIVE
ATTRIBUTEDEFINITIONS    Artist  S
ATTRIBUTEDEFINITIONS    SongTitle       S
KEYSCHEMA       Artist  HASH
KEYSCHEMA       SongTitle       RANGE
PROVISIONEDTHROUGHPUT   5       5
```

```bash
aws dynamodb list-tables
```
```text
TABLENAMES      MusicCollection
```
