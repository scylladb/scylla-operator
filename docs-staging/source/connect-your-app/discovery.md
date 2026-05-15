# Discovery endpoint

This page explains how ScyllaDB clusters are discoverable by clients on Kubernetes and how to expose the discovery endpoint beyond the cluster boundary.

## How discovery works

For every ScyllaCluster, the Operator creates a Kubernetes Service named `<cluster-name>-client` that matches all ScyllaDB Pods in the cluster by label. Kubernetes routes traffic only to Pods that pass their readiness probe. This Service acts as a stable entry point: clients connect to it to reach any available ScyllaDB node, and from there the driver automatically discovers all other nodes in the cluster.

```shell
kubectl get scyllacluster/scylladb service/scylladb-client
```

```
NAME                                          READY   MEMBERS   RACKS   AVAILABLE
scyllacluster.scylla.scylladb.com/scylladb    1       1         1       True

NAME                      TYPE        CLUSTER-IP     EXTERNAL-IP   PORT(S)
service/scylladb-client   ClusterIP   10.102.44.43   <none>        7000/TCP,7001/TCP,9042/TCP,9142/TCP,19042/TCP,19142/TCP,7199/TCP,10001/TCP,9180/TCP,5090/TCP,9100/TCP,9160/TCP,8043/TCP
```

### Using the endpoint

You can reach the discovery endpoint by:

- **ClusterIP**: `kubectl get service/<cluster-name>-client -o='jsonpath={.spec.clusterIP}'`
- **DNS name**: `<cluster-name>-client.<namespace>.svc` (for in-cluster clients)

Clients should use this endpoint as their initial contact point. The driver connects to one of the ready nodes, fetches the cluster topology, and establishes connections to all nodes.

## Exposing beyond the Kubernetes cluster

If you need to connect from outside the Kubernetes cluster and are using Pod IPs as the broadcast address type, you can expose the discovery endpoint by creating a separate LoadBalancer Service that selects the same Pods. Do **not** patch the operator-managed `<cluster-name>-client` Service directly — the operator reconciles it and will revert manual changes.

::::{tabs}
:::{group-tab} GKE
```yaml
apiVersion: v1
kind: Service
metadata:
  name: <cluster-name>-client-external
  namespace: scylla
  annotations:
    networking.gke.io/load-balancer-type: Internal
spec:
  type: LoadBalancer
  selector:
    scylla/cluster: <cluster-name>
    app: scylla
    scylla-operator.scylladb.com/pod-type: scylladb-node
  ports:
  - name: cql
    port: 9042
    targetPort: 9042
  - name: cql-ssl
    port: 9142
    targetPort: 9142
```

```shell
kubectl wait --for=jsonpath='{.status.loadBalancer.ingress}' service/<cluster-name>-client-external -n scylla
kubectl get service/<cluster-name>-client-external -n scylla -o='jsonpath={.status.loadBalancer.ingress[0].ip}'
```
:::

:::{group-tab} EKS
```yaml
apiVersion: v1
kind: Service
metadata:
  name: <cluster-name>-client-external
  namespace: scylla
  annotations:
    service.beta.kubernetes.io/aws-load-balancer-scheme: internal
    service.beta.kubernetes.io/aws-load-balancer-backend-protocol: tcp
spec:
  type: LoadBalancer
  selector:
    scylla/cluster: <cluster-name>
    app: scylla
    scylla-operator.scylladb.com/pod-type: scylladb-node
  ports:
  - name: cql
    port: 9042
    targetPort: 9042
  - name: cql-ssl
    port: 9142
    targetPort: 9142
```

```shell
kubectl wait --for=jsonpath='{.status.loadBalancer.ingress}' service/<cluster-name>-client-external -n scylla
kubectl get service/<cluster-name>-client-external -n scylla -o='jsonpath={.status.loadBalancer.ingress[0].hostname}'
```
:::
::::

:::{tip}
Having a stable discovery contact point is especially important when using ephemeral Pod IPs, because individual node IPs can change when Pods are rescheduled.
:::

## Related pages

- [Connect via CQL](connect-via-cql.md) — using the discovery endpoint for CQL connections.
- [Configure external access](configure-external-access.md) — configuring expose options for external connectivity.
- [Networking architecture](../understand/networking.md) — how Services and expose options work.
