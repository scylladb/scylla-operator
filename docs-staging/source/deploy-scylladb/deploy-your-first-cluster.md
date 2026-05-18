# Deploy your first cluster

This page walks you through creating a ScyllaDB cluster using the `ScyllaCluster` resource. It starts with a minimal development cluster you can deploy in minutes, then covers production-grade configuration. For multi-datacenter deployments, see [Deploy a multi-DC cluster](deploy-multi-dc-cluster.md).

:::{tip}
You can inspect all available API fields for your installed Operator version with:

```shell
kubectl explain --api-version='scylla.scylladb.com/v1' ScyllaCluster.spec
```
:::

## Prerequisites

- ScyllaDB Operator installed.
  Follow [Install with Helm](../install-operator/install-with-helm.md), [Install with GitOps](../install-operator/install-with-gitops.md), or [Install on OpenShift](../install-operator/install-on-openshift.md) if you have not done so yet.
- Dedicated nodes labeled and tainted per [Set up dedicated node pools](before-you-deploy/set-up-dedicated-node-pools.md).
- Nodes configured with local disk setup and kernel tuning via `NodeConfig`, and the Local CSI Driver installed to provision `PersistentVolumes` from local storage.
  Follow [Configure nodes](before-you-deploy/configure-nodes.md) if you have not done so yet.
- [`kubectl`](https://kubernetes.io/docs/tasks/tools/#kubectl) configured and pointed at the cluster.

## Quick path: create a development cluster

If you have ScyllaDB Operator installed and just want to get a ScyllaDB cluster running, follow these steps.

:::{include} /_snippets/scyllacluster-namespace-tip.md
:::

### Create a ScyllaDB configuration

Create a ConfigMap containing the `scylla.yaml` configuration. ScyllaDB Operator generates most ScyllaDB settings automatically (networking, listen addresses, seeds), but you can use this ConfigMap to fine-tune settings that ScyllaDB Operator does not manage:

```shell
kubectl apply --server-side -f=- <<EOF
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

:::{note}
Do not configure networking, listen addresses, broadcast addresses, or seed nodes in this ConfigMap — ScyllaDB Operator manages these automatically. Conflicting options are overridden by ScyllaDB Operator. You can safely tune application-level settings like authenticator, authorizer, buffer sizes, compaction throughput, etc.
:::

### Create a ScyllaCluster

:::{code-block} bash
:substitutions:
kubectl apply --server-side -f=- <<EOF
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
      scyllaConfig: scylladb-config
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
EOF
:::

The `developerMode: true` flag lowers resource requirements and relaxes some checks, making it suitable for quick testing. Do not use developer mode in production.

### Wait for the cluster to become ready

```shell
kubectl wait --for='condition=Progressing=False' --timeout=10m scyllaclusters.scylla.scylladb.com/scylladb
kubectl wait --for='condition=Degraded=False' --timeout=10m scyllaclusters.scylla.scylladb.com/scylladb
kubectl wait --for='condition=Available=True' --timeout=10m scyllaclusters.scylla.scylladb.com/scylladb
```

Verify the cluster status:

```shell
kubectl get scyllaclusters.scylla.scylladb.com/scylladb
```

Expected output:
```
NAME       READY   MEMBERS   RACKS   AVAILABLE   PROGRESSING   DEGRADED   AGE
scylladb   1       1         1       True        False         False      5m
```

You can also watch the pod status:

```shell
kubectl get pods -l scylla/cluster=scylladb -w
```

**Expected output:** All pods show `READY 2/2` (ScyllaDB container + Manager Agent sidecar) and `STATUS Running`.

Your ScyllaDB cluster is running. Explore the [next steps](#next-steps) to connect your application and prepare for production.

---

## Production deployment

For production-grade deployments with multi-rack, multi-zone configurations and properly sized resources, follow a [reference deployment](reference-deployments/index.md) for your platform.

## Key fields explained

For the full API reference, see the [API reference](../reference/api/index.md).

## Spread racks across availability zones

Each rack should map to a Kubernetes availability zone.
Use `placement.nodeAffinity` to pin each rack to a specific zone using the standard `topology.kubernetes.io/zone` label:

```yaml
placement:
  nodeAffinity:
    requiredDuringSchedulingIgnoredDuringExecution:
      nodeSelectorTerms:
      - matchExpressions:
        - key: topology.kubernetes.io/zone
          operator: In
          values:
          - <zone>
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

Replace `<zone>` with the actual zone name for each rack (e.g., `us-east1-b`, `us-east-1a`).

## Next steps

- Learn how to [connect your application](../connect-your-app/index.md) to ScyllaDB.
- Review the [production checklist](production-checklist.md) before going to production.
- See the [reference deployments](reference-deployments/index.md) for complete end-to-end examples on specific platforms.
