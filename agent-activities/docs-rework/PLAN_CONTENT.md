# Stage 2: Content Migration & Gap Filling Plan

## Goal

Fill the stub pages created in Stage 1 with real content by migrating from existing documentation and marking content gaps as actionable TODOs for team members. No new content is invented -- existing material is reorganized, split by Diataxis type, and gaps are documented for domain experts to fill.

## Priority Tiers

Every task is assigned a priority tier:

- **P0 -- Straightforward moves:** Content already exists at the new location (moved in Stage 1). Needs only minor Diataxis cleanup (remove mixed-type content) and path/cross-reference verification.
- **P1 -- Extract/merge/split:** Content exists but is split across files, mixed with other Diataxis types, or needs to be merged from multiple sources. Requires editorial work.
- **P2 -- Critical new content:** Pages that fill important gaps (production checklist, security, installation verification). Needs original writing by domain experts. Marked with TODO.
- **P3 -- Nice-to-have new content:** Pages that improve completeness but are not blocking. Can be deferred. Marked with TODO.

---

## Section 1: Get Started

### P0 -- Content in place (review only)

| Page | Source | Action |
|------|--------|--------|
| `get-started/gke.md` | `quickstarts/gke.md` (moved) | Review for consistency with new structure. Verify all cross-references point to new paths. No content changes expected. |
| `get-started/eks.md` | `quickstarts/eks.md` (moved) | Same as above. |

### P2 -- New content needed

| Page | Action |
|------|--------|
| `get-started/local.md` | **TODO for team:** Write a minimal Kind/Minikube quickstart. Should get a user from zero to a running 3-node ScyllaDB cluster on a laptop in under 15 minutes. No cloud account required. Include: prerequisites (Docker, Kind/Minikube, kubectl, Helm), install operator, create cluster, verify with cqlsh. Keep it as short as possible -- link to Tutorials for deeper learning. |

---

## Section 2: Concepts

### P0 -- Content in place (Diataxis cleanup)

These pages were moved in Stage 1. Each needs a Diataxis cleanup pass: ensure content is purely **Explanation** (understanding-oriented). Remove any step-by-step instructions (move them to the appropriate How-to or Tutorial page). Remove any pure reference tables (move them to Reference).

| Page | Source | Cleanup Needed |
|------|--------|----------------|
| `concepts/architecture-overview.md` | `architecture/overview.md` | Minor. Currently short (31 lines) with two SVG diagrams. Update SVG paths to new locations (`concepts/components-*.svg`). Extract any content that belongs in `operator-pattern.md`. |
| `concepts/storage.md` | `architecture/storage/overview.md` | Minor. Pure explanation content. Verify image/cross-reference paths. |
| `concepts/local-csi-driver.md` | `architecture/storage/local-csi-driver.md` | Minor. Pure explanation. Verify paths. |
| `concepts/manager-integration.md` | `architecture/manager.md` | Minor. Already explanation-oriented. Verify include path to `.internal/manager-license-note.md`. |
| `concepts/performance-and-tuning.md` | `architecture/tuning.md` | Moderate. Verify include paths. Ensure how-to content (specific sysctl commands) is referenced to Installation/Node Configuration rather than duplicated here. |
| `concepts/ipv6-networking.md` | `management/networking/ipv6/concepts/ipv6-networking.md` | Moderate. Already explanation-oriented (323 lines). Update all cross-references that pointed to sibling IPv6 files (now in `operations/networking/`, `reference/`, `troubleshooting/`). |

### P1 -- Extract/merge from existing content

