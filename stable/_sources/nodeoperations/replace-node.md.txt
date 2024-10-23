# Replacing a Scylla node

## Replacing a dead node
In the case of a host failure, it may not be possible to bring back the node to life.

Replace dead node operation will cause the other nodes in the cluster to stream data to the node that was replaced.
This operation can take some time (depending on the data size and network bandwidth).

_This procedure is for replacing one dead node. To replace more than one dead node, run the full procedure to completion one node at a time_

**Procedure**

1. Verify the status of the node using `nodetool status` command, the node with status DN is down and need to be replaced
    ```bash
    kubectl -n scylla exec -ti simple-cluster-us-east-1-us-east-1a-0 -c scylla -- nodetool status
    Datacenter: us-east-1
    =====================
    Status=Up/Down
    |/ State=Normal/Leaving/Joining/Moving
    --  Address        Load       Tokens       Owns    Host ID                               Rack
    UN  10.43.125.110  74.63 KB   256          ?       8ebd6114-969c-44af-a978-87a4a6c65c3e  us-east-1a
    UN  10.43.231.189  91.03 KB   256          ?       35d0cb19-35ef-482b-92a4-b63eee4527e5  us-east-1a
    DN  10.43.43.51    74.77 KB   256          ?       1ffa7a82-c41c-4706-8f5f-4d45a39c7003  us-east-1a
    ```
1. Identify service which is bound to down node by checking IP address
    ```bash
    kubectl -n scylla get svc
    NAME                                    TYPE        CLUSTER-IP      EXTERNAL-IP   PORT(S)                                                           AGE
    simple-cluster-client                   ClusterIP   None            <none>        9180/TCP                                                          3h12m
    simple-cluster-us-east-1-us-east-1a-0   ClusterIP   10.43.231.189   <none>        7000/TCP,7001/TCP,7199/TCP,10001/TCP,9042/TCP,9142/TCP,9160/TCP   3h12m
    simple-cluster-us-east-1-us-east-1a-1   ClusterIP   10.43.125.110   <none>        7000/TCP,7001/TCP,7199/TCP,10001/TCP,9042/TCP,9142/TCP,9160/TCP   3h11m
    simple-cluster-us-east-1-us-east-1a-2   ClusterIP   10.43.43.51     <none>        7000/TCP,7001/TCP,7199/TCP,10001/TCP,9042/TCP,9142/TCP,9160/TCP   3h5m
    ```
1. Drain node which we would like to replace using. **This command may delete your data from local disks attached to given node!**
    ```bash
    kubectl drain gke-scylla-demo-default-pool-b4b390a1-6j12 --ignore-daemonsets --delete-local-data
    ```

   Pod which will be replaced should enter the `Pending` state
    ```bash
    kubectl -n scylla get pods
    NAME                                    READY   STATUS    RESTARTS   AGE
    simple-cluster-us-east-1-us-east-1a-0   2/2     Running   0          3h21m
    simple-cluster-us-east-1-us-east-1a-1   2/2     Running   0          3h19m
    simple-cluster-us-east-1-us-east-1a-2   0/2     Pending   0          8m14s
    ```
1. To being node replacing, add `scylla/replace=""` label to service bound to pod we are replacing.
    ```bash
    kubectl -n scylla label svc simple-cluster-us-east-1-us-east-1a-2 scylla/replace=""
    ```
   Your failed Pod should be recreated on available k8s node
    ```bash
    kubectl -n scylla get pods
    NAME                                    READY   STATUS    RESTARTS   AGE
    simple-cluster-us-east-1-us-east-1a-0   2/2     Running   0          3h27m
    simple-cluster-us-east-1-us-east-1a-1   2/2     Running   0          3h25m
    simple-cluster-us-east-1-us-east-1a-2   1/2     Running   0          9s
    ```
   Because other nodes in cluster must stream data to new node this operation might take some time depending on how much data your cluster stores.
   After bootstraping is over, your new Pod should be ready to go.
   Old one shouldn't be no longer visible in `nodetool status`
   ```bash
   kubectl -n scylla exec -ti simple-cluster-us-east-1-us-east-1a-0 -c scylla -- nodetool status
   Datacenter: us-east-1
   =====================
   Status=Up/Down
   |/ State=Normal/Leaving/Joining/Moving
   --  Address        Load       Tokens       Owns    Host ID                               Rack
   UN  10.43.125.110  74.62 KB   256          ?       8ebd6114-969c-44af-a978-87a4a6c65c3e  us-east-1a
   UN  10.43.231.189  91.03 KB   256          ?       35d0cb19-35ef-482b-92a4-b63eee4527e5  us-east-1a
   UN  10.43.191.172  74.77 KB   256          ?       1ffa7a82-c41c-4706-8f5f-4d45a39c7003  us-east-1a
   ```
1. Run the repair on the cluster to make sure that the data is synced with the other nodes in the cluster. 
   You can use [Scylla Manager](../manager.md) to run the repair.
