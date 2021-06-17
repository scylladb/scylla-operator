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
./gke.sh -u "$GCP_USER" -p "$GCP_PROJECT" -z "$GCP_ZONE"

# Example:
# ./gke.sh -u yanniszark@arrikto.com -p gke-demo-226716 -z us-west1-b
```

:warning: Make sure to pass a ZONE (ex.: us-west1-b) and not a REGION (ex.: us-west1) or it will deploy nodes in each ZONE available in the region.

After you deploy, see how you can [benchmark your cluster with cassandra-stress](#benchmark-with-cassandra-stress).

## Walkthrough

### Google Kubernetes Engine Setup

#### Configure environment variables

First of all, we export all the configuration options as environment variables.
Edit according to your own environment.

```
GCP_USER=$( gcloud config list account --format "value(core.account)" )
GCP_PROJECT=$( gcloud config list project --format "value(core.project)" )
GCP_REGION=us-west1
GCP_ZONE=us-west1-b
CLUSTER_NAME=scylla-demo
CLUSTER_VERSION=$( gcloud container get-server-config --zone ${GCP_ZONE} --format "value(validMasterVersions[0])" )
```

#### Creating a GKE cluster

First we need to change kubelet CPU Manager policy to static by providing a config file. Create file called `systemconfig.yaml` with the following content:
```
kubeletConfig:
  cpuManagerPolicy: static
```

Then we'll create a GKE cluster with the following:

1. A NodePool of 4 `n1-standard-32` Nodes, where the Scylla Pods will be deployed. Each of these Nodes has 8 local SSDs attached, which will later be combined into a RAID0 array. It is important to disable `autoupgrade` and `autorepair`, since they increase the frequency of loss of data on local SSDs.

    ```
    gcloud container --project "${GCP_PROJECT}" \
    clusters create "${CLUSTER_NAME}" --username "admin" \
    --zone "${GCP_ZONE}" \
    --cluster-version "${CLUSTER_VERSION}" \
    --node-version "${CLUSTER_VERSION}" \
    --machine-type "n1-standard-32" \
    --num-nodes "4" \
    --disk-type "pd-ssd" --disk-size "20" \
    --local-ssd-count "8" \
    --node-taints role=scylla-clusters:NoSchedule \
    --image-type "UBUNTU_CONTAINERD" \
    --system-config-from-file=systemconfig.yaml \
    --enable-stackdriver-kubernetes \
    --no-enable-autoupgrade \
    --no-enable-autorepair
    ```

2. A NodePool of 2 `n1-standard-32` Nodes to deploy `cassandra-stress` later on.

    ```
    gcloud container --project "${GCP_PROJECT}" \
    node-pools create "cassandra-stress-pool" \
    --cluster "${CLUSTER_NAME}" \
    --zone "${GCP_ZONE}" \
    --node-version "${CLUSTER_VERSION}" \
    --machine-type "n1-standard-32" \
    --num-nodes "2" \
    --disk-type "pd-ssd" --disk-size "20" \
    --node-taints role=cassandra-stress:NoSchedule \
    --image-type "UBUNTU_CONTAINERD" \
    --no-enable-autoupgrade \
    --no-enable-autorepair
    ```

3. A NodePool of 1 `n1-standard-8` Node, where the operator and the monitoring stack will be deployed.
    ```
    gcloud container --project "${GCP_PROJECT}" \
    node-pools create "operator-pool" \
    --cluster "${CLUSTER_NAME}" \
    --zone "${GCP_ZONE}" \
    --node-version "${CLUSTER_VERSION}" \
    --machine-type "n1-standard-8" \
    --num-nodes "1" \
    --disk-type "pd-ssd" --disk-size "20" \
    --image-type "UBUNTU_CONTAINERD" \
    --no-enable-autoupgrade \
    --no-enable-autorepair
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

### Deploy Scylla cluster
In order for the example to work you need to modify the cluster definition in the following way:

```
sed -i "s/<gcp_region>/${GCP_REGION}/g;s/<gcp_zone>/${GCP_ZONE}/g" examples/gke/cluster.yaml
```

This will inject your region and zone into the cluster definition so that it matches the kubernetes cluster you just created.

Now you can follow the [generic guide](generic.md) to install the operator and launch your Scylla cluster in a highly performant environment.
