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

This section is the current-state audit for `docs/source` on this branch.

Rules applied:
- Diataxis is classified from the page's present structure and writing mode.
- Quality notes are concise and focus on documentation quality only (clarity, orientation, verification, cross-links), not page-content detail.
- Scope includes Markdown pages under `docs/source`, excluding maintenance files: `DOCS_HINTS.md`, `DOCS_STRUCTURE.md`, `DOCS_WORKLOG.md`, and this file.

## `/`

- `index.md` — **Navigation** — Strong landing structure with clear persona entry points; progression cues can still be tighter.

## `get-started/`

- `get-started/index.md` — **Navigation** — Explanation-focused entry page routing users to conceptual content; links to install and deploy sections for action.
- `get-started/what-is-scylladb-operator.md` — **Explanation** — Good conceptual framing with useful context links.
- `get-started/concepts-for-k8s-beginners.md` — **Explanation** — Effective bridge material; action handoff can be sharper.

## `install-operator/`

- `install-operator/index.md` — **Navigation** — Explicit three-path routing (GitOps, Helm, OpenShift) with prerequisite link.
- `install-operator/prerequisites.md` — **Reference** — Broad prerequisite coverage with good completeness cues.
- `install-operator/install-with-gitops.md` — **How-to** — Clear task-first procedure with useful verification points.
- `install-operator/install-with-helm.md` — **How-to** — Practical and well-linked; validation depth varies by step.
- `install-operator/install-on-openshift.md` — **How-to** — Comprehensive and verification-rich despite high detail density.
- `install-operator/set-up-multi-dc-infrastructure.md` — **How-to** — Actionable infrastructure sequence with strong command guidance.

## `deploy-scylladb/`

- `deploy-scylladb/index.md` — **Navigation** — Routes users to quick deploy path or platform-specific reference deployments.
- `deploy-scylladb/deploy-your-first-cluster.md` — **How-to** — Quick path (minimal dev cluster) with progressive disclosure into production configuration. Merged from former deploy-single-dc-cluster.md.
- `deploy-scylladb/reference-deployment-gke.md` — **Tutorial** — End-to-end GKE deployment from cluster creation through cqlsh verification and cleanup. Moved from former get-started/quickstart-gke.md.
- `deploy-scylladb/reference-deployment-eks.md` — **Tutorial** — End-to-end EKS deployment from cluster creation through cqlsh verification and cleanup. Moved from former get-started/quickstart-eks.md.
- `deploy-scylladb/reference-deployment-openshift.md` — **How-to** — Thin orchestration page directing users to OpenShift install guide then deploy-your-first-cluster.
- `deploy-scylladb/before-you-deploy/index.md` — **Navigation** — Useful subsection hub; decision cues can be stronger.
- `deploy-scylladb/before-you-deploy/configure-operator.md` — **How-to** — Focused and clear; success criteria are brief.
- `deploy-scylladb/before-you-deploy/set-up-dedicated-node-pools.md` — **How-to** — Practical and complete; completion criteria can be sharper.
- `deploy-scylladb/before-you-deploy/configure-cpu-pinning.md` — **How-to** — Detailed procedural flow with strong checks; scanability is moderate.
- `deploy-scylladb/before-you-deploy/configure-nodes.md` — **How-to** — Runbook-quality guidance with strong completeness cues.
- `deploy-scylladb/deploy-multi-dc-cluster.md` — **Tutorial** — Comprehensive walkthrough; cognitive load is higher due to depth.
- `deploy-scylladb/set-up-networking/index.md` — **Navigation** — Adequate networking hub; recommended next steps can be clearer.
- `deploy-scylladb/set-up-networking/expose-clusters.md` — **How-to** — Practical and command-driven with solid usability.
- `deploy-scylladb/set-up-networking/ipv6/index.md` — **Navigation** — Useful IPv6 hub; reading-order guidance can improve.
- `deploy-scylladb/set-up-networking/ipv6/get-started.md` — **Tutorial** — Strong guided path with meaningful checks.
- `deploy-scylladb/set-up-networking/ipv6/configure-dual-stack.md` — **How-to** — Well-structured procedure with good validation cues.
- `deploy-scylladb/set-up-networking/ipv6/configure-single-stack.md` — **How-to** — Clear and verification-oriented with strong sequencing.
- `deploy-scylladb/set-up-networking/ipv6/migration.md` — **How-to** — Strong migration workflow with useful checkpoints.
- `deploy-scylladb/set-up-networking/ipv6/troubleshooting.md` — **How-to** — High diagnostic utility with actionable routing.
- `deploy-scylladb/set-up-monitoring.md` — **How-to** — Strong task framing and good verification support.
- `deploy-scylladb/expose-grafana.md` — **How-to** — Clear sequence and practical examples; completion criteria are concise.
- `deploy-scylladb/production-checklist.md` — **How-to** — High-value operational checklist with strong cross-linking.

## `connect-your-app/`

- `connect-your-app/index.md` — **Navigation** — Clean section entry; path selection hints can be richer.
- `connect-your-app/connect-via-cql.md` — **How-to** — Practical and procedure-first with good operator usability.
- `connect-your-app/alternator.md` — **How-to** — Actionable flow with decent verification cues.
- `connect-your-app/discovery.md` — **Reference** — Useful lookup-style page with concise guidance.
- `connect-your-app/configure-external-access.md` — **How-to** — Task-oriented and structured with practical validation.

## `understand/`

