#!/bin/bash
set -euo pipefail

#########
# Start #
#########

display_usage() {
	echo "End-to-end deployment script for scylla on EKS."
	echo "usage: $0 -z|--eks-zone [EKS zone] -c|--k8s-cluster-name [cluster name (optional)]"
}

CLUSTER_NAME=scylla-demo

while (( "$#" )); do
  case "$1" in
    -z|--eks-zone)
      if [ -n "$2" ] && [ ${2:0:1} != "-" ]; then
        EKS_ZONE=$2
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

if [ -z "$EKS_ZONE" ]
then
  display_usage
  exit 1
fi

EKS_REGION=${EKS_ZONE:0:$((${#EKS_ZONE}-1))}

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

    echo "Checking if helm is present on the machine..."
    if ! hash helm 2>/dev/null; then
        echo "You need to install helm. See: https://docs.helm.sh/using_helm/#installing-helm"
        exit 1
    fi
}

function wait-for-object-creation {
    for i in {1..30}; do
        { kubectl -n "${1}" get "${2}" && break; } || sleep 1
    done
}

# Check if the environment has the prerequisites installed
check_prerequisites

# Create EKS cluster

sed "s/<eks_region>/${EKS_REGION}/g;s/<eks_cluster_name>/${CLUSTER_NAME}/g;s/<eks_zone>/${EKS_ZONE}/g" eks-cluster.yaml | eksctl create cluster -f -

# Configure node disks and network
kubectl apply -f node-setup-daemonset.yaml
wait-for-object-creation default daemonset.apps/raid-local-disks
kubectl rollout status --timeout=5m daemonset.apps/raid-local-disks

# Install local volume provisioner
echo "Installing local volume provisioner..."
helm install local-provisioner ../common/provisioner
echo "Your disks are ready to use."

echo "Starting the cert manger..."
kubectl apply -f ../common/cert-manager.yaml
kubectl wait --for condition=established --timeout=60s crd/certificates.cert-manager.io crd/issuers.cert-manager.io
wait-for-object-creation cert-manager deployment.apps/cert-manager-webhook
kubectl -n cert-manager rollout status --timeout=5m deployment.apps/cert-manager-webhook

echo "Starting the scylla operator..."
kubectl apply -f ../common/operator.yaml
kubectl wait --for condition=established crd/scyllaclusters.scylla.scylladb.com
wait-for-object-creation scylla-operator deployment.apps/scylla-operator
kubectl -n scylla-operator rollout status --timeout=5m deployment.apps/scylla-operator

echo "Starting the scylla cluster..."
sed "s/<eks_region>/${EKS_REGION}/g;s/<eks_zone>/${EKS_ZONE}/g" cluster.yaml | kubectl apply -f -
