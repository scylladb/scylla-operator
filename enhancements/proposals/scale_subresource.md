# Scale subresource for ScyllaCluster

## Summary

This proposal describes an enhancement of Scylla Operator enabling the scale subresource for ScyllaCluster custom resource.

## Motivation

When the scale subresource is enabled, the `/scale` subresource for the custom resource is exposed.
It allows for using `kubectl scale` to scale the resource. Furthermore, it is required by [Horizontal Pod Autoscaler](https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/) and [Vertical Pod Autoscaler](https://github.com/kubernetes/autoscaler/tree/master/vertical-pod-autoscaler) to operate on the custom resource.

### Goals

- Enabling the scale subresource in ScyllaCluster CustomResourceDefinition.
- Providing a way to scale all Racks simultaneously, with each Rack's Member count set to an equal value.

### Non-Goals

- Providing a way to scale each Rack separately.
- Providing support for autoscaling. Enabling the scale subresource is required, but not sufficient to support autoscaling of ScyllaClusters.

## Proposal

Since ScyllaCluster is a CustomResource and not a simple Kubernetes collection, to support scaling, it needs to have the scale subresource enabled.

Because ScyllaCluster does not have one, but multiple underlying StatefulSets (corresponding to Racks), it requires a way of overriding the existing way of specifying the number of replicas.
To do so, a new mode is introduced, in which the number of replicas in each StatefulSet is equal.

### User Stories

#### Scaling a ScyllaCluster

A user runs a ScyllaCluster with multiple Racks, each with the same number of Members. The user wants to scale all Racks to `n` Members.
The user is able to do so by modifying the Replicas field, e.g. by running `kubectl scale ScyllaCluster <cluster-name> --replicas=n`. 

### Notes/Constraints/Caveats [Optional]

None.

### Risks and Mitigations

None.

## Design Details

### ScyllaCluster CRD

To enable the scale subresource, `ScyllaCluster` CustomResourceDefinition will be extended as follows:

```go
type ScyllaClusterSpec struct {
	...
	
	// +optional 
	Replicas *int32 `json:"replicas,omitempty"`
}
```

```go
type ScyllaClusterStatus struct {
	...

	// +optional
	Replicas *int32  `json:"replicas,omitempty"`
}
```

```go
...
// +kubebuilder:subresource:scale:specpath=.spec.replicas,statuspath=.status.replicas
...
type ScyllaCluster struct {
	...
}
```

Setting the Replicas field in ScyllaClusterSpec will enforce equal number of Scylla nodes in each Rack. The number should always match Replicas.
To do so, Scylla Operator is extended with a mutating webhook overriding the Member counts.
Without Replicas set, the number of Members in each Rack is determined as before, by specifying it in Rack's specification.

Replicas field in ScyllaCluster's status is updated only when every Racks' readyMembers field value is equal. Replicas is set to this value then.

### Test Plan

#### Mutating webhook
The mutating webhook propagating the Replicas value to each Rack's Member count is unit-tested.

#### E2E
##### ScyllaCluster simultaneous scaling
- Create a ScyllaCluster with multiple Racks and `n` replicas
- Wait for the ScyllaCluster to deploy
- Verify that each Rack has `n` Members
- Scale ScyllaCluster to `m` replicas
- Wait for the ScyllaCluster to reconcile and become fully available again
- Verify that each Rack has `m` Members

### Upgrade / Downgrade Strategy

No upgrade/downgrade strategy required.

### Version Skew Strategy

Simultaneous scaling will not work in case of a version skew between ScyllaCluster and Scylla Operator.

Outdated ScyllaOperator will simply ignore the Replicas field of an up-to-date ScyllaCluster.
Eventually, Scylla Operator will be updated and will support the Replicas field.

An outdated ScyllaCluster won't have a Replicas field, so it will simply define Members in each Rack.

## Implementation History

- 2021-08-12 Proposal created

## Drawbacks

None.

## Alternatives

### Mutating webhook
Instead of extending Scylla Operator with a mutating webhook propagating the Replicas value to Members in each Rack, we could make Members field optional and create additional validations in the admission webhook.
Validations would enforce that Replicas and Members fields are mutually exclusive and expect either the Replicas field or every Member field to be set. This behaviour could be more transparent, albeit it would require setting the Members explicitly when unsetting Replicas. 
It would also call for more modifications to the controller's code.

### Setting Replicas in status
Instead of only updating the Replicas field in status when ReadyMembers are equal among all racks, we could set upper/lower bound depending on whether we are scaling up or down.

### Setting selector for scale subresource
This will be considered in the future. It is not a part of this enhancement, as it is only meant to support scaling, for which the selector is not required.

## Infrastructure Needed [optional]

None.