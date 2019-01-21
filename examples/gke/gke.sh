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

# Check if the environment has the prerequisites installed
check_prerequisites

# gcloud: Create GKE cluster

# Nodepool for scylla clusters
echo "Creating GKE cluster..."
gcloud beta container --project "${GCP_PROJECT}" \
clusters create "${CLUSTER_NAME}" --username "admin" \
--zone "${GCP_ZONE}" \
--cluster-version "1.11.6-gke.2" \
--node-version "1.11.6-gke.2" \
--machine-type "n1-standard-32" \
--num-nodes "5" \
--disk-type "pd-ssd" --disk-size "20" \
--local-ssd-count "8" \
--node-labels role=scylla-clusters \
--image-type "UBUNTU" \
--enable-cloud-logging --enable-cloud-monitoring \
--no-enable-autoupgrade --no-enable-autorepair

# Nodepool for cassandra-stress pods
gcloud beta container --project "${GCP_PROJECT}" \
node-pools create "cassandra-stress-pool" \
--cluster "${CLUSTER_NAME}" \
--zone "${GCP_ZONE}" \
--node-version "1.11.6-gke.2" \
--machine-type "n1-standard-32" \
--num-nodes "2" \
--disk-type "pd-ssd" --disk-size "20" \
--node-labels role=cassandra-stress \
--image-type "UBUNTU" \
--no-enable-autoupgrade --no-enable-autorepair

# Nodepool for scylla operator and monitoring
gcloud beta container --project "${GCP_PROJECT}" \
node-pools create "operator-pool" \
--cluster "${CLUSTER_NAME}" \
--zone "${GCP_ZONE}" \
--node-version "1.11.6-gke.2" \
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
sleep 180
if [ ! "$(kubectl version &> /dev/null)" ]; then
        echo "Waiting for gke cluster to finish upgrading the control plane..."
        sleep 30
fi


# gcloud: Get credentials for new cluster
echo "Getting credentials for newly created cluster..."
gcloud container clusters get-credentials "${CLUSTER_NAME}" --zone="${GCP_ZONE}"

# Setup GKE RBAC
echo "Setting up GKE RBAC..."
kubectl create clusterrolebinding cluster-admin-binding --clusterrole cluster-admin --user "${GCP_USER}"

# Setup Tiller
echo "Setting up Tiller..."
helm init
kubectl create serviceaccount --namespace kube-system tiller
kubectl create clusterrolebinding tiller-cluster-rule --clusterrole=cluster-admin --serviceaccount=kube-system:tiller
kubectl patch deploy --namespace kube-system tiller-deploy -p '{"spec":{"template":{"spec":{"serviceAccount":"tiller"}}}}'

# Install RAID Daemonset
echo "Installing RAID Daemonset..."
kubectl apply -f manifests/raid-daemonset.yaml

# Install cpu-policy Daemonset
echo "Installing cpu-policy Daemonset..."
sleep 5
kubectl apply -f manifests/cpu-policy-daemonset.yaml

# Wait for Tiller to become ready
until [[ $(kubectl get deployment tiller-deploy -n kube-system -o 'jsonpath={.status.readyReplicas}') -eq 1 ]];
do
    echo "Waiting for Tiller pod to become Ready..."
    sleep 5
done

# Install local volume provisioner
echo "Installing local volume provisioner..."
helm install --name local-provisioner manifests/provisioner
echo "Your disks are ready to use."

echo "Start the scylla operator:"
echo "kubectl apply -f manifests/operator.yaml"

echo "Start the scylla cluster:"
sed "s/<gcp_region>/${GCP_REGION}/g;s/<gcp_zone>/${GCP_ZONE}/g" manifests/cluster.yaml
echo "kubectl apply -f manifests/cluster.yaml"
