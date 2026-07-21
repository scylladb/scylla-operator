# Reference deployment: OpenShift

This guide walks you through deploying ScyllaDB on Red Hat OpenShift.

## Prerequisites

- An OpenShift Container Platform cluster (see the supported version range in [Releases](../../reference/releases.md)).
  If you do not have one yet, follow [Set up an OpenShift cluster for ScyllaDB](../../install-operator/provision-infrastructure/set-up-openshift-cluster.md).
- Dedicated nodes labeled and tainted per [Set up dedicated node pools](../before-you-deploy/set-up-dedicated-node-pools.md).
- ScyllaDB Operator installed.
  Follow [Install on OpenShift](../../install-operator/install-on-openshift.md) if you have not done so yet.
- [`kubectl`](https://kubernetes.io/docs/tasks/tools/#kubectl) or OpenShift CLI ([`oc`](https://docs.redhat.com/en/documentation/openshift_container_platform/latest/html/cli_tools/openshift-cli-oc)) installed.

## Set up nodes

Before deploying a `ScyllaCluster`, the dedicated nodes must be prepared with local disk setup and kernel tuning via `NodeConfig`, and the Local CSI Driver must be installed to provision `PersistentVolumes` from local storage.
Follow [Configure nodes](../before-you-deploy/configure-nodes.md), selecting the **OpenShift** tab where applicable, then return here to deploy the cluster.

## Deploy a ScyllaDB cluster

Follow the [Deploy your first cluster](../deploy-your-first-cluster.md) guide to create a ScyllaDB cluster. The quick path creates a minimal development cluster; the production section covers multi-rack, multi-zone deployments with properly sized resources.

:::{note}
On OpenShift, you can use the built-in User Workload Monitoring with Prometheus instead of deploying a standalone Prometheus instance. See [Set up ScyllaDB Monitoring on OpenShift](../set-up-monitoring/external-prometheus-on-openshift.md) for OpenShift-specific instructions.
:::

## Next steps

- Review the [production checklist](../production-checklist.md) before going to production.
- [Set up monitoring](../set-up-monitoring/external-prometheus-on-openshift.md) using OpenShift User Workload Monitoring.
- Learn how to [connect your application](../../connect-your-app/index.md) to ScyllaDB.
