# Configure precomputed IO properties

Provide precomputed IO properties to ScyllaDB to skip the automatic `iotune` benchmark that runs on first startup.

## When to use this

By default, ScyllaDB runs an `iotune` benchmark when a node starts for the first time.
The benchmark measures the IO capabilities of the underlying storage and writes the results to `io_properties.yaml`.
The results are cached on the persistent volume, so subsequent startups reuse them.

Running the benchmark is appropriate for most deployments.
However, you may want to provide precomputed values when:

- You know the exact IO characteristics of your storage (e.g. cloud provider published IOPS/throughput specs).
- You want to ensure consistent IO configuration across all nodes.
- You want to skip the benchmark to speed up initial cluster bootstrap.

## How it works

The Operator's sidecar checks for `/etc/scylla.d/io_properties.yaml` before starting ScyllaDB.
If the file exists, the sidecar passes `--io-setup=0 --io-properties-file=/etc/scylla.d/io_properties.yaml` to the ScyllaDB binary, which skips the iotune benchmark entirely.
If the file does not exist, the sidecar creates a symlink to a cache location on the persistent volume, where iotune results are stored after the first run.

To provide precomputed values, mount a ConfigMap containing `io_properties.yaml` into `/etc/scylla.d/` on the ScyllaDB container.

## Procedure

### Step 1: Create the IO properties ConfigMap

Create a ConfigMap containing your precomputed IO properties.
Refer to the [ScyllaDB IO properties documentation](https://opensource.docs.scylladb.com/stable/sysadmin/admin-tools/scylla-io-setup.html) for the file format.

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: scylla-io-properties
  namespace: scylla
data:
  io_properties.yaml: |
    disks:
      - mountpoint: /var/lib/scylla
        read_iops: 200000
        read_bandwidth: 1200000000
        write_iops: 100000
        write_bandwidth: 600000000
```

```bash
kubectl -n scylla apply -f io-properties-configmap.yaml
```

### Step 2: Mount the ConfigMap into the ScyllaDB container

:::::{tabs}
::::{group-tab} ScyllaCluster

Use `volumes` and `volumeMounts` on the rack spec to mount the ConfigMap into the ScyllaDB container:

```yaml
apiVersion: scylla.scylladb.com/v1
kind: ScyllaCluster
metadata:
  name: scylla
  namespace: scylla
spec:
  datacenter:
    name: us-east-1
    racks:
      - name: us-east-1a
        members: 3
        storage:
          capacity: 500Gi
        resources:
          limits:
            cpu: 4
            memory: 8Gi
        volumes:
          - name: io-properties
            configMap:
              name: scylla-io-properties
        volumeMounts:
          - name: io-properties
            mountPath: /etc/scylla.d/io_properties.yaml
            subPath: io_properties.yaml
            readOnly: true
```

### Step 3: Apply and verify

Apply the cluster spec.
After the pods start, verify that the IO properties are being used:

```bash
kubectl -n scylla exec -it scylla-us-east-1-us-east-1a-0 -c scylla -- cat /etc/scylla.d/io_properties.yaml
```

You should see your precomputed values.
The ScyllaDB logs should show that iotune was skipped:

```bash
kubectl -n scylla logs scylla-us-east-1-us-east-1a-0 -c scylla | grep -i "io properties"
```

:::{caution}
Providing incorrect IO properties can cause ScyllaDB to over-commit or under-utilise the disk subsystem, leading to performance degradation or instability.
Ensure the values match the actual capabilities of your storage.
:::

## Key considerations

| Consideration | Detail |
|---|---|
| Persistent cache | By default, iotune results are cached on the persistent volume. Precomputed values are only needed if you want to skip or override the benchmark. |
| Consistent values | All nodes sharing the same storage type should use the same IO properties. Mount the same ConfigMap across all racks that use the same storage class. |
| Rolling restart | Adding or changing the volume mount triggers a rolling restart. The Operator restarts nodes one at a time. |
| Storage class changes | If you change the storage class or move to a different disk type, update the IO properties ConfigMap to match the new storage. |

## Related pages

- [Pass additional ScyllaDB arguments](pass-scylladb-arguments.md) — passing arbitrary command-line flags to ScyllaDB
- [ScyllaDB IO Setup documentation](https://opensource.docs.scylladb.com/stable/sysadmin/admin-tools/scylla-io-setup.html) — upstream reference for the `io_properties.yaml` format
