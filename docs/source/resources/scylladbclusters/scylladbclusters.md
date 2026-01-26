# ScyllaDBClusters

## Introduction

The [ScyllaDBCluster](../../reference/api/groups/scylla.scylladb.com/scylladbclusters.rst) resource defines a multi-datacenter ScyllaDB cluster that can span multiple geo-distributed Kubernetes clusters.
This section provides an overview of its structure and demonstrates how to perform basic configurations and access the APIs.
It is not intended as a comprehensive guide to all capabilities. For a full list of available options, refer to the [generated API reference](../../reference/api/groups/scylla.scylladb.com/scylladbclusters.rst).

:::{caution}
ScyllaDBCluster is considered as a technical preview, which means users should be cautious when using it on environments other than development.
:::

::::{tip}
You can always see the currently supported API fields for a particular version installed in your cluster by running
:::{code}
kubectl explain --api-version='scylla.scylladb.com/v1alpha1' ScyllaDBCluster.spec
:::
::::

## Creating a ScyllaDBCluster

### Prerequisites

This document outlines the process of deploying a multi-datacenter ScyllaDB cluster. Before proceeding, ensure that the necessary infrastructure is in place.
We assume you have four interconnected Kubernetes clusters that can communicate with each other using Pod IPs. These clusters are categorized as follows:
- **Control Plane cluster** – Manages the entire ScyllaDB cluster.
- **Worker clusters** – Host the ScyllaDB datacenters.
- 
::::{note}
:class: dropdown
The Control Plane cluster does not have to be a separate Kubernetes cluster. One of the Worker clusters can also serve as the Control Plane cluster.
::::

Select one cluster to act as the Control Plane and install {{productName}} along with its prerequisites.
The remaining clusters will serve as Worker clusters and must meet the following criteria:
- Include a dedicated node pool for ScyllaDB nodes, consisting of at least three nodes deployed across different zones (each with a unique `topology.kubernetes.io/zone` label), configured for ScyllaDB, and labeled with `scylla.scylladb.com/node-type: scylla`.
- Have {{productName}} and its prerequisites installed.
- Run a storage provisioner capable of provisioning XFS volumes with the StorageClass `scylladb-local-xfs` on each node dedicated to ScyllaDB.

