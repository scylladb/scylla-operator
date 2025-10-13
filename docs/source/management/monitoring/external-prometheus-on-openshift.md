# ScyllaDB Monitoring on OpenShift

This guide will walk you through setting up monitoring stack for your ScyllaDB clusters using the
[ScyllaDBMonitoring](../../api-reference/groups/scylla.scylladb.com/scylladbmonitorings.rst) custom resource and an
external Prometheus instance that is already deployed in your Kubernetes cluster in an OpenShift cluster using User Workload Monitoring (UWM).

The guide assumes you have read the [overview](overview.md) and [setup](setup.md) of ScyllaDB monitoring and are familiar with the concepts of Prometheus and Grafana.
It will focus on the external Prometheus configuration options. For a complete reference of all configuration options,
see the [ScyllaDBMonitoring API reference](../../api-reference/groups/scylla.scylladb.com/scylladbmonitorings.rst).

## Requirements

This guide assumes you have {{productName}} and a ScyllaCluster already installed in your OpenShift cluster.
For more information on how to deploy {{productName}}, see [the installation guide](../../installation/overview.md).

We also assume you have `oc` CLI tool installed and configured to access your OpenShift cluster, and have the necessary
permissions to create ServiceAccounts and ClusterRoleBindings.

## Enable User Workload Monitoring in OpenShift

OpenShift provides a built-in Prometheus instance that can be used for monitoring user workloads.
To use this Prometheus instance, you need to enable User Workload Monitoring in your OpenShift cluster.
You can do this by following the [official OpenShift documentation](https://docs.redhat.com/en/documentation/openshift_container_platform/4.19/html/monitoring/configuring-user-workload-monitoring).

## Configure OpenShift metrics access for Grafana datasource

### Create ServiceAccount and ClusterRoleBinding

To allow ScyllaDBMonitoring-managed Grafana to access the OpenShift User Workload Monitoring Prometheus instance,
you need to create a ServiceAccount and a ClusterRoleBinding that grants the necessary permissions. We will use its
ServiceAccount token for configuring Grafana datasource. See the OpenShift's [Accessing metrics as a developer](https://docs.redhat.com/en/documentation/openshift_container_platform/4.19/html/monitoring/accessing-metrics#accessing-metrics-as-a-developer) 
article for more details.

:::{note}
We assume you have ScyllaCluster deployed in `scylla` namespace/project. Replace `scylla` with your namespace/project name if it's different.
:::

You can create the ServiceAccount and ClusterRoleBinding using the following commands:

```shell
oc create serviceaccount scylla-grafana-monitoring-viewer
oc create clusterrolebinding scylla-monitoring-grafana-cluster-monitoring-view --clusterrole=cluster-monitoring-view --serviceaccount=scylla:scylla-grafana-monitoring-viewer
```

### Create ServiceAccount token Secret

Next, you need to create a Secret that contains the ServiceAccount token. You can do this by running the following command:

```shell
echo 'apiVersion: v1
kind: Secret
metadata:
  name: scylla-monitoring-grafana-token
  namespace: scylla
  annotations:
    kubernetes.io/service-account.name: scylla-grafana-monitoring-viewer
type: kubernetes.io/service-account-token' | kubectl apply -f -
```

The Secret will be populated with the token automatically by Kubernetes and should be available under the `token` Secret key.

## Configure ScyllaDBMonitoring

You can see an example of a complete ScyllaDBMonitoring for UWM in this {{'[example manifest](https://raw.githubusercontent.com/{}/{}/examples/monitoring/v1alpha1/openshift-uwm.scylladbmonitoring.yaml)'.format(repository, revision)}}.

You can make a local copy of it (e.g., under `./scylladbmonitoring.yaml`) and modify it to fit your needs. We'll go
the options specific to OpenShift UWM below only. See the [setup](setup.md) guide for more details on other options.

### Set Prometheus mode to External

Since we are using the OpenShift User Workload Monitoring Prometheus instance, we need to set the Prometheus mode to External.

```yaml
spec:
  components:
    prometheus:
      mode: External
```

### Configure Grafana datasource

You need to configure Grafana to use the OpenShift UWM Prometheus instance as a datasource. You can do this by adding a datasource configuration
to the `spec.components.grafana.datasources` field.

```yaml
spec:
  components:
    grafana:
      datasources:
        - type: Prometheus
          url: "https://thanos-querier.openshift-monitoring.svc:9091" # OpenShift exposes UWM Prometheus via Thanos Querier.
          prometheusOptions:
            tls:
              caCertConfigMapRef:
                name: openshift-service-ca.crt                        # OpenShift provides a ConfigMap with the service CA certificate.
                key: service-ca.crt
            auth:
              type: BearerToken 
              bearerTokenOptions:
                secretRef:
                  name: scylla-monitoring-grafana-token               # The Secret we created earlier with the ServiceAccount token.
                  key: token
```

## Deploy ScyllaDBMonitoring

You can deploy the ScyllaDBMonitoring resource now. Please refer to the [setup](setup.md#deploy-scylladbmonitoring) guide for detailed instructions.

## Verify the setup

You can verify that configuration is correct by [accessing Grafana](exposing-grafana.md) and verifying you can see metrics from your ScyllaDB cluster.
