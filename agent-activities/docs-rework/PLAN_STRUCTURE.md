# Stage 1: Structure Bootstrap Plan

## Goal

Create the new 9-section documentation structure aligned with the Diataxis framework and user journey (Day 0/1/2). This stage focuses exclusively on **structure**: directories, file locations, toctrees, index pages, stub pages, and redirects. No content is written or rewritten -- existing content is moved as-is, and new pages are stubs with TODO markers.

## Design Decisions

These decisions were made during planning and apply throughout this stage:

1. **Follow Phase 2 proposal closely** -- the `Resources` section is eliminated; CRD-specific pages are redistributed by Diataxis type.
2. **Separate placeholders for Tutorials and Operations Guides** -- where topics overlap (e.g., "Upgrade ScyllaDB"), both sections get their own page. Tutorials will become progressive walkthroughs; Guides will be standalone task references. Content differentiation happens in Stage 2.
3. **MyST Markdown (.md)** for all new files -- consistent with the majority of existing docs.
4. **Stub files with TODOs** for all new pages -- each stub has a title, Diataxis type annotation, description of intended content, and a `TODO` marker.
5. **Unified pages with tabs** for ScyllaCluster/ScyllaDBCluster -- following the pattern already used in `exposing.md`. Where parallel pages exist, they are merged into a single page during Stage 2 (Stage 1 just moves the primary page).
6. **Move files, not copy** -- existing pages are moved to new locations; old paths get redirects.
7. **Dissolve IPv6 sub-Diataxis** -- IPv6 pages are integrated into their parent sections (Concepts, Operations Guides, Troubleshooting, Reference, Tutorials).

## Target Directory Structure

All paths relative to `docs/source/`.

