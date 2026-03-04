# Deploying a multi-DC cluster

This page walks you through creating a ScyllaDB cluster that spans multiple Kubernetes clusters using the `ScyllaDBCluster` resource. For single-datacenter deployments, see [Deploying a single-DC cluster](single-dc-cluster.md).

:::{caution}
ScyllaDBCluster is a **technical preview**. Exercise caution when using it outside development environments.
:::

:::{tip}
You can inspect all available API fields for your installed Operator version with:

```shell
kubectl explain --api-version='scylla.scylladb.com/v1alpha1' ScyllaDBCluster.spec
```
:::

## Prerequisites

This guide assumes you have:

- **Four interconnected Kubernetes clusters** that can communicate using Pod IPs:
  - One **Control Plane cluster** that manages the entire ScyllaDB cluster.
  - Three **Worker clusters** that host the ScyllaDB datacenters.

:::{note}
The Control Plane cluster does not have to be a separate Kubernetes cluster. One of the Worker clusters can also serve as the Control Plane cluster.
:::

**Each Worker cluster** must have:

- A dedicated node pool for ScyllaDB with at least three nodes across different availability zones (each with a unique `topology.kubernetes.io/zone` label), labeled with `scylla.scylladb.com/node-type: scylla`.
- ScyllaDB Operator and its prerequisites installed ([GitOps](../installation/gitops.md) or [Helm](../installation/helm.md)).
- A storage provisioner capable of provisioning XFS volumes with the StorageClass `scylladb-local-xfs` on each ScyllaDB-dedicated node.
- NodeConfig applied for [performance tuning](node-configuration.md).

**The Control Plane cluster** must have:

- ScyllaDB Operator and its prerequisites installed.

For infrastructure setup instructions, see [Setting up multi-DC infrastructure](../installation/multi-dc-infrastructure.md).

:::{caution}
You are strongly advised to enable bootstrap synchronisation in your ScyllaDB Operator installations to avoid potential stability issues when adding new nodes. See [Bootstrap synchronisation](../architecture/bootstrap-sync.md) for details.

This feature requires ScyllaDB 2025.2 or later.
:::

## Step 1: Set up Control Plane access

The Control Plane cluster must have access to each Worker cluster. Create a `RemoteKubernetesCluster` resource for each Worker cluster.

### Create credential Secrets

For each Worker cluster, create a Secret containing a kubeconfig with credentials that have the `scylladb:controller:operator-remote` ClusterRole in that Worker cluster.

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: dev-us-east-1
  namespace: remotekubernetescluster-credentials
type: Opaque
stringData:
  kubeconfig: |
    apiVersion: v1
    kind: Config
    clusters:
    - cluster:
        certificate-authority-data: <kube-apiserver-ca-bundle>
        server: <kube-apiserver-address>
      name: dev-us-east-1
    contexts:
    - context:
        cluster: dev-us-east-1
        user: dev-us-east-1
      name: dev-us-east-1
    current-context: dev-us-east-1
    users:
    - name: dev-us-east-1
      user:
        token: <token-having-remote-operator-cluster-role>
```

Repeat for each Worker cluster (`dev-us-central-1`, `dev-us-west-1`).

### Create RemoteKubernetesCluster resources

Create one `RemoteKubernetesCluster` for each Worker cluster:

::::{tabs}
:::{group-tab} dev-us-east-1
```yaml
apiVersion: scylla.scylladb.com/v1alpha1
kind: RemoteKubernetesCluster
metadata:
  name: dev-us-east-1
spec:
  kubeconfigSecretRef:
    name: dev-us-east-1
    namespace: remotekubernetescluster-credentials
```
:::

:::{group-tab} dev-us-central-1
```yaml
apiVersion: scylla.scylladb.com/v1alpha1
kind: RemoteKubernetesCluster
metadata:
  name: dev-us-central-1
spec:
  kubeconfigSecretRef:
    name: dev-us-central-1
    namespace: remotekubernetescluster-credentials
```
:::

:::{group-tab} dev-us-west-1
```yaml
apiVersion: scylla.scylladb.com/v1alpha1
kind: RemoteKubernetesCluster
metadata:
  name: dev-us-west-1
spec:
  kubeconfigSecretRef:
    name: dev-us-west-1
    namespace: remotekubernetescluster-credentials
