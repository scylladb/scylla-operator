# Reference deployment: EKS

This guide deploys a production-ready ScyllaDB cluster on Amazon Elastic Kubernetes Service (EKS).
By the end, you will have a 3-node ScyllaDB cluster spread across 3 availability zones, with performance tuning and local NVMe storage configured.

## Prerequisites

- An EKS cluster provisioned with a dedicated ScyllaDB node group with local NVMe SSDs.
  If you do not have one yet, follow [Set up an EKS cluster for ScyllaDB](https://operator.docs.scylladb.com/stable/install-operator/provision-infrastructure/set-up-eks-cluster.md).
- Dedicated nodes labeled and tainted per [Set up dedicated node pools](https://operator.docs.scylladb.com/stable/deploy-scylladb/before-you-deploy/set-up-dedicated-node-pools.md).
  The EKS cluster setup guide handles this automatically.
- ScyllaDB Operator installed.
  Follow [Install ScyllaDB Operator](https://operator.docs.scylladb.com/stable/install-operator/install-with-gitops.md) if you have not done so yet.
- [`kubectl`](https://kubernetes.io/docs/tasks/tools/#kubectl) configured and pointed at the cluster.
- The environment variables from the [EKS cluster setup guide](https://operator.docs.scylladb.com/stable/install-operator/provision-infrastructure/set-up-eks-cluster.md#set-environment-variables) exported in your shell (at minimum `AWS_REGION`, `EKS_AZ_1`, `EKS_AZ_2`, `EKS_AZ_3`).

## Work around the ScyllaDB utils image issue

The default ScyllaDB utils image used by the operator for node tuning jobs contains a broken `systemctl` binary that fails on EKS nodes running `irqbalance`.
Until this is fixed upstream, override the utils image with an older version that includes a working `systemctl`:

```console
kubectl apply --server-side -f=- <<EOF
apiVersion: scylla.scylladb.com/v1alpha1
kind: ScyllaOperatorConfig
metadata:
  name: cluster
spec:
  scyllaUtilsImage: "docker.io/scylladb/scylla:2025.1"
EOF
```

## Set up nodes

Before deploying a `ScyllaCluster`, the dedicated nodes must be prepared with local disk setup and kernel tuning via `NodeConfig`, and the Local CSI Driver must be installed to provision `PersistentVolumes` from the local NVMe storage.
Follow [Configure nodes](https://operator.docs.scylladb.com/stable/deploy-scylladb/before-you-deploy/configure-nodes.md), selecting the **EKS** tab where applicable, then return here to deploy the cluster.

## Deploy a ScyllaDB cluster

Create a ScyllaDB cluster with one rack per availability zone.
The resource requests are sized for `i4i.2xlarge` (8 vCPU, 64 GiB RAM), leaving headroom for the OS, kubelet, and `DaemonSets`.
See the [ScyllaDB system requirements](https://docs.scylladb.com/manual/stable/getting-started/system-requirements.html) for general sizing guidance.
Adjust if you use a different instance type.

```console
kubectl apply --server-side -f=- <<EOF
apiVersion: scylla.scylladb.com/v1
kind: ScyllaCluster
metadata:
  name: scylladb
spec:
  repository: docker.io/scylladb/scylla
  version: 2026.1.3
  agentVersion: 3.10.1
  automaticOrphanedNodeCleanup: true
  datacenter:
    name: ${AWS_REGION}
    racks:
    - name: ${EKS_AZ_1}
      members: 1
      storage:
        capacity: 1500Gi
        storageClassName: scylladb-local-xfs
      resources:
        requests:
          cpu: 6
          memory: 50Gi
        limits:
          cpu: 6
          memory: 50Gi
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
                - ${EKS_AZ_1}
        tolerations:
        - key: scylla-operator.scylladb.com/dedicated
          operator: Equal
          value: scyllaclusters
          effect: NoSchedule
    - name: ${EKS_AZ_2}
      members: 1
      storage:
        capacity: 1500Gi
        storageClassName: scylladb-local-xfs
      resources:
        requests:
          cpu: 6
          memory: 50Gi
        limits:
          cpu: 6
          memory: 50Gi
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
                - ${EKS_AZ_2}
        tolerations:
        - key: scylla-operator.scylladb.com/dedicated
          operator: Equal
          value: scyllaclusters
          effect: NoSchedule
    - name: ${EKS_AZ_3}
      members: 1
      storage:
        capacity: 1500Gi
        storageClassName: scylladb-local-xfs
      resources:
        requests:
          cpu: 6
          memory: 50Gi
        limits:
          cpu: 6
          memory: 50Gi
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
                - ${EKS_AZ_3}
        tolerations:
        - key: scylla-operator.scylladb.com/dedicated
          operator: Equal
          value: scyllaclusters
          effect: NoSchedule
EOF
```

## Verify the deployment

Wait for the cluster to become ready:

```console
kubectl wait --for='condition=Progressing=False' scyllacluster.scylla.scylladb.com/scylladb
kubectl wait --for='condition=Degraded=False' scyllacluster.scylla.scylladb.com/scylladb
kubectl wait --for='condition=Available=True' scyllacluster.scylla.scylladb.com/scylladb
```

Verify the cluster status:

```console
kubectl get scyllaclusters.scylla.scylladb.com/scylladb
```

Expected output:

```default
NAME       READY   MEMBERS   RACKS   AVAILABLE   PROGRESSING   DEGRADED   AGE
scylladb   3       3         3       True        False         False      5m
```

## Clean up

Delete the ScyllaDB cluster first so the underlying `PersistentVolumes` are released cleanly:

```console
kubectl delete scyllaclusters.scylla.scylladb.com/scylladb
```

To tear down the EKS infrastructure, follow the [Clean up](https://operator.docs.scylladb.com/stable/install-operator/provision-infrastructure/set-up-eks-cluster.md#clean-up) section of the EKS cluster setup guide.

## Next steps

- [Set up monitoring](https://operator.docs.scylladb.com/stable/deploy-scylladb/set-up-monitoring/index.md) with Prometheus and Grafana dashboards.
- Review the [production checklist](https://operator.docs.scylladb.com/stable/deploy-scylladb/production-checklist.md) before going live.
- Learn how to [connect your application](https://operator.docs.scylladb.com/stable/connect-your-app/index.md) to ScyllaDB.
