#!/usr/bin/env bash
#
# Set up an OKE cluster suitable for running ScyllaDB.
#
# This script creates:
# - A VCN with public and private subnets
# - An OKE Enhanced Cluster with VCN-Native Pod Networking
# - A general-purpose node pool for system workloads
# - A dedicated Dense I/O node pool for ScyllaDB
#
# Prerequisites:
# - oci CLI installed and configured (oci setup config)
# - kubectl installed
#
# Usage:
#   export OCI_REGION='us-sanjose-1'
#   export OCI_COMPARTMENT_OCID='ocid1.compartment.oc1..aaaaaaaa...'
#   ./setup-oke-cluster.sh

set -euo pipefail

# Target region and compartment.
# Replace the compartment OCID below with one of your own; you can list
# compartments with: oci iam compartment list --all
: "${OCI_REGION:?'Set OCI_REGION (e.g. us-sanjose-1)'}"
: "${OCI_COMPARTMENT_OCID:?'Set OCI_COMPARTMENT_OCID'}"

# [START env_defaults]
# Names used for the resources created below.
export OKE_CLUSTER_NAME="${OKE_CLUSTER_NAME:-scylladb-demo}"
export OKE_VCN_NAME="${OKE_CLUSTER_NAME}-vcn"

# Compute shapes.
# General-purpose pool runs system workloads (operator, cert-manager, etc.).
export GENERAL_NODE_SHAPE="${GENERAL_NODE_SHAPE:-VM.Standard.E4.Flex}"
export GENERAL_NODE_OCPUS="${GENERAL_NODE_OCPUS:-2}"
export GENERAL_NODE_MEMORY_GBS="${GENERAL_NODE_MEMORY_GBS:-16}"
# Dedicated ScyllaDB pool. Dense I/O shapes provide local NVMe SSDs.
# See https://docs.oracle.com/en-us/iaas/Content/Compute/References/computeshapes.htm#vm-dense
export SCYLLA_NODE_SHAPE="${SCYLLA_NODE_SHAPE:-VM.DenseIO2.8}"
# [END env_defaults]

# [START k8s_version]
export K8S_VERSION="$(
  oci ce cluster-options get \
    --cluster-option-id all \
    --region "${OCI_REGION}" \
    --query 'data."kubernetes-versions" | sort(@) | [-1]' --raw-output
)"
echo "K8S_VERSION=${K8S_VERSION}"
# [END k8s_version]

# [START availability_domain]
export OCI_AD="$(
  oci iam availability-domain list \
    --region "${OCI_REGION}" \
    --compartment-id "${OCI_COMPARTMENT_OCID}" \
    --query 'data[0].name' --raw-output
)"
echo "OCI_AD=${OCI_AD}"
# [END availability_domain]

# [START create_vcn]
oci network vcn create \
  --region "${OCI_REGION}" \
  --compartment-id "${OCI_COMPARTMENT_OCID}" \
  --display-name "${OKE_VCN_NAME}" \
  --cidr-blocks '["10.0.0.0/16"]' \
  --dns-label 'okevcn'

export OKE_VCN_OCID="$(
  oci network vcn list \
    --region "${OCI_REGION}" \
    --compartment-id "${OCI_COMPARTMENT_OCID}" \
    --display-name "${OKE_VCN_NAME}" \
    --query 'data[0].id' --raw-output
)"
# [END create_vcn]

# [START create_gateways]
oci network internet-gateway create \
  --region "${OCI_REGION}" \
  --compartment-id "${OCI_COMPARTMENT_OCID}" \
  --vcn-id "${OKE_VCN_OCID}" \
  --is-enabled true \
  --display-name "${OKE_CLUSTER_NAME}-igw"

oci network nat-gateway create \
  --region "${OCI_REGION}" \
  --compartment-id "${OCI_COMPARTMENT_OCID}" \
  --vcn-id "${OKE_VCN_OCID}" \
  --display-name "${OKE_CLUSTER_NAME}-natgw"

export OKE_IGW_OCID="$(oci network internet-gateway list --region "${OCI_REGION}" --compartment-id "${OCI_COMPARTMENT_OCID}" --vcn-id "${OKE_VCN_OCID}" --query 'data[0].id' --raw-output)"
export OKE_NATGW_OCID="$(oci network nat-gateway list --region "${OCI_REGION}" --compartment-id "${OCI_COMPARTMENT_OCID}" --vcn-id "${OKE_VCN_OCID}" --query 'data[0].id' --raw-output)"
# [END create_gateways]

