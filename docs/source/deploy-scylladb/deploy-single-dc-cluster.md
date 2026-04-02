# Deploy a single-DC cluster

This page walks you through creating a ScyllaDB cluster in a single datacenter using the `ScyllaCluster` resource. For multi-datacenter deployments, see [Deploy a multi-DC cluster](deploy-multi-dc-cluster.md).

:::{tip}
You can inspect all available API fields for your installed Operator version with:

```shell
kubectl explain --api-version='scylla.scylladb.com/v1' ScyllaCluster.spec
```
:::

## Prerequisites

- ScyllaDB Operator installed ([GitOps](../install-operator/install-with-gitops.md) or [Helm](../install-operator/install-with-helm.md))
- NodeConfig applied and healthy
- Local CSI Driver deployed (or another storage provisioner with XFS support)

## Create a ScyllaDB configuration

Create a ConfigMap containing the `scylla.yaml` configuration. The Operator generates most ScyllaDB settings automatically (networking, listen addresses, seeds), but you can use this ConfigMap to fine-tune settings that the Operator does not manage:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: scylladb-config
data:
  scylla.yaml: |
    authenticator: PasswordAuthenticator
    authorizer: CassandraAuthorizer
```

:::{note}
Do not configure networking, listen addresses, broadcast addresses, or seed nodes in this ConfigMap — the Operator manages these automatically. Conflicting options are overridden by the Operator. You can safely tune application-level settings like authenticator, authorizer, buffer sizes, compaction throughput, etc.
:::

```shell
kubectl apply --server-side -f scylladb-config.yaml
```

## Create a ScyllaCluster

:::{warning}
To ensure high availability and fault tolerance, **spread your nodes across multiple racks or availability zones**. As a general rule, use as many racks as your desired replication factor. For example, with replication factor 3, deploy across 3 different racks or availability zones.
:::

### Minimal example (development)

A minimal cluster for development and testing:

:::{code-block} yaml
:substitutions:
apiVersion: scylla.scylladb.com/v1
kind: ScyllaCluster
metadata:
  name: scylladb
spec:
  version: {{scyllaDBImageTag}}
  developerMode: true
  datacenter:
    name: us-east-1
    racks:
    - name: us-east-1a
      members: 1
      storage:
        capacity: 1Gi
        storageClassName: scylladb-local-xfs
      resources:
        requests:
          cpu: 10m
          memory: 100Mi
        limits:
          cpu: 1
          memory: 1Gi
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
        - key: scylla-operator.scylladb.com/dedicated
          operator: Equal
          value: scyllaclusters
          effect: NoSchedule
:::

The `developerMode: true` flag lowers resource requirements and relaxes some checks, making it suitable for quick testing. Do not use developer mode in production.

### Production example

A production-grade cluster with 3 racks across availability zones, authentication enabled, and properly sized resources:

:::{code-block} yaml
:substitutions:
apiVersion: scylla.scylladb.com/v1
kind: ScyllaCluster
metadata:
  name: scylladb
spec:
  repository: {{imageRepository}}
  version: {{scyllaDBImageTag}}
  agentVersion: {{agentVersion}}
  developerMode: false
  automaticOrphanedNodeCleanup: true
  datacenter:
    name: us-east-1
    racks:
    - name: us-east-1a
      members: 3
      scyllaConfig: scylladb-config
      storage:
        capacity: 100Gi
        storageClassName: scylladb-local-xfs
      resources:
        requests:
          cpu: 4
          memory: 32Gi
        limits:
          cpu: 4
          memory: 32Gi
      agentResources:
        requests:
          cpu: 50m
          memory: 10Mi
        limits:
          cpu: 50m
          memory: 10Mi
      placement:
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
        - key: scylla-operator.scylladb.com/dedicated
          operator: Equal
          value: scyllaclusters
          effect: NoSchedule
    - name: us-east-1b
      members: 3
      scyllaConfig: scylladb-config
      storage:
        capacity: 100Gi
        storageClassName: scylladb-local-xfs
      resources:
        requests:
          cpu: 4
          memory: 32Gi
        limits:
          cpu: 4
          memory: 32Gi
      agentResources:
        requests:
          cpu: 50m
          memory: 10Mi
        limits:
          cpu: 50m
          memory: 10Mi
      placement:
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
        - key: scylla-operator.scylladb.com/dedicated
          operator: Equal
          value: scyllaclusters
          effect: NoSchedule
    - name: us-east-1c
      members: 3
      scyllaConfig: scylladb-config
      storage:
        capacity: 100Gi
        storageClassName: scylladb-local-xfs
      resources:
        requests:
          cpu: 4
          memory: 32Gi
        limits:
          cpu: 4
          memory: 32Gi
      agentResources:
        requests:
          cpu: 50m
          memory: 10Mi
        limits:
          cpu: 50m
          memory: 10Mi
      placement:
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
        - key: scylla-operator.scylladb.com/dedicated
          operator: Equal
          value: scyllaclusters
          effect: NoSchedule
:::

:::{note}
Adjust CPU, memory, and storage values to match your workload requirements and instance types. The values above are illustrative. See [Sizing guide](../reference/sizing-guide.md) for guidance.
:::

:::{caution}
For CPU pinning and performance tuning, all containers in the pod must have **Guaranteed QoS class** (resource requests equal limits for both CPU and memory). This includes both the ScyllaDB container (`resources`) and the ScyllaDB Manager Agent sidecar (`agentResources`). See [CPU pinning](before-you-deploy/configure-cpu-pinning.md).
:::

## Key fields explained

For the full API reference, see the [API reference](../reference/api/).

## Wait for the cluster to become ready

```shell
kubectl wait --for='condition=Progressing=False' scyllacluster.scylla.scylladb.com/scylladb
kubectl wait --for='condition=Degraded=False' scyllacluster.scylla.scylladb.com/scylladb
kubectl wait --for='condition=Available=True' scyllacluster.scylla.scylladb.com/scylladb
```

You can also watch the pod status:

```shell
kubectl get pods -l scylla/cluster=scylladb -w
```

**Expected output:** All pods show `READY 2/2` (ScyllaDB container + Manager Agent sidecar) and `STATUS Running`.

## Spreading racks across availability zones

Each rack should map to a Kubernetes availability zone. Use `placement.nodeAffinity` to pin each rack to a specific zone:

::::{tabs}
:::{group-tab} GKE

```yaml
placement:
  nodeAffinity:
    requiredDuringSchedulingIgnoredDuringExecution:
      nodeSelectorTerms:
      - matchExpressions:
        - key: topology.kubernetes.io/zone
          operator: In
          values:
          - us-east1-b
        - key: scylla.scylladb.com/node-type
          operator: In
          values:
          - scylla
  tolerations:
  - key: scylla-operator.scylladb.com/dedicated
    operator: Equal
    value: scyllaclusters
    effect: NoSchedule
