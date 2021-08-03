# Improve UX around disks

## Summary

Currently, when a user wants to use Scylla with local drives, they need to manually
create an RAID0 array, format it to XFS and expose it as a PersistentVolume on every Node dedicated to Scylla.
Formatting operations are already implemented in scripts available in Scylla image.  
We can use a DaemonSet proposed in [perftune.md](perftune.md) to access and modify physical drives.  
For exposing PersistentVolumes we may deploy [storage-local-static-provisioner](https://github.com/kubernetes-sigs/sig-storage-local-static-provisioner) 

Another bad example of UX is io-setup benchmark not being cached. IOSetup takes ~2 minutes to run
and the result is saved in ephemeral volume, which means it has to be repeated on every
Pod restart.  
Scylla recommends, to have the same content of io_properties.yaml (benchmark result) on every node, and
for known instance types, these values are pre-computed. 
We may leverage that to speedup even first bootstrap.

## Motivation

Setting it up is not trivial, and different cloud providers have different setup scripts examples.
Because we recommend to use local drives, we should do our best to have good user experience around it.

### Goals

* Automate creating RAID0
* Automate setting up XFS partition
* Automate exposing local disks as PersistentVolumes
* Cache io-setup benchmark results
* Use precomputed io-setup benchmark results for known instance types

### Non-Goals

* Setup and format network attached drives.

## Proposal

Three main focus points of this proposals are:
* Setting up the disks
* Exposing disks as PersistentVolumes
* Caching of io-setup results

Each of them have to be optional, because users may have their own solution to each of this problem or our solution may not work 
on their types of machines.

The first two of those require root access to host machine, third one requires read access to Pod storage.
These are going to be run by DaemonSet described in [perftune.md](perftune.md), because it 
has root access to host machine, and it's running on every k8s node having Scylla Pods, so it will
be able to access both local PVs and network attached PVs. 
The third one requires access to Pod storage and ability to create ConfigMaps. 

For setting up the disks we are going to execute `scylla_raid_setup` script. 
This will create a RAID0 array and will format partition to XFS.

To expose disks as PersistentVolumes, Scylla Operator is going to reconcile an instance of [storage-local-static-provisioner](https://github.com/kubernetes-sigs/sig-storage-local-static-provisioner).
This provisioner watches for mounts in configurable location, and creates a PV for each of them.

Because for disk benchmarking not only PV access is required, but also CPU resources, we are going to inject 
commands into Scylla Pod. We cannot use Scylla to run it because in one case an additional parameter is needed. 
Details are described in separate paragraph.

### User Stories

#### Default Cloud Provider setup

On currently supported Cloud Providers (EKS, GKE) default instances have not formatted and unmounted local drives.
When user install Scylla Operator disks will be configured and exposed as PV automatically. Setting `local-raid-disks` 
storage class in ScyllaCluster RackSpec will configure Scylla data directory to be placed on local disk.

#### User provided Disk Provisioner

User may have his own Disk Provisioner. In such case, he should disable installing our provisioner by setting
`DiskSetup.InstallDiskProvisioner` to `false`.
If `DiskSetup.SetupLocalDisk` is enabled, he should also sync `DiskSetup.LocalDisksHostPath` with his provisioner config.

### Notes/Constraints/Caveats 

We assume that disk performance doesn't change during PV lifetime, so as long as PV exists the ConfigMap with
io-setup result will exist.
Users wanting their results to be recomputed, may remove key from ConfigMap holding it and restart Scylla.

### Risks and Mitigations

ScyllaNodeTuner DaemonSet will have system admin privileges, although it won't be exposed to end users and it will be running in a namespace owner by the cluster admin.
This ensures the ServiceAccount won't be accessible to regular users.

Data loss is possible in case of a bug in disk formatting logic. Because we are going to use script from Scylla which is 
already widely used across different types of deployments we should be safe. 

## Design Details

### API
ScyllaNodeTuner is going to be extended with following API:

```

type ScyllaNodeTunerConditionType string

type ScyllaNodeTunerCondition struct {
    Type   ScyllaNodeTunerConditionType `json:"type"`
    Status corev1.ConditionStatus `json:"status"`
}

type ScyllaNodeTunerStatus struct {
    ObservedGeneration *int64               `json:"observedGeneration,omitempty"`
    Conditions         []ScyllaNodeTunerCondition `json:"conditions"`
}

// ScyllaNodeTunerPlacement is used to distribute ScyllaNodeTuner Pod along k8s Nodes.
type ScyllaNodeTunerPlacement struct {
    Affinity *corev1.Affinity `json:"affinity,omitempty"`
    // +optional
    Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

    NodeSelector map[string]string `json:"nodeSelector"`
}

type ScyllaNodeTunerDiskBenchmarkOptions struct {
    // ignorePrecomputed controls if precomputed values for known instance types should be ignored.
    IgnorePrecomputed bool `json:"ignorePrecomputed"`
}

type DiskProvisionerOptions struct {
    // storageClass to which PV created by Disk Provisioner will be bound.
    StorageClass string `json:"storageClass"`
}

type ScyllaNodeTunerSpec struct {
    Placement ScyllaNodeTunerPlacement `json:"placement"`

    // diskSetupDisabled controls if ScyllaNodeTuner should set up local disks.
    // Currently it creates a RAID0 array and format partition to XFS.
    DiskSetupDisabled bool `json:"diskSetupDisabled"`

    // localDisksPath path on host where raid drive will be created. 
    // Disk Provisioner is going to use this path to watch for disks to expose.
    // Users using their own Disk Provisioner should sync this path with their installation.
    // Changing this value won't move existing mounted drives to new location.
    // +k8s:default="/mnt/raid-disks"
    LocalDisksPath string `json:"localDisksPath"`
    
    // pvProvisionerDisabled controls if PV Provisioner shouldn't be installed.
    PVProvisionerDisabled bool `json:"pvProvisionerDisabled"`
    
    // pvProvisionerOptions contains options for deployed PV Provisioner.
    PVProvisionerOptions *DiskProvisionerOptions `json:"pvProvisionerOptions"`

    // diskBenchmarkDisabled controls if disk benchmark cacheing should be disabled.
    DiskBenchmarkDisabled bool `json:"diskBenchmarkDisabled"`
    // diskBenchmarkOptions contains options for disk benchmark cacheing.
    DiskBenchmarkOptions  *ScyllaNodeTunerDiskBenchmarkOptions `json:"diskBenchmarkOptions"`
}

```

### RBAC

ClusterRole bound to ServiceAccount used by ScyllaNodeTuner DaemonSet is going to be extended with read permissions to PersistentVolumes and Pods.

Operator will provision a new ServiceAccount for Disk Provisioner DaemonSet with `system:persistent-volume-provisioner` ClusterRole.

### Flow

Scylla Operator is going to reconcile Disk Provisioner and ScyllaNodeTuner DaemonSet based on values in available ScyllaNodeTuner CR.

If `ScyllaNodeTuner.DiskSetupDisabled` is `false` ScyllaNodeTuner DaemonSet is going to run `scylla_raid_setup` script which 
discovers disks attached to the instance and creates RAID0, formats it to XFS. Created disks will be mounted in 
location specified in `ScyllaNodeTuner.LocalDisksPath`.

When new disk appears in specified location, Disk Provisioner will expose them as PersistentVolumes.

Sidecar before Scylla process is spawned, checks if ConfigMap `scylla-node-tuner` mounted as directory contains `iosetup-<pv-uid>` file. 
It doesn't use API, but check if filesystem contains such file.
If it exists and is not empty, its content is copied to `/etc/scylla.d/io_properties.yaml`. If not, sidecar proceeds to spawning Scylla.
If `ScyllaNodeTuner.DiskBenchmarkDisabled` is `true` Operator creates an empty `iosetup-<pv-uid>` key in `scylla-node-tuner` ConfigMap to unblock sidecar.
Otherwise, ScyllaNodeTuner is responsible for filling it in.

### Precomputed benchmark

`scylla_io_setup` script which contains logic for detecting and calculating precomputed values for known instances requires
special parameter (--ami) in order to skip actual benchmark on AWS and to use precomputed values. 
This parameter has unfortunate behavior when AWS instances are *not* in recommended set (i3, i3en, and more) - it just exits with 1 code.  
So before running this script with this parameter we have to be sure that we are on AWS and instance is recommended.  

ScyllaNodeTuner Pod is going to detect if we are running on AWS, and if instance type is one of the recommened types 
(we will hardcode the list based on `scylla_util`). If so an `--ami` parameter is added to `scylla_io_setup`.
GKE and other Clouds and deployments doesn't require any additional flags.  

If `ScyllaNodeTuner.DiskBenchmarkOptions.IgnorePrecomputed` is `true`, instead running `scylla_io_setup` ScyllaNodeTuner will direcly execute `iotune`.

#### User provided Disk Provisioner

User may have his own Disk Provisioner. In such case, he might want disable our provisioner by setting
`ScyllaNodeTuner.PVProvisionerDisabled` to `true`.
If `ScyllaNodeTuner.DiskSetupDisabled` is not disabled, he should also make sure that his Disk Provisioner is using `ScyllaNodeTuner.LocalDiskPath`.  

### Test Plan

Due to lack of local storage available in e2e it won't be possible to test the disk setup. This will have to be covered by the QA test suite.

E2E plan:
* verify if Node Tuner and Disk Provisioner DaemonSets are reconciled. 
* verify that Scylla use cached benchmark result if it's available.
* verify that Scylla benchmark result is cached in ConfigMap once finished.
* verify that precomputed benchmark results are ignored if flag is enabled. 

### Upgrade / Downgrade Strategy

Users having their own Disk Provisioner have to disable installing ours in ScyllaNodeTuner manifest during upgrade.
Downgrade requires a manual removal of created DaemonSets.

### Version Skew Strategy

Scylla Operator reconciles every component involved in described proposal, so versions of components should be in sync.
For a short period of time version of ScyllaCluster and components may vary. It will result in Scylla sidecar not using benchmark results. 
Eventually version of sidecar will be updated and correct benchmark will be picked up. 

## Drawbacks

None

## Alternatives

Different idea to inform sidecar if it has to wait for disk benchmark would be a dedicated ConfigMap with sidecar config.
Operator would have to keep track of Scylla and Node Tuner Pods running on the same Node, and create CM in each Scylla namespace.
Label approach with finalizer seems easier. 
