# Setting up ScyllaDB Monitoring

This guide will walk you through setting up a complete monitoring stack for your ScyllaDB clusters using the
[`ScyllaDBMonitoring`](../../reference/api/groups/scylla.scylladb.com/scylladbmonitorings.rst) custom resource and an external `Prometheus` instance.

The guide assumes you have read the [overview](overview.md) of ScyllaDB monitoring and are familiar with the concepts of Prometheus and Grafana.
It doesn't cover every possible configuration option, but it will highlight the most important ones. For a complete reference of all configuration options, 
see the [`ScyllaDBMonitoring` API reference](../../reference/api/groups/scylla.scylladb.com/scylladbmonitorings.rst).

## Requirements

Before you can set up your ScyllaDB monitoring, you need ScyllaDB Operator (along with the Prometheus Operator) and a `ScyllaCluster`
already installed in your Kubernetes cluster. For more information on how to deploy ScyllaDB Operator, see [the installation guide](../../installation/overview.md).

## Deploy external Prometheus

:::{note}
In this guide, we will deploy an external `Prometheus` instance that will be managed outside of `ScyllaDBMonitoring` by the Prometheus Operator.
If you already have an existing `Prometheus` instance in your cluster that you want to use, you can skip this step.
:::

### Create ServiceAccount for Prometheus

Prometheus needs a `ServiceAccount` to operate. You can create it using the following manifest:

```shell
kubectl create serviceaccount prometheus -n scylla
```

### Create ClusterRole and ClusterRoleBinding for Prometheus

Prometheus needs certain permissions to operate. You can create a `ClusterRole` and a `ClusterRoleBinding` that will bind
the role to the `ServiceAccount` created above using the following manifests:

:::{literalinclude} ../../../../examples/monitoring/v1alpha1/prometheus.clusterrole.yaml
:language: yaml
:::

:::{literalinclude} ../../../../examples/monitoring/v1alpha1/prometheus.clusterrolebinding.yaml
:language: yaml
:::

Apply them using the following commands:

:::{code-block} shell
:substitutions:
kubectl apply -n scylla --server-side -f=https://raw.githubusercontent.com/{{repository}}/{{revision}}/examples/monitoring/v1alpha1/prometheus.clusterrole.yaml
kubectl apply -n scylla --server-side -f=https://raw.githubusercontent.com/{{repository}}/{{revision}}/examples/monitoring/v1alpha1/prometheus.clusterrolebinding.yaml
:::

### Create Service for Prometheus

Prometheus needs a `Service` to be accessible within the cluster. You can create it using the following manifest:

:::{literalinclude} ../../../../examples/monitoring/v1alpha1/prometheus.service.yaml
:language: yaml
:::

Apply it using the following command:

:::{code-block} shell
:substitutions:
kubectl apply -n scylla --server-side -f=https://raw.githubusercontent.com/{{repository}}/{{revision}}/examples/monitoring/v1alpha1/prometheus.service.yaml
:::

### Deploy Prometheus instance

When deploying a `Prometheus`, you need to ensure `serviceAccountName` and `serviceName` are set correctly to the values created above:

:::{literalinclude} ../../../../examples/monitoring/v1alpha1/prometheus.yaml
:language: yaml
:dedent:
:start-after: "# [names]"
:end-before: "# [/names]"
:::

You also need to ensure that your `Prometheus` instance is configured to discover and scrape the
`ServiceMonitor` and `PrometheusRule` resources created by `ScyllaDBMonitoring`. This requires setting the
`serviceMonitorSelector` and `ruleSelector` in the `Prometheus` resource to match the labels used by `ScyllaDBMonitoring`, like so:

:::{literalinclude} ../../../../examples/monitoring/v1alpha1/prometheus.yaml
:language: yaml
:dedent:
:start-after: "# [selectors]"
:end-before: "# [/selectors]"
:::

#### Monitoring multiple ScyllaClusters with a single Prometheus

If you have more than one `ScyllaCluster` in your Kubernetes cluster, and you want to monitor all of them using the same `Prometheus` instance,
you can customize the selectors to match `ServiceMonitors` and `PrometheusRules` created for multiple `ScyllaClusters`.

Assuming you're going to create two `ScyllaDBMonitoring` objects named `cluster-1-monitoring` and `cluster-2-monitoring`
for two different `ScyllaClusters` in two distinct namespaces, you can set the selectors like this so that the `Prometheus` instance
will scrape both of them:

:::{literalinclude} ../../../../examples/monitoring/v1alpha1/multi-cluster.prometheus.yaml
:language: yaml
:dedent:
:start-after: "# [selectors]"
:end-before: "# [/selectors]"
:::