```
:::

:::{group-tab} EKS

```yaml
placement:
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
  - key: scylla-operator.scylladb.com/dedicated
    operator: Equal
    value: scyllaclusters
    effect: NoSchedule
```
:::
::::

## Forcing a rolling restart

To trigger a rolling restart without changing any configuration (for example, after modifying a ConfigMap that ScyllaDB does not live-reload), update the `forceRedeploymentReason` field:

```shell
kubectl patch scyllacluster scylladb --type=merge -p '{"spec":{"forceRedeploymentReason":"restart-2025-01-15"}}'
```

The Operator performs the restart one pod at a time in reverse ordinal order, respecting the PodDisruptionBudget. See [StatefulSets and racks](../understand/statefulsets-and-racks.md) for details on rolling update mechanics.

## IPv6 networking

To deploy a ScyllaDB cluster with IPv6 or dual-stack networking, see [IPv6 networking](set-up-networking/ipv6/index.md).
The single-DC deployment steps above apply to both IPv4 and IPv6 clusters; only the `spec.network` field differs.

## Related pages

- [Production checklist](production-checklist.md) — verify all production settings.
- [Dedicated node pools](before-you-deploy/set-up-dedicated-node-pools.md) — isolating ScyllaDB on dedicated nodes.
- [CPU pinning](before-you-deploy/configure-cpu-pinning.md) — configuring CPU exclusivity.
- [Node configuration](before-you-deploy/configure-nodes.md) — disk and performance tuning.
- [Connecting via CQL](../connect-your-app/connect-via-cql.md) — accessing your cluster.
- [Scaling](../operate/scale-cluster.md) — adding or removing nodes.
- [StatefulSets and racks](../understand/statefulsets-and-racks.md) — how racks map to StatefulSets.
- [Deploy a multi-DC cluster](deploy-multi-dc-cluster.md) — multi-datacenter deployment.
