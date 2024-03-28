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

1. A NodePool of 2 `n1-standard-8` Nodes, where the operator and the monitoring stack will be deployed. These are generic Nodes and their free capacity can be used for other purposes. 
   ```
   gcloud container \
   clusters create "${CLUSTER_NAME}" \
   --cluster-version "${CLUSTER_VERSION}" \
   --node-version "${CLUSTER_VERSION}" \
   --machine-type "n1-standard-8" \
   --num-nodes "2" \
   --disk-type "pd-ssd" --disk-size "20" \
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
   
3. A NodePool of 4 `n1-standard-32` Nodes, where the Scylla Pods will be deployed. Each of these Nodes has 8 local NVMe SSDs attached, which are provided as [raw block devices](https://cloud.google.com/kubernetes-engine/docs/how-to/persistent-volumes/local-ssd#raw-block). It is important to disable `autoupgrade` and `autorepair`. Automatic cluster upgrade or node repair has a hard timeout after which it no longer respect PDBs and force deletes the Compute Engine instances, which also deletes all data on the local SSDs. At this point, it's better to handle upgrades manually, with more control over the process and error handling.
   ```
   gcloud container \
   node-pools create "scylla-pool" \
   --cluster "${CLUSTER_NAME}" \
   --node-version "${CLUSTER_VERSION}" \
   --machine-type "n1-standard-32" \
   --num-nodes "4" \
   --disk-type "pd-ssd" --disk-size "20" \
   --local-nvme-ssd-block count="8" \
   --node-taints role=scylla-clusters:NoSchedule \
   --node-labels scylla.scylladb.com/node-type=scylla \
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


### Prerequisites

#### Setting up nodes for ScyllaDB

ScyllaDB, except when in developer mode, requires storage with XFS filesystem. The local NVMes from the cloud provider usually come as individual devices. To use their full capacity together, you'll first need to form a RAID array from those disks.
`NodeConfig` performs the necessary RAID configuration and XFS filesystem creation, as well as it optimizes the nodes. You can read more about it in [Performance tuning](performance.md) section of ScyllaDB Operator's documentation.

Deploy `NodeConfig` to let it take care of the above operations:
```
kubectl apply --server-side -f examples/gke/nodeconfig-alpha.yaml
```

#### Deploying Local Volume Provisioner

Afterwards, deploy ScyllaDB's [Local Volume Provisioner](https://github.com/scylladb/k8s-local-volume-provisioner), capable of dynamically provisioning PersistentVolumes for your ScyllaDB clusters on mounted XFS filesystems, earlier created over the configured RAID0 arrays.
```
kubectl -n local-csi-driver apply --server-side -f examples/common/local-volume-provisioner/local-csi-driver/
kubectl apply --server-side -f examples/common/local-volume-provisioner/storageclass_xfs.yaml
```

### Deploy Scylla cluster
In order for the example to work you need to modify the cluster definition in the following way:

```
sed -i "s/<gcp_region>/${GCP_REGION}/g;s/<gcp_zone>/${GCP_ZONE}/g" examples/gke/cluster.yaml
```

This will inject your region and zone into the cluster definition so that it matches the kubernetes cluster you just created.

### Installing the Scylla Operator and Scylla

Now you can follow the [generic guide](generic.md) to install the operator and launch your Scylla cluster in a highly performant environment.

#### Accessing the database

Instructions on how to access the database can also be found in the [generic guide](generic.md).

### Deleting a GKE cluster

Once you are done with your experiments delete your cluster using the following command:

```
gcloud container --project "${GCP_PROJECT}" clusters delete --zone "${GCP_ZONE}" "${CLUSTER_NAME}"
```
