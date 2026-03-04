# Security

This page explains the security model of ScyllaDB Operator: how TLS certificates are managed, how ScyllaDB authentication and authorization work on Kubernetes, how RBAC is structured, and how ScyllaDB Manager Agent communicates securely.

## TLS certificates

ScyllaDB Operator manages TLS certificates at two levels: for the **operator infrastructure** (webhook server) and for **ScyllaDB clusters** (client-to-node and inter-node encryption, Alternator).

### Operator webhook TLS

The operator's validating webhook server requires a TLS certificate so that the Kubernetes API server can securely call it. This certificate is provisioned by [cert-manager](https://cert-manager.io/):

1. A **self-signed Issuer** (`scylla-operator-selfsigned-issuer`) is created in the `scylla-operator` namespace.
2. A **Certificate** (`scylla-operator-serving-cert`) is issued for the DNS name `scylla-operator-webhook.scylla-operator.svc`, stored in a Secret of the same name.
3. The `ValidatingWebhookConfiguration` has the annotation `cert-manager.io/inject-ca-from: scylla-operator/scylla-operator-serving-cert`, which tells cert-manager to inject the CA bundle into the webhook configuration so the Kubernetes API server trusts the webhook's certificate.

cert-manager is a **hard dependency** of the operator. Without it, the webhook server cannot obtain a serving certificate and the operator cannot start.

### ScyllaDB cluster TLS

When the `AutomaticTLSCertificates` feature gate is enabled (default since v1.11, beta), the operator provisions a full PKI for each ScyllaDB cluster **without** requiring cert-manager. The operator manages certificates directly using its own internal certificate manager.

For each ScyllaDBDatacenter the operator creates:

| Resource | Type | Purpose |
|----------|------|---------|
| `<name>-local-client-ca` | Secret + ConfigMap | Client certificate authority — signs client certificates used for mutual TLS authentication to CQL. The CA bundle ConfigMap is distributed to ScyllaDB nodes as a truststore. |
| `<name>-local-user-admin` | Secret | Client certificate for the `admin` user — used for mTLS authentication to CQL. |
| `<name>-local-serving-ca` | Secret + ConfigMap | Serving certificate authority — signs the server certificate used by ScyllaDB for encrypted CQL connections. |
| `<name>-local-serving-certs` | Secret | Server certificate for ScyllaDB — contains a multi-SAN certificate covering all pod IPs, Service ClusterIPs, LoadBalancer addresses, internal DNS names, and `cql.<domain>` / `<hostID>.cql.<domain>` DNS names for each configured DNS domain. |
| `<name>-local-cql-connection-configs-admin` | Secret | Pre-built CQL connection configuration file containing the admin client certificate, key, and serving CA bundle for each DNS domain. |

**Certificate lifetimes:**

- CA certificates: 10-year validity, refreshed after 8 years.
- Serving certificates: 30-day validity, refreshed after 20 days.
- Client certificates: 10-year validity, refreshed after 8 years.

The operator watches all member Services and pod IPs and regenerates the serving certificate whenever the set of addresses changes (new pods, LoadBalancer address assignment, etc.).

#### How TLS is configured in ScyllaDB

When `AutomaticTLSCertificates` is enabled, the operator renders a managed `scylla.yaml` configuration that enables:

- **Client-to-node encryption** (`client_encryption_options`): enabled with `require_client_auth: true`, using the serving certificate and client CA truststore. CQL clients must present a valid client certificate signed by the cluster's client CA.
- **Encrypted CQL ports**: port `9142` (CQL over TLS) and port `19142` (shard-aware CQL over TLS) are configured alongside the unencrypted ports.

:::{note}
Inter-node encryption is configured through the same serving certificate infrastructure, but inter-node transport security settings depend on the ScyllaDB version and configuration.
:::

#### User-managed certificates

Both `ScyllaCluster` (v1) and `ScyllaDBDatacenter` (v1alpha1) support a `TLSCertificate` configuration with two modes:

- **`OperatorManaged`** (default): the operator provisions and rotates certificates automatically as described above. You can specify `additionalDNSNames` and `additionalIPAddresses` to include custom SANs in the serving certificate.
- **`UserManaged`**: you provide your own TLS certificate in a `kubernetes.io/tls` Secret referenced by `secretName`. The operator mounts this Secret but does not manage its lifecycle — you are responsible for rotation.

### Alternator TLS

When Alternator is enabled (`spec.alternator` in ScyllaCluster, `spec.scyllaDB.alternatorOptions` in ScyllaDBDatacenter), the operator creates a separate serving CA and certificate:

