# Set up an OpenShift cluster for ScyllaDB

This guide describes the infrastructure requirements for running ScyllaDB on Red Hat OpenShift.

## Cluster requirements

You need an OpenShift Container Platform cluster in a [supported version](https://operator.docs.scylladb.com/stable/reference/releases.md) with:

- **A dedicated machine pool for ScyllaDB** with local NVMe storage and sufficient CPU and memory.
  See [Set up dedicated node pools](https://operator.docs.scylladb.com/stable/deploy-scylladb/before-you-deploy/set-up-dedicated-node-pools.md) for labeling and taint requirements.
- **Sufficient CPU and memory** — see the [ScyllaDB system requirements](https://docs.scylladb.com/manual/stable/getting-started/system-requirements.html) for minimum and recommended specifications.
  Plan for at least 2 CPUs reserved for the OS, kubelet, and DaemonSets.

## Next steps

- [Install ScyllaDB Operator on OpenShift](https://operator.docs.scylladb.com/stable/install-operator/install-on-openshift.md) — install the operator via OLM.
