# Reference deployment: GKE

This guide deploys a production-ready ScyllaDB cluster on Google Kubernetes Engine (GKE).
By the end, you will have a 3-node ScyllaDB cluster spread across 3 zones, with performance tuning and local NVMe storage configured.

## Prerequisites

- A regional GKE cluster provisioned with a dedicated ScyllaDB node pool with local NVMe SSDs, spread across 3 zones.
  If you do not have one yet, follow [Set up a GKE cluster for ScyllaDB](../../install-operator/provision-infrastructure/set-up-gke-cluster.md).
- Dedicated nodes labeled and tainted per [Set up dedicated node pools](../before-you-deploy/set-up-dedicated-node-pools.md).
  The GKE cluster setup guide handles this automatically.
- ScyllaDB Operator installed.
  Follow [Install ScyllaDB Operator](../../install-operator/install-with-gitops.md) if you have not done so yet.
- [`kubectl`](https://kubernetes.io/docs/tasks/tools/#kubectl) configured and pointed at the cluster.
- The environment variables from the [GKE cluster setup guide](../../install-operator/provision-infrastructure/set-up-gke-cluster.md#set-environment-variables) exported in your shell (at minimum `GCP_REGION`, `GKE_ZONE_1`, `GKE_ZONE_2`, `GKE_ZONE_3`).

## Set up nodes

Before deploying a `ScyllaCluster`, the dedicated nodes must be prepared with local disk setup and kernel tuning via `NodeConfig`, and the Local CSI Driver must be installed to provision `PersistentVolumes` from the local NVMe storage.
Follow [Configure nodes](../before-you-deploy/configure-nodes.md), selecting the **GKE** tab where applicable, then return here to deploy the cluster.

## Deploy a ScyllaDB cluster

:::{include} /_snippets/scyllacluster-namespace-tip.md
:::

Create a ScyllaDB cluster with one rack per availability zone.
The resource requests are sized for `n2-highmem-16` (16 vCPU, 128 GiB RAM), leaving headroom for the OS, kubelet, and `DaemonSets`.
See the [ScyllaDB system requirements](https://docs.scylladb.com/manual/stable/getting-started/system-requirements.html) for general sizing guidance.
Adjust if you use a different machine type.

:::{code-block} console
:substitutions:
kubectl apply --server-side -f=- <<EOF
apiVersion: scylla.scylladb.com/v1
kind: ScyllaCluster
metadata:
  name: scylladb
spec:
  repository: {{imageRepository}}
  version: {{scyllaDBImageTag}}
  agentVersion: {{agentVersion}}
  automaticOrphanedNodeCleanup: true
  datacenter:
    name: ${GCP_REGION}
    racks:
    - name: ${GKE_ZONE_1}
      members: 1
      storage:
        capacity: 500Gi
        storageClassName: scylladb-local-xfs
      resources:
        requests:
          cpu: 14
          memory: 110Gi
        limits:
          cpu: 14
          memory: 110Gi
      agentResources:
        requests:
          cpu: 100m
          memory: 20Mi
        limits:
          cpu: 100m
          memory: 20Mi
      placement:
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
            - matchExpressions:
              - key: scylla.scylladb.com/node-type
                operator: In
                values:
                - scylla
              - key: topology.kubernetes.io/zone
                operator: In
                values:
                - ${GKE_ZONE_1}
        tolerations:
        - key: scylla-operator.scylladb.com/dedicated
          operator: Equal
          value: scyllaclusters
          effect: NoSchedule
    - name: ${GKE_ZONE_2}
      members: 1
      storage:
        capacity: 500Gi
        storageClassName: scylladb-local-xfs
      resources:
        requests:
          cpu: 14
          memory: 110Gi
        limits:
          cpu: 14
          memory: 110Gi
      agentResources:
        requests:
          cpu: 100m
          memory: 20Mi
        limits:
          cpu: 100m
          memory: 20Mi
      placement:
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
            - matchExpressions:
              - key: scylla.scylladb.com/node-type
                operator: In
                values:
                - scylla
              - key: topology.kubernetes.io/zone
                operator: In
                values:
                - ${GKE_ZONE_2}
        tolerations:
        - key: scylla-operator.scylladb.com/dedicated
          operator: Equal
          value: scyllaclusters
          effect: NoSchedule
    - name: ${GKE_ZONE_3}
      members: 1
      storage:
        capacity: 500Gi
        storageClassName: scylladb-local-xfs
      resources:
        requests:
          cpu: 14
          memory: 110Gi
        limits:
          cpu: 14
          memory: 110Gi
      agentResources:
        requests:
          cpu: 100m
          memory: 20Mi
        limits:
          cpu: 100m
          memory: 20Mi
      placement:
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
            - matchExpressions:
              - key: scylla.scylladb.com/node-type
                operator: In
                values:
                - scylla
              - key: topology.kubernetes.io/zone
                operator: In
                values:
                - ${GKE_ZONE_3}
        tolerations:
        - key: scylla-operator.scylladb.com/dedicated
          operator: Equal
          value: scyllaclusters
          effect: NoSchedule
EOF
:::

## Verify the deployment

Wait for the cluster to become ready:

:::{code-block} console
kubectl wait --for='condition=Progressing=False' scyllacluster.scylla.scylladb.com/scylladb
kubectl wait --for='condition=Degraded=False' scyllacluster.scylla.scylladb.com/scylladb
kubectl wait --for='condition=Available=True' scyllacluster.scylla.scylladb.com/scylladb
:::

Verify the cluster status:

:::{code-block} console
kubectl get scyllaclusters.scylla.scylladb.com/scylladb
:::

Expected output:

:::{code-block}
NAME       READY   MEMBERS   RACKS   AVAILABLE   PROGRESSING   DEGRADED   AGE
scylladb   3       3         3       True        False         False      5m
:::

## Clean up

Delete the ScyllaDB cluster first so the underlying `PersistentVolumes` are released cleanly:

:::{code-block} console
kubectl delete scyllaclusters.scylla.scylladb.com/scylladb
:::

To tear down the GKE infrastructure, follow the [Clean up](../../install-operator/provision-infrastructure/set-up-gke-cluster.md#clean-up) section of the GKE cluster setup guide.

## Next steps

- [Set up monitoring](../set-up-monitoring/index.md) with Prometheus and Grafana dashboards.
- Review the [production checklist](../production-checklist.md) before going live.
- Learn how to [connect your application](../../connect-your-app/index.md) to ScyllaDB.