```
docs/source/
├── index.md                                          # REWRITE (new 9-section toctree + landing page)
├── _static/                                          # UNCHANGED
├── _ext/                                             # UNCHANGED
├── .internal/                                        # UNCHANGED (shared include snippets)
│
├── get-started/                                      # NEW section (was: quickstarts/)
│   ├── index.md                                      # NEW index
│   ├── local.md                                      # NEW STUB (Kind/Minikube quickstart)
│   ├── gke.md                                        # MOVED from quickstarts/gke.md
│   └── eks.md                                        # MOVED from quickstarts/eks.md
│
├── concepts/                                         # NEW section (was: architecture/)
│   ├── index.md                                      # NEW index
│   ├── architecture-overview.md                      # MOVED from architecture/overview.md
│   ├── operator-pattern.md                           # NEW STUB
│   ├── crd-ecosystem.md                              # NEW STUB
│   ├── why-scylladb-on-kubernetes.md                 # NEW STUB
│   ├── racks-zones-topology.md                       # NEW STUB
│   ├── storage.md                                    # MOVED from architecture/storage/overview.md (+ local-csi-driver.md merged in Stage 2)
│   ├── local-csi-driver.md                           # MOVED from architecture/storage/local-csi-driver.md
│   ├── manager-integration.md                        # MOVED from architecture/manager.md
│   ├── networking-model.md                           # NEW STUB (Service Types and Exposure concepts)
│   ├── ipv6-networking.md                            # MOVED from management/networking/ipv6/concepts/ipv6-networking.md
│   ├── multi-datacenter-connectivity.md              # NEW STUB
│   ├── performance-and-tuning.md                     # MOVED from architecture/tuning.md
│   ├── failure-modes.md                              # NEW STUB
│   ├── components-cluster_scoped.svg                 # MOVED from architecture/components-cluster_scoped.svg
│   ├── components-namespaced.svg                     # MOVED from architecture/components-namespaced.svg
│   └── components.odg                                # MOVED from architecture/components.odg
│
├── installation/                                     # EXISTING section (restructured)
│   ├── index.md                                      # REWRITE (new toctree)
│   ├── overview.md                                   # KEEP in place
│   ├── kubernetes-prerequisites.md                   # KEEP in place
│   ├── node-configuration.md                         # NEW STUB (kernel, sysctls, CPU pinning; content from sysctls.md in Stage 2)
│   ├── helm.md                                       # KEEP in place
│   ├── gitops.md                                     # KEEP in place
│   ├── openshift.md                                  # KEEP in place
│   ├── verify-installation.md                        # NEW STUB
│   ├── deploy.odg                                    # KEEP in place
│   └── deploy.svg                                    # KEEP in place
│
├── tutorials/                                        # NEW section
│   ├── index.md                                      # NEW index
│   ├── day0/                                         # Day 0: Your First ScyllaDB Cluster
│   │   ├── index.md                                  # NEW index
│   │   ├── create-cluster.md                         # MOVED from resources/scyllaclusters/basics.md
│   │   ├── connect-cql.md                            # MOVED from resources/scyllaclusters/clients/cql.md
│   │   └── connect-alternator.md                     # MOVED from resources/scyllaclusters/clients/alternator.md
│   ├── day1/                                         # Day 1: Production Configuration
│   │   ├── index.md                                  # NEW index
│   │   ├── configure-storage-tuning.md               # NEW STUB (content from architecture/storage + tuning in Stage 2)
│   │   ├── setup-monitoring.md                       # NEW STUB (references ops guide)
│   │   ├── configure-backups.md                      # NEW STUB
│   │   ├── secure-your-cluster.md                    # NEW STUB
│   │   ├── expose-cluster.md                         # NEW STUB (references ops guide)
│   │   ├── production-checklist.md                   # NEW STUB
│   │   └── ipv6-getting-started.md                   # MOVED from management/networking/ipv6/tutorials/ipv6-getting-started.md
│   └── day2/                                         # Day 2: Operations Lifecycle
│       ├── index.md                                  # NEW index
│       ├── scale-cluster.md                          # NEW STUB
│       ├── upgrade-scylladb.md                       # NEW STUB (tutorial version, distinct from ops guide)
│       ├── upgrade-operator.md                       # NEW STUB (tutorial version, distinct from ops guide)
│       ├── backup-and-restore.md                     # NEW STUB
│       └── deploy-multi-datacenter.md                # NEW STUB
│
├── operations/                                       # NEW section (was: management/ + resources/*/nodeoperations/)
│   ├── index.md                                      # NEW index
│   ├── scaling/
│   │   ├── index.md                                  # NEW index
│   │   ├── add-remove-racks.md                       # NEW STUB
│   │   ├── horizontal-scaling.md                     # NEW STUB
│   │   └── volume-expansion.md                       # MOVED from resources/scyllaclusters/nodeoperations/volume-expansion.md
│   ├── upgrading/
│   │   ├── index.md                                  # NEW index
│   │   ├── upgrade-operator.md                       # MOVED from management/upgrading/upgrade.md
│   │   ├── upgrade-scylladb.md                       # MOVED from management/upgrading/upgrade-scylladb.md
│   │   └── rolling-update-strategy.md                # NEW STUB
│   ├── backup-and-restore/
│   │   ├── index.md                                  # NEW index
│   │   ├── configure-backup-schedules.md             # NEW STUB
│   │   └── restore-from-backup.md                    # MOVED from resources/scyllaclusters/nodeoperations/restore.md
│   ├── networking/
│   │   ├── index.md                                  # NEW index
│   │   ├── expose-clusters.md                        # MOVED from resources/common/exposing.md
│   │   ├── ipv6-configure.md                         # MOVED from management/networking/ipv6/how-to/ipv6-configure.md
│   │   ├── ipv6-configure-ipv6-first.md              # MOVED from management/networking/ipv6/how-to/ipv6-configure-ipv6-first.md
│   │   ├── ipv6-configure-ipv6-only.md               # MOVED from management/networking/ipv6/how-to/ipv6-configure-ipv6-only.md
│   │   ├── ipv6-migrate.md                           # MOVED from management/networking/ipv6/how-to/ipv6-migrate.md
│   │   ├── ipv6-dual-stack-architecture.svg          # MOVED from management/networking/ipv6/ipv6-dual-stack-architecture.svg
│   │   └── exposing-images/                          # MOVED from resources/common/exposing-images/
│   │       ├── clusterip.svg
│   │       ├── loadbalancer.svg
│   │       ├── multivpc.svg
│   │       └── podips.svg
│   ├── multi-datacenter/
│   │   ├── index.md                                  # NEW index
│   │   ├── multi-dc-gke.md                           # MOVED from resources/common/multidc/gke.md
│   │   ├── multi-dc-eks.md                           # MOVED from resources/common/multidc/eks.md
│   │   ├── deploy-multi-dc-cluster.md                # MOVED from resources/scyllaclusters/multidc/multidc.md
│   │   ├── remotekubernetesclusters.md               # MOVED from resources/remotekubernetesclusters.md
│   │   └── remotekubernetesclusters.svg              # MOVED from resources/remotekubernetesclusters.svg
│   ├── monitoring/
│   │   ├── index.md                                  # NEW index
│   │   ├── overview.md                               # MOVED from management/monitoring/overview.md
│   │   ├── setup.md                                  # MOVED from management/monitoring/setup.md
│   │   ├── expose-grafana.md                         # MOVED from management/monitoring/exposing-grafana.md
│   │   ├── external-prometheus-openshift.md          # MOVED from management/monitoring/external-prometheus-on-openshift.md
│   │   └── diagrams/
│   │       └── monitoring-overview.mmd               # MOVED from management/monitoring/diagrams/monitoring-overview.mmd
│   ├── security/
│   │   ├── index.md                                  # NEW index
│   │   ├── tls-configuration.md                      # NEW STUB
│   │   └── authentication-authorization.md           # NEW STUB
│   ├── node-operations/
│   │   ├── index.md                                  # NEW index
│   │   ├── replace-node.md                           # MOVED from resources/scyllaclusters/nodeoperations/replace-node.md
│   │   ├── automatic-cleanup.md                      # MOVED from resources/scyllaclusters/nodeoperations/automatic-cleanup.md
│   │   ├── maintenance-mode.md                       # MOVED from resources/scyllaclusters/nodeoperations/maintenance-mode.md
│   │   └── bootstrap-sync.md                         # MOVED from management/bootstrap-sync.md
│   └── data-management/
│       ├── index.md                                  # NEW index
│       ├── automatic-data-cleanup.md                 # MOVED from management/data-cleanup.md
│       ├── repair-scheduling.md                      # NEW STUB
│       └── compaction-strategy.md                    # NEW STUB
│
├── troubleshooting/                                  # NEW top-level section (was: support/troubleshooting/)
│   ├── index.md                                      # NEW index
│   ├── installation-issues.md                        # MOVED from support/troubleshooting/installation.md
│   ├── cluster-health-diagnostics.md                 # NEW STUB
│   ├── common-error-messages.md                      # NEW STUB
│   ├── must-gather.md                                # MOVED from support/must-gather.md
│   ├── known-issues.md                               # MOVED from support/known-issues.md
│   └── ipv6-troubleshoot.md                          # MOVED from management/networking/ipv6/how-to/ipv6-troubleshoot.md
│
├── reference/                                        # EXISTING section (expanded)
│   ├── index.md                                      # REWRITE (new toctree)
│   ├── api/                                          # UNCHANGED (auto-generated)
│   │   ├── index.rst
│   │   ├── templates/
│   │   └── groups/
│   │       └── scylla.scylladb.com/
│   │           └── ... (all .rst files unchanged)
│   ├── feature-gates.md                              # KEEP in place
│   ├── helm-chart-values.md                          # NEW STUB
│   ├── scyllaoperatorconfig-options.md               # MOVED from resources/scyllaoperatorconfigs.md
│   ├── kernel-parameters.md                          # NEW STUB (content from management/sysctls.md in Stage 2)
│   ├── ipv6-configuration.md                         # MOVED from management/networking/ipv6/reference/ipv6-configuration.md
│   └── compatibility-matrix.md                       # NEW STUB
│
├── release-notes/                                    # NEW section (was part of: support/)
│   ├── index.md                                      # NEW index
│   ├── releases.md                                   # MOVED from support/releases.md
│   └── migration-guides.md                           # NEW STUB
│
└── support/                                          # EXISTING section (slimmed down)
    ├── index.md                                      # REWRITE (new toctree)
    ├── overview.md                                   # KEEP in place
    └── contact-community.md                          # NEW STUB
```