| Page | Source Material | Action |
|------|-----------------|--------|
| `concepts/operator-pattern.md` | `concepts/architecture-overview.md` (moved), `concepts/crd-ecosystem-old.md` (holding file) | Extract the "operator pattern" explanation from `architecture-overview.md` -- the parts about CRDs, webhooks, controllers, and reconciliation. The architecture overview page should remain as a high-level summary; this page goes deeper into the operator pattern. If `architecture-overview.md` is too thin to split, create this as a TODO stub referencing what should be covered. |
| `concepts/crd-ecosystem.md` | `concepts/crd-ecosystem-old.md` (holding file from `resources/overview.md`) | Transform the old Resources overview (which was a navigation hub with topic-box links) into an Explanation page. Keep the CRD categorization (namespaced vs cluster-scoped). Remove topic-box navigation. Add explanatory prose about how the CRDs relate to each other. The `kubectl api-resources` tip can stay. |
| `concepts/networking-model.md` | `operations/networking/expose-clusters.md` | Extract the conceptual/explanatory portions of the exposing doc: the explanation of Service Types, Broadcast Options, and the four deployment scenarios (with diagrams). The expose-clusters.md page keeps the how-to instructions and reference tables; this page gets the "understanding" content. |
| `concepts/racks-zones-topology.md` | `tutorials/day0/create-cluster.md` (the AZ spreading section) | Extract the explanation of rack-to-AZ mapping from the create-cluster page. The create-cluster page should link here for deeper understanding. **TODO for team:** Expand with ScyllaDB-specific topology concepts (replication factor, token ring, rack-aware placement). |

### P2 -- New content needed

| Page | Action |
|------|--------|
| `concepts/why-scylladb-on-kubernetes.md` | **TODO for team:** Write a positioning/rationale page. Cover: benefits of operator-managed ScyllaDB vs bare-metal, automation capabilities, Day-2 operations automation, cloud-native integration. Keep it concise -- this is a decision-support page, not marketing. |
| `concepts/multi-datacenter-connectivity.md` | **TODO for team:** Write an explanation of multi-DC network architecture. Cover: cross-cluster networking requirements, RemoteKubernetesCluster role, seed node discovery, inter-DC latency considerations. Source: extract conceptual content from `operations/multi-datacenter/deploy-multi-dc-cluster.md` and `operations/multi-datacenter/remotekubernetesclusters.md`. |
| `concepts/failure-modes.md` | **TODO for team:** Write a failure modes explanation inspired by CloudNativePG's approach. Cover: node failure, disk failure, network partition, operator restart, etcd loss, and how the operator responds to each. Critical for production users of a distributed database. |

### Cleanup

After P1 extractions are complete:
- Delete `concepts/crd-ecosystem-old.md` (holding file consumed).

---

## Section 3: Installation

### P0 -- Content in place (review only)

| Page | Source | Action |
|------|--------|--------|
| `installation/overview.md` | Existing (kept in place) | Review. Currently mixes explanation (what gets installed) with navigation (links to install methods). Refactor to be a clean overview/explanation page. Remove topic-box navigation if present -- the toctree handles navigation now. |
| `installation/kubernetes-prerequisites.md` | Existing (kept in place) | Review for accuracy. No structural changes. |
| `installation/helm.md` | Existing (kept in place) | Review. Verify include paths to `.internal/helm-crd-warning.md` and `.internal/namespaces.md`. |
| `installation/gitops.md` | Existing (kept in place) | Review. Verify include paths to `.internal/` snippets. |
| `installation/openshift.md` | Existing (kept in place) | Review. Verify include paths to `.internal/` snippets. |

### P1 -- Extract/merge from existing content

| Page | Source Material | Action |
|------|-----------------|--------|
| `installation/node-configuration.md` | `installation/nodeconfigs-old.md` (from `resources/nodeconfigs.md`), `reference/kernel-parameters-old.md` (from `management/sysctls.md`) | Merge into a How-to page: "How to configure your Kubernetes nodes for ScyllaDB." Include: NodeConfig CRD usage (from nodeconfigs.md), recommended sysctl values (from sysctls.md), RAID and filesystem setup, CPU pinning. Move pure reference tables (sysctl value lists) to `reference/kernel-parameters.md`. Keep the how-to steps here. |

### P2 -- New content needed

| Page | Action |
|------|--------|
| `installation/verify-installation.md` | **TODO for team:** Write post-install verification steps. Cover: check operator pod status, verify CRD registration (`kubectl get crds`), verify webhooks, verify NodeConfig status conditions, run a smoke-test cluster creation. |

