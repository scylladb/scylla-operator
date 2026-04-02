# Expand storage volumes

Increase the persistent volume size of an existing ScyllaDB cluster when your data outgrows the initial storage allocation.

## Overview

Kubernetes StatefulSet `volumeClaimTemplates` are immutable — they cannot be updated in place.
Because the ScyllaDB Operator manages each rack as a StatefulSet, expanding storage requires an orphan-delete workflow:
you delete the controlling objects without deleting the underlying Pods or PVCs, patch the PVCs directly, then recreate the objects with the updated capacity.

The Operator does **not** automate volume expansion today — updating the storage capacity in the ScyllaCluster spec is rejected by webhook validation.
You must perform the manual procedure described below.

:::{caution}
The StorageClass used for your PersistentVolumeClaims must support volume expansion.
Verify by checking the `allowVolumeExpansion` field:

```bash
kubectl get storageclass
```

The `ALLOWVOLUMEEXPANSION` column must show `true` for the StorageClass used by your cluster.
If it does not, you must first update the StorageClass or migrate to one that supports expansion.
:::

:::{caution}
PVCs can only grow — they cannot be shrunk.
Ensure the target nodes have enough disk capacity before proceeding.
:::

## Expand storage in a ScyllaCluster

The following example assumes a ScyllaCluster named `scylla` in the `scylla` namespace.

### Step 1: Save the current ScyllaCluster definition

```bash
kubectl -n scylla get scyllacluster scylla -o yaml | yq 'del(
  .metadata.creationTimestamp,
  .metadata.generation,
  .metadata.uid,
  .metadata.resourceVersion,
  .status
)' > scyllaClusterDefinition.yaml
```

### Step 2: Orphan-delete the ScyllaCluster

Delete the ScyllaCluster object while preserving all dependent resources (ScyllaDBDatacenter, StatefulSets, Pods, PVCs):

```bash
kubectl -n scylla delete scyllacluster/scylla --cascade='orphan'
```

### Step 3: Orphan-delete the ScyllaDBDatacenter

The ScyllaDBDatacenter has the same name as the ScyllaCluster:

```bash
kubectl -n scylla delete scylladbdatacenter/scylla --cascade='orphan'
```

### Step 4: Orphan-delete the StatefulSets

Delete the StatefulSets to decouple them from the immutable `volumeClaimTemplates`, while keeping the Pods running:

```bash
kubectl -n scylla delete statefulset --selector scylla/cluster=scylla --cascade='orphan'
```

### Step 5: Patch the PVCs

List the PVCs belonging to the cluster:

```bash
kubectl -n scylla get pvc --selector scylla/cluster=scylla
```

```
NAME                              STATUS   VOLUME                                     CAPACITY   ACCESS MODES   STORAGECLASS         AGE
data-scylla-us-east-1-a-0         Bound    pvc-b96d6314-cd29-4f04-b86f-62a99634d62f   100Gi      RWO            scylladb-local-xfs   62m
data-scylla-us-east-1-a-1         Bound    pvc-a13b1123-4f04-b86f-cd29-77b00534f63a   100Gi      RWO            scylladb-local-xfs   62m
data-scylla-us-east-1-a-2         Bound    pvc-cd31cf0f-9daa-44c2-a0d9-8056780545cd   100Gi      RWO            scylladb-local-xfs   62m
```

For each PVC, patch it with the new desired size:

```bash
kubectl -n scylla patch pvc/data-scylla-us-east-1-a-0 -p '{"spec":{"resources":{"requests":{"storage":"300Gi"}}}}'
kubectl -n scylla patch pvc/data-scylla-us-east-1-a-1 -p '{"spec":{"resources":{"requests":{"storage":"300Gi"}}}}'
kubectl -n scylla patch pvc/data-scylla-us-east-1-a-2 -p '{"spec":{"resources":{"requests":{"storage":"300Gi"}}}}'
```

### Step 6: Update the ScyllaCluster definition and apply

Edit the saved definition to reflect the new storage capacity in `spec.datacenter.racks[*].storage.capacity`:

```yaml
spec:
  datacenter:
    racks:
      - name: us-east-1a
        storage:
          capacity: 300Gi    # updated from 100Gi
```

Apply the updated definition:

```bash
kubectl apply --server-side -f scyllaClusterDefinition.yaml
```

The Operator recreates the ScyllaDBDatacenter and StatefulSets with the new storage size.
The existing Pods are adopted by the new StatefulSets.

### Step 7: Verify the expansion

Depending on your storage provisioner, the expansion may happen online or require a Pod restart.
Verify the filesystem size from within each affected Pod:

```bash
kubectl -n scylla exec scylla-us-east-1-a-0 -c scylla -- df -h /var/lib/scylla
```

Repeat for all Pods in the cluster.
Check the PVC status to confirm the resize has completed:

```bash
kubectl -n scylla get pvc --selector scylla/cluster=scylla
```

The `CAPACITY` column should reflect the new size.

## Why the orphan-delete flow is necessary

The Operator represents each rack as a Kubernetes StatefulSet.
StatefulSet `volumeClaimTemplates` are [immutable by design](https://kubernetes.io/docs/concepts/workloads/controllers/statefulset/#stable-storage) in Kubernetes — once created, they cannot be changed.
Additionally, the Operator's webhook validation rejects changes to the storage fields to prevent inconsistencies between the spec and the actual StatefulSet.

The orphan-delete strategy (`--cascade=orphan`) removes the parent objects (ScyllaCluster → ScyllaDBDatacenter → StatefulSet) without deleting their children (Pods, PVCs).
This lets you patch the PVCs directly and then recreate the parent objects with the updated capacity, so the new StatefulSet's `volumeClaimTemplates` match the already-resized PVCs.

## Related pages

- [Scale a ScyllaDB cluster](scale-cluster.md) — changing the number of nodes instead of the volume size
- [StatefulSets and racks](../understand/statefulsets-and-racks.md) — why each rack maps to a StatefulSet and what constraints that implies
- [Replace nodes](replace-nodes.md) — replacing a node with a fresh PVC rather than expanding the existing one
