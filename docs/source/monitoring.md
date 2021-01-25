### Setting up Monitoring

Both Prometheus, Grafana and AlertManager were configured with specific rules for Scylla Monitoring.
All of them will be available under the `scylla-monitoring` namespace.
Customization can be done in `examples/common/monitoring/values.yaml`

1. Add monitoring stack charts repository
   ```
   helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
   helm repo update
   ```
1. Install monitoring stack
    ```
    helm install monitoring prometheus-community/kube-prometheus-stack --values examples/common/monitoring/values.yaml --create-namespace --namespace scylla-monitoring
    ```
   If you want to tweak the prometheus properties, for example it's assigned memory,
   you can override it by adding a command line argument like this: `--set prometheus.resources.limits.memory=4Gi`
   or edit values file located at `examples/common/monitoring/values.yaml`.

1. Install Service Monitors

    ServiceMonitors are used by the Prometheus to discover applications exposing metrics.

    ```
    # Scylla Service Monitor
    kubectl apply -f examples/common/monitoring/scylla-service-monitor.yaml

    # Scylla Manager Service Monitor
    kubectl apply -f examples/common/monitoring/scylla-manager-service-monitor.yaml
    ```

1. Download dashboards

   First you need to download the dashboards to make them available in Grafana.
   You can do this by running the following command:
    ```
    wget https://github.com/scylladb/scylla-monitoring/archive/scylla-monitoring-3.6.0.tar.gz
    tar -xvf scylla-monitoring-3.6.0.tar.gz
    ```

1. Install dashboards

    Scylla Monitoring comes with pre generated dashboards suitable for multiple Scylla versions.
    In this example we will use dashboards for Scylla 4.3, and Scylla Manager 2.2.
    Amend directory path to generated dashboards to version suitable for your deployment.

   Now the dashboards can be created like this:
    ```
    # Scylla dashboards
    kubectl -n scylla-monitoring create configmap scylla-dashboards --from-file=scylla-monitoring-scylla-monitoring-3.6.0/grafana/build/ver_4.3
    kubectl -n scylla-monitoring patch configmap scylla-dashboards  -p '{"metadata":{"labels":{"grafana_dashboard": "1"}}}'

    # Scylla Manager dashboards
    kubectl -n scylla-monitoring create configmap scylla-manager-dashboards --from-file=scylla-monitoring-scylla-monitoring-3.6.0/grafana/build/manager_2.2
    kubectl -n scylla-monitoring patch configmap scylla-manager-dashboards  -p '{"metadata":{"labels":{"grafana_dashboard": "1"}}}'
    ```

    Once Grafana sidecar picks up these dashboards they should be accessible in Grafana.

To access Grafana locally, run:
    ```
    kubectl -n scylla-monitoring port-forward deployment.apps/monitoring-grafana 3000
    ```

   You can find it on `http://127.0.0.1:3000` and login with the credentials `admin`:`admin`.
