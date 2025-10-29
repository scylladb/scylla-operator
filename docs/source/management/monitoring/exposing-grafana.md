# Exposing Grafana

This guide shows how to expose Grafana deployed by `ScyllaDBMonitoring` using an Ingress resource.

For accessing Grafana service from outside the Kubernetes cluster we recommend using an Ingress, although there are many other ways to do so.
When using Ingress, what matters is to direct your packets to the ingress controller Service/Pods and have the correct TLS SNI field set by the caller when reaching out to the service, so it is routed properly, and your client can successfully validate the grafana serving certificate.
This is easier when you are using a real DNS domain that resolves to your Ingress controller's IP address but most clients and tools allow setting the SNI field manually.

## Prerequisites

This assumes you have already deployed `ScyllaDBMonitoring` in your cluster. If you haven't done so, please follow the [ScyllaDB Monitoring setup](setup.md) guide first.

In the example below we're using HAProxy Ingress Controller. You can deploy it in your Kubernetes cluster using the provided
third-party example. If you already have it (or other Ingress Controller) deployed in your cluster, you can skip the below steps.

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

## Get Grafana credentials 

To access Grafana, you first need to collect the serving CA and the credentials.

```console
$ GRAFANA_SERVING_CERT="$( kubectl -n scylla get secret/example-grafana-serving-ca --template '{{ index .data "tls.crt" }}' | base64 -d )"
$ GRAFANA_USER="$( kubectl -n scylla get secret/example-grafana-admin-credentials --template '{{ index .data "username" }}' | base64 -d )"
$ GRAFANA_PASSWORD="$( kubectl -n scylla get secret/example-grafana-admin-credentials --template '{{ index .data "password" }}' | base64 -d )"
```

## Connect through Ingress using a resolvable domain

In production clusters, the Ingress controller and appropriate DNS records should be set up already. Often there is already a generic wildcard record like `*.app.mydomain` pointing to the Ingress controller's external IP. For custom service domains, it is usually a CNAME pointing to the Ingress controller's A record.

:::{note}
The ScyllaDBMonitoring example creates an Ingress object with `test-grafana.test.svc.cluster.local` DNS domain that you should adjust to your domain. Below examples use `example-grafana.apps.mydomain`.
:::

:::{note}
To test a resolvable domain from your machine without creating DNS records, you can adjust `/etc/hosts` or similar.
:::

```console
$ curl --fail -s -o /dev/null -w '%{http_code}' -L --cacert <( echo "${GRAFANA_SERVING_CERT}" ) "https://example-grafana.apps.mydomain" --user "${GRAFANA_USER}:${GRAFANA_PASSWORD}"
200
```

## Connect through Ingress using an unresolvable domain

To connect to an Ingress without a resolvable domain you first need to find out your Ingress controller's IP that can be resolved externally. Again, there are many ways to do so beyond the below examples.

Unless stated otherwise, we assume your Ingress is running on port 443.

```console
$ INGRESS_PORT=443
```

### Variants

#### Ingress ExternalIP

When you are running in a real cluster there is usually a cloud LoadBalancer or a bare metal alternative providing you with an externally reachable IP address.

```console
$ INGRESS_IP="$( kubectl -n=haproxy-ingress get service/haproxy-ingress --template='{{ ( index .status.loadBalancer.ingress 0 ).ip }}' )"
```

#### Ingress NodePort

NodePort is slightly less convenient, but it's available in development clusters as well.

```console
$ INGRESS_IP="$( kubectl get nodes --template='{{ $internal_ip := "" }}{{ $external_ip := "" }}{{ range ( index .items 0 ).status.addresses }}{{ if eq .type "InternalIP" }}{{ $internal_ip = .address }}{{ else if eq .type "ExternalIP" }}{{ $external_ip = .address }}{{ end }}{{ end }}{{ if $external_ip }}{{ $external_ip }}{{ else }}{{ $internal_ip }}{{ end }}' )"
$ INGRESS_PORT="$( kubectl -n=haproxy-ingress get services/haproxy-ingress --template='{{ range .spec.ports }}{{ if eq .port 443 }}{{ .nodePort }}{{ end }}{{ end }}' )"
```

#### Connection

```console
$ curl --fail -s -o /dev/null -w '%{http_code}' -L --cacert <( echo "${GRAFANA_SERVING_CERT}" ) "https://test-grafana.test.svc.cluster.local:${INGRESS_PORT}" --resolve "test-grafana.test.svc.cluster.local:${INGRESS_PORT}:${INGRESS_IP}" --user "${GRAFANA_USER}:${GRAFANA_PASSWORD}"
200
```
