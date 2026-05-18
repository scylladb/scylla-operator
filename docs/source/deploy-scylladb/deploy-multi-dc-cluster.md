# Deploy a multi-datacenter cluster

This guide describes how to deploy a multi-datacenter ScyllaDB cluster across multiple interconnected Kubernetes clusters using the ScyllaCluster API.

:::{warning}
ScyllaDB Operator only supports *manual configuration* of multi-datacenter ScyllaDB clusters using ScyllaCluster.
Although the ScyllaCluster API exposes the machinery necessary for setting up multi-datacenter clusters, Operator only automates operations for a single datacenter.

Operations related to multiple datacenters may require manual intervention.
Most notably, destroying one of the Kubernetes clusters or ScyllaDB datacenters leaves DN nodes behind in other datacenters, and their removal must be carried out manually.
:::

## Prerequisites

- Two interconnected Kubernetes clusters capable of communicating with each other over Pod IPs. If you have not set up the infrastructure yet, see [Set up multi-DC infrastructure](../install-operator/provision-infrastructure/set-up-multi-dc-infrastructure.md).
- Each cluster must have:
  - A node pool dedicated to ScyllaDB nodes composed of at least 3 nodes running in different zones, labeled with `scylla.scylladb.com/node-type: scylla`.
  - ScyllaDB Operator installed and its prerequisites met.
  - A storage provisioner capable of provisioning XFS volumes of StorageClass `scylladb-local-xfs` on each node dedicated to ScyllaDB.
