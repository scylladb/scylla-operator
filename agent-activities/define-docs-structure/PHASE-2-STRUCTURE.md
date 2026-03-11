# Phase 2: The Golden Standard Documentation Structure

## Design Principles

This structure is a **clean-slate ideal** -- it represents what the ScyllaDB Operator documentation *should* look like, unconstrained by the current layout. It is designed around three pillars:

1. **Diátaxis Framework:** Every section is annotated with its quadrant type -- `[Tutorial]`, `[How-to Guide]`, `[Explanation]`, or `[Reference]`. Content that mixes types is explicitly split.
2. **User Journey (Day 0/1/2):** The structure flows from "first contact" through "production deployment" to "ongoing operations," matching how real users adopt and operate ScyllaDB on Kubernetes.
3. **Domain Specificity:** Distributed database concerns (racks, AZs, repairs, compaction, multi-DC, storage tuning) are first-class citizens, not afterthoughts.

### Constraints respected
- **CRD API Reference pages are auto-generated** and their internal structure is not modified. The proposed structure references them as-is.
- **Both ScyllaCluster and ScyllaDBCluster** are kept as co-equal resource types, organized within their respective sections.

### Primary inspiration
- **Crunchy PGO** (Diátaxis: 4.5): Quickstart -> Tutorials (Day lifecycle) -> Guides (task-specific) -> Architecture -> Reference
- Supplemented by features stolen from CockroachDB, K8ssandra, StackGres, CloudNativePG, Cert-Manager, and Gateway API (see annotations below)

---

## Proposed Table of Contents

