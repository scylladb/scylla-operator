# Scylla Operator
> Kubernetes Operator for Scylla (Pre-release version :warning:)

![](https://pbs.twimg.com/media/DwknrKJWkAE7qEQ.jpg)

## TODO

- [x] Local disks
- [ ] Prometheus + Grafana Monitoring
- [ ] Node Exporter

## Index

1. [Pre-requisites](#pre-requisites)
2. [Installation](#installation)
   1. [Google Kubernetes Engine Setup](#google-kubernetes-engine-setup)
   2. [Installing Required Tools](#installing-required-tools)
   3. [Scylla Operator](#installing-the-scylla-operator)
   4. [Setting up Monitoring (Grafana + Prometheus)](#setting-up-monitoring)
3. [Usage Example](#usage-example)
4. [Release History](#release-history)
5. [License](#license)

## Pre-requisites

For now, we tested and support GKE, but might work on other environments. More coming up! 

## Installation


### Google Kubernetes Engine Setup

#### Creating a GKE cluster

The following creates a cluster with 8 local disks. Those are **ephemeral** ssds attached to each Kubernetes node. It is important to disable `autoupgrade` and `autorepair`, since they cause Kubernetes nodes restarts and consequent loss of data. If you don't want to use ephemeral storage, you can tweak this command to your liking.    

```
gcloud beta container --project $GOOGLE_PROJECT clusters create scylla-test-cluster \
                        --zone "us-west1-b" \
                        --username "admin" \
                        --cluster-version "1.11.5-gke.5" \
                        --machine-type "n1-standard-32" \
                        --image-type "UBUNTU" \
                        --disk-type "pd-ssd" \
                        --disk-size "20" \
                        --node-labels role=scylla \
                        --local-ssd-count "8" \
                        --num-nodes "6" \
                        --enable-cloud-logging \
                        --enable-cloud-monitoring \
                        --no-enable-ip-alias \
                        --no-enable-autoupgrade \
                        --no-enable-autorepair
```

#### Setting Yourself as `cluster-admin`
> (Because you will need permissions)

Get the credentials for your new cluster
```
gcloud container clusters get-credentials scylla-test-cluster --zone=us-west1-b
```

Create a ClusterRoleBinding for you
```
kubectl create clusterrolebinding cluster-admin-binding --clusterrole cluster-admin --user $GCP_USER
```


↳ Next step: [Installing Required Tools](#installing-required-tools)

---

### Installing Required Tools 

#### Installing Helm

Helm is needed to enable multiple features. If you don't have Helm installed in your cluster, follow this:

1. Go to their [docs](https://docs.helm.sh/using_helm/#installing-helm) to get the binary for your distro.
2. `helm init`
3. Give Helm `cluster-admin` role:
```
kubectl create serviceaccount --namespace kube-system tiller
kubectl create clusterrolebinding tiller-cluster-rule --clusterrole=cluster-admin --serviceaccount=kube-system:tiller
kubectl patch deploy --namespace kube-system tiller-deploy -p '{"spec":{"template":{"spec":{"serviceAccount":"tiller"}}}}'
```

#### Install RAID DaemonSet

(If you don't want to setup RAID local disks, just skip this step.)

Inside this repository folder, run:

```
kubectl apply -f docs/gke/manifests/raid-daemonset.yaml
```

#### Install `cpu-policy` Daemonset

(If you don't want to setup cpu-policy, just skip this step.)

```
kubectl apply -f docs/gke/manifests/cpu-policy-daemonset.yaml
```
#### Install the local provisioner

(If you don't want to setup the local provisioner, just skip this step.)

```
helm install --name local-provisioner docs/gke/manifests/provisioner
```


### Installing the Scylla Operator

```
kubectl apply -f docs/gke/manifests/operator.yaml
```

Spining up Scylla Cluster!

```
kubectl apply -f docs/gke/manifests/simple_cluster.yaml
```

Check the status of your cluster

```
kubectl describe cluster simple-cluster -n scylla
```

### Setting up Monitoring

Both Prometheus and Grafana were configured to work out-of-the-box with Scylla Operator. Both of them will be available under the `monitoring` namespace. If you want to customize them, you can edit `prometheus/values.yaml` and `grafana/values.yaml` then run the following commands:

1. Install Prometheus
```
helm upgrade --install scylla-prom --namespace monitoring ./prometheus/
```

2. Install Grafana
```
helm upgrade --install scylla-graf --namespace monitoring ./grafana/
```

To see Grafana locally, run:

```
export POD_NAME=$(kubectl get pods --namespace monitoring -l "app=grafana,release=scylla-graf" -o jsonpath="{.items[0].metadata.name}")
kubectl --namespace monitoring port-forward $POD_NAME 3000
```

And access `http://0.0.0.0:3000` from your browser.

:warning: Keep in mind that Grafana needs Prometheus DNS to be visible to get information. The Grafana available in this files was configured to work with the name `scylla-prom` and `monitoring` namespace. You can edit this configuration under `grafana/values.yaml`.


## Usage example

TODO

## Bugs

If you find a bug or need help running scylla, you can reach out in the following ways:
* [Slack](https://scylladb-users-slackin.herokuapp.com/) in the `#scylla-operator` channel.
* File an [issue](https://github.com/kubernetes-sigs/kubebuilder/issues) describing the problem and how to reproduce.

## Acknowledgements

This project is based on cassandra operator, a community effort started by [yanniszark](https://github.com/yanniszark) of [Arrikto](https://www.arrikto.com/), as part of the [Rook project](https://rook.io/).


## Release History


* 0.0.1
    * Work in progress

## License


See ``LICENSE`` for more information.


