# Run node cleanup after scaling the cluster

## Summary

The Operator supports both horizontal and vertical scaling, but the procedure isn’t in sync with ScyllaDB documentation,
because the cleanup part is not implemented. It’s important because upon scaling, the stored data on node disk might
become stale taking up unnecessary space or even cause data to be resurrected. Operator should support running the
cleanup to
keep disk space as low as possible and ensure clusters are stable and reliable over time.

## Motivation

The motivation behind running node cleanup after scaling a cluster is to avoid the accumulation of unnecessary data on
the node disks, and prevent from data resurrection. When nodes are added or removed from the cluster, they gain or lose
some tokens, which can result in files stored on the node disks still containing data associated with lost tokens. Over
time, this can lead to a build-up
of unnecessary data and cause disk space issues. Data resurrection happens when tombstones delete data on other nodes,
and the tombstone is eventually purged, leaving behind neither the data nor the tombstone. When cluster is scaled down
and token ownership is moved back to the
original node, that might still have the data, because the tombstone that deleted it was purged, the data gets
resurrected.
By running node cleanup after scaling, these files can be cleared, freeing up disk space and prevent from data
resurrection.

### Goals

* Running a node cleanup procedure after horizontally scaling the cluster.
* Running a node cleanup procedure after scaling is completed, on all nodes but the last one added.

### Non-Goals

* Running a node cleanup during off-peak hours.
* Running a node cleanup on all necessary cases.

## Proposal

I propose to extend the sidecar with a new controller that will project the latest cleaned up and existing hash of
tokens for each node as an annotation in the member service.
In addition, a new controller in Operator will be responsible for managing Jobs that will execute a cleanup on nodes
that require it. The trigger for the Job creation will be a mismatch between the current and latest hash.

### Risks and Mitigations

#### Cluster bootstrap time is increased

When a cluster is created for the first time, tokens are reshuffled when new nodes are joining sequentially. This will
cause the token hash mismatch and will require a cleanup once all nodes join the cluster. This will cause cluster
bootstrap time to be increased. But since the time window is short, nodes shouldn’t have much data stored so cleanup
won’t take long.

#### Cleanup is run when not necessary

This design doesn’t take into account whether a node received a token or lost it, it only detects ring changes and
reacts with a cleanup trigger upon change. When a node is decommissioned, tokens are redistributed and nodes getting
them don't require a cleanup since there’s no stale data on their disks associated with these new tokens. Operator
will trigger the cleanup anyway.
ScyllaDB plans to automate cleanup and cover all the cases, until then we will run unnecessary cleanup in some cases.

#### Cleanup is not run when necessary

When keyspace RF is decreased, nodes no longer need to keep extraneous copies of the data, cleanup could free the disks.
Approach designed here doesn't detect this case because the token ring is not changed.
When decommission fails in the middle the token ring won’t be changed, but the tokens might be rebuilt on the nodes
requiring a cleanup. Because this isn’t visible externally, the proposed solution won’t trigger a cleanup.
When replication strategy of keyspace is changed, token ownership of secondary replicas is changed but the ring stays
intact,
hence cleanup won't be triggered.

ScyllaDB plans to automate cleanup and cover all the cases, until then we won’t cover these cases.

## Design Details

Scylla-operator will be extended with a controller responsible for running a cleanup procedure when needed.

### Cleanup trigger

The root cause of the cleanup requirement is that during cluster changes tokens are moved between nodes.
There are two alternatives of detecting when this happened:

1. ScyllaCluster topology changes - new rack is added/removed, number of members changes, or additional CPUs are
   added/removed.
2. Check tokens in Scylla API, detect when it changed.

The first one is edge driven, there are multiple cases when tokens might move or not, for example if cpu resource change
won’t be big enough to allocate additional core the cleanup is not needed. For hybrid clusters or multi datacenter
clusters, topology change in the non-managed part should cause the cleanup of nodes in the managed part which is
impossible to detect. It’s also harder to implement since StatefulSets, which changes need to be detected, doesn't allow
for per node distinction, the state needs to be carried to other resources (for example member Service) which brings
unnecessary coupling.

The second one is checking the actual root cause, and seems more bulletproof.

### Implementation

Two new annotations in member services will be reconciled. Controller in the sidecar will
reconcile `internal.scylla-operator.scylladb.com/current-token-ring-hash`
and Operator will reconcile `internal.scylla-operator.scylladb.com/last-cleaned-up-token-ring-hash`.

The `current-token-ring-hash` one will always reflect the hashed output of Scylla API `/storage_service/tokens` endpoint
from a given
node. The `last-cleaned-up-token-ring-hash` will be initially empty and set after a cleanup job finishes
to `current-token-ring-hash` value that was present at the time the cleanup job was created.

The output of tokens endpoint is json formatted sorted array of strings (representing numbers), the annotation will hold
SHA512
hash of it as a value.

The cleanup call is synchronous, client only triggers the operation, and all the resource intensive operations are done
by Scylla. The duration of the operation is tied to the size of the entire dataset and disk performance. Cleanup on
nodes
that don't require it, also impacts the disk IO.
To prevent the controller sync loop from being too long, the cleanup controller in Operator will be responsible for
managing cleanup Jobs. The preconditions for Job for given node are:

* `current-token-ring-hash` and `last-cleaned-up-token-ring-hash` token ring hash annotations values are different.
* Observed generation is equal to ScyllaCluster generation.
* Topology changes are done and upgrade is done - ScyllaCluster Status contains Condition `StatefulSetControllerProgressing=False`.
* All nodes are UN - ScyllaCluster Status contains condition `Available=True` and `Degraded=False` conditions.

The Job requires access to Scylla API to initiate the cleanup. Because Scylla API is bound to localhost, it means we
have to go through Scylla Manager Agent which is able to forward the traffic to Scylla. It uses token based
authentication, which means the controller will have to mount the Secret into the Job as a volume.
The alternative is to extend the Operator sidecar with request forwarding capabilities, but since we already have this
capability there’s no need for duplication.

### Status

Cleanup status will be visible as condition in `ScyllaCluster.status.conditions` via `CleanupControllerProgressing` and
`CleanupControllerDegraded` and will influence the status of aggregated `Progressing` and `Degraded` ScyllaCluster
conditions.

## Test Plan

E2E tests testing whether correct annotations are projected into services, whether Jobs are created for all nodes
requiring a cleanup.

## Upgrade / Downgrade Strategy

Regular Operator upgrade/downgrade strategy.
In case of a downgrade, new annotation will persist in Services, cleanup Jobs won’t be removed until ScyllaCluster
exists.

## Version Skew Strategy

In case when an old Operator would be a leader, it would keep existing hash annotations without triggering cleanup. In
case a new Operator is a leader, cleanup would be triggered when all conditions are matched.

## Drawbacks

As one of non-goals is lack of running cleanup during off-peak hours, and because cleanup is run in some cases when it's
not needed users may see increased latency and decreased
throughput after scaling the cluster for a period of running cleanup Jobs.
