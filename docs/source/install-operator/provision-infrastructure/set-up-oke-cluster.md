# Set up an OKE cluster for ScyllaDB

This guide provisions an Oracle Container Engine for Kubernetes (OKE) cluster suitable for running ScyllaDB.
At the end, you will have:

- A Virtual Cloud Network (VCN) with public and private subnets.
- An OKE Enhanced Cluster with VCN-Native Pod Networking.
- A general-purpose node pool for system workloads.
- A dedicated Dense I/O node pool for ScyllaDB with the static CPU manager policy enabled.

:::{tip}
All the steps in this guide are also available as a single executable script:
{{ '[`examples/oke/setup-oke-cluster.sh`](https://github.com/{}/blob/{}/examples/oke/setup-oke-cluster.sh)'.format(repository, revision) }}.
Set `OCI_REGION` and `OCI_COMPARTMENT_OCID`, then run the script to provision everything in one go.
:::

## Prerequisites

- An Oracle Cloud Infrastructure account with [permissions to create networking, Container Engine, and Compute resources](https://docs.oracle.com/en-us/iaas/Content/ContEng/Concepts/contengpolicyconfig.htm) in a target compartment.
- The [`oci` CLI](https://docs.oracle.com/en-us/iaas/Content/API/SDKDocs/cliinstall.htm) installed and configured (`oci setup config`).
- [`kubectl`](https://kubernetes.io/docs/tasks/tools/#kubectl) installed.
- A Dense I/O instance shape (e.g., `VM.DenseIO2.8`) available in your target region with capacity for 3 worker nodes.
  See [ScyllaDB cloud instance recommendations for OCI](https://docs.scylladb.com/manual/stable/getting-started/cloud-instance-recommendations.html#oracle-cloud-infrastructure-oci) for recommended shapes and [system requirements](https://docs.scylladb.com/manual/stable/getting-started/system-requirements.html) for minimum specifications.

## Set environment variables

The rest of the guide refers to the variables defined here.

Set your OCI region and compartment — these have no defaults and must be provided:

:::{code-block} console
# You can list compartments with: oci iam compartment list --all
export OCI_REGION="<your-region>"               # e.g. us-sanjose-1
export OCI_COMPARTMENT_OCID="<your-compartment>" # e.g. ocid1.compartment.oc1..aaaa...
:::

The remaining variables have sensible defaults and can be copied as-is.
Override any value before running if needed:

:::{literalinclude} ../../../../examples/oke/setup-oke-cluster.sh
:language: bash
:start-after: "# [START env_defaults]"
:end-before: "# [END env_defaults]"
:::

Discover and pin the latest Kubernetes version supported by OKE in your region:

:::{literalinclude} ../../../../examples/oke/setup-oke-cluster.sh
:language: bash
:start-after: "# [START k8s_version]"
:end-before: "# [END k8s_version]"
:::

The first availability domain in the region is used in the commands below.
Capture it:

:::{literalinclude} ../../../../examples/oke/setup-oke-cluster.sh
:language: bash
:start-after: "# [START availability_domain]"
:end-before: "# [END availability_domain]"
:::

:::{note}
In single-AD regions (such as `us-sanjose-1`), high availability for ScyllaDB is achieved by spreading nodes across the three fault domains within the AD (`FAULT-DOMAIN-1`, `FAULT-DOMAIN-2`, `FAULT-DOMAIN-3`).
The `ScyllaCluster` manifest in the [reference deployment](../../deploy-scylladb/reference-deployments/reference-deployment-oke.md) uses one rack per fault domain.
:::

## Create the network

OKE requires a VCN with appropriate subnets, gateways, route tables, and security rules.
The topology created in this section is the minimum needed for an OKE cluster with VCN-Native Pod Networking and a public Kubernetes API endpoint.
This guide uses a public API endpoint for simplicity; for production environments, consider using a [private endpoint](https://docs.oracle.com/en-us/iaas/Content/ContEng/Concepts/contengnetworkconfig.htm) instead.
For full background on the supported topologies, see the [Container Engine for Kubernetes Networking](https://docs.oracle.com/en-us/iaas/Content/ContEng/Concepts/contengnetworkconfig.htm) reference.

The resources created are:

- a VCN (`10.0.0.0/16`),
- an Internet Gateway and a NAT Gateway,
- a public route table (default route → IGW) and a private route table (default route → NAT GW),
- a public security list (ingress TCP `6443` and `443` from anywhere, plus all intra-VCN traffic) and a private security list (ingress all from inside the VCN),
- three regional subnets — control plane, workers, and Kubernetes Service load balancers.

:::{figure} ../../deploy-scylladb/oke-network.drawio.svg
:class: sd-m-auto
:name: oke-network-architecture
:alt: OKE reference network architecture

OKE reference network architecture.
:::

### Create the VCN

:::{literalinclude} ../../../../examples/oke/setup-oke-cluster.sh
:language: bash
:start-after: "# [START create_vcn]"
:end-before: "# [END create_vcn]"
:::

:::{seealso}
[Creating a VCN](https://docs.oracle.com/en-us/iaas/Content/Network/Tasks/create_vcn.htm) in the OCI documentation.
:::

### Create the gateways

An Internet Gateway for the public subnets and a NAT Gateway so private worker nodes can reach the public Internet for image pulls and OS updates:

:::{literalinclude} ../../../../examples/oke/setup-oke-cluster.sh
:language: bash
:start-after: "# [START create_gateways]"
:end-before: "# [END create_gateways]"
:::

:::{seealso}
[Internet Gateway](https://docs.oracle.com/en-us/iaas/Content/Network/Tasks/managingIGs.htm) and [NAT Gateway](https://docs.oracle.com/en-us/iaas/Content/Network/Tasks/NATgateway.htm) in the OCI documentation.
:::

### Create the route tables

A public route table sending `0.0.0.0/0` to the Internet Gateway, and a private route table sending `0.0.0.0/0` to the NAT Gateway:

:::{literalinclude} ../../../../examples/oke/setup-oke-cluster.sh
:language: bash
:start-after: "# [START create_route_tables]"
:end-before: "# [END create_route_tables]"
:::

:::{seealso}
[VCN Route Tables](https://docs.oracle.com/en-us/iaas/Content/Network/Tasks/managingroutetables.htm) in the OCI documentation.
:::

### Create the security lists

Two security lists are created here: a public one (Kubernetes API and Service load balancers from the Internet, plus all intra-VCN traffic) and a private one (intra-VCN only).
For deployments with a tighter security posture, prefer the more granular Network Security Groups (NSGs) documented in the [OKE network resource configuration guide](https://docs.oracle.com/en-us/iaas/Content/ContEng/Concepts/contengnetworkconfig.htm).

:::{literalinclude} ../../../../examples/oke/setup-oke-cluster.sh
:language: bash
:start-after: "# [START create_security_lists]"
:end-before: "# [END create_security_lists]"
:::

:::{seealso}
[Security Lists](https://docs.oracle.com/en-us/iaas/Content/Network/Concepts/securitylists.htm) in the OCI documentation.
:::

### Create the subnets

Three regional subnets, all `/24`:

- `subnet-cp` — public, hosts the Kubernetes API endpoint.
- `subnet-workers` — private, hosts worker nodes and Pods.
- `subnet-lb` — public, hosts load balancers fronting Kubernetes Services of type `LoadBalancer`.

:::{literalinclude} ../../../../examples/oke/setup-oke-cluster.sh
:language: bash
:start-after: "# [START create_subnets]"
:end-before: "# [END create_subnets]"
:::

:::{seealso}
[Creating a Subnet](https://docs.oracle.com/en-us/iaas/Content/Network/Tasks/create_subnet.htm) in the OCI documentation.
:::

## Create the OKE cluster

:::{literalinclude} ../../../../examples/oke/setup-oke-cluster.sh
:language: bash
:start-after: "# [START create_cluster]"
:end-before: "# [END create_cluster]"
:::

:::{seealso}
[Creating a Cluster](https://docs.oracle.com/en-us/iaas/Content/ContEng/Tasks/create-cluster.htm) in the OCI documentation.
:::

### Discover the worker node image

OKE publishes pre-baked Oracle Linux images for each supported Kubernetes version.
Pick the latest one matching `K8S_VERSION`:

:::{literalinclude} ../../../../examples/oke/setup-oke-cluster.sh
:language: bash
:start-after: "# [START discover_node_image]"
:end-before: "# [END discover_node_image]"
:::

### Create the general-purpose node pool

This pool runs system components (cert-manager, ScyllaDB Operator, etc.):

:::{literalinclude} ../../../../examples/oke/setup-oke-cluster.sh
:language: bash
:start-after: "# [START create_system_pool]"
:end-before: "# [END create_system_pool]"
:::

### Create the dedicated ScyllaDB node pool

This pool is dedicated to ScyllaDB workloads.
The Cloud-Init script enables the [static CPU manager policy](https://kubernetes.io/docs/tasks/administer-cluster/cpu-management-policies/#static-policy-configuration), which is required for CPU pinning.

:::{literalinclude} ../../../../examples/oke/setup-oke-cluster.sh
:language: bash
:start-after: "# [START create_scylla_pool]"
:end-before: "# [END create_scylla_pool]"
:::

:::{note}
OKE's node-pool create API does not accept Kubernetes node taints directly.
The taint is applied via `kubectl` after the kubeconfig is fetched.
:::

:::{seealso}
[Creating a Node Pool](https://docs.oracle.com/en-us/iaas/Content/ContEng/Tasks/create-node-pool.htm) in the OCI documentation.
:::

## Get cluster credentials

:::{literalinclude} ../../../../examples/oke/setup-oke-cluster.sh
:language: bash
:start-after: "# [START get_credentials]"
:end-before: "# [END get_credentials]"
:::

Verify connectivity and node readiness:

:::{code-block} console
kubectl get nodes -L scylla.scylladb.com/node-type -L oci.oraclecloud.com/fault-domain
:::

Example expected output:

:::{code-block} console
NAME          STATUS   ROLES   AGE   VERSION   NODE-TYPE   FAULT-DOMAIN
10.0.1.10    Ready    node    10m   v1.32.1                FAULT-DOMAIN-1
10.0.1.20    Ready    node    5m    v1.32.1   scylla      FAULT-DOMAIN-1
10.0.1.21    Ready    node    5m    v1.32.1   scylla      FAULT-DOMAIN-2
10.0.1.22    Ready    node    5m    v1.32.1   scylla      FAULT-DOMAIN-3
:::

:::{seealso}
[Downloading a kubeconfig File](https://docs.oracle.com/en-us/iaas/Content/ContEng/Tasks/contengdownloadkubeconfigfile.htm) in the OCI documentation.
:::

## Taint the dedicated ScyllaDB nodes

Use the label applied at node-pool creation to select the dedicated nodes and apply the matching taint:

:::{literalinclude} ../../../../examples/oke/setup-oke-cluster.sh
:language: bash
:start-after: "# [START taint_nodes]"
:end-before: "# [END taint_nodes]"
:::

## Clean up

To tear down the infrastructure created by this guide, delete the OKE cluster first (this also deletes its node pools):

:::{code-block} console
oci ce cluster delete \
  --region "${OCI_REGION}" \
  --cluster-id "${OKE_CLUSTER_OCID}" \
  --force \
  --wait-for-state SUCCEEDED \
  --wait-for-state FAILED
:::

Then delete the network resources in reverse order of creation:

:::{code-block} console
oci network subnet delete         --region "${OCI_REGION}" --subnet-id "${OKE_LB_SUBNET_OCID}"      --force --wait-for-state TERMINATED
oci network subnet delete         --region "${OCI_REGION}" --subnet-id "${OKE_WORKERS_SUBNET_OCID}" --force --wait-for-state TERMINATED
oci network subnet delete         --region "${OCI_REGION}" --subnet-id "${OKE_CP_SUBNET_OCID}"      --force --wait-for-state TERMINATED

oci network security-list delete  --region "${OCI_REGION}" --security-list-id "${OKE_PUBLIC_SL_OCID}"  --force --wait-for-state TERMINATED
oci network security-list delete  --region "${OCI_REGION}" --security-list-id "${OKE_PRIVATE_SL_OCID}" --force --wait-for-state TERMINATED

oci network route-table delete    --region "${OCI_REGION}" --rt-id "${OKE_PRIVATE_RT_OCID}" --force --wait-for-state TERMINATED
oci network route-table delete    --region "${OCI_REGION}" --rt-id "${OKE_PUBLIC_RT_OCID}"  --force --wait-for-state TERMINATED

oci network nat-gateway delete      --region "${OCI_REGION}" --nat-gateway-id "${OKE_NATGW_OCID}"      --force --wait-for-state TERMINATED
oci network internet-gateway delete --region "${OCI_REGION}" --ig-id "${OKE_IGW_OCID}"                 --force --wait-for-state TERMINATED

oci network vcn delete              --region "${OCI_REGION}" --vcn-id "${OKE_VCN_OCID}"                --force --wait-for-state TERMINATED
:::

## Next steps

- Follow the [Reference deployment: OKE](../../deploy-scylladb/reference-deployments/reference-deployment-oke.md) for a complete ScyllaDB deployment on this cluster.
