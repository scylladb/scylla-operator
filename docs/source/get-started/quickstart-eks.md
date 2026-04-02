# Quickstart: ScyllaDB on EKS

This tutorial walks you through deploying a ScyllaDB cluster on Amazon Elastic Kubernetes Service (EKS) in about 30 minutes.
By the end, you will have a running ScyllaDB cluster, connect to it with `cqlsh`, and learn how to clean up.

:::{note}
This quickstart uses developer mode with minimal resources for evaluation purposes.
For production deployments, see the [production checklist](../deploy-scylladb/production-checklist.md).
:::

## Prerequisites

- An AWS account with permissions to create EKS clusters and EC2 instances.
- [`eksctl`](https://docs.aws.amazon.com/eks/latest/userguide/getting-started-eksctl.html) installed and configured with AWS credentials.
- `kubectl` installed.

## Create an EKS cluster

Download the cluster configuration file:

:::{code-block} shell
:substitutions:
curl --fail --retry 5 --retry-all-errors -o 'clusterconfig.eksctl.yaml' -L https://raw.githubusercontent.com/{{repository}}/{{revision}}/examples/eks/clusterconfig.eksctl.yaml
:::

This configuration creates:
- An EKS cluster in `eu-central-1` with three availability zones.
- A **ScyllaDB node pool** (`scylla-pool`) using `i4i.2xlarge` instances with local NVMe SSDs, one node per availability zone, with labels and taints to isolate ScyllaDB workloads.
- An **infrastructure node pool** (`infra-pool`) for system components.

Create the cluster:

:::{code-block} shell
eksctl create cluster -f=clusterconfig.eksctl.yaml
:::

:::{note}
Cluster creation takes approximately 15–20 minutes.
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

Apply the EKS NodeConfig to prepare the local NVMe disks (RAID, XFS filesystem, sysctls, and performance tuning):

:::{code-block} shell
:substitutions:
kubectl apply --server-side -f=https://raw.githubusercontent.com/{{repository}}/{{revision}}/examples/eks/nodeconfig-alpha.yaml
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

Create a minimal ScyllaDB cluster in developer mode with one node per availability zone:

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
  developerMode: true  # Disables performance tuning; do not use in production
  automaticOrphanedNodeCleanup: true
  datacenter:
    name: eu-central-1  # Must match the AWS region/naming convention you use for topology
    racks:
    - name: eu-central-1a  # One rack per availability zone; name matches the zone
      members: 1  # Number of ScyllaDB nodes in this rack
      scyllaConfig: scylladb-config  # References a ConfigMap with custom scylla.yaml settings
      storage:
        capacity: 100Gi  # Disk space per node; use at least 10Gi for dev, 500Gi+ for production
        storageClassName: scylladb-local-xfs
      resources:
        requests:  # requests must equal limits for CPU pinning (Guaranteed QoS)
          cpu: 10m
          memory: 100Mi
        limits:
          cpu: 1
          memory: 1Gi
      placement:
        nodeAffinity:  # Schedules ScyllaDB pods only on dedicated ScyllaDB nodes
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
            - matchExpressions:
              - key: topology.kubernetes.io/zone
                operator: In
                values:
                - eu-central-1a
              - key: scylla.scylladb.com/node-type
                operator: In
                values:
                - scylla
        tolerations:  # Allows ScyllaDB pods to run on tainted dedicated nodes
        - key: scylla-operator.scylladb.com/dedicated
          operator: Equal
          value: scyllaclusters
          effect: NoSchedule
    - name: eu-central-1b  # One rack per availability zone; name matches the zone
      members: 1  # Number of ScyllaDB nodes in this rack
      scyllaConfig: scylladb-config  # References a ConfigMap with custom scylla.yaml settings
      storage:
        capacity: 100Gi  # Disk space per node; use at least 10Gi for dev, 500Gi+ for production
        storageClassName: scylladb-local-xfs
      resources:
        requests:  # requests must equal limits for CPU pinning (Guaranteed QoS)
          cpu: 10m
          memory: 100Mi
        limits:
          cpu: 1
          memory: 1Gi
      placement:
        nodeAffinity:  # Schedules ScyllaDB pods only on dedicated ScyllaDB nodes
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
            - matchExpressions:
              - key: topology.kubernetes.io/zone
                operator: In
                values:
                - eu-central-1b
              - key: scylla.scylladb.com/node-type
                operator: In
                values:
                - scylla
        tolerations:  # Allows ScyllaDB pods to run on tainted dedicated nodes
        - key: scylla-operator.scylladb.com/dedicated
          operator: Equal
          value: scyllaclusters
          effect: NoSchedule
    - name: eu-central-1c  # One rack per availability zone; name matches the zone
      members: 1  # Number of ScyllaDB nodes in this rack
      scyllaConfig: scylladb-config  # References a ConfigMap with custom scylla.yaml settings
      storage:
        capacity: 100Gi  # Disk space per node; use at least 10Gi for dev, 500Gi+ for production
        storageClassName: scylladb-local-xfs
      resources:
        requests:  # requests must equal limits for CPU pinning (Guaranteed QoS)
          cpu: 10m
          memory: 100Mi
        limits:
          cpu: 1
          memory: 1Gi
      placement:
        nodeAffinity:  # Schedules ScyllaDB pods only on dedicated ScyllaDB nodes
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
            - matchExpressions:
              - key: topology.kubernetes.io/zone
                operator: In
                values:
                - eu-central-1c
              - key: scylla.scylladb.com/node-type
                operator: In
                values:
                - scylla
        tolerations:  # Allows ScyllaDB pods to run on tainted dedicated nodes
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
kubectl exec -it pod/scylladb-eu-central-1-eu-central-1a-0 -c scylla -- cqlsh localhost -u cassandra -p cassandra
:::

Try a few CQL commands:

:::{code-block} cql
CREATE KEYSPACE IF NOT EXISTS example WITH replication = {'class': 'NetworkTopologyStrategy', 'eu-central-1': 3};
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

Delete the EKS cluster:

:::{code-block} shell
eksctl delete cluster -f=clusterconfig.eksctl.yaml
:::

## Next steps

- Try the [GKE Quickstart](quickstart-gke.md) to deploy on Google Cloud.
- [Install](../install-operator/index.md) ScyllaDB Operator in your production environment.
- Review the [production checklist](../deploy-scylladb/production-checklist.md) before going to production.
- Learn how to [connect your application](../connect-your-app/index.md) to ScyllaDB.
