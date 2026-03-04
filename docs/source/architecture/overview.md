# Overview

ScyllaDB Operator is a set of controllers and API extensions that run inside your Kubernetes cluster.
It extends the Kubernetes API using [CustomResourceDefinitions (CRDs)](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/) and [dynamic admission webhooks](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/) to provide new resources.
These resources are reconciled by controllers embedded within the ScyllaDB Operator deployment.

## Reconciliation model

ScyllaDB Operator follows the standard Kubernetes [controller pattern](https://kubernetes.io/docs/concepts/architecture/controller/).
Each controller watches one or more resource types and continuously reconciles the desired state (what you declared in the custom resource spec) with the actual state (what currently exists in the cluster).

A reconciliation loop runs whenever a watched resource changes, or periodically on a 12-hour resync interval.
During each reconciliation, the controller:

1. Reads the current spec of the custom resource.
2. Computes the set of Kubernetes objects that should exist (StatefulSets, Services, ConfigMaps, Jobs, PodDisruptionBudgets, and more).
3. Creates, updates, or deletes objects to match the desired state.
4. Updates the custom resource's status to reflect the current state.

This means that manual changes to managed objects (such as editing a StatefulSet created by the Operator) will be reverted on the next reconciliation.
To make changes, always modify the custom resource spec.

## Deployments and namespaces

ScyllaDB Operator installs into three namespaces:

| Namespace | What runs there |
|---|---|
| `scylla-operator` | Two Deployments: the **controller manager** (`scylla-operator`) that runs all controllers, and the **webhook server** (`scylla-operator-webhook-server`) that validates API requests. Both run with 2 replicas by default and are protected by PodDisruptionBudgets. |
| `scylla-manager` | **ScyllaDB Manager** — a separate component that coordinates repair and backup tasks. It uses a small internal ScyllaDB cluster as its own database. Deployed optionally. |
| `scylla-operator-node-tuning` | **Node tuning agents** — privileged DaemonSets and Jobs created by the NodeConfig controller to set up disks, filesystems, and performance tuning on Kubernetes nodes. |

Your ScyllaDB clusters run in your own namespaces, separate from the Operator.

## Custom resources

The Operator provides the following CRDs, all in the `scylla.scylladb.com` API group:

### Cluster-scoped resources

Cluster-scoped resources affect the entire Kubernetes cluster and require elevated privileges.

| Resource | API version | Purpose |
|---|---|---|
| `NodeConfig` | `v1alpha1` | Configures Kubernetes nodes for ScyllaDB: RAID setup, filesystem creation, mount points, sysctls, and performance tuning. See [Tuning](tuning.md). |
| `ScyllaOperatorConfig` | `v1alpha1` | Global Operator configuration: auxiliary images, cluster domain, tuning image overrides. A singleton named `cluster`. |
| `RemoteKubernetesCluster` | `v1alpha1` | Connection credentials to a remote Kubernetes cluster for multi-datacenter deployments. |

### Namespaced resources

Namespaced resources are scoped to a Kubernetes namespace and can be managed by namespace-level RBAC.

| Resource | API version | Purpose |
|---|---|---|
| `ScyllaCluster` | `v1` (stable) | Defines a single-datacenter ScyllaDB cluster. The primary resource for most deployments. |
| `ScyllaDBDatacenter` | `v1alpha1` | Defines one ScyllaDB datacenter within a single Kubernetes cluster. Used internally by `ScyllaDBCluster` and can also be used directly. |
| `ScyllaDBCluster` | `v1alpha1` | Orchestrates a multi-datacenter ScyllaDB cluster spanning multiple Kubernetes clusters. |
| `ScyllaDBMonitoring` | `v1alpha1` | Defines a monitoring stack (Prometheus + Grafana) for ScyllaDB. See [Monitoring](monitoring.md). |
| `ScyllaDBManagerTask` | `v1alpha1` | Defines a backup or repair task managed by ScyllaDB Manager. |

:::{tip}
You can discover the available resources in your cluster by running:
```bash
kubectl api-resources --api-group=scylla.scylladb.com
```
and explore their schema with:
```bash
kubectl explain --api-version='scylla.scylladb.com/v1alpha1' NodeConfig.spec
```
:::

## Controllers

The controller manager runs 12 controllers:

| Controller | What it reconciles |
|---|---|
| ScyllaDBDatacenter | Creates and manages StatefulSets, Services, ConfigMaps, Jobs, PDBs, Ingresses, and TLS certificates for each ScyllaDB datacenter. The core controller. |
| ScyllaCluster | Bridges the stable `ScyllaCluster` (v1) API to the underlying `ScyllaDBDatacenter` resource, and synchronises Manager tasks. |
| ScyllaDBCluster | Orchestrates multi-datacenter clusters by managing `ScyllaDBDatacenter` resources across remote Kubernetes clusters. |
| NodeConfig | Deploys privileged DaemonSets and Jobs that set up RAID, filesystems, mount points, sysctls, and performance tuning on Kubernetes nodes. |
| NodeConfigPod | Watches ScyllaDB pods and creates per-pod tuning ConfigMaps that tie container-level tuning to a specific container instance. |
| ScyllaOperatorConfig | Reconciles the global Operator configuration, discovers cluster domain, and resolves auxiliary images. |
| ScyllaDBMonitoring | Deploys and configures Prometheus, Grafana, ServiceMonitors, PrometheusRules, and dashboards. |
| RemoteKubernetesCluster | Manages connections and informers for remote Kubernetes clusters used in multi-DC deployments. |
| OrphanedPV | Detects and cleans up PersistentVolumes that become orphaned when ScyllaDB nodes are removed. |
| ScyllaDBManager | Coordinates global ScyllaDB Manager state across all clusters. |
| ScyllaDBManagerClusterRegistration | Registers ScyllaDB clusters with ScyllaDB Manager. |
| ScyllaDBManagerTask | Reconciles backup and repair task definitions with ScyllaDB Manager. |

Additional controllers run **inside ScyllaDB pods** rather than in the Operator deployment:

| Controller | Where it runs | What it does |
|---|---|---|
| Sidecar | Main ScyllaDB container | Manages the ScyllaDB process lifecycle, syncs member service annotations with host ID and token ring state. See [Sidecar](sidecar.md). |
| Ignition | `scylladb-ignition` sidecar container | Evaluates startup prerequisites (tuning done, IP assigned, LB ready) and creates the ignition signal file. See [Ignition](ignition.md). |
| StatusReport | Main ScyllaDB container | Reports node status to `ScyllaDBDatacenterNodesStatusReport` resources for bootstrap synchronisation. |
| BootstrapBarrier | `scylladb-bootstrap-barrier` init container | Blocks pod startup until bootstrap preconditions are met (feature-gated). See [Bootstrap Sync](bootstrap-sync.md). |
| NodeSetup | Node tuning DaemonSet pods | Configures the host machine (runs via the `node-setup` subcommand). |

## Admission webhooks

The Operator runs a webhook server that validates **create** and **update** operations on all public CRDs.
The webhook server runs as a separate Deployment (`scylla-operator-webhook-server`) in the `scylla-operator` namespace and is exposed via a Service on port 443.

TLS certificates for the webhook server are provisioned automatically using [cert-manager](https://cert-manager.io/).
The `ValidatingWebhookConfiguration` is configured with a `caBundle` injected by cert-manager.

:::{note}
Some Kubernetes platforms have non-conformant networking that prevents the API server from reaching the webhook server.
If resource creation fails with webhook errors after installation, see [Troubleshooting Installation](../troubleshooting/installation.md).
:::

## Dependency chain

The Operator has the following installation dependencies, which must be deployed in order:

1. **cert-manager** — provisions TLS certificates for the webhook server. Must be available before the Operator is installed.
2. **Prometheus Operator** (optional) — provides the `ServiceMonitor` and `PrometheusRule` CRDs used by `ScyllaDBMonitoring`. Required only if you use `ScyllaDBMonitoring`.
3. **ScyllaDB Operator** — the controller manager and webhook server.
4. **Local CSI Driver** (optional) — provides dynamic local volume provisioning with XFS and project quotas. Required if you use local NVMe storage with the ScyllaDB-provided storage provisioner. See [Storage](storage.md).
5. **NodeConfig** — configures Kubernetes nodes for ScyllaDB (RAID, filesystem, tuning). See [Tuning](tuning.md).
6. **ScyllaDB Manager** (optional) — coordinates repair and backup tasks. Depends on the Operator because it uses a `ScyllaCluster` for its internal database.

For step-by-step installation instructions, see [GitOps Installation](../installation/gitops.md) or [Helm Installation](../installation/helm.md).