## Execution Steps

### Step 1: Create new directory tree

Create all new directories. Do not create files yet.

```
mkdir -p docs/source/get-started
mkdir -p docs/source/concepts
mkdir -p docs/source/tutorials/day0
mkdir -p docs/source/tutorials/day1
mkdir -p docs/source/tutorials/day2
mkdir -p docs/source/operations/scaling
mkdir -p docs/source/operations/upgrading
mkdir -p docs/source/operations/backup-and-restore
mkdir -p docs/source/operations/networking/exposing-images
mkdir -p docs/source/operations/multi-datacenter
mkdir -p docs/source/operations/monitoring/diagrams
mkdir -p docs/source/operations/security
mkdir -p docs/source/operations/node-operations
mkdir -p docs/source/operations/data-management
mkdir -p docs/source/troubleshooting
mkdir -p docs/source/release-notes
```

### Step 2: Move existing files to new locations

Each move is listed as `OLD_PATH -> NEW_PATH`. All paths relative to `docs/source/`.

#### 2.1 Get Started (from quickstarts/)

| Old Path | New Path |
|----------|----------|
| `quickstarts/gke.md` | `get-started/gke.md` |
| `quickstarts/eks.md` | `get-started/eks.md` |

#### 2.2 Concepts (from architecture/)

| Old Path | New Path |
|----------|----------|
| `architecture/overview.md` | `concepts/architecture-overview.md` |
| `architecture/storage/overview.md` | `concepts/storage.md` |
| `architecture/storage/local-csi-driver.md` | `concepts/local-csi-driver.md` |
| `architecture/manager.md` | `concepts/manager-integration.md` |
| `architecture/tuning.md` | `concepts/performance-and-tuning.md` |
| `architecture/components-cluster_scoped.svg` | `concepts/components-cluster_scoped.svg` |
| `architecture/components-namespaced.svg` | `concepts/components-namespaced.svg` |
| `architecture/components.odg` | `concepts/components.odg` |
| `management/networking/ipv6/concepts/ipv6-networking.md` | `concepts/ipv6-networking.md` |

#### 2.3 Installation (stays, minor additions)

No files move -- `installation/` stays as-is. New stubs are added in Step 3.

#### 2.4 Tutorials (from resources/, management/)

| Old Path | New Path |
|----------|----------|
| `resources/scyllaclusters/basics.md` | `tutorials/day0/create-cluster.md` |
| `resources/scyllaclusters/clients/cql.md` | `tutorials/day0/connect-cql.md` |
| `resources/scyllaclusters/clients/alternator.md` | `tutorials/day0/connect-alternator.md` |
| `management/networking/ipv6/tutorials/ipv6-getting-started.md` | `tutorials/day1/ipv6-getting-started.md` |

#### 2.5 Operations Guides (from management/, resources/)

| Old Path | New Path |
|----------|----------|
| `resources/scyllaclusters/nodeoperations/volume-expansion.md` | `operations/scaling/volume-expansion.md` |
| `management/upgrading/upgrade.md` | `operations/upgrading/upgrade-operator.md` |
| `management/upgrading/upgrade-scylladb.md` | `operations/upgrading/upgrade-scylladb.md` |
| `resources/scyllaclusters/nodeoperations/restore.md` | `operations/backup-and-restore/restore-from-backup.md` |
| `resources/common/exposing.md` | `operations/networking/expose-clusters.md` |
| `resources/common/exposing-images/clusterip.svg` | `operations/networking/exposing-images/clusterip.svg` |
| `resources/common/exposing-images/loadbalancer.svg` | `operations/networking/exposing-images/loadbalancer.svg` |
| `resources/common/exposing-images/multivpc.svg` | `operations/networking/exposing-images/multivpc.svg` |
| `resources/common/exposing-images/podips.svg` | `operations/networking/exposing-images/podips.svg` |
| `management/networking/ipv6/how-to/ipv6-configure.md` | `operations/networking/ipv6-configure.md` |
| `management/networking/ipv6/how-to/ipv6-configure-ipv6-first.md` | `operations/networking/ipv6-configure-ipv6-first.md` |
| `management/networking/ipv6/how-to/ipv6-configure-ipv6-only.md` | `operations/networking/ipv6-configure-ipv6-only.md` |
| `management/networking/ipv6/how-to/ipv6-migrate.md` | `operations/networking/ipv6-migrate.md` |
| `management/networking/ipv6/ipv6-dual-stack-architecture.svg` | `operations/networking/ipv6-dual-stack-architecture.svg` |
| `resources/common/multidc/gke.md` | `operations/multi-datacenter/multi-dc-gke.md` |
| `resources/common/multidc/eks.md` | `operations/multi-datacenter/multi-dc-eks.md` |
| `resources/scyllaclusters/multidc/multidc.md` | `operations/multi-datacenter/deploy-multi-dc-cluster.md` |
| `management/monitoring/overview.md` | `operations/monitoring/overview.md` |
| `management/monitoring/setup.md` | `operations/monitoring/setup.md` |
| `management/monitoring/exposing-grafana.md` | `operations/monitoring/expose-grafana.md` |
| `management/monitoring/external-prometheus-on-openshift.md` | `operations/monitoring/external-prometheus-openshift.md` |
| `management/monitoring/diagrams/monitoring-overview.mmd` | `operations/monitoring/diagrams/monitoring-overview.mmd` |
| `resources/scyllaclusters/nodeoperations/replace-node.md` | `operations/node-operations/replace-node.md` |
| `resources/scyllaclusters/nodeoperations/automatic-cleanup.md` | `operations/node-operations/automatic-cleanup.md` |
| `resources/scyllaclusters/nodeoperations/maintenance-mode.md` | `operations/node-operations/maintenance-mode.md` |
| `management/bootstrap-sync.md` | `operations/node-operations/bootstrap-sync.md` |
| `management/data-cleanup.md` | `operations/data-management/automatic-data-cleanup.md` |

