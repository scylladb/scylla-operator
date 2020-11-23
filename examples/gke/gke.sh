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

GCP_REGION=${GCP_ZONE:0:$((${#GCP_ZONE}-2))}

CLUSTER_VERSION="$(gcloud container get-server-config --zone ${GCP_ZONE} --format "value(validMasterVersions[0])")"

check_prerequisites() {
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

check_cluster_readiness() {
until [[ "$(gcloud container clusters list --zone=${GCP_ZONE} | grep ${CLUSTER_NAME} | awk '{ print $8 }')" == "RUNNING" ]]; do
  echo "Waiting for cluster readiness... "
  echo $(gcloud container clusters list --zone=${GCP_ZONE} | grep ${CLUSTER_NAME})
  sleep 10
  WAIT_TIME=$((WAIT_TIME+10))
  if [[  "$(gcloud container operations list --sort-by=START_TIME --filter="${CLUSTER_NAME} AND UPGRADE_MASTER" | grep RUNNING)" != "" ]]; then
    gcloud container operations list --sort-by=START_TIME --filter="${CLUSTER_NAME} AND UPGRADE_MASTER"
    gcloud container operations wait $(gcloud container operations list --sort-by=START_TIME --filter="${CLUSTER_NAME} AND UPGRADE_MASTER" | tail -1 | awk '{print $1}') --zone="${GCP_ZONE}"
  else 
    gcloud container operations list --sort-by=START_TIME --filter="${CLUSTER_NAME} AND UPGRADE_MASTER" | tail -1
  fi
done
gcloud container clusters list --zone="${GCP_ZONE}" | grep ${CLUSTER_NAME}
}

# Check if the environment has the prerequisites installed
check_prerequisites

# gcloud: Create GKE cluster

# Nodepool for scylla clusters
# Do NOT use gcloud beta
gcloud container --project "${GCP_PROJECT}" \
clusters create "${CLUSTER_NAME}" --username "admin" \
--zone "${GCP_ZONE}" \
--cluster-version "${CLUSTER_VERSION}" \
--node-version "${CLUSTER_VERSION}" \
--machine-type "n1-standard-32" \
--num-nodes "4" \
--disk-type "pd-ssd" --disk-size "20" \
--local-ssd-count "8" \
--node-taints role=scylla-clusters:NoSchedule \
--image-type "UBUNTU" \
--enable-cloud-logging --enable-stackdriver-kubernetes \
--no-enable-autoupgrade --no-enable-autorepair

# Nodepool for cassandra-stress pods
gcloud container --project "${GCP_PROJECT}" \
node-pools create "cassandra-stress-pool" \
--cluster "${CLUSTER_NAME}" \
--zone "${GCP_ZONE}" \
--node-version "${CLUSTER_VERSION}" \
--machine-type "n1-standard-32" \
--num-nodes "2" \
--disk-type "pd-ssd" --disk-size "20" \
--node-taints role=cassandra-stress:NoSchedule \
--image-type "UBUNTU" \
--no-enable-autoupgrade --no-enable-autorepair

# Nodepool for scylla operator and monitoring
gcloud container --project "${GCP_PROJECT}" \
node-pools create "operator-pool" \
--cluster "${CLUSTER_NAME}" \
--zone "${GCP_ZONE}" \
--node-version "${CLUSTER_VERSION}" \
--machine-type "n1-standard-8" \
--num-nodes "1" \
--disk-type "pd-ssd" --disk-size "20" \
--image-type "UBUNTU" \
--no-enable-autoupgrade --no-enable-autorepair

# After gcloud returns, it's going to upgrade the master
# making the cluster unavailable for a while.
# We deal with this by waiting a while for the unavailability
# to start and then polling with kubectl to detect when it ends.

# Is this command needed ???
gcloud container operations list --sort-by START_TIME | tail

echo "Waiting GKE to UPGRADE_MASTER"
sleep 120
check_cluster_readiness
# gcloud: Get credentials for new cluster
echo "Getting credentials for newly created cluster..."
gcloud container clusters get-credentials "${CLUSTER_NAME}" --zone="${GCP_ZONE}"

sleep 60
check_cluster_readiness
# Setup GKE RBAC
echo "Setting up GKE RBAC..."
kubectl create clusterrolebinding cluster-admin-binding --clusterrole cluster-admin --user "${GCP_USER}"

check_cluster_readiness
# Install RAID Daemonset
echo "Installing RAID Daemonset..."
kubectl apply -f raid-daemonset.yaml

check_cluster_readiness
# Install cpu-policy Daemonset
echo "Installing cpu-policy Daemonset..."
sleep 5
kubectl apply -f cpu-policy-daemonset.yaml

# Install local volume provisioner
echo "Installing local volume provisioner..."
helm install local-provisioner ../common/provisioner
echo "Your disks are ready to use."

echo "Starting the cert manger..."
kubectl apply -f ../common/cert-manager.yaml
sleep 30
echo "Starting the scylla operator..."
kubectl apply -f ../common/operator.yaml
sleep 30
echo "Starting the scylla cluster..."
sed "s/<gcp_region>/${GCP_REGION}/g;s/<gcp_zone>/${GCP_ZONE}/g" cluster.yaml | kubectl apply -f -
