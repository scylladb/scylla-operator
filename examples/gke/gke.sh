#!/bin/bash
set -e

display_usage() {
	echo "End-to-end deployment script for scylla on GKE."
	echo "usage: $0 [GCP user] [GCP project] [GCP zone]"
    echo "example: $0 yanniszark@arrikto.com gke-demo-226716 us-west1-b"
}

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
until [[ "$(gcloud container clusters list --zone=us-east1-b | grep scylla-demo | awk '{ print $8 }')" == "RUNNING" ]]; do 
  echo "Waiting for cluster readiness... "
  echo $(gcloud container clusters list --zone=us-east1-b | grep scylla-demo)
  sleep 10
  WAIT_TIME=$((WAIT_TIME+10))
  if [[  "$(gcloud container operations list --sort-by=START_TIME --filter="scylla-demo AND UPGRADE_MASTER" | grep RUNNING)" != "" ]]; then
    gcloud container operations list --sort-by=START_TIME --filter="scylla-demo AND UPGRADE_MASTER"
    gcloud container operations wait $(gcloud container operations list --sort-by=START_TIME --filter="scylla-demo AND UPGRADE_MASTER" | tail -1 | awk '{print $1}')
  else 
    gcloud container operations list --sort-by=START_TIME --filter="scylla-demo AND UPGRADE_MASTER" | tail -1
  fi
done
gcloud container clusters list --zone=us-east1-b | grep scylla-demo
}

check_tiller_readiness(){
until [[ $(kubectl get deployment tiller-deploy -n kube-system -o 'jsonpath={.status.readyReplicas}') -eq 1 ]];
do
    echo "Waiting for Tiller pod to become Ready..."
    sleep 5
done
}

if [ $# -ne 3 ]; then
    echo "illegal number of parameters"
    display_usage
    exit 1
fi

GCP_USER=$1
GCP_PROJECT=$2
GCP_ZONE=$3
GCP_REGION=${GCP_ZONE:0:$((${#GCP_ZONE}-2))}
CLUSTER_NAME=scylla-demo
CLUSTER_VERSION=1.12.7-gke.7

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
--num-nodes "5" \
--disk-type "pd-ssd" --disk-size "20" \
--local-ssd-count "8" \
--node-labels role=scylla-clusters \
--image-type "UBUNTU" \
--enable-cloud-logging --enable-cloud-monitoring \
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
--node-labels role=cassandra-stress \
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
--node-labels role=scylla-operator \
--image-type "UBUNTU" \
--no-enable-autoupgrade --no-enable-autorepair

# After gcloud returns, it's going to upgrade the master
# making the cluster unavailable for a while.
# We deal with this by waiting a while for the unavailability
# to start and then polling with kubectl to detect when it ends.

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
# Setup Tiller
echo "Setting up Tiller..."
helm init
kubectl create serviceaccount --namespace kube-system tiller
kubectl create clusterrolebinding tiller-cluster-rule --clusterrole=cluster-admin --serviceaccount=kube-system:tiller
kubectl patch deploy --namespace kube-system tiller-deploy -p '{"spec":{"template":{"spec":{"serviceAccount":"tiller"}}}}'

check_cluster_readiness
# Install RAID Daemonset
echo "Installing RAID Daemonset..."
kubectl apply -f raid-daemonset.yaml

check_cluster_readiness
# Install cpu-policy Daemonset
echo "Installing cpu-policy Daemonset..."
sleep 5
kubectl apply -f cpu-policy-daemonset.yaml

# Wait for Tiller to become ready
check_tiller_readiness
# Install local volume provisioner
echo "Installing local volume provisioner..."
helm install --name local-provisioner provisioner
echo "Your disks are ready to use."

echo "Start the scylla operator:"
echo "kubectl apply -f operator.yaml"

echo "Start the scylla cluster:"
sed -i "s/<gcp_region>/${GCP_REGION}/g;s/<gcp_zone>/${GCP_ZONE}/g" cluster.yaml
echo "kubectl apply -f cluster.yaml"
