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

## Set up core dump collection on GKE

GKE Ubuntu nodes do not ship `systemd-coredump` by default. The two manifests below handle all four setup steps via a single container on each ScyllaDB node. The container performs the setup once at startup and then sleeps, keeping the pod alive so that the DaemonSet re-applies the settings whenever the pod is evicted or rescheduled.

### 1. Create the ConfigMap

```yaml
# Recommended systemd-coredump configuration for nodes running ScyllaDB.
#
# Apply this ConfigMap alongside the setup-systemd-coredump DaemonSet so that
# the setup container writes these settings to /etc/systemd/coredump.conf on
# each node before activating kernel.core_pattern.
#
# Key tuning choices (adjust to your environment, refer to `man 5 coredump.conf`):
#   Storage=external  - write core dump files to /var/lib/systemd/coredump/
#                      (as opposed to the systemd journal or tmpfs).
#   Compress=yes      - compress dumps with zstd (saves significant disk space).
#   ProcessSizeMax=0 - do not truncate core dumps; ScyllaDB needs full cores.
#   ExternalSizeMax=0
#   MaxUse=20G        - cap the total space used for stored dumps.
#   KeepFree=10G      - always leave at least this much free on the target filesystem.
#
# IMPORTANT: ScyllaDB processes may allocate hundreds of gigabytes of memory.
# Even compressed core dumps can be very large. Adjust MaxUse and KeepFree to
# match the size of your host boot disk (or a dedicated volume if you mount one
# at /var/lib/systemd/coredump/).
apiVersion: v1
kind: ConfigMap
metadata:
  name: scylladb-coredump-conf
  namespace: scylla-operator
  labels:
    app.kubernetes.io/name: scylladb-coredump-setup
data:
  coredump.conf: |
    [Coredump]
    # Store core dumps as files on the host filesystem.
    Storage=external

    # Compress stored core dumps using zstd.
    Compress=yes

    # Do not truncate core dumps. ScyllaDB requires full core images for analysis.
    ProcessSizeMax=0
    ExternalSizeMax=0  
  
    # Maximum total disk space to use for all stored core dumps.
    # Increase if your nodes have larger disks or if you expect many simultaneous crashes.
    MaxUse=20G

    # Always keep at least this much disk space free on the target filesystem.
    KeepFree=10G
```

Download the manifest and edit `MaxUse` and `KeepFree` to match your environment before applying - see [Storage considerations]().

```bash
curl -fLO https://raw.githubusercontent.com/scylladb/scylla-operator/v1.22/examples/gke/coredumps/coredump-conf.configmap.yaml
```

```bash
vi coredump-conf.configmap.yaml
```

```bash
kubectl apply --server-side -f=coredump-conf.configmap.yaml
```

### 2. Deploy the setup DaemonSet