#### 2.6 Troubleshooting (from support/)

| Old Path | New Path |
|----------|----------|
| `support/troubleshooting/installation.md` | `troubleshooting/installation-issues.md` |
| `support/must-gather.md` | `troubleshooting/must-gather.md` |
| `support/known-issues.md` | `troubleshooting/known-issues.md` |
| `management/networking/ipv6/how-to/ipv6-troubleshoot.md` | `troubleshooting/ipv6-troubleshoot.md` |

#### 2.7 Reference (from resources/)

| Old Path | New Path |
|----------|----------|
| `resources/scyllaoperatorconfigs.md` | `reference/scyllaoperatorconfig-options.md` |
| `management/networking/ipv6/reference/ipv6-configuration.md` | `reference/ipv6-configuration.md` |

#### 2.8 Release Notes (from support/)

| Old Path | New Path |
|----------|----------|
| `support/releases.md` | `release-notes/releases.md` |

#### 2.9 Support (slimmed)

No files move -- `support/overview.md` stays. Only `releases.md`, `must-gather.md`, `known-issues.md`, and `troubleshooting/` are moved out.

#### 2.10 Files handled specially

These files from the old structure are consumed/merged during Stage 2 rather than moved directly:

| Old Path | Disposition |
|----------|-------------|
| `resources/scyllaclusters/clients/discovery.md` | Content merged into `tutorials/day0/connect-cql.md` in Stage 2. **Move to `tutorials/day0/discovery.md` for now** as a holding location; will be merged later. |
| `resources/scylladbclusters/scylladbclusters.md` | Content to be merged into unified pages (tabs pattern) in Stage 2. **Move to `tutorials/day0/create-cluster-scylladbcluster.md` for now** as a holding location. |
| `resources/overview.md` | CRD overview content feeds into `concepts/crd-ecosystem.md` in Stage 2. **Move to `concepts/crd-ecosystem-old.md` for now** as source material. |
| `resources/nodeconfigs.md` | Content splits between `installation/node-configuration.md` and `reference/`. **Move to `installation/nodeconfigs-old.md` for now** as source material. |
| `resources/remotekubernetesclusters.md` | Used in multi-DC context. **Move to `operations/multi-datacenter/remotekubernetesclusters.md`**. |
| `resources/remotekubernetesclusters.svg` | **Move to `operations/multi-datacenter/remotekubernetesclusters.svg`**. |
| `management/sysctls.md` | Content splits between `installation/node-configuration.md` and `reference/kernel-parameters.md` in Stage 2. **Move to `reference/kernel-parameters-old.md` for now** as source material. |
| `installation/overview.md` | **Keep in place**. Content will be refactored in Stage 2 to remove navigation-hub aspects. |
| `resources/scyllaclusters/nodeoperations/scylla-upgrade.md` | Overlaps with `management/upgrading/upgrade-scylladb.md`. Content to be merged in Stage 2. **Move to `operations/upgrading/scylla-upgrade-nodeops.md` for now** as source material. |

### Step 3: Create stub pages for new content

Each stub page follows this template:

```markdown
# {Page Title}

<!-- Diataxis type: {Tutorial|How-to Guide|Explanation|Reference} -->
<!-- Content status: NEW - to be written -->

:::{todo}
**Content to write:** {Brief description of what this page should contain.}

**Source material:** {Existing docs or external references that inform this page, or "None - original content needed."}
:::
```

#### 3.1 Get Started stubs

| File | Title | Diataxis | Description |
|------|-------|----------|-------------|
| `get-started/local.md` | Quickstart: Local Cluster (Kind/Minikube) | Tutorial | Minimal-prerequisites quickstart for running ScyllaDB on a laptop using Kind or Minikube. Single command installs, 3-node cluster, no cloud account required. |

#### 3.2 Concepts stubs

| File | Title | Diataxis | Description |
|------|-------|----------|-------------|
| `concepts/operator-pattern.md` | The Operator Pattern and Reconciliation Loop | Explanation | How the operator watches CRDs and converges state. Controller architecture, webhook flow, reconciliation loop. Source: partial content in `architecture/overview.md`. |
| `concepts/crd-ecosystem.md` | CRD Ecosystem | Explanation | Overview of all CRDs and how they relate to each other. Source: partial content in `resources/overview.md` (moved to `crd-ecosystem-old.md`). |
| `concepts/why-scylladb-on-kubernetes.md` | Why Run ScyllaDB on Kubernetes? | Explanation | Business and technical rationale for running ScyllaDB on K8s. None - original content needed. |
| `concepts/racks-zones-topology.md` | Racks, Availability Zones, and Topology | Explanation | How ScyllaDB's rack-aware replication maps to K8s topology. Source: partial content in `tutorials/day0/create-cluster.md` (AZ spreading section). |
| `concepts/networking-model.md` | Networking Model | Explanation | Service types, exposure options, how ScyllaDB networking maps to K8s networking. Source: conceptual parts of `operations/networking/expose-clusters.md`. |
| `concepts/multi-datacenter-connectivity.md` | Multi-Datacenter Connectivity | Explanation | Network architecture for multi-DC deployments, cross-cluster networking. Source: partial content in multi-DC guides. |
| `concepts/failure-modes.md` | Failure Modes | Explanation | What happens when nodes fail, disks die, network partitions occur, and how the operator responds. Inspired by CloudNativePG. None - original content needed. |

#### 3.3 Installation stubs

| File | Title | Diataxis | Description |
|------|-------|----------|-------------|
| `installation/node-configuration.md` | Node Configuration | How-to | Kernel params, sysctls, CPU pinning, RAID setup prerequisites. Source: `management/sysctls.md` + `resources/nodeconfigs.md` + `architecture/tuning.md`. |
| `installation/verify-installation.md` | Verify Your Installation | How-to | Post-install health checks: verify operator pod is running, CRDs are registered, webhooks are configured, node config applied. None - original content needed. |

#### 3.4 Tutorial stubs

