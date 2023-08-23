# Control external seeds

## Summary

This document proposes an extension to the existing ScyllaCluster API and corresponding changes to the ScyllaDB operator and sidecar.
The proposed extension should allow the end users to control external seeds provided to ScyllaDB nodes, which in turn should allow for manually setting up multi datacenter ScyllaDB clusters.

## Motivation

Currently, ScyllaDB Operator does not provide any support, neither automated, nor manual, for controlling external seeds propagated to ScyllaDB cluster nodes.
Controlling the provision of external seeds is a prerequisite for supporting both manual and automated setup of multi datacenter ScyllaDB clusters, a feature long requested by many of its users, and as such is a vital step on our roadmap.

### Goals

- Provide end users with a way of specifying external seeds.
- Support the extension in ScyllaDB Operator accordingly.
- Support for a manual procedure for creating multi datacenter ScyllaDB clusters and adding datacenters to existing ScyllaDB clusters, provided there is connectivity between the datacenters.

### Non-Goals

- Provide an automated way of creating a multi datacenter ScyllaDB cluster.
- Provide an automated way of extending existing ScyllaDB clusters with additional datacenters.
- Support a manual procedure for creating multi datacenter ScyllaDB clusters or extending existing ScyllaDB clusters without connectivity between the datacenters, i.e. this proposal shall not cover the infrastructure setup necessary for it, and only assume it is established.
- Support removal of a datacenter from a multi datacenter ScyllaDB cluster by just removing a ScyllaCluster. To remove a datacenter from a multi datacenter ScyllaDB cluster, a ScyllaCluster corresponding to that DC has to first be scaled down to zero and then the ScyllaCluster needs to be deleted. If externalSeeds of any other datacenters point to this datacenter, they have to be amended accordingly.
- Support a manual or automated procedure for creating a multi datacenter ScyllaDB cluster out of already bootstrapped datacenters.

## Proposal

This document proposes extending the existing ScyllaCluster CR with a new field in its Specification which allows the users to control the external seeds provided to ScyllaDB processes.
The changes required for ScyllaDB Operator and the sidecar to support the enhancement are also described.
In addition to that, the document also proposes the procedure for compiling the list of seeds propagated to ScyllaDB processes when external seeds are provided.

### User Stories

#### As a ScyllaDB cluster administrator:
- I want to control the list of external seeds propagated to ScyllaDB nodes, so that I can manually configure a multi datacenter ScyllaDB cluster.
- I want to control the list of external seeds propagated to ScyllaDB nodes, so that I can manually add a new datacenter to an existing ScyllaDB cluster.

### Notes/Constraints/Caveats [Optional]

#### Current role of seeds in ScyllaDB