### Cleanup

After P1 merge is complete:
- Delete `installation/nodeconfigs-old.md` (holding file consumed).

---

## Section 4: Tutorials

### P0 -- Content in place (Diataxis cleanup)

These pages were moved in Stage 1. Each needs a Diataxis cleanup pass: ensure content is a **Tutorial** (learning-oriented, progressive, assumes completion of previous steps).

| Page | Source | Cleanup Needed |
|------|--------|----------------|
| `tutorials/day0/create-cluster.md` | `resources/scyllaclusters/basics.md` | Significant. This is a 402-line page mixing tutorial, how-to, and reference content. **Actions:** (1) Keep the core tutorial path: create ConfigMap, create ScyllaCluster, verify. (2) Move AZ spreading section to `concepts/racks-zones-topology.md` (or link to it). (3) Move IPv6/dual-stack configuration section to a cross-reference to `operations/networking/ipv6-configure.md`. (4) Move rolling restart section to a cross-reference to Operations. (5) Add ScyllaDBCluster tabs (from holding file `create-cluster-scylladbcluster.md`) using the tab pattern from exposing.md. |
| `tutorials/day0/connect-cql.md` | `resources/scyllaclusters/clients/cql.md` | Moderate. Already a clean how-to. Merge discovery content from `tutorials/day0/discovery.md` (holding file) -- add a section on service discovery before the CQL connection instructions. Reframe as a tutorial ("Now that you have a cluster, let's connect to it"). |
| `tutorials/day0/connect-alternator.md` | `resources/scyllaclusters/clients/alternator.md` | Minor. Already a clean how-to. Reframe as tutorial with progressive narrative. |
| `tutorials/day1/ipv6-getting-started.md` | `management/networking/ipv6/tutorials/ipv6-getting-started.md` | Minor. Already tutorial-formatted (238 lines). Update cross-references to point to new IPv6 page locations. |

### P1 -- Stubs to content (referencing Operations Guides)

These stubs should be filled with tutorial-style content that wraps the corresponding Operations Guide. The tutorial provides narrative context, prerequisites, and a progressive walkthrough; the Operations Guide provides the standalone task reference. The tutorial should link to the guide for users who want just the steps.

| Page | Operations Guide Counterpart | Action |
|------|------------------------------|--------|
| `tutorials/day1/configure-storage-tuning.md` | `concepts/storage.md`, `concepts/performance-and-tuning.md` | Write a progressive tutorial: "Now that your cluster is running, let's configure it for production performance." Walk through storage class selection, local NVMe setup, tuning parameters. Link to concepts pages for deeper understanding. |
| `tutorials/day1/setup-monitoring.md` | `operations/monitoring/setup.md` | Write a tutorial wrapper: "Let's set up monitoring so you can observe your cluster." Provide narrative context about why monitoring matters for ScyllaDB, then walk through the setup. Link to the ops guide for standalone reference. |
| `tutorials/day1/expose-cluster.md` | `operations/networking/expose-clusters.md` | Write a tutorial wrapper: "Let's make your cluster accessible to applications." Walk through the most common scenario (ClusterIP within cluster, then LoadBalancer for external). Link to ops guide for all options. |
| `tutorials/day2/upgrade-scylladb.md` | `operations/upgrading/upgrade-scylladb.md` | Write a tutorial wrapper: "Your cluster is running version X -- let's upgrade to Y." Provide context about rolling upgrades, then walk through the procedure. Link to ops guide for standalone reference. |
| `tutorials/day2/upgrade-operator.md` | `operations/upgrading/upgrade-operator.md` | Write a tutorial wrapper: "The operator has a new version -- let's upgrade." Context about operator compatibility, then walkthrough. Link to ops guide. |

### P2 -- New content needed

