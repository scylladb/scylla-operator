# Quickstart: ScyllaDB on GKE

This tutorial walks you through deploying a ScyllaDB cluster on Google Kubernetes Engine (GKE) in about 30 minutes.
By the end, you will have a running ScyllaDB cluster, connect to it with `cqlsh`, and learn how to clean up.

:::{note}
This quickstart uses developer mode with minimal resources for evaluation purposes.
For production deployments, see the [production checklist](../deploying/production-checklist.md).
:::

## Prerequisites

- A Google Cloud project with billing enabled.
- The `gcloud` CLI installed and authenticated (`gcloud auth login`).
- `kubectl` installed.

## Create a GKE cluster

### Configure the kubelet for static CPU policy

Create a kubelet configuration file that enables the [static CPU manager policy](https://kubernetes.io/docs/tasks/administer-cluster/cpu-management-policies/#static-policy), which is required for CPU pinning:

:::{code-block} bash
cat > systemconfig.yaml <<EOF
kubeletConfig:
  cpuManagerPolicy: static
EOF
:::

### Create the cluster

Create a GKE cluster with a default node pool for system components:

:::{code-block} bash
gcloud container \
clusters create 'scylladb-quickstart' \
--region='us-central1' \
--cluster-version="latest" \
--machine-type='n2-standard-8' \
--num-nodes='1' \
--disk-type='pd-ssd' --disk-size='20' \
--image-type='UBUNTU_CONTAINERD' \
--no-enable-autoupgrade \
--no-enable-autorepair
:::

### Create a dedicated node pool for ScyllaDB

Create a node pool with local NVMe SSDs spread across multiple zones for availability:

:::{code-block} bash
gcloud container \
node-pools create 'scylladb-pool' \
--region='us-central1' \
--node-locations='us-central1-a,us-central1-b,us-central1-c' \
--cluster='scylladb-quickstart' \
--node-version="latest" \
--machine-type='n2-standard-8' \
--num-nodes='1' \
--disk-type='pd-ssd' --disk-size='20' \
--local-nvme-ssd-block='count=1' \
--image-type='UBUNTU_CONTAINERD' \
--system-config-from-file='systemconfig.yaml' \
--no-enable-autoupgrade \
--no-enable-autorepair \
--node-labels='scylla.scylladb.com/node-type=scylla' \
--node-taints='scylla-operator.scylladb.com/dedicated=scyllaclusters:NoSchedule'
:::

:::{note}
Using `--region` with `--node-locations` creates nodes across multiple zones within the region.
This is important for spreading ScyllaDB racks across availability zones.
With `--num-nodes=1` and three zones, you get one node per zone (three nodes total).
:::

### Get cluster credentials

:::{code-block} bash
gcloud container clusters get-credentials 'scylladb-quickstart' --region='us-central1'
:::

## Install xfsprogs

GKE Ubuntu images starting with version `1.32.1-gke.1002000` do not include the `xfsprogs` package, which is required for formatting local NVMe disks with XFS.
Install it using the provided DaemonSet:

:::{code-block} shell
:substitutions:
kubectl apply --server-side -f=https://raw.githubusercontent.com/{{repository}}/{{revision}}/examples/gke/install-xfsprogs.daemonset.yaml
:::

Wait for it to complete on all nodes:

:::{code-block} shell
kubectl -n=kube-system rollout status --timeout=5m daemonset.apps/install-xfsprogs
:::

## Install ScyllaDB Operator

### Install cert-manager

:::{code-block} shell
:substitutions:
kubectl apply --server-side -f=https://raw.githubusercontent.com/{{repository}}/{{revision}}/examples/third-party/cert-manager.yaml
:::

:::{code-block} shell
kubectl wait --for condition=established --timeout=60s crd/certificates.cert-manager.io crd/issuers.cert-manager.io
for deploy in cert-manager{,-cainjector,-webhook}; do
    kubectl -n=cert-manager rollout status --timeout=10m deployment.apps/"${deploy}"
done
:::

### Install Prometheus Operator

:::{code-block} shell
:substitutions:
kubectl apply -n prometheus-operator --server-side -f=https://raw.githubusercontent.com/{{repository}}/{{revision}}/examples/third-party/prometheus-operator.yaml
:::

:::{code-block} shell
kubectl wait --for='condition=established' crd/prometheuses.monitoring.coreos.com crd/prometheusrules.monitoring.coreos.com crd/servicemonitors.monitoring.coreos.com
kubectl -n=prometheus-operator rollout status --timeout=10m deployment.apps/prometheus-operator
:::

### Install ScyllaDB Operator

:::{code-block} shell
:substitutions:
kubectl -n=scylla-operator apply --server-side -f=https://raw.githubusercontent.com/{{repository}}/{{revision}}/deploy/operator.yaml
:::

:::{code-block} shell
kubectl wait --for='condition=established' crd/scyllaclusters.scylla.scylladb.com crd/nodeconfigs.scylla.scylladb.com crd/scyllaoperatorconfigs.scylla.scylladb.com crd/scylladbmonitorings.scylla.scylladb.com
kubectl -n=scylla-operator rollout status --timeout=10m deployment.apps/{scylla-operator,webhook-server}
:::

### Set up NodeConfig

Apply the GKE NodeConfig to prepare the local NVMe disks (RAID, XFS filesystem, sysctls, and performance tuning):

:::{code-block} shell
:substitutions:
kubectl apply --server-side -f=https://raw.githubusercontent.com/{{repository}}/{{revision}}/examples/gke/nodeconfig-alpha.yaml
:::

Wait for all nodes to be configured:

:::{code-block} shell
kubectl wait --for='condition=Reconciled' --timeout=10m nodeconfigs.scylla.scylladb.com/scylladb-nodepool-1
:::

### Install Local CSI Driver

:::{code-block} shell
:substitutions:
kubectl -n=local-csi-driver apply --server-side -f=https://raw.githubusercontent.com/{{repository}}/{{revision}}/examples/common/local-volume-provisioner/local-csi-driver/{00_clusterrole_def,00_clusterrole_def_openshift,00_clusterrole,00_namespace,00_scylladb-local-xfs.storageclass,10_csidriver,10_serviceaccount,20_clusterrolebinding,50_daemonset}.yaml
:::

:::{code-block} shell
kubectl -n=local-csi-driver rollout status --timeout=10m daemonset.apps/local-csi-driver
:::

### Install ScyllaDB Manager

:::{code-block} shell
:substitutions:
kubectl -n=scylla-manager apply --server-side -f=https://raw.githubusercontent.com/{{repository}}/{{revision}}/deploy/manager-dev.yaml
:::

:::{code-block} shell
kubectl -n=scylla-manager rollout status --timeout=10m deployment.apps/scylla-manager
:::

## Deploy a ScyllaDB cluster

Create a ScyllaDB configuration file with authentication enabled:

:::{code-block} bash
kubectl apply --server-side -f=- <<EOF
apiVersion: v1
kind: ConfigMap
metadata:
  name: scylladb-config
data:
  scylla.yaml: |
    authenticator: PasswordAuthenticator
    authorizer: CassandraAuthorizer
EOF
:::

Create a minimal ScyllaDB cluster in developer mode with one node per zone:

:::{code-block} bash
:substitutions:
kubectl apply --server-side -f=- <<EOF
apiVersion: scylla.scylladb.com/v1
kind: ScyllaCluster
metadata:
  name: scylladb
spec:
  repository: {{imageRepository}}
  version: {{scyllaDBImageTag}}
  agentVersion: {{agentVersion}}
  developerMode: true
  automaticOrphanedNodeCleanup: true
  datacenter:
    name: us-central1
    racks:
    - name: us-central1-a
      members: 1
      scyllaConfig: scylladb-config
      storage:
        capacity: 100Gi
        storageClassName: scylladb-local-xfs
      resources:
        requests:
          cpu: 10m
          memory: 100Mi
        limits:
          cpu: 1
          memory: 1Gi
      placement:
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
            - matchExpressions:
              - key: topology.kubernetes.io/zone
                operator: In
                values:
                - us-central1-a
              - key: scylla.scylladb.com/node-type
                operator: In
                values:
                - scylla
        tolerations:
        - key: scylla-operator.scylladb.com/dedicated
          operator: Equal
          value: scyllaclusters
          effect: NoSchedule
    - name: us-central1-b
      members: 1
      scyllaConfig: scylladb-config
      storage:
        capacity: 100Gi
        storageClassName: scylladb-local-xfs
      resources:
        requests:
          cpu: 10m
          memory: 100Mi
        limits:
          cpu: 1
          memory: 1Gi
      placement:
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
            - matchExpressions:
              - key: topology.kubernetes.io/zone
                operator: In
                values:
                - us-central1-b
              - key: scylla.scylladb.com/node-type
                operator: In
                values:
                - scylla
        tolerations:
        - key: scylla-operator.scylladb.com/dedicated
          operator: Equal
          value: scyllaclusters
          effect: NoSchedule
    - name: us-central1-c
      members: 1
      scyllaConfig: scylladb-config
      storage:
        capacity: 100Gi
        storageClassName: scylladb-local-xfs
      resources:
        requests:
          cpu: 10m
          memory: 100Mi
        limits:
          cpu: 1
          memory: 1Gi
      placement:
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
            - matchExpressions:
              - key: topology.kubernetes.io/zone
                operator: In
                values:
                - us-central1-c
              - key: scylla.scylladb.com/node-type
                operator: In
                values:
                - scylla
        tolerations:
        - key: scylla-operator.scylladb.com/dedicated
          operator: Equal
          value: scyllaclusters
          effect: NoSchedule
EOF
:::

:::{note}
This cluster uses `developerMode: true` with minimal resources for quick evaluation.
Developer mode disables several production optimizations, including I/O scheduling and memory locking.
Do not use developer mode for production workloads or performance benchmarking.
:::

Wait for the cluster to become available:

:::{code-block} shell
kubectl wait --for='condition=Available=True' --timeout=10m scyllaclusters.scylla.scylladb.com/scylladb
:::

Verify the cluster status:

:::{code-block} shell
kubectl get scyllaclusters.scylla.scylladb.com/scylladb
:::

Expected output:
```
NAME       READY   MEMBERS   RACKS   AVAILABLE   PROGRESSING   DEGRADED   AGE
scylladb   3       3         3       True        False         False      5m
```

## Connect with cqlsh

Connect to the cluster using `cqlsh` from within one of the ScyllaDB pods:

:::{code-block} shell
kubectl exec -it pod/scylladb-us-central1-us-central1-a-0 -c scylla -- cqlsh localhost -u cassandra -p cassandra
:::

Try a few CQL commands:

:::{code-block} cql
CREATE KEYSPACE IF NOT EXISTS example WITH replication = {'class': 'NetworkTopologyStrategy', 'us-central1': 3};
USE example;
CREATE TABLE users (id UUID PRIMARY KEY, name TEXT, email TEXT);
INSERT INTO users (id, name, email) VALUES (uuid(), 'Alice', 'alice@example.com');
SELECT * FROM users;
:::

Type `exit` to leave the `cqlsh` session.

## Clean up

Delete the ScyllaDB cluster:

:::{code-block} shell
kubectl delete scyllaclusters.scylla.scylladb.com/scylladb
:::

Delete the GKE cluster:

:::{code-block} shell
gcloud container clusters delete --region='us-central1' 'scylladb-quickstart'
:::

## Next steps

- Follow the [EKS Quickstart](quickstart-eks.md) to try the same on AWS.
- [Install](../installation/index.md) ScyllaDB Operator in your production environment.
- Review the [production checklist](../deploying/production-checklist.md) before going to production.
- Learn how to [connect your application](../connecting/index.md) to ScyllaDB.
