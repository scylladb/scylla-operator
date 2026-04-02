# Set up multi-DC infrastructure

This page explains how to set up the networking infrastructure required to run a ScyllaDB cluster across multiple Kubernetes clusters using `ScyllaCluster` resources with `externalSeeds`. Before following this guide, complete the [GitOps](install-with-gitops.md) or [Helm](install-with-helm.md) installation in **every** Kubernetes cluster that will host a ScyllaDB datacenter.

## Architecture

A multi-DC ScyllaDB deployment on Kubernetes consists of two or more interconnected Kubernetes clusters. Each cluster hosts one `ScyllaCluster` resource representing one ScyllaDB datacenter. The datacenters discover each other through external seeds — Pod IPs or domain names of nodes in other datacenters.

## Networking requirements

All ScyllaDB pods across all participating clusters must be able to communicate directly using **Pod IPs**. Each `ScyllaCluster` should be configured with:

- `nodeService.type: Headless` — no virtual IPs, Services resolve directly to Pod IPs.
- `broadcastOptions.nodes.type: PodIP` — inter-node traffic uses Pod IPs.
- `broadcastOptions.clients.type: PodIP` — client traffic uses Pod IPs.

This means the VPC/network configuration must ensure that Pod CIDRs are routable between all Kubernetes clusters. The specific mechanism depends on your platform. For alternative exposure configurations, see [Expose ScyllaDB clusters](../deploy-scylladb/set-up-networking/expose-clusters.md).

:::{caution}
Pod CIDRs across clusters **must not overlap**. Plan your subnet and Pod CIDR allocations before creating the clusters.
:::

## Platform-specific setup

### GKE: Shared VPC

On GKE, the simplest approach is to create all clusters in a **single shared VPC** with non-overlapping subnets.

#### 1. Create a VPC network

```shell
gcloud compute networks create scylladb --subnet-mode=custom
```

#### 2. Create subnets with non-overlapping CIDRs

```shell
# Subnet for the first cluster (e.g., us-east1)
gcloud compute networks subnets create scylladb-us-east1 \
  --network=scylladb \
  --region=us-east1 \
  --range=10.0.0.0/20 \
  --secondary-range=pods=10.1.0.0/16,services=10.2.0.0/20

# Subnet for the second cluster (e.g., us-west1)
gcloud compute networks subnets create scylladb-us-west1 \
  --network=scylladb \
  --region=us-west1 \
  --range=172.16.0.0/20 \
  --secondary-range=pods=172.17.0.0/16,services=172.18.0.0/20
```

#### 3. Create GKE clusters in the shared VPC

```shell
gcloud container clusters create scylladb-us-east1 \
  --location=us-east1-b \
  --network=scylladb \
  --subnetwork=scylladb-us-east1 \
  --cluster-secondary-range-name=pods \
  --services-secondary-range-name=services \
  --enable-ip-alias

gcloud container clusters create scylladb-us-west1 \
  --location=us-west1-b \
  --network=scylladb \
  --subnetwork=scylladb-us-west1 \
  --cluster-secondary-range-name=pods \
  --services-secondary-range-name=services \
  --enable-ip-alias
```

#### 4. Update firewall rules for cross-cluster Pod communication

GKE creates firewall rules per cluster that only allow traffic from that cluster's own Pod CIDR. You need to update them to include all clusters' Pod CIDRs:

```shell
# Find the firewall rule for each cluster
gcloud compute firewall-rules list --filter='name~gke-scylladb'

# Update each rule to include all Pod CIDRs
gcloud compute firewall-rules update gke-scylladb-us-east1-<hash>-all \
  --source-ranges=10.1.0.0/16,172.17.0.0/16

gcloud compute firewall-rules update gke-scylladb-us-west1-<hash>-all \
  --source-ranges=10.1.0.0/16,172.17.0.0/16
```

Replace `<hash>` with the actual hash from your firewall rule names.

