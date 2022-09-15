# Deploying Scylla Manager on a Kubernetes Cluster

Scylla Manager is a product for database operations automation,
it can schedule tasks such as repairs and backups.
Scylla Manager can manage multiple Scylla clusters and run cluster-wide tasks
in a controlled and predictable way.

Scylla Manager is available for Scylla Enterprise customers and Scylla Open Source users.
With Scylla Open Source, Scylla Manager is limited to 5 nodes.
See the Scylla Manager [Proprietary Software License Agreement](https://www.scylladb.com/scylla-manager-software-license-agreement/) for details.

## Prerequisites

* Kubernetes cluster
* Scylla Operator with Manager controller enabled - see [generic guide](generic.md)

## Architecture

Scylla Manager in K8s needs:
- Dedicated Scylla Cluster
  Scylla Manager persists its state to a Scylla cluster.
  Small single node cluster is just enough.

- Scylla Operator with Manager Controller

  Main mission of Controller is to watch changes of Manager CRD or Scylla Clusters, and synchronize two states.
    1. What user wants - ScyllaManager with Tasks definition in CRD.
    2. What Controller registered - Registered Clusters and Tasks - CRD status.


  When Scylla Manager CRD is being deployed, Manager Controller will automatically create/update Manager instance and its services.  
  When Scylla Cluster CRD is being deployed and its labels match specified selector in Manager CRD, 
  controller will register it in Scylla Manager.
  Once Cluster is fully up and running, it will schedule all tasks defined in Manager CRD.
  Controller also supports task updates and unscheduling.


## Deploy Scylla Manager

Deploy the Scylla Manager using the following commands:

```console
kubectl apply -f examples/common/managerv2.yaml
```

This will install the Scylla Manager and two Scylla Clusters in the `example-manager` namespace.



You can check if the Scylla Manager is up and running with:

```console
kubectl get pods -n example-manager
NAME                                               READY   STATUS    RESTARTS   AGE
scylla-manager-5b965b694-xgvdp                     1/1     Running   0          55s
scylla-manager-cluster-manager-dc-manager-rack-0   2/2     Running   0          55s
simple-cluster-us-east-1-us-east-1a-0              2/2     Running   0          55s
```

As you can see there are three pods:
* `scylla-manager-cluster-manager-dc-manager-rack-0` - is a single node Scylla Cluster for Manager to work.
* `simple-cluster-us-east-1-us-east-1a-0` - is a single node Scylla Cluster to be managed by Scylla Manager.
* `scylla-manager-5b965b694-xgvdp ` - Scylla Manager.


To check logs of Scylla Manager itself, use following command:
```console
kubectl -n example-manager logs scylla-manager-5b965b694-xgvdp
```

The output should be something like:

```console
{"L":"INFO","T":"2022-09-13T10:45:16.216Z","M":"Starting Prometheus server","address":":5090","_trace_id":"Irep-sKHST6zSDCQpAnzSQ"}
{"L":"INFO","T":"2022-09-13T10:45:16.216Z","M":"Starting debug server","address":"127.0.0.1:5112","_trace_id":"Irep-sKHST6zSDCQpAnzSQ"}
{"L":"INFO","T":"2022-09-13T10:45:16.216Z","M":"Service started","_trace_id":"Irep-sKHST6zSDCQpAnzSQ"}
```
To check the logs of the controller, execute following command:
 ```console
kubectl -n scylla-operator logs deployment.apps/scylla-operator
```

## Cluster registration

Scylla Manager will be automatically registering all the Scylla Clusters which labels match the selector.

 ```console
kubectl -n example-manager describe ScyllaManager

[...]
Spec:
  [...]
  Scylla Cluster Selector:
    Match Labels:
      Example - Manager:     true
      Managed:               true
      Managed - By:          scylla-manager
```

```console
kubeclt -n example-manager describe ScyllaCluster simple-cluster

[...]
Metadata:
  name:         simple-cluster
  namespace:    example-manager
  labels:       example-manager=true
                managed=true
                managed-by=scylla-manager
```
## Task scheduling

Task are scheduled via the Manager CRD. 

The following example shows repair task scheduled every 2 minutes.

```console
kubectl -n example-manager describe ScyllaManager

Spec:
  [...]
  Repairs:
    Cron:                   @every 2m
    Intensity:              0
    Name:                   repair-task
    Num Retries:            0
    Parallel:               0
    Small Table Threshold:  1GiB
    Timezone:               UTC
```
To check registered tasks you can use the following command:
```console 
kubectl -n example-manager exec -ti scylla-manager-5b965b694-xgvdp -- sctool tasks

Cluster: example-manager/simple-cluster (6c33390f-19ac-4736-a6c3-2e4907f6bd6b)
+------------------------+-------------+--------+----------+---------+-------+--------------+------------+--------+--------+
| Task                   | Schedule    | Window | Timezone | Success | Error | Last Success | Last Error | Status | Next   |
+------------------------+-------------+--------+----------+---------+-------+--------------+------------+--------+--------+
| healthcheck/rest       | @every 1m0s |        | UTC      | 53      | 0     | 48s ago      |            | DONE   | in 11s |
| healthcheck/cql        | @every 15s  |        | UTC      | 215     | 0     | 3s ago       |            | DONE   | in 11s |
| healthcheck/alternator | @every 15s  |        | UTC      | 219     | 0     | 4s ago       |            | DONE   | in 10s |
| repair/repair-task     | @every 2m   |        | UTC      | 26      | 0     | 1m11s ago    |            | DONE   | in 47s |
+------------------------+-------------+--------+----------+---------+-------+--------------+------------+--------+--------+
```


## Scylla Manager Status

```console 
k get -n example-manager ScyllaManager -o yaml 
```

The output should be the following:
```console
  [...]
  status:
    conditions:
    - lastTransitionTime: "2022-09-13T10:45:41Z"
      status: "True"
      type: Available
    - lastTransitionTime: "2022-09-13T10:43:11Z"
      status: "False"
      type: Degraded
    managedClusters:
    - id: 6c33390f-19ac-4736-a6c3-2e4907f6bd6b
      name: example-manager/simple-cluster
      registered: true
      repairs:
      - id: 92538142-c682-4a70-8f2f-68010f8e0a8d
        name: repair-task
        status: DONE
    observedGeneration: 1
    readyReplicas: 1
    replicas: 1
    unavailableReplicas: 0
    updatedReplicas: 1
```
- there are two conditions 
  - Available - true if at least one replica is up
  - Degraded - true if at least one replica is outdated
- simple-cluster has been registered in Manager.
  - it has been given an id
  - repair task has been scheduled in this cluster 

## Clean Up

To clean up all resources associated with Scylla Manager, you can run the commands below.

**NOTE:** this will destroy your Scylla Manager database and delete all of its associated data.

```console
kubectl delete -f examples/common/managerv2.yaml
```

## [Known Manager Issues](known_issues.md)