| File | Title | Diataxis | Description |
|------|-------|----------|-------------|
| `tutorials/day1/configure-storage-tuning.md` | Configure Storage and Performance Tuning | Tutorial | Progressive tutorial: choose storage class, configure local NVMe, set tuning parameters. Source: `concepts/storage.md` + `concepts/performance-and-tuning.md`. |
| `tutorials/day1/setup-monitoring.md` | Set Up Monitoring | Tutorial | Deploy ScyllaDB Monitoring stack end-to-end. Source: references `operations/monitoring/setup.md`. |
| `tutorials/day1/configure-backups.md` | Configure Backups | Tutorial | Set up ScyllaDB Manager backup schedules. None - original content needed. |
| `tutorials/day1/secure-your-cluster.md` | Secure Your Cluster | Tutorial | Enable TLS, configure authentication and RBAC. None - original content needed. |
| `tutorials/day1/expose-cluster.md` | Expose Your Cluster to External Clients | Tutorial | Tutorial walkthrough of exposure options. Source: references `operations/networking/expose-clusters.md`. |
| `tutorials/day1/production-checklist.md` | Production Checklist | Tutorial | Comprehensive go-live validation checklist: hardware sizing, storage, kernel tuning, network config, monitoring, backup strategy, HA, security hardening. Inspired by CockroachDB. None - original content needed. |
| `tutorials/day2/scale-cluster.md` | Scale Your Cluster | Tutorial | Add/remove nodes walkthrough. Source: partial content scattered across resource docs. |
| `tutorials/day2/upgrade-scylladb.md` | Upgrade ScyllaDB Version | Tutorial | Progressive tutorial for ScyllaDB rolling upgrade. Source: references `operations/upgrading/upgrade-scylladb.md`. |
| `tutorials/day2/upgrade-operator.md` | Upgrade ScyllaDB Operator | Tutorial | Progressive tutorial for operator upgrade. Source: references `operations/upgrading/upgrade-operator.md`. |
| `tutorials/day2/backup-and-restore.md` | Perform a Backup and Restore | Tutorial | End-to-end backup + restore walkthrough. Source: references `operations/backup-and-restore/`. |
| `tutorials/day2/deploy-multi-datacenter.md` | Deploy Multi-Datacenter | Tutorial | End-to-end multi-DC deployment tutorial. Source: references `operations/multi-datacenter/`. |

#### 3.5 Operations stubs

| File | Title | Diataxis | Description |
|------|-------|----------|-------------|
| `operations/scaling/add-remove-racks.md` | Add/Remove Racks | How-to | Modify rack topology in a running cluster. None - original content needed. |
| `operations/scaling/horizontal-scaling.md` | Horizontal Scale-Out/In | How-to | Add or remove nodes within existing racks. Source: partial content in resource docs. |
| `operations/upgrading/rolling-update-strategy.md` | Rolling Update Strategy | How-to | How rolling updates work, pod disruption budgets, tuning update speed. None - original content needed. |
| `operations/backup-and-restore/configure-backup-schedules.md` | Configure Backup Schedules | How-to | Set up automated backups via ScyllaDB Manager. None - original content needed. |
| `operations/security/tls-configuration.md` | TLS Configuration | How-to | Enable and configure TLS for client and inter-node communication. None - original content needed. |
| `operations/security/authentication-authorization.md` | Authentication and Authorization | How-to | User management, RBAC, authentication mechanisms. None - original content needed. |
| `operations/data-management/repair-scheduling.md` | Repair Scheduling | How-to | Set up and monitor anti-entropy repairs via ScyllaDB Manager. None - original content needed. |
| `operations/data-management/compaction-strategy.md` | Compaction Strategy Configuration | How-to | Configure and tune compaction strategies for ScyllaDB. None - original content needed. |

#### 3.6 Troubleshooting stubs

| File | Title | Diataxis | Description |
|------|-------|----------|-------------|
| `troubleshooting/cluster-health-diagnostics.md` | Cluster Health Diagnostics | How-to | How to assess cluster health, interpret status conditions, debug unhealthy clusters. None - original content needed. |
| `troubleshooting/common-error-messages.md` | Common Error Messages | Reference | Searchable reference of error messages and resolutions. None - original content needed. |

#### 3.7 Reference stubs

| File | Title | Diataxis | Description |
|------|-------|----------|-------------|
| `reference/helm-chart-values.md` | Helm Chart Values | Reference | Complete Helm chart configuration reference. None - original content needed (generated or extracted from chart). |
| `reference/kernel-parameters.md` | Kernel Parameters (sysctls) | Reference | Reference table of all kernel parameters the operator configures. Source: `management/sysctls.md` reference content. |
| `reference/compatibility-matrix.md` | Compatibility Matrix | Reference | Supported K8s versions, ScyllaDB versions, platforms (GKE, EKS, OpenShift). None - original content needed. |

#### 3.8 Release Notes stubs

| File | Title | Diataxis | Description |
|------|-------|----------|-------------|
| `release-notes/migration-guides.md` | Migration Guides | Reference | Version-to-version migration instructions. None - original content needed. |

#### 3.9 Support stubs

| File | Title | Diataxis | Description |
|------|-------|----------|-------------|
| `support/contact-community.md` | Contact and Community | Reference | Slack, GitHub, mailing lists, community resources. Source: possibly extracted from `support/overview.md` or new. |

### Step 4: Create/update index pages with toctrees

Every section needs an `index.md` with a toctree. Below is the exact content for each.

#### 4.1 Root `index.md`

Rewrite `docs/source/index.md`. The toctree changes to the 9-section structure:

```markdown
:::{toctree}
:hidden:
:maxdepth: 1

get-started/index
concepts/index
installation/index
tutorials/index
operations/index
troubleshooting/index
reference/index
release-notes/index
support/index
:::
```

The hero-box section stays but the topic-box grid is updated to reflect the new 9 sections (Get Started, Concepts, Installation, Tutorials, Operations, Troubleshooting, Reference, Release Notes, Support).

#### 4.2 `get-started/index.md`

```markdown
# Get Started

:::{toctree}
:maxdepth: 1

local
gke
eks
:::
```

#### 4.3 `concepts/index.md`

