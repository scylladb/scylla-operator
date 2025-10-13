# Setting up ScyllaDB Monitoring on OpenShift

This guide will walk you through setting up a monitoring stack for your ScyllaDB clusters using the
[`ScyllaDBMonitoring`](../../api-reference/groups/scylla.scylladb.com/scylladbmonitorings.rst) custom resource and an
external Prometheus instance that is already deployed in your Kubernetes cluster in an OpenShift cluster using [User Workload Monitoring (UWM)][user-workload-monitoring].

The guide assumes you have read the [overview](overview.md) and [setup](setup.md) of ScyllaDB monitoring and are familiar with the concepts of Prometheus and Grafana.

## Requirements

This guide assumes you have {{productName}} and a `ScyllaCluster` already installed in your OpenShift cluster.
For more information on how to deploy {{productName}}, see [the installation guide](../../installation/overview.md).

:::{note}
The {{productName}} installation process on OpenShift is the same as on vanilla Kubernetes. However, unlike Kubernetes,
OpenShift includes a built-in Prometheus Operator and [User Workload Monitoring (UWM)][user-workload-monitoring] for user
workloads. Therefore, instead of deploying Prometheus using ScyllaDBMonitoring, we configure it to use the external
Prometheus instance provided by OpenShift UWM.
:::

We also assume you have the `oc` CLI tool installed and configured to access your OpenShift cluster, and have the necessary
permissions to create `ServiceAccounts` and `ClusterRoleBindings`.

## Enable User Workload Monitoring in OpenShift

OpenShift provides a built-in Prometheus instance that can be used for monitoring user workloads.
To use this Prometheus instance, you need to enable User Workload Monitoring in your OpenShift cluster.
You can do this by following the [official OpenShift documentation][user-workload-monitoring].

## Configure OpenShift metrics access for Grafana datasource

### Create ServiceAccount and ClusterRoleBinding

To allow `ScyllaDBMonitoring`-managed Grafana to access the OpenShift User Workload Monitoring Prometheus instance,
you need to create a `ServiceAccount` and a `ClusterRoleBinding` that grants the necessary permissions. We will use its
`ServiceAccount` token for configuring Grafana datasource. See the OpenShift's [Accessing metrics as a developer](https://docs.redhat.com/en/documentation/openshift_container_platform/4.19/html/monitoring/accessing-metrics#accessing-metrics-as-a-developer) 
article for more details.

:::{note}
We assume you have `ScyllaCluster` deployed in `scylla` namespace/project. Replace `scylla` with your namespace/project name if it's different.
:::

You can create the `ServiceAccount` and `ClusterRoleBinding` using the following commands:

```shell
oc create serviceaccount scylla-grafana-monitoring-viewer
oc create clusterrolebinding scylla-monitoring-grafana-cluster-monitoring-view --clusterrole=cluster-monitoring-view --serviceaccount=scylla:scylla-grafana-monitoring-viewer
```

### Create ServiceAccount token Secret

Next, you need to create a `Secret` that contains the `ServiceAccount` token. The following manifest will create such a `Secret`:

:::{literalinclude} ../../../../examples/monitoring/v1alpha1/openshift/sa-token.secret.yaml
:language: yaml
:::

You can create it using the following command:

:::{code-block} shell
:substitutions:
kubectl apply -n scylla -f https://raw.githubusercontent.com/{{repository}}/{{revision}}/examples/monitoring/v1alpha1/openshift/sa-token.secret.yaml
:::

The `Secret` will be populated with the token automatically by Kubernetes and should be available under the `token` key of this `Secret`. Verify it by running:

```shell
kubectl -n scylla get secret scylla-monitoring-grafana-token -o=jsonpath='{.data.token}'
```

You should see the encoded token printed to the console.

## Create service CA certificate ConfigMap

OpenShift uses a self-signed CA to sign the certificates for its internal services. To allow Grafana to trust the OpenShift UWM Prometheus instance,
you need to create a `ConfigMap` that, when properly annotated, will be populated with the OpenShift service CA certificate.
Please refer to the [OpenShift documentation](https://docs.redhat.com/en/documentation/openshift_container_platform/4.20/html/security_and_compliance/certificate-types-and-descriptions#cert-types-service-ca-certificates)
for more details on this mechanism.

The following manifest will create such a `ConfigMap`:

:::{literalinclude} ../../../../examples/monitoring/v1alpha1/openshift/service-ca.configmap.yaml
:language: yaml
:::

You can create it using the following command:

:::{code-block} shell
:substitutions:
kubectl apply -n scylla -f https://raw.githubusercontent.com/{{repository}}/{{revision}}/examples/monitoring/v1alpha1/openshift/service-ca.configmap.yaml
:::

You can verify that the `ConfigMap` has been populated with the service CA certificate under the `service-ca.crt` key by running:

```shell
kubectl -n scylla get configmap example-openshift-service-ca -o=jsonpath='{.data.service-ca\.crt}'
```

You should see the PEM-encoded CA certificate printed to the console.

## Deploy ScyllaDBMonitoring

The following `ScyllaDBMonitoring` configuration will set up the monitoring stack to use the OpenShift UWM Prometheus instance
as an external Prometheus datasource for Grafana:

:::{literalinclude} ../../../../examples/monitoring/v1alpha1/openshift/uwm.scylladbmonitoring.yaml
:language: yaml
:::

You can apply it using `kubectl`:

:::{code-block} shell
:substitutions:
kubectl apply -n scylla --server-side -f=https://raw.githubusercontent.com/{{repository}}/{{revision}}/examples/monitoring/v1alpha1/openshift/uwm.scylladbmonitoring.yaml
:::

See the [Setting up ScyllaDBMonitoring](setup.md#deploy-scylladbmonitoring) guide for more details on deploying `ScyllaDBMonitoring`.

## Verify the setup

You can verify that configuration is correct by [accessing Grafana](exposing-grafana.md) and verifying you can see metrics from your ScyllaDB cluster.

[user-workload-monitoring]: https://docs.redhat.com/en/documentation/openshift_container_platform/4.20/html/monitoring/configuring-user-workload-monitoring