| Page | Action |
|------|--------|
| `tutorials/day1/configure-backups.md` | **TODO for team:** Write a tutorial for setting up ScyllaDB Manager backup schedules. Cover: Manager prerequisites, configuring a backup task, verifying backups run, checking backup status. |
| `tutorials/day1/secure-your-cluster.md` | **TODO for team:** Write a tutorial for security hardening. Cover: enabling TLS (client-to-node, node-to-node), configuring authentication, basic RBAC setup. |
| `tutorials/day1/production-checklist.md` | **TODO for team:** Write the Production Checklist (inspired by CockroachDB). Subsections: Hardware and Node Sizing, Storage Recommendations, Kernel and OS Tuning, Network Configuration, Monitoring and Alerting, Backup Strategy, High Availability Checklist, Security Hardening. Each subsection should have specific, actionable checkpoints. |
| `tutorials/day2/scale-cluster.md` | **TODO for team:** Write a scaling tutorial. Cover: adding nodes to an existing rack, adding a new rack, verifying data redistribution, removing nodes. Source: partial content scattered across existing resource docs. |
| `tutorials/day2/backup-and-restore.md` | **TODO for team:** Write an end-to-end backup and restore tutorial. Cover: take a backup, verify it, simulate data loss, restore from backup, verify restoration. |
| `tutorials/day2/deploy-multi-datacenter.md` | **TODO for team:** Write an end-to-end multi-DC tutorial. Simplified version of the operations guide. Cover: prepare two clusters, configure networking, deploy DC1, deploy DC2, verify cross-DC replication. |

### Cleanup

After P0/P1 work is complete:
- Delete `tutorials/day0/discovery.md` (merged into `connect-cql.md`).
- Delete `tutorials/day0/create-cluster-scylladbcluster.md` (merged into `create-cluster.md` via tabs).

---

## Section 5: Operations Guides

### P0 -- Content in place (Diataxis cleanup)

These pages were moved in Stage 1. Each needs a Diataxis cleanup pass: ensure content is a **How-to Guide** (task-oriented, standalone, goal-focused).

| Page | Source | Cleanup Needed |
|------|--------|----------------|
| `operations/scaling/volume-expansion.md` | `resources/scyllaclusters/nodeoperations/volume-expansion.md` | Minor. Already task-focused. Verify cross-references. |
| `operations/upgrading/upgrade-operator.md` | `management/upgrading/upgrade.md` | Minor. Already task-focused. Verify cross-references. |
| `operations/upgrading/upgrade-scylladb.md` | `management/upgrading/upgrade-scylladb.md` | Moderate. Merge content from `operations/upgrading/scylla-upgrade-nodeops.md` (holding file from `resources/scyllaclusters/nodeoperations/scylla-upgrade.md`). The two files cover the same topic from different angles -- combine into a single authoritative guide. Verify include paths. |
| `operations/backup-and-restore/restore-from-backup.md` | `resources/scyllaclusters/nodeoperations/restore.md` | Minor. Already task-focused. Verify cross-references. |
| `operations/networking/expose-clusters.md` | `resources/common/exposing.md` | Significant. This is a 523-line page mixing reference and explanation. **Actions:** (1) Keep the how-to portions: step-by-step configuration for each exposure scenario. (2) Extract the conceptual explanation of Service Types and Broadcast Options to `concepts/networking-model.md`. (3) Keep the reference tables (configuration options) here or consider moving pure reference to `reference/`. (4) Verify SVG image paths (`exposing-images/`) and include paths. |
| `operations/networking/ipv6-configure.md` | `management/networking/ipv6/how-to/ipv6-configure.md` | Minor. Already well-structured how-to. Update cross-references. |
| `operations/networking/ipv6-configure-ipv6-first.md` | `management/networking/ipv6/how-to/ipv6-configure-ipv6-first.md` | Minor. Same as above. |
| `operations/networking/ipv6-configure-ipv6-only.md` | `management/networking/ipv6/how-to/ipv6-configure-ipv6-only.md` | Minor. Same as above. |
| `operations/networking/ipv6-migrate.md` | `management/networking/ipv6/how-to/ipv6-migrate.md` | Minor. Same as above. |
| `operations/multi-datacenter/multi-dc-gke.md` | `resources/common/multidc/gke.md` | Minor. Already task-focused. Verify cross-references. |
| `operations/multi-datacenter/multi-dc-eks.md` | `resources/common/multidc/eks.md` | Minor. Same as above. |
| `operations/multi-datacenter/deploy-multi-dc-cluster.md` | `resources/scyllaclusters/multidc/multidc.md` | Moderate. Long (607 lines) but already task-focused. Extract conceptual multi-DC explanation to `concepts/multi-datacenter-connectivity.md`. Update include paths (`.internal/rf-warning.md` depth changed). Verify literalinclude paths. |
| `operations/multi-datacenter/remotekubernetesclusters.md` | `resources/remotekubernetesclusters.md` | Minor. Short reference/how-to. Verify SVG path and cross-references. |
| `operations/monitoring/overview.md` | `management/monitoring/overview.md` | Minor. Explanation content (architecture of monitoring). Consider whether this should move to Concepts instead. Verify include path for Mermaid diagram. |
| `operations/monitoring/setup.md` | `management/monitoring/setup.md` | Minor. Already task-focused. Verify cross-references. |
| `operations/monitoring/expose-grafana.md` | `management/monitoring/exposing-grafana.md` | Minor. Already task-focused. |
| `operations/monitoring/external-prometheus-openshift.md` | `management/monitoring/external-prometheus-on-openshift.md` | Minor. Already task-focused. |
| `operations/node-operations/replace-node.md` | `resources/scyllaclusters/nodeoperations/replace-node.md` | Minor. Already task-focused. Verify cross-references. |
| `operations/node-operations/automatic-cleanup.md` | `resources/scyllaclusters/nodeoperations/automatic-cleanup.md` | Minor. Already task-focused. |
| `operations/node-operations/maintenance-mode.md` | `resources/scyllaclusters/nodeoperations/maintenance-mode.md` | Minor. Already task-focused. |
| `operations/node-operations/bootstrap-sync.md` | `management/bootstrap-sync.md` | Minor. Already task-focused. Verify include path (depth changed). |
| `operations/data-management/automatic-data-cleanup.md` | `management/data-cleanup.md` | Minor. Already task-focused. |

