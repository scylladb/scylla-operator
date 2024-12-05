# Resizing storage in ScyllaCluster

Due to limitations of the underlying StatefulSet, ScyllaClusters don't allow storage changes. The following procedure describes how to adjust the storage manually.

Let's use the following pods and StatefulSet in the resize examples below.
   ``` 
   kubectl get pods                                                      
                 
   NAME                                    READY   STATUS    RESTARTS   AGE
   simple-cluster-us-east-1-us-east-1a-0   2/2     Running   0          12m
   simple-cluster-us-east-1-us-east-1a-1   2/2     Running   0          11m
   simple-cluster-us-east-1-us-east-1a-2   2/2     Running   0          9m27s
    
   kubectl get statefulset
   
   NAME                                  READY   AGE
   simple-cluster-us-east-1-us-east-1a   3/3     12m
   ``` 


## Increasing the storage size

1. Orphan delete ScyllaCluster
    ```
    kubectl delete scyllacluster/simple-cluster --cascade='orphan'
   
    scyllacluster.scylla.scylladb.com "simple-cluster" deleted
    ```
2. Orphan delete StatefulSet
    ```
    kubectl delete statefulset --selector scylla/cluster=simple-cluster --cascade='orphan'
   
    statefulset.apps "simple-cluster-us-east-1-us-east-1a" deleted
    ```
3. Change storage request in PVCs.
    ```
    kubectl patch pvc/data-simple-cluster-us-east-1-us-east-1a-0 -p '{"spec":{"resources":{"requests":{"storage":"3Gi"}}}}'
    
    persistentvolumeclaim/data-simple-cluster-us-east-1-us-east-1a-0 patched
    ```
    ```
    kubectl patch pvc/data-simple-cluster-us-east-1-us-east-1a-1 -p '{"spec":{"resources":{"requests":{"storage":"3Gi"}}}}'
    
    persistentvolumeclaim/data-simple-cluster-us-east-1-us-east-1a-1 patched
    ```
    ```
    kubectl patch pvc/data-simple-cluster-us-east-1-us-east-1a-2 -p '{"spec":{"resources":{"requests":{"storage":"3Gi"}}}}'
    
    persistentvolumeclaim/data-simple-cluster-us-east-1-us-east-1a-2 patched
    ```

    If the PVC change is denied because the PVC doesn't support volume expansion
    ```
    kubectl patch pvc/data-simple-cluster-us-east-1-us-east-1a-0 -p '{"spec":{"resources":{"requests":{"storage":"3Gi"}}}}'
    
    Error from server (Forbidden): persistentvolumeclaims "data-simple-cluster-us-east-1-us-east-1a-0" is forbidden: only dynamically provisioned pvc can be resized and the storageclass that provisions the pvc must support resize
    ``` 
    then continue to step 4. 

4. Apply updated ScyllaCluster definition with the new storage size in the `spec.datacenter.racks[0].storage.capacity` field:
   ```
   kubectl apply -f clusterDefinition.yaml --server-side=true
   ```
   or, you use Helm to manage your ScyllaCluster, edit your values YAML with the cluster definition and its `racks[0].storage.capacity` field and apply the change with:
   ```
   helm upgrade simple-cluster scylla/scylla --values values.cluster.yaml
   ```
   The StatefulSet will be recreated by the Scylla Operator with the new storage size.

5. (Only if PVC does not support volume expansion) You need to replace every Scylla node, one by one, following the procedure described in [replacing nodes](/resources/scyllaclusters/nodeoperations/replace-node). Start with the last node in the StatefulSet and move towards the first node. This will recreate each node and their PVCs with the new size.


## Decreasing the storage size

1. Orphan delete ScyllaCluster
    ```
    kubectl delete scyllacluster/simple-cluster --cascade='orphan'
   
    scyllacluster.scylla.scylladb.com "simple-cluster" deleted
    ```

2. Orphan delete Statefulset
    ```
    kubectl delete statefulset --selector scylla/cluster=simple-cluster --cascade='orphan'
   
    statefulset.apps "simple-cluster-us-east-1-us-east-1a" deleted
    ```

3. Apply updated ScyllaCluster definition with the new storage size in the `spec.datacenter.racks[0].storage.capacity` field.
    ```
    kubectl apply -f clusterDefinition.yaml --server-side=true
    ```
   or, if you use Helm to manage your ScyllaCluster, edit your values YAML with the cluster definition and its `racks[0].storage.capacity` field and apply the change with:
    ```
    helm upgrade simple-cluster scylla/scylla --values values.cluster.yaml
    ```
   The StatefulSet will be recreated by the Scylla Operator with the new storage size, but the actual change will NOT be automatically applied.

4. You need to replace every Scylla node, one by one, following the procedure described in [replacing nodes](/resources/scyllaclusters/nodeoperations/replace-node). Start with the last node in the StatefulSet and move towards the first node. This will recreate each node and their PVCs with the new size.