- kubectl — A command line tool for working with Kubernetes clusters. See [Install Tools](https://kubernetes.io/docs/tasks/tools/) in Kubernetes documentation.

## Concepts

### External seeds

The `externalSeeds` field in ScyllaCluster's specification enables control over external seeds that are propagated to ScyllaDB as `--seed-provider-parameters seeds=<external-seeds>`.
In this context, "external" means external to the datacenter being specified by the API.
The provided seeds are used by nodes as initial points of contact, allowing them to discover the cluster ring topology when joining.

### Networking

This guide assumes interconnectivity over Pod IPs. Configure the ScyllaDB clusters to communicate over Pod IPs using `exposeOptions`:

```yaml
apiVersion: scylla.scylladb.com/v1
kind: ScyllaCluster
spec:
  exposeOptions:
    nodeService:
      type: Headless
    broadcastOptions:
      clients:
        type: PodIP
      nodes:
        type: PodIP
```

For other network configurations, refer to [Set up networking](set-up-networking/index.md).

## Set kubectl contexts

Retrieve the context of each cluster:

```shell
kubectl config current-context
```

Save the contexts as environment variables:

```shell
export CONTEXT_DC1=<context-of-first-cluster>
export CONTEXT_DC2=<context-of-second-cluster>
```

## Deploy the first datacenter

Create a dedicated namespace:

```shell
kubectl --context="${CONTEXT_DC1}" create ns scylla
```

:::{warning}
To ensure high availability and fault tolerance in ScyllaDB, spread your nodes across multiple racks or availability zones. As a general rule of thumb, use as many racks as your desired replication factor.

For example, if your replication factor is `3`, deploy your nodes across 3 different racks or availability zones.
:::

:::{caution}
The `.spec.name` field of the ScyllaCluster objects represents the ScyllaDB cluster name and must be consistent across all datacenters.
The datacenter names, specified in `.spec.datacenter.name`, must be unique across the entire multi-datacenter cluster.
:::

Save the following manifest as `dc1.yaml`. Adjust the zone names, resource requests, and storage capacity to match your infrastructure:

```yaml
apiVersion: scylla.scylladb.com/v1
kind: ScyllaCluster
metadata:
  name: scylla-cluster
  namespace: scylla
spec:
  version: 6.2.2
  cpuset: true
  automaticOrphanedNodeCleanup: true
  exposeOptions:
    broadcastOptions:
      clients:
        type: PodIP
      nodes:
        type: PodIP
    nodeService:
      type: Headless
  datacenter:
    name: us-east-1
    racks:
    - name: a
      members: 1
      storage:
        storageClassName: scylladb-local-xfs
        capacity: 1800G
      resources:
        requests:
          cpu: 7
          memory: 56G
        limits:
          cpu: 7
          memory: 56G
      placement:
        podAntiAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
          - topologyKey: kubernetes.io/hostname
            labelSelector:
              matchLabels:
                app.kubernetes.io/name: scylla
                scylla/cluster: scylla-cluster
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
            - matchExpressions:
              - key: topology.kubernetes.io/zone
                operator: In
                values:
                - us-east-1a
              - key: scylla.scylladb.com/node-type
                operator: In
                values:
                - scylla
        tolerations:
        - effect: NoSchedule
          key: scylla-operator.scylladb.com/dedicated
          operator: Equal
          value: scyllaclusters
    - name: b
      members: 1
      storage:
        storageClassName: scylladb-local-xfs
        capacity: 1800G
      resources:
        requests:
          cpu: 7
          memory: 56G
        limits:
          cpu: 7
          memory: 56G
      placement:
        podAntiAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
          - topologyKey: kubernetes.io/hostname
            labelSelector:
              matchLabels:
                app.kubernetes.io/name: scylla
                scylla/cluster: scylla-cluster
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
            - matchExpressions:
              - key: topology.kubernetes.io/zone
                operator: In
                values:
                - us-east-1b
              - key: scylla.scylladb.com/node-type
                operator: In
                values:
                - scylla
        tolerations:
        - effect: NoSchedule
          key: scylla-operator.scylladb.com/dedicated
          operator: Equal
          value: scyllaclusters
    - name: c
      members: 1
      storage:
        storageClassName: scylladb-local-xfs
        capacity: 1800G
      resources:
        requests:
          cpu: 7
          memory: 56G
        limits:
          cpu: 7
          memory: 56G
      placement:
        podAntiAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
          - topologyKey: kubernetes.io/hostname
            labelSelector:
              matchLabels:
                app.kubernetes.io/name: scylla
                scylla/cluster: scylla-cluster
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
            - matchExpressions:
              - key: topology.kubernetes.io/zone
                operator: In
                values:
                - us-east-1c
              - key: scylla.scylladb.com/node-type
                operator: In
                values:
                - scylla
        tolerations:
        - effect: NoSchedule
          key: scylla-operator.scylladb.com/dedicated
          operator: Equal
          value: scyllaclusters
```

Apply the manifest:

```shell
kubectl --context="${CONTEXT_DC1}" apply --server-side -f=dc1.yaml
```

Wait for the cluster to be fully rolled out:

```shell
kubectl --context="${CONTEXT_DC1}" -n=scylla wait --for='condition=Progressing=False' scyllaclusters.scylla.scylladb.com/scylla-cluster
kubectl --context="${CONTEXT_DC1}" -n=scylla wait --for='condition=Degraded=False' scyllaclusters.scylla.scylladb.com/scylla-cluster
kubectl --context="${CONTEXT_DC1}" -n=scylla wait --for='condition=Available=True' scyllaclusters.scylla.scylladb.com/scylla-cluster
```

Verify that all nodes are in UN state:

```shell
kubectl --context="${CONTEXT_DC1}" -n=scylla exec -it pod/scylla-cluster-us-east-1-a-0 -c=scylla -- nodetool status
```

Expected output:

```console
Datacenter: us-east-1
=====================
Status=Up/Down
|/ State=Normal/Leaving/Joining/Moving
--  Address      Load       Tokens       Owns    Host ID                               Rack
UN  10.0.70.195  290 KB     256          ?       494277b9-121c-4af9-bd63-3d0a7b9305f7  c
UN  10.0.59.24   559 KB     256          ?       a3a98e08-0dfd-4a25-a96a-c5ab2f47eb37  b
UN  10.0.19.237  107 KB     256          ?       64b6292a-327f-4128-852a-6004039f402e  a
```

## Retrieve external seeds

Retrieve the Pod IPs of the first datacenter's nodes to use as external seeds for the second datacenter:

```shell
kubectl --context="${CONTEXT_DC1}" -n=scylla get pod/scylla-cluster-us-east-1-a-0 --template='{{ .status.podIP }}'
kubectl --context="${CONTEXT_DC1}" -n=scylla get pod/scylla-cluster-us-east-1-b-0 --template='{{ .status.podIP }}'
kubectl --context="${CONTEXT_DC1}" -n=scylla get pod/scylla-cluster-us-east-1-c-0 --template='{{ .status.podIP }}'
```

:::{warning}
Due to the ephemeral nature of Pod IPs, using them as seeds in production environments is not recommended. Pods may change their IPs during the cluster's lifecycle, causing the seeds to become stale.

In production environments, use domain names or non-ephemeral IP addresses as external seeds. Pod IPs are used in this example for simplicity.
:::

## Deploy the second datacenter

Create a dedicated namespace:

```shell
kubectl --context="${CONTEXT_DC2}" create ns scylla
```

Save the following manifest as `dc2.yaml`. Replace the values in `.spec.externalSeeds` with the Pod IP addresses retrieved from the first datacenter. Adjust zone names to match your second cluster's infrastructure:

```yaml
apiVersion: scylla.scylladb.com/v1
kind: ScyllaCluster
metadata:
  name: scylla-cluster
  namespace: scylla
spec:
  version: 6.2.2
  cpuset: true
  automaticOrphanedNodeCleanup: true
  exposeOptions:
    broadcastOptions:
      clients:
        type: PodIP
      nodes:
        type: PodIP
    nodeService:
      type: Headless
  externalSeeds:
  - 10.0.19.237
  - 10.0.59.24
  - 10.0.70.195
  datacenter:
    name: us-east-2
    racks:
    - name: a
      members: 1
      storage:
        storageClassName: scylladb-local-xfs
        capacity: 1800G
      resources:
        requests:
          cpu: 7
          memory: 56G
        limits:
          cpu: 7
          memory: 56G
      placement:
        podAntiAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
          - topologyKey: kubernetes.io/hostname
            labelSelector:
              matchLabels:
                app.kubernetes.io/name: scylla
                scylla/cluster: scylla-cluster
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
            - matchExpressions:
              - key: topology.kubernetes.io/zone
                operator: In
                values:
                - us-east-2a
              - key: scylla.scylladb.com/node-type
                operator: In
                values:
                - scylla
        tolerations:
        - effect: NoSchedule
          key: scylla-operator.scylladb.com/dedicated
          operator: Equal
          value: scyllaclusters
    - name: b
      members: 1
      storage:
        storageClassName: scylladb-local-xfs
        capacity: 1800G
      resources:
        requests:
          cpu: 7
          memory: 56G
        limits:
          cpu: 7
          memory: 56G
      placement:
        podAntiAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
          - topologyKey: kubernetes.io/hostname
            labelSelector:
              matchLabels:
                app.kubernetes.io/name: scylla
                scylla/cluster: scylla-cluster
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
            - matchExpressions:
              - key: topology.kubernetes.io/zone
                operator: In
                values:
                - us-east-2b
              - key: scylla.scylladb.com/node-type
                operator: In
                values:
                - scylla
        tolerations:
        - effect: NoSchedule
          key: scylla-operator.scylladb.com/dedicated
          operator: Equal
          value: scyllaclusters
    - name: c
      members: 1
      storage:
        storageClassName: scylladb-local-xfs
        capacity: 1800G
      resources:
        requests:
          cpu: 7
          memory: 56G
        limits:
          cpu: 7
          memory: 56G
      placement:
        podAntiAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
          - topologyKey: kubernetes.io/hostname
            labelSelector:
              matchLabels:
                app.kubernetes.io/name: scylla
                scylla/cluster: scylla-cluster
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
            - matchExpressions:
              - key: topology.kubernetes.io/zone
                operator: In
                values:
                - us-east-2c
              - key: scylla.scylladb.com/node-type
                operator: In
                values:
                - scylla
        tolerations:
        - effect: NoSchedule
          key: scylla-operator.scylladb.com/dedicated
          operator: Equal
          value: scyllaclusters
```

Apply the manifest:

```shell
kubectl --context="${CONTEXT_DC2}" apply --server-side -f=dc2.yaml
```

Wait for the second datacenter to roll out:

```shell
kubectl --context="${CONTEXT_DC2}" -n=scylla wait --for='condition=Progressing=False' scyllaclusters.scylla.scylladb.com/scylla-cluster
kubectl --context="${CONTEXT_DC2}" -n=scylla wait --for='condition=Degraded=False' scyllaclusters.scylla.scylladb.com/scylla-cluster
kubectl --context="${CONTEXT_DC2}" -n=scylla wait --for='condition=Available=True' scyllaclusters.scylla.scylladb.com/scylla-cluster
```

Verify the multi-datacenter cluster:

```shell
kubectl --context="${CONTEXT_DC2}" -n=scylla exec -it pod/scylla-cluster-us-east-2-a-0 -c=scylla -- nodetool status
```

Expected output:

```console
Datacenter: us-east-1
=====================
Status=Up/Down
|/ State=Normal/Leaving/Joining/Moving
--  Address        Load       Tokens       Owns    Host ID                               Rack
UN  10.0.70.195    705 KB     256          ?       494277b9-121c-4af9-bd63-3d0a7b9305f7  c
UN  10.0.59.24     764 KB     256          ?       a3a98e08-0dfd-4a25-a96a-c5ab2f47eb37  b
UN  10.0.19.237    634 KB     256          ?       64b6292a-327f-4128-852a-6004039f402e  a
Datacenter: us-east-2
=====================
Status=Up/Down
|/ State=Normal/Leaving/Joining/Moving
--  Address        Load       Tokens       Owns    Host ID                               Rack
UN  172.16.39.209  336 KB     256          ?       7c30ea55-7a4f-4d93-86f7-c881772ebe62  b
UN  172.16.25.18   759 KB     256          ?       665dde7e-e420-4db3-8c54-ca71efd39b2e  a
UN  172.16.87.27   503 KB     256          ?       c19c89cb-e24c-4062-9df4-2aa90ab29a99  c
```

## Related pages

- [Set up multi-DC infrastructure](../install-operator/provision-infrastructure/set-up-multi-dc-infrastructure.md)
- [Set up networking](set-up-networking/index.md)
- [Deploy your first cluster](deploy-your-first-cluster.md)
