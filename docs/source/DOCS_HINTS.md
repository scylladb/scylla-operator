# Documentation Quality Checklist

## DOCS STRUCTURE DEFINITION DIRECTIVES
1. Define clear information hierarchy with logical grouping of related topics
1. Create consistent navigation patterns that reduce cognitive load
1. Design clear entry points for different user personas and skill levels
1. Each document should have a well-defined Diataxis category.

## DOCS WRITING DIRECTIVES
1. Establish naming conventions for all documentation artifacts
1. Implement cross-referencing system between related concepts
1. Use consistent terminology throughout all documentation
1. Eliminate ambiguous language that could be interpreted multiple ways
1. Specify exact command syntax with required and optional parameters
1. Include expected outputs for all executable examples
1. Provide explicit success criteria for each procedure
1. Document side effects and dependencies for all operations

## Audience Analysis
1. Identify all user personas with their goals, pain points, and expertise levels

   **Platform / Kubernetes Administrator** (Advanced)
   - *Goals:* Install and upgrade the Operator and its dependencies, configure Kubernetes prerequisites (CPU policy, node labels, xfsprogs, firewalls), manage cluster-scoped resources (`NodeConfig`, `ScyllaOperatorConfig`), set up multi-cluster networking.
   - *Pain points:* Platform-specific incompatibilities (Container OS, Bottlerocket), webhook connectivity issues with non-conformant networking, complex multi-DC infrastructure setup, understanding cluster-admin vs. namespace-level privileges.

   **Database Operator / SRE** (Intermediate–Advanced)
   - *Goals:* Create and configure `ScyllaCluster`/`ScyllaDBCluster` resources, perform day-2 operations (scaling, upgrading, node replacement, volume expansion, maintenance mode), manage backups/repairs via ScyllaDB Manager, set up monitoring, deploy multi-DC clusters.
   - *Pain points:* Complex upgrade paths with version-specific steps, manual restore procedures via `sctool`, balancing between `ScyllaCluster` (single-DC) and `ScyllaDBCluster` (multi-DC, tech preview), bootstrap sync requirements.

   **Application Developer** (Beginner–Intermediate in Kubernetes)
   - *Goals:* Connect to ScyllaDB from application code (CQL or Alternator/DynamoDB API), discover node endpoints for driver configuration, set up authentication credentials, understand TLS connectivity.
   - *Pain points:* Understanding Kubernetes networking concepts (ClusterIP, LoadBalancer, headless) for connection strings, TLS certificate management for remote connections, navigating between ScyllaDB-native and Operator-specific docs.

   **Support Engineer / Incident Responder** (Intermediate–Advanced)
   - *Goals:* Collect diagnostics using `must-gather`, troubleshoot installation and cluster health issues, check known issues and workarounds, verify version compatibility.
   - *Pain points:* Limited troubleshooting coverage (only installation issues), no log message reference or error code documentation, no diagnostic flowcharts or decision trees.

   **Junior Support Engineer** (Beginner in Kubernetes, Intermediate in ScyllaDB)
   - *Goals:* Triage basic ScyllaDB issues on Kubernetes using familiar ScyllaDB concepts, collect diagnostics with step-by-step guidance, escalate issues with enough context for senior engineers, map ScyllaDB-native troubleshooting knowledge onto Kubernetes-managed clusters.
   - *Pain points:* Unfamiliar with `kubectl`, pod lifecycles, and Kubernetes networking—needs docs to bridge ScyllaDB concepts to their Kubernetes equivalents (e.g., node → pod, service config file → CR spec). Struggles to interpret Kubernetes-layer failures (CrashLoopBackOff, pending pods, webhook errors) that mask underlying ScyllaDB issues. Lacks context on when a problem is a Kubernetes issue vs. a ScyllaDB issue. Needs copy-paste-ready commands rather than commands that require Kubernetes fluency to adapt.

   **Contributor / Operator Developer** (Advanced)
   - *Goals:* Set up local development environment, build/test/debug the Operator, understand API conventions and coding standards, run E2E tests locally, submit PRs that pass CI.
   - *Pain points:* Complex dependency chain, rootless Podman setup for Kind, no explicit development guide beyond CONTRIBUTING.md.

   **Evaluator / Architect** (Intermediate)
   - *Goals:* Understand what the Operator solves, evaluate architecture and design decisions, assess platform support and compatibility, review release cadence and support lifecycle, compare single-DC vs. multi-DC models.
   - *Pain points:* No dedicated "Why ScyllaDB Operator" page, limited security documentation, no capacity planning guidance, multi-DC support marked as tech preview.

