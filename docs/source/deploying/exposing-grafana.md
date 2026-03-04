# Exposing Grafana

This page explains how to expose the Grafana dashboard deployed by `ScyllaDBMonitoring` using a Kubernetes Ingress.

## Prerequisites

- [Monitoring](monitoring.md) deployed with a `ScyllaDBMonitoring` resource.
- An Ingress controller installed in your cluster (this guide uses HAProxy as an example).

## How Grafana is exposed by default

When you create a `ScyllaDBMonitoring` resource, the Operator deploys a Grafana instance and creates:

| Resource | Name | Description |
|----------|------|-------------|
| Service | `<monitoring-name>-grafana` | ClusterIP Service on port 3000. |
| Secret | `<monitoring-name>-grafana-serving-ca` | Self-signed CA used to generate the serving certificate. |
| Secret | `<monitoring-name>-grafana-serving-cert` | TLS serving certificate (30-day validity, auto-rotated). |
| Secret | `<monitoring-name>-grafana-admin-credentials` | Grafana admin username and password. |

By default, Grafana is only accessible within the cluster via the ClusterIP Service.

## Using a custom TLS certificate

To use your own TLS certificate instead of the auto-generated self-signed one, set `servingCertSecretName` on the `ScyllaDBMonitoring` resource:

```yaml
spec:
  components:
    grafana:
      servingCertSecretName: my-grafana-cert
```

The Secret must contain `tls.crt` and `tls.key` entries. When set, the Operator skips creating the self-signed CA and serving certificate.

For production deployments, use [cert-manager](https://cert-manager.io/) with Let's Encrypt or your organization's PKI to issue and rotate the certificate automatically.

## Step 1: Install an Ingress controller

If you do not already have an Ingress controller, install one. This example uses HAProxy:

```shell
kubectl apply -f https://raw.githubusercontent.com/haproxytech/kubernetes-ingress/master/deploy/haproxy-ingress.yaml
kubectl -n haproxy-controller rollout status deployment/haproxy-ingress
```

## Step 2: Create an Ingress resource

Create an Ingress that routes traffic to the Grafana Service. Since Grafana serves TLS, configure the Ingress controller to trust Grafana's backend certificate:

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: example-grafana
  namespace: scylla
  annotations:
    haproxy.org/server-ssl: "true"
    haproxy.org/server-ca: "default/example-grafana-serving-ca"
spec:
  ingressClassName: haproxy
  tls:
  - hosts:
    - grafana.example.com
  rules:
  - host: grafana.example.com
    http:
      paths:
      - backend:
          service:
            name: example-grafana
            port:
              number: 3000
        path: /
        pathType: Prefix
```

:::{note}
Replace `example` with the name of your `ScyllaDBMonitoring` resource and `grafana.example.com` with your actual domain. The `haproxy.org/server-ca` annotation should reference the namespace and name of the Grafana serving CA Secret.
:::

```shell
kubectl -n scylla apply --server-side -f grafana-ingress.yaml
```

## Step 3: Retrieve admin credentials

```shell
kubectl -n scylla get secret/example-grafana-admin-credentials \
  -o jsonpath='{.data.password}' | base64 -d
```

The default username is `admin`.

## Step 4: Verify connectivity

Get the Ingress address:

```shell
kubectl -n scylla get ingress example-grafana
```

Test the connection:

```shell
curl -k https://grafana.example.com/api/health
```

Expected output:

```json
{"commit":"...","database":"ok","version":"..."}
```

## Local development alternative

For quick local access without an Ingress controller, use port-forwarding:

```shell
kubectl -n scylla port-forward svc/example-grafana 3000:3000
```

Then open `https://localhost:3000` in your browser.

## Related pages

- [Monitoring](monitoring.md) — deploying ScyllaDBMonitoring, Prometheus, and Grafana.
- [Operator configuration](operator-configuration.md) — Grafana image configuration.
- [Production checklist](production-checklist.md) — monitoring as a production requirement.