### EKS: VPC peering

On EKS, clusters typically run in separate VPCs (often in different regions). Use **VPC peering** to route traffic between them.

#### 1. Create EKS clusters with non-overlapping VPC CIDRs

```yaml
# cluster-us-east-1.yaml
apiVersion: eksctl.io/v1alpha5
kind: ClusterConfig
metadata:
  name: scylladb-us-east-1
  region: us-east-1
vpc:
  cidr: 10.0.0.0/16
```

```yaml
# cluster-us-east-2.yaml
apiVersion: eksctl.io/v1alpha5
kind: ClusterConfig
metadata:
  name: scylladb-us-east-2
  region: us-east-2
vpc:
  cidr: 172.16.0.0/16
```

```shell
eksctl create cluster -f cluster-us-east-1.yaml
eksctl create cluster -f cluster-us-east-2.yaml
```

#### 2. Create a VPC peering connection

```shell
# Get VPC IDs
VPC_ID_1=$(aws ec2 describe-vpcs --region us-east-1 \
  --filters Name=tag:eksctl.cluster.k8s.io/v1alpha1/cluster-name,Values=scylladb-us-east-1 \
  --query 'Vpcs[0].VpcId' --output text)

VPC_ID_2=$(aws ec2 describe-vpcs --region us-east-2 \
  --filters Name=tag:eksctl.cluster.k8s.io/v1alpha1/cluster-name,Values=scylladb-us-east-2 \
  --query 'Vpcs[0].VpcId' --output text)

# Create peering connection
PEERING_ID=$(aws ec2 create-vpc-peering-connection \
  --vpc-id "${VPC_ID_1}" \
  --peer-vpc-id "${VPC_ID_2}" \
  --peer-region us-east-2 \
  --query 'VpcPeeringConnection.VpcPeeringConnectionId' --output text)

# Accept the peering connection in the peer region
aws ec2 accept-vpc-peering-connection --region us-east-2 \
  --vpc-peering-connection-id "${PEERING_ID}"
```

#### 3. Update route tables

Add routes in each VPC's route table so traffic destined for the other VPC is sent through the peering connection:

```shell
# Get route table IDs
RT_1=$(aws ec2 describe-route-tables --region us-east-1 \
  --filters Name=vpc-id,Values="${VPC_ID_1}" \
  --query 'RouteTables[0].RouteTableId' --output text)

RT_2=$(aws ec2 describe-route-tables --region us-east-2 \
  --filters Name=vpc-id,Values="${VPC_ID_2}" \
  --query 'RouteTables[0].RouteTableId' --output text)

# Add routes
aws ec2 create-route --region us-east-1 \
  --route-table-id "${RT_1}" \
  --destination-cidr-block 172.16.0.0/16 \
  --vpc-peering-connection-id "${PEERING_ID}"

aws ec2 create-route --region us-east-2 \
  --route-table-id "${RT_2}" \
  --destination-cidr-block 10.0.0.0/16 \
  --vpc-peering-connection-id "${PEERING_ID}"
```

#### 4. Update security groups

Allow traffic from the peered VPC's CIDR in each cluster's node security group:

```shell
# Get the shared node security group for each cluster
SG_1=$(aws ec2 describe-security-groups --region us-east-1 \
  --filters Name=tag:aws:eks:cluster-name,Values=scylladb-us-east-1 \
  --query 'SecurityGroups[?contains(GroupName, `ClusterSharedNodeSecurityGroup`)].GroupId' --output text)

SG_2=$(aws ec2 describe-security-groups --region us-east-2 \
  --filters Name=tag:aws:eks:cluster-name,Values=scylladb-us-east-2 \
  --query 'SecurityGroups[?contains(GroupName, `ClusterSharedNodeSecurityGroup`)].GroupId' --output text)

# Allow all traffic from the peered VPC
aws ec2 authorize-security-group-ingress --region us-east-1 \
  --group-id "${SG_1}" \
  --protocol all --cidr 172.16.0.0/16

aws ec2 authorize-security-group-ingress --region us-east-2 \
  --group-id "${SG_2}" \
  --protocol all --cidr 10.0.0.0/16
```

