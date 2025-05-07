# Discovering ScyllaDB Nodes

## Preface

Outside the Kubernetes ecosystem, ScyllaDB nodes are usually set up on static IP addresses 
and a fixed subset of them is configured as the initial contact points for ScyllaDB clients.
This has several disadvantages because this particular subset of nodes can be down at the time the client is (re)started,
or the nodes can be replaced and change their IP addresses.
The latter can be mitigated by using DNS and updating the records.

Scylla Operator allows setting up ScyllaDB in several network configurations, some of which are based on ephemeral IPs.
This makes solving the above-mentioned issues more pressing.  

## ScyllaDB Discovery Endpoint

For every ScyllaCluster, the operator will set up a Kubernetes Service (an internal load balancer) selecting all ScyllaDB nodes
and the internal Kubernetes controllers make sure to continuously update its endpoints with a subset of ScyllaDB nodes that are ready.
Because of that, you can always talk to ScyllaDB through this endpoint, as long as there are nodes to back it.
In case you are in the same Kubernetes cluster you can also use internal DNS for this service.

Clients can use this endpoint for the initial connection to reach one of the ScyllaDB nodes that are ready
and from there the drivers will automatically discover the per-node IP address for every ScyllaDB node that's part of this cluster.

This service is called `<sc-name>-client` and in its default configuration it uses ClusterIP which is virtual and local to the Kubernetes cluster.
It can be configured to be backed by an external load balancer, be exposed through an Ingress, an additional hop or in other ways.
Depending on how you have configured the networking, use the appropriate IP address or DNS name for your client.

Here is an example of how the unmodified service looks like:

```bash
kubectl get scyllacluster/scylla service/scylla-client
```
```
NAME                                       READY   MEMBERS   RACKS   AVAILABLE   PROGRESSING   DEGRADED   AGE
scyllacluster.scylla.scylladb.com/scylla   1       1         1       True        True          True       10d

NAME                    TYPE        CLUSTER-IP     EXTERNAL-IP   PORT(S)                                                                                                                   AGE
service/scylla-client   ClusterIP   10.102.44.43   <none>        7000/TCP,7001/TCP,9042/TCP,9142/TCP,19042/TCP,19142/TCP,7199/TCP,10001/TCP,9180/TCP,5090/TCP,9100/TCP,9160/TCP,8043/TCP   10d
```

You can get only the ClusterIP using
```bash
kubectl get service/scylla-client -o='jsonpath={.spec.clusterIP}'
```
or use its DNS name (`scylla-client.<sc-namespace>.svc`).

### Exposing Discovery Endpoint Behind Kubernetes Cluster Boundary

In case you are connecting from outside the Kubernetes cluster and using Pod IPs as your exposure type, you can expose just the `<sc-name>-client` service using an internal load balancer.
Having a stable contact point is especially important when using ephemeral Pod IPs.
Services configure internal load balancer using provider-specific annotations, so this may differ with your provider.

:::{tip}
To learn more about exposing ScyllaClusters, visit our dedicated documentation [page](../exposing.md).
:::

::::{tab-set}
:::{tab-item} GKE
```bash
kubectl patch service/<sc-name>-client -p '{"metadata": {"annotations": {"networking.gke.io/load-balancer-type": "Internal"}}, "spec": {"type": "LoadBalancer"}}'
kubectl wait --for=jsonpath='{.status.loadBalancer.ingress}' service/<sc-name>-client
kubectl get service/<sc-name>-client -o='jsonpath={.status.loadBalancer.ingress[0].ip}'
```
:::
:::{tab-item} EKS
```bash
kubectl patch service/<sc-name>-client -p '{"metadata": {"annotations": {"service.beta.kubernetes.io/aws-load-balancer-scheme": "internal", "service.beta.kubernetes.io/aws-load-balancer-backend-protocol": "tcp"}}, "spec": {"type": "LoadBalancer"}}'
kubectl wait --for=jsonpath='{.status.loadBalancer.ingress}' service/<sc-name>-client
kubectl get service/<sc-name>-client -o='jsonpath={.status.loadBalancer.ingress[0].hostname}'
```
:::
::::
