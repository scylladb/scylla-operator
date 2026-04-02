# Collect core dumps

This guide explains how to configure core dump collection on Kubernetes nodes running ScyllaDB Operator-managed ScyllaDB clusters and how to retrieve the resulting dump files.

## Background

Core dump handling is controlled by `kernel.core_pattern` (see Linux [man page](https://man7.org/linux/man-pages/man5/core.5.html)). In Kubernetes, writing dumps to an absolute path inside the container means they are lost on pod restart.
We recommend piping dumps through the `systemd-coredump` tool, which stores them on the host filesystem independently of pod lifetime.

## Platform requirements

Collecting a core dump requires the following prerequisites on the Kubernetes worker node where the process expected to crash is scheduled. To make things simpler, it can be done on all worker nodes.
This must be completed **before the anticipated crash**.

1. **`systemd-coredump` installed** - the helper binary that receives the core image from the kernel and writes it to disk.

2. **`/etc/systemd/coredump.conf` configured** - controls storage location (`Storage=external`), compression (`Compress=yes`), and disk space limits (`MaxUse`, `KeepFree`, `ProcessSizeMax`, `ExternalSizeMax`). 

3. **`kernel.core_pattern` set** to pipe crashes through `systemd-coredump`.

4. **`systemd-coredump.socket` active**. 

A ready-to-use setup for GKE is provided below. On other platforms, apply these four steps using the OS package manager and systemd tooling available on the node.

## Setting up core dump collection on GKE

GKE Ubuntu nodes do not ship `systemd-coredump` by default. The two manifests below handle all four setup steps via a single container on each ScyllaDB node. The container performs the setup once at startup and then sleeps, keeping the pod alive so that the DaemonSet re-applies the settings whenever the pod is evicted or rescheduled.

### 1. Create the ConfigMap

:::{literalinclude} ../../../examples/gke/coredumps/coredump-conf.configmap.yaml
:language: yaml
:::

Download the manifest and edit `MaxUse` and `KeepFree` to match your environment before applying - see [Storage considerations](#storage-considerations).

:::{code-block} bash
:substitutions:
curl -fLO https://raw.githubusercontent.com/{{repository}}/{{revision}}/examples/gke/coredumps/coredump-conf.configmap.yaml
:::

:::{code-block} bash
vi coredump-conf.configmap.yaml
:::

:::{code-block} bash
kubectl apply --server-side -f=coredump-conf.configmap.yaml
:::

### 2. Deploy the setup DaemonSet

:::{literalinclude} ../../../examples/gke/coredumps/setup-systemd-coredump.daemonset.yaml
:language: yaml
:::

:::{code-block} bash
:substitutions:
kubectl apply --server-side -f=https://raw.githubusercontent.com/{{repository}}/{{revision}}/examples/gke/coredumps/setup-systemd-coredump.daemonset.yaml
:::

Wait for the DaemonSet to roll out on all ScyllaDB nodes:

:::{code-block} bash
kubectl -n scylla-operator rollout status daemonset/scylladb-coredump-setup
:::

### 3. Verify the configuration

After the DaemonSet rolls out, confirm `kernel.core_pattern` is correctly set on each node. List the dedicated ScyllaDB nodes:

:::{code-block} bash
kubectl get nodes -l scylla.scylladb.com/node-type=scylla -o name
:::

Run the following command for each node, replacing `<node-name>` with the actual name:

:::{code-block} bash
kubectl debug node/<node-name> -it --profile=sysadmin --image=docker.io/library/ubuntu:24.04 -- \
  nsenter --mount=/proc/1/ns/mnt -- sysctl -n kernel.core_pattern
:::

Expected output:

:::{code-block} console
|/usr/lib/systemd/systemd-coredump %P %u %g %s %t 9223372036854775808 %h %d
:::

Also verify that `systemd-coredump.socket` is active:

:::{code-block} bash
kubectl debug node/<node-name> -it --profile=sysadmin --image=docker.io/library/ubuntu:24.04 -- \
  nsenter --mount=/proc/1/ns/mnt -- systemctl is-active systemd-coredump.socket
:::

The output must be `active`.

## Verifying that core dump collection works end to end

The steps below trigger a test crash of a running ScyllaDB process and confirm the dump was captured by `systemd-coredump`.

:::{warning}
This procedure intentionally crashes a ScyllaDB node. Only run it when the cluster can tolerate losing one member temporarily.
:::

### 1. Find the pod and its node

:::{code-block} bash
NAMESPACE=<namespace>
kubectl get pods -n "${NAMESPACE}" -l scylla-operator.scylladb.com/pod-type=scylladb-node -o wide
:::

Store the pod name and the node it is scheduled on:

:::{code-block} bash
POD_NAME=<pod-name>
NODE_NAME=<node-name>
:::

### 2. Trigger the crash

Inside a pod managed by ScyllaDB Operator, the sidecar is PID 1 and the `scylla` binary runs as a child process. Send a `SIGABRT` signal to the `scylla` process to trigger a crash and core dump:

:::{code-block} bash
kubectl exec -n "${NAMESPACE}" "${POD_NAME}" -c scylla -- sh -c 'kill -ABRT $(pgrep -x scylla)'
:::

ScyllaDB logs a backtrace and terminates. The pod stays running because the Scylla Operator sidecar (PID 1) is unaffected; the Operator will restart the ScyllaDB process automatically. The dump is written to the node's host filesystem before the process exits.

### 3. Confirm the dump was captured

Confirm the dump was captured using coredumpctl list - see [Retrieving core dumps from nodes](#retrieving-core-dumps-from-nodes) for details.

## Retrieving core dumps from nodes

Core dumps are stored at `/var/lib/systemd/coredump/` on the host.

### 1. List available dumps

:::{code-block} bash
kubectl debug "node/${NODE_NAME}" -it --profile=sysadmin --image=docker.io/library/ubuntu:24.04 -- \
  nsenter --mount=/proc/1/ns/mnt -- coredumpctl list
:::

Store the PID of the desired dump from the output:

:::{code-block} bash
DUMP_PID=<pid>
:::

### 2. Export a specific dump

Start a debug pod on the node so that we can use `kubectl exec` to retrieve the dump file:

:::{code-block} bash
kubectl debug "node/${NODE_NAME}" --profile=sysadmin --image=docker.io/library/ubuntu:24.04 -- sleep 3600
:::

Store the debug pod name:

:::{code-block} bash
DEBUG_POD_NAME=<debug-pod-name>
:::

Pull the dump file from the node to your local machine (it can be very large, so this may take some time):

:::{code-block} bash
kubectl exec "${DEBUG_POD_NAME}" -- \
    nsenter --mount=/proc/1/ns/mnt -- coredumpctl dump "${DUMP_PID}" \
    > scylla.core
:::

You can verify the dump with `file scylla.core` - it should show ELF 64-bit LSB core file.

## Storage considerations

Take into account that ScyllaDB core dumps can be very large. You will need spare disk space larger than that of ScyllaDB's RAM. Core dump storage is controlled by the `[Coredump]` section of `/etc/systemd/coredump.conf`.

:::{note}
`systemd-coredump` will automatically delete the oldest dump files when the `MaxUse` or `KeepFree` thresholds are exceeded, so some dumps may be lost if a node generates many crashes in a short period of time and the disk is nearly full.
:::

To avoid losing dumps due to insufficient disk space, consider the following:

- **Attach a dedicated disk** to each ScyllaDB node at `/var/lib/systemd/coredump/` so core dumps do not compete with the OS for disk space.
- **Offload dumps to object storage** - the [IBM core-dump-handler](https://github.com/IBM/core-dump-handler) project provides a Helm chart that installs a similar `kernel.core_pattern` pipe handler and automatically uploads dumps to an S3-compatible bucket. This is a good option if you need centralized, long-term dump storage.
