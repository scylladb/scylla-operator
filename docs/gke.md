# Deploying Scylla on GKE

This guide is focused on deploying Scylla on GKE with maximum performance. It sets up the kubelets on GKE nodes to run with [static cpu policy](https://kubernetes.io/blog/2018/07/24/feature-highlight-cpu-manager/) and uses [local sdd disks](https://cloud.google.com/kubernetes-engine/docs/how-to/persistent-volumes/local-ssd) in RAID0 for maximum performance.

Because this guide focuses on showing a glimpse of the true performance of Scylla, we use 32 core machines with local SSDs. This might be overkill if all you want is a quick setup to play around with scylla operator. If you just want to quickly set up a Scylla cluster for the first time, we suggest you look at the [quickstart guide](generic.md) first.

## TL;DR;

If you don't want to run the commands step-by-step, you can just run a script that will set everything up for you:
```bash
# Edit according to your preference
GCP_USER=$(gcloud config list account --format "value(core.account)")
GCP_PROJECT=$(gcloud config list project --format "value(core.project)")
GCP_ZONE=us-west1-b

# From inside the examples/gke folder
cd examples/gke
./gke.sh "$GCP_USER" "$GCP_PROJECT" "$GCP_ZONE"

# Example:
# ./gke.sh yanniszark@arrikto.com gke-demo-226716 us-west1-b
```

:warning: Make sure to pass a ZONE (ex.: us-west1-b) and not a REGION (ex.: us-west1) or it will deploy nodes in each ZONE available in the region.

After you deploy, see how you can [benchmark your cluster with cassandra-stress](#benchmark-with-cassandra-stress).

## Walkthrough


### Google Kubernetes Engine Setup

#### Configure environment variables

First of all, we export all the configuration options as environment variables.
Edit according to your own environment.

```
GCP_USER=$(gcloud config list account --format "value(core.account)")
GCP_PROJECT=$(gcloud config list project --format "value(core.project)")
GCP_REGION=us-west1
GCP_ZONE=us-west1-b
CLUSTER_NAME=scylla-demo
```

#### Creating a GKE cluster

For this guide, we'll create a GKE cluster with the following:

1. A NodePool of 3 `n1-standard-32` Nodes, where the Scylla Pods will be deployed. Each of these Nodes has 8 local SSDs attached, which will later be combined into a RAID0 array. It is important to disable `autoupgrade` and `autorepair`, since they cause loss of data on local SSDs. 

```
gcloud beta container --project "${GCP_PROJECT}" \
clusters create "${CLUSTER_NAME}" --username "admin" \
--zone "${GCP_ZONE}" \
--cluster-version "1.12.7-gke.10" \
--machine-type "n1-standard-32" \
--num-nodes "3" \
--disk-type "pd-ssd" --disk-size "20" \
--local-ssd-count "8" \
--node-taints role=scylla-clusters:NoSchedule \
--image-type "UBUNTU" \
--enable-cloud-logging --enable-cloud-monitoring \
--no-enable-autoupgrade --no-enable-autorepair
```

2. A NodePool of 2 `n1-standard-32` Nodes to deploy `cassandra-stress` later on.

```
gcloud beta container --project "${GCP_PROJECT}" \
node-pools create "cassandra-stress-pool" \
--cluster "${CLUSTER_NAME}" \
--zone "${GCP_ZONE}" \
--machine-type "n1-standard-32" \
--num-nodes "2" \
--disk-type "pd-ssd" --disk-size "20" \
--node-taints role=cassandra-stress:NoSchedule \
--image-type "UBUNTU" \
--no-enable-autoupgrade --no-enable-autorepair
```

3. A NodePool of 1 `n1-standard-4` Node, where the operator and the monitoring stack will be deployed.
```
gcloud beta container --project "${GCP_PROJECT}" \
node-pools create "operator-pool" \
--cluster "${CLUSTER_NAME}" \
--zone "${GCP_ZONE}" \
--machine-type "n1-standard-4" \
--num-nodes "1" \
--disk-type "pd-ssd" --disk-size "20" \
--image-type "UBUNTU" \
--no-enable-autoupgrade --no-enable-autorepair
```

#### Setting Yourself as `cluster-admin`
> (By default GKE doesn't give you the necessary RBAC permissions)

Get the credentials for your new cluster
```
gcloud container clusters get-credentials "${CLUSTER_NAME}" --zone="${GCP_ZONE}"
```

Create a ClusterRoleBinding for you
```
kubectl create clusterrolebinding cluster-admin-binding --clusterrole cluster-admin --user "${GCP_USER}"
```


### Installing Required Tools 

#### Installing Helm

Helm is needed to enable multiple features. If you don't have Helm installed in your cluster, follow this:

1. Go to the [helm docs](https://docs.helm.sh/using_helm/#installing-helm) to get the binary for your distro.
2. `helm init`
3. Give Helm `cluster-admin` role:
```
kubectl create serviceaccount --namespace kube-system tiller
kubectl create clusterrolebinding tiller-cluster-rule --clusterrole=cluster-admin --serviceaccount=kube-system:tiller
kubectl patch deploy --namespace kube-system tiller-deploy -p '{"spec":{"template":{"spec":{"serviceAccount":"tiller"}}}}'
```

#### Install RAID DaemonSet

To combine the local disks together in RAID0 arrays, we deploy a DaemonSet to do the work for us.

```
kubectl apply -f examples/gke/raid-daemonset.yaml
```

#### Install the local provisioner

After combining the local SSDs into RAID0 arrays, we deploy the local volume provisioner, which will discover their mount points and make them available as PersistentVolumes.
```
helm install --name local-provisioner examples/gke/provisioner
```

#### Install `cpu-policy` Daemonset

Scylla achieves top-notch performance by pinning the CPUs it uses. To enable this behaviour in Kubernetes, the kubelet must be configured with the [static CPU policy](https://kubernetes.io/blog/2018/07/24/feature-highlight-cpu-manager/). To configure the kubelets in the `scylla` and `cassandra-stress` NodePools, we deploy a DaemonSet to do the work for us. You'll notice the Nodes getting cordoned for a little while, but then everything will come back to normal.
```
kubectl apply -f examples/gke/cpu-policy-daemonset.yaml
```

### Installing the Scylla Operator

```
kubectl apply -f examples/gke/operator.yaml
```

Spinning up the Scylla Cluster!

```
sed "s/<gcp_region>/${GCP_REGION}/g;s/<gcp_zone>/${GCP_ZONE}/g" examples/gke/cluster.yaml | kubectl apply -f -
```

Check the status of your cluster

```
kubectl describe cluster scylla-cluster -n scylla
```

### Setting up Monitoring

Both Prometheus and Grafana were configured with specific rules for Scylla Operator. Both of them will be available under the `monitoring` namespace. Customization can be done in `examples/gke/prometheus/values.yaml` and `examples/gke/grafana/values.yaml`.

1. Install Prometheus
```
helm upgrade --install scylla-prom --namespace monitoring stable/prometheus -f examples/gke/prometheus/values.yaml
```

2. Install Grafana
```
helm upgrade --install scylla-graf --namespace monitoring stable/grafana -f examples/gke/grafana/values.yaml
```

To see Grafana locally, run:

```
export POD_NAME=$(kubectl get pods --namespace monitoring -l "app=grafana,release=scylla-graf" -o jsonpath="{.items[0].metadata.name}")
kubectl --namespace monitoring port-forward $POD_NAME 3000
```

And access `http://0.0.0.0:3000` from your browser and login with the credentials `admin`:`admin`.

3. Install dashboards

Get Grafana password and forward to localhost
```
export GRAFANA_PASSWORD=$(kubectl get secret --namespace monitoring scylla-graf-grafana -o jsonpath="{.data.admin-password}" | base64 --decode ; echo)
export POD_NAME=$(kubectl get pods --namespace monitoring -l "app=grafana,release=scylla-graf" -o jsonpath="{.items[0].metadata.name}")
kubectl --namespace monitoring port-forward $POD_NAME 3000
```

Clone scylla-grafana-monitoring project
```
git clone https://github.com/scylladb/scylla-grafana-monitorin /tmp
cd /tmp/scylla-grafana-monitoring
git checkout scylla-monitoring-2.3
rm -rf grafana/build/* && ./load-grafana.sh -a $GRAFANA_PASSWORD

```

:warning: Keep in mind that this is a test setup. For production use, check grafana and prometheus helm chart page for advanced deployment instructions.


## Benchmark with cassandra-stress

After deploying our cluster along with the monitoring, we can benchmark it using cassandra-stress and see its performance in Grafana. We have a mini cli that generates Kubernetes Jobs that run cassandra-stress against a cluster.

> Because cassandra-stress doesn't scale well to multiple cores, we use multiple jobs with a small core count for each

```bash

# Run a benchmark with 10 jobs, with 6 cpus and 50.000.000 operations each.
# Each Job will throttle throughput to 30.000 ops/sec for a total of 300.000 ops/sec.
scripts/cass-stress-gen.py --num-jobs=10 --cpu=6 --memory=20G --ops=50000000 --limit=30000
kubectl apply -f scripts/cassandra-stress.yaml
```

While the benchmark is running, open up Grafana and take a look at the monitoring metrics.

After the Jobs finish, clean them up with:
```bash
kubectl delete -f scripts/cassandra-stress.yaml
```
