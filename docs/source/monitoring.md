# Monitoring

Scylla Operator 1.8 introduced a new API resource `ScyllaDBMonitoring`, allowing users to deploy a managed monitoring 
setup for their Scylla Clusters.

```yaml
apiVersion: scylla.scylladb.com/v1alpha1
kind: ScyllaDBMonitoring
metadata:
  name: example
spec:
  type: Platform
  endpointsSelector:
    matchLabels:
      app.kubernetes.io/name: scylla
      scylla-operator.scylladb.com/scylla-service-type: identity
      scylla/cluster: replace-with-your-scyllacluster-name
  components:
    prometheus:
      storage:
        volumeClaimTemplate:
          spec:
            resources:
              requests:
                storage: 1Gi
    grafana:
      exposeOptions:
        webInterface:
          ingress:
            ingressClassName: haproxy
            dnsDomains:
            - test-grafana.test.svc.cluster.local
            annotations:
              haproxy-ingress.github.io/ssl-passthrough: "true"
```

For details, refer to the below command:
```console
$ kubectl explain scylladbmonitorings.scylla.scylladb.com/v1alpha1
```

## Deploy managed monitoring

**Note**: as of v1.8, ScyllaDBMonitoring is experimental. The API is currently in version v1alpha1 and may change in future versions.

### Requirements

Before you can set up your ScyllaDB monitoring, you need Scylla Operator already installed in your Kubernetes cluster.
For more information on how to deploy Scylla Operator, see:
* [Deploying Scylla on a Kubernetes Cluster](generic.md)
* [Deploying Scylla stack using Helm Charts](helm.md)

The above example of the monitoring setup also makes use of HAProxy Ingress and Prometheus Operator.
You can deploy them in your Kubernetes cluster using the provided third party examples. If you already have them deployed
in your cluster, you can skip the below steps.

#### Deploy Prometheus Operator
Deploy Prometheus Operator using kubectl:
```console
$ kubectl -n prometheus-operator apply --server-side -f ./examples/third-party/prometheus-operator
```

##### Wait for Prometheus Operator to roll out
```console
$ kubectl -n prometheus-operator rollout status --timeout=5m deployments.apps/prometheus-operator
deployment "prometheus-operator" successfully rolled out
```

#### Deploy HAProxy Ingress
Deploy HAProxy Ingress using kubectl:
```console
$ kubectl -n haproxy-ingress apply --server-side -f ./examples/third-party/haproxy-ingress
```

##### Wait for HAProxy Ingress to roll out
```console
$ kubectl -n haproxy-ingress rollout status --timeout=5m deployments.apps/haproxy-ingress
deployment "haproxy-ingress" successfully rolled out
```

### Deploy ScyllaDBMonitoring

First, update the `endpointsSelector` in `examples/monitoring/v1alpha1/scylladbmonitoring.yaml` with a label
matching your ScyllaCluster instance name.

Deploy the monitoring setup using kubectl:
```console
$ kubectl -n scylla apply --server-side -f ./examples/monitoring/v1alpha1/scylladbmonitoring.yaml
```

Scylla Operator will notice the new ScyllaDBMonitoring object, and it will reconcile all necessary resources.

#### Wait for ScyllaDBMonitoring to roll out
```console
$ kubectl wait --for='condition=Progressing=False' scylladbmonitorings.scylla.scylladb.com/example
scylladbmonitoring.scylla.scylladb.com/example condition met

$ kubectl wait --for='condition=Degraded=False' scylladbmonitorings.scylla.scylladb.com/example
scylladbmonitoring.scylla.scylladb.com/example condition met

$ kubectl wait --for='condition=Available=True' scylladbmonitorings.scylla.scylladb.com/example
scylladbmonitoring.scylla.scylladb.com/example condition met
```

#### Wait for Prometheus to roll out
```console
$ kubectl rollout status --timeout=5m statefulset.apps/prometheus-example
statefulset rolling update complete 1 pods at revision prometheus-example-65b89d55bb...
```