With [scylladb/scylladb#6848](https://github.com/scylladb/scylladb/pull/6848), the role of seeds in ScyllaDB has been reduced to initial point of contact nodes.
According to the [documentation](https://opensource.docs.scylladb.com/stable/kb/seed-nodes.html): "**Once the nodes have joined the cluster, seed node has no function**".
However, investigating the gossiper's code proves that the seed nodes provided to ScyllaDB processes on initialization serve as fallback in case of no live nodes to talk with: https://github.com/scylladb/scylladb/blob/39ca07c49b257d37856164ea5b845f7b98224b98/gms/gossiper.cc#L1013.

Other than that, seeds do not have any additional significance past bootstrap.

#### Multi DC setup procedure

[Create a ScyllaDB Cluster - Multi Data Centers (DC)](https://opensource.docs.scylladb.com/branch-5.3/operating-scylla/procedures/cluster-management/create-cluster-multidc.html#procedure) procedure in ScyllaDB documentation explicitly states to use IP address of the first node, *and only the first node*, as seed for all nodes in all datacenters.
This has proven to be incorrect in two ways: 
1. Domain name addresses are also accepted as seeds. 
2. There is no special significance to the first node of the first DC - it is just a simplification used for the purpose of minimising the risk of creating multiple clusters instead of a single, multi DC cluster.

The specified constrains hold - the datacenters need a matching cluster name, distinct datacenter names across the cluster. The rack names only have to be distinct across a datacenter, not across the entire multi DC cluster.

### Risks and Mitigations

#### Lookup of seed addresses on ScyllaDB initialization
Currently, ScyllaDB performs an eager lookup of provided seeds on initialisation: https://github.com/scylladb/scylladb/blob/39ca07c49b257d37856164ea5b845f7b98224b98/init.cc#L35.
An error in address lookup will hence cause ScyllaDB process to fail initialisation. This is expected during bootstrap.

However, this is also going to cause an already bootstrapped node to fail after it has been restarted, e.g.:
```
ERROR 2023-08-03 08:35:54,663 [shard 0] init - Bad configuration: invalid value in 'seeds': 'dummy.scylla.svc.cluster.local': std::system_error (error C-Ares:4, dummy.scylla.svc.cluster.local: Not found)
```

Although the [documentation](https://opensource.docs.scylladb.com/stable/kb/seed-nodes.html) claims, that "**Once the nodes have joined the cluster, seed node has no function**", introducing the option to control the external seeds might cause instability to the ScyllaDB nodes controlled by the operator due to the above.
To mitigate this, an issue has been opened with ScyllaDB core: [scylladb/scylladb#14945](https://github.com/scylladb/scylladb/issues/14945).

## Design Details

### ScyllaCluster API

The following extension to the existing ScyllaCluster CR API is proposed:
```go
// ScyllaClusterSpec defines the desired state of Cluster.
type ScyllaClusterSpec struct {
    ...
	
	// externalSeeds specifies the external seeds to propagate to ScyllaDB binary on startup as "seeds".
	// See https://opensource.docs.scylladb.com/stable/kb/seed-nodes.html.
	// +optional
	ExternalSeeds []string `json:"externalSeeds,omitempty"`
}
```

The provided `externalSeeds` are passed to the sidecar through a new `--external-seeds` flag. StatefulSet reconciliation in ScyllaDB operator is modified accordingly.
The sidecar's strategy for choosing seeds to provide to ScyllaDB processes is modified to follow the below pseudocode:
```go
func chooseSeeds(externalSeeds []string, seedFromLocalDC string) []string {
  if is first node in DC {
    if externalSeeds not empty {
      return externalSeeds
    }
    return seedFromLocalDC
  }

  return append(externalSeeds, seedFromLocalDC)
}
```
The strategy for getting a seed from local DC (`seedFromLocalDC`) is not modified, i.e. for the first running node in the DC, its own address (`broadcast_address`) is selected, otherwise an address of one other running node from its own DC is selected.

It is important that the external seeds, and only the external seeds, are provided as seeds for the first running node in a datacenter, if they are at all provided by the end user. If external seeds were only appended to the node's own IP, it could result in creating split clusters in case of lack of connectivity between the datacenters.
For the other nodes, a seed from their own DC is appended to the external seeds for increased resiliency.

The above changes allow for configuring multi datacenter ScyllaDB clusters by manually providing external seeds.
The below examples assume the two datacenters are to be deployed in one Kubernetes cluster.

### Creating a multi datacenter ScyllaDB cluster
To create a multi datacenter ScyllaDB cluster, an end user would first deploy a ScyllaCluster.
To create the second datacenter, the end user would deploy a second ScyllaCluster with the same name, in a different namespace, with a distinct datacenter name and `.spec.externalSeeds` set to a list of IP address or domain names under which one or more nodes of the first datacenter are reachable from the second datacenter. 
ScyllaDB operator will propagate the declared `externalSeeds` through a sidecar flag, and the sidecar will take it into account when configuring the seeds provided to ScyllaDB nodes of the second datacenter. The two datacenters will then form a single multi datacenter cluster.

### Adding a new datacenter to an existing ScyllaDB cluster
To add a new datacenter to an existing ScyllaCluster, the same procedure as above is followed, only skipping the first step of deploying the first ScyllaCluster.

### Test Plan

To test the proposed API extension, unit tests verifying that the external seeds are propagated to the sidecar will be added.
Additionally, E2E tests should verify whether a single, multidatacenter ScyllaDB cluster is formed when additional datacenters with external seeds pointing to existing datacenter are created.

### Upgrade / Downgrade Strategy

Regular Operator upgrade/downgrade strategy.

#### Operator and ScyllaCluster CR version skew
In case of an old operator, regardless of the `externalSeeds` value, the opeerator will ignore it, and the seeds won't be propagated to the ScyllaDB Statefulset's Pod template.
In case of a new operator, if the `externalSeeds` are nil, the operator won't take any action.

## Implementation History

- 2023-07-31: Initial enhancement proposal

## Drawbacks

### Potential misconceptions related to multi datacenter setup
One of the non-goals of this proposal is automating the creation of multi datacenter ScyllaDB clusters. Extending the ScyllaCluster API with an option of controlling the external seeds provided to ScyllaDB nodes might create a misconception that ScyllaDB operator automates the set operations related to a multi datacenter setup, which would normally be handled in a single datacenter cluster. Such operations might involve e.g. decommissioning the nodes of a datacenter before deleting the ScyllaCluster resource.

## Alternatives

### Seed overriding in sidecar
As an alternative to the proposed algorithm compiling the seeds provided to ScyllaDB nodes subsequent to the first in the DC, overriding any seeds with just the external seeds has been considered.
However, it proved to be less resilient in cases where the node would have no live nodes to talk with and it would fall back to using the initially provided seeds: https://github.com/scylladb/scylladb/blob/39ca07c49b257d37856164ea5b845f7b98224b98/gms/gossiper.cc#L1013.

### Additional CR
As an alternative to enhancing the existing ScyllaCluster CR API, creating an additional, separate CR, used for linking existing datacenters could be considered.
However, rolling out the underlying ScyllaCluster resources is necessary to generate the adjusted configuration. Therefore, it doesn't seem to have any benefits over the proposed solution.
