# Deploy a multi-datacenter ScyllaDB cluster in multiple interconnected Kubernetes clusters

This document describes the process of deploying a Multi Datacenter ScyllaDB cluster in multiple interconnected Kubernetes clusters.

This guide will walk you through the example procedure of deploying two datacenters in distinct regions of a selected cloud provider.

:::{note}
This guide is dedicated to deploying multi-datacenter ScyllaDB clusters and does not discuss unrelated configuration options.
For details of ScyllaDB cluster deployments and their configuration, refer to [Deploying Scylla on a Kubernetes Cluster](../generic.md) in ScyllaDB Operator documentation.
:::

## Prerequisites

As this document describes the procedure of deploying a Multi Datacenter ScyllaDB cluster, you are expected to have the required infrastructure prepared.
Let's assume two interconnected Kubernetes clusters, capable of communicating with each other over PodIPs, with each cluster meeting the following requirements:
- a node pool dedicated to ScyllaDB nodes composed of at least 3 nodes running in different zones (with unique `topology.kubernetes.io/zone` label), configured to run ScyllaDB, each labeled with `scylla.scylladb.com/node-type: scylla`
- running ScyllaDB Operator and its prerequisites
- running a storage provisioner capable of provisioning XFS volumes of StorageClass `scylladb-local-xfs` in each of the nodes dedicated to ScyllaDB instances

You can refer to one of our guides describing the process of preparing such infrastructure:
- [Build multiple Amazon EKS clusters with Inter-Kubernetes networking](eks.md)
- [Build multiple GKE clusters with Inter-Kubernetes networking](gke.md)

Additionally, to follow the below guide, you need to install and configure the following tools that you will need to manage Kubernetes resources:
- kubectl – A command line tool for working with Kubernetes clusters.

