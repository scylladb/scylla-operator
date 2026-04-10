# Reference deployment: OpenShift

This guide walks you through deploying ScyllaDB on Red Hat OpenShift. It combines the OpenShift-specific Operator installation with the standard ScyllaDB cluster deployment.

If you already have ScyllaDB Operator installed, skip to [step 2](#step-2-deploy-a-scylladb-cluster).

## Prerequisites

- An OpenShift Container Platform cluster (see the supported version range in [Releases](../reference/releases.md))
- An account with `cluster-admin` permissions
- `kubectl` or OpenShift CLI (`oc`) installed

## Step 1: Install ScyllaDB Operator

Follow the complete [Install on OpenShift](../install-operator/install-on-openshift.md) guide to install ScyllaDB Operator and all its dependencies via the Operator Lifecycle Manager (OLM). This covers:

- Installing ScyllaDB Operator from the OpenShift software catalog
- Setting up NodeConfig for local storage and performance tuning
- Installing the Local CSI Driver
- Installing ScyllaDB Manager

Once you have completed all steps in the installation guide and verified that the Operator, NodeConfig, Local CSI Driver, and ScyllaDB Manager are healthy, continue to step 2.

## Step 2: Deploy a ScyllaDB cluster

Follow the [Deploy your first cluster](deploy-your-first-cluster.md) guide to create a ScyllaDB cluster. The quick path creates a minimal development cluster; the production section covers multi-rack, multi-zone deployments with properly sized resources.

:::{note}
On OpenShift, you can use the built-in User Workload Monitoring with Prometheus instead of deploying a standalone Prometheus instance. See [Set up monitoring](set-up-monitoring.md) for OpenShift-specific instructions.
:::

## Next steps

- Review the [production checklist](production-checklist.md) before going to production.
- [Set up monitoring](set-up-monitoring.md) using OpenShift User Workload Monitoring.
- Learn how to [connect your application](../connect-your-app/index.md) to ScyllaDB.