:::{caution}
You are strongly advised to [enable bootstrap synchronisation](../../reference/feature-gates.md#bootstrapsynchronisation) in your {{productName}} installations to avoid potential stability issues when adding new nodes to your ScyllaDB clusters.
:::

For guidance on setting up such infrastructure, refer to one of the following resources:

- [Build multiple Amazon EKS clusters with Inter-Kubernetes networking](../common/multidc/eks.md)
- [Build multiple GKE clusters with Inter-Kubernetes networking](../common/multidc/gke.md)

```{image} ./scylladbcluster.svg
:name: scylladbcluster-diagram
:align: center
```

### Performance tuning

To achieve optimal performance and low latency, ensure that automatic tuning is enabled.  
We recommend reviewing our [tuning architecture documentation](../../architecture/tuning.md) and becoming familiar with the [NodeConfig resource](../nodeconfigs.md).  
A NodeConfig resource should be deployed in each Worker cluster.

### Control Plane access

The Control Plane cluster, along with {{productName}} running in it, must have access to the Worker clusters to reconcile the datacenters.  
To abstract the connection details, you'll create a RemoteKubernetesCluster resource in the Control Plane cluster for each Worker cluster.

Refer to the [RemoteKubernetesCluster documentation](../remotekubernetesclusters.md) for instructions on setting up credentials.  
In this guide, we assume that the credential Secrets are placed in the `remotekubernetescluster-credentials` namespace.  
However, you are free to use your own naming conventions.

Create three RemoteKubernetesCluster resources in the Control Plane cluster—one for each Worker cluster.

::::{tabs}
:::{group-tab} dev-us-east-1
```{literalinclude} ../../../../examples/multi-dc/cluster-wide-resources/00_dev-us-east-1.remotekubernetescluster.yaml
```
:::
:::{group-tab} dev-us-central-1
```{literalinclude} ../../../../examples/multi-dc/cluster-wide-resources/00_dev-us-central-1.remotekubernetescluster.yaml
```
:::
:::{group-tab} dev-us-west-1
```{literalinclude} ../../../../examples/multi-dc/cluster-wide-resources/00_dev-us-west-1.remotekubernetescluster.yaml
```
:::
::::

### Enable authorization and authentication in ScyllaDB cluster
:::{code-block} bash
:linenos:

kubectl --context=${CONTROL_PLANE_CONTEXT} apply --server-side -f=- <<EOF
apiVersion: v1
kind: ConfigMap
metadata:
  name: scylladb-config
data:
  scylla.yaml: |
    authenticator: PasswordAuthenticator
    authorizer: CassandraAuthorizer
    # Other options
EOF
:::

This configuration, defined in the Control Plane cluster, will be propagated to the datacenters that reference it.

Next, we can create a basic ScyllaDBCluster to launch ScyllaDB.  
In this example, the ScyllaDBCluster is a uniform resource cluster consisting of three datacenters, each with three racks.

Note that each datacenter is reconciled in a specific Worker cluster, determined by the reference to the corresponding RemoteKubernetesCluster name in its specification.

:::{code-block} bash
:substitutions:
:linenos:

kubectl --context=${CONTROL_PLANE_CONTEXT} apply --server-side -f=- <<EOF
apiVersion: scylla.scylladb.com/v1alpha1
kind: ScyllaDBCluster
metadata:
  name: dev-cluster
spec:
  scyllaDB:
    image: {{imageRepository}}:{{scyllaDBImageTag}}
  scyllaDBManagerAgent:
    image: docker.io/scylladb/scylla-manager-agent:{{agentVersion}}
  datacenterTemplate:
    placement:
      nodeAffinity:
        requiredDuringSchedulingIgnoredDuringExecution:
          nodeSelectorTerms:
          - matchExpressions:
            - key: scylla.scylladb.com/node-type
              operator: In
              values:
              - scylla
      tolerations:
      - effect: NoSchedule
        key: scylla-operator.scylladb.com/dedicated
        operator: Equal
        value: scyllaclusters
    scyllaDBManagerAgent:
      resources:
        limits:
          cpu: 100m
          memory: 100Mi
    scyllaDB:
      customConfigMapRef: scylladb-config
      resources:
        limits:
          cpu: 2
          memory: 8Gi
      storage:
        capacity: 100Gi
    rackTemplate:
      nodes: 1
      racks:
      - name: a
      - name: b
      - name: c
  datacenters:
  - name: us-east-1
    remoteKubernetesClusterName: dev-us-east-1
  - name: us-central-1
    remoteKubernetesClusterName: dev-us-central-1
  - name: us-west-1
    remoteKubernetesClusterName: dev-us-west-1
  EOF
:::


:::{note}
Values in these examples are only illustratory.
You should always adjust the resources and storage capacity depending on your needs or the size and the type of your Kubernetes nodes.
Similarly, the tolerations will differ depending on how and whether you set up dedicated node pools, or the placement if you want to set affinity for your rack to an availability zone or a failure domain.
:::

Wait for it to deploy, by watching status conditions.

:::{include} ./../../.internal/wait-for-status-conditions.scylladbcluster.code-block.md
:::

Datacenters in Worker clusters are reconciled in unique namespaces. Their names are visible in `ScyllaDBCluster.status.datacenters`.

To verify the cluster state, execute `nodetool status` in one of the worker nodes:
:::{code-block} bash
export DEV-US-EAST-1-NS=$( kubectl --context=${DEV-US-EAST-1-CONTEXT} get scylladbcluster dev-cluster -o yaml | yq '.status.datacenters[] | select(.name == "dc-0") | .remoteNamespaceName' )
kubectl --context=${DEV-US-EAST-1-CONTEXT} --namespace=${DEV-US-EAST-1-NS} exec dev-cluster-us-east-1-a-0 -c scylla -- nodetool status
:::
:::{code-block} bash
Datacenter: us-east-1
===========================
Status=Up/Down
|/ State=Normal/Leaving/Joining/Moving
-- Address        Load    Tokens Owns Host ID                              Rack      
UN 10.221.135.48  3.30 KB 256    ?    5dd7f301-62d7-4ab7-986a-e7ea9d21be4d us-east-1a
UN 10.221.140.203 3.48 KB 256    ?    2f725f88-33fa-4ca7-b366-fa35e63e7c72 us-east-1b
UN 10.221.150.121 3.67 KB 256    ?    7063a262-fa3f-4f69-8a60-720f464b1483 us-east-1c
Datacenter: us-central-1
===========================
Status=Up/Down
|/ State=Normal/Leaving/Joining/Moving
-- Address       Load    Tokens Owns Host ID                              Rack      
UN 10.222.66.154 3.56 KB 256    ?    b17f0b94-150b-477a-8b43-c7f54b3ba357 us-central-1b
UN 10.222.70.252 3.50 KB 256    ?    d99aa9b7-6fb6-46b7-bb7d-9af165dc4379 us-central-1a
UN 10.222.73.52  3.35 KB 256    ?    b71980a3-cb54-4650-a4ac-ed75d31500c8 us-central-1c
Datacenter: us-west-1
===========================
Status=Up/Down
|/ State=Normal/Leaving/Joining/Moving
-- Address       Load    Tokens Owns Host ID                              Rack      
UN 10.223.66.154 3.56 KB 256    ?    870cdce1-54cb-49bc-b107-a6fab16268dd us-west-1b
UN 10.223.70.252 3.50 KB 256    ?    07d01c9f-6ef7-476c-99b0-0482bdbfa823 us-west-1a
UN 10.223.73.52  3.35 KB 256    ?    ba2ea9e0-8924-40df-8fa2-a61d1fd263f9 us-west-1c
:::

## Forcing a rolling restart

When you change a ScyllaDB config option that's not live reloaded by ScyllaDB, or want to trigger a rolling restart for a different reason, 
ScyllaDBCluster allows triggering the rolling restarts declaratively by changing `ScyllaDBCluster.spec.forceRedeploymentReason` to any other value. 
This will trigger a rolling restart of all ScyllaDB nodes,
always respecting the [PodDisruptionBudget](https://kubernetes.io/docs/concepts/workloads/pods/disruptions/#pod-disruption-budgets) of given datacenter, keeping the cluster available.

## Next steps

To follow up with other advanced topics, see [the section index for options](./index.md).
