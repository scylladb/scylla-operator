# ScyllaDB Monitoring setup

This guide will walk you through setting up a complete monitoring stack for your ScyllaDB clusters using the
[ScyllaDBMonitoring](../../api-reference/groups/scylla.scylladb.com/scylladbmonitorings.rst) custom resource.

The guide assumes you have read the [overview](overview.md) of ScyllaDB monitoring and are familiar with the concepts of Prometheus and Grafana.
It doesn't cover every possible configuration option, but it will highlight the most important ones. For a complete reference of all configuration options, 
see the [ScyllaDBMonitoring API reference](../../api-reference/groups/scylla.scylladb.com/scylladbmonitorings.rst).

## Requirements

Before you can set up your ScyllaDB monitoring, you need {{productName}} and a ScyllaCluster already installed in your Kubernetes cluster.
For more information on how to deploy {{productName}}, see [the installation guide](../../installation/overview.md).

### Deploy Prometheus Operator

:::{note}
ScyllaDBMonitoring relies on the Prometheus Operator for managing Prometheus-related resources.
You can deploy it in your Kubernetes cluster using the provided third-party examples. If you already have it deployed
in your cluster, you can skip this step.
:::

Deploy Prometheus Operator using kubectl:

:::{code-block} shell
:substitutions:
kubectl apply -n prometheus-operator --server-side -f=https://raw.githubusercontent.com/{{repository}}/{{revision}}/examples/third-party/prometheus-operator
:::

#### Wait for Prometheus Operator to roll out

```console
kubectl -n prometheus-operator rollout status --timeout=5m deployments.apps/prometheus-operator
```

## Configure ScyllaDBMonitoring

You can see an example of a complete ScyllaDBMonitoring in this {{'[example manifest](https://raw.githubusercontent.com/{}/{}/examples/monitoring/v1alpha1/scylladbmonitoring.yaml)'.format(repository, revision)}}. 

You can make a local copy of it (e.g., under `./scylladbmonitoring.yaml`) and modify it to fit your needs. We'll go through the most important options below.

### Endpoints selector

This field is used to select the ScyllaDB nodes that will be monitored. In `matchLabels` you should specify the labels that match the ScyllaDB
Services' labels that are added by {{productName}}:

- `scylla-operator.scylladb.com/scylla-service-type: member` - This is a static label that's always added to each ScyllaDB node Service.
- `scylla/cluster: <scyllacluster-name>` - This label's value depends on your ScyllaCluster name. Replace `<scyllacluster-name>` with the actual name of your ScyllaCluster.

```yaml
spec:
  endpointsSelector:
    matchLabels:
      scylla-operator.scylladb.com/scylla-service-type: member # Each Scylla node Service has this label.
      scylla/cluster: <replace-with-your-scyllacluster-name>   # Replace with your ScyllaCluster name.
```

### Prometheus configuration

By default, ScyllaDBMonitoring will deploy a Prometheus instance for you (Managed mode). You can also change the mode to
External if you want to use an existing Prometheus instance in your cluster.

```yaml
spec:
  components:
    prometheus:
      mode: Managed # or External
```

:::::{tabs}
::::{group-tab} Managed mode

In Managed mode, you can configure various aspects of the Prometheus instance that will be deployed for you.
You can set resource requests and limits (under `resources`), persistency (under `storage`), and workload placement (under `placement`).

::::

::::{group-tab} External mode

In External mode, you need to ensure that your existing Prometheus instance is configured to discover and scrape the
ServiceMonitor and PrometheusRule resources created by ScyllaDBMonitoring. This requires setting the
`serviceMonitorSelector` and `ruleSelector` in the Prometheus resource to match the labels used by ScyllaDBMonitoring
(or leaving them empty to select all resources).

You also need to configure Grafana so that it can access the Prometheus instance. This will be explained in the Grafana configuration section below.
::::
:::::

### Grafana configuration

You can configure various aspects of the Grafana instance that will be deployed for you under `spec.components.grafana`). You can set resource requests
and limits (under `resources`), persistency (under `storage`), workload placement (under `placement`), Grafana authentication
(under `authentication`), and serving certificate (under `servingCertSecretName`).

:::::{tabs}
::::{group-tab} Prometheus in Managed mode

If you use `Managed` Prometheus mode, no additional configuration is needed. ScyllaDBMonitoring will automatically configure
Grafana to use the Prometheus instance as its datasource.

::::

::::{group-tab} Prometheus in External mode

If you use `External` Prometheus mode, you should configure a datasource (under `datasources`) that points to your existing Prometheus instance.
In the datasource configuration, you should use the internal Kubernetes service name of your Prometheus instance as the URL (e.g., `http://prometheus-operated.monitoring.svc.cluster.local:9090`).
Depending on your Prometheus configuration, you may also need to configure TLS (under `tls`) and authentication (under `authentication`).

::::
:::::

## Deploy ScyllaDBMonitoring

Deploy the monitoring setup using kubectl:
```console
kubectl apply -n scylla --server-side -f=./scylladbmonitoring.yaml
```

Scylla Operator will notice the new ScyllaDBMonitoring object, and it will reconcile all necessary resources.

### Wait for ScyllaDBMonitoring to roll out
```console
kubectl wait --timeout=5m --for='condition=Available=True' scylladbmonitorings.scylla.scylladb.com/example
```

:::{note}
ScyllaDBMonitoring's `Available` condition will be set to `True` when all managed resources are created. However, it doesn't
guarantee that all resources are fully rolled out. You should still wait for Prometheus and Grafana to be fully rolled out
before accessing Grafana.
:::

### Wait for Prometheus to roll out (if in Managed mode)
```console
kubectl rollout status --timeout=5m statefulset.apps/prometheus-example
```

### Wait for Grafana to roll out
```console
kubectl rollout status --timeout=5m deployments.apps/example-grafana
```

At this point, you should have a fully functional monitoring stack for your ScyllaDB cluster.

To learn how to access Grafana, see the [Exposing Grafana](exposing-grafana.md) guide.