```
ScyllaDB Operator Documentation
├── 1. Get Started
│   ├── Quickstart: Local Cluster (Kind/Minikube)
│   ├── Quickstart: GKE
│   └── Quickstart: EKS
│
├── 2. Concepts
│   ├── Architecture Overview
│   │   ├── The Operator Pattern and Reconciliation Loop
│   │   └── CRD Ecosystem
│   ├── ScyllaDB on Kubernetes
│   │   ├── Why Run ScyllaDB on Kubernetes?
│   │   ├── Racks, Availability Zones, and Topology
│   │   └── Storage: Local NVMe vs Network-Attached
│   ├── ScyllaDB Manager Integration
│   ├── Networking Model
│   │   ├── Service Types and Exposure
│   │   ├── IPv6 Networking
│   │   └── Multi-Datacenter Connectivity
│   ├── Performance and Tuning Principles
│   └── Failure Modes
│
├── 3. Installation
│   ├── Prerequisites
│   │   ├── Kubernetes Platform Requirements
│   │   └── Node Configuration (kernel, sysctls, CPU pinning)
│   ├── Install with Helm
│   ├── Install with GitOps (kubectl)
│   ├── Install on OpenShift
│   └── Verify Your Installation
│
├── 4. Tutorials
│   ├── Day 0: Your First ScyllaDB Cluster
│   │   ├── Create a 3-Node Cluster
│   │   ├── Connect a CQL Client
│   │   └── Connect with Alternator (DynamoDB API)
│   ├── Day 1: Production Configuration
│   │   ├── Configure Storage and Performance Tuning
│   │   ├── Set Up Monitoring (ScyllaDB Monitoring Stack)
│   │   ├── Configure Backups
│   │   ├── Secure Your Cluster (TLS, Authentication)
│   │   ├── Expose Your Cluster to External Clients
│   │   └── Production Checklist
│   │       ├── Hardware and Node Sizing
│   │       ├── Storage Recommendations
│   │       ├── Kernel and OS Tuning
│   │       ├── Network Configuration
│   │       ├── Monitoring and Alerting
│   │       ├── Backup Strategy
│   │       ├── High Availability Checklist
│   │       └── Security Hardening
│   └── Day 2: Operations Lifecycle
│       ├── Scale Your Cluster (Add/Remove Nodes)
│       ├── Upgrade ScyllaDB Version
│       ├── Upgrade ScyllaDB Operator
│       ├── Perform a Backup and Restore
│       └── Deploy Multi-Datacenter
│
├── 5. Operations Guides
│   ├── Scaling
│   │   ├── Add/Remove Racks
│   │   ├── Horizontal Scale-Out/In
│   │   └── Volume Expansion (Resize Storage)
│   ├── Upgrading
│   │   ├── Upgrade ScyllaDB Operator
│   │   ├── Upgrade ScyllaDB Version
│   │   └── Rolling Update Strategy
│   ├── Backup and Restore
│   │   ├── Configure Backup Schedules (via ScyllaDB Manager)
│   │   └── Restore from Backup
│   ├── Networking
│   │   ├── Expose Clusters via LoadBalancer/NodePort/Ingress
│   │   ├── Configure IPv6 Networking
│   │   └── Migrate Clusters to IPv6
│   ├── Multi-Datacenter
│   │   ├── Build Multi-DC on GKE
│   │   ├── Build Multi-DC on EKS
│   │   └── Deploy Multi-DC ScyllaDB Cluster
│   ├── Monitoring
│   │   ├── Set Up ScyllaDB Monitoring
│   │   ├── Expose Grafana
│   │   └── External Prometheus on OpenShift
│   ├── Security
│   │   ├── TLS Configuration
│   │   └── Authentication and Authorization
│   ├── Node Operations
│   │   ├── Replace a Failed Node
│   │   ├── Automatic Cleanup on K8s Node Loss
│   │   ├── Maintenance Mode
│   │   └── Bootstrap Synchronization
│   └── Data Management
│       ├── Automatic Data Cleanup
│       ├── Repair Scheduling (via ScyllaDB Manager)
│       └── Compaction Strategy Configuration
│
├── 6. Troubleshooting
│   ├── Installation Issues
│   ├── Cluster Health Diagnostics
│   ├── Common Error Messages
│   ├── Gathering Data with must-gather
│   └── Known Issues
│
├── 7. Reference
│   ├── API Reference (auto-generated, unchanged)
│   │   └── scylla.scylladb.com
│   │       ├── NodeConfig
│   │       ├── RemoteKubernetesCluster
│   │       ├── RemoteOwner
│   │       ├── ScyllaCluster
│   │       ├── ScyllaDBCluster
│   │       ├── ScyllaDBDatacenter
│   │       ├── ScyllaDBDatacenterNodesStatusReport
│   │       ├── ScyllaDBManagerClusterRegistration
│   │       ├── ScyllaDBManagerTask
│   │       ├── ScyllaDBMonitoring
│   │       └── ScyllaOperatorConfig
│   ├── Helm Chart Values
│   ├── Feature Gates
│   ├── Configuration Reference
│   │   ├── ScyllaOperatorConfig Options
│   │   └── Kernel Parameters (sysctls)
│   └── Compatibility Matrix
│       ├── Supported Kubernetes Versions
│       ├── Supported ScyllaDB Versions
│       └── Supported Platforms (GKE, EKS, OpenShift)
│
├── 8. Release Notes
│   ├── Releases
│   └── Migration Guides
│
└── 9. Support
    ├── Support Overview
    └── Contact and Community
```

---

## Detailed Section Descriptions

### 1. Get Started `[Tutorial]`

**Source of Inspiration:** Crunchy PGO's single-page Quickstart; K8ssandra's persona-based quickstarts

**Purpose:** Get a user from zero to a running ScyllaDB cluster in under 15 minutes. These are *not* production-ready setups -- they are learning environments.

| Page | Description | Content Status |
|---|---|---|
| Quickstart: Local Cluster (Kind/Minikube) | Minimal prerequisites, single command installs, 3-node cluster on a laptop | **NEW** -- does not exist today |
| Quickstart: GKE | Production-path quickstart on Google Cloud | EXISTS as `quickstarts/gke.md` |
| Quickstart: EKS | Production-path quickstart on AWS | EXISTS as `quickstarts/eks.md` |

**Critical change from current docs:** Quickstarts move from position 5 in the sidebar to position 1. This is the single most impactful navigation change. A local Kind/Minikube quickstart is added because not every user has a cloud account on day one.

---

### 2. Concepts `[Explanation]`

**Source of Inspiration:** CloudNativePG's "Before You Start" and "Failure Modes"; Gateway API's Concepts section; StackGres's "Features" section

**Purpose:** Build understanding. No step-by-step instructions here -- only mental models, architecture diagrams, and design rationale. Users read these pages to understand *why* things work the way they do.

