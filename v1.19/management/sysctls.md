# Configuring kernel parameters (sysctls)

This guide explains how to configure Linux kernel parameters (sysctls) for nodes running production-grade ScyllaDB deployments.

Scylla Operator enables you to manage sysctl settings in a Kubernetes-native way using the NodeConfig resource.

#### NOTE
To configure sysctls for multi-datacenter ScyllaDB deployments, simply perform the steps outlined in this guide in each worker cluster.

## Configuring the recommended sysctls for ScyllaDB nodes

The below NodeConfig manifest configures the sysctls recommended for production-grade ScyllaDB deployments. Use the placement criteria to target the nodes used for running ScyllaDB workloads in your cluster.

```yaml
apiVersion: scylla.scylladb.com/v1alpha1
kind: NodeConfig
metadata:
  name: scylladb-nodepool-1
spec:
  sysctls:
  - name: fs.aio-max-nr
    value: "30000000"
  - name: fs.file-max
    value: "9223372036854775807"
  - name: fs.nr_open
    value: "1073741816"
  - name: fs.inotify.max_user_instances
    value: "1200"
  - name: vm.swappiness
    value: "1"
  - name: vm.vfs_cache_pressure
    value: "2000"
  placement:
    nodeSelector:
      scylla.scylladb.com/node-type: scylla
    tolerations:
    - effect: NoSchedule
      key: scylla-operator.scylladb.com/dedicated
      operator: Equal
      value: scyllaclusters
```

### Configuring sysctls via GitOps (kubectl)

To configure kernel parameters on selected nodes in your cluster, simply apply the NodeConfig manifest.

To apply the above example manifest with `kubectl`, run the following command:

```console
kubectl apply --server-side -f=https://raw.githubusercontent.com/scylladb/scylla-operator/v1.19/examples/common/configure-sysctls.nodeconfig.yaml
```

After applying the manifest, wait for the NodeConfig to be reconciled.

```bash
kubectl wait --timeout=10m --for='condition=Progressing=False' nodeconfigs.scylla.scylladb.com/scylladb-nodepool-1
kubectl wait --timeout=10m --for='condition=Degraded=False' nodeconfigs.scylla.scylladb.com/scylladb-nodepool-1
kubectl wait --timeout=10m --for='condition=Available=True' nodeconfigs.scylla.scylladb.com/scylladb-nodepool-1
```

### Configuring sysctls via Helm

We do not currently provide Helm charts for NodeConfig configuration.
In case you installed Scylla Operator using Helm, you still need use the GitOps (kubectl) instructions for NodeConfig setup.
