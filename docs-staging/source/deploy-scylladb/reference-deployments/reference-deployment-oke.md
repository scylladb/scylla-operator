# Reference deployment: OKE

This guide deploys a production-ready ScyllaDB cluster on Oracle Container Engine for Kubernetes (OKE).
By the end, you will have a 3-node ScyllaDB cluster spread across 3 fault domains, with performance tuning and local NVMe storage configured.

## Prerequisites

- An OKE cluster provisioned with a dedicated Dense I/O node pool for ScyllaDB.
  If you do not have one yet, follow [Set up an OKE cluster for ScyllaDB](../../install-operator/provision-infrastructure/set-up-oke-cluster.md).
- Dedicated nodes labeled and tainted per [Set up dedicated node pools](../before-you-deploy/set-up-dedicated-node-pools.md).
  The OKE cluster setup guide handles this automatically.
- ScyllaDB Operator installed.
  Follow [Install ScyllaDB Operator](../../install-operator/install-with-gitops.md) if you have not done so yet.
- [`kubectl`](https://kubernetes.io/docs/tasks/tools/#kubectl) configured and pointed at the cluster.
- The environment variables from the [OKE cluster setup guide](../../install-operator/provision-infrastructure/set-up-oke-cluster.md#set-environment-variables) exported in your shell (at minimum `OCI_REGION`).

## Set up nodes

Before deploying a `ScyllaCluster`, the dedicated nodes must be prepared with local disk setup and kernel tuning via `NodeConfig`, and the Local CSI Driver must be installed to provision `PersistentVolumes` from the local NVMe storage.
Follow [Configure nodes](../before-you-deploy/configure-nodes.md), selecting the **OKE** tab where applicable, then return here to deploy the cluster.

## Deploy a ScyllaDB cluster

:::{include} /_snippets/scyllacluster-namespace-tip.md
:::

Create a ScyllaDB cluster with one rack per fault domain.
The resource requests are sized for `VM.DenseIO2.8` (8 OCPUs, 120 GB RAM), leaving headroom for the OS, kubelet, and `DaemonSets`.
See the [ScyllaDB system requirements](https://docs.scylladb.com/manual/stable/getting-started/system-requirements.html) for general sizing guidance.
Adjust if you use a different shape.

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
    name: ${OCI_REGION}
    racks:
    - name: fault-domain-1
      members: 1
      storage:
        capacity: 100Gi
        storageClassName: scylladb-local-xfs
      resources:
        requests:
          cpu: 6
          memory: 90Gi
        limits:
          cpu: 6
          memory: 90Gi
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
              - key: oci.oraclecloud.com/fault-domain
                operator: In
                values:
                - FAULT-DOMAIN-1
        tolerations:
        - key: scylla-operator.scylladb.com/dedicated
          operator: Equal
          value: scyllaclusters
          effect: NoSchedule
    - name: fault-domain-2
      members: 1
      storage:
        capacity: 100Gi
        storageClassName: scylladb-local-xfs
      resources:
        requests:
          cpu: 6
          memory: 90Gi
        limits:
          cpu: 6
          memory: 90Gi
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
              - key: oci.oraclecloud.com/fault-domain
                operator: In
                values:
                - FAULT-DOMAIN-2
        tolerations:
        - key: scylla-operator.scylladb.com/dedicated
          operator: Equal
          value: scyllaclusters
          effect: NoSchedule
    - name: fault-domain-3
      members: 1
      storage:
        capacity: 100Gi
        storageClassName: scylladb-local-xfs
      resources:
        requests:
          cpu: 6
          memory: 90Gi
        limits:
          cpu: 6
          memory: 90Gi
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
              - key: oci.oraclecloud.com/fault-domain
                operator: In
                values:
                - FAULT-DOMAIN-3
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

To tear down the OKE infrastructure, follow the [Clean up](../../install-operator/provision-infrastructure/set-up-oke-cluster.md#clean-up) section of the OKE cluster setup guide.

## Next steps

- [Set up monitoring](../set-up-monitoring.md) with Prometheus and Grafana dashboards.
- Review the [production checklist](../production-checklist.md) before going live.
- Learn how to [connect your application](../../connect-your-app/index.md) to ScyllaDB.