```
:::
::::

Wait for each RemoteKubernetesCluster to become available:

```shell
kubectl --context="${CONTROL_PLANE_CONTEXT}" wait --for='condition=Available=True' remotekubernetescluster.scylla.scylladb.com/dev-us-east-1
kubectl --context="${CONTROL_PLANE_CONTEXT}" wait --for='condition=Available=True' remotekubernetescluster.scylla.scylladb.com/dev-us-central-1
kubectl --context="${CONTROL_PLANE_CONTEXT}" wait --for='condition=Available=True' remotekubernetescluster.scylla.scylladb.com/dev-us-west-1
```

## Step 2: Enable authentication

Create a ConfigMap in the Control Plane cluster with the ScyllaDB configuration. This configuration is automatically propagated to all datacenters that reference it.

```shell
kubectl --context="${CONTROL_PLANE_CONTEXT}" apply --server-side -f=- <<EOF
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

## Step 3: Create the ScyllaDBCluster

The ScyllaDBCluster resource is created in the Control Plane cluster. Each datacenter references a `RemoteKubernetesCluster` that determines which Worker cluster hosts it.

The `datacenterTemplate` provides defaults inherited by all datacenters. Individual datacenters can override these defaults.

:::{code-block} yaml
:substitutions:
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
:::

:::{note}
Adjust the resources and storage capacity to match your workload requirements and instance types. The values above are illustrative. The tolerations and placement settings should match your dedicated node pool configuration. See [Dedicated node pools](dedicated-node-pools.md).
:::

:::{warning}
ScyllaDBCluster uses ClusterIP Services by default. Ensure that your CNI allows external ClusterIP connectivity between Kubernetes clusters. Otherwise, configure a different exposure type using `spec.exposeOptions`.
:::

## Step 4: Wait for the cluster to become ready

```shell
kubectl --context="${CONTROL_PLANE_CONTEXT}" wait --for='condition=Progressing=False' scylladbcluster.scylla.scylladb.com/dev-cluster
kubectl --context="${CONTROL_PLANE_CONTEXT}" wait --for='condition=Degraded=False' scylladbcluster.scylla.scylladb.com/dev-cluster
kubectl --context="${CONTROL_PLANE_CONTEXT}" wait --for='condition=Available=True' scylladbcluster.scylla.scylladb.com/dev-cluster
```

## Step 5: Verify the cluster

Datacenters in Worker clusters are reconciled in unique namespaces. Their names are visible in `ScyllaDBCluster.status.datacenters[].remoteNamespaceName`.

Run `nodetool status` against one of the ScyllaDB nodes to verify multi-datacenter connectivity:

```shell
export US_EAST_1_NS=$(kubectl --context="${CONTROL_PLANE_CONTEXT}" get scylladbcluster dev-cluster -o jsonpath='{.status.datacenters[?(@.name=="us-east-1")].remoteNamespaceName}')
kubectl --context="${DEV_US_EAST_1_CONTEXT}" --namespace="${US_EAST_1_NS}" exec dev-cluster-us-east-1-a-0 -c scylla -- nodetool status
```

**Expected output** — all nodes across all datacenters show `UN` (Up and Normal):

```
Datacenter: us-east-1
===========================
Status=Up/Down
|/ State=Normal/Leaving/Joining/Moving
-- Address        Load    Tokens Owns Host ID                              Rack      
UN 10.221.135.48  3.30 KB 256    ?    5dd7f301-62d7-4ab7-986a-e7ea9d21be4d a
UN 10.221.140.203 3.48 KB 256    ?    2f725f88-33fa-4ca7-b366-fa35e63e7c72 b
UN 10.221.150.121 3.67 KB 256    ?    7063a262-fa3f-4f69-8a60-720f464b1483 c
Datacenter: us-central-1
===========================
Status=Up/Down
|/ State=Normal/Leaving/Joining/Moving
-- Address       Load    Tokens Owns Host ID                              Rack      
UN 10.222.66.154 3.56 KB 256    ?    b17f0b94-150b-477a-8b43-c7f54b3ba357 b
UN 10.222.70.252 3.50 KB 256    ?    d99aa9b7-6fb6-46b7-bb7d-9af165dc4379 a
UN 10.222.73.52  3.35 KB 256    ?    b71980a3-cb54-4650-a4ac-ed75d31500c8 c
Datacenter: us-west-1
===========================
Status=Up/Down
|/ State=Normal/Leaving/Joining/Moving
-- Address       Load    Tokens Owns Host ID                              Rack      
UN 10.223.66.154 3.56 KB 256    ?    870cdce1-54cb-49bc-b107-a6fab16268dd b
UN 10.223.70.252 3.50 KB 256    ?    07d01c9f-6ef7-476c-99b0-0482bdbfa823 a
UN 10.223.73.52  3.35 KB 256    ?    ba2ea9e0-8924-40df-8fa2-a61d1fd263f9 c
```

