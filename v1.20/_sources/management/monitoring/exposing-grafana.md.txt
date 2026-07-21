# Exposing Grafana

This guide shows how to expose Grafana deployed by `ScyllaDBMonitoring` using an `Ingress` resource.

:::{note}
For accessing the Grafana service from outside the Kubernetes cluster we document using an `Ingress`, although there are other options like an `HTTPRoute` from the Gateway API.
Use whatever method fits your use case best.
:::

## Prerequisites

This assumes that you have already deployed a `ScyllaDBMonitoring` in your cluster. If you haven't done so, please follow the [ScyllaDB Monitoring setup](setup.md) guide first.

In the example below we're using the HAProxy Ingress Controller. You can deploy it in your Kubernetes cluster using the provided
third-party example. If you already have it (or another Ingress Controller) deployed in your cluster, you can skip the below steps.

### Install HAProxy Ingress

Deploy HAProxy Ingress using kubectl:
:::{code-block} shell
:substitutions:
kubectl apply -n haproxy-ingress --server-side -f=https://raw.githubusercontent.com/{{repository}}/{{revision}}/examples/third-party/haproxy-ingress.yaml
:::

Wait for HAProxy Ingress to roll out:
```console
kubectl -n haproxy-ingress rollout status --timeout=5m deployments.apps/haproxy-ingress
```

## Expose Grafana using Ingress

ScyllaDB Operator creates a `ClusterIP` Service named `<scyllaDBMonitoringName>-grafana` for each `ScyllaDBMonitoring`.
Grafana serves TLS using a self-signed certificate that's signed by a CA stored in a Secret named `<scyllaDBMonitoringName>-grafana-serving-ca` by default. 

:::{note}
You can use your own serving certificate by setting `ScyllaDBMonitoring`'s `spec.components.grafana.servingCertSecretName` field.
:::

Create the following Ingress resource that will route requests with `test-grafana.test.svc.cluster.local` SNI to the Grafana:

:::{literalinclude} ../../../../examples/monitoring/v1alpha1/grafana-haproxy.ingress.yaml
:language: yaml
:linenos:
:::

You can apply the above manifest using `kubectl`:

:::{code-block} shell
:substitutions:
kubectl apply -n scylla --server-side -f=https://raw.githubusercontent.com/{{repository}}/{{revision}}/examples/monitoring/v1alpha1/grafana-haproxy.ingress.yaml
:::

:::{note}
In production, you should make sure that the Ingress controller properly terminates TLS using certificates issued by a trusted CA,
e.g. using [cert-manager](https://cert-manager.io/docs/) to automatically issue and renew certificates from Let's Encrypt.
:::

## Verify connection

### Get Grafana credentials

To access Grafana, you need to collect the credentials.

```console
GRAFANA_USER="$( kubectl -n scylla get secret/example-grafana-admin-credentials --template '{{ index .data "username" }}' | base64 -d )"
GRAFANA_PASSWORD="$( kubectl -n scylla get secret/example-grafana-admin-credentials --template '{{ index .data "password" }}' | base64 -d )"
```

### Get Ingress IP and Port

If your cluster supports `LoadBalancer` services, your Ingress should be assigned an external IP address. You can get it by running:

```console
INGRESS_IP="$( kubectl -n haproxy-ingress get svc haproxy-ingress --template '{{ index .status.loadBalancer.ingress 0 "ip" }}' )"
INGRESS_PORT="443"
```

Otherwise, if you're running this locally (e.g. using `minikube` or `kind`), you can port-forward the Ingress controller service to your local machine:

```console
kubectl -n haproxy-ingress port-forward svc/haproxy-ingress 8443:443 &
INGRESS_IP="127.0.0.1"
INGRESS_PORT="8443"
```

### Test connection

Now, you can verify the connection to the Grafana through the Ingress.

```console
curl --fail -s -o /dev/null -w '%{http_code}' -k \
    --resolve "test-grafana.test.svc.cluster.local:${INGRESS_PORT}:${INGRESS_IP}" \
    --user "${GRAFANA_USER}:${GRAFANA_PASSWORD}" \
    "https://test-grafana.test.svc.cluster.local:${INGRESS_PORT}/"
```

You should see `200` as the output, indicating a successful connection.
