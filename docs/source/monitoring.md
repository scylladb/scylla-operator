### Setting up Monitoring

Both Prometheus, Grafana and AlertManager were configured with specific rules for Scylla Monitoring.
All of them will be available under the `scylla-monitoring` namespace.
Customization can be done in `examples/common/monitoring/values.yaml`


1. Download Scylla Monitoring

   First you need to download Scylla Monitoring, which contains Grafana dashboards and custom Prometheus rules.
   You can do this by running the following command:
   ```
   mkdir scylla-monitoring
   curl -L https://github.com/scylladb/scylla-monitoring/tarball/branch-4.0 | tar -xzf - -C scylla-monitoring --strip-components=1
   ```

1. Add monitoring stack charts repository

   ```
   helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
   helm repo update
   ```

1. Install monitoring stack

   ```
   helm install monitoring prometheus-community/kube-prometheus-stack --create-namespace --namespace scylla-monitoring -f examples/common/monitoring/values.yaml -f <( cd scylla-monitoring/prometheus/prom_rules && find * -maxdepth 1 -type f -iregex '.*\.\(yaml\|yml\)' -print0 | xargs -0 yq ea '{(filename | sub("\/", "-")): .} | . as $item ireduce ({}; . * $item) | {"additionalPrometheusRulesMap": .}' )
   ```

   If you want to tweak the Prometheus properties, for example it's assigned memory,
   you can override it by adding a command line argument like this: `--set prometheus.resources.limits.memory=4Gi`
   or edit values file located at `examples/common/monitoring/values.yaml`.

   The `yq` command prepares and formats custom Prometheus rules included in Scylla Monitoring and passes them as
   additional values to `helm install` command.
   It results in creating additional PrometheusRules: custom resources used to mount the rules into Prometheus.

1. Install Service Monitors

    ServiceMonitors are used by the Prometheus to discover applications exposing metrics.

    ```
    # Scylla Service Monitor
    kubectl apply -f examples/common/monitoring/scylla-service-monitor.yaml

    # Scylla Manager Service Monitor
    kubectl apply -f examples/common/monitoring/scylla-manager-service-monitor.yaml
    ```

1. Install dashboards

    Scylla Monitoring comes with pre generated dashboards suitable for multiple Scylla versions.
    In this example we will use dashboards for Scylla 4.6, and Scylla Manager 3.0.
    Amend directory path to generated dashboards to version suitable for your deployment.

   Now the dashboards can be created like this:
   ```
   # Scylla dashboards
   for f in scylla-monitoring/grafana/build/ver_4.6/*.json; do
     kubectl -n scylla-monitoring create configmap scylla-dashboard-"$( basename "${f}" '.json' )" --from-file="${f}" --dry-run=client -o yaml | kubectl label -f- --dry-run=client -o yaml --local grafana_dashboard=1 | kubectl apply --server-side -f-
   done

   # Scylla Manager dashboards
   for f in scylla-monitoring/grafana/build/manager_3.0/*.json; do
     kubectl -n scylla-monitoring create configmap scylla-manager-dashboard-"$( basename "${f}" '.json' )" --from-file="${f}" --dry-run=client -o yaml | kubectl label -f- --dry-run=client -o yaml --local grafana_dashboard=1 | kubectl apply --server-side -f-
   done
    ```

    Once Grafana sidecar picks up these dashboards they should be accessible in Grafana.

To access Grafana locally, run:
    ```
    kubectl -n scylla-monitoring port-forward deployment.apps/monitoring-grafana 3000
    ```

   You can find it on `http://127.0.0.1:3000` and login with the credentials `admin`:`admin`.