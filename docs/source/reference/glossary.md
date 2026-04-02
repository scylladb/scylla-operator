# Glossary

Terminology used in ScyllaDB Operator documentation. This glossary maps ScyllaDB concepts to their Kubernetes equivalents and defines Operator-specific terms.

## ScyllaDB ↔ Kubernetes concept mapping

| ScyllaDB concept | Kubernetes equivalent | Notes |
|---|---|---|
| Node | Pod | A single ScyllaDB process runs inside one pod. |
| Rack | StatefulSet | One StatefulSet per rack. Pod names follow `<cluster>-<dc>-<rack>-<ordinal>`. See [StatefulSets and racks](../understand/statefulsets-and-racks.md). |
| Datacenter | ScyllaCluster | One `ScyllaCluster` resource per datacenter. For multi-DC, one per DC connected via `externalSeeds`. |
| Cluster | One or more ScyllaCluster resources | The top-level resource representing the entire ScyllaDB deployment. For multi-DC, multiple `ScyllaCluster` resources with the same `.metadata.name` are connected via `externalSeeds`. |
| Configuration file (`scylla.yaml`) | Custom resource spec | Configuration is declared in the CR and applied by the Operator. |
| `nodetool` | `kubectl exec` + `nodetool` | Run inside the ScyllaDB pod. See [nodetool alternatives](nodetool-alternatives.md). |
| Repair / backup scheduling | ScyllaDB Manager tasks | Defined in the cluster spec or as ScyllaDBManagerTask resources. |

## Custom resources

**ScyllaCluster** (`v1`, stable)
: The primary resource for single-datacenter deployments. Defines one ScyllaDB datacenter within a Kubernetes cluster. For multi-DC, deploy one `ScyllaCluster` per datacenter and connect them via `externalSeeds`. See [Deploy a multi-datacenter cluster](../deploy-scylladb/deploy-multi-dc-cluster.md).

**NodeConfig** (`v1alpha1`, cluster-scoped)
: Configures Kubernetes nodes for ScyllaDB: RAID setup, filesystem creation, mount points, sysctls, and performance tuning.

**ScyllaOperatorConfig** (`v1alpha1`, cluster-scoped)
: Global Operator configuration. A singleton resource named `cluster`.

**ScyllaDBMonitoring** (`v1alpha1`)
: Defines a monitoring stack (Prometheus + Grafana) for ScyllaDB.

## Operator components

**Controller manager**
: The main Deployment (`scylla-operator` in the `scylla-operator` namespace) that runs all controllers.

**Webhook server**
: A separate Deployment (`scylla-operator-webhook-server`) that validates create/update operations on CRDs via Kubernetes admission webhooks. TLS certificates provisioned by cert-manager.

**ScyllaDB Manager**
: A separate component (deployed in the `scylla-manager` namespace) that coordinates repair and backup tasks.

**ScyllaDB Manager Agent**
: A sidecar container in each ScyllaDB pod that communicates with ScyllaDB Manager.

**Local CSI Driver**
: A DaemonSet-based CSI driver that provisions PersistentVolumes from local NVMe storage configured by NodeConfig. Uses XFS with project quotas.

## ScyllaDB concepts

**Shard**
: ScyllaDB's internal parallelism unit — each CPU core runs one shard. Relevant for CPU pinning and the `--smp` flag.

**Token ring**
: The distributed hash ring that determines data ownership across nodes. The Operator tracks token ring changes to trigger automatic data cleanup.

**Seed nodes**
: Nodes used by new nodes to discover the cluster. Managed automatically by the Operator.

**Host ID**
: A unique UUID identifying a ScyllaDB node in the cluster ring. Tracked via member Service annotations.

**Gossip**
: The peer-to-peer protocol ScyllaDB nodes use to share cluster state (membership, schema, health).

**Snitch**
: The component that determines rack and datacenter placement for each node. Configured automatically by the sidecar.

**CQL**
: Cassandra Query Language — the primary client protocol for ScyllaDB.

**Alternator**
: ScyllaDB's DynamoDB-compatible API. Enabled via `spec.alternator` in the cluster resource.

**nodetool**
: ScyllaDB's command-line administration tool. Communicates via REST API (unlike Apache Cassandra's JMX-based version). See [nodetool alternatives](nodetool-alternatives.md).

## Internal mechanisms

**Ignition**
: The startup gating mechanism that prevents ScyllaDB from starting before prerequisites are met (tuning complete, network identity assigned). See [Ignition](../understand/ignition.md).

**Bootstrap synchronisation**
: Blocks a new node's startup until every existing node is confirmed healthy. Feature-gated via `BootstrapSynchronisation`. See [Bootstrap sync](../understand/bootstrap-sync.md).

**Reconciliation**
: The Kubernetes controller pattern where each controller continuously compares desired state (spec) with actual state and takes corrective action.

**Member Service**
: A dedicated Kubernetes Service for each ScyllaDB node, providing a stable network identity independent of pod restarts.

**Discovery endpoint**
: A Kubernetes Service that allows clients to discover all ScyllaDB nodes. Used as the CQL contact point.

**PodDisruptionBudget (PDB)**
: Kubernetes mechanism limiting voluntary pod evictions. The Operator creates a PDB with `maxUnavailable: 1` per datacenter. See [Pod disruption budgets](../understand/pod-disruption-budgets.md).

**SidecarRuntimeConfig**
: A per-pod tuning ConfigMap created by the NodeConfigPod controller. Must exist before ignition allows ScyllaDB to start.

**must-gather**
: An embedded diagnostic collection tool that captures logs, nodetool output, and system state into an archive. See [must-gather](../troubleshoot/collect-debugging-information/must-gather.md).

## Feature gates

**AutomaticTLSCertificates**
: Beta feature gate (default `true` since v1.11). Enables automated TLS certificate provisioning for encrypted CQL communication.

**BootstrapSynchronisation**
: Alpha feature gate (default `false`, since v1.19). Requires ScyllaDB ≥ 2025.2. See [Feature gates](feature-gates.md).

## Dependencies

**cert-manager**
: A Kubernetes add-on that provisions TLS certificates for the webhook server. Must be installed before the Operator.

**Prometheus Operator**
: Provides ServiceMonitor and PrometheusRule CRDs used by ScyllaDBMonitoring. Required only if monitoring is used.
