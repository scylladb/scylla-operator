# Deploying Scylla on EKS

This guide is focused on deploying Scylla on EKS with improved performance.
Performance tricks used by the script won't work with different machine tiers.
It sets up the kubelets on EKS nodes to run with [static cpu policy](https://kubernetes.io/blog/2018/07/24/feature-highlight-cpu-manager/) and uses [local sdd disks](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ssd-instance-store.html) in RAID0 for maximum performance.

Most of the commands used to setup the Scylla cluster are the same for all environments
As such we have tried to keep them separate in the [general guide](generic.md).

## TL;DR;

If you don't want to run the commands step-by-step, you can just run a script that will set everything up for you:
```bash
# Edit according to your preference
EKS_REGION=us-east-1
EKS_ZONES=us-east-1a,us-east-1b,us-east-1c

# From inside the examples/eks folder
cd examples/eks
./eks.sh -z "$EKS_ZONES" -r "$EKS_REGION"
```

After you deploy, see how you can [benchmark your cluster with cassandra-stress](#benchmark-with-cassandra-stress).

## Walkthrough

### EKS Setup

#### Configure environment variables

First of all, we export all the configuration options as environment variables.
Edit according to your own environment.

```
EKS_REGION=us-east-1
EKS_ZONES=us-east-1a,us-east-1b,us-east-1c
CLUSTER_NAME=scylla-demo
```

#### Creating an EKS cluster

For this guide, we'll create an EKS cluster with the following:

* A NodeGroup of 3 `i3-2xlarge` Nodes, where the Scylla Pods will be deployed. These nodes will only accept pods having `scylla-clusters` toleration.

```
  - name: scylla-pool
    instanceType: i3.2xlarge
    desiredCapacity: 3
    labels:
      scylla.scylladb.com/node-type: scylla
    taints:
      role: "scylla-clusters:NoSchedule"
    ssh:
      allow: true
    kubeletExtraConfig:
      cpuManagerPolicy: static
```

* A NodeGroup of 4 `c4.2xlarge` Nodes to deploy `cassandra-stress` later on. These nodes will only accept pods having `cassandra-stress` toleration.

```
  - name: cassandra-stress-pool
    instanceType: c4.2xlarge
    desiredCapacity: 4
    labels:
      pool: "cassandra-stress-pool"
    taints:
      role: "cassandra-stress:NoSchedule"
    ssh:
      allow: true
```

* A NodeGroup of 1 `i3.large` Node, where the monitoring stack and operator will be deployed.
```
  - name: monitoring-pool
    instanceType: i3.large
    desiredCapacity: 1
    labels:
      pool: "monitoring-pool"
    ssh:
      allow: true
```

### Prerequisites

#### Installing script third party dependencies

Script requires several dependencies:
- eksctl - See: https://docs.aws.amazon.com/eks/latest/userguide/getting-started-eksctl.html
- kubectl - See: https://kubernetes.io/docs/tasks/tools/install-kubectl/

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

### Installing the Scylla Operator and Scylla

Now you can follow the [generic guide](generic.md) to launch your Scylla cluster in a highly performant environment.

#### Accessing the database

Instructions on how to access the database can also be found in the [generic guide](generic.md).

### Deleting an EKS cluster

Once you are done with your experiments delete your cluster using the following command:

```
eksctl delete cluster "${CLUSTER_NAME}"
```