| Page | Description | Content Status |
|---|---|---|
| Architecture Overview | The operator pattern, controller reconciliation, webhook architecture | EXISTS (partially) as `architecture/overview.md` |
| The Operator Pattern and Reconciliation Loop | How the operator watches CRDs and converges state | EXISTS (partially) in `architecture/overview.md` |
| CRD Ecosystem | Overview of all CRDs and how they relate to each other | **NEW** -- currently scattered across `resources/overview.md` |
| Why Run ScyllaDB on Kubernetes? | Business and technical rationale | **NEW** |
| Racks, Availability Zones, and Topology | How ScyllaDB's rack-aware replication maps to K8s topology | **NEW** -- critical for a distributed database |
| Storage: Local NVMe vs Network-Attached | Storage trade-offs, performance implications, CSI driver role | EXISTS as `architecture/storage/overview.md` and `architecture/storage/local-csi-driver.md` |
| ScyllaDB Manager Integration | What Manager does, how it integrates with the operator | EXISTS as `architecture/manager.md` |
| Service Types and Exposure | How ScyllaDB services are exposed in K8s | EXISTS (partially) in `resources/common/exposing.md` |
| IPv6 Networking | IPv6 concepts and dual-stack architecture | EXISTS as `management/networking/ipv6/concepts/ipv6-networking.md` |
| Multi-Datacenter Connectivity | Network architecture for multi-DC deployments | **NEW** -- currently buried in how-to guides |
| Performance and Tuning Principles | Tuning philosophy, what the operator automates vs. what users configure | EXISTS as `architecture/tuning.md` |
| Failure Modes | What happens when nodes fail, disks die, network partitions occur, and how the operator responds | **NEW** -- inspired by CloudNativePG |

---

### 3. Installation `[How-to Guide]`

**Source of Inspiration:** Cert-Manager's numbered step 0; Crunchy PGO's Installation section

**Purpose:** Get the operator installed correctly. Separate from quickstarts (which are about running ScyllaDB clusters) and tutorials (which are about learning).

| Page | Description | Content Status |
|---|---|---|
| Prerequisites: Kubernetes Platform Requirements | K8s version, RBAC, CRD requirements | EXISTS as `installation/kubernetes-prerequisites.md` |
| Prerequisites: Node Configuration | Kernel params, sysctls, CPU pinning, RAID setup | EXISTS (partially) in `management/sysctls.md` and `architecture/tuning.md` |
| Install with Helm | Helm chart installation steps | EXISTS as `installation/helm.md` |
| Install with GitOps (kubectl) | Manifest-based installation | EXISTS as `installation/gitops.md` |
| Install on OpenShift | OpenShift-specific installation | EXISTS as `installation/openshift.md` |
| Verify Your Installation | Post-install health checks, verify operator is running, check CRDs | **NEW** -- currently users have no verification steps |

---

### 4. Tutorials `[Tutorial]`

**Source of Inspiration:** Crunchy PGO's Day-0/1/2 lifecycle tutorials; Cert-Manager's numbered progression; CockroachDB's Production Checklist (as Day-1 subsection)

**Purpose:** Progressive learning paths that build on each other. Unlike Guides (which are standalone), Tutorials assume the user completed the previous step. They are organized around the operational lifecycle: Day 0 (try), Day 1 (configure for production), Day 2 (operate and maintain).

#### Day 0: Your First ScyllaDB Cluster

| Page | Description | Content Status |
|---|---|---|
| Create a 3-Node Cluster | Deploy a basic ScyllaCluster/ScyllaDBCluster resource | EXISTS (partially) in `resources/scyllaclusters/basics.md` and `resources/scylladbclusters/scylladbclusters.md` |
| Connect a CQL Client | Service discovery, connecting from within and outside K8s | EXISTS as `resources/scyllaclusters/clients/cql.md` and `resources/scyllaclusters/clients/discovery.md` |
| Connect with Alternator (DynamoDB API) | Set up and use the DynamoDB-compatible API | EXISTS as `resources/scyllaclusters/clients/alternator.md` |

#### Day 1: Production Configuration