1. Map user journeys from initial setup through advanced use cases

   **Platform / Kubernetes Administrator**
   1. Evaluate platform requirements and compatibility (supported K8s versions, platforms, architectures).
   1. Prepare Kubernetes cluster (CPU manager policy, node labels, xfsprogs, firewall rules, storage class).
   1. Install the Operator and its dependencies (cert-manager, Prometheus Operator, Local CSI Driver).
   1. Configure cluster-scoped resources (NodeConfig for tuning/RAID/filesystem, ScyllaOperatorConfig for global settings).
   1. Set up networking (expose options, multi-cluster interconnectivity, IPv6 if needed).
   1. Plan capacity (node sizing, storage, network bandwidth) and document environment-specific constraints.
   1. Upgrade the Operator across versions, validate cluster health post-upgrade.
   1. Troubleshoot platform-level issues (webhook connectivity, kubelet misconfiguration, storage provisioner failures).

   **Database Operator / SRE**
   1. Deploy a first ScyllaDB cluster using a quickstart or guided tutorial.
   1. Configure the cluster (racks, zones, ScyllaDB config files, resource requests, storage).
   1. Set up monitoring (ScyllaDBMonitoring, Prometheus, Grafana dashboards).
   1. Connect ScyllaDB Manager for automated repairs and backups; schedule backup tasks.
   1. Perform routine operations: scaling out/in, upgrading ScyllaDB versions, expanding volumes.
   1. Handle incidents: replace a failed node, enable maintenance mode, trigger manual repairs.
   1. Restore from backup to a new cluster; validate data integrity after restore.
   1. Deploy a multi-datacenter cluster (single-DC ScyllaCluster first, then multi-DC via ScyllaDBCluster).
   1. Optimize performance (tuning profiles, CPU pinning verification, sysctl adjustments).
   1. Upgrade the Operator and ScyllaDB together across a maintenance window; roll back if needed.

   **Application Developer**
   1. Understand what the Operator provides and how applications connect to ScyllaDB on Kubernetes.
   1. Discover cluster endpoints (headless service DNS, discovery endpoint, LoadBalancer).
   1. Connect via CQL with authentication and TLS from inside the cluster.
   1. Connect via CQL from outside the cluster (LoadBalancer, TLS certificates, credentials).
   1. Connect via Alternator (DynamoDB-compatible API) with AWS SDK.
   1. Configure driver settings (contact points, local DC, load balancing policy, retry policy).
   1. Understand how scaling and upgrades affect application connectivity (rolling restarts, node replacement).

   **Support Engineer / Incident Responder**
   1. Collect diagnostics using must-gather; understand the output structure.
   1. Assess cluster health: check pod status, ScyllaCluster conditions, nodetool status, Manager task status.
   1. Identify the failure domain: is this a Kubernetes issue, Operator issue, or ScyllaDB issue?
   1. Follow diagnostic flowcharts for common symptoms (pods not starting, nodes joining slowly, performance degradation, TLS errors).
   1. Look up error messages and log patterns in a structured reference.
   1. Check known issues and version-specific workarounds.
   1. Determine whether a workaround exists or escalation is needed; collect context for escalation.

   **Junior Support Engineer**
   1. Map familiar ScyllaDB concepts to Kubernetes equivalents (node → pod, config file → CR spec, nodetool → kubectl exec).
   1. Run copy-paste diagnostic commands to assess cluster state without deep K8s knowledge.
   1. Interpret common Kubernetes failure states (CrashLoopBackOff, Pending, ImagePullBackOff) in the context of ScyllaDB.
   1. Follow a decision tree: "Is the problem with Kubernetes, the Operator, or ScyllaDB?"
   1. Collect must-gather output and attach it to a support ticket with structured context.
   1. Escalate to senior engineers with a clear summary of what was checked and what was found.

   **Contributor / Operator Developer**
   1. Set up local development environment (Go, Docker/Podman, Kind, dependencies).
   1. Build and run the Operator locally; understand the build system and Makefile targets.
   1. Understand the codebase architecture: controllers, webhooks, API types, helpers.
   1. Learn API conventions and how to add or modify CRD fields.
   1. Write and run unit tests, integration tests, and E2E tests locally.
   1. Submit a PR that passes CI; understand what CI checks and how to interpret failures.
   1. Debug a failing controller reconciliation loop using logs and events.

   **Evaluator / Architect**
   1. Understand what the ScyllaDB Operator is and what problems it solves versus manual deployment.
   1. Review the architecture: components, CRDs, reconciliation model, storage integration.
   1. Assess platform compatibility (cloud providers, K8s distributions, ScyllaDB versions).
   1. Evaluate operational capabilities (automated repairs, backups, upgrades, scaling, monitoring).
   1. Understand the multi-datacenter story (single-DC vs. multi-DC, tech preview status, networking requirements).
   1. Review the security model (TLS, authentication, RBAC, network policies).
   1. Assess release cadence, support lifecycle, and upgrade path guarantees.
   1. Make a go/no-go decision with a clear understanding of what is production-ready versus tech preview.

