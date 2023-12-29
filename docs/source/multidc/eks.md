# Build multiple Amazon EKS clusters with inter-Kubernetes networking

This document describes the process of creating multiple Amazon EKS clusters in different regions, using separate VPCs, and explains the steps necessary for configuring inter-Kubernetes networking between the clusters.
The interconnected clusters can serve as a platform for [deploying a multi-datacenter ScyllaDB cluster](multidc.md).

This guide will walk you through the process of creating and configuring EKS clusters in two distinct regions. Although it is only an example setup, it can easily be built upon to create infrastructure tailored to your specific needs.
For simplicity, several predefined values are used throughout the document. The values are only exemplary and can be adjusted to your preference.

## Prerequisites

To follow the below guide, you first need to install and configure the tools that you will need to create and manage AWS and Kubernetes resources:
- eksctl – A command line tool for working with EKS clusters.
- kubectl – A command line tool for working with Kubernetes clusters.

For more information see [Getting started with Amazon EKS – eksctl](https://docs.aws.amazon.com/eks/latest/userguide/getting-started-eksctl.html) in AWS documentation.

## Create EKS clusters

### Create the first EKS cluster

Below is the required specification for the first cluster.

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

Specify the first cluster's configuration file and save it as `cluster-us-east-1.yaml`.
Refer to [Creating an EKS cluster](../eks.md#creating-an-eks-cluster) section of ScyllaDB Operator documentation for the reference of the configuration of node groups.

To deploy the first cluster, use the below command:
```shell
eksctl create cluster -f=cluster-us-east-1.yaml
```

Run the following command to learn the status and VPC ID of the cluster:
```shell
eksctl get cluster --name=scylladb-us-east-1 --region=us-east-1
```

You will need to get the cluster's context for future operations. To do so, use the below command:
```shell
kubectl config current-context
```

For any `kubectl` commands that you will want to run against this cluster, use the `--context` flag with the value returned by the above command.

#### Deploy ScyllaDB Operator

Once the cluster is ready, refer to [Deploying Scylla on a Kubernetes Cluster](../generic.md) to deploy the ScyllaDB Operator and its prerequisites.

#### Prepare nodes for running ScyllaDB

Then, prepare the nodes for running ScyllaDB workloads and deploy a volume provisioner following the steps described in [Deploying Scylla on EKS](../eks.md#prerequisites) in ScyllaDB Operator documentation.

### Create the second EKS cluster

Below is the required specification for the second cluster. As was the case with the first cluster, the provided values are only exemplary and can be adjusted according to your needs.

:::{caution}
It is required that the VPCs of the two EKS clusters have non-overlapping IPv4 network ranges.
:::

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

Follow analogous steps to create the second EKS cluster and prepare it for running ScyllaDB.

## Configure the network

The prepared Kubernetes clusters each have a dedicated VPC network.
To be able to route the traffic between the two VPC networks, you need to create a networking connection between them, otherwise known as [VPC peering](https://docs.aws.amazon.com/vpc/latest/peering/what-is-vpc-peering.html).

### Create VPC peering

Refer to [Create a VPC peering connection](https://docs.aws.amazon.com/vpc/latest/peering/create-vpc-peering-connection.html#create-vpc-peering-connection-local) in AWS documentation for instructions on creating a VPC peering connection between the two earlier created VPCs.

In this example, the ID of the created VPC peering connection is `pcx-08077dcc008fbbab6`.

### Update route tables

To enable private IPv4 traffic between the instances in the VPC peered network, you need to establish a communication channel by adding a route to the route tables associated with all the subnets associated with the instances for both VPCs.
The destination of the new route in a given route table is the CIDR of the VPC of the other cluster and the target is the ID of the VPC peering connection.

The following is an example of the route tables that enable communication of instances in two peered VPCs. Each table has a local route and the added route which sends traffic targeted at the other VPC to the peered network connection. The other preconfigured routes are omitted for readability.

<table>
<thead>
  <tr>
    <th>Route table</th>
    <th>Destination</th>
    <th>Target</th>
  </tr>
</thead>
<tbody>
  <tr>
    <td rowspan="2">eksctl-scylladb-us-east-1-cluster/PublicRouteTable</td>
    <td>10.0.0.0/16</td>
    <td>local</td>
  </tr>
  <tr>
    <td>172.16.0.0/16</td>
    <td>pcx-08077dcc008fbbab6</td>
  </tr>
  <tr>
    <td rowspan="2">eksctl-scylladb-us-east-2-cluster/PublicRouteTable</td>
    <td>172.16.0.0/16</td>
    <td>local</td>
  </tr>
  <tr>
    <td>10.0.0.0/16</td>
    <td>pcx-08077dcc008fbbab6</td>
  </tr>
</tbody>
</table>


Refer to [Update your route tables for a VPC peering connection](https://docs.aws.amazon.com/vpc/latest/peering/vpc-peering-routing.html) in AWS documentation for more information.

### Update security groups

To allow traffic to flow to and from instances associated with security groups in the peered VPC, you need to update the inbound rules of the VPCs' shared security groups.

Below is an example of the inbound rules that to be added to the corresponding security groups of the two VPCs.

| Security group name                                                            | Type        | Protocol | Port range | Source               |
|--------------------------------------------------------------------------------|-------------|----------|------------|----------------------|
| eksctl-scylladb-us-east-1-cluster-ClusterSharedNodeSecurityGroup-TD05V9EVU3B8  | All traffic | All      | All        | Custom 172.16.0.0/16 |
| eksctl-scylladb-us-east-2-cluster-ClusterSharedNodeSecurityGroup-1FR9YDLU0VE7M | All traffic | All      | All        | Custom 10.0.0.0/16   |

The names of the shared security groups of your VPCs should be similar to the ones presented in the example.

---

Having followed the above steps, you should now have a platform prepared for deploying a multi-datacenter ScyllaDB cluster.
Refer to [Deploy a multi-datacenter ScyllaDB cluster in multiple interconnected Kubernetes clusters](multidc.md) in ScyllaDB Operator documentation for guidance.
