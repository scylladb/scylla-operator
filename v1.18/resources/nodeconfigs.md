# NodeConfigs

NodeConfig is an API object that helps you set up and tune the nodes.

```yaml
apiVersion: scylla.scylladb.com/v1alpha1
kind: NodeConfig
metadata:
  name: scylladb-pool-1
spec:
  localDiskSetup:
    raids:
    - name: nvmes
      type: RAID0
      RAID0:
        devices:
          nameRegex: ^/dev/nvme\d+n\d+$
    filesystems:
    - device: /dev/md/nvmes
      type: xfs
    mounts:
    - device: /dev/md/nvmes
      mountPoint: /var/lib/persistent-volumes
      unsupportedOptions:
      - prjquota
  placement:
    nodeSelector:
      scylla.scylladb.com/node-type: scylla
    tolerations:
    - effect: NoSchedule
      key: scylla-operator.scylladb.com/dedicated
      operator: Equal
      value: scyllaclusters
```

## Disk setup

NodeConfig can set up the disks into a RAID array, create a filesystem and mount it somewhere, so it can be consumed by the [Local CSI Driver](https://operator.docs.scylladb.com/v1.18/architecture/storage/local-csi-driver.md)

## Performance tuning

Unless you explicitly disable tuning on a NodeConfig, all matching Kubernetes nodes are subject to tuning.
You can learn more about tuning in [a dedicated architecture section](https://operator.docs.scylladb.com/v1.18/architecture/tuning.md)

#### WARNING
We recommend that you first try out the performance tuning on a pre-production instance.
Given the nature of the underlying tuning script, undoing the changes requires rebooting the Kubernetes node(s).

## Status

Given NodeConfig specification needs to reference local disk by names or that the referenced storage can be already used / mounted by something else, you should pay special attention to verifying that everything succeeded.
NodeConfig have the standard aggregated conditions to easily check whether everything went fine:

```bash
kubectl get nodeconfigs.scylla.scylladb.com/scylladb-pool-1
```

```console
NAME              AVAILABLE   PROGRESSING   DEGRADED   AGE
scylladb-pool-1   True        False         False      37d
```

or programmatically wait for it:

```default
kubectl wait --timeout=10m --for='condition=Progressing=False' nodeconfigs.scylla.scylladb.com/scylladb-pool-1
kubectl wait --timeout=10m --for='condition=Degraded=False' nodeconfigs.scylla.scylladb.com/scylladb-pool-1
kubectl wait --timeout=10m --for='condition=Available=True' nodeconfigs.scylla.scylladb.com/scylladb-pool-1
```

If the NodeConfig doesn’t reach the expected state, look at the fine-grained conditions in its status to find the cause.
