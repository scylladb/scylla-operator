#!/bin/bash
set -e

#########
# Start #
#########

display_usage() {
	echo "End-to-end deployment script for scylla on GKE."
	echo "usage: $0 -u|--gcp-user [GCP user] -p|--gcp-project [GCP project] -z|--gcp-zone [GCP zone] -c|--k8s-cluster-name [cluster name (optional)]"
}

CLUSTER_NAME=$([ -z "$USER" ] && echo "scylla-demo" || echo "$USER-scylla-demo")

while (( "$#" )); do
  case "$1" in
    -u|--gcp-user)
      if [ -n "$2" ] && [ ${2:0:1} != "-" ]; then
        GCP_USER=$2
        shift 2
      else
        echo "Error: Argument for $1 is missing" >&2
        exit 1
      fi
      ;;
    -p|--gcp-project)
      if [ -n "$2" ] && [ ${2:0:1} != "-" ]; then
        GCP_PROJECT=$2
        shift 2
      else
        echo "Error: Argument for $1 is missing" >&2
        exit 1
      fi
      ;;
    -z|--gcp-zone)
      if [ -n "$2" ] && [ ${2:0:1} != "-" ]; then
        GCP_ZONE=$2
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

if [ "x$GCP_USER" == "x" ]
then
  display_usage
  exit 1
fi

if [ "x$GCP_PROJECT" == "x" ]
then
  display_usage
  exit 1
fi

if [ "x$GCP_ZONE" == "x" ]
then
  display_usage
  exit 1
fi

gcloud() {
  command gcloud "$@" --project "${GCP_PROJECT}" --zone "${GCP_ZONE}"
}

GCP_REGION="${GCP_ZONE:0:$((${#GCP_ZONE}-2))}"
CLUSTER_VERSION=$(gcloud container get-server-config --format "value(validMasterVersions[0])")

check_prerequisites() {
    echo "Checking if kubectl is present on the machine..."
    if ! hash kubectl 2>/dev/null; then
        echo "You need to install kubectl. See: https://kubernetes.io/docs/tasks/tools/install-kubectl/"
        exit 1
    fi
}

check_cluster_readiness() {
until [[ "$(gcloud container clusters list | grep ${CLUSTER_NAME} | awk '{ print $8 }')" == "RUNNING" ]]; do
  echo "Waiting for cluster readiness... "
  echo $(gcloud container clusters list | grep ${CLUSTER_NAME})
  sleep 10
  WAIT_TIME=$((WAIT_TIME+10))
  if [[  "$(gcloud container operations list --sort-by=START_TIME --filter="${CLUSTER_NAME} AND UPGRADE_MASTER" | grep RUNNING)" != "" ]]; then
    gcloud container operations list --sort-by=START_TIME --filter="${CLUSTER_NAME} AND UPGRADE_MASTER"
    gcloud container operations wait $(gcloud container operations list --sort-by=START_TIME --filter="${CLUSTER_NAME} AND UPGRADE_MASTER" | tail -1 | awk '{print $1}')
  else
    gcloud container operations list --sort-by=START_TIME --filter="${CLUSTER_NAME} AND UPGRADE_MASTER" | tail -1
  fi
done
gcloud container clusters list | grep "${CLUSTER_NAME}"
}

function wait-for-object-creation {
    for i in {1..30}; do
        { kubectl -n "${1}" get "${2}" && break; } || sleep 1
    done
}

# Check if the environment has the prerequisites installed
check_prerequisites

# gcloud: Create GKE cluster
# Nodepool for scylla operator and monitoring
gcloud container \
clusters create "${CLUSTER_NAME}" \
--cluster-version "${CLUSTER_VERSION}" \
--node-version "${CLUSTER_VERSION}" \
--machine-type "n1-standard-8" \
--num-nodes "2" \
--disk-type "pd-ssd" --disk-size "20" \
--image-type "UBUNTU_CONTAINERD" \
--enable-stackdriver-kubernetes \
--no-enable-autoupgrade \
--no-enable-autorepair

# Nodepool for cassandra-stress pods
gcloud container \
node-pools create "cassandra-stress-pool" \
--cluster "${CLUSTER_NAME}" \
--node-version "${CLUSTER_VERSION}" \
--machine-type "n1-standard-32" \
--num-nodes "2" \
--disk-type "pd-ssd" --disk-size "20" \
--node-taints role=cassandra-stress:NoSchedule \
--image-type "UBUNTU_CONTAINERD" \
--no-enable-autoupgrade \
--no-enable-autorepair

# Nodepool for scylla clusters
gcloud container \
node-pools create "scylla-pool" \
--cluster "${CLUSTER_NAME}" \
--node-version "${CLUSTER_VERSION}" \
--machine-type "n1-standard-32" \
--num-nodes "4" \
--disk-type "pd-ssd" --disk-size "20" \
--local-nvme-ssd-block count="8" \
--node-taints role=scylla-clusters:NoSchedule \
--node-labels scylla.scylladb.com/node-type=scylla \
--image-type "UBUNTU_CONTAINERD" \
--system-config-from-file=systemconfig.yaml \
--no-enable-autoupgrade \
--no-enable-autorepair

# After gcloud returns, it's going to upgrade the master
# making the cluster unavailable for a while.
# We deal with this by waiting a while for the unavailability
# to start and then polling with kubectl to detect when it ends.

echo "Waiting GKE to UPGRADE_MASTER"
sleep 120
check_cluster_readiness
# gcloud: Get credentials for new cluster
echo "Getting credentials for newly created cluster..."
gcloud container clusters get-credentials "${CLUSTER_NAME}"

# Setup GKE RBAC
echo "Setting up GKE RBAC..."
kubectl create clusterrolebinding cluster-admin-binding --clusterrole cluster-admin --user "${GCP_USER}"

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

echo "Configuring nodes..."
kubectl apply -f nodeconfig-alpha.yaml

# Install local volume provisioner
echo "Installing local volume provisioner..."
kubectl -n local-csi-driver apply --server-side -f ../common/local-volume-provisioner/local-csi-driver/
wait-for-object-creation local-csi-driver daemonset.apps/local-csi-driver
kubectl -n local-csi-driver rollout status --timeout=5m daemonset.apps/local-csi-driver
kubectl apply --server-side -f ../common/local-volume-provisioner/storageclass_xfs.yaml
echo "Your disks are ready to use."

echo "Starting the scylla cluster..."
sed "s/<gcp_region>/${GCP_REGION}/g;s/<gcp_zone>/${GCP_ZONE}/g" cluster.yaml | kubectl apply -f -
