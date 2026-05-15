# Set up an OpenShift cluster for ScyllaDB

This guide describes the infrastructure requirements for running ScyllaDB on Red Hat OpenShift.

## Cluster requirements

You need an OpenShift Container Platform cluster in a [supported version](../../reference/releases.md) with:

- **A dedicated machine pool for ScyllaDB** with local NVMe storage and sufficient CPU and memory.
  See [Set up dedicated node pools](../../deploy-scylladb/before-you-deploy/set-up-dedicated-node-pools.md) for labeling and taint requirements.
- **Sufficient CPU and memory** — see the [ScyllaDB system requirements](https://docs.scylladb.com/manual/stable/getting-started/system-requirements.html) for minimum and recommended specifications.
  Plan for at least 2 CPUs reserved for the OS, kubelet, and DaemonSets.

:::{tip}
[Red Hat OpenShift Service on AWS (ROSA)](https://aws.amazon.com/rosa/) provides a managed OpenShift experience with access to NVMe-backed instance types suitable for ScyllaDB.
:::

## Next steps

- [Install ScyllaDB Operator on OpenShift](../install-on-openshift.md) — install the operator via OLM.