# [START create_route_tables]
oci network route-table create \
  --region "${OCI_REGION}" \
  --compartment-id "${OCI_COMPARTMENT_OCID}" \
  --vcn-id "${OKE_VCN_OCID}" \
  --display-name "${OKE_CLUSTER_NAME}-rt-public" \
  --route-rules "[{\"destination\": \"0.0.0.0/0\", \"destinationType\": \"CIDR_BLOCK\", \"networkEntityId\": \"${OKE_IGW_OCID}\"}]"

oci network route-table create \
  --region "${OCI_REGION}" \
  --compartment-id "${OCI_COMPARTMENT_OCID}" \
  --vcn-id "${OKE_VCN_OCID}" \
  --display-name "${OKE_CLUSTER_NAME}-rt-private" \
  --route-rules "[{\"destination\": \"0.0.0.0/0\", \"destinationType\": \"CIDR_BLOCK\", \"networkEntityId\": \"${OKE_NATGW_OCID}\"}]"

export OKE_PUBLIC_RT_OCID="$(oci network route-table list --region "${OCI_REGION}" --compartment-id "${OCI_COMPARTMENT_OCID}" --vcn-id "${OKE_VCN_OCID}" --display-name "${OKE_CLUSTER_NAME}-rt-public"  --query 'data[0].id' --raw-output)"
export OKE_PRIVATE_RT_OCID="$(oci network route-table list --region "${OCI_REGION}" --compartment-id "${OCI_COMPARTMENT_OCID}" --vcn-id "${OKE_VCN_OCID}" --display-name "${OKE_CLUSTER_NAME}-rt-private" --query 'data[0].id' --raw-output)"
# [END create_route_tables]

# [START create_security_lists]
# Public security list: allows HTTPS to the Kubernetes API endpoint (6443)
# and to Service-type=LoadBalancer load balancers (443) from anywhere, plus
# unrestricted intra-VCN traffic.
oci network security-list create \
  --region "${OCI_REGION}" \
  --compartment-id "${OCI_COMPARTMENT_OCID}" \
  --vcn-id "${OKE_VCN_OCID}" \
  --display-name "${OKE_CLUSTER_NAME}-sl-public" \
  --egress-security-rules '[{"destination": "0.0.0.0/0", "destinationType": "CIDR_BLOCK", "protocol": "all", "isStateless": false}]' \
  --ingress-security-rules '[
    {"source": "0.0.0.0/0",  "sourceType": "CIDR_BLOCK", "protocol": "6",   "isStateless": false, "tcpOptions": {"destinationPortRange": {"min": 6443, "max": 6443}}},
    {"source": "0.0.0.0/0",  "sourceType": "CIDR_BLOCK", "protocol": "6",   "isStateless": false, "tcpOptions": {"destinationPortRange": {"min": 443,  "max": 443}}},
    {"source": "10.0.0.0/16","sourceType": "CIDR_BLOCK", "protocol": "all", "isStateless": false}
  ]'

# Private security list: allows all intra-VCN traffic; egress everywhere.
oci network security-list create \
  --region "${OCI_REGION}" \
  --compartment-id "${OCI_COMPARTMENT_OCID}" \
  --vcn-id "${OKE_VCN_OCID}" \
  --display-name "${OKE_CLUSTER_NAME}-sl-private" \
  --egress-security-rules '[{"destination": "0.0.0.0/0", "destinationType": "CIDR_BLOCK", "protocol": "all", "isStateless": false}]' \
  --ingress-security-rules '[
    {"source": "10.0.0.0/16", "sourceType": "CIDR_BLOCK", "protocol": "all", "isStateless": false}
  ]'

export OKE_PUBLIC_SL_OCID="$(oci network security-list list --region "${OCI_REGION}" --compartment-id "${OCI_COMPARTMENT_OCID}" --vcn-id "${OKE_VCN_OCID}" --display-name "${OKE_CLUSTER_NAME}-sl-public"  --query 'data[0].id' --raw-output)"
export OKE_PRIVATE_SL_OCID="$(oci network security-list list --region "${OCI_REGION}" --compartment-id "${OCI_COMPARTMENT_OCID}" --vcn-id "${OKE_VCN_OCID}" --display-name "${OKE_CLUSTER_NAME}-sl-private" --query 'data[0].id' --raw-output)"
# [END create_security_lists]

