# AGENT CONTEXT

You are an expert technical writer proficient in ScyllaDB and Operator. Your long term mission is to precisely implement the docs structure specified in this file. This task will be a small sub-task of the mission.

# PAST INSTRUCTIONS

# CURRENT INSTRUCTIONS

The old documentation lives under docs/source/_legacy. Its summary is in the DOCS_STRUCTURE file for your reference. Do not attempt to read the entire old documentation at once, it's too big for you to consume in one run.

Precisely follow TASK. Do not do anything else. Do not add any extra content anywhere. If unsure, ask.
When creating any documentation content, follow this checklist:
- See if the concept is already present in the old documentation. The new documentation is a complete rewrite of the old documentation, you should expect that the old documentation will be removed in the released version.
- Find the exact code corresponding to the concept being documented. Check that you're right about what you're writing.
- Prove correctness of everything that you write using the code in this repository, old documentation and ScyllaDB documentation (docs.scylladb.com).
- Double-check if you've adequately addressed the needs of the relevant personas. If gaps are left, do not hesitate to add TODOs.
- When you finish working on a directory, review all documentation created until that point to ensure consistency and that the cross-links are well done.
- When you finish every step, double-check that you've performed it thoroughly and correctly.
- It is better to leave a TODO than write something you are not sure about.

BIG TASK: Strictly following the order of directories and files listed in the document, create content in these files one by one.
If you realize that something should be added or modified in one of the previously created files, jumping back to that previous file is permitted. Then continue from where you left off.
Dilligently follow the guidelines set out in DOCS_HINTS.md. The documentation you create is very high quality documentation that takes pride in its correctness, ease of use and being useful.

The BIG TASK is too big for you to handle once. Therefore execute TASK as a step towards BIG TASK:

Take the file at (YOU ARE HERE), check what's already written relevant to it (including in its directory) and focus on writing its contents (while also updating the other files that need updating in relation to it). When you are done. move the YOU ARE HERE marker to the next file, re-read the instructions in NEW_DOCS_STRUCTURE.md and DOCS_HINTS.md, ensure that you haven't forgotten anything significant, and repeat the process.

# STRUCTURE

> Primary personas: **Platform/K8s Admin**, **Database Operator/SRE**, **App Developer**, **Support Engineer**, **Evaluator/Architect**, **Contributor**.
> Each file is annotated with its Diataxis category, a content directive, and source provenance.

---

## `/index.md`
Landing page with persona-oriented entry points (not just topic boxes). Cards for: "Evaluate", "Install", "Deploy a cluster", "Connect your app", "Operate", "Troubleshoot", "Contribute".
**Diataxis**: Navigation. **Source**: Rewrite from `index.md`; current version is topic-oriented, not persona-oriented.

---

## `getting-started/`

### `getting-started/index.md`
Section index. **Diataxis**: Navigation.

### `getting-started/what-is-scylladb-operator.md`
**Diataxis**: Explanation. What the Operator does, what problems it solves vs. manual deployment, single-DC vs. multi-DC model, tech preview status of `ScyllaDBCluster`. Target: Evaluator/Architect.
**Source**: NEW. Addresses the missing "Why ScyllaDB Operator" page ([DOCS_HINTS: Evaluator pain point]).