#### Wait for Grafana to roll out
```console
$ kubectl rollout status --timeout=5m deployments.apps/example-grafana
deployment "example-grafana" successfully rolled out
```

### Accessing Grafana

For accessing Grafana service from outside the Kubernetes cluster we recommend using an Ingress, although there are many other ways to do so.
When using Ingress, what matters is to direct your packets to the ingress controller Service/Pods and have the correct TLS SNI field set by the caller when reaching out to the service, so it is routed properly, and your client can successfully validate the grafana serving certificate.
This is easier when you are using a real DNS domain that resolves to your Ingress controller's IP address but most clients and tools allow setting the SNI field manually.

### Prerequisites

To access Grafana, you first need to collect the serving CA and the credentials.

```console
$ GRAFANA_SERVING_CERT="$( kubectl -n scylla get secret/example-grafana-serving-ca --template '{{ index .data "tls.crt" }}' | base64 -d )"
$ GRAFANA_USER="$( kubectl -n scylla get secret/example-grafana-admin-credentials --template '{{ index .data "username" }}' | base64 -d )"
$ GRAFANA_PASSWORD="$( kubectl -n scylla get secret/example-grafana-admin-credentials --template '{{ index .data "password" }}' | base64 -d )"
```

### Connecting through Ingress using a resolvable domain

In production clusters, the Ingress controller and appropriate DNS records should be set up already. Often there is already a generic wildcard record like `*.app.mydomain` pointing to the Ingress controller's external IP. For custom service domains, it is usually a CNAME pointing to the Ingress controller's A record.

Note: The ScyllaDBMonitoring example creates an Ingress object with `test-grafana.test.svc.cluster.local` DNS domain that you should adjust to your domain. Below examples use `example-grafana.apps.mydomain`.

Note: To test a resolvable domain from your machine without creating DNS records, you can adjust `/etc/hosts` or similar.

```console
$ curl --fail -s -o /dev/null -w '%{http_code}' -L --cacert <( echo "${GRAFANA_SERVING_CERT}" ) "https://example-grafana.apps.mydomain" --user "${GRAFANA_USER}:${GRAFANA_PASSWORD}"
200
```

### Connecting through Ingress using an unresolvable domain

To connect to an Ingress without a resolvable domain you first need to find out your Ingress controller's IP that can be resolved externally. Again, there are many ways to do so beyond the below examples.

Unless stated otherwise, we assume your Ingress is running on port 443.

```console
$ INGRESS_PORT=443
```

#### Variants

##### Ingress ExternalIP

When you are running in a real cluster there is usually a cloud LoadBalancer or a bare metal alternative providing you with an externally reachable IP address.

```console
$ INGRESS_IP="$( kubectl -n=haproxy-ingress get service/haproxy-ingress --template='{{ ( index .status.loadBalancer.ingress 0 ).ip }}' )"
```

##### Ingress NodePort

NodePort is slightly less convenient, but it's available in development clusters as well.

```console
$ INGRESS_IP="$( kubectl get nodes --template='{{ $internal_ip := "" }}{{ $external_ip := "" }}{{ range ( index .items 0 ).status.addresses }}{{ if eq .type "InternalIP" }}{{ $internal_ip = .address }}{{ else if eq .type "ExternalIP" }}{{ $external_ip = .address }}{{ end }}{{ end }}{{ if $external_ip }}{{ $external_ip }}{{ else }}{{ $internal_ip }}{{ end }}' )"
$ INGRESS_PORT="$( kubectl -n=haproxy-ingress get services/haproxy-ingress --template='{{ range .spec.ports }}{{ if eq .port 443 }}{{ .nodePort }}{{ end }}{{ end }}' )"
```

##### Connection

```console
$ curl --fail -s -o /dev/null -w '%{http_code}' -L --cacert <( echo "${GRAFANA_SERVING_CERT}" ) "https://test-grafana.test.svc.cluster.local:${INGRESS_PORT}" --resolve "test-grafana.test.svc.cluster.local:${INGRESS_PORT}:${INGRESS_IP}" --user "${GRAFANA_USER}:${GRAFANA_PASSWORD}"
200
```
