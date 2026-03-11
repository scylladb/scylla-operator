# Configure coredumps

Set up coredump collection for ScyllaDB processes running on Kubernetes so that crashes can be analyzed post-mortem.

## Overview

When ScyllaDB crashes, a coredump provides the process memory and register state needed for root-cause analysis.
On Kubernetes, coredump collection requires host-level configuration because ScyllaDB runs inside a container.

## Configure systemd-coredump

The recommended approach is to use `systemd-coredump` on the Kubernetes nodes.

### Step 1: Configure coredump storage

Create or update `/etc/systemd/coredump.conf` on each ScyllaDB node:

```ini
[Coredump]
Storage=external
Compress=yes
ProcessSizeMax=infinity
ExternalSizeMax=infinity
MaxUse=100G
```

:::{caution}
**Storage sizing:** A ScyllaDB coredump can be as large as the process's memory allocation.
If your ScyllaDB pods have a 64 GiB memory limit, ensure at least 64 GiB of free space on the coredump storage volume.
Configure `MaxUse` accordingly.
:::

### Step 2: Set the kernel core pattern

Ensure the kernel is configured to use `systemd-coredump`:

```bash
echo '|/usr/lib/systemd/systemd-coredump %P %u %g %s %t %c %h' > /proc/sys/kernel/core_pattern
```

This can be applied via NodeConfig by adding a sysctl:

```yaml
apiVersion: scylla.scylladb.com/v1alpha1
kind: NodeConfig
metadata:
  name: scylla-node-config
spec:
  disableOptimizations: false
  placement:
    nodeSelector:
      scylla.scylladb.com/node-type: scylla
  localDiskSetup:
    # ... existing disk setup ...
  sysctls:
    - name: kernel.core_pattern
      value: "|/usr/lib/systemd/systemd-coredump %P %u %g %s %t %c %h"
```

:::{note}
Some managed Kubernetes providers may not allow modifying `kernel.core_pattern`.
Check your provider's documentation for coredump support.
:::

### Step 3: Verify resource limits

ScyllaDB must have an unlimited `RLIMIT_CORE` to generate coredumps.
Verify on a running pod:

```bash
kubectl -n scylla exec -it <pod-name> -c scylla -- prlimit --pid=$(pidof scylla) | grep CORE
```

The `SOFT` and `HARD` limits should show `unlimited`.

## Collect coredumps

After a crash, list available coredumps on the node:

```bash
# SSH to the node or exec into a privileged pod
coredumpctl list
```

Extract a specific coredump:

```bash
coredumpctl dump <PID> -o /tmp/scylla-coredump
```

## Off-the-shelf tooling

For automated coredump collection in Kubernetes, consider the [IBM core-dump-handler](https://github.com/IBM/core-dump-handler).
It watches for coredumps on nodes and uploads them to object storage.

## Related pages

- [Configure nodes](../deploy-scylladb/configure-nodes.md)
- [Production checklist](../deploy-scylladb/production-checklist.md)
- [Collect debugging information](collect-debugging-information/index.md)