In this case, we're setting `serviceMonitorNamespaceSelector` and `ruleNamespaceSelector` to an empty selector, which means that `Prometheus`
will look for `ServiceMonitors` and `PrometheusRules` in all namespaces.

:::{warning}
If you want to monitor multiple `ScyllaClusters` using a single `Prometheus` instance, ensure all selected `ScyllaClusters`
have unique names across the entire Kubernetes cluster (not just within their namespaces).
Our monitoring setup cannot currently distinguish metrics collected from `ScyllaClusters` with colliding names, even if
they are in different namespaces.
:::

Please refer to the [Prometheus Operator documentation](https://prometheus-operator.dev/docs/api-reference/api/#monitoring.coreos.com/v1.PrometheusSpec)
for more details on how to configure selectors in the `Prometheus` specification.

#### Deploy Prometheus

You can deploy a `Prometheus` instance by executing the following command:

:::{code-block} shell
:substitutions:
kubectl apply -n scylla --server-side -f=https://raw.githubusercontent.com/{{repository}}/{{revision}}/examples/monitoring/v1alpha1/prometheus.yaml
:::

#### Wait for Prometheus to roll out

```console
kubectl -n scylla rollout status --timeout=5m statefulset.apps/prometheus-prometheus
```

## Configure ScyllaDBMonitoring

To set up the monitoring stack, you, finally, need to create a `ScyllaDBMonitoring` object.

:::{note}
If you want to customize the `ScyllaDBMonitoring` configuration, we will explain the important configuration options in the following sections.
:::

### Endpoints selector

:::{literalinclude} ../../../../examples/monitoring/v1alpha1/scylladbmonitoring.yaml
:language: yaml
:dedent:
:start-after: "# [endpointsSelector]"
:end-before: "# [/endpointsSelector]"
:::

This field is used to select the ScyllaDB nodes that will be monitored. In `matchLabels` you should specify the labels that match the ScyllaDB
Services' labels that are added by ScyllaDB Operator:

- `scylla-operator.scylladb.com/scylla-service-type: member` - This is a static label that's always added to each ScyllaDB node Service.
- `scylla/cluster` - This label's value depends on your ScyllaCluster name. Replace it with the actual name of your `ScyllaCluster`.

### Prometheus configuration

By default, `ScyllaDBMonitoring` deploys a `Prometheus` instance for you (Managed mode). However, this mode is deprecated,
and we recommend using an existing Prometheus instance instead (External mode). External mode is used in the example manifest.

:::{literalinclude} ../../../../examples/monitoring/v1alpha1/scylladbmonitoring.yaml
:language: yaml
:dedent:
:start-after: "# [prometheusMode]"
:end-before: "# [/prometheusMode]"
:::

### Grafana configuration

You can configure various aspects of the Grafana instance that will be deployed for you under `spec.components.grafana`). You can set resource requests
and limits (under `resources`), persistency (under `storage`), workload placement (under `placement`), Grafana authentication
(under `authentication`), and serving certificate (under `servingCertSecretName`).

If you use the `External` Prometheus mode, you should configure a datasource (under `datasources`) that points to your existing Prometheus instance.
In the datasource configuration, you should use the internal Kubernetes `Service` name of your Prometheus instance as the URL (`http://prometheus.scylla.svc.cluster.local:9090` in this example).
Depending on your Prometheus configuration, you may also need to configure TLS (under `tls`) and authentication (under `auth`).

:::{literalinclude} ../../../../examples/monitoring/v1alpha1/scylladbmonitoring.yaml
:language: yaml
:dedent:
:start-after: "# [grafanaDatasources]"
:end-before: "# [/grafanaDatasources]"
:::

## Deploy ScyllaDBMonitoring

Deploy the monitoring using kubectl:

:::{code-block} shell
:substitutions:
kubectl apply -n scylla --server-side -f=https://raw.githubusercontent.com/{{repository}}/{{revision}}/examples/monitoring/v1alpha1/scylladbmonitoring.yaml
:::

ScyllaDB Operator will notice the new `ScyllaDBMonitoring` object, and it will reconcile all necessary resources.

### Wait for ScyllaDBMonitoring to roll out
```console
kubectl wait -n scylla --timeout=5m --for='condition=Available=True' scylladbmonitorings.scylla.scylladb.com/example
```

:::{note}
`ScyllaDBMonitoring`'s `Available` condition will be set to `True` when all managed resources are created. However, it doesn't
guarantee that all resources are fully rolled out. You should still wait for Grafana to be fully rolled out
before accessing it.
:::

### Wait for Grafana to roll out
```console
kubectl rollout status -n scylla --timeout=5m deployments.apps/example-grafana
```

At this point, you should have a fully functional monitoring stack for your ScyllaDB cluster.

To learn how to access Grafana, see the [Exposing Grafana](exposing-grafana.md) guide.
