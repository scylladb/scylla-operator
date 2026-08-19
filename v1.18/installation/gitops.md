# GitOps (kubectl)

## Disclaimer

The following commands reference manifests that come from the same repository as the source code is being built from.
This means we can’t have a pinned reference to the latest release as that is a [chicken-egg problem](https://en.wikipedia.org/wiki/Chicken_or_the_egg). Therefore, we use a rolling tag (e.g., `{major}.{minor}` or `latest`) for the particular branch in our manifests.

## Installation

### Prerequisites

Scylla Operator has a few dependencies that you need to install to your cluster first.

In case you already have a supported version of each of these dependencies installed in your cluster, you can skip this part.

#### Cert Manager

```shell
kubectl apply --server-side -f=https://raw.githubusercontent.com/scylladb/scylla-operator/v1.18/examples/third-party/cert-manager.yaml
```

```shell
# Wait for CRDs to propagate to all apiservers.
kubectl wait --for condition=established --timeout=60s crd/certificates.cert-manager.io crd/issuers.cert-manager.io

# Wait for components that other steps depend on.
for deploy in cert-manager{,-cainjector,-webhook}; do
    kubectl -n=cert-manager rollout status --timeout=10m deployment.apps/"${deploy}"
done

# Wait for webhook CA secret to be created.
for i in {1..30}; do
    { kubectl -n=cert-manager get secret/cert-manager-webhook-ca && break; } || sleep 1
done
```

#### Prometheus Operator

```shell
kubectl -n=prometheus-operator apply --server-side -f=https://raw.githubusercontent.com/scylladb/scylla-operator/v1.18/examples/third-party/prometheus-operator.yaml
```

```shell
# Wait for CRDs to propagate to all apiservers.
kubectl wait --for='condition=established' crd/prometheuses.monitoring.coreos.com crd/prometheusrules.monitoring.coreos.com crd/servicemonitors.monitoring.coreos.com

# Wait for prometheus operator deployment.
kubectl -n=prometheus-operator rollout status --timeout=10m deployment.apps/prometheus-operator
```

### Scylla Operator

Once you have the dependencies installed and available in your cluster, it is the time to install Scylla Operator.

```shell
kubectl -n=scylla-operator apply --server-side -f=https://raw.githubusercontent.com/scylladb/scylla-operator/v1.18/deploy/operator.yaml
```

```shell
# Wait for CRDs to propagate to all apiservers.
kubectl wait --for='condition=established' crd/scyllaclusters.scylla.scylladb.com crd/nodeconfigs.scylla.scylladb.com crd/scyllaoperatorconfigs.scylla.scylladb.com crd/scylladbmonitorings.scylla.scylladb.com

# Wait for the components to deploy.
kubectl -n=scylla-operator rollout status --timeout=10m deployment.apps/{scylla-operator,webhook-server}
```

### Setting up local storage on nodes and enabling tuning

### GKE (NVMe)

```shell
kubectl -n=scylla-operator apply --server-side -f=https://raw.githubusercontent.com/scylladb/scylla-operator/v1.18/examples/gke/nodeconfig-alpha.yaml
```

### EKS (NVMe)

```shell
kubectl -n=scylla-operator apply --server-side -f=https://raw.githubusercontent.com/scylladb/scylla-operator/v1.18/examples/eks/nodeconfig-alpha.yaml
```

### Any platform (Loop devices)

```shell
kubectl -n=scylla-operator apply --server-side -f=https://raw.githubusercontent.com/scylladb/scylla-operator/v1.18/examples/generic/nodeconfig-alpha.yaml
```

#### NOTE
Performance tuning is enabled for all nodes that are selected by [NodeConfig](https://operator.docs.scylladb.com/v1.18/resources/nodeconfigs.md) by default, unless opted-out.

```shell
# Wait for NodeConfig to apply changes to the Kubernetes nodes.
kubectl wait --for='condition=Reconciled' --timeout=10m nodeconfigs.scylla.scylladb.com/scylladb-nodepool-1
```

### Local CSI driver

```shell
kubectl -n=local-csi-driver apply --server-side -f=https://raw.githubusercontent.com/scylladb/scylla-operator/v1.18/examples/common/local-volume-provisioner/local-csi-driver/{00_clusterrole_def,00_clusterrole_def_openshift,00_clusterrole,00_namespace,00_scylladb-local-xfs.storageclass,10_csidriver,10_serviceaccount,20_clusterrolebinding,50_daemonset}.yaml
```

```shell
# Wait for it to deploy.
kubectl -n=local-csi-driver rollout status --timeout=10m daemonset.apps/local-csi-driver
```

### ScyllaDB Manager

#### NOTE
ScyllaDB Manager is available for ScyllaDB Enterprise customers and ScyllaDB Open Source users.
With ScyllaDB Open Source, ScyllaDB Manager is limited to 5 nodes.
See the ScyllaDB Manager [Proprietary Software License Agreement](https://www.scylladb.com/scylla-manager-software-license-agreement/) for details.

### Production (sized)

```shell
kubectl -n=scylla-manager apply --server-side -f=https://raw.githubusercontent.com/scylladb/scylla-operator/v1.18/deploy/manager-prod.yaml
```

### Development (sized)

```shell
kubectl -n=scylla-manager apply --server-side -f=https://raw.githubusercontent.com/scylladb/scylla-operator/v1.18/deploy/manager-dev.yaml
```

```shell
# Wait for it to deploy.
kubectl -n=scylla-manager rollout status --timeout=10m deployment.apps/scylla-manager
```

## Next steps

Now that you’ve successfully installed Scylla Operator, it’s time to look at [how to run ScyllaDB](https://operator.docs.scylladb.com/v1.18/resources/scyllaclusters/basics.md).
