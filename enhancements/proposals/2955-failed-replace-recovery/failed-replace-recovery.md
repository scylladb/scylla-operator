# Procedure to Recover from Failed Node Replace

## Summary

Node replace operations can fail, leaving the ScyllaDB cluster in an inconsistent state in which Operator cannot apply any further configuration updates.
The [failed membership change](https://docs.scylladb.com/manual/branch-2025.1/operating-scylla/procedures/cluster-management/handling-membership-change-failures.html#cleaning-up-after-a-failed-membership-change) tutorial in the ScyllaDB docs is not correct for Operator, because there is a need to also keep the Kubernetes state (the `StatefulSet`, the node service, the storage) consistent.

## Motivation

No well-defined recovery path in scenarios where Operator gets stuck for unhandled reasons.

### Goals

- Enable users and CX with a well-defined procedure to perform when the node replace is stuck.
- Put this procedure in Operator's public docs.
- Understand if this procedure is reusable for different classes of failures.

### Non-Goals

- Add more automation to Operator

## Proposal

The proposal is to add a guide that builds upon the [failed membership change guide](https://docs.scylladb.com/manual/branch-2025.1/operating-scylla/procedures/cluster-management/handling-membership-change-failures.html#cleaning-up-after-a-failed-membership-change) that would employ the following steps:

#### Verify that there is indeed a node that failed to join the cluster

Perform `nodetool status` on a functioning node of the cluster (different than the culprit node). You should see a node with status `DN` or `?N`.

```console
$ kubectl exec -n examplens scylla-exampledc-somehealthynode-5 -- nodetool status

Datacenter: exampledc
=====================
Status=Up/Down
|/ State=Normal/Leaving/Joining/Moving
-- Address        Load      Tokens Owns Host ID                              Rack
UN 10.152.183.112 491.57 KB 256    ?    e7478c73-07a9-4fb2-a435-6603ccc9e6bd examplerack
DN 10.152.183.214 466.12 KB 256    ?    09d815de-6f6d-4394-8439-bd8d34231835 examplerack
UN 10.152.183.43  456.25 KB 256    ?    ac4e578d-cc82-4b71-9ba1-0f40aede9e8d examplerack
```

#### Pause the components that might interfere with the procedure

This assumes that the failed node's name is `scylla-exampledc-examplerack-1` in the namespace `examplens`

Stop the Operator (keep the ScyllaDB instances running, but prevent reconciliation). In a future improvement we can add a field to `ScyllaCluster` and/or `ScyllaDBDatacenter` to prevent Operator from "seeing" it, instead of scaling down to 0.

```console
$ # Before performing this, take note of the number of replicas in the deployment before this operation (typically 2).
$ # You will need to scale back to that number later.
$ kubectl scale -n scylla-operator deploy/scylla-operator --replicas=0 --timeout=5m
```
Prevent the StatefulSet from recreating the pod instantly. We achieve this by orphan deleting the StatefulSet for that specific rack,
because Operator will recreate the StatefulSet in its exact form when Operator is scaled back up at the end of the procedure.

```console
$ kubectl delete statefulset -n examplens scylla-exampledc-examplerack --cascade=orphan
```

#### Obtain the Host ID of the culprit node, and the Host IDs of potential ghost nodes.

Follow the [_Step One: Determining Host IDs of Ghost Members_](https://docs.scylladb.com/manual/branch-2025.1/operating-scylla/procedures/cluster-management/handling-membership-change-failures.html#step-one-determining-host-ids-of-ghost-members) guide and note down the Host IDs of any potential _ghost nodes_.

#### Stop the culprit node

**WARNING**: This deletes the node's data.

```console
$ # Note: This command will put the PVC in the "Terminating" state and block until the pod gets deleted by the later step.
$ kubectl delete persistentvolumeclaim -n scylla scylla-exampledc-examplerack-1

$ # Stop the node that is failing to join the cluster.
$ kubectl delete pod -n examplens scylla-exampledc-examplerack-1

$ # Delete the service associated with the node.
$ # Operator interprets this as a need to provision a brand new node instead of attempting to replace the old one in the rack.
$ kubectl delete service -n examplens scylla-exampledc-examplerack-1
```

#### Remove the culprit node and any possible _ghost nodes_ from the ScyllaDB cluster

Follow the [_Step Two: Removing the Ghost Members_](https://docs.scylladb.com/manual/branch-2025.1/operating-scylla/procedures/cluster-management/handling-membership-change-failures.html#step-two-removing-the-ghost-members) part of the failed membership change guide to `nodetool removenode` the node that failed to join the cluster and any _ghost members_.

Repeat as necessary:

```console
$ kubectl exec -n examplens scylla-exampledc-somehealthynode-5 -- nodetool removenode HOST_ID_OF_THE_NODE_TO_REMOVE
```

Verify that the culprit node and/or the ghost nodes are no longer present in `nodetool status`:

```console
$ kubectl exec -n examplens scylla-exampledc-somehealthynode-5 -- nodetool status

Datacenter: exampledc
=====================
Status=Up/Down
|/ State=Normal/Leaving/Joining/Moving
-- Address        Load      Tokens Owns Host ID                              Rack
UN 10.152.183.112 491.57 KB 256    ?    e7478c73-07a9-4fb2-a435-6603ccc9e6bd examplerack
UN 10.152.183.43  456.25 KB 256    ?    ac4e578d-cc82-4b71-9ba1-0f40aede9e8d examplerack
```

#### Resume Operator and let it heal the cluster

Resume Operator by scaling back to the original number of replicas.

```console
$ # Replace NNN with the number of replicas from before the initial scale-down-to-0.
$ # That number is typically 2.
$ kubectl scale -n scylla-operator deploy/scylla-operator --replicas=NNN --timeout=5m
```

#### Verify healing results

Sit back and see Operator:
- recreate the (identical) `StatefulSet` for the rack under repair (`scylla-exampledc-examplerack`),
- recreate the (identical) `Service` for the node that failed to replace previously (`scylla-exampledc-examplerack-1`),
- create new `Pod` and `PersistentVolumeClaim` for a net new node in place of the culprit node (each named `scylla-exampledc-examplerack-1`),

After that happens, run `nodetool status` to see that the new node has joined the cluster and is `UN`:

```console
$ kubectl exec -n examplens scylla-exampledc-somehealthynode-5 -- nodetool status

Datacenter: exampledc
=====================
Status=Up/Down
|/ State=Normal/Leaving/Joining/Moving
-- Address        Load      Tokens Owns Host ID                              Rack
UN 10.152.183.112 491.57 KB 256    ?    e7478c73-07a9-4fb2-a435-6603ccc9e6bd examplerack
UN 10.152.183.234 493.66 KB 256    ?    NEW-UUID-DIFFERENT-THAN-BEFORE-abcde examplerack
UN 10.152.183.43  456.25 KB 256    ?    ac4e578d-cc82-4b71-9ba1-0f40aede9e8d examplerack
```

### Notes/Constraints/Caveats [Optional]

Question to reviewers.
- Should we make this tutorial target other use cases as well (failed bootstrap in general?)
- Can we expect in the general case that this will guarantee a return of the data integrity conditions one would expect from a successful node replace?

### Risks and Mitigations

## Alternatives

- Implement a "freeze" switch in Operator to mitigate the need to scale it down to zero.
- Implement this sequence of operations in Operator directly.