## Key fields explained

### ScyllaDB configuration

| Field | Description |
|-------|-------------|
| `spec.scyllaDB.image` | ScyllaDB container image (e.g., `docker.io/scylladb/scylla:2025.4.2`). **Required.** |
| `spec.scyllaDBManagerAgent.image` | Manager Agent container image. |
| `spec.forceRedeploymentReason` | Change to trigger a rolling restart across all datacenters. |
| `spec.disableAutomaticOrphanedNodeReplacement` | When `true`, disables automatic replacement of nodes whose Kubernetes node no longer exists. |

### Datacenter template

The `datacenterTemplate` provides defaults inherited by every datacenter. Each datacenter can override any of these fields.

| Field | Description |
|-------|-------------|
| `placement` | `nodeAffinity` and `tolerations` for scheduling. |
| `scyllaDB.customConfigMapRef` | Name of a ConfigMap containing `scylla.yaml`. |
| `scyllaDB.resources` | CPU and memory for the ScyllaDB container. |
| `scyllaDB.storage.capacity` | PersistentVolume size per node. **Immutable.** |
| `scyllaDBManagerAgent.resources` | CPU and memory for the Manager Agent sidecar. |
| `rackTemplate.nodes` | Default number of nodes per rack. |
| `racks` | List of rack definitions (name and optional overrides). |

### Datacenter specification

| Field | Description |
|-------|-------------|
| `name` | Datacenter name used by the GossipingPropertyFileSnitch. |
| `remoteKubernetesClusterName` | References the `RemoteKubernetesCluster` that hosts this datacenter. |
| `forceRedeploymentReason` | Change to trigger a rolling restart for this datacenter only. |

### Immutable fields

The following fields cannot be changed after creation:

- `spec.clusterName`
- `spec.exposeOptions.nodeService.type`
- `spec.exposeOptions.broadcastOptions.clients.type`
- `spec.exposeOptions.broadcastOptions.nodes.type`
- Storage capacity at all levels

Datacenters and racks can only be removed when their node count is 0 and the status is up-to-date.

### Expose options

By default, ScyllaDBCluster uses `Headless` node services with `PodIP` broadcast addresses. This requires cross-cluster Pod IP routing.

| Field | Default |
|-------|---------|
| `exposeOptions.nodeService.type` | `Headless` |
| `exposeOptions.broadcastOptions.clients.type` | `PodIP` |
| `exposeOptions.broadcastOptions.nodes.type` | `PodIP` |

## Forcing a rolling restart

To trigger a rolling restart without changing configuration, update `forceRedeploymentReason`:

```shell
kubectl --context="${CONTROL_PLANE_CONTEXT}" patch scylladbcluster dev-cluster --type=merge \
  -p '{"spec":{"forceRedeploymentReason":"restart-2025-01-15"}}'
```

This triggers a rolling restart of all ScyllaDB nodes across all datacenters, respecting the PodDisruptionBudget of each datacenter.

To restart only a specific datacenter, set `forceRedeploymentReason` on the datacenter entry instead:

```shell
kubectl --context="${CONTROL_PLANE_CONTEXT}" patch scylladbcluster dev-cluster --type=json \
  -p '[{"op":"replace","path":"/spec/datacenters/0/forceRedeploymentReason","value":"restart-dc-2025-01-15"}]'
```

## Related pages

- [Setting up multi-DC infrastructure](../installation/multi-dc-infrastructure.md) — networking between Kubernetes clusters.
- [Bootstrap synchronisation](../architecture/bootstrap-sync.md) — how nodes coordinate joining.
- [Deploying a single-DC cluster](single-dc-cluster.md) — single-datacenter deployment using ScyllaCluster.
- [Dedicated node pools](dedicated-node-pools.md) — isolating ScyllaDB on dedicated nodes.
- [Node configuration](node-configuration.md) — disk and performance tuning.
- [Connecting via CQL](../connecting/cql.md) — accessing your cluster.
