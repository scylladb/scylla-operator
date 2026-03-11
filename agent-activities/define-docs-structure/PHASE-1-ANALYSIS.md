# Phase 1: Comparative Analysis of Documentation Sites

## Overview

This analysis evaluates 10 documentation sites against three criteria derived from the [Diátaxis framework](https://diataxis.fr/):

1. **Diátaxis Score (1-5):** How well does the site separate *Tutorials* (Learning), *How-to Guides* (Tasks), *Reference* (Information), and *Explanation* (Understanding)?
   - *1 = Mixed/Messy, 5 = Perfect Separation*
2. **General Quality Score (1-5):** Assessed on clarity, navigation, "Production Readiness" guidance, and visual aids.
3. **Domain Similarity (1-5):** How similar is the software's operational complexity to ScyllaDB (a distributed, stateful NoSQL database on Kubernetes)?
   - *1 = Stateless/Simple, 5 = Stateful/Distributed*

---

## Summary Table

| # | Documentation Site | Diátaxis (1-5) | Quality (1-5) | Domain Similarity (1-5) | Key "Stealable" Feature |
|---|---|:---:|:---:|:---:|---|
| 1 | **ScyllaDB Operator** (Target) | 2 | 3 | 5 | IPv6 section already uses Diátaxis sub-structure (tutorials/how-to/reference/concepts) |
| 2 | **Strimzi** (Kafka) | 3 | 3.5 | 4 | Separation of "Overview" / "Deploying & Managing" / "API Reference" as distinct books |
| 3 | **CloudNativePG** (Postgres) | 2.5 | 4 | 3.5 | "Before You Start" prerequisites page; dedicated "Failure Modes" page |
| 4 | **CockroachDB** | 3.5 | 5 | 3 | "Production Checklist" -- single authoritative page covering topology, hardware, security, networking, monitoring, clock sync |
| 5 | **Cert-Manager** | 4 | 4.5 | 1 | Numbered lifecycle workflow (0->1->2->3->4) guiding the user through a natural progression |
| 6 | **K8s Gateway API** | 4.5 | 4.5 | 1 | Role-oriented personas (Infrastructure Provider / Cluster Operator / App Developer) |
| 7 | **K8ssandra** (Cassandra) | 3 | 3 | 5 | Task-oriented structure: Connect, Scale, Secure, Backup, Repair, Monitor; persona-based quickstarts (Developer vs SRE) |
| 8 | **StackGres** (Postgres) | 3.5 | 4 | 3.5 | "Runbooks" section with concrete operational scenarios; "Features" section explaining capabilities before the admin manual |
| 9 | **Crossplane** | 4 | 4 | 1.5 | Welcome page explaining how to navigate the docs; concept-first then guides then reference |
| 10 | **Crunchy PGO** | 4.5 | 4.5 | 3.5 | Perfect Diátaxis: Quickstart / Tutorials (Day-0->1->2) / Guides (task-specific) / Architecture / References |

---

## Detailed Justifications

### 1. ScyllaDB Operator (Target)

**Diátaxis: 2 | Quality: 3 | Domain: 5**

The current documentation organizes content by *resource type* (ScyllaClusters, ScyllaDBClusters, NodeConfigs) and *infrastructure concern* (Architecture, Installation, Management) rather than by user task or Diátaxis quadrant. Tutorials, how-to guides, and reference material are mixed together within the "Resources" section -- for example, the ScyllaClusters subsection contains node operation how-to guides, client connection tutorials, and multi-DC deployment instructions all at the same level.

**Strengths:**
- The IPv6 networking subsection is a proof-of-concept that Diátaxis works here: it properly separates tutorials, how-to, reference, and concepts into distinct subdirectories.
- Architecture section provides genuine explanation content.
- API Reference is cleanly auto-generated.

**Weaknesses:**
- Quickstarts are buried after Resources in the sidebar (position 5 of 7), violating the user journey principle.
- No progressive learning path (tutorials that build on each other).
- No production readiness guide or checklist.
- The "Management" section mixes unrelated operational concerns (sysctls, bootstrap sync, data cleanup, upgrading, monitoring, networking) without clear categorization.
- Troubleshooting is buried inside Support with only one page (installation issues).

---

### 2. Strimzi (Kafka)

**Diátaxis: 3 | Quality: 3.5 | Domain: 4**

Strimzi separates its documentation into distinct "books" -- Overview, Deploying/Managing/Upgrading, and API Reference (called "Configuring"). This is a meaningful step toward Diátaxis separation, giving users clear entry points based on their intent.

**Strengths:**
- Clear book-based separation: users can go to "Overview" to learn, "Deploying" to do, "Configuring" to look up.
- Quickstart is prominently featured on the documentation landing page.
- Example custom resources are linked from GitHub, keeping docs focused.

**Weaknesses:**
- The "Deploying, Managing, and Upgrading" guide conflates tutorials with operational procedures -- it is one long book rather than separated by type.
- No dedicated Concepts/Explanation section.
- The landing page lists links but doesn't guide the user through a journey.

**Domain Similarity:** High. Kafka is a distributed, stateful system with brokers, replication, rack awareness, consumer groups, and multi-DC concerns. Strimzi's operator must handle rolling upgrades, topic management, and user authentication -- similar operational complexity to ScyllaDB.

---

### 3. CloudNativePG (Postgres)

**Diátaxis: 2.5 | Quality: 4 | Domain: 3.5**

CloudNativePG has an extremely comprehensive flat topic list -- over 40 pages at the same sidebar level covering everything from Bootstrap to Fencing to PostGIS. The quality of individual pages is excellent: thorough, well-structured, with clear code examples and explanations.

**Strengths:**
- Extremely thorough coverage -- every operational concern has a dedicated page.
- "Before You Start" prerequisites section is valuable for onboarding.
- Dedicated "Failure Modes" page explains what can go wrong and how the operator handles it.
- "Operator Capability Levels" page sets expectations clearly.

**Weaknesses:**
- The flat structure means there is no Diátaxis separation -- a page on "Backup" is at the same level as "PostGIS" and "Fencing."
- No tutorials or progressive learning paths.
- A user must know what they're looking for to navigate effectively; discoverability is poor.

**Domain Similarity:** Moderate-to-high. PostgreSQL on Kubernetes is stateful with persistent storage, HA, backup/recovery, and replication concerns. However, it's a single-primary system with replicas, not a distributed/sharded database like ScyllaDB. The operational complexity is lower (no racks, no multi-shard coordination, no repair/compaction).

---

### 4. CockroachDB

**Diátaxis: 3.5 | Quality: 5 | Domain: 3**

CockroachDB documentation is the gold standard for production database guidance. The "Production Checklist" page alone is one of the most valuable pieces of database documentation on the internet -- it covers topology, hardware sizing (with specific vCPU/RAM/IOPS ratios), storage recommendations, networking configurations, security best practices, clock synchronization, cache sizing, and cloud-specific configurations (AWS, GCP, Azure) in a single authoritative document.

**Strengths:**
- "Production Checklist" is the definitive example of what ScyllaDB Operator docs should include.
- Excellent separation at the top level: Quickstart, Architecture, Deploy guides, SQL reference, FAQ.
- Cloud-specific recommendations are detailed and prescriptive (specific instance types, disk types, IOPS provisioning).
- Deep technical content -- e.g., clock synchronization section explains NTP, leap second smearing, and PTP hardware clocks.

**Weaknesses:**
- Not a K8s operator -- the docs cover bare-metal, VM, and cloud deployments, so the K8s-specific structure doesn't directly translate.
- Some sections are extremely long (the Production Checklist itself is a wall of text that could benefit from sub-navigation).

**Domain Similarity:** Moderate. CockroachDB is a distributed SQL database with racks/AZs, replication, and multi-region concerns. However, it's not a NoSQL wide-column store and the operational model (self-managed binary vs. K8s operator) is fundamentally different.

---

### 5. Cert-Manager

**Diátaxis: 4 | Quality: 4.5 | Domain: 1**

Cert-Manager is one of the best-structured Kubernetes operator documentation sets. It uses a numbered lifecycle workflow that guides users through the natural progression of working with certificates:

```
0. Installation -> 1. Configuring Issuers -> 2. Requesting Certificates -> 3. Distributing Trust -> 4. Defining Policy
```

This numbered approach is immediately intuitive -- users understand they need to complete step 0 before step 1.

**Strengths:**
- Numbered lifecycle sections create a clear user journey.
- Separate Tutorials section with real-world scenarios (NGINX + Let's Encrypt, GKE + Ingress, AKS + LoadBalancer).
- "DevOps Tips" section for operational concerns (Prometheus metrics, backup/restore, scaling).
- Reference section cleanly separated with API docs, CLI reference, and concepts.

**Weaknesses:**
- Domain is fundamentally different -- certificate management is stateless and simple compared to distributed database operations.
- The numbered workflow only works because cert-manager has a linear lifecycle; ScyllaDB's operational model is more branching.

**Domain Similarity:** Minimal. Cert-Manager manages TLS certificates -- it is stateless, has no data persistence concerns, no distributed coordination, and no performance tuning requirements.

---

### 6. Kubernetes Gateway API

**Diátaxis: 4.5 | Quality: 4.5 | Domain: 1**

Near-perfect Diátaxis implementation. The documentation is clearly separated into:
- **Overview** (Concepts: API Overview, Roles, Security, Versioning, Use Cases)
- **Guides** (Getting Started tutorials, User Guides with specific tasks)
- **Reference** (API Types, API Specification)
- **Enhancements** (GEPs -- design decisions and proposals)

**Strengths:**
- The role-oriented persona approach (Ian the Infrastructure Provider, Chihiro the Cluster Operator, Ana the Application Developer) is genuinely innovative. This could be adapted for ScyllaDB where a Platform Engineer, a DBA, and an Application Developer have different documentation needs.
- Clean MkDocs Material theme with excellent navigation.
- Concepts section covers troubleshooting, conformance, and tooling separately from reference.

**Weaknesses:**
- Domain is fundamentally different -- a networking API spec, not an operator managing stateful workloads.
- The persona model requires significant content investment to implement properly.

**Domain Similarity:** Minimal. Gateway API is a stateless API specification -- no data, no storage, no operational lifecycle.

---

### 7. K8ssandra (Cassandra)

**Diátaxis: 3 | Quality: 3 | Domain: 5**

K8ssandra is the closest domain match to ScyllaDB Operator -- Apache Cassandra is a wide-column NoSQL distributed database with racks, repair, compaction, multi-DC replication, and similar operational characteristics.

**Strengths:**
- Task-oriented `/tasks/` structure maps directly to Day-2 operations: Connect, Migrate, Scale, Secure, Backup/Restore, Repair, Monitor, Troubleshoot.
- Persona-based quickstarts: "Developer" and "SRE" get different quickstart paths, recognizing different needs.
- Components section explains each sub-system (Operator, Stargate, Cass Operator, Reaper, Medusa, Metrics Collector) with architecture diagrams.
- Rack affinities are a first-class documented concept -- directly relevant for ScyllaDB.

**Weaknesses:**
- Overall polish and depth is lower than other documentation sets -- some pages feel sparse.
- The Diátaxis separation is incomplete: Components section mixes explanation with reference; Tasks section mixes tutorials with how-to guides.
- No production checklist or readiness guide.
- The CRD reference section is bloated with version-specific pages (30+ CRD pages for individual releases).

---

### 8. StackGres (Postgres)

**Diátaxis: 3.5 | Quality: 4 | Domain: 3.5**

StackGres has a thoughtful structure: Introduction (concepts/architecture) -> Features (explanation) -> Getting Started -> Administration Manual (how-to) -> CRD Reference -> Developer Documentation -> API Reference -> Runbooks.

**Strengths:**
- **"Runbooks" section is outstanding** -- concrete operational scenarios like "Volume Downsize," "Backup Large Databases," "Maintenance with Zero-Downtime," "Recovering PGDATA from Existing Volume." These are the kind of real-world guides that operators need but rarely find in docs.
- "Features" section explains every capability *before* the Administration Manual -- users understand what's possible before learning how to do it.
- "Administration Manual" is organized as a progressive guide: Creating -> Connecting -> Backups -> Configuration -> SQL Scripts -> Extensions -> Monitoring -> HA -> Security -> Tuning.
- CRD Reference is clean and focused.

**Weaknesses:**
- The "Administration Manual" is essentially one large how-to guide without clear separation between different task types.
- "Developer Documentation" (building StackGres, running e2e tests) is mixed in with user-facing docs rather than in a contributing guide.

**Domain Similarity:** Moderate-to-high. PostgreSQL on K8s with Patroni HA, backup/restore via WAL-G, connection pooling, monitoring, and sharding (newer feature). More complex than CloudNativePG but still a single-primary architecture, not distributed/sharded like ScyllaDB.

---

### 9. Crossplane

**Diátaxis: 4 | Quality: 4 | Domain: 1.5**

Crossplane has a clean, concept-first structure with excellent onboarding. The Welcome page explicitly explains how to navigate the documentation -- a simple but effective technique that most projects skip.

**Strengths:**
- "What's Crossplane?" -> "What's New?" -> "Get Started" -> concept domains -> "Guides" -> "Reference" is a natural flow.
- "Get Started" has three separate paths (Composition, Managed Resources, Operations) recognizing different user intents.
- Guides are clearly task-oriented and labeled as such.
- Clean separation between concept documentation (Composition, Managed Resources, Operations, Packages) and reference documentation (CLI, API).

**Weaknesses:**
- Domain is fundamentally different -- Crossplane is an infrastructure orchestration framework, not a database operator.
- Some sections feel overly nested (Managed Resources has 4 sub-pages covering related but distinct topics).

**Domain Similarity:** Low. Crossplane manages infrastructure resources declaratively. It has no data persistence, no performance tuning, no distributed coordination.

---

### 10. Crunchy PGO (Postgres Operator)

**Diátaxis: 4.5 | Quality: 4.5 | Domain: 3.5**

**This is the single best template for ScyllaDB Operator docs.** Crunchy PGO achieves near-perfect Diátaxis separation with a structure that is both intuitive and rigorous:

```
Quickstart -> Overview -> Installation -> Tutorials -> Guides -> Architecture -> Upgrade -> FAQ -> Releases -> References -> Support
```

**Strengths:**
- **Tutorials are structured as a Day-0/1/2 lifecycle:** "Basic Setup" -> "Backup and Disaster Recovery" -> "Day Two Tasks" -> "Cluster Management." This matches how operators actually learn and work.
- **Guides are task-specific and unordered:** "Auto-Growable Disk," "Logical Replication," "Major Version Upgrade," "Huge Pages," "Volume Snapshots" -- each is a self-contained how-to for a specific need.
- **Quickstart is a single page** -- get running fast, learn later.
- **Architecture is a separate section** -- not mixed into tutorials or guides.
- **References section** is distinct from Guides -- API docs, Helm values, configuration options.
- PDF export available for offline access.

**Weaknesses:**
- PostgreSQL is not distributed/sharded -- the operational complexity is lower than ScyllaDB.
- Some guides could benefit from more depth (they are concise, sometimes to a fault).

**Domain Similarity:** Moderate-to-high. PGO manages PostgreSQL on Kubernetes with HA, backup/DR, connection pooling, and monitoring. The operator pattern and lifecycle management are directly applicable. The gap is in distributed database concerns (racks, repair, compaction, multi-DC sharding).

---

## Key Takeaways for Phase 2

### Primary structural inspiration: Crunchy PGO

The Quickstart -> Tutorials (Day-0/1/2) -> Guides (task-specific) -> Architecture -> Reference model is the cleanest, most scalable structure and the closest match to our needs.

### Critical features to adopt from other sources

| Feature | Source | Why |
|---|---|---|
| Production Checklist page | CockroachDB | ScyllaDB has complex hardware, storage, and tuning requirements that deserve a single authoritative reference |
| Task-verb organization for operations | K8ssandra | Scale, Secure, Backup, Repair, Monitor, Troubleshoot -- maps to distributed database Day-2 concerns |
| Runbooks for advanced scenarios | StackGres | Real-world operational scenarios (node recovery, volume resize, zero-downtime maintenance) |
| "Before You Start" prerequisites | CloudNativePG | Establish baseline knowledge before diving into tutorials |
| Failure Modes documentation | CloudNativePG | Critical for a distributed database -- what happens when a node dies, a disk fails, network partitions |
| Role-based content tagging | Gateway API | Help Platform Engineers and Application Developers find relevant content |
| Numbered lifecycle progression in tutorials | Cert-Manager | Day 0 -> Day 1 -> Day 2 is the natural learning journey |
| Persona-based quickstarts | K8ssandra | "Developer" vs "SRE" quickstart paths |

### Content gaps in current ScyllaDB Operator docs

1. **No production readiness guide** -- users have no checklist for going to production
2. **No progressive learning path** -- no tutorials that build on each other
3. **No failure modes documentation** -- critical for a distributed database
4. **No security guide** -- TLS, authentication, RBAC are not documented
5. **No repair/compaction guidance** -- fundamental ScyllaDB operations
6. **Troubleshooting is minimal** -- one page for installation issues only
7. **No performance benchmarking guide** -- users need to validate their setup
8. **No migration guide** -- neither ScyllaDB data migrations nor operator version migrations