# [START create_subnets]
# Control-plane subnet (public).
oci network subnet create \
  --region "${OCI_REGION}" \
  --compartment-id "${OCI_COMPARTMENT_OCID}" \
  --vcn-id "${OKE_VCN_OCID}" \
  --display-name "${OKE_CLUSTER_NAME}-subnet-cp" \
  --cidr-block '10.0.0.0/24' \
  --dns-label 'cp' \
  --route-table-id "${OKE_PUBLIC_RT_OCID}" \
  --security-list-ids "[\"${OKE_PUBLIC_SL_OCID}\"]" \
  --prohibit-public-ip-on-vnic false

# Worker (and pod) subnet (private).
oci network subnet create \
  --region "${OCI_REGION}" \
  --compartment-id "${OCI_COMPARTMENT_OCID}" \
  --vcn-id "${OKE_VCN_OCID}" \
  --display-name "${OKE_CLUSTER_NAME}-subnet-workers" \
  --cidr-block '10.0.1.0/24' \
  --dns-label 'workers' \
  --route-table-id "${OKE_PRIVATE_RT_OCID}" \
  --security-list-ids "[\"${OKE_PRIVATE_SL_OCID}\"]" \
  --prohibit-public-ip-on-vnic true

# Service-LB subnet (public).
oci network subnet create \
  --region "${OCI_REGION}" \
  --compartment-id "${OCI_COMPARTMENT_OCID}" \
  --vcn-id "${OKE_VCN_OCID}" \
  --display-name "${OKE_CLUSTER_NAME}-subnet-lb" \
  --cidr-block '10.0.2.0/24' \
  --dns-label 'lb' \
  --route-table-id "${OKE_PUBLIC_RT_OCID}" \
  --security-list-ids "[\"${OKE_PUBLIC_SL_OCID}\"]" \
  --prohibit-public-ip-on-vnic false

export OKE_CP_SUBNET_OCID="$(     oci network subnet list --region "${OCI_REGION}" --compartment-id "${OCI_COMPARTMENT_OCID}" --vcn-id "${OKE_VCN_OCID}" --display-name "${OKE_CLUSTER_NAME}-subnet-cp"      --query 'data[0].id' --raw-output)"
export OKE_WORKERS_SUBNET_OCID="$(oci network subnet list --region "${OCI_REGION}" --compartment-id "${OCI_COMPARTMENT_OCID}" --vcn-id "${OKE_VCN_OCID}" --display-name "${OKE_CLUSTER_NAME}-subnet-workers" --query 'data[0].id' --raw-output)"
export OKE_LB_SUBNET_OCID="$(     oci network subnet list --region "${OCI_REGION}" --compartment-id "${OCI_COMPARTMENT_OCID}" --vcn-id "${OKE_VCN_OCID}" --display-name "${OKE_CLUSTER_NAME}-subnet-lb"      --query 'data[0].id' --raw-output)"
# [END create_subnets]

# [START create_cluster]
oci ce cluster create \
  --region "${OCI_REGION}" \
  --compartment-id "${OCI_COMPARTMENT_OCID}" \
  --vcn-id "${OKE_VCN_OCID}" \
  --name "${OKE_CLUSTER_NAME}" \
  --kubernetes-version "${K8S_VERSION}" \
  --type ENHANCED_CLUSTER \
  --endpoint-subnet-id "${OKE_CP_SUBNET_OCID}" \
  --endpoint-public-ip-enabled true \
  --service-lb-subnet-ids "[\"${OKE_LB_SUBNET_OCID}\"]" \
  --cluster-pod-network-options '[{"cniType": "OCI_VCN_IP_NATIVE"}]' \
  --max-wait-seconds 1800 --wait-interval-seconds 30 \
  --wait-for-state SUCCEEDED \
  --wait-for-state FAILED

export OKE_CLUSTER_OCID="$(
  oci ce cluster list \
    --region "${OCI_REGION}" \
    --compartment-id "${OCI_COMPARTMENT_OCID}" \
    --name "${OKE_CLUSTER_NAME}" \
    --lifecycle-state ACTIVE \
    --query 'data[0].id' --raw-output
)"
# [END create_cluster]

# [START discover_node_image]
K8S_VERSION_BARE="${K8S_VERSION#v}"
# shellcheck disable=SC2016
OKE_IMAGE_QUERY=$(printf 'max_by(data.sources[?contains("source-name",`Oracle-Linux-8.`) && contains("source-name",`-OKE-%s-`) && !contains("source-name",`aarch64`) && !contains("source-name",`GPU`)], &"source-name")."image-id"' "${K8S_VERSION_BARE}")

