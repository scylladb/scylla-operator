#!/bin/bash
set -euo pipefail

#########
# Start #
#########

display_usage() {
	echo "End-to-end deployment script for scylla on EKS."
	echo "usage: $0 -r|--eks-region [EKS region] -z|--eks-zones [comma separated list of zones] -c|--k8s-cluster-name [cluster name (optional)]"
}

CLUSTER_NAME=scylla-demo

while (( "$#" )); do
  case "$1" in
    -r|--eks-region)
      if [ -n "$2" ] && [ ${2:0:1} != "-" ]; then
        EKS_REGION=$2
        shift 2
      else
        echo "Error: Argument for $1 is missing" >&2
        exit 1
      fi
      ;;
    -z|--eks-zones)
      if [ -n "$2" ] && [ ${2:0:1} != "-" ]; then
        IFS=', ' read -r -a EKS_ZONES <<< "$2"
        shift 2
      else
        echo "Error: Argument for $1 is missing" >&2
        exit 1
      fi
      ;;
    -c|--k8s-cluster-name)
      if [ -n "$2" ] && [ ${2:0:1} != "-" ]; then
        CLUSTER_NAME=$2
        shift 2
      else
        echo "Error: Argument for $1 is missing" >&2
        exit 1
      fi
      ;;
    -h|--help)
      display_usage
      exit 1
      ;;
    -*|--*=) # unsupported flags
      echo "Error: Unsupported flag $1" >&2
      exit 1
      ;;
    *) # preserve positional arguments
      PARAMS="$PARAMS $1"
      shift
      ;;
  esac
done

if [ -z "$EKS_REGION" ] || [ -z "$EKS_ZONES" ]
then
  display_usage
  exit 1
fi

function check_input() {
  for zone in "${EKS_ZONES[@]}"
  do
    if ! aws ec2 describe-availability-zones --region $EKS_REGION | jq -e ".AvailabilityZones[] | select(.ZoneName==\"$zone\")" > /dev/null; then
      echo "Availability zone $zone not found in $EKS_REGION"
      exit 1
    fi
  done
}

check_prerequisites() {
  echo "Checking if eksctl is present on the machine..."
    if ! hash eksctl 2>/dev/null; then
        echo "You need to install eksctl. See: https://docs.aws.amazon.com/eks/latest/userguide/getting-started-eksctl.html"
        exit 1
    fi

    echo "Checking if kubectl is present on the machine..."
    if ! hash kubectl 2>/dev/null; then
        echo "You need to install kubectl. See: https://kubernetes.io/docs/tasks/tools/install-kubectl/"
        exit 1
    fi

    echo "Checking if aws is present on the machine..."
    if ! hash aws 2>/dev/null; then
        echo "You need to install AWS CLI. See: https://aws.amazon.com/cli/"
        exit 1
    fi

    echo "Checking if yq is present on the machine..."
    if ! hash yq 2>/dev/null; then
        echo "You need to install yq. See: https://github.com/mikefarah/yq"
        exit 1
    fi
}

function wait-for-object-creation {
    for i in {1..30}; do
        { kubectl -n "${1}" get "${2}" && break; } || sleep 1
    done
}

# Check if user provided values makes sense
check_input

# Check if the environment has the prerequisites installed
check_prerequisites

# Create EKS cluster

EKS_ZONES_QUOTED=$(printf ',"%s"' "${EKS_ZONES[@]}")
EKS_ZONES_QUOTED="${EKS_ZONES_QUOTED:1}"
yq eval -P ".metadata.region = \"${EKS_REGION}\" | .metadata.name = \"${CLUSTER_NAME}\" | .availabilityZones |= [${EKS_ZONES_QUOTED}] | (.nodeGroups[] | select(.name==\"scylla-pool\") | .availabilityZones) |= [${EKS_ZONES_QUOTED}]" eks-cluster.yaml | eksctl create cluster -f -

echo "Starting the cert manger..."
kubectl apply -f ../common/cert-manager.yaml
kubectl wait --for condition=established --timeout=60s crd/certificates.cert-manager.io crd/issuers.cert-manager.io
wait-for-object-creation cert-manager deployment.apps/cert-manager-webhook
kubectl -n cert-manager rollout status --timeout=5m deployment.apps/cert-manager-webhook

echo "Starting the scylla operator..."
kubectl apply -f ../common/operator.yaml
kubectl wait --for condition=established crd/nodeconfigs.scylla.scylladb.com
kubectl wait --for condition=established crd/scyllaclusters.scylla.scylladb.com
wait-for-object-creation scylla-operator deployment.apps/scylla-operator
kubectl -n scylla-operator rollout status --timeout=5m deployment.apps/scylla-operator
kubectl -n scylla-operator rollout status --timeout=5m deployment.apps/webhook-server

# Configure nodes
kubectl apply -f nodeconfig-alpha.yaml
wait-for-object-creation default nodeconfig.scylla.scylladb.com/cluster

# Install local volume provisioner
echo "Installing local volume provisioner..."
kubectl -n local-csi-driver apply --server-side -f ../common/local-volume-provisioner/local-csi-driver/
wait-for-object-creation local-csi-driver daemonset.apps/local-csi-driver
kubectl -n local-csi-driver rollout status --timeout=5m daemonset.apps/local-csi-driver
kubectl apply --server-side -f ../common/local-volume-provisioner/storageclass_xfs.yaml
echo "Your disks are ready to use."

echo "Starting the scylla cluster..."
kubectl apply -f cluster.yaml