```yaml
# This DaemonSet installs and configures systemd-coredump on GKE nodes running
# ScyllaDB so that core dumps are captured and stored on the host filesystem at
# /var/lib/systemd/coredump/.
#
# GKE nodes use Ubuntu with apt-get as the package manager.
# systemd-coredump is not installed by default on GKE nodes; a single
# long-running container installs it and then performs the following setup steps:
#   1. Install the systemd-coredump package via apt-get.
#   2. Apply the recommended /etc/systemd/coredump.conf configuration.
#   3. Set kernel.core_pattern to pipe core dumps through systemd-coredump.
#   4. Start systemd-coredump.socket so the helper can connect to it when a
#      crash occurs.
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: scylladb-coredump-setup
  namespace: scylla-operator
  labels:
    app.kubernetes.io/name: scylladb-coredump-setup
spec:
  selector:
    matchLabels:
      app.kubernetes.io/name: scylladb-coredump-setup
  template:
    metadata:
      labels:
        app.kubernetes.io/name: scylladb-coredump-setup
    spec:
      # Target only the nodes that run ScyllaDB workloads.
      nodeSelector:
        scylla.scylladb.com/node-type: scylla
      tolerations:
      - key: scylla-operator.scylladb.com/dedicated
        operator: Equal
        value: scyllaclusters
        effect: NoSchedule
      # hostPID is required so that "nsenter -t 1" targets the host's systemd
      # (PID 1) rather than the container's init process. This is needed for
      # systemctl to communicate with the host's D-Bus and start
      # systemd-coredump.socket.
      hostPID: true
      containers:
      - name: setup-coredump
        image: docker.io/library/ubuntu:24.04
        imagePullPolicy: IfNotPresent
        securityContext:
          privileged: true
        readinessProbe:
          exec:
            command:
            - cat
            - /tmp/setup-complete
          initialDelaySeconds: 5
          periodSeconds: 5
        resources:
          requests:
            cpu: 1m
            memory: 32Mi
          limits:
            cpu: 100m
            memory: 128Mi
        command:
        - /bin/bash
        - -euEo
        - pipefail
        - -O
        - inherit_errexit
        - -c
        - |
          # Run a command inside the host's mount + UTS namespaces using nsenter
          # so that package managers and sysctl operate on the real host.
          host_exec() {
            nsenter --mount=/host/proc/1/ns/mnt --uts=/host/proc/1/ns/uts -- "$@"
          }

          echo "Installing systemd-coredump via apt-get..."
          host_exec apt-get update -y -qq
          host_exec apt-get install -y -qq systemd-coredump

          # Apply the coredump configuration from the mounted ConfigMap.
          if [ -f /config/coredump.conf ]; then
            echo "Applying custom /etc/systemd/coredump.conf..."
            cp /config/coredump.conf /host/etc/systemd/coredump.conf
            nsenter -t 1 --mount --uts --ipc --net -- systemctl daemon-reload || true
          fi

          # Retrieve the path to the systemd-coredump helper binary.
          # On GKE Ubuntu nodes this is /usr/lib/systemd/systemd-coredump.
          SYSTEMD_COREDUMP_BIN="$(host_exec sh -c 'command -v systemd-coredump 2>/dev/null || echo /usr/lib/systemd/systemd-coredump')"

          echo "Setting kernel.core_pattern to pipe through ${SYSTEMD_COREDUMP_BIN}..."
          # The format string passes 8 positional arguments to the helper
          # (systemd-coredump >= 252 requires exactly 8):
          #   %P                   PID of the crashing process (initial PID namespace)
          #   %u                   UID of the crashing process
          #   %g                   GID of the crashing process
          #   %s                   Signal number
          #   %t                   Unix timestamp of the crash
          #   9223372036854775808  A large hardcoded value passed in place of %c (the core file size rlimit) to prevent truncation
          #   %h                   Hostname
          #   %d                   Directory fd - lets systemd-coredump read /proc/<PID> metadata after the process exits
          host_exec sysctl -w "kernel.core_pattern=|${SYSTEMD_COREDUMP_BIN} %P %u %g %s %t 9223372036854775808 %h %d"

          echo "kernel.core_pattern is now:"
          host_exec sysctl -n kernel.core_pattern

          # Start systemd-coredump.socket on the host so the helper can hand off
          # the core image for processing. Without an active socket the helper
          # exits silently and no core file is written.
          # nsenter -t 1 with mount+UTS+IPC+net namespaces makes the host binaries
          # and the D-Bus socket visible to systemctl.
          echo "Starting systemd-coredump.socket on the host..."
          nsenter -t 1 --mount --uts --ipc --net -- systemctl start systemd-coredump.socket
          echo "systemd-coredump.socket is now active."

          # Keep the pod running so that the DaemonSet re-applies the settings
          # on eviction or reschedule.
          echo "Setup complete. Sleeping indefinitely..."
          touch /tmp/setup-complete
          exec sleep infinity
        volumeMounts:
        - name: host
          mountPath: /host
        - name: coredump-config
          mountPath: /config
          readOnly: true
      volumes:
      - name: host
        hostPath:
          path: /
          type: Directory
      - name: coredump-config
        configMap:
          name: scylladb-coredump-conf
  updateStrategy:
    type: RollingUpdate
```

```bash
kubectl apply --server-side -f=https://raw.githubusercontent.com/scylladb/scylla-operator/v1.22/examples/gke/coredumps/setup-systemd-coredump.daemonset.yaml
```

Wait for the DaemonSet to roll out on all ScyllaDB nodes:

```bash
kubectl -n scylla-operator rollout status daemonset/scylladb-coredump-setup
```

### 3. Verify the configuration

After the DaemonSet rolls out, confirm `kernel.core_pattern` is correctly set on each node. List the dedicated ScyllaDB nodes:

```bash
kubectl get nodes -l scylla.scylladb.com/node-type=scylla -o name
```

