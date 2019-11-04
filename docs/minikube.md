# Deploying Scylla on Minikube

The easiest and quickest way to try Scylla on Kubernetes!

## Prerequisites

* [Minikube](https://kubernetes.io/docs/tasks/tools/install-minikube/)

## Walkthrough


### Start Minikube:

For this guide, we will need 4 CPUs and 4GB RAM.

```bash
minikube start --cpus=4 --memory=4096
```


### Deploy the Scylla Operator:

```bash
kubectl apply -f examples/minikube/operator.yaml
```

This will install the operator StatefulSet in namespace scylla-operator-system. You can check if the operator is up and running with:
 
```bash
kubectl -n scylla-operator-system get pods
```
 

### Create a Scylla Cluster

```bash
kubectl apply -f examples/minikube/cluster.yaml
```

We can verify that a Kubernetes object has been created that represents our new Scylla cluster with the command below.
This is important because it shows that  has successfully extended Kubernetes to make Scylla clusters a first class citizen in the Kubernetes cloud-native environment.

```bash
kubectl -n scylla get clusters.scylla.scylladb.com
```

You can also track the state of a Scylla cluster from its status. To check the current status of a Cluster, run:

```bash
kubectl -n scylla describe clusters.scylla.scylladb.com simple-cluster
```

You can also run the following command to watch the Pods until they become Ready:
```bash
kubectl -n scylla get pods --watch -l app=scylla
```

### Accessing the Database

* From kubectl:

To get a cqlsh shell in your new Cluster:
```bash
kubectl exec -n scylla -it simple-cluster-us-east-1-us-east-1a-0 -- cqlsh
> DESCRIBE KEYSPACES;
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

## Configure Scylla

The operator can take a ConfigMap and apply it to the scylla.yaml configuration file.
This is done by adding a ConfigMap to Kubernetes and refering to this in the Rack specification.
The ConfigMap is just a file called `scylla.yaml` that has the properties you want to change in it.
The operator will take the default properties for the rest of the configuration. 

* Create a ConfigMap the default name that the operator uses is `scylla-config`:
```console
kubectl create configmap scylla-config -n scylla --from-file=/path/to/scylla.yaml
```
* Wait for the mount to propagate and then restart the cluster:
```console
kubectl rollout restart -n scylla statefulset/simple-cluster-us-east-1-us-east-1a
```
* The new config should be applied automatically by the operator, check the logs to be sure.

Configuring `cassandra-rackdc.properties` is done by adding the file to the same mount as `scylla.yaml`.
```console
kubectl create configmap scylla-config -n scylla --from-file=/tmp/scylla.yaml --from-file=/tmp/cassandra-rackdc.properties -o yaml --dry-run | kubectl replace -f -
```
The operator will then apply the overridable properties `prefer_local` and `dc_suffix` if they are available in the provided mounted file.

## Clean Up
 
To clean up all resources associated with this walk-through, you can run the commands below.

**NOTE:** this will destroy your database and delete all of its associated data.

```console
kubectl delete -f examples/minikube/cluster.yaml
kubectl delete -f examples/minikube/operator.yaml
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

### Notice

This guide deploys Scylla is aimed at simplicity and because of that,
Scylla is deployed with sub-optimal performance settings.

For deploying Scylla with the optimal configuration, see the more advanced
[GKE guide](gke.md).