| Page | Description | Content Status |
|---|---|---|
| Configure Storage and Performance Tuning | Local NVMe, storage classes, tuning parameters | EXISTS (partially) across `architecture/tuning.md` and `architecture/storage/` |
| Set Up Monitoring | Deploy ScyllaDB Monitoring stack, configure dashboards | EXISTS as `management/monitoring/setup.md` |
| Configure Backups | Set up ScyllaDB Manager backup schedules | **NEW** -- backup configuration is currently sparse |
| Secure Your Cluster | TLS, authentication, RBAC | **NEW** -- security documentation does not exist |
| Expose Your Cluster to External Clients | LoadBalancer, NodePort, Ingress configuration | EXISTS as `resources/common/exposing.md` |
| **Production Checklist** | Single authoritative page for go-live validation | **NEW** -- inspired by CockroachDB |

The **Production Checklist** is a comprehensive verification document with subsections:
- Hardware and Node Sizing (CPU, RAM, disk ratios for ScyllaDB workloads)
- Storage Recommendations (local NVMe preferred, filesystem, RAID, volume sizing)
- Kernel and OS Tuning (sysctls, CPU governor, IRQ balancing, huge pages)
- Network Configuration (inter-node latency, MTU, DNS, service mesh considerations)
- Monitoring and Alerting (which metrics to alert on, dashboard setup verification)
- Backup Strategy (RPO/RTO targets, backup schedule validation, restore testing)
- High Availability Checklist (rack-aware topology, replication factor, pod disruption budgets)
- Security Hardening (TLS enforcement, network policies, RBAC, image scanning)

#### Day 2: Operations Lifecycle

| Page | Description | Content Status |
|---|---|---|
| Scale Your Cluster | Add/remove nodes, expand racks | EXISTS (partially) -- scattered across resource docs |
| Upgrade ScyllaDB Version | Rolling upgrade of ScyllaDB itself | EXISTS as `management/upgrading/upgrade-scylladb.md` |
| Upgrade ScyllaDB Operator | Operator upgrade procedure | EXISTS as `management/upgrading/upgrade.md` |
| Perform a Backup and Restore | End-to-end backup and restore walkthrough | EXISTS (partially) as `resources/scyllaclusters/nodeoperations/restore.md` |
| Deploy Multi-Datacenter | End-to-end multi-DC deployment tutorial | EXISTS (partially) across `resources/scyllaclusters/multidc/` |

---

### 5. Operations Guides `[How-to Guide]`

**Source of Inspiration:** K8ssandra's task-verb structure; StackGres's Runbooks; Crunchy PGO's Guides section

**Purpose:** Standalone how-to guides for specific operational tasks. Unlike Tutorials, these do not build on each other -- a user arrives with a specific need ("I need to resize storage") and follows the guide to completion. Organized by task domain with verb-oriented titles.

#### Scaling

| Page | Description | Content Status |
|---|---|---|
| Add/Remove Racks | Modify rack topology in a running cluster | **NEW** |
| Horizontal Scale-Out/In | Add or remove nodes within existing racks | EXISTS (partially) in resource docs |
| Volume Expansion (Resize Storage) | Expand PVC storage for running clusters | EXISTS as `resources/scyllaclusters/nodeoperations/volume-expansion.md` |

#### Upgrading

| Page | Description | Content Status |
|---|---|---|
| Upgrade ScyllaDB Operator | Step-by-step operator upgrade | EXISTS as `management/upgrading/upgrade.md` |
| Upgrade ScyllaDB Version | Rolling ScyllaDB upgrade | EXISTS as `management/upgrading/upgrade-scylladb.md` and `resources/scyllaclusters/nodeoperations/scylla-upgrade.md` |
| Rolling Update Strategy | How rolling updates work, pod disruption budgets, tuning update speed | **NEW** |

#### Backup and Restore

| Page | Description | Content Status |
|---|---|---|
| Configure Backup Schedules | Set up automated backups via ScyllaDB Manager | **NEW** |
| Restore from Backup | Full restore procedure | EXISTS as `resources/scyllaclusters/nodeoperations/restore.md` |

#### Networking

