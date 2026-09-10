# Setting up ScyllaDB Monitoring

This guide will walk you through setting up a complete monitoring stack for your ScyllaDB clusters using the
[`ScyllaDBMonitoring`](https://operator.docs.scylladb.com/v1.19/reference/api/groups/scylla.scylladb.com/scylladbmonitorings.md) custom resource and an external `Prometheus` instance.

The guide assumes you have read the [overview](https://operator.docs.scylladb.com/v1.19/management/monitoring/overview.md) of ScyllaDB monitoring and are familiar with the concepts of Prometheus and Grafana.
It doesn’t cover every possible configuration option, but it will highlight the most important ones. For a complete reference of all configuration options,
see the [`ScyllaDBMonitoring` API reference](https://operator.docs.scylladb.com/v1.19/reference/api/groups/scylla.scylladb.com/scylladbmonitorings.md).

## Requirements

Before you can set up your ScyllaDB monitoring, you need Scylla Operator (along with the Prometheus Operator) and a `ScyllaCluster`
already installed in your Kubernetes cluster. For more information on how to deploy Scylla Operator, see [the installation guide](https://operator.docs.scylladb.com/v1.19/installation/overview.md).

## Deploy external Prometheus

#### NOTE
In this guide, we will deploy an external `Prometheus` instance that will be managed outside of `ScyllaDBMonitoring` by the Prometheus Operator.
If you already have an existing `Prometheus` instance in your cluster that you want to use, you can skip this step.

### Create ServiceAccount for Prometheus

Prometheus needs a `ServiceAccount` to operate. You can create it using the following manifest:

```shell
kubectl create serviceaccount prometheus -n scylla
```

### Create ClusterRole and ClusterRoleBinding for Prometheus

Prometheus needs certain permissions to operate. You can create a `ClusterRole` and a `ClusterRoleBinding` that will bind
the role to the `ServiceAccount` created above using the following manifests:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: prometheus
rules:
  - apiGroups: [""]
    resources:
      - nodes
      - nodes/metrics
      - services
      - endpoints
      - pods
    verbs: ["get", "list", "watch"]
  - apiGroups: [""]
    resources:
      - configmaps
    verbs: ["get"]
  - apiGroups:
      - discovery.k8s.io
    resources:
      - endpointslices
    verbs: ["get", "list", "watch"]
  - apiGroups:
      - networking.k8s.io
    resources:
      - ingresses
    verbs: ["get", "list", "watch"]
  - nonResourceURLs: ["/metrics"]
    verbs: ["get"]
```

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: prometheus
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: prometheus
subjects:
  - kind: ServiceAccount
    name: prometheus
    namespace: scylla
```

Apply them using the following commands:

```shell
kubectl apply -n scylla --server-side -f=https://raw.githubusercontent.com/scylladb/scylla-operator/v1.19/examples/monitoring/v1alpha1/prometheus.clusterrole.yaml
kubectl apply -n scylla --server-side -f=https://raw.githubusercontent.com/scylladb/scylla-operator/v1.19/examples/monitoring/v1alpha1/prometheus.clusterrolebinding.yaml
```

### Create Service for Prometheus

Prometheus needs a `Service` to be accessible within the cluster. You can create it using the following manifest:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: prometheus
spec:
  type: ClusterIP
  selector:
      app.kubernetes.io/name: prometheus
      app.kubernetes.io/instance: prometheus
      prometheus: prometheus
  ports:
    - name: web
      protocol: TCP
      port: 9090
```

Apply it using the following command:

```shell
kubectl apply -n scylla --server-side -f=https://raw.githubusercontent.com/scylladb/scylla-operator/v1.19/examples/monitoring/v1alpha1/prometheus.service.yaml
```

### Deploy Prometheus instance

When deploying a `Prometheus`, you need to ensure `serviceAccountName` and `serviceName` are set correctly to the values created above:

```yaml
spec:
  serviceAccountName: "prometheus"
  serviceName: "prometheus"
```

You also need to ensure that your `Prometheus` instance is configured to discover and scrape the
`ServiceMonitor` and `PrometheusRule` resources created by `ScyllaDBMonitoring`. This requires setting the
`serviceMonitorSelector` and `ruleSelector` in the `Prometheus` resource to match the labels used by `ScyllaDBMonitoring`, like so:

```yaml
serviceMonitorSelector:
  matchLabels:
    scylla-operator.scylladb.com/scylladbmonitoring-name: "example"
ruleSelector:
  matchLabels:
    scylla-operator.scylladb.com/scylladbmonitoring-name: "example"
```

#### Monitoring multiple ScyllaClusters with a single Prometheus

If you have more than one `ScyllaCluster` in your Kubernetes cluster, and you want to monitor all of them using the same `Prometheus` instance,
you can customize the selectors to match `ServiceMonitors` and `PrometheusRules` created for multiple `ScyllaClusters`.

Assuming you’re going to create two `ScyllaDBMonitoring` objects named `cluster-1-monitoring` and `cluster-2-monitoring`
for two different `ScyllaClusters` in two distinct namespaces, you can set the selectors like this so that the `Prometheus` instance
will scrape both of them:

```yaml
serviceMonitorNamespaceSelector: {} # Select ServiceMonitors in all namespaces.
serviceMonitorSelector: # Select ServiceMonitors created by multiple ScyllaDBMonitoring instances.
  matchExpressions:
    - key: scylla-operator.scylladb.com/scylladbmonitoring-name
      operator: In
      values:
        - "cluster-1-monitoring"
        - "cluster-2-monitoring"
ruleNamespaceSelector: {} # Select Rules in all namespaces.
ruleSelector: # Select Rules created by multiple ScyllaDBMonitoring instances.
  matchExpressions:
    - key: scylla-operator.scylladb.com/scylladbmonitoring-name
      operator: In
      values:
        - "cluster-1-monitoring"
        - "cluster-2-monitoring"
```

In this case, we’re setting `serviceMonitorNamespaceSelector` and `ruleNamespaceSelector` to an empty selector, which means that `Prometheus`
will look for `ServiceMonitors` and `PrometheusRules` in all namespaces.

#### WARNING
If you want to monitor multiple `ScyllaClusters` using a single `Prometheus` instance, ensure all selected `ScyllaClusters`
have unique names across the entire Kubernetes cluster (not just within their namespaces).
Our monitoring setup cannot currently distinguish metrics collected from `ScyllaClusters` with colliding names, even if
they are in different namespaces.

Please refer to the [Prometheus Operator documentation](https://prometheus-operator.dev/docs/api-reference/api/#monitoring.coreos.com/v1.PrometheusSpec)
for more details on how to configure selectors in the `Prometheus` specification.

#### Deploy Prometheus

You can deploy a `Prometheus` instance by executing the following command:

```shell
kubectl apply -n scylla --server-side -f=https://raw.githubusercontent.com/scylladb/scylla-operator/v1.19/examples/monitoring/v1alpha1/prometheus.yaml
```

#### Wait for Prometheus to roll out

```console
kubectl -n scylla rollout status --timeout=5m statefulset.apps/prometheus-prometheus
```

## Configure ScyllaDBMonitoring

To set up the monitoring stack, you, finally, need to create a `ScyllaDBMonitoring` object.

#### NOTE
If you want to customize the `ScyllaDBMonitoring` configuration, we will explain the important configuration options in the following sections.

### Endpoints selector

```yaml
endpointsSelector:
  matchLabels:
    app.kubernetes.io/name: scylla
    scylla-operator.scylladb.com/scylla-service-type: member
    scylla/cluster: scylla # Replace with your ScyllaCluster name.
```

This field is used to select the ScyllaDB nodes that will be monitored. In `matchLabels` you should specify the labels that match the ScyllaDB
Services’ labels that are added by Scylla Operator:

- `scylla-operator.scylladb.com/scylla-service-type: member` - This is a static label that’s always added to each ScyllaDB node Service.
- `scylla/cluster` - This label’s value depends on your ScyllaCluster name. Replace it with the actual name of your `ScyllaCluster`.

### Prometheus configuration

By default, `ScyllaDBMonitoring` deploys a `Prometheus` instance for you (Managed mode). However, this mode is deprecated,
and we recommend using an existing Prometheus instance instead (External mode). External mode is used in the example manifest.

```yaml
components:
  prometheus:
    mode: External
```

### Grafana configuration

You can configure various aspects of the Grafana instance that will be deployed for you under `spec.components.grafana`). You can set resource requests
and limits (under `resources`), persistency (under `storage`), workload placement (under `placement`), Grafana authentication
(under `authentication`), and serving certificate (under `servingCertSecretName`).

If you use the `External` Prometheus mode, you should configure a datasource (under `datasources`) that points to your existing Prometheus instance.
In the datasource configuration, you should use the internal Kubernetes `Service` name of your Prometheus instance as the URL (`http://prometheus.scylla.svc.cluster.local:9090` in this example).
Depending on your Prometheus configuration, you may also need to configure TLS (under `tls`) and authentication (under `auth`).

```yaml
datasources:
  - name: prometheus
    type: Prometheus
    url: http://prometheus.scylla.svc.cluster.local:9090
    prometheusOptions: {}
```

## Deploy ScyllaDBMonitoring

Deploy the monitoring using kubectl:

```shell
kubectl apply -n scylla --server-side -f=https://raw.githubusercontent.com/scylladb/scylla-operator/v1.19/examples/monitoring/v1alpha1/scylladbmonitoring.yaml
```

ScyllaDB Operator will notice the new `ScyllaDBMonitoring` object, and it will reconcile all necessary resources.

### Wait for ScyllaDBMonitoring to roll out

```console
kubectl wait -n scylla --timeout=5m --for='condition=Available=True' scylladbmonitorings.scylla.scylladb.com/example
```

#### NOTE
`ScyllaDBMonitoring`’s `Available` condition will be set to `True` when all managed resources are created. However, it doesn’t
guarantee that all resources are fully rolled out. You should still wait for Grafana to be fully rolled out
before accessing it.

### Wait for Grafana to roll out

```console
kubectl rollout status -n scylla --timeout=5m deployments.apps/example-grafana
```

At this point, you should have a fully functional monitoring stack for your ScyllaDB cluster.

To learn how to access Grafana, see the [Exposing Grafana](https://operator.docs.scylladb.com/v1.19/management/monitoring/exposing-grafana.md) guide.
