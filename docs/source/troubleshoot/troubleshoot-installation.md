# Troubleshoot installation issues

Diagnose and resolve problems that occur when installing ScyllaDB Operator, including webhook failures, CRD propagation issues, and cert-manager problems.

## Webhook connectivity failures

ScyllaDB Operator uses validating and mutating webhooks.
The Kubernetes API server must be able to reach the webhook Service on port 443.

### Symptoms

- `kubectl apply` for ScyllaCluster resources hangs or returns a timeout error.
- Operator logs show no incoming requests even though resources are being created.
- API server logs show `webhook call failed` or `connection refused` errors.

### Diagnosis

```bash
# Check webhook configurations
kubectl get validatingwebhookconfiguration
kubectl get mutatingwebhookconfiguration

# Check the webhook service
kubectl -n scylla-operator get svc scylla-operator-webhook

# Check that the webhook pod is running
kubectl -n scylla-operator get pods

# Test connectivity from the API server's perspective
kubectl -n scylla-operator get endpoints scylla-operator-webhook
```

### Platform-specific issues

#### EKS with custom CNI

EKS can break webhook connectivity when used with [custom CNI networking](https://github.com/aws/containers-roadmap/issues/1215).
The API server cannot reach pods on the custom CNI network.

:::{note}
Use a conformant Kubernetes cluster that supports webhooks.
Workarounds such as reconfiguring the webhook to use Ingress or `hostNetwork` are beyond standard supported configurations.
:::

#### GKE private clusters

GKE private clusters require a firewall rule to allow the API server to reach webhook Services.

See [GKE: Adding firewall rules](https://cloud.google.com/kubernetes-engine/docs/how-to/latest/network-isolation#add_firewall_rules) for instructions.
The rule must allow traffic from the control plane CIDR to the webhook pod port (5000 by default).

## CRD not installed

### Symptoms

```
error: the server doesn't have a resource type "scyllaclusters"
```

### Diagnosis

```bash
kubectl get crd scyllaclusters.scylla.scylladb.com
```

### Resolution

Install the CRDs:

:::{note}
The commands below use `{{repository}}` and `{{revision}}` placeholders.
When reading this page on the versioned documentation site, these are automatically substituted with `scylladb/scylla-operator` and the current release tag.

If you are running these commands outside the documentation site (e.g., copied from a terminal), replace `{{repository}}` with `scylladb/scylla-operator` and `{{revision}}` with your operator version tag (e.g., `v1.20.0`).
:::

::::{tabs}
:::{group-tab} GitOps
```bash
kubectl apply --server-side -f=https://raw.githubusercontent.com/{{repository}}/{{revision}}/deploy/operator.yaml
```
:::
:::{group-tab} Helm
Helm does not update CRDs automatically.
Apply them manually:
```bash
kubectl apply --server-side -f=https://raw.githubusercontent.com/{{repository}}/{{revision}}/helm/scylla-operator/crds/
```
:::
::::

## cert-manager issues

ScyllaDB Operator uses cert-manager to provision the TLS certificate for the webhook server.

### Symptoms

- Operator pods are `Running` but webhook calls fail with TLS errors.
- Certificate or CertificateRequest resources show `NotReady`.

### Diagnosis

```bash
# Check cert-manager is running
kubectl -n cert-manager get pods

# Check the certificate
kubectl -n scylla-operator get certificate
kubectl -n scylla-operator describe certificate

# Check certificate requests
kubectl -n scylla-operator get certificaterequest
```

### Common causes

| Cause | Resolution |
|---|---|
| cert-manager not installed | Install cert-manager ([Prerequisites](../install-operator/prerequisites.md)) |
| cert-manager CRDs missing | `kubectl apply -f https://github.com/cert-manager/cert-manager/releases/latest/download/cert-manager.crds.yaml` |
| cert-manager pods not ready | Check cert-manager pod logs: `kubectl -n cert-manager logs deploy/cert-manager` |
| Self-signed issuer not created | The Operator deployment manifests include the issuer; re-apply if missing |

## Operator pod not starting

### Symptoms

- Operator pods stuck in `Pending`, `CrashLoopBackOff`, or `ImagePullBackOff`.

### Diagnosis

```bash
kubectl -n scylla-operator describe pod <pod-name>
kubectl -n scylla-operator logs <pod-name>
```

### Common causes

| Cause | Resolution |
|---|---|
| `ImagePullBackOff` | Verify the image tag exists and the registry is accessible |
| `CrashLoopBackOff` | Check logs for startup errors; verify RBAC permissions |
| `Pending` | Check node resources and scheduling constraints |
| Leader election failure | Check if another Operator instance is running; verify lease objects in the namespace |

## Verifying a successful installation

After installation, verify all components:

```bash
# Operator pods
kubectl -n scylla-operator get pods

# CRDs
kubectl get crd | grep scylla

# Webhooks
kubectl get validatingwebhookconfiguration | grep scylla
kubectl get mutatingwebhookconfiguration | grep scylla

# Create a test ScyllaCluster (optional)
kubectl create namespace scylla-test
kubectl -n scylla-test apply --server-side -f=https://raw.githubusercontent.com/{{repository}}/{{revision}}/examples/generic/cluster.yaml
```

## Related pages

- [Installation prerequisites](../install-operator/prerequisites.md)
- [Install with GitOps](../install-operator/install-with-gitops.md)
- [Install with Helm](../install-operator/install-with-helm.md)
- [Diagnostic flowchart](diagnostic-flowchart.md)