| Page | Description | Content Status |
|---|---|---|
| Expose Clusters via LoadBalancer/NodePort/Ingress | External access configuration | EXISTS as `resources/common/exposing.md` |
| Configure IPv6 Networking | Dual-stack and IPv6-only setup | EXISTS as `management/networking/ipv6/how-to/ipv6-configure.md` and related pages |
| Migrate Clusters to IPv6 | Online migration to IPv6 | EXISTS as `management/networking/ipv6/how-to/ipv6-migrate.md` |

#### Multi-Datacenter

| Page | Description | Content Status |
|---|---|---|
| Build Multi-DC on GKE | GKE-specific multi-cluster networking setup | EXISTS as `resources/common/multidc/gke.md` |
| Build Multi-DC on EKS | EKS-specific multi-cluster networking setup | EXISTS as `resources/common/multidc/eks.md` |
| Deploy Multi-DC ScyllaDB Cluster | Deploy ScyllaDB across prepared multi-DC infrastructure | EXISTS as `resources/scyllaclusters/multidc/multidc.md` |

#### Monitoring

| Page | Description | Content Status |
|---|---|---|
| Set Up ScyllaDB Monitoring | Deploy monitoring stack | EXISTS as `management/monitoring/setup.md` |
| Expose Grafana | External access to Grafana dashboards | EXISTS as `management/monitoring/exposing-grafana.md` |
| External Prometheus on OpenShift | OpenShift-specific monitoring integration | EXISTS as `management/monitoring/external-prometheus-on-openshift.md` |

#### Security

| Page | Description | Content Status |
|---|---|---|
| TLS Configuration | Enable and configure TLS for client and inter-node communication | **NEW** |
| Authentication and Authorization | User management, RBAC, authentication mechanisms | **NEW** |

#### Node Operations

| Page | Description | Content Status |
|---|---|---|
| Replace a Failed Node | Manual node replacement procedure | EXISTS as `resources/scyllaclusters/nodeoperations/replace-node.md` |
| Automatic Cleanup on K8s Node Loss | How the operator handles lost Kubernetes nodes | EXISTS as `resources/scyllaclusters/nodeoperations/automatic-cleanup.md` |
| Maintenance Mode | Put nodes into maintenance mode for safe operations | EXISTS as `resources/scyllaclusters/nodeoperations/maintenance-mode.md` |
| Bootstrap Synchronization | Coordinate bootstrap operations in ScyllaDB clusters | EXISTS as `management/bootstrap-sync.md` |

#### Data Management

| Page | Description | Content Status |
|---|---|---|
| Automatic Data Cleanup | Configure automatic data cleanup after topology changes | EXISTS as `management/data-cleanup.md` |
| Repair Scheduling (via ScyllaDB Manager) | Set up and monitor anti-entropy repairs | **NEW** -- critical for distributed databases |
| Compaction Strategy Configuration | Configure and tune compaction strategies | **NEW** -- critical for ScyllaDB performance |

---

### 6. Troubleshooting `[How-to Guide]`

**Source of Inspiration:** CloudNativePG's troubleshooting; K8ssandra's troubleshoot task; Cert-Manager's Troubleshooting & FAQ

**Purpose:** Help users diagnose and resolve problems. Promoted to a top-level section (currently buried under Support with one page).

| Page | Description | Content Status |
|---|---|---|
| Installation Issues | Common installation failures and fixes | EXISTS as `support/troubleshooting/installation.md` |
| Cluster Health Diagnostics | How to assess cluster health, interpret status conditions | **NEW** |
| Common Error Messages | Searchable reference of error messages and resolutions | **NEW** |
| Gathering Data with must-gather | Collect diagnostic data for support | EXISTS as `support/must-gather.md` |
| Known Issues | Active known issues and workarounds | EXISTS as `support/known-issues.md` |

---

### 7. Reference `[Reference]`

**Source of Inspiration:** Crunchy PGO's References; Cert-Manager's Reference section; Gateway API's API Types

**Purpose:** Pure lookup material. No narrative, no step-by-step instructions -- just facts, specifications, and configuration options.