| Resource | Purpose |
|----------|---------|
| `<name>-alternator-local-serving-ca` | CA for Alternator serving certificates. |
| `<name>-alternator-local-serving-certs` | Server certificate for the Alternator HTTPS endpoint (port 8043). |

The Alternator certificate includes the same SANs as the CQL serving certificate, plus any `additionalDNSNames` and `additionalIPAddresses` specified in `servingCertificate.operatorManagedOptions`.

Like CQL certificates, Alternator certificates can be set to `UserManaged` if you want to provide your own.

## Authentication and authorization

### CQL authentication

ScyllaDB authentication and authorization are **not** enabled by the operator by default. To enable them, provide a ConfigMap with a custom `scylla.yaml` that sets:

```yaml
authenticator: PasswordAuthenticator
authorizer: CassandraAuthorizer
```

This ConfigMap is referenced in the ScyllaCluster or ScyllaDBCluster spec and is merged with the operator-managed configuration.

When `AutomaticTLSCertificates` is enabled, CQL connections also require mutual TLS (client certificate authentication). This provides two layers of authentication: TLS client certificates at the transport level and username/password at the CQL protocol level.

The default superuser credentials are `cassandra`/`cassandra`. You should change the superuser password immediately after enabling authentication, and create application-specific roles with minimal privileges.

### Alternator authorization

Alternator authorization is controlled by the `insecureDisableAuthorization` field in the Alternator spec:

- When authorization is **enabled** (the default for new clusters), Alternator requests must include valid AWS Signature Version 4 credentials. The credentials are derived from CQL roles — the `username` maps to the AWS access key, and the `salted_hash` of the password maps to the secret key.
- When `insecureDisableAuthorization` is set to `true`, any request is accepted without authentication. This is intended only for development and testing.

:::{caution}
For backwards compatibility, authorization is disabled when using a manual Alternator port (the deprecated `port` field) and `insecureDisableAuthorization` is not explicitly set. Always set `insecureDisableAuthorization: false` explicitly in production.
:::

## RBAC

