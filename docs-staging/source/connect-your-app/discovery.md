# Discovery endpoint

This page explains how ScyllaDB clusters are discoverable by clients on Kubernetes and how to expose the discovery endpoint beyond the cluster boundary.

## How discovery works

For every ScyllaCluster, the Operator creates a Kubernetes Service named `<cluster-name>-client` that selects all ready ScyllaDB nodes. This Service acts as a stable entry point: clients connect to it to reach any available ScyllaDB node, and from there the driver automatically discovers all other nodes in the cluster.

```shell
kubectl get scyllacluster/scylladb service/scylladb-client
```

```
NAME                                          READY   MEMBERS   RACKS   AVAILABLE
scyllacluster.scylla.scylladb.com/scylladb    1       1         1       True

NAME                      TYPE        CLUSTER-IP     EXTERNAL-IP   PORT(S)
service/scylladb-client   ClusterIP   10.102.44.43   <none>        7000/TCP,7001/TCP,9042/TCP,9142/TCP,...
```

### Using the endpoint

You can reach the discovery endpoint by:

- **ClusterIP**: `kubectl get service/<cluster-name>-client -o='jsonpath={.spec.clusterIP}'`
- **DNS name**: `<cluster-name>-client.<namespace>.svc` (for in-cluster clients)

Clients should use this endpoint as their initial contact point. The driver connects to one of the ready nodes, fetches the cluster topology, and establishes connections to all nodes.

## Exposing beyond the Kubernetes cluster

If you need to connect from outside the Kubernetes cluster and are using Pod IPs as the broadcast address type, you can expose the `<cluster-name>-client` Service using a cloud provider's internal load balancer.

::::{tabs}
:::{group-tab} GKE
```shell
kubectl patch service/<cluster-name>-client -p '{"metadata": {"annotations": {"networking.gke.io/load-balancer-type": "Internal"}}, "spec": {"type": "LoadBalancer"}}'
kubectl wait --for=jsonpath='{.status.loadBalancer.ingress}' service/<cluster-name>-client
kubectl get service/<cluster-name>-client -o='jsonpath={.status.loadBalancer.ingress[0].ip}'
```

Expected output:
```
10.128.0.5
```
:::

:::{group-tab} EKS
```shell
kubectl patch service/<cluster-name>-client -p '{"metadata": {"annotations": {"service.beta.kubernetes.io/aws-load-balancer-scheme": "internal", "service.beta.kubernetes.io/aws-load-balancer-backend-protocol": "tcp"}}, "spec": {"type": "LoadBalancer"}}'
kubectl wait --for=jsonpath='{.status.loadBalancer.ingress}' service/<cluster-name>-client
kubectl get service/<cluster-name>-client -o='jsonpath={.status.loadBalancer.ingress[0].hostname}'
```

Expected output:
```
internal-a1b2c3d4e5f6g7h8-123456789.us-east-1.elb.amazonaws.com
```
:::
::::

:::{tip}
Having a stable discovery contact point is especially important when using ephemeral Pod IPs, because individual node IPs can change when pods are rescheduled.
:::

## Troubleshoot discovery failures

**Discovery Service has no EXTERNAL-IP**: The LoadBalancer has not yet been provisioned. Wait 1–2 minutes and retry, or check cloud provider events with `kubectl describe service <name> -n scylla`.

**Clients cannot reach discovery endpoint**: Verify that firewall rules allow traffic on port 9042. See [Prerequisites](../install-operator/prerequisites.md).

**Driver loses all contact points after a restart**: The ClusterIP (`<cluster>-client`) is stable across pod restarts. Prefer it over Pod IPs as the initial contact point. See [Connect via CQL](connect-via-cql.md).

## Related pages

- [Connect via CQL](connect-via-cql.md) — using the discovery endpoint for CQL connections.
- [Configure external access](configure-external-access.md) — configuring expose options for external connectivity.
- [Networking architecture](../understand/networking.md) — how Services and expose options work.