See [Install Tools](https://kubernetes.io/docs/tasks/tools/) in Kubernetes documentation for reference.

## Multi Datacenter ScyllaDB Cluster

In v1.11, ScyllaDB Operator introduced support for manual multi-datacenter ScyllaDB cluster deployments.

:::{warning}
ScyllaDB Operator only supports *manual configuration* of multi-datacenter ScyllaDB clusters.
In other words, although ScyllaCluster API exposes the machinery necessary for setting up multi-datacenter ScylaDB clusters, the ScyllaDB Operator only automates operations for a single datacenter.

Operations related to multiple datacenters may require manual intervention of a human operator.
Most notably, destroying one of the Kubernetes clusters or ScyllaDB datacenters is going to leave DN nodes behind in other datacenters, and their removal has to be carried out manually.
:::

The main mechanism used to set up a manual multi-datacenter ScyllaDB cluster is a field in ScyllaCluster's specification - `externalSeeds`.

### External seeds

The `externalSeeds` field in ScyllaCluster's specification enables control over external seeds that are propagated to ScyllaDB binary as `--seed-provider-parameters seeds=<external-seeds>`.
In this context, external should be understood as "external to the datacenter being specified by the API".
The provided seeds are used by the nodes as initial points of contact, which allows them to discover the cluster ring topology when joining it.

Refer to [Scylla Seed Nodes](https://opensource.docs.scylladb.com/stable/kb/seed-nodes.html) in ScyllaDB documentation for more information regarding the function of seed nodes in ScyllaDB.
For more details regarding the function and implementation of external seeds, refer to [the original enhancement proposal](https://github.com/scylladb/scylla-operator/tree/v1.11/enhancements/proposals/1304-external-seeds).

### Networking

Since this guide assumes interconnectivity over PodIPs of the Kubernetes clusters, you are going to configure the ScyllaDB cluster's nodes to communicate over PodIPs.
This is enabled by a subset of `exposeOptions` specified in ScyllaCluster API, introduced in v1.11.

For this particular setup, define the ScyllaClusers as follows:
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

However, other configuration options allow for the manual deployment of multi-datacenter ScyllaDB clusters in different network setups. For details, refer to [Exposing ScyllaClusters](../exposing.md) in ScyllaDB Operator documentation.

#### Deploy a multi-datacenter ScyllaDB Cluster

#### Using context

Let's specify contexts for `kubectl` commands used throughout the guide.
To retrieve the context of your current cluster, run:
```shell
kubectl config current-context
```

Save the contexts of the two clusters, which you are going to deploy the datacenters in, as `CONTEXT_DC1` and `CONTEXT_DC2` environment variables correspondingly.

#### Deploy the first datacenter

First, run the below command to create a dedicated 'scylla' namespace:
```shell
kubectl --context="${CONTEXT_DC1}" create ns scylla
```

For this guide, let's assume that your cluster is running in `us-east-1` region and the nodes dedicated to running ScyllaDB nodes are running in zones `us-east-1a`, `us-east-1b` and `us-east-1c` correspondingly. If that is not the case, adjust the manifest accordingly.

:::{caution}
The `.spec.name` field of the ScyllaCluster objects represents the ScyllaDB cluster name and has to be consistent across all datacenters of this ScyllaDB cluster.
The names of the datacenters, specified in `.spec.datacenter.name`, have to be unique across the entire multi-datacenter cluster.

For more information see [Create a ScyllaDB Cluster - Multi Data Centers (DC)](https://opensource.docs.scylladb.com/stable/operating-scylla/procedures/cluster-management/create-cluster-multidc.html) in ScyllaDB documentation.
:::

Save the ScyllaCluster manifest in `dc1.yaml`:
```yaml
apiVersion: scylla.scylladb.com/v1
kind: ScyllaCluster
metadata:
  name: scylla-cluster
  namespace: scylla
spec:
  agentVersion: 3.2.8
  version: 5.4.3
  cpuset: true
  sysctls:
  - "fs.aio-max-nr=2097152"
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
      agentResources:
        requests:
          cpu: 100m
          memory: 250M
        limits:
          cpu: 100m
          memory: 250M
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
        - key: role
          operator: Equal
          value: scylla-clusters
          effect: NoSchedule
    - name: b
      members: 1
      storage:
        storageClassName: scylladb-local-xfs
        capacity: 1800G
      agentResources:
        requests:
          cpu: 100m
          memory: 250M
        limits:
          cpu: 100m
          memory: 250M
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
        - key: role
          operator: Equal
          value: scylla-clusters
          effect: NoSchedule
    - name: c
      members: 1
      storage:
        storageClassName: scylladb-local-xfs
        capacity: 1800G
      agentResources:
        requests:
          cpu: 100m
          memory: 250M
        limits:
          cpu: 100m
          memory: 250M
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
        - key: role
          operator: Equal
          value: scylla-clusters
          effect: NoSchedule
```

Apply the manifest:
```shell
kubectl --context="${CONTEXT_DC1}" apply --server-side -f=dc1.yaml
```

Wait for the cluster to be fully rolled out:
```shell
kubectl --context="${CONTEXT_DC1}" -n=scylla wait --for='condition=Progressing=False' scyllaclusters.scylla.scylladb.com/scylla-cluster
```
```console
scyllacluster.scylla.scylladb.com/scylla-cluster condition met
```

```shell
kubectl --context="${CONTEXT_DC1}" -n=scylla wait --for='condition=Degraded=False' scyllaclusters.scylla.scylladb.com/scylla-cluster
```
```console
scyllacluster.scylla.scylladb.com/scylla-cluster condition met
```

```shell
kubectl --context="${CONTEXT_DC1}" -n=scylla wait --for='condition=Available=True' scyllaclusters.scylla.scylladb.com/scylla-cluster
```
```console
scyllacluster.scylla.scylladb.com/scylla-cluster condition met
```

You can now verify that all the nodes of your cluster are in UN state:
```shell
kubectl --context="${CONTEXT_DC1}" -n=scylla exec -it pod/scylla-cluster-us-east-1-a-0 -c=scylla -- nodetool status
```

The expected output should look similar to the below:
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

##### Retrieve PodIPs of ScyllaDB nodes for use as external seeds

:::{warning}
Due to the ephemeral nature of PodIPs, it is ill-advised to use them as seeds in production environments. 
This is because there is a high likelihood that the Pods of your ScyllaDB clusters will change their IPs during the cluster's lifecycle, and so the provided seeds will no longer point to the ScyllaDB nodes.
It is undesired, as the seeds provided on node's startup may serve as fallback contact points when all of the node's peers are unreachable.
In production environments, it is recommended that you use domain names or non-ephemeral IP addresses as external seeds.
PodIPs are being used in this example for the sheer simplicity of this setup.
:::

Use the below commands and their expected outputs as a reference for retrieving the PodIPs used by the cluster for inter-node communication.
```shell
kubectl --context="${CONTEXT_DC1}" -n=scylla get pod/scylla-cluster-us-east-1-a-0 --template='{{ .status.podIP }}'
```
```console
10.0.19.237
```

```shell
kubectl --context="${CONTEXT_DC1}" -n=scylla get pod/scylla-cluster-us-east-1-b-0 --template='{{ .status.podIP }}'
```
```console
10.0.59.24
```

```shell
kubectl --context="${CONTEXT_DC1}" -n=scylla get pod/scylla-cluster-us-east-1-c-0 --template='{{ .status.podIP }}'
```
```console
10.0.70.195
```

You are going to utilize the retrieved addresses as seeds for the other datacenter.

#### Deploy the second datacenter

To deploy the second datacenter, you will follow similar steps.

First, create a dedicated 'scylla' namespace:
```shell
kubectl --context="${CONTEXT_DC2}" create ns scylla
```

Replace the values in `.spec.externalSeeds` of the below manifest with the Pod IP addresses that you retrieved earlier.
The provided values are going to serve as initial contact points for the joining nodes of the second datacenter.

For this guide, let's assume that the second cluster is running in `us-east-2` region and the nodes dedicated for running ScyllaDB nodes are running in zones `us-east-2a`, `us-east-2b` and `us-east-2c` correspondingly. If that is not the case, adjust the manifest accordingly.
Having configured it, save the manifest as `dc2.yaml`:
```yaml
apiVersion: scylla.scylladb.com/v1
kind: ScyllaCluster
metadata:
  name: scylla-cluster
  namespace: scylla
spec:
  agentVersion: 3.2.8
  version: 5.4.3
  cpuset: true
  sysctls:
  - "fs.aio-max-nr=2097152"
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
      agentResources:
        requests:
          cpu: 100m
          memory: 250M
        limits:
          cpu: 100m
          memory: 250M
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
        - key: role
          operator: Equal
          value: scylla-clusters
          effect: NoSchedule
    - name: b
      members: 1
      storage:
        storageClassName: scylladb-local-xfs
        capacity: 1800G
      agentResources:
        requests:
          cpu: 100m
          memory: 250M
        limits:
          cpu: 100m
          memory: 250M
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
        - key: role
          operator: Equal
          value: scylla-clusters
          effect: NoSchedule
    - name: c
      members: 1
      storage:
        storageClassName: scylladb-local-xfs
        capacity: 1800G
      agentResources:
        requests:
          cpu: 100m
          memory: 250M
        limits:
          cpu: 100m
          memory: 250M
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
        - key: role
          operator: Equal
          value: scylla-clusters
          effect: NoSchedule
```

To apply the manifest, run:
```shell
kubectl --context="${CONTEXT_DC2}" -n=scylla apply --server-side -f=dc2.yaml
```

Wait for the second datacenter to roll out:
```shell
kubectl --context="${CONTEXT_DC2}" -n=scylla wait --for='condition=Progressing=False' scyllaclusters.scylla.scylladb.com/scylla-cluster
```
```console
scyllacluster.scylla.scylladb.com/scylla-cluster condition met
```

```shell
kubectl --context="${CONTEXT_DC2}" -n=scylla wait --for='condition=Degraded=False' scyllaclusters.scylla.scylladb.com/scylla-cluster
```
```console
scyllacluster.scylla.scylladb.com/scylla-cluster condition met
```

```shell
kubectl --context="${CONTEXT_DC2}" -n=scylla wait --for='condition=Available=True' scyllaclusters.scylla.scylladb.com/scylla-cluster
```
```console
scyllacluster.scylla.scylladb.com/scylla-cluster condition met
```

You can verify that the nodes have joined the existing cluster and that you are now running a multi-datacenter ScyllaDB cluster by running `nodetool status` with the below command:
```shell
kubectl --context="${CONTEXT_DC2}" -n=scylla exec -it pod/scylla-cluster-us-east-2-a-0 -c=scylla -- nodetool status
```
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

## Scylla Manager

To integrate a multi-datacenter ScyllaDB cluster with Scylla Manager, you must deploy the Scylla Manager in only one datacenter.

In this example, let's choose the Kubernetes cluster deployed in the first datacenter to host it.
To deploy Scylla Manager, follow the steps described in [Deploying Scylla Manager on a Kubernetes Cluster](../manager.md)
in ScyllaDB Operator documentation. 

In order to define the Scylla Manager tasks, add them to the ScyllaCluster object deployed in the same Kubernetes cluster 
in which your Scylla Manager is running.

Every datacenter (represented by ScyllaCluster CR) is, by default, provisioned with a new, random Scylla Manager Agent auth token.
To use Scylla Manager with multiple datacenter (represented by ScyllaClusters), you have to make sure they all use the same token.

Extract it from the first datacenter with the below command:
```shell
kubectl --context="${CONTEXT_DC1}" -n=scylla get secrets/scylla-cluster-auth-token --template='{{ index .data "auth-token.yaml" }}' | base64 -d
```
```console
auth_token: 84qtsfvm98qzmps8s65zr2vtpb8rg4sdzcbg4pbmg2pfhxwpg952654gj86tzdljfqnsghndljm58mmhpmwfgpsvjx2kkmnns8bnblmgkbl9n8l9f64rs6tcvttm7kmf
```

Save the output, replace the token with your own, and patch the secret in the second datacenter with the below command:
```shell
kubectl --context="${CONTEXT_DC2}" -n=scylla patch secret/scylla-cluster-auth-token--type='json' -p='[{"op": "add", "path": "/stringData", "value": {"auth-token.yaml": "auth_token: 84qtsfvm98qzmps8s65zr2vtpb8rg4sdzcbg4pbmg2pfhxwpg952654gj86tzdljfqnsghndljm58mmhpmwfgpsvjx2kkmnns8bnblmgkbl9n8l9f64rs6tcvttm7kmf"}}]'
```

Execute a rolling restart of the nodes in DC2 to make sure they pick up the new token:
```shell
kubectl --context="${CONTEXT_DC2}" -n=scylla patch scyllacluster/scylla-cluster --type='merge' -p='{"spec": {"forceRedeploymentReason": "sync scylla-manager-agent token ('"$( date )"')"}}'
```


## ScyllaDBMonitoring

To monitor your cluster, deploy ScyllaDBMonitoring in every datacenter independently.
To deploy ScyllaDB Monitoring, follow the steps described in [Deploy managed monitoring](../monitoring.md#deploy-managed-monitoring) in ScyllaDB Operator documentation.