```markdown
# Concepts

:::{toctree}
:maxdepth: 1

architecture-overview
operator-pattern
crd-ecosystem
why-scylladb-on-kubernetes
racks-zones-topology
storage
local-csi-driver
manager-integration
networking-model
ipv6-networking
multi-datacenter-connectivity
performance-and-tuning
failure-modes
:::
```

#### 4.4 `installation/index.md` (REWRITE)

```markdown
# Installation

:::{toctree}
:maxdepth: 1

overview
kubernetes-prerequisites
node-configuration
helm
gitops
OpenShift <openshift>
verify-installation
:::
```

#### 4.5 `tutorials/index.md`

```markdown
# Tutorials

:::{toctree}
:maxdepth: 1

day0/index
day1/index
day2/index
:::
```

#### 4.6 `tutorials/day0/index.md`

```markdown
# Day 0: Your First ScyllaDB Cluster

:::{toctree}
:maxdepth: 1

create-cluster
connect-cql
connect-alternator
:::
```

Note: `discovery.md` (holding file) and `create-cluster-scylladbcluster.md` (holding file) are intentionally NOT in the toctree -- they are source material for Stage 2 merges. They should be listed as `:orphan:` or excluded until merged.

#### 4.7 `tutorials/day1/index.md`

```markdown
# Day 1: Production Configuration

:::{toctree}
:maxdepth: 1

configure-storage-tuning
setup-monitoring
configure-backups
secure-your-cluster
expose-cluster
production-checklist
ipv6-getting-started
:::
```

#### 4.8 `tutorials/day2/index.md`

```markdown
# Day 2: Operations Lifecycle

:::{toctree}
:maxdepth: 1

scale-cluster
upgrade-scylladb
upgrade-operator
backup-and-restore
deploy-multi-datacenter
:::
```

#### 4.9 `operations/index.md`

```markdown
# Operations Guides

:::{toctree}
:maxdepth: 1

scaling/index
upgrading/index
backup-and-restore/index
networking/index
multi-datacenter/index
monitoring/index
security/index
node-operations/index
data-management/index
:::
```

#### 4.10 `operations/scaling/index.md`

```markdown
# Scaling

:::{toctree}
:maxdepth: 1

add-remove-racks
horizontal-scaling
volume-expansion
:::
```

#### 4.11 `operations/upgrading/index.md`

```markdown
# Upgrading

:::{toctree}
:maxdepth: 1

upgrade-operator
upgrade-scylladb
rolling-update-strategy
:::
```

Note: `scylla-upgrade-nodeops.md` (holding file) is NOT in toctree -- source material for Stage 2 merge. Mark as `:orphan:`.

#### 4.12 `operations/backup-and-restore/index.md`

```markdown
# Backup and Restore

:::{toctree}
:maxdepth: 1

configure-backup-schedules
restore-from-backup
:::
```

#### 4.13 `operations/networking/index.md`

```markdown
# Networking

:::{toctree}
:maxdepth: 1

expose-clusters
ipv6-configure
ipv6-configure-ipv6-first
ipv6-configure-ipv6-only
ipv6-migrate
:::
```

#### 4.14 `operations/multi-datacenter/index.md`

```markdown
# Multi-Datacenter

:::{toctree}
:maxdepth: 1

multi-dc-gke
multi-dc-eks
deploy-multi-dc-cluster
remotekubernetesclusters
:::
```

#### 4.15 `operations/monitoring/index.md`

```markdown
# Monitoring

:::{toctree}
:maxdepth: 1

overview
setup
expose-grafana
external-prometheus-openshift
:::
```

#### 4.16 `operations/security/index.md`

```markdown
# Security

:::{toctree}
:maxdepth: 1

tls-configuration
authentication-authorization
:::
```

#### 4.17 `operations/node-operations/index.md`

```markdown
# Node Operations

:::{toctree}
:maxdepth: 1

replace-node
automatic-cleanup
maintenance-mode
bootstrap-sync
:::
```

#### 4.18 `operations/data-management/index.md`

```markdown
# Data Management

:::{toctree}
:maxdepth: 1

automatic-data-cleanup
repair-scheduling
compaction-strategy
:::
```

#### 4.19 `troubleshooting/index.md`

```markdown
# Troubleshooting

:::{toctree}
:maxdepth: 1

installation-issues
cluster-health-diagnostics
common-error-messages
must-gather
known-issues
ipv6-troubleshoot
:::
```

#### 4.20 `reference/index.md` (REWRITE)

```markdown
# Reference

:::{toctree}
:maxdepth: 1

api/index
feature-gates
helm-chart-values
scyllaoperatorconfig-options
kernel-parameters
ipv6-configuration
compatibility-matrix
:::
```

#### 4.21 `release-notes/index.md`

```markdown
# Release Notes

:::{toctree}
:maxdepth: 1

releases
migration-guides
:::
```

#### 4.22 `support/index.md` (REWRITE)

```markdown
# Support

:::{toctree}
:maxdepth: 1

overview
contact-community
:::
```

### Step 5: Fix all `{include}` paths

After files move, relative `:::{include}` paths to `.internal/` snippets break. Each moved file that uses includes must have its include paths updated.

