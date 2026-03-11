# Documentation Structure Analysis - Diataxis Classification

This document classifies all documentation content in ScyllaDB Operator according to the [Diataxis framework](https://diataxis.fr/), which organizes documentation into four categories:

- **Tutorials**: Learning-oriented, step-by-step guides for beginners
- **How-to guides**: Task-oriented, goal-focused instructions for specific problems
- **Explanation**: Understanding-oriented, theoretical knowledge and context
- **Reference**: Information-oriented, technical descriptions and specifications

---

## 📚 Tutorials

### Quickstarts
**File**: `quickstarts/gke.md`, `quickstarts/eks.md`
- **Content**: End-to-end cluster setup on cloud platforms
  - Creating GKE/EKS clusters with specific configurations
  - Setting up Kubernetes prerequisites
  - Installing xfsprogs
  - Deploying ScyllaDB Operator
  - Creating ScyllaDB clusters
  - Accessing the clusters
  - Cleanup procedures
- **Classification**: Tutorial - provides complete learning path from zero to working cluster

### IPv6 Getting Started
**File**: `management/networking/ipv6/tutorials/ipv6-getting-started.md`
- **Content**: First IPv6-enabled cluster with step-by-step guidance
- **Classification**: Tutorial - learning-oriented IPv6 setup

---

## 🛠️ How-to Guides

### Installation Procedures

#### GitOps Installation
**File**: `installation/gitops.md`
- **Content**: Installing ScyllaDB Operator using kubectl/manifests
  - Installing dependencies (Cert Manager, Prometheus Operator)
  - Deploying ScyllaDB Operator
  - Setting up local storage with NodeConfig
  - Installing Local CSI driver
  - Deploying ScyllaDB Manager
  - Setting up monitoring stack
- **Classification**: How-to - task-oriented installation procedure

#### Helm Installation
**File**: `installation/helm.md`
- **Content**: Installing ScyllaDB stack using Helm charts
  - Adding Helm repository
  - Deploying Cert Manager
  - Customizing and installing ScyllaDB Operator chart
  - Customizing and installing ScyllaDB chart
  - Customizing and installing ScyllaDB Manager chart
  - Monitoring setup with ServiceMonitors
  - Cleanup procedures
- **Classification**: How-to - specific installation method

### Cluster Management

#### Creating ScyllaClusters
**File**: `resources/scyllaclusters/basics.md`
- **Content**: Creating and configuring ScyllaDB clusters
  - Creating ScyllaDB config files
  - Defining ScyllaCluster resources
  - IPv6 and dual-stack configuration
  - Forcing rolling restarts
  - Spreading racks over availability zones
  - Enterprise configuration
- **Classification**: How-to - practical cluster creation tasks

#### Creating ScyllaDBClusters (Multi-DC)
**File**: `resources/scylladbclusters/scylladbclusters.md`
- **Content**: Creating multi-datacenter ScyllaDB clusters
  - Prerequisites setup
  - Creating RemoteKubernetesCluster resources
  - Configuring authentication/authorization
  - Creating ScyllaDBCluster resource
  - Forcing rolling restarts
- **Classification**: How-to - multi-DC deployment procedure

#### Upgrading ScyllaDB Operator
**File**: `management/upgrading/upgrade.md`
- **Content**: Upgrading ScyllaDB Operator installations
  - GitOps upgrade procedure
  - Helm upgrade procedure
  - Version-specific upgrade steps (e.g., 1.17 to 1.18)
- **Classification**: How-to - specific upgrade tasks

#### Upgrading ScyllaDB Clusters
**File**: `management/upgrading/upgrade-scylladb.md`
- **Content**: Upgrading ScyllaDB cluster versions
  - GitOps upgrade for ScyllaCluster and ScyllaDBCluster
  - Helm upgrade for ScyllaCluster
  - Waiting for rollout completion
- **Classification**: How-to - specific upgrade procedure

#### Monitoring Setup
**File**: `management/monitoring/setup.md`
- **Content**: Setting up complete monitoring stack
  - Deploying external Prometheus
  - Creating ServiceAccount, ClusterRole, Service for Prometheus
  - Configuring ScyllaDBMonitoring resource
  - Monitoring multiple ScyllaClusters
  - Configuring Grafana
- **Classification**: How-to - monitoring deployment procedure

#### Exposing Grafana
**File**: `management/monitoring/exposing-grafana.md`
- **Content**: Exposing Grafana using Ingress
  - Installing HAProxy Ingress
  - Creating Ingress resource
  - Verifying connection with credentials
- **Classification**: How-to - specific networking task

#### Configuring Sysctls
**File**: `management/sysctls.md`
- **Content**: Configuring kernel parameters for ScyllaDB nodes
  - Creating NodeConfig with recommended sysctls
  - Applying via GitOps
- **Classification**: How-to - system configuration task

### Node Operations

#### Replacing Nodes
**File**: `resources/scyllaclusters/nodeoperations/replace-node.md`
- **Content**: Replacing dead ScyllaDB nodes
  - Verifying node status with nodetool
  - Identifying service bound to down node
  - Draining node
  - Labeling service to trigger replacement
  - Running repair after replacement
- **Classification**: How-to - operational procedure

#### Volume Expansion
**File**: `resources/scyllaclusters/nodeoperations/volume-expansion.md`
- **Content**: Resizing storage in ScyllaCluster
  - Orphan deleting ScyllaCluster, ScyllaDBDatacenter, StatefulSets
  - Patching PVCs with new size
  - Re-applying ScyllaCluster definition
  - Verifying expansion
- **Classification**: How-to - storage management task

#### Maintenance Mode
**File**: `resources/scyllaclusters/nodeoperations/maintenance-mode.md`
- **Content**: Enabling/disabling maintenance mode
  - Labeling service to enable maintenance
  - Removing label to disable
- **Classification**: How-to - operational procedure

#### Restore from Backup
**File**: `resources/scyllaclusters/nodeoperations/restore.md`
- **Content**: Restoring from ScyllaDB Manager backups
  - Listing available snapshots
  - Restoring schema
  - Handling version-specific requirements
  - Restoring tables
- **Classification**: How-to - backup/restore procedure

### Client Connectivity

#### Using CQL
**File**: `resources/scyllaclusters/clients/cql.md`
- **Content**: Connecting to ScyllaDB with cqlsh
  - Enabling authentication/authorization
  - Using embedded cqlsh (localhost)
  - Setting up remote cqlsh with TLS
  - Generating credentials file
  - Accessing via podman/docker
- **Classification**: How-to - client connection procedure

#### Using Alternator (DynamoDB)
**File**: `resources/scyllaclusters/clients/alternator.md`
- **Content**: Using Alternator DynamoDB-compatible API
  - Enabling Alternator
  - Getting credentials
  - Using AWS CLI with Alternator
- **Classification**: How-to - API usage procedure

#### Discovering Nodes
**File**: `resources/scyllaclusters/clients/discovery.md`
- **Content**: Using ScyllaDB Discovery Endpoint
  - Understanding the discovery service
  - Getting ClusterIP or DNS name
  - Exposing discovery endpoint with LoadBalancer
- **Classification**: How-to - networking/discovery procedure

### IPv6 Networking How-tos

#### Configure Dual-Stack (IPv4-first)
**File**: `management/networking/ipv6/how-to/ipv6-configure.md`
- **Content**: Setting up dual-stack with IPv4 primary
- **Classification**: How-to - specific networking configuration

#### Configure Dual-Stack (IPv6-first)
**File**: `management/networking/ipv6/how-to/ipv6-configure-ipv6-first.md`
- **Content**: Setting up dual-stack with IPv6 primary
- **Classification**: How-to - specific networking configuration

#### Configure IPv6-Only
**File**: `management/networking/ipv6/how-to/ipv6-configure-ipv6-only.md`
- **Content**: Setting up IPv6 single-stack
- **Classification**: How-to - specific networking configuration

#### Migrate to IPv6
**File**: `management/networking/ipv6/how-to/ipv6-migrate.md`
- **Content**: Migrating existing clusters from IPv4 to IPv6
- **Classification**: How-to - migration procedure

#### Troubleshoot IPv6
**File**: `management/networking/ipv6/how-to/ipv6-troubleshoot.md`
- **Content**: Diagnosing and resolving IPv6 networking issues
- **Classification**: How-to - troubleshooting procedure

### Troubleshooting

#### Installation Troubleshooting
**File**: `support/troubleshooting/installation.md`
- **Content**: Debugging installation issues
- **Classification**: How-to - troubleshooting procedure

### Support Operations

#### Must-gather
**File**: `support/must-gather.md`
- **Content**: Collecting diagnostic information
  - Running must-gather with podman/docker
  - Limiting to namespace
  - Collecting all resources
  - Including/excluding specific resources
- **Classification**: How-to - diagnostic data collection procedure

---

## 📖 Explanation

### Architecture

#### Overview
**File**: `architecture/overview.md`
- **Content**: ScyllaDB Operator components and design
  - CustomResourceDefinitions and webhooks
  - Storage provisioner requirements
  - Cluster-scoped and namespaced component diagrams
- **Classification**: Explanation - architectural understanding

#### Tuning
**File**: `architecture/tuning.md`
- **Content**: Performance tuning concepts
  - CPU pinning and static CPU policy
  - Network interrupt handling
  - Perftune script operation
  - Node tuning vs Pod tuning
- **Classification**: Explanation - understanding tuning mechanisms

#### ScyllaDB Manager Integration
**File**: `architecture/manager.md`
- **Content**: ScyllaDB Manager architecture and integration
  - Global manager instance concept
  - Task synchronization (repair/backup)
  - Security considerations (shared namespace)
  - Accessing manager manually
- **Classification**: Explanation - understanding manager integration

#### Storage Overview
**File**: `architecture/storage/overview.md`
- **Content**: Storage provisioner concepts
  - Local vs network storage
  - Local disk setup challenges
  - NodeConfig role
  - Supported provisioners
- **Classification**: Explanation - storage architecture concepts

#### Local CSI Driver
**File**: `architecture/storage/local-csi-driver.md`
- **Content**: Local CSI Driver explanation
  - Container Storage Interface implementation
  - Dynamic provisioning on local disks
  - Directory-based volumes with xfs prjquota
- **Classification**: Explanation - CSI driver concepts

### Networking

#### Exposing ScyllaDB Clusters
**File**: `resources/common/exposing.md`
- **Content**: Network exposure concepts and options
  - exposeOptions field explanation
  - Node service types (Headless, ClusterIP, LoadBalancer)
  - Broadcast options for clients and nodes
  - Platform-specific annotations
- **Classification**: Explanation - networking architecture and concepts

#### IPv6 Networking Concepts
**File**: `management/networking/ipv6/concepts/ipv6-networking.md`
- **Content**: Deep dive into IPv6 support
  - Operator orchestration of IPv6
  - IP family selection behavior
  - Why first IP family matters
  - Dual-stack architecture explained
  - Client connectivity patterns
  - DNS configuration importance
- **Classification**: Explanation - understanding IPv6 implementation

### Management Features

#### Automatic Data Cleanup
**File**: `management/data-cleanup.md`
- **Content**: Automatic cleanup feature explanation
  - Why cleanup is needed (stale data, data resurrection)
  - Cleanup triggering mechanism (token ring changes)
  - How to inspect cleanup jobs
  - Known limitations
- **Classification**: Explanation - understanding automatic operations

#### Bootstrap Synchronisation
**File**: `management/bootstrap-sync.md`
- **Content**: Automated bootstrap synchronisation
  - Why synchronisation is necessary (ScyllaDB requirement)
  - How the barrier mechanism works
  - ScyllaDBDatacenterNodeStatuses CRD role
  - Enabling and overriding behavior
- **Classification**: Explanation - understanding bootstrap safety

#### Monitoring Overview
**File**: `management/monitoring/overview.md`
- **Content**: Monitoring architecture explanation
  - ScyllaDBMonitoring components (Prometheus, Grafana)
  - Managed vs External modes for Prometheus
  - ServiceMonitor and PrometheusRule resources
  - Prometheus Operator dependency
- **Classification**: Explanation - monitoring architecture concepts

---

## 📋 Reference

### API Documentation

#### Generated API Reference
**Files**: `reference/api/index.rst`, `reference/api/groups/scylla.scylladb.com/*.rst`
- **Content**: Complete API specifications for all CRDs
  - ScyllaCluster
  - ScyllaDBCluster
  - ScyllaDBMonitoring
  - ScyllaDBDatacenter
  - ScyllaDBManagerTask
  - NodeConfig
  - ScyllaOperatorConfig
  - RemoteKubernetesCluster
  - RemoteOwner
- **Classification**: Reference - API specifications

#### Feature Gates
**File**: `reference/feature-gates.md`
- **Content**: Feature gate reference
  - Available feature gates table
  - AutomaticTLSCertificates description
  - BootstrapSynchronisation description
  - How to configure feature gates (GitOps and Helm)
- **Classification**: Reference - configuration options

### Resources Reference

#### NodeConfigs
**File**: `resources/nodeconfigs.md`
- **Content**: NodeConfig API object reference
  - Disk setup capabilities (RAID, filesystem, mounts)
  - Performance tuning
  - Status conditions
- **Classification**: Reference - resource description

#### ScyllaOperatorConfigs
**File**: `resources/scyllaoperatorconfigs.md`
- **Content**: ScyllaOperatorConfig reference
  - Global configuration structure
  - Singleton resource (cluster name)
  - Status fields (auxiliary images)
  - ScyllaDB Enterprise tuning configuration
- **Classification**: Reference - resource description

#### RemoteKubernetesClusters
**File**: `resources/remotekubernetesclusters.md`
- **Content**: RemoteKubernetesCluster reference
  - Resource purpose for multi-DC
  - Authorization methods (Secret-based)
  - Status conditions
- **Classification**: Reference - resource description

#### IPv6 Configuration Reference
**File**: `management/networking/ipv6/reference/ipv6-configuration.md`
- **Content**: Complete IPv6 API reference
  - Configuration fields specification
  - Production readiness status
- **Classification**: Reference - IPv6 configuration options

### Installation Reference

#### Overview
**File**: `installation/overview.md`
- **Content**: Installation architecture and modes
  - ScyllaDB Operator components overview
  - Installation modes (GitOps, Helm)
  - Component diagrams
  - Upgrade notes
- **Classification**: Reference - installation overview and options

#### Kubernetes Prerequisites
**File**: `installation/kubernetes-prerequisites.md`
- **Content**: Requirements specification
  - Kubelet static CPU policy configuration
  - Node labels
  - Package requirements (xfsprogs)
  - Platform-specific configurations
- **Classification**: Reference - system requirements

### Support Reference

#### Releases
**File**: `support/releases.md`
- **Content**: Release information
  - Release schedule
  - Supported releases table with dates
  - Backport policy
  - CI/CD and automated promotions
  - Support matrix (Kubernetes, OpenShift, ScyllaDB versions)
  - Supported architectures and environments
- **Classification**: Reference - version compatibility and support

#### Known Issues
**File**: `support/known-issues.md`
- **Content**: Documented issues and workarounds
  - ScyllaDB Manager on Minikube
  - TRUNCATE queries on Minikube (hairpinning)
- **Classification**: Reference - known limitations

### Operations Reference

#### ScyllaCluster Node Operations Index
**File**: `resources/scyllaclusters/nodeoperations/index.rst`
- **Content**: Table of contents for node operations
  - Upgrade
  - Replace node
  - Automatic cleanup
  - Maintenance mode
  - Restore
  - Volume expansion
- **Classification**: Reference - operations catalog

#### Automatic Cleanup (inspecting)
**File**: `resources/scyllaclusters/nodeoperations/automatic-cleanup.md`
- **Content**: Inspecting cleanup jobs section
  - JobControllerProgressing condition
  - Checking job status with kubectl
  - Event inspection
- **Classification**: Reference - status inspection (partial reference content)

### Multi-DC Reference

#### Multi-DC Examples (GKE)
**File**: `resources/common/multidc/gke.md`
- **Content**: Platform-specific multi-DC setup
- **Classification**: Reference - platform configuration examples

#### Multi-DC Examples (EKS)
**File**: `resources/common/multidc/eks.md`
- **Content**: Platform-specific multi-DC setup
- **Classification**: Reference - platform configuration examples

---

## 🏠 Navigation & Overview Pages

### Main Index
**File**: `index.md`
- **Content**: Documentation landing page
  - Links to major sections
  - ScyllaDB Operator introduction
  - Topic boxes for navigation
- **Classification**: Navigation

### Section Indexes
**Files**: Multiple `index.md` and `index.rst` files
- `installation/index.md`
- `management/index.md`
- `management/monitoring/index.md`
- `management/upgrading/index.md`
- `architecture/index.md`
- `architecture/storage/index.md`
- `resources/index.md`
- `resources/scyllaclusters/index.md`
- `resources/scyllaclusters/clients/index.md`
- `resources/scylladbclusters/index.md`
- `quickstarts/index.md`
- `support/index.md`
- `support/troubleshooting/index.rst`
- `reference/index.md`
- `management/networking/index.md`
- `management/networking/ipv6/index.md`
- **Content**: Section overviews and navigation
- **Classification**: Navigation

### Overview Pages

#### Resources Overview
**File**: `resources/overview.md`
- **Content**: Overview of resource types
  - Namespaced vs cluster-scoped resources
  - Resource discovery commands
  - Topic boxes linking to resource types
- **Classification**: Navigation with explanation elements

#### Support Overview
**File**: `support/overview.md`
- **Content**: Support information
  - Paid support availability
  - Links to troubleshooting
  - Must-gather tool reference
- **Classification**: Navigation

---

## 📝 Summary Statistics

### Content Distribution by Type

| Type | Count | Percentage |
|------|-------|------------|
| **How-to Guides** | 30+ | ~50% |
| **Explanation** | 15+ | ~25% |
| **Reference** | 15+ | ~25% |
| **Tutorials** | 3 | ~5% |
| **Navigation** | 15+ | (Supporting) |

### Key Observations

1. **Strong how-to focus**: Documentation emphasizes practical, task-oriented guides, which is appropriate for an operator installation and management tool.

2. **Good architectural explanation**: Solid coverage of architecture, tuning, storage, and networking concepts helps users understand the system.

3. **Comprehensive reference**: Complete API documentation, feature gates, and support matrices provide necessary technical details.

4. **Limited tutorials**: Only quickstarts and IPv6 getting started qualify as true tutorials. Most "getting started" content is actually how-to guides assuming some prerequisite knowledge.

5. **Well-organized navigation**: Clear section structure with overview pages helps users find relevant content.

### Recommendations for Improvement

1. **Add more tutorials**: Create learning-oriented paths for:
   - First-time ScyllaDB Operator users
   - Basic cluster operations walkthrough
   - Development environment setup
   - Multi-datacenter deployment from scratch

2. **Expand explanation content**:
   - Disaster recovery concepts
   - Backup/restore architecture
   - Security model and best practices
   - Capacity planning guidance

3. **Enhance reference content**:
   - Command reference (kubectl commands)
   - Configuration examples catalog
   - Troubleshooting decision trees
   - Performance tuning parameter reference

4. **Create concept-to-practice bridges**: Link explanation content more explicitly to corresponding how-to guides and vice versa.