### P1 -- Merge from existing content

| Page | Source Material | Action |
|------|-----------------|--------|
| `operations/upgrading/upgrade-scylladb.md` | + `operations/upgrading/scylla-upgrade-nodeops.md` (holding file) | Merge the node-operations perspective into the main upgrade guide. The holding file covers ScyllaDB version upgrades from the "node operations" angle; the main file covers it from the "management" angle. Combine into a single authoritative how-to. |
| `operations/scaling/horizontal-scaling.md` | Partial content in `tutorials/day0/create-cluster.md` and resource docs | Extract any horizontal scaling instructions from existing docs. If insufficient content exists, mark the remainder as TODO. |

### P2 -- New content needed

| Page | Action |
|------|--------|
| `operations/scaling/add-remove-racks.md` | **TODO for team:** Write how-to for modifying rack topology. Cover: adding a new rack to an existing cluster, removing an empty rack, rebalancing after topology changes. |
| `operations/upgrading/rolling-update-strategy.md` | **TODO for team:** Write explanation/how-to for rolling update mechanics. Cover: how the operator performs rolling updates, PodDisruptionBudgets, controlling update speed, monitoring update progress. |
| `operations/backup-and-restore/configure-backup-schedules.md` | **TODO for team:** Write how-to for ScyllaDB Manager backup automation. Cover: creating backup tasks, scheduling, retention policies, monitoring backup status, alerting on failures. |
| `operations/security/tls-configuration.md` | **TODO for team:** Write how-to for TLS setup. Cover: generating/providing certificates, enabling client-to-node TLS, enabling node-to-node TLS, certificate rotation. |
| `operations/security/authentication-authorization.md` | **TODO for team:** Write how-to for auth setup. Cover: enabling authentication in ScyllaDB config, creating users, configuring RBAC, rotating credentials. |
| `operations/data-management/repair-scheduling.md` | **TODO for team:** Write how-to for anti-entropy repair. Cover: what repair does and why it's critical, configuring repair schedules via ScyllaDB Manager, monitoring repair progress, troubleshooting failed repairs. |
| `operations/data-management/compaction-strategy.md` | **TODO for team:** Write how-to for compaction tuning. Cover: available compaction strategies (STCS, LCS, ICS, TWCS), when to use each, how to configure via ScyllaDB config, monitoring compaction performance. |