| Moved File (new location) | Include Directive | Old Relative Path | New Relative Path |
|---------------------------|-------------------|--------------------|-------------------|
| `tutorials/day0/create-cluster.md` | `:::{include}` | `../../.internal/rf-warning.md` | `../../.internal/rf-warning.md` (same depth -- OK) |
| `tutorials/day0/create-cluster.md` | `:::{include}` | `../../.internal/tuning-qos-caution.md` | `../../.internal/tuning-qos-caution.md` (same depth -- OK) |
| `tutorials/day0/create-cluster.md` | `:::{include}` | `./../../.internal/wait-for-status-conditions.scyllacluster.code-block.md` | `../../.internal/wait-for-status-conditions.scyllacluster.code-block.md` |
| `concepts/performance-and-tuning.md` | `:::{include}` | `../.internal/tuning-warning.md` | `../.internal/tuning-warning.md` (same depth -- OK) |
| `concepts/performance-and-tuning.md` | `:::{include}` | `../.internal/tuning-qos-caution.md` | `../.internal/tuning-qos-caution.md` (same depth -- OK) |
| `concepts/manager-integration.md` | `:::{include}` | `../.internal/manager-license-note.md` | `../.internal/manager-license-note.md` (same depth -- OK) |
| `operations/upgrading/upgrade-scylladb.md` | `:::{include}` | `./../../.internal/wait-for-*` | `../../.internal/wait-for-*` (same depth -- OK) |
| `operations/node-operations/bootstrap-sync.md` | `:::{include}` | `../.internal/bootstrap-sync-min-scylladb-version-caution.md` | `../../.internal/bootstrap-sync-min-scylladb-version-caution.md` (depth changed: 1 -> 2) |
| `operations/networking/expose-clusters.md` | `{include}` | `../../.internal/scylladbcluster-cluster-ip-exposure-caution.md` | `../../.internal/scylladbcluster-cluster-ip-exposure-caution.md` (same depth -- OK) |
| `operations/multi-datacenter/deploy-multi-dc-cluster.md` | `:::{include}` | `../../../.internal/rf-warning.md` | `../../.internal/rf-warning.md` (depth changed: 3 -> 2) |
| `operations/monitoring/overview.md` | `:::{include}` | `diagrams/monitoring-overview.mmd` | `diagrams/monitoring-overview.mmd` (same relative -- OK, diagrams dir moves with it) |
| `installation/nodeconfigs-old.md` | `:::{include}` | `../.internal/tuning-warning.md` | `../.internal/tuning-warning.md` (same depth -- OK) |
| `installation/nodeconfigs-old.md` | `:::{include}` | `./../.internal/wait-for-status-conditions.nodeconfig.code-block.md` | `../.internal/wait-for-status-conditions.nodeconfig.code-block.md` (same depth -- OK) |
| `reference/kernel-parameters-old.md` | `:::{include}` | `./../.internal/wait-for-status-conditions.nodeconfig.code-block.md` | `../.internal/wait-for-status-conditions.nodeconfig.code-block.md` (same depth -- OK) |
| `reference/scyllaoperatorconfig-options.md` | (no includes) | N/A | N/A |

**Important:** Every moved file must be checked for:
1. `:::{include}` / `{include}` directives (relative paths to `.internal/`)
2. `{literalinclude}` directives (relative paths to example files)
3. Cross-references to other docs (relative links `[text](../path.md)`)
4. Image references (SVG, PNG paths)

A comprehensive sweep of all moved files is required. The table above covers the known `.internal/` includes; additional references (literalincludes, cross-references, images) must be audited file-by-file during execution.

### Step 6: Fix `literalinclude` and cross-reference paths

Files that use `{literalinclude}` reference example YAML files in the repository (typically under `examples/`). Since these reference paths from the doc's old location, they may use relative paths that break after the move.

Key files to audit:
- `tutorials/day0/create-cluster.md` (was `resources/scyllaclusters/basics.md`) -- likely has literalincludes
- `tutorials/day0/create-cluster-scylladbcluster.md` (was `resources/scylladbclusters/scylladbclusters.md`) -- has literalincludes
- `operations/multi-datacenter/deploy-multi-dc-cluster.md` (was `resources/scyllaclusters/multidc/multidc.md`) -- has literalincludes
- `operations/multi-datacenter/remotekubernetesclusters.md` (was `resources/remotekubernetesclusters.md`) -- has literalincludes

Cross-references between docs (e.g., `[text](../resources/common/exposing.md)`) must be updated to reflect new paths throughout the entire doc tree, including files that were not themselves moved but reference files that were.

### Step 7: Mark orphan files

Files that are holding/source material for Stage 2 and are NOT included in any toctree must be marked with the `:orphan:` directive at the top to prevent Sphinx build warnings:

```markdown
:orphan:

# {Original Title}
...
```

Files to mark as orphan:
- `tutorials/day0/discovery.md`
- `tutorials/day0/create-cluster-scylladbcluster.md`
- `concepts/crd-ecosystem-old.md`
- `installation/nodeconfigs-old.md`
- `reference/kernel-parameters-old.md`
- `operations/upgrading/scylla-upgrade-nodeops.md`

### Step 8: Update `redirects.yaml`

Add redirects for all old paths to new paths. Redirects use the URL path format (not file paths).