| Section | Description | Content Status |
|---|---|---|
| **API Reference** (auto-generated) | All CRD specifications -- **unchanged from current auto-generation pipeline** | EXISTS as `reference/api/` |
| Helm Chart Values | Complete Helm chart configuration reference | **NEW** |
| Feature Gates | Feature gate reference | EXISTS as `reference/feature-gates.md` |
| ScyllaOperatorConfig Options | Full configuration reference for operator settings | EXISTS (partially) as `resources/scyllaoperatorconfigs.md` |
| Kernel Parameters (sysctls) | Reference table of all kernel parameters the operator configures | EXISTS (partially) in `management/sysctls.md` |
| Compatibility Matrix | Supported versions of K8s, ScyllaDB, and platforms | **NEW** -- critical for production planning |

**Note on API Reference:** The CRD API reference pages under `reference/api/groups/scylla.scylladb.com/` are auto-generated and their internal structure is preserved exactly as-is. The proposed structure simply positions them within the broader Reference section.

---

### 8. Release Notes `[Reference]`

**Source of Inspiration:** Crunchy PGO's Release Notes; Cert-Manager's Releases section

| Page | Description | Content Status |
|---|---|---|
| Releases | Version history and changelogs | EXISTS as `support/releases.md` |
| Migration Guides | Version-to-version migration instructions | **NEW** -- currently no migration guides exist |

---

### 9. Support `[Reference]`

**Source of Inspiration:** Crunchy PGO's Support; Cert-Manager's Support page

| Page | Description | Content Status |
|---|---|---|
| Support Overview | Support channels, SLAs, enterprise support | EXISTS as `support/overview.md` |
| Contact and Community | Slack, GitHub, mailing lists, community resources | **NEW** (or merged from existing support overview) |

---

## Key Structural Differences from Current Documentation

| Aspect | Current | Proposed | Rationale |
|---|---|---|---|
| **Quickstart position** | 5th in sidebar (after Resources) | 1st in sidebar | User journey: first contact should be frictionless |
| **Tutorial section** | Does not exist | Day 0/1/2 progressive learning | Users need guided paths, not just reference pages |
| **Content organization** | By resource type (ScyllaClusters, ScyllaDBClusters) | By user task (Scale, Backup, Secure, Monitor) | Users think in tasks, not API objects |
| **Production Checklist** | Does not exist | Subsection of Tutorial Day 1 | CockroachDB proves this is the most-visited page for production users |
| **Troubleshooting** | 1 page buried under Support | Top-level section with 5 pages | Problems are urgent; discoverability matters |
| **Resources section** | Top-level section mixing tutorials, how-to, and reference | Eliminated -- content redistributed by Diátaxis type | The "Resources" concept is an API organization, not a user need |
| **Security docs** | Do not exist | Operations Guide section + Tutorial Day 1 | Security is a production requirement, not optional |
| **Failure modes** | Not documented | Concepts section | Distributed databases fail in complex ways; users need to understand them |
| **Concepts section** | "Architecture" with 4 pages | "Concepts" with 12 pages covering architecture, storage, networking, tuning, failure | Understanding is a distinct need from doing |
| **Repair/compaction** | Not documented | Operations Guide: Data Management | Fundamental distributed database operations |

---

## Content Audit: What Exists vs. What's New

### Existing content to relocate (28 pages)

These pages exist today and would be reorganized into the new structure:

- `quickstarts/gke.md` -> Get Started / Quickstart: GKE
- `quickstarts/eks.md` -> Get Started / Quickstart: EKS
- `architecture/overview.md` -> Concepts / Architecture Overview
- `architecture/storage/overview.md` -> Concepts / Storage: Local NVMe vs Network-Attached
- `architecture/storage/local-csi-driver.md` -> Concepts / Storage (merged)
- `architecture/tuning.md` -> Concepts / Performance and Tuning Principles
- `architecture/manager.md` -> Concepts / ScyllaDB Manager Integration
- `installation/kubernetes-prerequisites.md` -> Installation / Prerequisites
- `installation/helm.md` -> Installation / Install with Helm
- `installation/gitops.md` -> Installation / Install with GitOps
- `installation/openshift.md` -> Installation / Install on OpenShift
- `resources/scyllaclusters/basics.md` -> Tutorials Day 0 / Create a 3-Node Cluster
- `resources/scyllaclusters/clients/cql.md` -> Tutorials Day 0 / Connect a CQL Client
- `resources/scyllaclusters/clients/alternator.md` -> Tutorials Day 0 / Connect with Alternator
- `resources/scyllaclusters/clients/discovery.md` -> Tutorials Day 0 / Connect a CQL Client (merged)
- `management/upgrading/upgrade.md` -> Operations Guides / Upgrading / Upgrade Operator
- `management/upgrading/upgrade-scylladb.md` -> Operations Guides / Upgrading / Upgrade ScyllaDB Version
- `resources/scyllaclusters/nodeoperations/volume-expansion.md` -> Operations Guides / Scaling / Volume Expansion
- `resources/scyllaclusters/nodeoperations/replace-node.md` -> Operations Guides / Node Operations / Replace a Failed Node
- `resources/scyllaclusters/nodeoperations/automatic-cleanup.md` -> Operations Guides / Node Operations / Automatic Cleanup
- `resources/scyllaclusters/nodeoperations/maintenance-mode.md` -> Operations Guides / Node Operations / Maintenance Mode
- `resources/scyllaclusters/nodeoperations/restore.md` -> Operations Guides / Backup and Restore / Restore from Backup
- `resources/common/exposing.md` -> Operations Guides / Networking / Expose Clusters
- `resources/common/multidc/gke.md` -> Operations Guides / Multi-Datacenter / Build Multi-DC on GKE
- `resources/common/multidc/eks.md` -> Operations Guides / Multi-Datacenter / Build Multi-DC on EKS
- `resources/scyllaclusters/multidc/multidc.md` -> Operations Guides / Multi-Datacenter / Deploy Multi-DC
- `management/monitoring/setup.md` -> Operations Guides / Monitoring / Set Up ScyllaDB Monitoring
- `management/monitoring/exposing-grafana.md` -> Operations Guides / Monitoring / Expose Grafana
- `management/monitoring/external-prometheus-on-openshift.md` -> Operations Guides / Monitoring / External Prometheus
- `management/networking/ipv6/how-to/ipv6-configure.md` -> Operations Guides / Networking / Configure IPv6
- `management/networking/ipv6/how-to/ipv6-migrate.md` -> Operations Guides / Networking / Migrate to IPv6
- `management/networking/ipv6/concepts/ipv6-networking.md` -> Concepts / IPv6 Networking
- `management/bootstrap-sync.md` -> Operations Guides / Node Operations / Bootstrap Synchronization
- `management/data-cleanup.md` -> Operations Guides / Data Management / Automatic Data Cleanup
- `management/sysctls.md` -> Installation Prerequisites + Reference / Kernel Parameters
- `support/troubleshooting/installation.md` -> Troubleshooting / Installation Issues
- `support/must-gather.md` -> Troubleshooting / Gathering Data with must-gather
- `support/known-issues.md` -> Troubleshooting / Known Issues
- `support/releases.md` -> Release Notes / Releases
- `support/overview.md` -> Support / Support Overview
- `reference/feature-gates.md` -> Reference / Feature Gates
- `reference/api/*` -> Reference / API Reference (unchanged)

### New content to author (15 pages)

These pages do not exist today and represent content gaps:

1. **Quickstart: Local Cluster (Kind/Minikube)** -- critical for first-time users without cloud access
2. **CRD Ecosystem overview** -- how all CRDs relate to each other
3. **Why Run ScyllaDB on Kubernetes?** -- positioning and rationale
4. **Racks, Availability Zones, and Topology** -- distributed database fundamentals
5. **Multi-Datacenter Connectivity** (concept) -- network architecture for multi-DC
6. **Failure Modes** -- what happens when things go wrong
7. **Verify Your Installation** -- post-install validation steps
8. **Production Checklist** -- hardware, storage, tuning, security, monitoring checklist
9. **TLS Configuration** -- security how-to guide
10. **Authentication and Authorization** -- security how-to guide
11. **Repair Scheduling** -- anti-entropy repair operations
12. **Compaction Strategy Configuration** -- performance operations
13. **Cluster Health Diagnostics** -- troubleshooting guide
14. **Common Error Messages** -- error reference
15. **Compatibility Matrix** -- supported version combinations
16. **Helm Chart Values** -- configuration reference
17. **Migration Guides** -- version-to-version upgrade guides
18. **Rolling Update Strategy** -- how rolling updates work
19. **Configure Backup Schedules** -- backup automation via Manager
20. **Add/Remove Racks** -- rack topology changes
