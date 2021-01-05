# Deploying Scylla on GKE

This guide is focused on deploying Scylla on GKE with maximum performance (without any persistence guarantees).
It sets up the kubelets on GKE nodes to run with [static cpu policy](https://kubernetes.io/blog/2018/07/24/feature-highlight-cpu-manager/) and uses [local sdd disks](https://cloud.google.com/kubernetes-engine/docs/how-to/persistent-volumes/local-ssd) in RAID0 for maximum performance.

Most of the commands used to setup the Scylla cluster are the same for all environments
As such we have tried to keep them separate in the [general guide](generic.md).

## TL;DR;

If you don't want to run the commands step-by-step, you can just run a script that will set everything up for you:
```bash
# Edit according to your preference
GCP_USER=$(gcloud config list account --format "value(core.account)")
GCP_PROJECT=$(gcloud config list project --format "value(core.project)")
GCP_ZONE=us-west1-b

# From inside the examples/gke folder
cd examples/gke
./gke.sh "$GCP_USER" "$GCP_PROJECT" "$GCP_ZONE"

# Example:
# ./gke.sh yanniszark@arrikto.com gke-demo-226716 us-west1-b
```

:warning: Make sure to pass a ZONE (ex.: us-west1-b) and not a REGION (ex.: us-west1) or it will deploy nodes in each ZONE available in the region.

After you deploy, see how you can [benchmark your cluster with cassandra-stress](#benchmark-with-cassandra-stress).

## Walkthrough

### Google Kubernetes Engine Setup

#### Configure environment variables

First of all, we export all the configuration options as environment variables.
Edit according to your own environment.

```
GCP_USER=$(gcloud config list account --format "value(core.account)")
GCP_PROJECT=$(gcloud config list project --format "value(core.project)")
GCP_REGION=us-west1
GCP_ZONE=us-west1-b
CLUSTER_NAME=scylla-demo
```

#### Creating a GKE cluster

For this guide, we'll create a GKE cluster with the following:

1. A NodePool of 3 `n1-standard-32` Nodes, where the Scylla Pods will be deployed. Each of these Nodes has 8 local SSDs attached, which will later be combined into a RAID0 array. It is important to disable `autoupgrade` and `autorepair`, since they increase the frequency of loss of data on local SSDs. 

```
gcloud beta container --project "${GCP_PROJECT}" \
clusters create "${CLUSTER_NAME}" --username "admin" \
--zone "${GCP_ZONE}" \
--cluster-version "1.15.11-gke.3" \
--machine-type "n1-standard-32" \
--num-nodes "3" \
--disk-type "pd-ssd" --disk-size "20" \
--local-ssd-count "8" \
--node-taints role=scylla-clusters:NoSchedule \
--image-type "UBUNTU" \
--enable-stackdriver-kubernetes \
--no-enable-autoupgrade --no-enable-autorepair
```

2. A NodePool of 2 `n1-standard-32` Nodes to deploy `cassandra-stress` later on.

```
gcloud beta container --project "${GCP_PROJECT}" \
node-pools create "cassandra-stress-pool" \
--cluster "${CLUSTER_NAME}" \
--zone "${GCP_ZONE}" \
--machine-type "n1-standard-32" \
--num-nodes "2" \
--disk-type "pd-ssd" --disk-size "20" \
--node-taints role=cassandra-stress:NoSchedule \
--image-type "UBUNTU" \
--no-enable-autoupgrade --no-enable-autorepair
```

3. A NodePool of 1 `n1-standard-8` Node, where the operator and the monitoring stack will be deployed.
```
gcloud beta container --project "${GCP_PROJECT}" \
node-pools create "operator-pool" \
--cluster "${CLUSTER_NAME}" \
--zone "${GCP_ZONE}" \
--machine-type "n1-standard-8 \
--num-nodes "1" \
--disk-type "pd-ssd" --disk-size "20" \
--image-type "UBUNTU" \
--no-enable-autoupgrade --no-enable-autorepair
```

#### Setting Yourself as `cluster-admin`
> (By default GKE doesn't give you the necessary RBAC permissions)

Get the credentials for your new cluster
```
gcloud container clusters get-credentials "${CLUSTER_NAME}" --zone="${GCP_ZONE}"
```

Create a ClusterRoleBinding for your user.
In order for this to work you need to have at least permission `container.clusterRoleBindings.create`.
The easiest way to obtain this permission is to enable the `Kubernetes Engine Admin` role for your user in the GCP IAM web interface.
```
kubectl create clusterrolebinding cluster-admin-binding --clusterrole cluster-admin --user "${GCP_USER}"
```


### Installing Required Tools 

#### Installing Helm

If you don't have Helm installed, Go to the [helm docs](https://docs.helm.sh/using_helm/#installing-helm) to get the binary for your distro.

#### Install RAID DaemonSet

To combine the local disks together in RAID0 arrays, we deploy a DaemonSet to do the work for us.

```
kubectl apply -f examples/gke/raid-daemonset.yaml
```

#### Install the local provisioner

After combining the local SSDs into RAID0 arrays, we deploy the local volume provisioner, which will discover their mount points and make them available as PersistentVolumes.
```
helm install local-provisioner examples/common/provisioner
```

#### Install `cpu-policy` Daemonset

Scylla achieves top-notch performance by pinning the CPUs it uses. To enable this behaviour in Kubernetes, the kubelet must be configured with the [static CPU policy](https://kubernetes.io/blog/2018/07/24/feature-highlight-cpu-manager/). To configure the kubelets in the `scylla` and `cassandra-stress` NodePools, we deploy a DaemonSet to do the work for us. You'll notice the Nodes getting cordoned for a little while, but then everything will come back to normal.
```
kubectl apply -f examples/gke/cpu-policy-daemonset.yaml
```

### Deploy Cert Manager

You can either follow [upsteam instructions](https://cert-manager.io/docs/installation/kubernetes/) or use following command:

```console
kubectl apply -f examples/common/cert-manager.yaml
```

This will install Cert Manager with self signed certificate.  

Once it's deployed, wait until all Cert Manager pods will enter into Running state:

```console
kubectl wait -n cert-manager --for=condition=ready pod -l app=cert-manager --timeout=60s
```


### Installing the Scylla Operator

In order for the example to work you need to modify the cluster definition in the following way:

```
sed -i "s/<gcp_region>/${GCP_REGION}/g;s/<gcp_zone>/${GCP_ZONE}/g" examples/gke/cluster.yaml
```

This will inject your region and zone into the cluster definition so that it matches the kubernetes cluster you just created.

Now you can follow the [generic guide](generic.md) to install the operator and launch your Scylla cluster in a highly performant environment.