### Cleanup

After P1 merge is complete:
- Delete `operations/upgrading/scylla-upgrade-nodeops.md` (holding file consumed).

---

## Section 6: Troubleshooting

### P0 -- Content in place (review only)

| Page | Source | Cleanup Needed |
|------|--------|----------------|
| `troubleshooting/installation-issues.md` | `support/troubleshooting/installation.md` | Minor. Verify cross-references point to new installation paths. |
| `troubleshooting/must-gather.md` | `support/must-gather.md` | Minor. Verify cross-references. |
| `troubleshooting/known-issues.md` | `support/known-issues.md` | Minor. Verify cross-references. |
| `troubleshooting/ipv6-troubleshoot.md` | `management/networking/ipv6/how-to/ipv6-troubleshoot.md` | Minor. Update cross-references to new IPv6 page locations. |

### P2 -- New content needed

| Page | Action |
|------|--------|
| `troubleshooting/cluster-health-diagnostics.md` | **TODO for team:** Write a diagnostic guide. Cover: how to interpret ScyllaCluster/ScyllaDBCluster status conditions, common unhealthy states, kubectl commands for debugging (describe, logs, events), checking Prometheus metrics for health. |
| `troubleshooting/common-error-messages.md` | **TODO for team:** Write a searchable error reference. Compile common error messages from operator logs, events, and status conditions. For each: the exact error text, what it means, likely causes, and resolution steps. |

---

## Section 7: Reference

### P0 -- Content in place (no changes needed)

| Page | Source | Action |
|------|--------|--------|
| `reference/api/` (entire directory) | Existing (unchanged) | No changes. Auto-generated content stays as-is. |
| `reference/feature-gates.md` | Existing (kept in place) | Review. Verify include path to `.internal/bootstrap-sync-min-scylladb-version-caution.md`. |
| `reference/scyllaoperatorconfig-options.md` | `resources/scyllaoperatorconfigs.md` (moved) | Minor. Already reference-oriented (34 lines). Verify cross-references. |
| `reference/ipv6-configuration.md` | `management/networking/ipv6/reference/ipv6-configuration.md` (moved) | Moderate. Update cross-references to new IPv6 page locations (concepts, operations, troubleshooting). |

### P1 -- Extract from existing content

| Page | Source Material | Action |
|------|-----------------|--------|
| `reference/kernel-parameters.md` | `reference/kernel-parameters-old.md` (from `management/sysctls.md`) | Extract the reference table of sysctl values from the old sysctls page. Remove the how-to instructions (those go into `installation/node-configuration.md`). Keep the pure reference: parameter name, recommended value, description. |

### P2 -- New content needed

| Page | Action |
|------|--------|
| `reference/helm-chart-values.md` | **TODO for team:** Create a complete Helm chart values reference. Can be auto-generated from chart's `values.yaml` with comments, or manually documented. Cover all configurable parameters for the operator Helm chart. |
| `reference/compatibility-matrix.md` | **TODO for team:** Create a compatibility matrix. Cover: supported Kubernetes versions, supported ScyllaDB versions (Open Source and Enterprise), supported platforms (GKE, EKS, OpenShift with version ranges), operator-to-ScyllaDB version compatibility. Use the existing substitution variables (`{{supportedKubernetesVersionRange}}`, `{{supportedOpenShiftVersionRange}}`) where appropriate. |

### Cleanup

After P1 extraction is complete:
- Delete `reference/kernel-parameters-old.md` (holding file consumed).

---

## Section 8: Release Notes

### P0 -- Content in place (no changes needed)

| Page | Source | Action |
|------|--------|--------|
| `release-notes/releases.md` | `support/releases.md` (moved) | Review. Verify any cross-references. |

### P3 -- Nice-to-have new content