Run the following command for each node, replacing `<node-name>` with the actual name:

```bash
kubectl debug node/<node-name> -it --profile=sysadmin --image=docker.io/library/ubuntu:24.04 -- \
  nsenter --mount=/proc/1/ns/mnt -- sysctl -n kernel.core_pattern
```

Expected output:

```console
|/usr/lib/systemd/systemd-coredump %P %u %g %s %t 9223372036854775808 %h %d
```

Also verify that `systemd-coredump.socket` is active:

```bash
kubectl debug node/<node-name> -it --profile=sysadmin --image=docker.io/library/ubuntu:24.04 -- \
  nsenter --mount=/proc/1/ns/mnt -- systemctl is-active systemd-coredump.socket
```

The output must be `active`.

## Verify that core dump collection works end to end

The steps below trigger a test crash of a running ScyllaDB process and confirm the dump was captured by `systemd-coredump`.

#### WARNING
This procedure intentionally crashes a ScyllaDB node. Only run it when the cluster can tolerate losing one member temporarily.

### 1. Find the pod and its node

```bash
NAMESPACE=<namespace>
kubectl get pods -n "${NAMESPACE}" -l scylla-operator.scylladb.com/pod-type=scylladb-node -o wide
```

Store the pod name and the node it is scheduled on:

```bash
POD_NAME=<pod-name>
NODE_NAME=<node-name>
```

### 2. Trigger the crash

Inside a pod managed by ScyllaDB Operator, the sidecar is PID 1 and the `scylla` binary runs as a child process. Send a `SIGABRT` signal to the `scylla` process to trigger a crash and core dump:

```bash
kubectl exec -n "${NAMESPACE}" "${POD_NAME}" -c scylla -- sh -c 'kill -ABRT $(pgrep -x scylla)'
```

ScyllaDB logs a backtrace and terminates. The pod stays running because the ScyllaDB Operator sidecar (PID 1) is unaffected; ScyllaDB Operator will restart the ScyllaDB process automatically. The dump is written to the node’s host filesystem before the process exits.

### 3. Confirm the dump was captured

Confirm the dump was captured using coredumpctl list - see [Retrieving core dumps from nodes]() for details.

## Retrieve core dumps from nodes

Core dumps are stored at `/var/lib/systemd/coredump/` on the host.

### 1. List available dumps

```bash
kubectl debug "node/${NODE_NAME}" -it --profile=sysadmin --image=docker.io/library/ubuntu:24.04 -- \
  nsenter --mount=/proc/1/ns/mnt -- coredumpctl list
```

Store the PID of the desired dump from the output:

```bash
DUMP_PID=<pid>
```

### 2. Export a specific dump

Start a debug pod on the node so that we can use `kubectl exec` to retrieve the dump file:

```bash
kubectl debug "node/${NODE_NAME}" --profile=sysadmin --image=docker.io/library/ubuntu:24.04 -- sleep 3600
```

Store the debug pod name:

```bash
DEBUG_POD_NAME=<debug-pod-name>
```

Pull the dump file from the node to your local machine (it can be very large, so this may take some time):

```bash
kubectl exec "${DEBUG_POD_NAME}" -- \
    nsenter --mount=/proc/1/ns/mnt -- coredumpctl dump "${DUMP_PID}" \
    > scylla.core
```

You can verify the dump with `file scylla.core` - it should show ELF 64-bit LSB core file.

## Storage considerations

Take into account that ScyllaDB core dumps can be very large. You will need spare disk space larger than that of ScyllaDB’s RAM. Core dump storage is controlled by the `[Coredump]` section of `/etc/systemd/coredump.conf`.

#### NOTE
`systemd-coredump` will automatically delete the oldest dump files when the `MaxUse` or `KeepFree` thresholds are exceeded, so some dumps may be lost if a node generates many crashes in a short period of time and the disk is nearly full.

To avoid losing dumps due to insufficient disk space, consider the following:

- **Attach a dedicated disk** to each ScyllaDB node at `/var/lib/systemd/coredump/` so core dumps do not compete with the OS for disk space.
- **Offload dumps to object storage** - the [IBM core-dump-handler](https://github.com/IBM/core-dump-handler) project provides a Helm chart that installs a similar `kernel.core_pattern` pipe handler and automatically uploads dumps to an S3-compatible bucket. This is a good option if you need centralized, long-term dump storage.
