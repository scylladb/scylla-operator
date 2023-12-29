# Build multiple GKE clusters with inter-Kubernetes networking

This document describes the process of creating multiple GKE clusters in a shared VPC and explains the steps necessary for configuring inter-Kubernetes networking between clusters in different regions.
The interconnected clusters can serve as a platform for [deploying a Multi Datacenter ScyllaDB cluster](multidc.md).

This guide will walk you through the process of creating and configuring GKE clusters in two distinct regions. Although it is only an example setup, it can easily be built upon to create infrastructure tailored to your specific needs.
For simplicity, several predefined values are used throughout the document. The values are only exemplary and can be adjusted to your preference.

## Prerequisites

To follow the below guide, you first need to install and configure the following tools that you will need to create and manage GCP and Kubernetes resources:
- gcloud CLI - Google Cloud Command Line Interface, a command line tool for working with Google Cloud resources and services directly.
- kubectl – A command line tool for working with Kubernetes clusters.

See [Install the Google Cloud CLI](https://cloud.google.com/sdk/docs/install-sdk) in GCP documentation and [Install Tools](https://kubernetes.io/docs/tasks/tools/) in Kubernetes documentation for reference.

## Create and configure a VPC network

For the clusters to have inter-Kubernetes networking, you will create a virtual network shared between all the instances, with dedicated subnets for each of the clusters.
To create the subnets manually, create the network in custom subnet mode.

### Create the VPC network

Run the below command to create the network:
```shell
gcloud compute networks create scylladb --subnet-mode=custom
```

With the VPC network created, create a dedicated subnet with secondary CIDR ranges for their Pod and Service pools in each region which the clusters will reside in.

### Create VPC network subnets

To create a subnet for the first cluster in region `us-east1`, run the below command:
```shell
gcloud compute networks subnets create scylladb-us-east1 \
    --region=us-east1 \
    --network=scylladb \
    --range=10.0.0.0/20 \
    --secondary-range='cluster=10.1.0.0/16,services=10.2.0.0/20'
```

To create a subnet for the second cluster in region `us-west1`, run the below command:
```shell
gcloud compute networks subnets create scylladb-us-west1 \
    --region=us-west1 \
    --network=scylladb \
    --range=172.16.0.0/20 \
    --secondary-range='cluster=172.17.0.0/16,services=172.18.0.0/20'
```

:::{caution}
It is required that the IPv4 address ranges of the subnets allocated for the GKE clusters do not overlap.
:::

Refer to [Create a VPC-native cluster](https://cloud.google.com/kubernetes-engine/docs/how-to/alias-ips) and [Alias IP ranges](https://cloud.google.com/vpc/docs/alias-ip) in GKE documentation for more information about VPC native clusters and alias IP ranges.

## Create GKE clusters

With the VPC network created, you will now create two VPC native GKE clusters in dedicated regions.

### Create the first GKE cluster

Run the following command to create the first GKE cluster in the `us-east1` region:
```shell
gcloud container clusters create scylladb-us-east1 \
    --location=us-east1-b \
    --node-locations='us-east1-b,us-east1-c' \
    --machine-type=n1-standard-8 \
    --num-nodes=1 \
    --disk-type=pd-ssd \
    --disk-size=20 \
    --image-type=UBUNTU_CONTAINERD \
    --no-enable-autoupgrade \
    --no-enable-autorepair \
    --enable-ip-alias \
    --network=scylladb \
    --subnetwork=scylladb-us-east1 \
    --cluster-secondary-range-name=cluster \
    --services-secondary-range-name=services
```

Refer to [Creating a GKE cluster](../gke.md#creating-a-gke-cluster) section of ScyllaDB Operator documentation for more information regarding the configuration and deployment of additional node pools, including the one dedicated for ScyllaDB nodes.

You will need to get the cluster's context for future operations. To do so, use the below command:
```shell
kubectl config current-context
```

For any `kubectl` commands that you will want to run against this cluster, use the `--context` flag with the value returned by the above command.

#### Deploy ScyllaDB Operator

Once the cluster is ready, refer to [Deploying Scylla on a Kubernetes Cluster](../generic.md) to deploy the ScyllaDB Operator and its prerequisites.

#### Prepare nodes for running ScyllaDB

Then, prepare the nodes for running ScyllaDB workloads and deploy a volume provisioner following the steps described in [Deploying Scylla on GKE](../gke.md) page of the documentation.

### Create the second GKE cluster

Run the following command to create the second GKE cluster in the `us-west1` region:
```shell
gcloud container clusters create scylladb-us-west1 \
    --location=us-west1-b \
    --node-locations='us-west1-b,us-west1-c' \
    --machine-type=n1-standard-8 \
    --num-nodes=1 \
    --disk-type=pd-ssd \
    --disk-size=20 \
    --image-type=UBUNTU_CONTAINERD \
    --no-enable-autoupgrade \
    --no-enable-autorepair \
    --enable-ip-alias \
    --network=scylladb \
    --subnetwork=scylladb-us-west1 \
    --cluster-secondary-range-name=cluster \
    --services-secondary-range-name=services
```

Follow analogous steps to create the second GKE cluster and prepare it for running ScyllaDB.

## Configure the firewall rules

When creating a cluster, GKE creates several ingress firewall rules that enable the instances to communicate with each other.
To establish interconnectivity between the two created Kubernetes clusters, you will now add the allocated IPv4 address ranges to their corresponding source address ranges.

First, retrieve the name of the firewall rule associated with the first cluster, which permits traffic between all Pods on a cluster, as required by the Kubernetes networking model.
The rule name is in the following format: `gke-[cluster-name]-[cluster-hash]-all`.

To retrieve it, run the below command:
```shell
gcloud compute firewall-rules list --filter='name~gke-scylladb-us-east1-.*-all'
```

The output should resemble the following:
```console
NAME                                NETWORK   DIRECTION  PRIORITY  ALLOW                     DENY  DISABLED
gke-scylladb-us-east1-f17db261-all  scylladb  INGRESS    1000      udp,icmp,esp,ah,sctp,tcp        False
```

Modify the rule by updating the rule's source ranges with the allocated Pod IPv4 address ranges of both clusters:
```shell
gcloud compute firewall-rules update gke-scylladb-us-east1-f17db261-all --source-ranges='10.1.0.0/16,172.17.0.0/16'
```

Follow the analogous steps for the other cluster. In this example, its corresponding firewall rule name is `gke-scylladb-us-west1-0bb60902-all`. To update it, you would run:
```shell
gcloud compute firewall-rules update gke-scylladb-us-west1-0bb60902-all --source-ranges='10.1.0.0/16,172.17.0.0/16'
```

Refer to [Automatically created firewall rules](https://cloud.google.com/kubernetes-engine/docs/concepts/firewall-rules) in GKE documentation for more information.

---

Having followed the above steps, you should now have a platform prepared for deploying a multi-datacenter ScyllaDB cluster.
Refer to [Deploy a multi-datacenter ScyllaDB cluster in multiple interconnected Kubernetes clusters](multidc.md) in ScyllaDB Operator documentation for guidance.
