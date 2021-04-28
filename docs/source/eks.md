# Deploying Scylla on EKS

This guide is focused on deploying Scylla on EKS with improved performance.
Performance tricks used by the script won't work with different machine tiers.
It sets up the kubelets on EKS nodes to run with [static cpu policy](https://kubernetes.io/blog/2018/07/24/feature-highlight-cpu-manager/) and uses [local sdd disks](https://cloud.google.com/kubernetes-engine/docs/how-to/persistent-volumes/local-ssd) in RAID0 for maximum performance.

Most of the commands used to setup the Scylla cluster are the same for all environments
As such we have tried to keep them separate in the [general guide](generic.md).

## TL;DR;

If you don't want to run the commands step-by-step, you can just run a script that will set everything up for you:
```bash
# Edit according to your preference
EKS_ZONE=us-east-1a

# From inside the examples/eks folder
cd examples/eks
./eks.sh -z "$EKS_ZONE"
```

:warning: Make sure to pass a ZONE (ex.: eu-central-1a) and not a REGION (ex.: eu-central-1).

After you deploy, see how you can [benchmark your cluster with cassandra-stress](#benchmark-with-cassandra-stress).

## Walkthrough

### EKS Setup

#### Configure environment variables

First of all, we export all the configuration options as environment variables.
Edit according to your own environment.

```
EKS_ZONE=us-east-1a
CLUSTER_NAME=scylla-demo
```

#### Creating a EKS cluster

For this guide, we'll create a EKS cluster with the following:

* A NodeGroup of 3 `i3-2xlarge` Nodes, where the Scylla Pods will be deployed. These nodes will only accept pods having `scylla-clusters` toleration.

```
  - name: scylla-pool
    instanceType: i3.2xlarge
    desiredCapacity: 3
    labels:
      pool: "scylla-pool"
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

### Installing Required Tools

#### Installing script third party dependencies

Script requires several dependencies:
- Helm - See: https://docs.helm.sh/using_helm/#installing-helm
- eksctl - See: https://docs.aws.amazon.com/eks/latest/userguide/getting-started-eksctl.html
- kubectl - See: https://kubernetes.io/docs/tasks/tools/install-kubectl/


#### Install the local provisioner

We deploy the local volume provisioner, which will discover their mount points and make them available as PersistentVolumes.
```
helm install local-provisioner examples/common/provisioner
```

### Installing the Scylla Operator and Scylla

Now you can follow the [generic guide](generic.md) to launch your Scylla cluster in a highly performant environment.

#### Deleting a EKS cluster

Once you are done with your experiments delete your cluster using following command:

```
eksctl delete cluster <cluster_name>
```