1. Determine prerequisite knowledge required for each documentation section
1. Identify common misconceptions that need explicit clarification

## Content Completeness
1. Document all configuration parameters with defaults, constraints, and examples

    **Open gaps from issue tracker:**
    - Document passing additional ScyllaDB arguments, including recovery scenarios like setting the log level on unhealthy setups (mid-rollout, node down). Cover both `v1` and `v1alpha1` APIs. ([#3199](https://github.com/scylladb/scylla-operator/issues/3199))
    - Document "standard" aggregated conditions (`Available`, `Progressing`, `Degraded`): their semantics, granular subresource conditions, and how they get aggregated. Link from the troubleshooting guide. ([#3022](https://github.com/scylladb/scylla-operator/issues/3022))
    - Document how to set `RLIMIT_NOFILE` (max open files) for ScyllaDB pods via the `fs.nr_open` sysctl. ([#2937](https://github.com/scylladb/scylla-operator/issues/2937))
    - Document how to set up precomputed `io_properties` for ScyllaDB. ([#2205](https://github.com/scylladb/scylla-operator/issues/2205))
    - Describe recommended deployment topology for ScyllaClusters (e.g., at least 3 nodes per DC). ([#1605](https://github.com/scylladb/scylla-operator/issues/1605))
    - Document storage capacity discovery. ([#2350](https://github.com/scylladb/scylla-operator/issues/2350))

1. Provide end-to-end examples for each major workflow

    **Open gaps from issue tracker:**
    - Provide a minimal, functional GKE deployment example for newcomers (dev mode, no tuning). ([#641](https://github.com/scylladb/scylla-operator/issues/641))
    - GKE example should create ScyllaDB pool nodes across multiple zones. ([#1465](https://github.com/scylladb/scylla-operator/issues/1465))
    - Replace outdated AWS `i3` instance type with a modern, supported instance in examples. ([#2994](https://github.com/scylladb/scylla-operator/issues/2994))
    - Fix various inaccuracies on the generic deployment page: wrong container count (2/2 vs 3/3), deprecated `--dry-run` flag, wrong `scripts/` path for cassandra-stress, missing NodePort/port-forwarding guidance. ([#2080](https://github.com/scylladb/scylla-operator/issues/2080))
    - Fix monitoring page issues: inconsistent `$` prefix on commands, missing `-n scylla` namespace, unhelpful placeholder cluster names, no port-forward example for Grafana access. ([#2080](https://github.com/scylladb/scylla-operator/issues/2080))
    - Complete day-2 operations documentation: extending storage, changing log level. ([#2450](https://github.com/scylladb/scylla-operator/issues/2450))
    - Audit and complete all ScyllaDB operations procedures. ([#2426](https://github.com/scylladb/scylla-operator/issues/2426))
    - Node replacement docs should mention the `ignore_dead_nodes_for_replace` option. ([#2184](https://github.com/scylladb/scylla-operator/issues/2184))
    - Include building and custom deployment instructions for developers. ([#3278](https://github.com/scylladb/scylla-operator/issues/3278))

1. Include failure scenarios with symptoms, root causes, and remediation steps

    **Open gaps from issue tracker:**
    - Document coredump configuration on Kubernetes: systemd-coredump setup, storage requirements, and recommendations. ([#2917](https://github.com/scylladb/scylla-operator/issues/2917))
    - Cover must-gather collection for minikube setups. ([#1628](https://github.com/scylladb/scylla-operator/issues/1628))
    - Document must-gather collection with alternative container runtimes (e.g., Podman). ([#2384](https://github.com/scylladb/scylla-operator/issues/2384))

1. Document integration points with external systems

    **Open gaps from issue tracker:**
    - Document ScyllaDB Manager deployment and integration end-to-end. ([#1898](https://github.com/scylladb/scylla-operator/issues/1898))
    - Document configuring workload identities (IAM roles) with ScyllaDB Manager Agent for EKS and GKE object storage access. ([#1697](https://github.com/scylladb/scylla-operator/issues/1697))
    - Remove unmanaged (legacy) monitoring leftovers from docs. ([#2407](https://github.com/scylladb/scylla-operator/issues/2407))

1. Cover upgrade and migration paths between versions

    **Open gaps from issue tracker:**
    - Document GKE upgrade procedures (control plane and node pools). ([#2446](https://github.com/scylladb/scylla-operator/issues/2446))
    - Document properly upgrading Kubernetes nodes on EKS and GKE to always respect PDBs. ([#2381](https://github.com/scylladb/scylla-operator/issues/2381))
    - Add a "Review Release Notes" section to upgrade documentation. ([#2513](https://github.com/scylladb/scylla-operator/issues/2513))

1. Include performance characteristics and capacity planning guidelines

    **Open gaps from issue tracker:**
    - Add a sizing guide for common platforms (instance types, resource requests, storage). ([#2349](https://github.com/scylladb/scylla-operator/issues/2349))
    - Add a dedicated docs section about benchmarking ScyllaDB on Kubernetes. ([#2365](https://github.com/scylladb/scylla-operator/issues/2365))
    - Create a production readiness checklist covering: NodeConfig value, dedicated node pools, performance tuning co-location, coredumps, monitoring, recommended resource requests/limits, CPU pinning, XFS trimming. ([#2916](https://github.com/scylladb/scylla-operator/issues/2916))

1. Document security considerations and hardening procedures
1. Provide disaster recovery and backup procedures

1. Ensure installation documentation is accurate and complete

    **Open gaps from issue tracker:**
    - Installation documentation is outdated: README vs website prerequisites out of sync, Kubernetes version 1.16+ is stale. ([#2820](https://github.com/scylladb/scylla-operator/issues/2820))
    - Local CSI Driver installation step is missing from the Helm installation guide. ([#2567](https://github.com/scylladb/scylla-operator/issues/2567))
    - Unresolved template parameters in installation docs cause dead links. ([#2477](https://github.com/scylladb/scylla-operator/issues/2477))
    - Variable substitution not. Define clear information hierarchy with logical grouping of related topics
1. Create consistent navigation patterns that reduce cognitive load
1. Design clear entry points for different user personas and skill levels
1. Each document should have a well-defined Diataxis category. working in parts of the docs, producing raw template syntax. ([#2626](https://github.com/scylladb/scylla-operator/issues/2626))
    - Unify dependency requirement syntax across all docs pages. ([#3063](https://github.com/scylladb/scylla-operator/issues/3063))

1. Maintain consistent and accurate terminology

    **Open gaps from issue tracker:**
    - Stop referring to "ScyllaDB Open Source" and "ScyllaDB Enterprise" in docs; update to current product naming. ([#3066](https://github.com/scylladb/scylla-operator/issues/3066))

1. Cover Multi-DC documentation

    **Open gaps from issue tracker:**
    - Document Multi-DC installation procedure. ([#2855](https://github.com/scylladb/scylla-operator/issues/2855))
    - Document Multi-DC day-2 operations: node replacement (logging into individual DCs), upgrades. ([#2649](https://github.com/scylladb/scylla-operator/issues/2649))
    - Document and test Multi-DC node replacement procedure. ([#2861](https://github.com/scylladb/scylla-operator/issues/2861))
    - Document and test Multi-DC scaling procedures. ([#2859](https://github.com/scylladb/scylla-operator/issues/2859))
    - Document and test Multi-DC client connections. ([#2862](https://github.com/scylladb/scylla-operator/issues/2862))
    - Document and test Multi-DC Operator upgrade procedure. ([#2857](https://github.com/scylladb/scylla-operator/issues/2857))
    - Document Multi-DC known gaps and limitations. ([#2856](https://github.com/scylladb/scylla-operator/issues/2856))

## Troubleshooting Support
1. Create diagnostic flowcharts for common problem categories
1. Document observable symptoms with corresponding root causes
1. Provide step-by-step debugging procedures
1. Include log message reference with severity levels and meanings
1. Document error codes with explanations and resolution steps
1. Provide health check procedures and validation commands
1. Include common anti-patterns and their consequences

## Accessibility & Usability
1. Ensure all procedures are reproducible by following steps linearly
1. Provide context before diving into technical details
1. Include visual aids where they reduce explanation complexity
1. Offer both quick-start paths and comprehensive references in separate documents
1. Design scannable content with clear headings and bullet points
1. Provide search-optimized content with relevant keywords

## Practical Examples
1. Include minimal working examples for each feature
1. Provide production-ready configurations with security best practices
1. Show before/after states for configuration changes
1. Demonstrate common customization scenarios
1. Include examples of integration with related tools

## Prerequisites & Dependencies
1. List all required software versions and compatibility matrices (existing support releases docs)
1. Document environmental requirements and constraints in guides and how-tos where it's necessary
1. Identify network requirements and firewall rules
1. Document resource requirements (CPU, memory, storage, network)

## Edge Cases & Limitations
1. Document known limitations and workarounds
1. Specify boundary conditions and scale limits
1. Cover platform-specific behaviors and differences
1. Document unsupported configurations explicitly
1. Include compatibility notes for different environments

## Operational Runbook Elements
1. Provide monitoring and alerting guidelines
1. Document routine maintenance procedures
1. Include incident response procedures
1. Provide rollback procedures for all changes
1. Document validation steps after each operation
