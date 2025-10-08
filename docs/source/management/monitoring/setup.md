# ScyllaDB Monitoring setup

This guide will walk you through setting up a complete monitoring stack for your ScyllaDB clusters using the
[ScyllaDBMonitoring](../../api-reference/groups/scylla.scylladb.com/scylladbmonitorings.rst) custom resource.

## Requirements

Before you can set up your ScyllaDB monitoring, you need {{productName}} already installed in your Kubernetes cluster.
For more information on how to deploy {{productName}}, see [the installation guide](../../installation/overview.md).

ScyllaDBMonitoring relies on the Prometheus Operator for managing Prometheus-related resources.
You can deploy it in your Kubernetes cluster using the provided third-party examples. If you already have it deployed
in your cluster, you can skip the below steps.

### Deploy Prometheus Operator

Deploy Prometheus Operator using kubectl:

:::{code-block} shell
:substitutions:
kubectl apply -n prometheus-operator --server-side -f=https://raw.githubusercontent.com/{{repository}}/{{revision}}/examples/third-party/prometheus-operator
:::

#### Wait for Prometheus Operator to roll out

```console
kubectl -n prometheus-operator rollout status --timeout=5m deployments.apps/prometheus-operator
```

## Deploy ScyllaDBMonitoring

You can see an example of a complete ScyllaDBMonitoring in this {{'[example manifest](https://raw.githubusercontent.com/{}/{}/examples/monitoring/v1alpha1/scylladbmonitoring.yaml)'.format(repository, revision)}}. 

Make a local copy of it and update the `endpointsSelector` in the example with a label matching your ScyllaCluster instance name.

Deploy the monitoring setup using kubectl:
```console
kubectl apply -n scylla --server-side -f=./scylladbmonitoring.yaml
```

Scylla Operator will notice the new ScyllaDBMonitoring object, and it will reconcile all necessary resources.

### Wait for ScyllaDBMonitoring to roll out
```console
kubectl wait --for='condition=Progressing=False' scylladbmonitorings.scylla.scylladb.com/example
kubectl wait --for='condition=Degraded=False' scylladbmonitorings.scylla.scylladb.com/example
kubectl wait --for='condition=Available=True' scylladbmonitorings.scylla.scylladb.com/example
```

### Wait for Prometheus to roll out
```console
kubectl rollout status --timeout=5m statefulset.apps/prometheus-example
```

### Wait for Grafana to roll out
```console
kubectl rollout status --timeout=5m deployments.apps/example-grafana
```

At this point, you should have a fully functional monitoring stack for your ScyllaDB cluster.

To learn how to access Grafana, see the [Exposing Grafana](exposing-grafana.md) guide.