- `understand/index.md` — **Navigation** — Good conceptual hub; suggested learning order can be clearer.
- `understand/overview.md` — **Explanation** — Strong architectural orientation and contextual linking.
- `understand/storage.md` — **Explanation** — Clear conceptual model with good framing.
- `understand/tuning.md` — **Explanation** — Valuable mechanism-level coverage; action-link density can be improved.
- `understand/manager.md` — **Explanation** — Well-structured conceptual page with good contextual depth.
- `understand/networking.md` — **Explanation** — Broad conceptual coverage; dense segments reduce quick scanability.
- `understand/monitoring.md` — **Explanation** — Clear architecture-level explanation with concise scope.
- `understand/bootstrap-sync.md` — **Explanation** — Strong mechanism clarity and solid coherence.
- `understand/automatic-data-cleanup.md` — **Explanation** — Focused conceptual treatment with consistent framing.
- `understand/sidecar.md` — **Explanation** — Rich internals coverage; readability is affected by density.
- `understand/ignition.md` — **Explanation** — Concise and coherent mechanism explanation.
- `understand/pod-disruption-budgets.md` — **Explanation** — Clear intent and behavior framing.
- `understand/security.md` — **Explanation** — Broad and informative with helpful context links.
- `understand/statefulsets-and-racks.md` — **Explanation** — Well-scoped conceptual mapping with clear operational implications.

## `operate/`

- `operate/index.md` — **Navigation** — Effective day-2 entry; workflow prioritization can be improved.
- `operate/scale-cluster.md` — **How-to** — Clear sequence with practical commands and solid flow.
- `operate/replace-nodes.md` — **How-to** — Strong runbook style and clear task orientation.
- `operate/expand-storage-volumes.md` — **How-to** — Actionable structure with useful verification cues.
- `operate/use-maintenance-mode.md` — **How-to** — Direct and easy to follow with concise safety framing.
- `operate/back-up-and-restore.md` — **Navigation** — Useful split-entry page; outcome cues can be stronger.
- `operate/schedule-backups.md` — **How-to** — Detailed and practical with strong completeness signals.
- `operate/restore-from-backup.md` — **How-to** — Structured and actionable with adequate checks.
- `operate/perform-rolling-restart.md` — **How-to** — Short and clear; rollback guidance is minimal.
- `operate/migrate-rack-to-new-node-pool.md` — **How-to** — Good stepwise migration guidance.
- `operate/pass-scylladb-arguments.md` — **How-to** — Concise and useful with practical execution guidance.
- `operate/configure-io-properties.md` — **How-to** — Focused and readable with good task orientation.

## `upgrade/`

- `upgrade/index.md` — **Navigation** — Clear section entry; scenario-based routing can improve.
- `upgrade/upgrade-operator.md` — **How-to** — Strong upgrade procedure with clear safeguards and checks.
- `upgrade/upgrade-scylladb.md` — **How-to** — Actionable and detailed with solid verification.
- `upgrade/upgrade-kubernetes.md` — **How-to** — Practical workflow with good validation support.

## `troubleshoot/`

- `troubleshoot/index.md` — **Navigation** — Strong symptom-based hub with good link coverage.
- `troubleshoot/diagnostic-flowchart.md` — **Navigation** — Effective decision-routing with strong cross-linking.
- `troubleshoot/troubleshoot-installation.md` — **How-to** — Well-structured runbook with clear verification points.
- `troubleshoot/check-cluster-health.md` — **How-to** — High-quality health-check flow with strong validation orientation.
- `troubleshoot/investigate-restarts.md` — **How-to** — Systematic investigation path with good completeness.
- `troubleshoot/diagnose-node-not-starting.md` — **How-to** — Strong diagnostic flow with actionable progression.
- `troubleshoot/change-log-level.md` — **How-to** — Focused operational procedure with clear intent.
- `troubleshoot/recover-from-failed-replace.md` — **How-to** — Detailed recovery runbook with robust structure.
- `troubleshoot/troubleshoot-performance.md` — **How-to** — Robust diagnostic workflow; long sections affect quick scanning.
- `troubleshoot/common-errors.md` — **Reference** — Effective quick-lookup mapping with concise guidance.
- `troubleshoot/configure-coredumps.md` — **How-to** — Structured and practical with useful verification hints.
- `troubleshoot/collect-debugging-information/index.md` — **Navigation** — Clear data-collection hub; method-selection guidance can be richer.
- `troubleshoot/collect-debugging-information/must-gather.md` — **How-to** — Practical command guidance with strong usability.
- `troubleshoot/collect-debugging-information/must-gather-contents.md` — **Reference** — Useful artifact lookup with concise interpretation support.
- `troubleshoot/collect-debugging-information/system-tables.md` — **How-to** — Clear query-driven diagnostic procedure.

## `reference/`

- `reference/index.md` — **Navigation** — Clear reference landing with strong discoverability.
- `reference/feature-gates.md` — **Reference** — Useful parameter lookup with consistent clarity.
- `reference/ipv6-configuration.md` — **Reference** — Strong configuration lookup utility.
- `reference/releases.md` — **Reference** — Comprehensive support and compatibility matrix page.
- `reference/known-issues.md` — **Reference** — Helpful issue catalog with practical links.
- `reference/conditions.md` — **Reference** — Well-scoped lookup page with good structure.
- `reference/sizing-guide.md` — **Reference** — Clear capacity guidance with strong completeness cues.
- `reference/benchmarking.md` — **How-to** — Procedure-oriented benchmarking workflow with practical checkpoints.
- `reference/glossary.md` — **Reference** — Useful terminology baseline supporting consistency.
- `reference/nodetool-alternatives.md` — **Reference** — High-value comparison reference; density affects scan speed.

## `contributing/`

- `contributing/index.md` — **How-to** — Good contributor workflow orientation with clear entry actions.

## Notes

- This is an as-built structure/status map, not a forward migration plan.
- Auto-generated API reference under `docs/source/reference/api/` remains out of scope for this Markdown audit.
