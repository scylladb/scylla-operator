# Deploying Scylla on a Kubernetes Cluster

This is a guide to deploy a Scylla Cluster in a generic Kubernetes environment, meaning that Scylla will not be deployed with the ideal performance.
Scylla performs the best when it has fast disks and direct access to the cpu.
This requires some extra setup, which is platform-specific.
For specific configuration and setup, check for details about your particular environment:

* [GKE](gke.md)

## Prerequisites

* A Kubernetes cluster
* A [Storage Class](https://kubernetes.io/docs/concepts/storage/storage-classes/) to provision [PersistentVolumes](https://kubernetes.io/docs/concepts/storage/persistent-volumes/).
* Helm 3 installed, Go to the [helm docs](https://docs.helm.sh/using_helm/#installing-helm) if you need to install it.
  Make sure that you enable the [stable repository](https://github.com/helm/charts#how-do-i-enable-the-stable-repository-for-helm-3)

## Running locally

Running kubernetes locally is a daunting and error prone task.
Fortunately there are ways to make life easier and [Minikube](https://minikube.sigs.k8s.io/docs/) makes it a breeze.

We need to give minikube a little bit more resources than default so start minikube like this:
```console
minikube start --cpus=6
```

Then make kubectl aware of this local installation like this:
```console
eval $(minikube docker-env)
```

## Deploy Cert Manager
First deploy Cert Manager, you can either follow [upsteam instructions](https://cert-manager.io/docs/installation/kubernetes/) or use following command:

```console
kubectl apply -f examples/common/cert-manager.yaml
```
This will install Cert Manager to provision a self-signed certificate.

Once it's deployed, wait until Cert Manager is ready:

```console
kubectl wait --for condition=established crd/certificates.cert-manager.io crd/issuers.cert-manager.io
kubectl -n cert-manager rollout status deployment.apps/cert-manager-webhook
```

## Deploy Scylla Operator

Deploy the Scylla Operator using the following commands:

```console
kubectl apply -f examples/common/operator.yaml
```

This will install the operator in namespace `scylla-operator`.
Wait until it's ready:

```console
kubectl wait --for condition=established crd/scyllaclusters.scylla.scylladb.com
kubectl -n scylla-operator rollout status deployment.apps/scylla-operator
```

If you want to check the logs of the operator you can do so with:

 ```console
kubectl -n scylla-operator logs deployment.apps/scylla-operator
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
kubectl -n scylla get ScyllaCluster
```

Checking the pods that are created is as easy as:

```console
kubectl -n scylla get pods
```

The output should be something like:

```console
NAME                                    READY   STATUS    RESTARTS   AGE
simple-cluster-us-east-1-us-east-1a-0   2/2     Running   0          9m49s
simple-cluster-us-east-1-us-east-1a-1   2/2     Running   0          7m43s
simple-cluster-us-east-1-us-east-1a-2   2/2     Running   0          6m46s
```

It is important to note that the operator creates these instances according to a pattern.
This pattern is as follows: `CLUSTER_NAME-DATACENTER_NAME-RACK_NAME-INSTANCE_NUMBER` as specified in `cluster.yaml`.

In the above example we have the following properties:

 - CLUSTER_NAME: `simple-cluster`
 - DATACENTER_NAME: `us-east-1`
 - RACK_NAME: `us-east-1a`
 - INSTANCE_NUMBER: An automatically generated number attached to the pod name.

We picked the names to resemble something you can find in a cloud service but this is inconsequential, they can be set to anything you want.

To check if all the desired members are running, you should see the same number of entries from the following command as the number of members that was specified in `cluster.yaml`:

```console
kubectl -n scylla get pod -l app=scylla
```

You can also track the state of a Scylla cluster from its status. To check the current status of a Cluster, run:

```console
kubectl -n scylla describe ScyllaCluster simple-cluster
```

Checking the logs of the running scylla instances can be done like this:

```console
kubectl -n scylla logs simple-cluster-us-east-1-us-east-1a-0 scylla
```

### Configure host networking

To squeeze the most out of your deployment it is sometimes necessary to employ [host networking](https://kubernetes.io/docs/concepts/services-networking/).
To enable this the CRD allows for specifying a `network` parameter as such:

```yaml
version: 4.0.0
  agentVersion: 2.0.2
  cpuset: true
  network:
    hostNetworking: true
```

This will result in hosts network to be used for the Scylla Stateful Set deployment.

### Configure container kernel parameters

Sometimes it is necessary to run the container with different kernel parameters.
In order to support this, the Scylla Operator defines a cluster property `sysctls` that is a list of the desired key-value pairs to set.

___For example___: To increase the number events available for asynchronous IO processing in the Linux kernel to N set sysctls to`fs.aio-max-nr=N`.

```yaml
racks:
  - name: rack-name
    scyllaConfig: "scylla-config"
    scyllaAgentConfig: "scylla-agent-config"
    sysctls:
      - "fs.aio-max-nr=2097152"
```

### Deploying Alternator

The operator is also capable of deploying [Alternator](https://www.scylladb.com/alternator/) instead of the regular Scylla.
This requires a small change in the cluster definition.
Change the `cluster.yaml` file from this:
```yaml
spec:
  version: 4.0.0
  agentVersion: 2.0.2
  developerMode: true
  datacenter:
    name: us-east-1
```
to this:
```yaml
spec:
  version: 4.0.0
  alternator:
    port: 8000
    writeIsolation: only_rmw_uses_lwt
  agentVersion: 2.0.2
  developerMode: true
  datacenter:
    name: us-east-1
```
You can specify whichever port you want.

You must provide desired write isolation, supported values are: "always", "forbid_rmw", "only_rmw_uses_lwt".
Difference between those isolation levels can be found in Scylla Alternator documentation.

Once this is done the regular CQL ports will no longer be available, the cluster is a pure Alienator cluster.

## Accessing the Database

* From kubectl:

To get a cqlsh shell in your new Cluster:
```console
kubectl exec -n scylla -it simple-cluster-us-east-1-us-east-1a-0 -- cqlsh
> DESCRIBE KEYSPACES;
```


* From inside a Pod:

When you create a new Cluster,  automatically creates a Service for the clients to use in order to access the Cluster.
The service's name follows the convention `<cluster-name>-client`.
You can see this Service in your cluster by running:
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

If you are running the Alternator you can access the API on the port you specified using plain http.

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

## Configure Scylla Manager Agent

The operator creates a second container for each scylla instance that runs [Scylla Manager Agent](https://hub.docker.com/r/scylladb/scylla-manager-agent).
This container serves as a sidecar and it's the main endpoint for interacting with Scylla API.
The Scylla Manager Agent can be configured with various things such as the security token used to allow access to Scylla API and storage providers for backups.

To configure the agent you just create a new secret called _scylla-agent-config-secret_ and populate it with the contents in the `scylla-manager-agent.yaml` file like this:
```console
kubectl create secret -n scylla generic scylla-agent-config-secret --from-file scylla-manager-agent.yaml
```

See [Scylla Manager Agent configuration](https://docs.scylladb.com/operating-scylla/manager/2.0/agent-configuration-file/) for a complete reference of the Scylla Manager agent config file.

### Scylla Manager Agent auth token

Operator provisions Agent auth token by copying value from user provided config secret or auto generates it if it's empty.
To check which value is being used, decode content of `<cluster-name>-auth-token` secret. 
To change it simply remove the secret. Operator will create a new one. To pick up the change in the cluster, initiate a rolling restart.

## Set up monitoring

To set up monitoring using Prometheus and Grafana follow [this guide](monitoring.md).

## Scale Up

The operator supports scale up of a rack as well as addition of new racks. To make the changes, you can use:
```console
kubectl -n scylla edit ScyllaCluster simple-cluster
```
* To scale up a rack, change the `Spec.Members` field of the rack to the desired value.
* To add a new rack, append the `racks` list with a new rack. Remember to choose a different rack name for the new rack.
* After editing and saving the yaml, check your cluster's Status and Events for information on what's happening:
```console
kubectl -n scylla describe ScyllaCluster simple-cluster
```

## Benchmark with cassandra-stress

After deploying our cluster along with the monitoring, we can benchmark it using cassandra-stress and see its performance in Grafana. We have a mini cli that generates Kubernetes Jobs that run cassandra-stress against a cluster.

> Because cassandra-stress doesn't scale well to multiple cores, we use multiple jobs with a small core count for each

```bash

# Run a benchmark with 10 jobs, with 6 cpus and 50.000.000 operations each.
# Each Job will throttle throughput to 30.000 ops/sec for a total of 300.000 ops/sec.
hack/cass-stress-gen.py --num-jobs=10 --cpu=6 --memory=20G --ops=50000000 --limit=30000
kubectl apply -f scripts/cassandra-stress.yaml
```

Make sure you set the proper arguments in case you have altered things such as _name_ or _namespace_.

```bash
./hack/cass-stress-gen.py -h
usage: cass-stress-gen.py [-h] [--num-jobs NUM_JOBS] [--name NAME] [--namespace NAMESPACE] [--scylla-version SCYLLA_VERSION] [--host HOST] [--cpu CPU] [--memory MEMORY] [--ops OPS] [--threads THREADS] [--limit LIMIT]
                          [--connections-per-host CONNECTIONS_PER_HOST] [--print-to-stdout] [--nodeselector NODESELECTOR]

Generate cassandra-stress job templates for Kubernetes.

optional arguments:
  -h, --help            show this help message and exit
  --num-jobs NUM_JOBS   number of Kubernetes jobs to generate - defaults to 1
  --name NAME           name of the generated yaml file - defaults to cassandra-stress
  --namespace NAMESPACE
                        namespace of the cassandra-stress jobs - defaults to "default"
  --scylla-version SCYLLA_VERSION
                        version of scylla server to use for cassandra-stress - defaults to 4.0.0
  --host HOST           ip or dns name of host to connect to - defaults to scylla-cluster-client.scylla.svc
  --cpu CPU             number of cpus that will be used for each job - defaults to 1
  --memory MEMORY       memory that will be used for each job in GB, ie 2G - defaults to 2G * cpu
  --ops OPS             number of operations for each job - defaults to 10000000
  --threads THREADS     number of threads used for each job - defaults to 50 * cpu
  --limit LIMIT         rate limit for each job - defaults to no rate-limiting
  --connections-per-host CONNECTIONS_PER_HOST
                        number of connections per host - defaults to number of cpus
  --print-to-stdout     print to stdout instead of writing to a file
  --nodeselector NODESELECTOR
                        nodeselector limits cassandra-stress pods to certain nodes. Use as a label selector, eg. --nodeselector role=scylla
```
While the benchmark is running, open up Grafana and take a look at the monitoring metrics.

After the Jobs finish, clean them up with:
```bash
kubectl delete -f scripts/cassandra-stress.yaml
```

## Scale Down

The operator supports scale down of a rack. To make the changes, you can use:
```console
kubectl -n scylla edit ScyllaCluster simple-cluster
```
* To scale down a rack, change the `Spec.Members` field of the rack to the desired value.
* After editing and saving the yaml, check your cluster's Status and Events for information on what's happening:
```console
kubectl -n scylla describe ScyllaCluster simple-cluster
```

## Clean Up

To clean up all resources associated with this walk-through, you can run the commands below.

**NOTE:** this will destroy your database and delete all of its associated data.

```console
kubectl delete -f examples/generic/cluster.yaml
kubectl delete -f examples/common/operator.yaml
kubectl delete -f examples/common/cert-manager.yaml
```

## Troubleshooting

If the cluster does not come up, the first step would be to examine the operator's logs:

```console
kubectl -n scylla-operator logs deployment.apps/scylla-operator
```

If everything looks OK in the operator logs, you can also look in the logs for one of the Scylla instances:

```console
kubectl -n scylla logs simple-cluster-us-east-1-us-east-1a-0
```