## Verifying connectivity

After setting up the infrastructure, verify end-to-end Pod connectivity between clusters before deploying ScyllaDB.

### Step 1: Deploy a test pod in each cluster

In cluster 1:
```bash
kubectl --context=<cluster1-context> run net-test --image=busybox --restart=Never -- sleep 3600
```

In cluster 2:
```bash
kubectl --context=<cluster2-context> run net-test --image=busybox --restart=Never -- sleep 3600
```

Wait for the pods to be running before continuing (up to 60 seconds):
```bash
kubectl --context=<cluster1-context> wait pod net-test --for=condition=Ready --timeout=60s
kubectl --context=<cluster2-context> wait pod net-test --for=condition=Ready --timeout=60s
```

Expected output:
```
pod/net-test condition met
pod/net-test condition met
```

### Step 2: Get Pod IPs

```bash
CLUSTER1_POD_IP=$(kubectl --context=<cluster1-context> get pod net-test -o jsonpath='{.status.podIP}')
CLUSTER2_POD_IP=$(kubectl --context=<cluster2-context> get pod net-test -o jsonpath='{.status.podIP}')
echo "Cluster 1 pod IP: $CLUSTER1_POD_IP"
echo "Cluster 2 pod IP: $CLUSTER2_POD_IP"
```

Expected output:
```
Cluster 1 pod IP: 10.1.x.x
Cluster 2 pod IP: 172.17.x.x
```

Each IP should belong to the Pod CIDR of its respective cluster and must not overlap.

### Step 3: Test cross-cluster connectivity

From cluster 1 to cluster 2:
```bash
kubectl --context=<cluster1-context> exec net-test -- wget -qO- http://$CLUSTER2_POD_IP
```

From cluster 2 to cluster 1:
```bash
kubectl --context=<cluster2-context> exec net-test -- wget -qO- http://$CLUSTER1_POD_IP
```

A connection timeout indicates the routing or peering is not configured correctly. A refused connection (not a timeout) means routing works but no server is running — which is expected and confirms connectivity.

### Step 4: Verify ScyllaDB ports

Confirm that ScyllaDB inter-node (7000, 7001) and CQL (9042) ports are reachable across clusters:

```bash
kubectl --context=<cluster1-context> exec net-test -- nc -zv $CLUSTER2_POD_IP 7000
kubectl --context=<cluster1-context> exec net-test -- nc -zv $CLUSTER2_POD_IP 7001
kubectl --context=<cluster1-context> exec net-test -- nc -zv $CLUSTER2_POD_IP 9042
```

These commands will only show successful results once a ScyllaDB pod is listening on the target. Before deploying ScyllaDB, a refused connection (not a timeout) is the expected outcome and confirms that routing is working.

### Step 5: Clean up test pods

```bash
kubectl --context=<cluster1-context> delete pod net-test
kubectl --context=<cluster2-context> delete pod net-test
```

If connectivity fails, check:
- VPC peering connection status (EKS) or firewall rules (GKE).
- Route tables include the peer VPC CIDR.
- Security groups or firewall rules allow traffic from the peer CIDR.
- Pod CIDRs do not overlap between clusters.

## Related pages

- [Prerequisites](prerequisites.md) — Kubernetes version requirements per platform.
- [Install with GitOps](install-with-gitops.md) — installing ScyllaDB Operator in each cluster.
- [Deploy a multi-DC cluster](../deploy-scylladb/deploy-multi-dc-cluster.md) — creating a multi-datacenter cluster using multiple `ScyllaCluster` resources.
- [Architecture overview](../understand/overview.md) — multi-DC component model.
- [Networking](../understand/networking.md) — broadcast options and Service types.
