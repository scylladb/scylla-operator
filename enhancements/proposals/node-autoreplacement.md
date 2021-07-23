# Node Autoreplacement

## Summary

Node Autoreplacement enhancement extends Scylla Operator with a functionality of automated replacement of nodes which lost their storage, which, at this point, renders them getting stuck on joining the cluster.

This proposal suggests creating a module responsible for triggering a node replacement in such circumstances.
As a side effect, introducing such functionality would allow for simplifying the existing `OrphanedPVController` by
reducing its task to evicting orphaned PVs.

## Motivation

Currently, when a node looses its storage, it gets stuck on joining a cluster, since Scylla produces a following runtime error:
```
Startup failed: std::runtime_error (A node with address <ip> already exists, cancelling join. Use replace_address if you want to replace this node.)
```

Successfully restarting the node requires a manual intervention of a human operator by annotating the
service corresponding to the affected node with a `scylla/replace` label. The main goal of this proposal is to improve
Scylla Operator's stability in running Scylla independently of its administrators' interventions.

### Goals

- Successfully discovering and autoreplacing nodes which lost their storage without requiring user intervention.

### Non-Goals

- Removing orphaned PVs. This is left for `OrphanedPVController`.

## Proposal

Extend ScyllaCluster controller with code responsible for checking whether a given Scylla node:
1. is **not** a first bootstrapping node in the cluster
2. already exists in gossip

In case both of these conditions are true, a dedicated ConfigMap is created/updated.
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: <service_name>-autoreplacement-config
  ...
data:
  autoreplacement.config: |
    replace-address-first-boot: <clusterIP>    
```

The config is later appended to the generic config during Scylla setup, which effectively triggers running Scylla with `replace-address-first-boot` flag.
The ConfigMap takes precedence over the generic config. In case `replace-address-first-boot` is already defined there, it is overwritten.

The ConfigMap is optional and unavailable during the first start of a pod. It is not live reloaded and only read during the entrypoint setup, after every restart of a pod.
The ScyllaCluster controller talks to the nodes whenever a generated ConfigMap differs from the existing one, which essentially comes down to two cases: after first boot and after a change to corresponding service's `ClusterIP`.

It is necessary to check whether a node is first in the cluster, as the first node in the cluster skips bootstrap, and therefore the flag can't be set.
Checking for the IP already existing in gossip is necessary as it is not possible to replace a node that does not.
Setting `replace-address-first-boot` results in a desired output, i.e. a node which lost its storage will be replaced during bootstrap.
Otherwise, setting the flag is safe as Scylla ignores it after bootstrap.

Running a manual node replacement does not affect the autoreplacement process.

Introducing this enhancement additionally allows for removing replacement functionality from `OrphanedPVController`. The controller would be simplified and only be responsible for removing the orphaned PVs.

### User Stories

#### Running ScyllaCluster on AWS/GCP

The user is running a ScyllaCluster on AWS/GCP cluster with local storage. One of the instances is subject to an event resulting in erasing the local storage (see [AWS Instance store lifetime](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/InstanceStorage.html#instance-store-lifetime) and [GCP Local SSD data persistance](https://cloud.google.com/compute/docs/disks/local-ssd#data_persistence))  .
The Scylla node scheduled on this instance loses its storage, even though a PVC bound to it is not removed. The Scylla node gets replaced automatically.

### Notes/Constraints/Caveats [Optional]

- It would be preferred to use [Immutable ConfigMaps](https://kubernetes.io/docs/concepts/configuration/configmap/#configmap-immutable). It protects the system from any accidental updates or unwanted user interventions that could cause the cluster to misbehave. 
- In case there is only one Scylla node in a cluster, the node can't be replaced, as `replace-address` and `replace-address-first-boot` are not allowed for seed nodes, and the singular node is its own seed.

### Risks and Mitigations

- The proposed design is based on a fact that `replace-address-first-boot` flag is ignored after bootstrap. Any changes to this behaviour or side effects introduced could result in an undefined behaviour of the enhancement. To mitigate, we could try to have it covered by tests in Scylla.

## Design Details

To check whether the Scylla node is not the first to bootstrap in a cluster and already exists in gossip, the controller uses [`ScyllaClient's Status`](https://github.com/scylladb/scylla-operator/blob/689c0a0a82aeea0fef72b2cd11af0333d1d2e180/pkg/scyllaclient/client.go#L98).
Client is created with default config. Controller authenticates with the already known token. It uses all nodes of a given cluster as a pool of hosts. The conditions for creating the ConfigMap are met if the node's IP is among the returned hosts.

`Sidecar` accesses the ConfigMap through a dedicated Volume. 

### Test Plan

- E2E test plan:
  - Create a ScyllaCluster with 2 nodes
  - Wait for the ScyllaCluster to deploy
  - Remove first node's PVC
  - Remove first node's Pod
  - Await PVC's and Pod's deletion
  - Wait for the ScyllaCluster to reconcile and become fully available again
  - Verify ScyllaCluster

### Upgrade / Downgrade Strategy

No upgrade/downgrade action is needed, no manifests are changed.

### Version Skew Strategy

There might occur a version skew between a Scylla Operator and a ScyllaCluster deployed by a Scylla Operator of a different version.

1. ScyllaCluster deployed with an unupgraded Scylla Operator. Upgraded Scylla Operator running: ScyllaCluster would ignore the ConfigMap.
2. ScyllaCluster deployed with upgraded Scylla Operator. Unupgraded Scylla Operator running: no ConfigMaps would be created.

In both cases the Node Autoreplacement feature would fail. The administrator would have to use the manual node replacement as a fallback.

## Implementation History

- 1.5, enhancement proposal created

## Drawbacks

- Controller needs to make API calls to Scylla nodes. However, it would only do so on first start of each node or if `ClusterIP` of a corresponding service changed. 

## Alternatives

- Alternatives to extending the controller:
    - A new InitContainer could be added to the StatefulSet's `PodTemplateSpec`, although it seems more cumbersome compared to extending Scylla Operator's controller, as it would require performing many operations that the controller does anyway. 
    - This functionality could be added to the existing sidecar, since triggering the replacement is dependent on the sidecar anyway. However, sidecar is meant to be removed in the future and such approach impedes the separation of concerns even further.
- Instead of merging the `ConfigMap` with the general Scylla config, a different approach (mapping to a file, creating an environment variable) could be used to trigger setting the `replace-address-first-boot` flag.
- `ScyllaClient` makes multiple API calls in order to get a status of each host. While this enhancement only needs to know whether a given node is in gossip, a simpler `ScyllaClient` method could be added.

## Infrastructure Needed [optional]

None.