### `getting-started/quickstart-gke.md`
**Diataxis**: Tutorial. Zero-to-running-cluster on GKE in ≤30 min. Dev-mode, no tuning, minimal cluster (3 nodes). Includes connecting with cqlsh and cleanup.
**Source**: Rewrite from `quickstarts/gke.md`. Fix multi-zone node pool ([#1465](https://github.com/scylladb/scylla-operator/issues/1465)), add minimal dev example ([#641](https://github.com/scylladb/scylla-operator/issues/641)).

### `getting-started/quickstart-eks.md`
**Diataxis**: Tutorial. Same structure as GKE quickstart but for EKS. Replace outdated `i3` instance type ([#2994](https://github.com/scylladb/scylla-operator/issues/2994)).
**Source**: Rewrite from `quickstarts/eks.md`.

### `getting-started/concepts-for-k8s-beginners.md`
**Diataxis**: Explanation. Bridge document: maps ScyllaDB-native concepts to Kubernetes equivalents (node→pod, config file→CR spec, nodetool→kubectl exec, service config→ScyllaCluster YAML). Target: Junior Support Engineers and App Developers new to K8s.
**Source**: NEW. Addresses Junior Support Engineer pain points.

---

## `architecture/`

### `architecture/index.md`
Section index. **Diataxis**: Navigation.

### `architecture/overview.md`
**Diataxis**: Explanation. Components (CRDs, webhooks, controllers), cluster-scoped vs. namespaced, reconciliation model, dependency chain. Include diagrams.
**Source**: Adapt from `architecture/overview.md`. Existing content is solid but needs reconciliation-model explanation and updated diagrams.

### `architecture/storage.md`
**Diataxis**: Explanation. Local vs. network storage tradeoffs, NodeConfig role in disk setup, Local CSI Driver (xfs prjquota, dynamic provisioning), supported provisioners. Merge storage overview and Local CSI Driver into one cohesive page.
**Source**: Merge `architecture/storage/overview.md` + `architecture/storage/local-csi-driver.md`.

### `architecture/tuning.md`
**Diataxis**: Explanation. The two-level performance tuning system and how it optimizes ScyllaDB on Kubernetes. Must cover:
- **Node-level tuning** (`NodePerftune` + `NodeSysctls`): runs once per node when a NodeConfig is created. Executes `perftune` on the host to tune kernel parameters, network device settings, disk devices, and spread IRQs. Applies sysctls. Uses privileged DaemonSets and Jobs in the `scylla-operator-node-tuning` namespace. Does **not** block ScyllaDB startup.
- **Pod/container-level tuning** (`ContainerPerftune` + `ContainerResourceLimits`): runs per ScyllaDB Pod creation/restart. Spreads IRQs across CPUs not used by the specific ScyllaDB container. Adjusts container resource limits. Creates a per-pod tuning ConfigMap (`SidecarRuntimeConfig`) tied to the container instance. **Blocks ScyllaDB startup via ignition** until complete.
- **CPU pinning**: kubelet static CPU manager policy, Guaranteed QoS class requirement (requests == limits), how `perftune` pins IRQs to non-ScyllaDB CPUs.
- **How tuning flows into ignition**: the ignition controller checks that the tuning ConfigMap exists with matching container ID and no blocking NodeConfigs remain before allowing ScyllaDB to start. Cross-reference `architecture/ignition.md`.
- **Key controllers**: NodeConfig controller (orchestrates node-level tuning), NodeConfigPod controller (watches ScyllaDB pods, creates per-pod tuning ConfigMaps).
**Source**: Adapt from `architecture/tuning.md`. Expand with node vs. pod tuning distinction and ignition integration. Add QoS class implications.

### `architecture/manager.md`
**Diataxis**: Explanation. Manager deployment model, task sync (repair/backup), security (shared namespace), limitations.
**Source**: Adapt from `architecture/manager.md`. Extend with end-to-end Manager integration ([#1898](https://github.com/scylladb/scylla-operator/issues/1898)).

### `architecture/networking.md`
**Diataxis**: Explanation. Network exposure model (Headless/ClusterIP/LoadBalancer), broadcast options, dual-stack/IPv6 architecture, platform-specific annotations. Consolidate all networking concepts.
**Source**: Merge `resources/common/exposing.md` + `management/networking/ipv6/concepts/ipv6-networking.md`. Current content is scattered.

### `architecture/monitoring.md`
**Diataxis**: Explanation. ScyllaDBMonitoring components, managed vs. external Prometheus, ServiceMonitor/PrometheusRule, Prometheus Operator dependency.
**Source**: Adapt from `management/monitoring/overview.md`.

### `architecture/bootstrap-sync.md`
**Diataxis**: Explanation. Why bootstrap synchronisation exists, barrier mechanism, ScyllaDBDatacenterNodeStatuses role.
**Source**: Adapt from `management/bootstrap-sync.md`.

### `architecture/automatic-data-cleanup.md`
**Diataxis**: Explanation. Why cleanup happens, trigger mechanism, inspection.
**Source**: Adapt from `management/data-cleanup.md` + `resources/scyllaclusters/nodeoperations/automatic-cleanup.md`.

### `architecture/sidecar.md`
**Diataxis**: Explanation. The containers that make up a ScyllaDB Pod and how they interact. Must cover:
- **Init containers**: the operator binary injector (copies `/usr/bin/scylla-operator` into shared volume), `sysctl-buddy` (applies sysctl settings from annotation), `scylladb-bootstrap-barrier` (blocks startup until bootstrap preconditions are met, feature-gated).
- **Main container**: runs ScyllaDB, but first waits for ignition (`/mnt/shared/ignition.done` file), then execs the sidecar binary which configures and starts ScyllaDB as a subprocess.
- **Sidecar containers**: `scylladb-api-status-probe` (readiness/liveness probe endpoint), `scylladb-ignition` (evaluates prerequisites — tuning done, IPs assigned — and creates the ignition signal file), ScyllaDB Manager Agent (optional, also waits for ignition before starting).
- **The `sidecar` subcommand**: runs inside the main container; configures `scylla.yaml`, snitch properties, IO properties, seeds, CPU pinning; resolves broadcast addresses and rack/DC placement; starts ScyllaDB as a subprocess and manages its lifecycle; runs the SidecarController (syncs member service annotations with HostID, token ring hash, handles decommission) and a StatusReporter.
- How containers coordinate through the shared `/mnt/shared` volume and the ignition signal file.
- Cross-reference `architecture/ignition.md` for the startup gating mechanism.
Target: Database Operator/SRE, Support Engineer, Contributor.
**Source**: NEW. No existing page explains the pod anatomy; users must reverse-engineer it from YAML manifests and source code.

### `architecture/ignition.md`
**Diataxis**: Explanation. The startup gating mechanism that prevents ScyllaDB from starting before all prerequisites are met. Must cover:
- **Why ignition exists**: ScyllaDB must not start until node tuning is complete, network identity is assigned, and the container is ready. Starting prematurely causes performance issues or incorrect configuration.
- **Signal-file mechanism**: the ignition controller creates `/mnt/shared/ignition.done`; the ScyllaDB container polls for this file in a shell loop before exec-ing into the sidecar process. The Manager Agent also waits on the same file.
- **Prerequisites evaluated**: (1) identity Service has a LoadBalancer ingress (if using LB broadcast), (2) Pod has an IP, (3) ScyllaDB container ID is set, (4) a tuning ConfigMap exists for the pod with matching container ID and no blocking NodeConfigs remain.
- **Cleanup on shutdown**: the `preStop` hook removes the signal file so a restarting container must be re-ignited.
- **Force override**: the `internal.scylla-operator.scylladb.com/ignition-override` annotation can force ignition to `"true"` or `"false"` for debugging.
- **Readiness probe**: ignition container exposes `/readyz` on port 42081, returning 200 only when ignited.
- Cross-reference `architecture/sidecar.md` (pod containers) and `architecture/tuning.md` (why tuning must complete before ignition).
Target: Database Operator/SRE, Support Engineer, Contributor.
**Source**: NEW. Ignition is a critical internal mechanism that is completely undocumented.

### `architecture/pod-disruption-budgets.md`
**Diataxis**: Explanation. How the Operator uses Kubernetes PodDisruptionBudgets (PDBs) to protect ScyllaDB availability during voluntary disruptions. Must cover:
- **What a PDB is**: Kubernetes mechanism that limits how many pods can be voluntarily evicted simultaneously (node drain, cluster autoscaler, maintenance).
- **ScyllaDB cluster PDB** (per-datacenter): created with `maxUnavailable: 1`, ensuring at most one ScyllaDB node is disrupted at a time. Selector excludes cleanup job pods so they don't count toward PDB calculations.
- **Operator and webhook server PDBs**: `minAvailable: 1` PDBs that protect the operator deployment and webhook server when running with multiple replicas.
- **PDB interaction with operations**: how PDBs cooperate with scale-down (one member decommissioned at a time), rolling upgrades (partition-based rollout, one pod at a time), node replacement (one node replaced at a time), and Kubernetes node drains.
- **Common pitfall**: PDBs can block node drains if the cluster is already degraded (e.g., a node is already down); explain how to diagnose and work around this.
- Cross-reference `architecture/statefulsets-and-racks.md` (rolling updates) and `operating/scaling.md`.
Target: Platform/K8s Admin, Database Operator/SRE.
**Source**: NEW. PDBs are created automatically but never explained; users encounter them during node drains and get confused.

### `architecture/security.md`
**Diataxis**: Explanation. TLS certificates (automatic via cert-manager), authentication/authorization model, RBAC, network policies, ScyllaDB Manager shared-namespace security. Target: Evaluator/Architect, Platform Admin.
**Source**: NEW. Addresses missing security documentation ([DOCS_HINTS]).

### `architecture/statefulsets-and-racks.md` (DONE)
**Diataxis**: Explanation. How the Operator maps ScyllaDB topology onto Kubernetes primitives. Must cover:
- Each ScyllaDB **rack** is backed by a **StatefulSet** — one StatefulSet per rack, with ordered pod naming (`<cluster>-<dc>-<rack>-<ordinal>`).
- **Pod identity and ordering** — StatefulSet guarantees stable network identity and persistent storage per pod; ordinal determines startup/shutdown order.
- **Scaling** — adding nodes appends pods at the end of the StatefulSet; removing nodes removes from the end. Implications: you cannot remove an arbitrary node in the middle without replacing it first.
- **Rolling updates** — StatefulSet `updateStrategy` controls how config/image changes propagate; pods are updated in reverse ordinal order. A stuck rollout (e.g., new pod won't become Ready) blocks the remaining pods from updating.
- **Operating on a node in the middle** — how to replace a specific node (by ordinal), what happens to its PVC, why maintenance mode exists, and how the Operator handles a pod that is not the head or tail of the set.
- **Partition rollouts** — using StatefulSet partition to control how far a rolling update progresses; useful for canary-style rollouts or unsticking a blocked update.
- Cross-reference to `operating/scaling.md`, `operating/replacing-nodes.md`, and `troubleshooting/changing-log-level.md`.
Target: Database Operator/SRE, Support Engineer.
**Source**: NEW. No existing page explains this; users must infer StatefulSet behavior from Kubernetes docs.

---

## `installation/`

### `installation/index.md` (DONE)
Section index. **Diataxis**: Navigation.

### `installation/prerequisites.md` (DONE)
**Diataxis**: Reference. Kubernetes version requirements, kubelet static CPU policy, node labels, xfsprogs, platform-specific notes (GKE, EKS, OpenShift), firewall rules, resource requirements.
**Source**: Rewrite from `installation/kubernetes-prerequisites.md`. Fix outdated K8s version references ([#2820](https://github.com/scylladb/scylla-operator/issues/2820)). Unify dependency syntax ([#3063](https://github.com/scylladb/scylla-operator/issues/3063)).

### `installation/gitops.md` (DONE)
**Diataxis**: How-to. Step-by-step GitOps/manifest installation: cert-manager → Prometheus Operator → ScyllaDB Operator → Local CSI Driver → NodeConfig → ScyllaDB Manager → ScyllaDBMonitoring. Each step has verification and expected output. End with a callout linking to `deploying/production-checklist.md` for production-hardening follow-up. [#2916]
**Source**: Rewrite from `installation/gitops.md`. Fix template parameter issues ([#2477](https://github.com/scylladb/scylla-operator/issues/2477), [#2626](https://github.com/scylladb/scylla-operator/issues/2626)).

### `installation/helm.md` (DONE)
**Diataxis**: How-to. Step-by-step Helm installation. Include Local CSI Driver step ([#2567](https://github.com/scylladb/scylla-operator/issues/2567)). CRD update warning. **Must include NodeConfig setup step** (currently only in GitOps instructions; Helm path omits it). Link to `deploying/production-checklist.md` as an optional production-hardening follow-up. [#2916 comment]
**Source**: Rewrite from `installation/helm.md`.

### `installation/multi-dc-infrastructure.md` (DONE)
**Diataxis**: How-to. Setting up multi-cluster networking for multi-DC deployments. Platform examples for GKE and EKS. RemoteKubernetesCluster setup, inter-cluster connectivity.
**Source**: Merge `resources/common/multidc/gke.md` + `resources/common/multidc/eks.md` + parts of `resources/scylladbclusters/scylladbclusters.md`. Address [#2855](https://github.com/scylladb/scylla-operator/issues/2855).

---

## `deploying/`

### `deploying/index.md` (DONE)
Section index. **Diataxis**: Navigation.

### `deploying/single-dc-cluster.md` (DONE)
**Diataxis**: How-to. Creating a ScyllaCluster: config files, rack definitions, zone spreading, resource requests, storage, authentication. Include minimal and production-ready examples.
**Source**: Rewrite from `resources/scyllaclusters/basics.md`. Fix container count, deprecated flags, wrong paths ([#2080](https://github.com/scylladb/scylla-operator/issues/2080)). Add deployment topology guidance ([#1605](https://github.com/scylladb/scylla-operator/issues/1605)).

### `deploying/multi-dc-cluster.md` (DONE)
**Diataxis**: How-to. Creating a ScyllaDBCluster across multiple Kubernetes clusters. Prerequisites, RemoteKubernetesCluster, authentication, bootstrap sync.
**Source**: Rewrite from `resources/scylladbclusters/scylladbclusters.md`. Tech preview callout.

### `deploying/node-configuration.md` (DONE)
**Diataxis**: How-to. Setting up NodeConfig: RAID, filesystem, mount points, sysctls, performance tuning. Must cover:
- **Explain the value of NodeConfig** — why it exists, what it configures, why production setups need it. [#2916 req 1]
- **Configuring taints/tolerations on NodeConfig** so that performance tuning targets the same K8s nodes where ScyllaDB Pods run. Explicit worked example showing matching selectors between NodeConfig and ScyllaCluster. [#2916 req 3]
- **`RLIMIT_NOFILE` (`fs.nr_open`)** — how to set max open files limit for ScyllaDB pods via sysctl. [#2937](https://github.com/scylladb/scylla-operator/issues/2937) [#2916 req 4]
- **XFS online discard (trimming)** — recommend enabling `discard` mount option for SSD space reclamation. Document using `unsupportedOptions` if the default doesn't enable it, or show the NodeConfig field if it does. Explain why online discard is preferred over periodic fstrim. [#3013](https://github.com/scylladb/scylla-operator/issues/3013) [#2916 req 9]
**Source**: Merge `resources/nodeconfigs.md` + `management/sysctls.md`. New content for XFS trimming and value explanation.

### `deploying/dedicated-node-pools.md` (DONE)
**Diataxis**: How-to. Configuring ScyllaDB to run on dedicated Kubernetes node pools. Covers:
- Creating a dedicated node pool with labels and taints (GKE, EKS, generic examples).
- Setting `nodeAffinity` and `tolerations` on ScyllaCluster / ScyllaDBCluster so that ScyllaDB Pods schedule only on dedicated nodes.
- Setting matching `nodeSelector` / `tolerations` on NodeConfig so that performance tuning and disk setup target the same dedicated nodes.
- Clearly recommended pattern: label + taint the pool, tolerate + require in both ScyllaCluster and NodeConfig.
[#2916 req 2, req 3]
**Source**: NEW. Currently no single page covers this cross-cutting concern; affinity/toleration guidance is scattered.

### `deploying/operator-configuration.md` (DONE)
**Diataxis**: How-to. Configuring ScyllaOperatorConfig: auxiliary images, tuning images, enterprise settings.
**Source**: Adapt from `resources/scyllaoperatorconfigs.md`.

### `deploying/monitoring.md` (DONE)
**Diataxis**: How-to. End-to-end monitoring setup: Prometheus, ScyllaDBMonitoring, Grafana. Multi-cluster monitoring. Fix missing namespace, inconsistent command formatting ([#2080](https://github.com/scylladb/scylla-operator/issues/2080)). Remove legacy monitoring references ([#2407](https://github.com/scylladb/scylla-operator/issues/2407)). Referenced from `deploying/production-checklist.md` as a production requirement. [#2916 req 6]
**Source**: Rewrite from `management/monitoring/setup.md`.

### `deploying/cpu-pinning.md` (DONE)
**Diataxis**: How-to. Configuring CPU pinning for ScyllaDB on Kubernetes. Covers:
- Kubelet `static` CPU manager policy prerequisite.
- Setting `Guaranteed` QoS class (requests == limits) on ScyllaCluster.
- Verifying CPU pinning is active (checking cgroup assignments, perftune output).
- Common pitfalls: wrong QoS class, kubelet not configured, sidecar containers breaking pinning.
- A single, straight recommendation instead of scattered fragments.
[#2916 req 8]
**Source**: NEW. Consolidates scattered CPU pinning guidance from `architecture/tuning.md`, `installation/kubernetes-prerequisites.md`, and `resources/scyllaclusters/basics.md` into one actionable how-to.

### `deploying/exposing-grafana.md` (DONE)
**Diataxis**: How-to. Ingress for Grafana with HAProxy example.
**Source**: Adapt from `management/monitoring/exposing-grafana.md`.

### `deploying/production-checklist.md` (DONE)
**Diataxis**: Reference. Actionable checklist with a row per item, expected state, and link to the how-to that covers it. Items:
1. **NodeConfig deployed** — explain the value; link to `deploying/node-configuration.md`. [#2916 req 1]
2. **Dedicated node pool** — ScyllaDB runs on isolated nodes with taints/tolerations; link to `deploying/dedicated-node-pools.md`. [#2916 req 2]
3. **Performance tuning co-location** — NodeConfig targets the same nodes as ScyllaDB Pods; link to `deploying/dedicated-node-pools.md`. [#2916 req 3]
4. **`RLIMIT_NOFILE` set** — `fs.nr_open` sysctl configured via NodeConfig; link to `deploying/node-configuration.md`. [#2937](https://github.com/scylladb/scylla-operator/issues/2937) [#2916 req 4]
5. **Coredumps enabled** — systemd-coredump configured with sufficient backing storage; link to `troubleshooting/coredumps.md`. [#2414](https://github.com/scylladb/scylla-operator/issues/2414) [#2916 req 5]
6. **Monitoring deployed** — ScyllaDBMonitoring with Prometheus and Grafana; link to `deploying/monitoring.md`. [#2916 req 6]
7. **Resource requests/limits set** — explicit values for both ScyllaDB container and ScyllaDB Manager Agent sidecar; document that ScyllaDB derives its memory allocation from the container limit. Link to `reference/sizing-guide.md`. [#2916 req 7]
8. **CPU pinning active** — kubelet static CPU policy, Guaranteed QoS, verified pinning; link to `deploying/cpu-pinning.md`. [#2916 req 8]
9. **XFS online discard enabled** — `discard` mount option on ScyllaDB data volumes for SSD trimming; link to `deploying/node-configuration.md`. [#3013](https://github.com/scylladb/scylla-operator/issues/3013) [#2916 req 9]
10. **io_properties configured** — precomputed I/O properties for consistent performance; link to `operating/configuring-io-properties.md`. [#2205](https://github.com/scylladb/scylla-operator/issues/2205)
11. **Backups scheduled** — ScyllaDB Manager backup tasks with object storage IAM; link to `operating/backup-and-restore.md`.

 TODO: cross-check this list against [`scylla_setup.py`](https://github.com/scylladb/scylladb/blob/fc37518affc84e62ce2455f3a9dd580485b3de68/dist/common/scripts/scylla_setup) to ensure no Kubernetes-applicable setting is omitted. [#2916 req 10]
**Source**: NEW. Addresses [#2916](https://github.com/scylladb/scylla-operator/issues/2916).

---

## `connecting/`

### `connecting/index.md` (DONE)
Section index. **Diataxis**: Navigation.

### `connecting/cql.md` (DONE)
**Diataxis**: How-to. Connecting via CQL: authentication setup, embedded cqlsh, remote cqlsh with TLS, credentials, podman/docker access. Include driver configuration tips (contact points, local DC, load balancing, retry).
**Source**: Rewrite from `resources/scyllaclusters/clients/cql.md`. Add driver configuration guidance.

### `connecting/alternator.md` (DONE)
**Diataxis**: How-to. Enabling and using Alternator DynamoDB-compatible API with AWS CLI/SDK.
**Source**: Adapt from `resources/scyllaclusters/clients/alternator.md`.

### `connecting/discovery.md` (DONE)
**Diataxis**: How-to. Using the discovery endpoint: ClusterIP, DNS, LoadBalancer exposure.
**Source**: Adapt from `resources/scyllaclusters/clients/discovery.md`.

### `connecting/external-access.md` (DONE)
**Diataxis**: How-to. Connecting from outside the Kubernetes cluster. LoadBalancer setup, TLS certificates for external clients, firewall rules.
**Source**: NEW. Combines scattered external access information. Target: App Developer.

---

## `operating/`

### `operating/index.md` (DONE)
Section index. **Diataxis**: Navigation.

### `operating/scaling.md` (DONE)
**Diataxis**: How-to. Scaling clusters up/down: adding racks, changing replica count. Include multi-DC scaling ([#2859](https://github.com/scylladb/scylla-operator/issues/2859)). Explain that nodes are added/removed at the end of each rack's StatefulSet; cross-reference `architecture/statefulsets-and-racks.md` for why arbitrary mid-set removal is not possible.
**Source**: NEW. Addresses [#2426](https://github.com/scylladb/scylla-operator/issues/2426) and [#2450](https://github.com/scylladb/scylla-operator/issues/2450).

### `operating/upgrading-operator.md` (DONE)
**Diataxis**: How-to. Upgrading ScyllaDB Operator: GitOps and Helm paths, version-specific steps, rollback. Include "Review Release Notes" callout ([#2513](https://github.com/scylladb/scylla-operator/issues/2513)).
**Source**: Rewrite from `management/upgrading/upgrade.md`.

### `operating/upgrading-scylladb.md` (DONE)
**Diataxis**: How-to. Upgrading ScyllaDB versions for ScyllaCluster and ScyllaDBCluster. Wait for rollout, verify health.
**Source**: Rewrite from `management/upgrading/upgrade-scylladb.md` + `resources/scyllaclusters/nodeoperations/scylla-upgrade.md`.

### `operating/upgrading-kubernetes.md` (DONE)
**Diataxis**: How-to. Upgrading GKE/EKS Kubernetes control plane and node pools while respecting PDBs.
**Source**: NEW. Addresses [#2446](https://github.com/scylladb/scylla-operator/issues/2446), [#2381](https://github.com/scylladb/scylla-operator/issues/2381).

### `operating/replacing-nodes.md` (DONE)
**Diataxis**: How-to. Replacing dead nodes: verify status, drain, trigger replacement, repair. Include `ignore_dead_nodes_for_replace` ([#2184](https://github.com/scylladb/scylla-operator/issues/2184)). Multi-DC node replacement ([#2861](https://github.com/scylladb/scylla-operator/issues/2861)). When the replace itself fails, cross-reference `troubleshooting/recovering-from-failed-replace.md`.
**Source**: Rewrite from `resources/scyllaclusters/nodeoperations/replace-node.md`.

### `operating/volume-expansion.md` (DONE)
**Diataxis**: How-to. Expanding storage: orphan delete flow, PVC patching, verification.
**Source**: Adapt from `resources/scyllaclusters/nodeoperations/volume-expansion.md`. Address [#2450](https://github.com/scylladb/scylla-operator/issues/2450).

### `operating/maintenance-mode.md` (DONE)
**Diataxis**: How-to. Enabling/disabling maintenance mode via service labels.
**Source**: Adapt from `resources/scyllaclusters/nodeoperations/maintenance-mode.md`.

### `operating/backup-and-restore.md` (YOU ARE HERE)
**Diataxis**: How-to. End-to-end: scheduling backups via ScyllaDB Manager tasks, configuring object storage (IAM roles for EKS/GKE — [#1697](https://github.com/scylladb/scylla-operator/issues/1697)), listing snapshots, restoring schema and tables to a new cluster.
**Source**: Rewrite from `resources/scyllaclusters/nodeoperations/restore.md`. Extend with backup scheduling and IAM configuration.

### `operating/rolling-restart.md`
**Diataxis**: How-to. Forcing rolling restarts for ScyllaCluster and ScyllaDBCluster.
**Source**: Extract from `resources/scyllaclusters/basics.md` and `resources/scylladbclusters/scylladbclusters.md`.

### `operating/migrating-rack-to-new-node-pool.md`
**Diataxis**: How-to. Migrating a ScyllaDB rack from one Kubernetes node pool to another without downtime. Step-by-step procedure:
1. **Create a new rack** in the ScyllaCluster/ScyllaDBCluster spec that targets the new node pool via `nodeSelector` and `tolerations`. Set the new rack's `members` to `0` initially.
2. **Gradually scale up the new rack** — increase `members` by 1 at a time. Each new node joins the cluster, takes ownership of a token range, and begins streaming data.
3. **Gradually scale down the old rack** — decrease `members` by 1 at a time. Each removed node is decommissioned (tokens redistributed, data streamed away) before the pod is deleted.
4. **Maintain balance** — keep the total node count stable during migration by scaling up the new rack before scaling down the old rack. Verify data distribution with `nodetool status` after each step.
5. **Delete the old rack** — once the old rack has 0 members and all data has been streamed away, remove the old rack definition from the spec.
- **Prerequisites**: the new node pool must already exist with appropriate labels, taints, instance types, and NodeConfig. Cross-reference `deploying/dedicated-node-pools.md`.
- **Caveats**: replication factor must accommodate the temporary rack imbalance; run repair after migration completes; ensure the new rack is in the same datacenter.
- Cross-reference `architecture/statefulsets-and-racks.md` (scaling mechanics), `operating/scaling.md`.
Target: Platform/K8s Admin, Database Operator/SRE.
**Source**: NEW. This is a common operational procedure with no existing documentation.

### `operating/passing-scylladb-arguments.md`
**Diataxis**: How-to. Passing additional ScyllaDB arguments through the ScyllaCluster / ScyllaDBCluster spec. Cover both v1 and v1alpha1 APIs. Explain the normal path where a spec change triggers a rolling restart.
For **emergency scenarios** where a rolling restart is not possible (stuck rollout, degraded cluster), cross-reference `troubleshooting/changing-log-level.md`.
**Source**: NEW. Addresses [#3199](https://github.com/scylladb/scylla-operator/issues/3199).

### `operating/configuring-io-properties.md`
**Diataxis**: How-to. Setting up precomputed `io_properties` for ScyllaDB.
**Source**: NEW. Addresses [#2205](https://github.com/scylladb/scylla-operator/issues/2205).

---

## `networking/`

### `networking/index.md`
Section index. **Diataxis**: Navigation.

### `networking/exposing-clusters.md`
**Diataxis**: How-to. Configuring `exposeOptions`: node service types, broadcast options, platform-specific annotations.
**Source**: Rewrite from `resources/common/exposing.md` (procedural parts only; conceptual parts move to `architecture/networking.md`).

### `networking/ipv6-getting-started.md`
**Diataxis**: Tutorial. First IPv6-enabled cluster step-by-step.
**Source**: Adapt from `management/networking/ipv6/tutorials/ipv6-getting-started.md`.

### `networking/ipv6-configure-dual-stack.md`
**Diataxis**: How-to. Dual-stack setup (IPv4-first and IPv6-first variants).
**Source**: Merge `management/networking/ipv6/how-to/ipv6-configure.md` + `management/networking/ipv6/how-to/ipv6-configure-ipv6-first.md`.

### `networking/ipv6-configure-single-stack.md`
**Diataxis**: How-to. IPv6-only single-stack setup.
**Source**: Adapt from `management/networking/ipv6/how-to/ipv6-configure-ipv6-only.md`.

### `networking/ipv6-migration.md`
**Diataxis**: How-to. Migrating from IPv4 to IPv6.
**Source**: Adapt from `management/networking/ipv6/how-to/ipv6-migrate.md`.

### `networking/ipv6-troubleshooting.md`
**Diataxis**: How-to. Diagnosing IPv6 connectivity issues.
**Source**: Adapt from `management/networking/ipv6/how-to/ipv6-troubleshoot.md`.

---

## `troubleshooting/`

### `troubleshooting/index.md`
Section index with symptom-based navigation: "Pods not starting", "Cluster degraded", "Connection failures", "Upgrade failures", "Installation issues". **Diataxis**: Navigation.

### `troubleshooting/diagnostic-flowchart.md`
**Diataxis**: How-to. Top-level decision tree: Is this a Kubernetes issue, Operator issue, or ScyllaDB issue? Branch to specific symptom pages. Copy-paste-ready diagnostic commands.
**Source**: NEW. Addresses Support Engineer and Junior Support Engineer personas.

### `troubleshooting/installation.md`
**Diataxis**: How-to. Debugging installation failures: webhook connectivity, CRD propagation, cert-manager issues.
**Source**: Rewrite from `support/troubleshooting/installation.md`.

### `troubleshooting/cluster-health.md`
**Diataxis**: How-to. Checking cluster health: pod status, ScyllaCluster conditions, nodetool status, aggregated conditions semantics ([#3022](https://github.com/scylladb/scylla-operator/issues/3022)).
**Source**: NEW.

### `troubleshooting/investigating-restarts.md`
**Diataxis**: How-to. Investigating the cause of ScyllaDB pod/container restarts. Must cover:
- **Identifying that a restart occurred**: `kubectl get pods` showing non-zero restart count, `kubectl describe pod` showing `lastState.terminated` with exit code and reason (OOMKilled, Error, etc.), container creation timestamp vs. pod creation timestamp discrepancy.
- **Restart count and reason**: reading `containerStatuses[].restartCount`, `containerStatuses[].lastState.terminated.reason`, `containerStatuses[].lastState.terminated.exitCode`, and `containerStatuses[].lastState.terminated.finishedAt` to determine when and why the container restarted.
- **Pod events**: `kubectl describe pod` events section — look for `Killing`, `BackOff`, `OOMKilling`, `FailedScheduling`, `Unhealthy` (liveness probe failure) events.
- **Distinguishing restart causes**: OOMKilled, liveness probe failure (ScyllaDB unresponsive — check logs for long GC pauses, compaction stalls), CrashLoopBackOff (ScyllaDB fails to start — check previous container logs), node eviction (check node conditions and events).
- **Collecting evidence**: previous container logs (`kubectl logs --previous`), must-gather archive, pod YAML with full status. Cross-reference `troubleshooting/collecting-debugging-information/must-gather.md`.
Target: Database Operator/SRE, Support Engineer.
**Source**: NEW.

### `troubleshooting/node-not-starting.md`
**Diataxis**: How-to. Diagnosing and resolving a ScyllaDB node that is not starting up. Must cover:
- **Identifying the situation**: pod stuck in `Pending` (scheduling failure), `Init:*` (init container not completing), `PodInitializing` (ignition not firing), `CrashLoopBackOff` (ScyllaDB crashing on startup), or `Running` but not `Ready` (readiness probe failing).
- **Pending pods**: check events for `FailedScheduling` — insufficient resources, node affinity/taint mismatch, PVC binding failure. Verify dedicated node pool exists and selectors match.
- **Init container stuck**: check which init container is running (`kubectl describe pod`). If `scylladb-bootstrap-barrier` — bootstrap sync hasn't completed. If `sysctl-buddy` — sysctl application failing.
- **Ignition not completing**: pod is `Running` but ScyllaDB hasn't started. Check ignition sidecar logs, verify NodeConfig tuning has completed (check tuning ConfigMap), verify Service has LoadBalancer IP (if using LB broadcast). Cross-reference `architecture/ignition.md`.
- **CrashLoopBackOff**: check previous container logs for ScyllaDB startup errors. Common causes: corrupt sstables, invalid configuration, wrong seeds, disk permission issues.
- **Ways to recover**: recreate the pod (`kubectl delete pod`), clear PVC data (data loss — last resort), check and fix NodeConfig, adjust resource requests, fix node pool selectors.
- Cross-reference `troubleshooting/investigating-restarts.md` and `architecture/sidecar.md`.
Target: Database Operator/SRE, Support Engineer.
**Source**: NEW.

### `troubleshooting/changing-log-level.md`
**Diataxis**: How-to. Changing the ScyllaDB log level on a live cluster when a normal rolling restart is not feasible. Must cover:
- **Scenario 1: StatefulSet stuck mid-rollout** — a rolling update is blocked because the updated pod won't become Ready. How to change log level on the already-running (old) pods via the ScyllaDB REST API (`kubectl exec` + `curl`/`scylla api` endpoint) without triggering another rollout.
- **Scenario 2: Degraded cluster** — one or more nodes are down and a full rolling restart would violate availability. Same REST API approach to change log level on the healthy pods.
- **Scenario 3: Normal path** — when a rolling restart is acceptable, change the log level via ScyllaCluster spec (cross-reference `operating/passing-scylladb-arguments.md`).
- Explain which ScyllaDB REST API endpoint controls log levels, how to verify the change took effect, and that REST API changes are ephemeral (lost on pod restart).
- Cross-reference `architecture/statefulsets-and-racks.md` to explain why a stuck rollout blocks further updates and what "partition" means in this context.
Target: Database Operator/SRE, Support Engineer.
**Source**: NEW.

### `troubleshooting/recovering-from-failed-replace.md`
**Diataxis**: How-to. Step-by-step fallback procedure when a node replace operation fails and leaves the cluster in an inconsistent state (the Operator is stuck and cannot apply further configuration changes). Must cover:
- Verifying via `nodetool status` that a node is stuck (DN, ?N) and correlating with Operator/pod logs.
- Backing up data and capturing a must-gather archive before any destructive action.
- **Pausing the Operator** — scaling `scylla-operator` deployment to 0 replicas while keeping ScyllaDB instances running. Note current replica count for later restore.
- **Orphan-deleting the rack's StatefulSet** — `kubectl delete statefulset --cascade=orphan` to prevent pod recreation while Operator is down. Warning box about the consequences of forgetting `--cascade=orphan`.
- Determining Host IDs of the culprit node and any ghost members (cross-reference ScyllaDB [failed membership change guide](https://docs.scylladb.com/manual/branch-2025.1/operating-scylla/procedures/cluster-management/handling-membership-change-failures.html)).
- **Stopping the culprit node** — deleting its Pod, PVC (data loss), and Service. Explain that deleting the Service signals the Operator to provision a brand-new node instead of retrying the replace.
- **Removing ghost members** — `nodetool removenode` for the culprit and any ghost Host IDs; verify with `nodetool status`.
- **Resuming the Operator** — scaling back to the original replica count. Operator recreates the StatefulSet, Service, Pod, and PVC for a net-new node.
- Verification: new node joins as UN; run repair.
- Multi-DC note: perform the same steps in the worker cluster where the failed node is located.
- Failure modes and recovery: what to do if the user accidentally cascading-deletes the StatefulSet (PVCs survive, Operator recreates pods on scale-up); how to manually recreate the StatefulSet from must-gather if the Operator fails to start.
Cross-reference `architecture/statefulsets-and-racks.md` (why orphan-deleting a StatefulSet is safe) and `operating/replacing-nodes.md` (normal replace path).
**Source**: NEW. Based on `enhancements/proposals/2955-failed-replace-recovery/failed-replace-recovery.md`.

### `troubleshooting/performance.md`
**Diataxis**: How-to. Diagnosing performance degradation: CPU pinning verification, tuning misconfiguration, storage bottlenecks.
**Source**: NEW.

### `troubleshooting/common-errors.md`
**Diataxis**: Reference. Table of common error messages/log patterns, Kubernetes failure states (CrashLoopBackOff, Pending, ImagePullBackOff) mapped to ScyllaDB context, root causes, and resolution links.
**Source**: NEW. Addresses Junior Support Engineer persona.

### `troubleshooting/collecting-debugging-information/index.md`
Section index for debugging data collection methods. **Diataxis**: Navigation.

### `troubleshooting/collecting-debugging-information/must-gather.md`
**Diataxis**: How-to. Collecting Kubernetes-level diagnostics with the embedded must-gather tool: podman/docker/minikube, namespace scoping, include/exclude resources. Attach to support tickets.
**Source**: Rewrite from `support/must-gather.md`. Cover minikube ([#1628](https://github.com/scylladb/scylla-operator/issues/1628)) and Podman ([#2384](https://github.com/scylladb/scylla-operator/issues/2384)).

### `troubleshooting/collecting-debugging-information/must-gather-contents.md`
**Diataxis**: Reference. Explains the per-node "special" files captured by must-gather and how to interpret them. Must cover:
- **`nodetool-status.log`** — cluster topology: which nodes are up/down/joining/leaving, load, host IDs, token ownership, datacenter/rack placement. How to read it for diagnosing membership issues.
- **`nodetool-gossipinfo.log`** — Gossip protocol state per node: heartbeat generation/version, status, schema version, host ID, tokens, RPC/listen addresses, ScyllaDB version. Useful for diagnosing split-brain, schema disagreements, or nodes stuck in abnormal gossip states.
- **`df.log`** (`df -h`) — disk space usage for all mounted filesystems inside the container. Critical for diagnosing disk-full conditions, verifying data/commitlog volume sizes.
- **`io_properties.yaml`** — ScyllaDB I/O properties configuration (disk bandwidth and IOPS parameters). Verify that I/O tuning was applied correctly.
- **`scylla-rlimits.log`** (`prlimit --pid=$(pidof scylla)`) — OS resource limits of the running ScyllaDB process: open files (`NOFILE`), address space, locked memory, core dump size. Important for debugging limit-related errors (e.g., "too many open files").
- **`kubelet-cpu_manager_state.log`** (NodeConfig pods only) — kubelet CPU Manager state showing which CPUs are exclusively assigned to which containers. Critical for verifying CPU pinning.
- **Pod logs** — current, previous, and terminated container logs for every init/regular/ephemeral container. Explain `--log-bytes` size limiting.
- **stderr files** — command stderr is captured separately (e.g., `nodetool-status.log.stderr`).
- **Output directory structure** — how files are organized under `namespaces/<ns>/pods/<pod-name>/`.
Target: Support Engineer, Database Operator/SRE.
**Source**: NEW. Users receive must-gather archives but have no guide to interpreting the contents.

### `troubleshooting/collecting-debugging-information/system-tables.md`
**Diataxis**: How-to. Querying ScyllaDB system tables for debugging information. Must cover:
- Which system tables are most useful for diagnostics (`system.log`, `system_distributed.repair_history`, `system.sstable_activity`, `system.large_partitions`, `system.runtime_info`, etc.).
- How to query them via `cqlsh` on a running pod (`kubectl exec`).
- When to use system tables vs. must-gather (system tables capture ScyllaDB-internal state that must-gather does not).
- Including system table output when filing support tickets.
Target: Database Operator/SRE, Support Engineer.
**Source**: NEW.

### `troubleshooting/coredumps.md`
**Diataxis**: How-to. Configuring systemd-coredump on Kubernetes. Must cover:
- systemd-coredump setup on nodes (via NodeConfig or host-level configuration).
- **Backing storage sizing** — ensure enough disk space to accommodate a full ScyllaDB process coredump (proportional to memory allocation). [#2414](https://github.com/scylladb/scylla-operator/issues/2414) [#2916 req 5]
- Collecting coredumps from nodes.
- Hint at off-the-shelf tooling (e.g., IBM/core-dump-handler).
**Source**: NEW. Addresses [#2917](https://github.com/scylladb/scylla-operator/issues/2917) and [#2414](https://github.com/scylladb/scylla-operator/issues/2414).

---

## `reference/`

### `reference/index.md`
Section index. **Diataxis**: Navigation.

### `reference/api/` (directory)
**Diataxis**: Reference. Auto-generated API reference for all CRDs. Keep existing generation pipeline.
**Source**: Keep `reference/api/` as-is (auto-generated).

### `reference/feature-gates.md`
**Diataxis**: Reference. Feature gates table with descriptions and configuration (GitOps/Helm).
**Source**: Adapt from `reference/feature-gates.md`.

### `reference/ipv6-configuration.md`
**Diataxis**: Reference. Complete IPv6 API field reference.
**Source**: Adapt from `management/networking/ipv6/reference/ipv6-configuration.md`.

### `reference/releases.md`
**Diataxis**: Reference. Release schedule, supported releases, backport policy, support matrix (K8s, OpenShift, ScyllaDB versions, architectures, environments).
**Source**: Adapt from `support/releases.md`. Update product naming ([#3066](https://github.com/scylladb/scylla-operator/issues/3066)).

### `reference/known-issues.md`
**Diataxis**: Reference. Known issues with workarounds, platform-specific caveats, multi-DC limitations ([#2856](https://github.com/scylladb/scylla-operator/issues/2856)).
**Source**: Rewrite from `support/known-issues.md`. Expand beyond minikube-only issues.

### `reference/sizing-guide.md`
**Diataxis**: Reference. Instance types per platform, storage sizing, network bandwidth. Must include:
- **Recommended resource requests/limits for ScyllaDB container** — how memory limit maps to ScyllaDB's memory allocation. [#2916 req 7]
- **Recommended resource requests/limits for ScyllaDB Manager Agent sidecar** — separate guidance since it shares the pod. [#2916 req 7]
- Why requests must equal limits for Guaranteed QoS (prerequisite for CPU pinning). [#2916 req 8]
**Source**: NEW. Addresses [#2349](https://github.com/scylladb/scylla-operator/issues/2349) and [#2916 req 7].

### `reference/benchmarking.md`
**Diataxis**: Reference. How to benchmark ScyllaDB on Kubernetes, cassandra-stress setup, interpreting results.
**Source**: NEW. Addresses [#2365](https://github.com/scylladb/scylla-operator/issues/2365).

### `reference/glossary.md`
**Diataxis**: Reference. Terminology: ScyllaCluster, ScyllaDBCluster, ScyllaDBDatacenter, rack, datacenter, ScyllaDB Manager, shard, etc. Map Kubernetes terms to ScyllaDB terms.
**Source**: NEW. Addresses consistent terminology requirement.

### `reference/nodetool-alternatives.md`
**Diataxis**: Reference. For users proficient with [ScyllaDB's `nodetool`](https://docs.scylladb.com/manual/stable/operating-scylla/nodetool.html): why direct `nodetool` operations that change cluster state must not be used with the Operator, and what to do instead. Note that ScyllaDB's `nodetool` is a distinct implementation from Apache Cassandra's — it communicates with ScyllaDB via the REST API (not JMX), though many command names are the same.

Must cover each state-changing `nodetool` command with its Operator-compatible alternative:

| `nodetool` command | Risk with Operator | Operator alternative |
|---|---|---|
| [`decommission`](https://docs.scylladb.com/manual/stable/operating-scylla/nodetool-commands/decommission.html) | **High.** Desyncs StatefulSet replica count and Operator tracking labels. Operator won't know the node was decommissioned. | Scale down the rack's `members` count by 1. The Operator labels the service, the sidecar calls decommission, and the StatefulSet scales down after completion. |
| [`removenode`](https://docs.scylladb.com/manual/stable/operating-scylla/nodetool-commands/removenode.html) | **High.** Removes a node from the ring without Operator knowledge. Operator expects to manage membership via scale-down or replace. | Use the `scylla/replace` label on the member Service to trigger Operator-managed replacement. For dead nodes, see `troubleshooting/recovering-from-failed-replace.md`. |
| [`repair`](https://docs.scylladb.com/manual/stable/operating-scylla/nodetool-commands/repair.html) | **Low.** Safe to run manually but redundant. | Configure repair tasks via ScyllaDB Manager (managed through the ScyllaCluster spec). Manager handles scheduling and distributed coordination. |
| [`cleanup`](https://docs.scylladb.com/manual/stable/operating-scylla/nodetool-commands/cleanup.html) | **Low.** Safe and idempotent, but the Operator runs cleanup automatically when it detects token ring changes. | Automatic — the Operator tracks token ring hash changes per service and spawns cleanup Jobs when they diverge. Manual cleanup is only needed after replication factor changes (which the Operator does not detect). |
| [`drain`](https://docs.scylladb.com/manual/stable/operating-scylla/nodetool-commands/drain.html) | **Medium.** Puts node in DRAINED state; sidecar may restart ScyllaDB if a decommission is pending. | Drain runs automatically as a `preStop` lifecycle hook before every pod shutdown. No manual invocation needed. |
| `move` | **High.** Changes token ownership without Operator awareness. | Not supported. Use scaling (add/remove nodes) to rebalance. |
| [`rebuild`](https://docs.scylladb.com/manual/stable/operating-scylla/nodetool-commands/rebuild.html) | **High.** Streams data from another datacenter; Operator does not track rebuild state. Running during an Operator-managed operation can cause conflicts. | No Operator alternative. If rebuild is needed (e.g., adding a new DC), coordinate manually — pause Operator reconciliation or ensure no other operations are in progress. |
| [`disablebinary`](https://docs.scylladb.com/manual/stable/operating-scylla/nodetool-commands/disablebinary.html) / [`enablebinary`](https://docs.scylladb.com/manual/stable/operating-scylla/nodetool-commands/enablebinary.html) | **High.** Disabling native transport makes the node unreachable to clients and may cause readiness probe failures, triggering Operator recovery actions. | Do not use. If you need to stop traffic to a node, use maintenance mode (`operating/maintenance-mode.md`). |
| [`disablegossip`](https://docs.scylladb.com/manual/stable/operating-scylla/nodetool-commands/disablegossip.html) / [`enablegossip`](https://docs.scylladb.com/manual/stable/operating-scylla/nodetool-commands/enablegossip.html) | **High.** Disabling gossip effectively marks the node as down in the cluster, confusing the Operator's health checks and potentially triggering unwanted recovery. | Do not use. |
| [`snapshot`](https://docs.scylladb.com/manual/stable/operating-scylla/nodetool-commands/snapshot.html) / [`clearsnapshot`](https://docs.scylladb.com/manual/stable/operating-scylla/nodetool-commands/clearsnapshot.html) | **Low.** Safe to use. | Snapshots are taken automatically before rolling updates. For backup, use ScyllaDB Manager backup tasks. |
| [`compact`](https://docs.scylladb.com/manual/stable/operating-scylla/nodetool-commands/compact.html) | **Low.** Safe to trigger manually. | No Operator alternative; manual compaction is acceptable. |
| [`flush`](https://docs.scylladb.com/manual/stable/operating-scylla/nodetool-commands/flush.html) | **Low.** Safe to trigger manually. | No Operator alternative; flushing memtables is acceptable. |
| [`scrub`](https://docs.scylladb.com/manual/stable/operating-scylla/nodetool-commands/scrub.html) | **Low.** Safe to trigger manually. | No Operator alternative; manual scrub is acceptable. |
| [`setlogginglevel`](https://docs.scylladb.com/manual/stable/operating-scylla/nodetool-commands/setlogginglevel.html) | **Low.** Ephemeral change, lost on pod restart. Safe to use for live debugging. | Preferred method for emergency log-level changes when a rolling restart is not possible. Cross-reference `troubleshooting/changing-log-level.md`. For persistent changes, use ScyllaCluster spec (`operating/passing-scylladb-arguments.md`). |
| [`disableautocompaction`](https://docs.scylladb.com/manual/stable/operating-scylla/nodetool-commands/disableautocompaction.html) / [`enableautocompaction`](https://docs.scylladb.com/manual/stable/operating-scylla/nodetool-commands/enableautocompaction.html) | **Medium.** Ephemeral change, but disabling auto-compaction can cause unbounded growth of SSTables if forgotten. Operator does not track this state. | No Operator alternative; use with caution and re-enable promptly. |
| [`refresh`](https://docs.scylladb.com/manual/stable/operating-scylla/nodetool-commands/refresh.html) | **Low.** Loads SSTables placed on disk; does not affect cluster membership. | No Operator alternative; safe to use. |
| [`upgradesstables`](https://docs.scylladb.com/manual/stable/operating-scylla/nodetool-commands/upgradesstables.html) | **Low.** Rewrites SSTables to the latest format; safe but resource-intensive. | No Operator alternative; safe to use, typically needed after ScyllaDB version upgrade. |
| [`setcompactionthroughput`](https://docs.scylladb.com/manual/stable/operating-scylla/nodetool-commands/setcompactionthroughput.html) / [`setstreamthroughput`](https://docs.scylladb.com/manual/stable/operating-scylla/nodetool-commands/setstreamthroughput.html) | **Low.** Ephemeral tuning changes, lost on restart. | No Operator alternative; safe to use for temporary tuning. For persistent changes, configure via ScyllaCluster spec. |
| [`settraceprobability`](https://docs.scylladb.com/manual/stable/operating-scylla/nodetool-commands/settraceprobability.html) | **Low.** Ephemeral diagnostic setting. | No Operator alternative; safe to use for debugging. |
| [`stop`](https://docs.scylladb.com/manual/stable/operating-scylla/nodetool-commands/stop.html) (compaction) | **Medium.** Stops in-progress compaction. Safe but may need to be repeated if compaction restarts. | No Operator alternative; use with caution. |
- **General rule**: if a `nodetool` command is read-only (e.g., [`status`](https://docs.scylladb.com/manual/stable/operating-scylla/nodetool-commands/status.html), [`gossipinfo`](https://docs.scylladb.com/manual/stable/operating-scylla/nodetool-commands/gossipinfo.html), [`info`](https://docs.scylladb.com/manual/stable/operating-scylla/nodetool-commands/info.html), [`ring`](https://docs.scylladb.com/manual/stable/operating-scylla/nodetool-commands/ring.html), [`cfstats`](https://docs.scylladb.com/manual/stable/operating-scylla/nodetool-commands/cfstats.html), [`tablestats`](https://docs.scylladb.com/manual/stable/operating-scylla/nodetool-commands/tablestats.html), [`compactionstats`](https://docs.scylladb.com/manual/stable/operating-scylla/nodetool-commands/compactionstats.html), [`toppartitions`](https://docs.scylladb.com/manual/stable/operating-scylla/nodetool-commands/toppartitions.html)), it is always safe to use. If it changes cluster state, consult this table.
- Cross-reference `architecture/statefulsets-and-racks.md`, `operating/scaling.md`, `operating/replacing-nodes.md`.
Target: Database Operator/SRE who are experienced ScyllaDB administrators transitioning to Kubernetes.
**Source**: NEW.

---

## `contributing/`

### `contributing/index.md`
Section index. **Diataxis**: Navigation.

### `contributing/development-setup.md`
**Diataxis**: Tutorial. Setting up local dev environment: Go, Docker/Podman, Kind, dependencies. Build and run the Operator locally.
**Source**: NEW. Link to `CONTRIBUTING.md`. Addresses [#3278](https://github.com/scylladb/scylla-operator/issues/3278).

### `contributing/building-and-testing.md`
**Diataxis**: How-to. Makefile targets, running unit/integration/E2E tests, interpreting CI failures.
**Source**: NEW. Link to `CONTRIBUTING.md`.

### `contributing/api-conventions.md`
**Diataxis**: Reference. API conventions for adding/modifying CRD fields.
**Source**: Link to `API_CONVENTIONS.md`.

---

## Files to delete (not carried forward)

The following existing files are fully superseded by the new structure and should be removed:

- `quickstarts/` → replaced by `getting-started/quickstart-*.md`
- `architecture/storage/` (2 files) → merged into `architecture/storage.md`
- `installation/overview.md` → content split between `installation/prerequisites.md` and `architecture/overview.md`
- `installation/kubernetes-prerequisites.md` → replaced by `installation/prerequisites.md`
- `management/` (entire tree) → content redistributed to `operating/`, `deploying/`, `networking/`, `architecture/`
- `resources/` (entire tree) → content redistributed to `deploying/`, `connecting/`, `operating/`, `reference/`
- `support/` → content redistributed to `troubleshooting/`, `reference/`
- All existing `index.md` files → replaced by new section indexes- **Why this matters**: the Operator reconciles cluster state continuously. Any out-of-band change to cluster membership or topology creates a mismatch between the Operator's expected state and reality, leading to stuck rollouts, failed replacements, or data loss.
