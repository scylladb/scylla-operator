## Context

We are aiming to restructure the [ScyllaDB Operator Documentation](https://operator.docs.scylladb.com/stable/) to become a world-class example of Kubernetes documentation. To do this, we must first analyze the current "Gold Standards" in the ecosystem, score them against rigorous criteria (specifically the [Diátaxis framework](https://diataxis.fr/)), and then synthesize a new, ideal information architecture for ScyllaDB.

## Phase 1: Comparative Analysis

**Goal:** Analyze the following documentation sites to understand why they are successful and how close they are to our domain.

**Input URLs to Analyze:**

1. **ScyllaDB Operator (Target):** `https://operator.docs.scylladb.com/stable/`
2. **Strimzi (Kafka):** `https://strimzi.io/documentation/`
3. **CloudNativePG (Postgres):** `https://cloudnative-pg.io/documentation/current/`
4. **CockroachDB (DB):** `https://www.cockroachlabs.com/docs/stable/recommended-production-settings`
5. **Cert-Manager (Security):** `https://cert-manager.io/docs/`
6. **Kubernetes Gateway API (Standard):** `https://gateway-api.sigs.k8s.io/`
7. **K8ssandra (Cassandra - High Domain Similarity):** `https://docs.k8ssandra.io/
8. **StackGres (Postgres):** `https://stackgres.io/doc/latest/`
9. **Crossplane:** `https://docs.crossplane.io/v2.2/`
10. **Crunchy Postgres for Kubernetes:** `https://access.crunchydata.com/documentation/postgres-operator/latest`

**Action:**
For *each* of the URLs above, generate a structured analysis containing:

1. **Diátaxis Score (1-5):** How well does it separate *Tutorials* (Learning), *How-to* (Tasks), *Reference* (Information), and *Explanation* (Understanding)?
* *1 = Mixed/Messy, 5 = Perfect Separation.*

2. **General Quality Score (1-5):** Assessed on clarity, navigation, "Production Readiness" guides, and visual aids.
3. **Domain Similarity (1-5):** How similar is the software's operational complexity to ScyllaDB (a distributed, stateful NoSQL database)?
* *1 = Stateless/Simple (e.g., Cert-Manager), 5 = Stateful/Distributed (e.g., K8ssandra).*

4. **Key "Stealable" Feature:** One specific structural idea or section pattern we should adopt (e.g., "The way Strimzi separates User vs. Dev" or "CockroachDB's Production Checklist").

**Output Format for Phase 1:**
Produce a Markdown table summarizing the scores, followed by a brief text justification for each.

## Phase 2: The Golden Standard Structure

**Goal:** Create the ideal Table of Contents (ToC) for the ScyllaDB Operator, ignoring the constraints of the *current* docs but focusing on the *needs* of a ScyllaDB user.

**Guiding Principles:**

* **Adhere to Diátaxis:** Distinctly separate *getting started* from *production maintenance* and *architectural concepts*.
* **User Journey Focus:** The structure must flow from "Day 0" (Install/Try) to "Day 1" (Config/Secure) to "Day 2" (Backup/Upgrade/Repair).
* **Domain Specificity:** It must include sections relevant to distributed data (e.g., Racks, AZs, Repairs, Compaction) which simpler operators do not have.

**Action:**
Generate a detailed hierarchical outline (H1, H2, H3) for the new documentation.

* **Annotate** each major section with *[Type]* (e.g., [Tutorial], [Guide], [Explanation], [Reference]).
* **Annotate** specific sections with *[Source of Inspiration]* (e.g., "Modeled after CloudNativePG's Architecture section").

**Example Structure to Generate:**

```markdown
# 1. Get Started (Tutorials)
   - Quickstart: 3-Node Cluster (Kind/Minikube)
   - First App: Connecting a Scylla Driver to K8s
# 2. Concepts (Explanation)
   - Architecture: The Operator Pattern
   - Storage: Local NVMe vs. Network Storage
   ...
# 3. Operations (How-to Guides)
   ...
```
