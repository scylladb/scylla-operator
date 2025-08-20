# Kubernetes prerequisites

Because {{productName}} aims to leverage the best performance available, there are a few extra steps that need to be configured on your Kubernetes cluster.

## Kubelet

### Static CPU policy

By default, *kubelet* uses the CFS quota to enforce pod CPU limits.
When the Kubernetes node runs a lot of CPU-bound Pods, the processes can move over different CPU cores, depending on whether the Pod is throttled and which CPU cores are available.
However, kubelet may be configured to assign CPUs exclusively by setting the CPU manager policy to static.

To get the best performance and latency ScyllaDB Pods should run under [static CPU policy](https://kubernetes.io/docs/tasks/administer-cluster/cpu-management-policies/#configuration) to pin cores.

:::{note}
Configuring kubelet options is provider specific.
We provide a few examples for the major ones in this section, otherwise please consult the documentation for your Kubernetes platform.
:::

:::::{tabs}

::::{group-tab} GKE

GKE allows you to set static CPU policy using a [node system configuration](https://cloud.google.com/kubernetes-engine/docs/how-to/node-system-config):
:::{code} yaml
:number-lines:
kubeletConfig:
  cpuManagerPolicy: static
:::

::::

::::{group-tab} EKS
`eksctl` allows you to set static CPU policy for each node pool like:

:::{code} yaml
:number-lines:
apiVersion: eksctl.io/v1alpha5
kind: ClusterConfig
# ...
nodeGroups:
- name: scylla-pool
  kubeletExtraConfig:
    cpuManagerPolicy: static
:::

::::

:::::

## Nodes

### Labels

For the purposes of the installation guides, we assume that the nodes meant to run ScyllaDB (ScyllaClusters) have label `scylla.scylladb.com/node-type=scylla`.

### Packages

#### xfsprogs

{{productName}}'s [NodeConfig](../resources/nodeconfigs.md) controller requires `xfsprogs` to be installed on the Kubernetes
nodes to format the local disks with XFS file system.

This package may be not installed by default on some Kubernetes platforms, like GKE starting with version `1.32.1-gke.1002000`.

:::::{tabs}

::::{group-tab} GKE

You can install it on your cluster's nodes using a DaemonSet created with the following command:
:::{code-block} shell
:substitutions:
kubectl apply -f https://raw.githubusercontent.com/{{repository}}/{{revision}}/examples/gke/install-xfsprogs.daemonset.yaml
:::

::::

:::::
