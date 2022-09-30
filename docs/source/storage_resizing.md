# Resizing storage in Scylla Cluster

Due to limitations of the underlying StatefulSets, ScyllaClusters don't allow storage changes. The following procedure describes how to adjust the storage manually.

Let's use the following pods and statefulsets in the resize example below. 
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

1. Orphan delete Scylla Cluster
    ```
    kubectl delete scyllacluster/simple-cluster --cascade='orphan'
   
    scyllacluster.scylla.scylladb.com "simple-cluster" deleted
    ```
2. Orphan delete Statefulsets
    ```
    kubectl delete statefulset --selector scylla/cluster=simple-cluster --cascade='orphan'
   
    statefulset.apps "simple-cluster-us-east-1-us-east-1a" deleted
    ```
3. Change storage request in PVCs
    ```
    kubectl patch pvc/data-simple-cluster-us-east-1-us-east-1a-0 -p '{"spec":{"resources":{"requests":{"storage":"2Gi"}}}}'
    
    persistentvolumeclaim/data-simple-cluster-us-east-1-us-east-1a-0 patched
    ```
    ```
    kubectl patch pvc/data-simple-cluster-us-east-1-us-east-1a-1 -p '{"spec":{"resources":{"requests":{"storage":"2Gi"}}}}'
    
    persistentvolumeclaim/data-simple-cluster-us-east-1-us-east-1a-1 patched
    ```
    ```
    kubectl patch pvc/data-simple-cluster-us-east-1-us-east-1a-2 -p '{"spec":{"resources":{"requests":{"storage":"2Gi"}}}}'
    
    persistentvolumeclaim/data-simple-cluster-us-east-1-us-east-1a-2 patched
    ```

    If the PVC change is denied because the PVC doesn't support volume expansion
    ```
    kubectl patch pvc/data-simple-cluster-us-east-1-us-east-1a-0 -p '{"spec":{"resources":{"requests":{"storage":"3Gi"}}}}'
    
    Error from server (Forbidden): persistentvolumeclaims "data-simple-cluster-us-east-1-us-east-1a-0" is forbidden: only dynamically provisioned pvc can be resized and the storageclass that provisions the pvc must support resize
    ``` 
    you need replace every node following the procedure described in [replacing nodes](nodeoperations/replace_node.md). 


4. Apply updated ScyllaCluster definition 
   ```
   kubectl apply -f clusterDefinition.yaml --server-side=true
   ```