The operator uses [Kubernetes RBAC](https://kubernetes.io/docs/reference/access-authn-authz/rbac/) to control access at three levels: operator permissions, ScyllaDB pod permissions, and user-facing roles.

### Operator ClusterRole

The `scylladb:controller:operator` ClusterRole is an [aggregated ClusterRole](https://kubernetes.io/docs/reference/access-authn-authz/rbac/#aggregated-clusterroles) that combines permissions from multiple component ClusterRoles selected by the label `rbac.operator.scylladb.com/aggregate-to-scylla-operator: "true"`. This includes permissions to manage pods, Services, StatefulSets, PVCs, Secrets, ConfigMaps, Jobs, PodDisruptionBudgets, ServiceMonitors, PrometheusRules, and all ScyllaDB CRDs.

The operator runs as the `scylla-operator` ServiceAccount in the `scylla-operator` namespace, bound to the aggregate ClusterRole via a ClusterRoleBinding.

On OpenShift, additional permissions are added through a separate component ClusterRole (e.g., SecurityContextConstraints access).

### Member ClusterRole

Each ScyllaDB pod runs with a dedicated ServiceAccount created per ScyllaDBDatacenter (named `<datacenter-name>-member`). This ServiceAccount is bound to the `scyllacluster-member` aggregate ClusterRole via a namespaced RoleBinding.

The member ClusterRole grants the sidecar running inside each ScyllaDB pod the minimum permissions it needs:

- **Read** pods, Secrets, ConfigMaps, Services, and StatefulSets — the sidecar reads its own identity, certificates, and cluster topology.
- **Write** pods and Services — the sidecar updates annotations on its own Service (host ID, token ring hash) and pod labels.
- **Read** `ScyllaDBDatacenterNodeStatusReports` — used for bootstrap synchronisation state.

The member role does **not** grant permissions to modify StatefulSets, create or delete pods, or access resources outside the namespace.

### User-facing ClusterRoles

The operator installs two ClusterRoles for Kubernetes users who manage ScyllaDB resources:

| ClusterRole | Aggregated to | Permissions |
|-------------|---------------|-------------|
| `scyllacluster-view` | `admin`, `edit`, `view` | Read-only access to all ScyllaDB CRDs (ScyllaClusters, ScyllaDBDatacenters, ScyllaDBClusters, ScyllaDBMonitorings, ScyllaDBManagerClusterRegistrations, ScyllaDBManagerTasks). |
| `scyllacluster-edit` | `admin`, `edit` | Create, update, patch, and delete ScyllaDB CRDs. |

These ClusterRoles are aggregated into the default Kubernetes `admin`, `edit`, and `view` ClusterRoles. This means any user who has the Kubernetes `edit` role in a namespace can automatically create and manage ScyllaDB clusters in that namespace.

### Monitoring and remote ClusterRoles

Additional aggregate ClusterRoles exist for specific components:

- **Prometheus** (`scylladb:controller:prometheus`): permissions to read endpoints, pods, Services, and nodes. Required for Prometheus service discovery.
- **Grafana** (`scylladb:controller:grafana`): permissions to manage Grafana-related ConfigMaps and Secrets.
- **Remote operator** (`scylladb:controller:remote-operator`): permissions for the operator to manage resources in remote Kubernetes clusters for multi-DC deployments.

## ScyllaDB Manager Agent security

ScyllaDB Manager communicates with ScyllaDB nodes through the [Manager Agent](manager.md), which runs as a sidecar container in each ScyllaDB pod. The Agent exposes a REST API on port 10001.

### Authentication token

The operator generates a unique authentication token for each ScyllaDBDatacenter and stores it in a Secret (`<name>-agent-auth-token`). This token is:

- Mounted into the Agent sidecar container as a configuration file.
- Used by ScyllaDB Manager to authenticate API calls to the Agent.
- Used by the operator itself when it communicates with the Agent (for example, to coordinate cleanup jobs).

For multi-DC `ScyllaDBCluster` deployments, the operator can override per-datacenter tokens with a shared token (via an internal annotation) so that ScyllaDB Manager can use a single credential across all datacenters.

### Shared namespace

ScyllaDB Manager is deployed in the `scylla-manager` namespace. It needs network access to the Agent API port (10001) on every ScyllaDB pod, regardless of namespace. The Manager does not require RBAC access to the Kubernetes API — it communicates directly with the Agents over HTTP using the authentication token.

:::{caution}
The Agent authentication token is a bearer token. Anyone with read access to the token Secret can impersonate ScyllaDB Manager and issue commands to the Agent (backups, repairs, host operations). Restrict Secret read access in the ScyllaDB namespace to trusted operators.
:::

## Webhook validation

The operator runs a dedicated webhook server (separate Deployment from the operator controller) that validates all CREATE and UPDATE operations on ScyllaDB CRDs:

- `ScyllaCluster` (v1)
- `NodeConfig`, `ScyllaOperatorConfig`, `ScyllaDBDatacenter`, `ScyllaDBCluster`, `ScyllaDBManagerClusterRegistration`, `ScyllaDBManagerTask`, `ScyllaDBMonitoring` (v1alpha1)

The webhook is configured with `failurePolicy: Fail`, meaning that if the webhook server is unreachable, the Kubernetes API server rejects the request. This prevents invalid configurations from being applied when the webhook is down, but it also means the webhook server must be available for any ScyllaDB resource mutation.

The webhook server has its own PDB (`minAvailable: 1`) to ensure availability during node drains. See [Pod disruption budgets](pod-disruption-budgets.md).

## Network policies

The operator does **not** create Kubernetes NetworkPolicy resources. If your cluster enforces network policies, you must ensure that the following traffic is allowed:

| Source | Destination | Port | Purpose |
|--------|-------------|------|---------|
| ScyllaDB pods | ScyllaDB pods | 7000, 7001 | Inter-node communication |
| ScyllaDB pods | ScyllaDB pods | 9042, 9142 | CQL (seed resolution, sidecar) |
| Application pods | ScyllaDB pods | 9042, 9142, 19042, 19142 | CQL client traffic |
| Application pods | ScyllaDB pods | 8000, 8043 | Alternator (if enabled) |
| ScyllaDB Manager | ScyllaDB pods | 10001 | Manager Agent API |
| Prometheus | ScyllaDB pods | 9180, 5090, 9100 | Metrics scraping |
| Kubernetes API server | Webhook server pods | 5000 | Admission webhook calls |
| Operator pods | Kubernetes API server | 443 | Controller reconciliation |

## Related pages

- [Overview](overview.md) — reconciliation model and component layout.
- [Manager](manager.md) — ScyllaDB Manager deployment model.
- [Pod disruption budgets](pod-disruption-budgets.md) — how operator and webhook PDBs protect availability.
- [Sidecar](sidecar.md) — containers in a ScyllaDB pod, including the Agent sidecar.
- [Ignition](ignition.md) — startup gating that depends on certificate readiness.
