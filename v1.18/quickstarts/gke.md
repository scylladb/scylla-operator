# Deploying ScyllaDB on GKE

This is a quickstart guide to help you set up a basic GKE cluster quickly with local NVMes and solid performance.

This is by no means a complete guide, and you should always consult your Kubernetes provider’s documentation.

## Creating a GKE cluster

First, we need to create a kubelet config to configure [static CPU policy](https://operator.docs.scylladb.com/v1.18/installation/kubernetes-prerequisites.md#static-cpu-policy):

```bash

cat > systemconfig.yaml <<EOF
kubeletConfig:
  cpuManagerPolicy: static
EOF
```

Then we’ll create a GKE cluster with the following:

```bash
gcloud container \
clusters create 'my-k8s-cluster' \
--zone='us-central1' \
--cluster-version="latest" \
--machine-type='n2-standard-8' \
--num-nodes='2' \
--disk-type='pd-ssd' --disk-size='20' \
--image-type='UBUNTU_CONTAINERD' \
--enable-stackdriver-kubernetes \
--no-enable-autoupgrade \
--no-enable-autorepair
```

and then we’ll create a dedicated pool with NVMes for ScyllaDB

```bash
gcloud container \
node-pools create 'scyllaclusters' \
--zone='us-central1' \
--cluster='my-k8s-cluster' \
--node-version="latest" \
--machine-type='n2-standard-16' \
--num-nodes='4' \
--disk-type='pd-ssd' --disk-size='20' \
--local-nvme-ssd-block='count=4' \
--image-type='UBUNTU_CONTAINERD' \
--system-config-from-file='systemconfig.yaml' \
--no-enable-autoupgrade \
--no-enable-autorepair \
--node-labels='scylla.scylladb.com/node-type=scylla' \
--node-taints='scylla-operator.scylladb.com/dedicated=scyllaclusters:NoSchedule'
```

## Installing Kubernetes prerequisites

### xfsprogs

Beginning with GKE version `1.32.1-gke.1002000`, the Ubuntu image used by GKE clusters no longer provides the `xfsprogs` package by default.
This package is required to format the local NVMe disks used by ScyllaDB. Please refer to the [xfsprogs section](https://operator.docs.scylladb.com/v1.18/installation/kubernetes-prerequisites.md#xfsprogs) of the Kubernetes prerequisites page for more details.

## Deploying Scylla Operator

To deploy Scylla Operator follow the [installation guide](https://operator.docs.scylladb.com/v1.18/installation/overview.md).

## Creating ScyllaDB

To deploy a ScyllaDB cluster please head to [our dedicated section on the topic](https://operator.docs.scylladb.com/v1.18/resources/scyllaclusters/basics.md).

## Accessing ScyllaDB

We also have a whole section dedicated to [how you can access the ScyllaDB cluster you’ve just created](https://operator.docs.scylladb.com/v1.18/resources/scyllaclusters/clients/index.md).

### Deleting a GKE cluster

Once you are done with your experiments you can delete your cluster using the following command:

```default
gcloud container clusters delete --zone='us-central1' 'my-k8s-cluster'
```
