# Deploying Scylla on a Kubernetes Cluster

This is a guide to deploy a Scylla Cluster in a generic Kubernetes environment, meaning that Scylla will not be deployed with the ideal performance. Scylla performs the best when it has fast disks and direct access to the cpu. This requires some extra setup, which is platform-specific. To deploy Scylla with maximum performance, follow the guide for your environment:

* [GKE](gke.md)

## Prerequisites

* A Kubernetes cluster (version >= 1.11)
* A [Storage Class](https://kubernetes.io/docs/concepts/storage/storage-classes/) to provision [PersistentVolumes](https://kubernetes.io/docs/concepts/storage/persistent-volumes/).


## Deploy Scylla Operator

First deploy the  Scylla Operator using the following commands:

```console
kubectl apply -f examples/generic/operator.yaml
```

This will install the operator StatefulSet in namespace scylla-operator-system. You can check if the operator is up and running with:
 
```console
kubectl -n scylla-operator-system get pod
```
 
## Create and Initialize a Scylla Cluster

Now that the operator is running, we can create an instance of a Scylla cluster by creating an instance of the `clusters.scylla.scylladb.com` resource.
Some of that resource's values are configurable, so feel free to browse `cluster.yaml` and tweak the settings to your liking.
Full details for all the configuration options can be found in the [Scylla Cluster CRD documentation](scylla_cluster_crd.md).

When you are ready to create a Scylla cluster, simply run:

```console
kubectl create -f examples/generic/cluster.yaml
```

We can verify that a Kubernetes object has been created that represents our new Scylla cluster with the command below.
This is important because it shows that  has successfully extended Kubernetes to make Scylla clusters a first class citizen in the Kubernetes cloud-native environment.

```console
kubectl -n scylla get clusters.scylla.scylladb.com
```

To check if all the desired members are running, you should see the same number of entries from the following command as the number of members that was specified in `cluster.yaml`:

```console
kubectl -n scylla get pod -l app=scylla
```

You can also track the state of a Scylla cluster from its status. To check the current status of a Cluster, run:

```console
kubectl -n scylla describe clusters.scylla.scylladb.com simple-cluster
```

## Accessing the Database

* From kubectl:

To get a cqlsh shell in your new Cluster:
```console
kubectl exec -n scylla -it simple-cluster-east-1-east-1a-0 -- cqlsh
> DESCRIBE KEYSPACES;
```


* From inside a Pod:

When you create a new Cluster,  automatically creates a Service for the clients to use in order to access the Cluster. The service's name follows the convention `<cluster-name>-client`. You can see this Service in your cluster by running:
```console
kubectl -n scylla describe service simple-cluster-client
```
Pods running inside the Kubernetes cluster can use this Service to connect to Scylla.
Here's an example using the [Python Driver](https://github.com/datastax/python-driver):
```python
from cassandra.cluster import Cluster

cluster = Cluster(['simple-cluster-client.scylla.svc'])
session = cluster.connect()
```

## Scale Up

The operator supports scale up of a rack as well as addition of new racks. To make the changes, you can use:
```console
kubectl -n scylla edit clusters.scylla.scylladb.com simple-cluster
```
* To scale up a rack, change the `Spec.Members` field of the rack to the desired value.
* To add a new rack, append the `racks` list with a new rack. Remember to choose a different rack name for the new rack.
* After editing and saving the yaml, check your cluster's Status and Events for information on what's happening:  
```console
kubectl -n scylla describe clusters.scylla.scylladb.com simple-cluster 
```

 
## Scale Down

The operator supports scale down of a rack. To make the changes, you can use:
```console
kubectl -n scylla edit clusters.scylla.scylladb.com simple-cluster
```
* To scale down a rack, change the `Spec.Members` field of the rack to the desired value.
* After editing and saving the yaml, check your cluster's Status and Events for information on what's happening:
```console
kubectl -n scylla describe clusters.scylla.scylladb.com simple-cluster
```
  
## Clean Up
 
To clean up all resources associated with this walk-through, you can run the commands below.

**NOTE:** this will destroy your database and delete all of its associated data.

```console
kubectl delete -f examples/generic/cluster.yaml
kubectl delete -f examples/generic/operator.yaml
```

## Troubleshooting

If the cluster does not come up, the first step would be to examine the operator's logs:

```console
kubectl -n scylla-operator-system logs -l app=scylla-operator
```

If everything looks OK in the operator logs, you can also look in the logs for one of the Scylla instances:

```console
kubectl -n scylla logs simple-cluster-us-east-1-us-east-1a-0
```
