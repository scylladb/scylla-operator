# Upgrading ScyllaDB clusters

Upgrading your ScyllaDB cluster to a newer version is automated by ScyllaDB Operator and performed using a rolling update 
strategy to maintain availability. It is as simple as updating the ScyllaDB image reference in your ScyllaDB cluster specification.

:::{warning}
While the cluster remains operational throughout the process, applications requiring strict consistency levels (such as `QUORUM`) 
may experience transient unavailability. This can occur if the cluster topology view has not yet fully converged across all 
nodes before the next node is restarted. 

We recommend scheduling upgrades during periods of low application traffic to minimize potential disruptions.

Issue tracking fix for this behavior: [scylla-operator #1077](https://github.com/scylladb/scylla-operator/issues/1077).
:::

:::{warning}
ScyllaDB version upgrades must be performed consecutively, meaning **you must not skip any major or minor version on the upgrade path**.
Before upgrading to the next version, ensure the entire ScyllaDB cluster has been successfully upgraded.
For details, refer to the [Upgrade procedure in ScyllaDB's documentation](https://enterprise.docs.scylladb.com/stable/upgrade/index.html#upgrade-upgrade-procedures).
:::

:::{caution}
Before upgrading ScyllaDB, ensure the target version is supported by the version of ScyllaDB Operator you are using.
Refer to the [support matrix](./../../support/releases.md#support-matrix) for information on version compatibility.
:::

## Upgrade via GitOps (kubectl)

To upgrade your ScyllaDB cluster using GitOps (kubectl), adjust the ScyllaDB image tag/reference to the target one in your ScyllaDB cluster specification and re-apply the manifest.

:::::{tabs}
::::{group-tab} ScyllaCluster
:::{code-block} yaml
:substitutions:

apiVersion: scylla.scylladb.com/v1
kind: ScyllaCluster
metadata:
  name: scylladb
spec:
  version: {{scyllaDBImageTag}} # Specify the target ScyllaDB image tag.
  # ...
:::

After reapplying the manifest, wait for your ScyllaCluster to roll out.
:::{include} ./../../.internal/wait-for-status-conditions.scyllacluster.code-block.md
:::

::::
::::{group-tab} ScyllaDBCluster
:::{code-block} yaml
:substitutions:

apiVersion: scylla.scylladb.com/v1alpha1
kind: ScyllaDBCluster
metadata:
  name: dev-cluster
spec:
  scyllaDB:
    image: {{imageRepository}}:{{scyllaDBImageTag}} # Specify the target ScyllaDB image reference.
  # ...
:::

After reapplying the manifest, wait for your ScyllaDBCluster to roll out.
:::{include} ./../../.internal/wait-for-status-conditions.scylladbcluster.code-block.md
:::

::::
:::::

:::{include} ./../../.internal/wait-for-all-nodes-un.md
:::

## Upgrade via Helm

:::{important}
ScyllaDB Operator does not yet support Helm installation path for managed multi-datacenter ScyllaDB clusters.
:::

To upgrade your ScyllaDB cluster using Helm, upgrade your Helm release with the target ScyllaDB image tag/reference.

:::::{tabs}
::::{group-tab} ScyllaCluster
:::{code-block} shell
:substitutions:

helm upgrade scylla scylla/scylla --reuse-values --set=scyllaImage.tag={{scyllaDBImageTag}}
:::

After upgrading the release, wait for your ScyllaCluster to roll out.
:::{include} ./../../.internal/wait-for-status-conditions.scyllacluster.code-block.md
:::

::::
::::

:::{include} ./../../.internal/wait-for-all-nodes-un.md
:::
