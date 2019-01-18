# Deploying Scylla on GKE

This guide is focused on deploying Scylla on GKE with maximum performance. It sets up the kubelets on GKE nodes to run with [static cpu policy](https://kubernetes.io/blog/2018/07/24/feature-highlight-cpu-manager/) and uses [local sdd disks](https://cloud.google.com/kubernetes-engine/docs/how-to/persistent-volumes/local-ssd) in RAID0 for maximum performance.

## TLDR

If you don't want to run the commands step-by-step, you can just run a script that will set everything up for you:
```bash
# From inside the docs/gke folder 
./gke.sh [GCP user] [GCP project] [GCP zone]

# Example:
# ./gke.sh yanniszark@arrikto.com gke-demo-226716 us-west1-b
```

After you deploy, see how you can [benchmark your cluster with cassandra-stress]().

## Walkthrough

// Port juliana's guide here

## Benchmark with cassandra-stress

After deploying our cluster along with the monitoring, we can benchmark it using cassandra-stress. We have a mini cli that generates Kubernetes Jobs that run cassandra-stress against a cluster.

```bash
# Download cass-stress-gen.py
wget https://gist.githubusercontent.com/yanniszark/08b1e4938ef0030bcf65cdf6ae80eb12/raw/92a03e952da50f42107e5776cd8923539f466ab5/cass-stress-gen.py
chmod +x ./cass-stress-gen.py

# Run a benchmark with 9 jobs, with 10 cpus and 50000000 operations each
./cass-stress-gen.py --num-jobs=9 --cpu=10 --ops 50000000 --nodeselector role=cassandra-stress
kubectl apply -f ./cassandra-stress.yaml
``` 

After the Jobs finish, clean them up with:
```bash
kubectl delete -f ./cassandra-stress.yaml
```