# Deploy a multi-datacenter cluster

This page walks you through creating a ScyllaDB cluster that spans multiple Kubernetes clusters using multiple `ScyllaCluster` resources connected via `externalSeeds`. Each `ScyllaCluster` manages one datacenter. For single-datacenter deployments, see [Deploy your first cluster](deploy-your-first-cluster.md).

:::{warning}
ScyllaDB Operator only automates operations for a single datacenter. Operations related to multiple datacenters may require manual intervention. Most notably, destroying one of the Kubernetes clusters or ScyllaDB datacenters leaves DN (Down/Normal) nodes in other datacenters, and their removal must be carried out manually using `nodetool removenode`.
:::

:::{tip}
You can inspect all available API fields for your installed Operator version with:

```shell
kubectl explain --api-version='scylla.scylladb.com/v1' ScyllaCluster.spec
```
:::

## How external seeds work

The `externalSeeds` field in the `ScyllaCluster` spec propagates external seed addresses to the ScyllaDB seed provider configuration on startup. Seeds are initial contact points that let joining nodes discover the cluster ring topology.

For more information about seed nodes, see [ScyllaDB Seed Nodes](https://opensource.docs.scylladb.com/stable/kb/seed-nodes.html) in the ScyllaDB documentation.

## Prerequisites

This guide assumes you have **two interconnected Kubernetes clusters** capable of communicating using Pod IPs. Each cluster must have:

- A dedicated node pool for ScyllaDB with at least three nodes across different availability zones (each with a unique `topology.kubernetes.io/zone` label), labeled with `scylla.scylladb.com/node-type: scylla`.
- ScyllaDB Operator and its prerequisites installed ([GitOps](../install-operator/install-with-gitops.md) or [Helm](../install-operator/install-with-helm.md)).
- A storage provisioner capable of provisioning XFS volumes with the StorageClass `scylladb-local-xfs` on each ScyllaDB-dedicated node.
- NodeConfig applied for [performance tuning](before-you-deploy/configure-nodes.md).

For infrastructure networking setup, see [Set up multi-DC infrastructure](../install-operator/set-up-multi-dc-infrastructure.md).

:::{caution}
You are strongly advised to enable bootstrap synchronisation in your ScyllaDB Operator installations to avoid potential stability issues when adding new nodes. See [Bootstrap synchronisation](../understand/bootstrap-sync.md) for details.

This feature requires ScyllaDB 2025.2 or later.
:::

## Configure networking

Since this guide assumes inter-cluster connectivity over Pod IPs, configure each `ScyllaCluster` to broadcast Pod IPs for both inter-node and client communication:

```yaml
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

This requires that Pod CIDRs are routable between all Kubernetes clusters and do not overlap. For alternative exposure configurations, see [Expose ScyllaDB clusters](set-up-networking/expose-clusters.md).

## Step 1: Set kubectl contexts

Retrieve the kubectl contexts for both clusters:

```shell
kubectl config current-context
```

Save them as environment variables:

```shell
export CONTEXT_DC1=<context-for-first-cluster>
export CONTEXT_DC2=<context-for-second-cluster>
```

## Step 2: Deploy the first datacenter

Create the `scylla` namespace:

```shell
kubectl --context="${CONTEXT_DC1}" create ns scylla
```

:::{warning}
To ensure high availability and fault tolerance, **spread your nodes across multiple racks or availability zones**. As a general rule, use as many racks as your desired replication factor. For example, with replication factor 3, deploy across 3 different racks or availability zones.
:::

:::{important}
The `.metadata.name` of all `ScyllaCluster` resources that form a multi-datacenter cluster **must be identical** — this is the ScyllaDB cluster name. The `.spec.datacenter.name` must be **unique** across all datacenters in the cluster.

For more information, see [Create a ScyllaDB Cluster - Multi Data Centers (DC)](https://opensource.docs.scylladb.com/stable/operating-scylla/procedures/cluster-management/create-cluster-multidc.html) in the ScyllaDB documentation.
:::

Save the following manifest as `dc1.yaml`. Adjust the zone names, resources, and storage to match your environment:

:::{code-block} yaml
:substitutions:
apiVersion: scylla.scylladb.com/v1
kind: ScyllaCluster
metadata:
  name: scylla-cluster
  namespace: scylla
spec:
  agentVersion: {{agentVersion}}
  version: {{scyllaDBImageTag}}
  cpuset: true
  automaticOrphanedNodeCleanup: true
  exposeOptions:
    nodeService:
      type: Headless
    broadcastOptions:
      clients:
        type: PodIP
      nodes:
        type: PodIP
  datacenter:
    name: us-east-1
    racks:
    - name: a
      members: 1
      scyllaConfig: scylladb-config
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
      agentResources:
        requests:
          cpu: 100m
          memory: 250M
        limits:
          cpu: 100m
          memory: 250M
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
      scyllaConfig: scylladb-config
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
      agentResources:
        requests:
          cpu: 100m
          memory: 250M
        limits:
          cpu: 100m
          memory: 250M
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
      scyllaConfig: scylladb-config
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
      agentResources:
        requests:
          cpu: 100m
          memory: 250M
        limits:
          cpu: 100m
          memory: 250M
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
:::

Before deploying, create the ScyllaDB configuration ConfigMap to enable authentication:

```shell
kubectl --context="${CONTEXT_DC1}" -n scylla apply --server-side -f- <<EOF
apiVersion: v1
kind: ConfigMap
metadata:
  name: scylladb-config
data:
  scylla.yaml: |
    authenticator: PasswordAuthenticator
    authorizer: CassandraAuthorizer
EOF
```

Apply the ScyllaCluster manifest:

```shell
kubectl --context="${CONTEXT_DC1}" -n scylla apply --server-side -f dc1.yaml
```

Wait for the cluster to be fully rolled out:

```shell
kubectl --context="${CONTEXT_DC1}" -n scylla wait --for='condition=Progressing=False' scyllaclusters.scylla.scylladb.com/scylla-cluster
```
```console
scyllacluster.scylla.scylladb.com/scylla-cluster condition met
```

```shell
kubectl --context="${CONTEXT_DC1}" -n scylla wait --for='condition=Degraded=False' scyllaclusters.scylla.scylladb.com/scylla-cluster
```
```console
scyllacluster.scylla.scylladb.com/scylla-cluster condition met
```

```shell
kubectl --context="${CONTEXT_DC1}" -n scylla wait --for='condition=Available=True' scyllaclusters.scylla.scylladb.com/scylla-cluster
```
```console
scyllacluster.scylla.scylladb.com/scylla-cluster condition met
```

Verify that all nodes are in UN (Up/Normal) state:

```shell
kubectl --context="${CONTEXT_DC1}" -n scylla exec -it pod/scylla-cluster-us-east-1-a-0 -c scylla -- nodetool status
```

**Expected output:**

```
Datacenter: us-east-1
=====================
Status=Up/Down
|/ State=Normal/Leaving/Joining/Moving
--  Address      Load       Tokens       Owns    Host ID                               Rack
UN  10.0.70.195  290 KB     256          ?       494277b9-121c-4af9-bd63-3d0a7b9305f7  c
UN  10.0.59.24   559 KB     256          ?       a3a98e08-0dfd-4a25-a96a-c5ab2f47eb37  b
UN  10.0.19.237  107 KB     256          ?       64b6292a-327f-4128-852a-6004039f402e  a
```

## Step 3: Retrieve Pod IPs for external seeds

Retrieve the Pod IPs of the DC1 nodes. These are used as external seeds for the second datacenter.

:::{warning}
Pod IPs are ephemeral. In production environments, use domain names or non-ephemeral IP addresses as external seeds, because Pod IPs change during the cluster lifecycle and stale seeds cannot serve as fallback contact points. Pod IPs are used in this example for simplicity.
:::

```shell
kubectl --context="${CONTEXT_DC1}" -n scylla get pod/scylla-cluster-us-east-1-a-0 --template='{{ .status.podIP }}'
```
```console
10.0.19.237
```

```shell
kubectl --context="${CONTEXT_DC1}" -n scylla get pod/scylla-cluster-us-east-1-b-0 --template='{{ .status.podIP }}'
```
```console
10.0.59.24
```

```shell
kubectl --context="${CONTEXT_DC1}" -n scylla get pod/scylla-cluster-us-east-1-c-0 --template='{{ .status.podIP }}'
```
```console
10.0.70.195
```

## Step 4: Deploy the second datacenter

Create the `scylla` namespace:

```shell
kubectl --context="${CONTEXT_DC2}" create ns scylla
```

Create the same ScyllaDB configuration ConfigMap in DC2:

```shell
kubectl --context="${CONTEXT_DC2}" -n scylla apply --server-side -f- <<EOF
apiVersion: v1
kind: ConfigMap
metadata:
  name: scylladb-config
data:
  scylla.yaml: |
    authenticator: PasswordAuthenticator
    authorizer: CassandraAuthorizer
EOF
```

Save the following manifest as `dc2.yaml`. Replace the `externalSeeds` values with the Pod IPs retrieved in the previous step. Adjust zone names to match your second cluster's region:

:::{code-block} yaml
:substitutions:
apiVersion: scylla.scylladb.com/v1
kind: ScyllaCluster
metadata:
  name: scylla-cluster
  namespace: scylla
spec:
  agentVersion: {{agentVersion}}
  version: {{scyllaDBImageTag}}
  cpuset: true
  automaticOrphanedNodeCleanup: true
  exposeOptions:
    nodeService:
      type: Headless
    broadcastOptions:
      clients:
        type: PodIP
      nodes:
        type: PodIP
  externalSeeds:
  - 10.0.19.237
  - 10.0.59.24
  - 10.0.70.195
  datacenter:
    name: us-east-2
    racks:
    - name: a
      members: 1
      scyllaConfig: scylladb-config
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
      agentResources:
        requests:
          cpu: 100m
          memory: 250M
        limits:
          cpu: 100m
          memory: 250M
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
      scyllaConfig: scylladb-config
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
      agentResources:
        requests:
          cpu: 100m
          memory: 250M
        limits:
          cpu: 100m
          memory: 250M
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
      scyllaConfig: scylladb-config
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
      agentResources:
        requests:
          cpu: 100m
          memory: 250M
        limits:
          cpu: 100m
          memory: 250M
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
:::

Apply the manifest:

```shell
kubectl --context="${CONTEXT_DC2}" -n scylla apply --server-side -f dc2.yaml
```

Wait for the second datacenter to roll out:

```shell
kubectl --context="${CONTEXT_DC2}" -n scylla wait --for='condition=Progressing=False' scyllaclusters.scylla.scylladb.com/scylla-cluster
```
```console
scyllacluster.scylla.scylladb.com/scylla-cluster condition met
```

```shell
kubectl --context="${CONTEXT_DC2}" -n scylla wait --for='condition=Degraded=False' scyllaclusters.scylla.scylladb.com/scylla-cluster
```
```console
scyllacluster.scylla.scylladb.com/scylla-cluster condition met
```

```shell
kubectl --context="${CONTEXT_DC2}" -n scylla wait --for='condition=Available=True' scyllaclusters.scylla.scylladb.com/scylla-cluster
```
```console
scyllacluster.scylla.scylladb.com/scylla-cluster condition met
```

## Step 5: Verify the multi-datacenter cluster

Run `nodetool status` against a node in DC2 to verify that both datacenters are visible:

```shell
kubectl --context="${CONTEXT_DC2}" -n scylla exec -it pod/scylla-cluster-us-east-2-a-0 -c scylla -- nodetool status
```

**Expected output** — all nodes across both datacenters show `UN` (Up/Normal):

```
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

## ScyllaDB Manager integration

To integrate a multi-datacenter cluster with ScyllaDB Manager, deploy Manager in **only one** datacenter. Manager communicates with all nodes across datacenters through the Manager Agent running in each pod.

Every `ScyllaCluster` is provisioned with a unique, randomly generated auth token stored in a Secret named `<cluster-name>-auth-token`. For Manager to manage nodes in all datacenters, every datacenter must use the **same** auth token. Synchronize the token from one datacenter to the others, apply a rolling restart to pick it up, and define backup and repair tasks on the `ScyllaCluster` in the cluster where Manager is deployed. For the full procedure, see [ScyllaDB Manager](../understand/manager.md) and [Back up and restore](../operate/back-up-and-restore.md).

## Monitoring

For multi-DC clusters, deploy a `ScyllaDBMonitoring` resource independently in each datacenter's Kubernetes cluster. Each instance monitors the local `ScyllaCluster` through a local Prometheus and Grafana stack.

See [Set up monitoring](set-up-monitoring.md) for the deployment procedure.

## Related pages

- [Set up multi-DC infrastructure](../install-operator/set-up-multi-dc-infrastructure.md) — networking between Kubernetes clusters.
- [Bootstrap synchronisation](../understand/bootstrap-sync.md) — how nodes coordinate joining.
- [Deploy your first cluster](deploy-your-first-cluster.md) — single-datacenter deployment.
- [Set up dedicated node pools](before-you-deploy/set-up-dedicated-node-pools.md) — isolating ScyllaDB on dedicated nodes.
- [Configure nodes](before-you-deploy/configure-nodes.md) — disk and performance tuning.
- [Expose ScyllaDB clusters](set-up-networking/expose-clusters.md) — configuring Service types and broadcast options.
- [Connect via CQL](../connect-your-app/connect-via-cql.md) — accessing your cluster.
