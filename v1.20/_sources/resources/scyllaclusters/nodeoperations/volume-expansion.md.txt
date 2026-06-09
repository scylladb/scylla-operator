# Resizing storage in ScyllaCluster


::::{caution}
The StorageClass used for your PersistentVolumeClaims must support volume expansion. To verify if your StorageClass supports resizing, run:

```bash
kubectl get storageclass
```

Check the `allowVolumeExpansion` column. It should be set to `true` for StorageClass used by ScyllaCluster.
::::


Let's use the following ScyllaCluster as an example.
```bash
kubectl get scyllacluster

NAME     READY   MEMBERS   RACKS   AVAILABLE   PROGRESSING   DEGRADED   AGE
scylla   3       3         1       True        False         False      62m
```

## Increasing the storage size

1. Since we are going to delete the ScyllaCluster object, make sure to have a copy of the existing one.
   ```bash
   kubectl get scyllacluster scylla -o yaml | yq 'del(
    .metadata.creationTimestamp,
    .metadata.generation,
    .metadata.uid,
    .metadata.resourceVersion,
    .status
   )' > scyllaClusterDefinition.yaml
   ```

2. Orphan delete ScyllaCluster, to delete the object, but preserve dependant objects.
    ```bash
    kubectl delete scyllacluster/scylla --cascade='orphan'

    scyllacluster.scylla.scylladb.com "scylla" deleted
    ```
3. Orphan delete the underlying ScyllaDBDatacenter. It has the same name as ScyllaCluster.
   ```bash
   kubectl delete scylladbdatacenter/scylla --cascade='orphan'

   scylladbdatacenter.scylla.scylladb.com "scylla" deleted
   ```
4. Orphan delete the underlying StatefulSets to remove objects but preserve underlying Pods.
   ```bash
   kubectl delete statefulset --selector scylla/cluster=scylla --cascade='orphan'
   
   statefulset.apps "scylla-us-east-1-us-east-1a" deleted
    ```
5. List PVCs belonging to ScyllaCluster
   ```bash
   kubectl get pvc --selector scylla/cluster=scylla
   
   NAME                                 STATUS   VOLUME                                     CAPACITY   ACCESS MODES   STORAGECLASS         VOLUMEATTRIBUTESCLASS   AGE
   data-scylla-us-east-1-a-0            Bound    pvc-b96d6314-cd29-4f04-b86f-62a99634d62f   100Gi      RWO            scylladb-local-xfs   <unset>                 62m
   data-scylla-us-east-1-a-1            Bound    pvc-a13b1123-4f04-b86f-cd29-77b00534f63a   100Gi      RWO            scylladb-local-xfs   <unset>                 62m
   data-scylla-us-east-1-a-2            Bound    pvc-cd31cf0f-9daa-44c2-a0d9-8056780545cd   100Gi      RWO            scylladb-local-xfs   <unset>                 62m
   ```

6. For each PVC from the list, patch it by providing the desired volume size.
   ::::{caution}
   PVC's cannot be shrinked. Figure out the expected volume size, ensuring Nodes hosting them have enough disk capacity.
   ::::
   ```bash
   kubectl patch pvc/data-scylla-us-east-1-a-0 -p '{"spec":{"resources":{"requests":{"storage":"300Gi"}}}}'

   persistentvolumeclaim/data-scylla-us-east-1-a-0 patched
   ```
7. Update saved ScyllaCluster definition with the new storage size `spec.datacenter.racks[*].storage.capacity` field, and apply it:
   ```bash
   kubectl apply --server-side -f scyllaClusterDefinition.yaml 
   ```

   The ScyllaDBDatacenter and StatefulSets will be recreated by the ScyllaDB Operator with the new storage size.
8. Depending on your storage provisioner, affected Pods may or may not require a restart.
   Verify if storage was resized by checking volume size from within the affected Pod.
   ```bash
   kubectl exec scylla-us-east-1-a-0 -c scylla -- df -h /var/lib/scylla
   ```