| Page | Action |
|------|--------|
| `release-notes/migration-guides.md` | **TODO for team:** Write version-to-version migration guides when breaking changes are introduced. Initially this can be a stub that links to relevant sections of release notes that contain breaking changes. |

---

## Section 9: Support

### P0 -- Content in place (no changes needed)

| Page | Source | Action |
|------|--------|--------|
| `support/overview.md` | Existing (kept in place) | Review. If community/contact info is extracted to `contact-community.md`, trim this page to focus on support channels and SLAs. |

### P3 -- Nice-to-have new content

| Page | Action |
|------|--------|
| `support/contact-community.md` | **TODO for team:** Write a community/contact page. Cover: Slack channels, GitHub repository links, mailing lists, community meetings, contributing guide link. Can be extracted from `support/overview.md` if that page already contains this information, or written fresh. |

---

## Cross-Cutting Tasks

These tasks apply across all sections and should be done after the per-section work:

### Cross-Reference Audit

After all content migrations are complete, perform a full cross-reference audit:

1. **Run `make linkcheck`** to find all broken internal and external links.
2. **Search for old paths** in all doc files:
   ```bash
   grep -r "architecture/" docs/source/ --include="*.md"
   grep -r "resources/" docs/source/ --include="*.md"
   grep -r "management/" docs/source/ --include="*.md"
   grep -r "quickstarts/" docs/source/ --include="*.md"
   grep -r "support/troubleshooting" docs/source/ --include="*.md"
   grep -r "support/must-gather" docs/source/ --include="*.md"
   grep -r "support/known-issues" docs/source/ --include="*.md"
   grep -r "support/releases" docs/source/ --include="*.md"
   ```
3. **Update all cross-references** to point to new locations.

### Orphan File Cleanup

Verify all 6 orphan holding files from Stage 1 have been consumed and deleted:

| Holding File | Consumed By | Delete After |
|-------------|-------------|--------------|
| `tutorials/day0/discovery.md` | `tutorials/day0/connect-cql.md` | Section 4 P0 complete |
| `tutorials/day0/create-cluster-scylladbcluster.md` | `tutorials/day0/create-cluster.md` | Section 4 P0 complete |
| `concepts/crd-ecosystem-old.md` | `concepts/crd-ecosystem.md` | Section 2 P1 complete |
| `installation/nodeconfigs-old.md` | `installation/node-configuration.md` | Section 3 P1 complete |
| `reference/kernel-parameters-old.md` | `reference/kernel-parameters.md` | Section 7 P1 complete |
| `operations/upgrading/scylla-upgrade-nodeops.md` | `operations/upgrading/upgrade-scylladb.md` | Section 5 P1 complete |

### Landing Page Update

Rewrite the root `index.md` landing page:
- Update the hero-box text if needed.
- Replace the topic-box grid with links to the new 9 sections.
- Ensure the description text for each section accurately reflects the new content organization.

### Final Build Verification

```bash
cd docs && make dirhtml
```

Confirm zero warnings and zero errors.

---

## Summary Statistics

| Category | Count |
|----------|-------|
| P0 tasks (straightforward review) | 30 pages |
| P1 tasks (extract/merge/split) | 8 pages |
| P2 tasks (critical new content -- TODO for team) | 15 pages |
| P3 tasks (nice-to-have new content -- TODO for team) | 3 pages |
| Holding files to delete after merge | 6 |
| Cross-cutting tasks | 3 (cross-ref audit, orphan cleanup, landing page) |

### Recommended Execution Order

1. **All P0 tasks** (review and verify moved content) -- can be done in parallel across sections
2. **All P1 tasks** (extract/merge) -- depends on P0 being complete so source material is stable
3. **Delete holding files** -- after P1 tasks consume them
4. **Cross-reference audit** -- after all moves and merges are complete
5. **Landing page update** -- after structure is finalized
6. **P2 tasks** (critical new content) -- can be assigned to team members in parallel
7. **P3 tasks** (nice-to-have) -- lowest priority, defer as needed
8. **Final build verification** -- after all content work is complete
