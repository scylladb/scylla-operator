# Overview

ScyllaDB works with both local and network attached storage provisioners, but our primary focus is around the local storage that can provide the best performance. We support using 2 local provisioners: [ScyllaDB Local CSI Driver](https://github.com/scylladb/local-csi-driver) and [Kubernetes SIG Storage Local Persistence Volume Static Provisioner](https://github.com/kubernetes-sigs/sig-storage-local-static-provisioner).

## Setting up local disks

When a Kubernetes node having local disk(s) is created, the storage is usually uninitialized. This heavily depends on your platform and its options, but even when provisioned with mounted disks, they usually don't set up *RAID*, nor have the option to choose a file system type. (ScyllaDB needs the storage to be formatted with `xfs`.)

Setting up the RAID arrays, formatting the file system or mounting it in a declarative manner is challenging and that's one of the reasons we have created the [NodeConfig](../../resources/nodeconfigs.md) custom resource.

## Supported local provisioners

### ScyllaDB Local CSI driver

ScyllaDB Local CSI Driver supports dynamic provisioning on local disks and sharing the storage capacity.
It is based on dynamic directories and **xfs prjquota**.
It allows [PersistentVolumes](https://kubernetes.io/docs/concepts/storage/persistent-volumes/) to be created dynamically for a corresponding [PersistentVolumeClaim](https://kubernetes.io/docs/concepts/storage/volumes/#persistentvolumeclaim) by automatically provisioning directories created on disks attached to instances. On supported filesystems, directories have quota limitations to ensure volume size limits.

At this point, the Local CSI Driver doesn't support provisioning block devices.

### Kubernetes SIG Storage Static Local Volume Provisioner

The local volume static provisioner is a Kubernetes SIG Storage project that can turn your disks into dedicated and isolated PersistentVolumes but all of them have to be created manually.