```yaml
# Docs rework: Structure migration redirects

# Get Started (was Quickstarts)
/stable/quickstarts/index.html: /stable/get-started/index.html
/stable/quickstarts/gke.html: /stable/get-started/gke.html
/stable/quickstarts/eks.html: /stable/get-started/eks.html

# Concepts (was Architecture)
/stable/architecture/index.html: /stable/concepts/index.html
/stable/architecture/overview.html: /stable/concepts/architecture-overview.html
/stable/architecture/storage/index.html: /stable/concepts/storage.html
/stable/architecture/storage/overview.html: /stable/concepts/storage.html
/stable/architecture/storage/local-csi-driver.html: /stable/concepts/local-csi-driver.html
/stable/architecture/manager.html: /stable/concepts/manager-integration.html
/stable/architecture/tuning.html: /stable/concepts/performance-and-tuning.html

# Resources -> various locations
/stable/resources/index.html: /stable/concepts/crd-ecosystem.html
/stable/resources/overview.html: /stable/concepts/crd-ecosystem.html
/stable/resources/scyllaclusters/index.html: /stable/tutorials/day0/index.html
/stable/resources/scyllaclusters/basics.html: /stable/tutorials/day0/create-cluster.html
/stable/resources/scyllaclusters/clients/index.html: /stable/tutorials/day0/connect-cql.html
/stable/resources/scyllaclusters/clients/cql.html: /stable/tutorials/day0/connect-cql.html
/stable/resources/scyllaclusters/clients/alternator.html: /stable/tutorials/day0/connect-alternator.html
/stable/resources/scyllaclusters/clients/discovery.html: /stable/tutorials/day0/connect-cql.html
/stable/resources/scyllaclusters/nodeoperations/index.html: /stable/operations/node-operations/index.html
/stable/resources/scyllaclusters/nodeoperations/volume-expansion.html: /stable/operations/scaling/volume-expansion.html
/stable/resources/scyllaclusters/nodeoperations/replace-node.html: /stable/operations/node-operations/replace-node.html
/stable/resources/scyllaclusters/nodeoperations/automatic-cleanup.html: /stable/operations/node-operations/automatic-cleanup.html
/stable/resources/scyllaclusters/nodeoperations/maintenance-mode.html: /stable/operations/node-operations/maintenance-mode.html
/stable/resources/scyllaclusters/nodeoperations/restore.html: /stable/operations/backup-and-restore/restore-from-backup.html
/stable/resources/scyllaclusters/nodeoperations/scylla-upgrade.html: /stable/operations/upgrading/upgrade-scylladb.html
/stable/resources/scyllaclusters/multidc/index.html: /stable/operations/multi-datacenter/index.html
/stable/resources/scyllaclusters/multidc/multidc.html: /stable/operations/multi-datacenter/deploy-multi-dc-cluster.html
/stable/resources/common/exposing.html: /stable/operations/networking/expose-clusters.html
/stable/resources/common/multidc/gke.html: /stable/operations/multi-datacenter/multi-dc-gke.html
/stable/resources/common/multidc/eks.html: /stable/operations/multi-datacenter/multi-dc-eks.html
/stable/resources/scylladbclusters/index.html: /stable/tutorials/day0/create-cluster.html
/stable/resources/scylladbclusters/scylladbclusters.html: /stable/tutorials/day0/create-cluster.html
/stable/resources/nodeconfigs.html: /stable/installation/node-configuration.html
/stable/resources/scyllaoperatorconfigs.html: /stable/reference/scyllaoperatorconfig-options.html
/stable/resources/remotekubernetesclusters.html: /stable/operations/multi-datacenter/remotekubernetesclusters.html

# Management -> Operations
/stable/management/index.html: /stable/operations/index.html
/stable/management/sysctls.html: /stable/installation/node-configuration.html
/stable/management/bootstrap-sync.html: /stable/operations/node-operations/bootstrap-sync.html
/stable/management/data-cleanup.html: /stable/operations/data-management/automatic-data-cleanup.html
/stable/management/upgrading/index.html: /stable/operations/upgrading/index.html
/stable/management/upgrading/upgrade.html: /stable/operations/upgrading/upgrade-operator.html
/stable/management/upgrading/upgrade-scylladb.html: /stable/operations/upgrading/upgrade-scylladb.html
/stable/management/monitoring/index.html: /stable/operations/monitoring/index.html
/stable/management/monitoring/overview.html: /stable/operations/monitoring/overview.html
/stable/management/monitoring/setup.html: /stable/operations/monitoring/setup.html
/stable/management/monitoring/exposing-grafana.html: /stable/operations/monitoring/expose-grafana.html
/stable/management/monitoring/external-prometheus-on-openshift.html: /stable/operations/monitoring/external-prometheus-openshift.html
/stable/management/networking/index.html: /stable/operations/networking/index.html
/stable/management/networking/ipv6/index.html: /stable/operations/networking/ipv6-configure.html
/stable/management/networking/ipv6/tutorials/ipv6-getting-started.html: /stable/tutorials/day1/ipv6-getting-started.html
/stable/management/networking/ipv6/how-to/ipv6-configure.html: /stable/operations/networking/ipv6-configure.html
/stable/management/networking/ipv6/how-to/ipv6-configure-ipv6-first.html: /stable/operations/networking/ipv6-configure-ipv6-first.html
/stable/management/networking/ipv6/how-to/ipv6-configure-ipv6-only.html: /stable/operations/networking/ipv6-configure-ipv6-only.html
/stable/management/networking/ipv6/how-to/ipv6-migrate.html: /stable/operations/networking/ipv6-migrate.html
/stable/management/networking/ipv6/how-to/ipv6-troubleshoot.html: /stable/troubleshooting/ipv6-troubleshoot.html
/stable/management/networking/ipv6/reference/ipv6-configuration.html: /stable/reference/ipv6-configuration.html
/stable/management/networking/ipv6/concepts/ipv6-networking.html: /stable/concepts/ipv6-networking.html

# Support -> various locations
/stable/support/known-issues.html: /stable/troubleshooting/known-issues.html
/stable/support/troubleshooting/index.html: /stable/troubleshooting/index.html
/stable/support/troubleshooting/installation.html: /stable/troubleshooting/installation-issues.html
/stable/support/must-gather.html: /stable/troubleshooting/must-gather.html
/stable/support/releases.html: /stable/release-notes/releases.html
```

**Note:** These redirects must be duplicated for `/master/` as well. The existing redirects in `redirects.yaml` should be reviewed and updated or removed if they point to paths that no longer exist.

### Step 9: Clean up old directories

After all files are moved, remove empty old directories:

```
docs/source/quickstarts/                    # empty after moves
docs/source/architecture/                   # empty after moves (including storage/)
docs/source/management/                     # empty after moves (all subdirs)
docs/source/resources/                      # empty after moves (all subdirs)
docs/source/support/troubleshooting/        # empty after moves
```

**Verify each directory is truly empty before removing.** The `support/` directory itself stays (it still has `overview.md` and `contact-community.md`).

### Step 10: Verify build passes

```bash
cd docs && make setup && make dirhtml
```

Fix any:
- Broken toctree references
- Missing files referenced in toctrees
- Broken `{include}` or `{literalinclude}` paths
- Broken cross-references
- Sphinx warnings about documents not in any toctree (orphan check)

### Step 11: Verify redirects

```bash
cd docs && make redirects
```

Confirm redirect HTML files are generated for all old paths.

## Summary Statistics

| Category | Count |
|----------|-------|
| Files moved | ~45 (including SVGs, diagrams, holding files) |
| New stub pages created | 30 |
| New index pages created | 18 |
| Index pages rewritten | 4 (root, installation, reference, support) |
| Old directories removed | 5 top-level (quickstarts, architecture, management, resources, support/troubleshooting) |
| Redirects added | ~55 |
| Orphan holding files | 6 |
