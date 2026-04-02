# Set up monitoring

This page walks you through setting up monitoring for ScyllaDB clusters using `ScyllaDBMonitoring`, Prometheus, and Grafana.

## Overview

ScyllaDB exposes Prometheus metrics natively. The `ScyllaDBMonitoring` resource tells the Operator to create `ServiceMonitor` and `PrometheusRule` resources that configure Prometheus to scrape ScyllaDB metrics and evaluate alerting rules. The Operator also deploys a Grafana instance pre-loaded with ScyllaDB dashboards from [scylla-monitoring](https://github.com/scylladb/scylla-monitoring/).

## Prerequisites

- ScyllaDB Operator installed ([GitOps](../install-operator/install-with-gitops.md) or [Helm](../install-operator/install-with-helm.md))
- [Prometheus Operator](https://github.com/prometheus-operator/prometheus-operator) installed (provides `Prometheus`, `ServiceMonitor`, and `PrometheusRule` CRDs)
- A running ScyllaCluster

## Step 1: Deploy Prometheus

Create the RBAC resources and a `Prometheus` instance that will scrape ScyllaDB metrics.

### Service account and RBAC

```shell
kubectl -n scylla create serviceaccount prometheus
```

```shell
kubectl apply --server-side -f - <<EOF
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: prometheus
rules:
- apiGroups: [""]
  resources: [nodes, nodes/metrics, services, endpoints, pods]
  verbs: ["get", "list", "watch"]
- apiGroups: [""]
  resources: [configmaps]
  verbs: ["get"]
- apiGroups: [discovery.k8s.io]
  resources: [endpointslices]
  verbs: ["get", "list", "watch"]
- apiGroups: [networking.k8s.io]
  resources: [ingresses]
  verbs: ["get", "list", "watch"]
- nonResourceURLs: ["/metrics"]
  verbs: ["get"]
EOF
```

```shell
kubectl apply --server-side -f - <<EOF
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
EOF
```

### Prometheus Service

```shell
kubectl -n scylla apply --server-side -f - <<EOF
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
EOF
```

### Prometheus instance

Create a `Prometheus` resource (from Prometheus Operator) that selects `ServiceMonitor` and `PrometheusRule` resources created by `ScyllaDBMonitoring`:

```shell
kubectl -n scylla apply --server-side -f - <<EOF
apiVersion: monitoring.coreos.com/v1
kind: Prometheus
metadata:
  name: prometheus
spec:
  serviceAccountName: prometheus
  serviceName: prometheus
  version: v3.5.0
  securityContext:
    runAsNonRoot: true
    runAsUser: 65534
    fsGroup: 65534
  web:
    pageTitle: ScyllaDB Prometheus
  serviceMonitorSelector:
    matchLabels:
      scylla-operator.scylladb.com/scylladbmonitoring-name: example
  ruleSelector:
    matchLabels:
      scylla-operator.scylladb.com/scylladbmonitoring-name: example
EOF
```

:::{note}
The `serviceMonitorSelector` and `ruleSelector` labels must match the name of your `ScyllaDBMonitoring` resource (in this example, `example`). The Operator automatically labels the `ServiceMonitor` and `PrometheusRule` resources it creates with `scylla-operator.scylladb.com/scylladbmonitoring-name: <name>`.
:::

## Step 2: Create a ScyllaDBMonitoring resource

Create a `ScyllaDBMonitoring` resource that selects the ScyllaDB endpoints and points Grafana to your Prometheus instance:

```yaml
apiVersion: scylla.scylladb.com/v1alpha1
kind: ScyllaDBMonitoring
metadata:
  name: example
  namespace: scylla
spec:
  type: Platform
  endpointsSelector:
    matchLabels:
      app.kubernetes.io/name: scylla
      scylla-operator.scylladb.com/scylla-service-type: member
      scylla/cluster: scylladb
  components:
    prometheus:
      mode: External
    grafana:
      datasources:
      - name: prometheus
        type: Prometheus
        url: http://prometheus.scylla.svc.cluster.local:9090
        prometheusOptions: {}
```

:::{note}
Update the `scylla/cluster` label value in `endpointsSelector` to match your ScyllaCluster name. The `url` must point to the Prometheus Service created in Step 1.
:::

```shell
kubectl -n scylla apply --server-side -f scylladbmonitoring.yaml
```

## Step 3: Wait for deployment

```shell
kubectl -n scylla wait --for='condition=Available=True' scylladbmonitoring.scylla.scylladb.com/example
```

Verify that the Grafana pod is running:

```shell
kubectl -n scylla rollout status deployment/example-grafana
```

### Verify metrics are being scraped

Confirm that Prometheus is successfully scraping ScyllaDB metrics.

**Check ServiceMonitor status:**
```bash
kubectl -n scylla-monitoring get servicemonitor
```
Expected output shows ServiceMonitors for your cluster:
```
NAME      AGE
scylla    2m
```

**Query active Prometheus targets:**

Forward the Prometheus port and query its API:
```bash
kubectl -n scylla-monitoring port-forward svc/prometheus-operated 9090:9090 &
curl -s 'http://localhost:9090/api/v1/targets' | python3 -m json.tool | grep -A3 '"job":"scylla"'
```
Look for targets with `"health":"up"`. If targets show `"health":"down"`, check the `lastError` field for the scrape failure reason.

**Check a ScyllaDB metric:**
```bash
curl -s 'http://localhost:9090/api/v1/query?query=scylla_transport_requests_served' | python3 -m json.tool
```
A non-empty `result` array confirms metrics are flowing.

## Key fields explained

### ScyllaDBMonitoring spec

| Field | Description |
|-------|-------------|
| `type` | `Platform` (recommended) or `SaaS`. Controls how monitoring components are deployed. |
| `endpointsSelector` | Label selector that matches ScyllaDB member service endpoints. |
| `components.prometheus.mode` | `External` (recommended) — the Operator creates ServiceMonitor and PrometheusRule resources for an external Prometheus instance. |
| `components.grafana.datasources` | Prometheus datasource configuration for Grafana. Maximum 1 datasource. |

### Grafana datasource options

| Field | Description |
|-------|-------------|
| `url` | URL of the Prometheus endpoint. |
| `prometheusOptions.tls.caCertConfigMapRef` | ConfigMap name containing the Prometheus CA certificate. |
| `prometheusOptions.tls.insecureSkipVerify` | Skip TLS verification (not recommended for production). |
| `prometheusOptions.authentication.bearerToken.secretRef` | Secret name containing the bearer token for Prometheus authentication. |

## Multi-cluster monitoring

To monitor multiple ScyllaDB clusters from a single Prometheus instance, configure the Prometheus resource to select `ServiceMonitor` and `PrometheusRule` resources across namespaces:

```yaml
apiVersion: monitoring.coreos.com/v1
kind: Prometheus
metadata:
  name: prometheus
spec:
  serviceAccountName: prometheus
  serviceName: prometheus
  version: v3.5.0
  securityContext:
    runAsNonRoot: true
    runAsUser: 65534
    fsGroup: 65534
  serviceMonitorNamespaceSelector: {}
  serviceMonitorSelector:
    matchExpressions:
    - key: scylla-operator.scylladb.com/scylladbmonitoring-name
      operator: In
      values:
      - cluster-1-monitoring
      - cluster-2-monitoring
  ruleNamespaceSelector: {}
  ruleSelector:
    matchExpressions:
    - key: scylla-operator.scylladb.com/scylladbmonitoring-name
      operator: In
      values:
      - cluster-1-monitoring
      - cluster-2-monitoring
```

Setting `serviceMonitorNamespaceSelector: {}` and `ruleNamespaceSelector: {}` allows Prometheus to discover resources across all namespaces. The `matchExpressions` filter ensures only the listed `ScyllaDBMonitoring` instances are scraped.

## Multi-datacenter monitoring

For multi-DC clusters using multiple `ScyllaCluster` resources across separate Kubernetes clusters, deploy a `ScyllaDBMonitoring` resource independently in each datacenter's Kubernetes cluster. Each `ScyllaDBMonitoring` instance monitors the local `ScyllaCluster` through a local Prometheus and Grafana stack.

See [Deploy a multi-DC cluster](deploy-multi-dc-cluster.md) for the overall multi-DC deployment procedure.

## OpenShift User Workload Monitoring

On OpenShift, you can use the built-in User Workload Monitoring instead of deploying your own Prometheus instance. This requires:

1. A ServiceAccount with `cluster-monitoring-view` ClusterRole.
2. A bearer token Secret for authentication.
3. A CA certificate ConfigMap injected by OpenShift (`service.beta.openshift.io/inject-cabundle: "true"`).
4. A ScyllaDBMonitoring resource with `BearerToken` authentication and the Thanos Querier URL as datasource.

```yaml
components:
  grafana:
    datasources:
    - name: prometheus
      type: Prometheus
      url: https://thanos-querier.openshift-monitoring.svc:9091
      prometheusOptions:
        tls:
          caCertConfigMapRef: serving-certs-ca-bundle
        authentication:
          bearerToken:
            secretRef: prometheus-auth-credentials
```

## Accessing Grafana

After deployment, the Grafana dashboard is available via the `example-grafana` Service on port 3000.

For local development, use port-forwarding:

```shell
kubectl -n scylla port-forward svc/example-grafana 3000:3000
```

Retrieve the admin credentials:

```shell
kubectl -n scylla get secret/example-grafana-admin-credentials -o jsonpath='{.data.password}' | base64 -d
```

For production access via Ingress, see [Exposing Grafana](expose-grafana.md).

## Related pages

- [Exposing Grafana](expose-grafana.md) — configuring Ingress for Grafana.
- [Operator configuration](before-you-deploy/configure-operator.md) — Grafana and Prometheus image settings.
- [Production checklist](production-checklist.md) — monitoring as a production requirement.
- [Prerequisites](../install-operator/prerequisites.md) — Prometheus Operator dependency.