export OKE_NODE_IMAGE_OCID="$(
  oci ce node-pool-options get \
    --node-pool-option-id "${OKE_CLUSTER_OCID}" \
    --region "${OCI_REGION}" \
    --query "${OKE_IMAGE_QUERY}" \
    --raw-output
)"
echo "OKE_NODE_IMAGE_OCID=${OKE_NODE_IMAGE_OCID}"
# [END discover_node_image]

# [START create_system_pool]
oci ce node-pool create \
  --region "${OCI_REGION}" \
  --compartment-id "${OCI_COMPARTMENT_OCID}" \
  --cluster-id "${OKE_CLUSTER_OCID}" \
  --name 'system' \
  --kubernetes-version "${K8S_VERSION}" \
  --node-shape "${GENERAL_NODE_SHAPE}" \
  --node-shape-config "{\"ocpus\": ${GENERAL_NODE_OCPUS}, \"memoryInGBs\": ${GENERAL_NODE_MEMORY_GBS}}" \
  --node-source-details "{\"sourceType\": \"IMAGE\", \"imageId\": \"${OKE_NODE_IMAGE_OCID}\", \"bootVolumeSizeInGBs\": 50}" \
  --placement-configs "[{\"availabilityDomain\": \"${OCI_AD}\", \"subnetId\": \"${OKE_WORKERS_SUBNET_OCID}\"}]" \
  --pod-subnet-ids "[\"${OKE_WORKERS_SUBNET_OCID}\"]" \
  --size 1 \
  --max-wait-seconds 1800 --wait-interval-seconds 30 \
  --wait-for-state SUCCEEDED \
  --wait-for-state FAILED
# [END create_system_pool]

# [START create_scylla_pool]
CLOUD_INIT_BASE64="$(base64 -w0 <<'EOF'
#!/bin/bash
set -euo pipefail
curl --fail -H "Authorization: Bearer Oracle" -L0 \
  http://169.254.169.254/opc/v2/instance/metadata/oke_init_script \
  | base64 --decode > /var/run/oke-init.sh
bash /var/run/oke-init.sh --kubelet-extra-args "--cpu-manager-policy=static"
EOF
)"

oci ce node-pool create \
  --region "${OCI_REGION}" \
  --compartment-id "${OCI_COMPARTMENT_OCID}" \
  --cluster-id "${OKE_CLUSTER_OCID}" \
  --name 'scylla' \
  --kubernetes-version "${K8S_VERSION}" \
  --node-shape "${SCYLLA_NODE_SHAPE}" \
  --node-source-details "{\"sourceType\": \"IMAGE\", \"imageId\": \"${OKE_NODE_IMAGE_OCID}\", \"bootVolumeSizeInGBs\": 50}" \
  --placement-configs "[
    {\"availabilityDomain\": \"${OCI_AD}\", \"subnetId\": \"${OKE_WORKERS_SUBNET_OCID}\", \"faultDomains\": [\"FAULT-DOMAIN-1\", \"FAULT-DOMAIN-2\", \"FAULT-DOMAIN-3\"]}
  ]" \
  --pod-subnet-ids "[\"${OKE_WORKERS_SUBNET_OCID}\"]" \
  --size 3 \
  --initial-node-labels '[{"key": "scylla.scylladb.com/node-type", "value": "scylla"}]' \
  --node-metadata "{\"user_data\": \"${CLOUD_INIT_BASE64}\"}" \
  --max-wait-seconds 1800 --wait-interval-seconds 30 \
  --wait-for-state SUCCEEDED \
  --wait-for-state FAILED
# [END create_scylla_pool]

# [START get_credentials]
oci ce cluster create-kubeconfig \
  --region "${OCI_REGION}" \
  --cluster-id "${OKE_CLUSTER_OCID}" \
  --file "${HOME}/.kube/config" \
  --token-version '2.0.0' \
  --kube-endpoint PUBLIC_ENDPOINT
# [END get_credentials]

# [START taint_nodes]
kubectl taint nodes -l 'scylla.scylladb.com/node-type=scylla' \
  scylla-operator.scylladb.com/dedicated=scyllaclusters:NoSchedule --overwrite
# [END taint_nodes]

echo ""
echo "OKE cluster '${OKE_CLUSTER_NAME}' is ready."
echo "Verify nodes with: kubectl get nodes -L scylla.scylladb.com/node-type -L oci.oraclecloud.com/fault-domain"
