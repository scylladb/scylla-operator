# Set up multi-DC infrastructure

This guide describes how to create multiple interconnected Kubernetes clusters in different regions that can serve as a platform for [deploying a multi-datacenter ScyllaDB cluster](../../deploy-scylladb/deploy-multi-dc-cluster.md).

The guide walks through creating and configuring clusters in two distinct regions. The provided values are exemplary and can be adjusted to your needs.

## Prerequisites

- kubectl — A command line tool for working with Kubernetes clusters. See [Install Tools](https://kubernetes.io/docs/tasks/tools/) in Kubernetes documentation.
- Cloud provider CLI (see the relevant tab below).

## Set up the infrastructure

::::::::::{tabs}

:::::::::{group-tab} Amazon EKS

### Prerequisites

- eksctl — A command line tool for working with EKS clusters. See [Getting started with Amazon EKS – eksctl](https://docs.aws.amazon.com/eks/latest/userguide/getting-started-eksctl.html) in AWS documentation.

### Create EKS clusters

#### Create the first EKS cluster

Below is the required specification for the first cluster:

```yaml
apiVersion: eksctl.io/v1alpha5
kind: ClusterConfig

metadata:
  name: scylladb-us-east-1
  region: us-east-1

availabilityZones:
- us-east-1a
- us-east-1b
- us-east-1c

vpc:
  cidr: 10.0.0.0/16

nodeGroups:
  ...
```

Save it as `cluster-us-east-1.yaml` and deploy:

```shell
eksctl create cluster -f=cluster-us-east-1.yaml
```

Retrieve the cluster's context for future operations:

```shell
kubectl config current-context
```

Save it as `CONTEXT_DC1` for use in subsequent commands.

#### Deploy ScyllaDB Operator

Deploy ScyllaDB Operator in this cluster by following the [installation guide](../index.md).

#### Create the second EKS cluster

:::{caution}
The VPCs of the two EKS clusters must have non-overlapping IPv4 network ranges.
:::

Below is the required specification for the second cluster:

```yaml
apiVersion: eksctl.io/v1alpha5
kind: ClusterConfig

metadata:
  name: scylladb-us-east-2
  region: us-east-2

availabilityZones:
- us-east-2a
- us-east-2b
- us-east-2c

vpc:
  cidr: 172.16.0.0/16

nodeGroups:
  ...
```

Follow analogous steps to create the second EKS cluster and deploy ScyllaDB Operator.

### Configure the network

The clusters each have a dedicated VPC. To route traffic between them, create a [VPC peering connection](https://docs.aws.amazon.com/vpc/latest/peering/what-is-vpc-peering.html).

#### Create VPC peering

Refer to [Create a VPC peering connection](https://docs.aws.amazon.com/vpc/latest/peering/create-vpc-peering-connection.html) in AWS documentation.

In this example, the ID of the created VPC peering connection is `pcx-08077dcc008fbbab6`.

#### Update route tables

Add a route to the route tables associated with all subnets in both VPCs. The destination is the CIDR of the other VPC and the target is the VPC peering connection ID.

Example route tables:

| Route table | Destination | Target |
|---|---|---|
| eksctl-scylladb-us-east-1-cluster/PublicRouteTable | 10.0.0.0/16 | local |
| | 172.16.0.0/16 | pcx-08077dcc008fbbab6 |
| eksctl-scylladb-us-east-2-cluster/PublicRouteTable | 172.16.0.0/16 | local |
| | 10.0.0.0/16 | pcx-08077dcc008fbbab6 |

Refer to [Update your route tables for a VPC peering connection](https://docs.aws.amazon.com/vpc/latest/peering/vpc-peering-routing.html) in AWS documentation.

#### Update security groups

Allow traffic between the two VPCs by adding inbound rules to their shared security groups:

| Security group | Type | Protocol | Port range | Source |
|---|---|---|---|---|
| eksctl-scylladb-us-east-1-cluster-ClusterSharedNodeSecurityGroup-... | All traffic | All | All | 172.16.0.0/16 |
| eksctl-scylladb-us-east-2-cluster-ClusterSharedNodeSecurityGroup-... | All traffic | All | All | 10.0.0.0/16 |

:::::::::

:::::::::{group-tab} Google Cloud GKE

### Prerequisites

- gcloud CLI — Google Cloud Command Line Interface. See [Install the Google Cloud CLI](https://cloud.google.com/sdk/docs/install-sdk) in GCP documentation.

### Create and configure a VPC network

For inter-Kubernetes networking, create a shared VPC with dedicated subnets for each cluster.

#### Create the VPC network

```shell
gcloud compute networks create scylladb --subnet-mode=custom
```

#### Create VPC network subnets

Create a subnet for the first cluster in `us-east1`:

```shell
gcloud compute networks subnets create scylladb-us-east1 \
    --region=us-east1 \
    --network=scylladb \
    --range=10.0.0.0/20 \
    --secondary-range='cluster=10.1.0.0/16,services=10.2.0.0/20'
```

Create a subnet for the second cluster in `us-west1`:

```shell
gcloud compute networks subnets create scylladb-us-west1 \
    --region=us-west1 \
    --network=scylladb \
    --range=172.16.0.0/20 \
    --secondary-range='cluster=172.17.0.0/16,services=172.18.0.0/20'
```

:::{caution}
The IPv4 address ranges of the subnets allocated for the GKE clusters must not overlap.
:::

Refer to [Create a VPC-native cluster](https://cloud.google.com/kubernetes-engine/docs/how-to/alias-ips) and [Alias IP ranges](https://cloud.google.com/vpc/docs/alias-ip) in GKE documentation.

### Create GKE clusters

#### Create the first GKE cluster

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

Retrieve the cluster's context for future operations:

```shell
kubectl config current-context
```

Save it as `CONTEXT_DC1` for use in subsequent commands.

#### Deploy ScyllaDB Operator

Deploy ScyllaDB Operator in this cluster by following the [installation guide](../index.md).

#### Create the second GKE cluster

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

Follow analogous steps to deploy ScyllaDB Operator in the second cluster.

### Configure firewall rules

GKE creates ingress firewall rules that enable instances within a cluster to communicate. To establish interconnectivity between the two clusters, add the allocated Pod IPv4 address ranges to their corresponding source ranges.

Retrieve the firewall rule for the first cluster (format: `gke-[cluster-name]-[cluster-hash]-all`):

```shell
gcloud compute firewall-rules list --filter='name~gke-scylladb-us-east1-.*-all'
```

```console
NAME                                NETWORK   DIRECTION  PRIORITY  ALLOW                     DENY  DISABLED
gke-scylladb-us-east1-f17db261-all  scylladb  INGRESS    1000      udp,icmp,esp,ah,sctp,tcp        False
```

Update the rule with Pod CIDR ranges from both clusters:

```shell
gcloud compute firewall-rules update gke-scylladb-us-east1-f17db261-all --source-ranges='10.1.0.0/16,172.17.0.0/16'
```

Follow analogous steps for the second cluster's firewall rule:

```shell
gcloud compute firewall-rules update gke-scylladb-us-west1-0bb60902-all --source-ranges='10.1.0.0/16,172.17.0.0/16'
```

Refer to [Automatically created firewall rules](https://cloud.google.com/kubernetes-engine/docs/concepts/firewall-rules) in GKE documentation.

:::::::::

::::::::::

## Next steps

With the infrastructure prepared, proceed to [Deploy a multi-datacenter cluster](../../deploy-scylladb/deploy-multi-dc-cluster.md).

## Related pages

- [Deploy a multi-datacenter cluster](../../deploy-scylladb/deploy-multi-dc-cluster.md)
- [Set up dedicated node pools](index.md)
