# Extend NodeConfig with kernel parameter configuration (sysctls)

## Summary

This enhancement proposal aims to extend ScyllaDB Operator's node tuning capabilities with the ability to configure kernel parameters (sysctls) for ScyllaDB nodes.

## Motivation

ScyllaDB nodes require kernel parameter configuration for optimal performance and stability. 
It is currently possible to configure these parameters through `ScyllaCluster.scylla.scylladb.com/v1.spec.sysctls`.
However, when designing `ScyllaDBDatacenter.scylla.scylladb.com/v1alpha1`, we moved away from this approach as it's conceptually incorrect to configure sysctls per ScyllaDB deployment, given a single Kubernetes node can be shared by multiple ScyllaDB nodes in multi-tenant scenarios.
The kernel parameters are node-wide, some of them non-namespaced, and they should be configured per node, not per ScyllaDB deployment.

### Goals

- Allow for configuration of kernel parameters (sysctls) on a per-node basis for selected Kubernetes nodes.
- Allow for configuration of node-level (non-namespaced) sysctls.
- Allow for configuration of node-level sysctls for users of `ScyllaDBDatacenter.scylla.scylladb.com/v1alpha1` and `ScyllaDBCluster.scylla.scylladb.com/v1alpha1` APIs.

### Non-Goals

- Allow for independent configuration of namespaced sysctls per ScyllaDB Pod.

## Proposal

I propose to extend the ScyllaDB Operator's `NodeConfig.scylla.scylladb.com/v1alpha1` API to allow users to specify kernel parameters (sysctls) that should be configured in Kubernetes nodes matching the specified placement rules.
Sysctls will be configured by a dedicated Job, created by the existing NodeTune controller, running as part of the existing node setup DaemonSet.

### User Stories

#### Configuring sysctls in Kubernetes nodes
As a user, I want to configure kernel parameters sysctls on Kubernetes nodes that run ScyllaDB deployments, so that I can ensure optimal performance and stability of ScyllaDB nodes.
To do that, I create a NodeConfig object with placement rules that match the nodes where ScyllaDB deployments are running, and specify the sysctls to be configured.
In case a Kubernetes node is shared by multiple ScyllaDB nodes, I adjust the sysctl values to account for the needs of all ScyllaDB nodes running on that node.

### Notes/Constraints/Caveats

#### Non-goal of configuration of namespaced sysctls per ScyllaDB Pod
This proposal does not introduce the ability to configure namespaced sysctls per ScyllaDB Pod.
This is considered a separate feature orthogonal to the subject of this proposal.

### Risks and Mitigations

No known risks.

## Design Details

### API changes for `NodeConfig.scylla.scylladb.com/v1alpha1`

The NodeConfig API will be extended to include a new field specifying a list of sysctls to be configured on the nodes matching the placement rules.

```golang
type NodeConfigSpec struct {
    ...
	
    // sysctls specifies a list of sysctls to configure on the node.
    // +optional
    Sysctls []corev1.Sysctl `json:"sysctls,omitempty"`
}
```

#### `.spec.disableOptmizations` and `.spec.sysctls`

The existing `.spec.disableOptimizations` field will not affect the configuration of specified sysctls.
The sysctls specified in `.spec.sysctls` will always be configured, regardless of the value of `.spec.disableOptimizations`.
To minimize the risk of confusion, the `.spec.disableOptmizations` comment will be adjusted to clarify that it only affects performance tuning.

To disable the configuration of sysctls, the user should not specify the `.spec.sysctls` field.
However, removing values from `.spec.sysctls` will not revert the earlier configured sysctls to their initial values.

#### NodeConfig Status

The semantics of the existing NodeConfig status' `TunedNode` field will be extended to also take into account whether the specified sysctls were successfully configured on all matching nodes.

Sysctl configuration will also be reflected in NodeConfig's status conditions using the existing job controller's conditions, similar to how the existing node tuning is reported by the controller.

### NodeTune controller

The existing NodeTune controller, running as part of the node setup DaemonSet, will be extended to handle the sysctls configuration.
The controller will create dedicated Jobs configuring sysctls on the nodes matching the placement rules specified in the NodeConfig.

### ScyllaDB recommended sysctls

ScyllaDB Operator will not configure any sysctls by default, excluding any sysctls independently configured by the perftune script.
Users are expected to configure sysctls based on their specific use cases and requirements, including the number of ScyllaDB nodes running on Kubernetes nodes matched by NodeConfig's placement rules.
However, the ScyllaDB Operator documentation will provide the recommended sysctls for nodes running ScyllaDB Pods, as specified in the ScyllaDB's GitHub repository: https://github.com/scylladb/scylladb/tree/7bb43d812e1c512c92541a93603fcea0499e6d05/dist/common/sysctl.d.
In particular, the documentation will highlight which sysctls should be set to specific values and which should be adjusted to the number of ScyllaDB nodes running on the same Kubernetes node.
There is no reasonable way to automate the suggested configuration, so the documentation will be updated on a best-effort basis to reflect the latest recommendations from ScyllaDB repository.

### `ScyllaCluster.scylla.scylladb.com/v1.spec.sysctls` deprecation

The existing sysctl configuration option in `ScyllaCluster.scylla.scylladb.com/v1` will be marked as deprecated in the API description.
Users will be advised to remove the deprecated `.spec.sysctls` field from their `ScyllaCluster.scylla.scylladb.com/v1` resources before using the new `NodeConfig.scylla.scylladb.com/v1alpha1.spec.sysctls`, as otherwise the two might result in performing conflicting actions.
The support for the deprecated `.spec.sysctls` field should not be removed until NodeConfig's promotion to a stable version.

Additionally, the admission webhook will be extended to return a warning message when the deprecated `.spec.sysctls` field is specified in `ScyllaCluster.scylla.scylladb.com/v1` resources.

### Test Plan

Validation of the sysctl configuration and controller logic will be covered by unit tests.
An E2E test will be added to verify that the sysctls specified in the NodeConfig are correctly configured on the nodes matching the placement rules.

### Upgrade / Downgrade Strategy

No specific upgrade or downgrade strategy is required.

### Version Skew Strategy

No specific version skew strategy is required.
The introduced API extension is an optional field. The new operator version will ignore empty fields in the unupdated CRs. The old operator version will ignore the new fields set on updated CRs.

## Implementation History

- 2025-08-12: Enhancement proposal introduced.

## Drawbacks

No known drawbacks.

## Alternatives

### Default sysctl configuration for ScyllaDB nodes
As an alternative to documenting the recommended sysctls would be for ScyllaDB Operator be to configure a predefined set of sysctls for ScyllaDB nodes by default, based on the number of ScyllaDB Pods run on a Kubernetes node, without requiring users to specify them in the NodeConfig.
However, I consider this approach to be difficult to maintain and too rigid to accommodate different use cases and requirements.

### Specifying a multiplier for selected sysctls
As an alternative to advising users to adjust the values of selected sysctls based on the number of ScyllaDB Pods running on a Kubernetes node, we could allow users to specify a multiplier alongside the specified sysctls.
The multiplier would affect a predefined set of sysctls, which we would consider as necessary to scale according to the number of ScyllaDB Pods.
However, I consider this approach to be confusing and difficult to maintain and document, as it would require us to track which sysctls should be scaled and update it in the code paths, instead of just the documentation.
I consider the selected approach of documenting the recommended sysctls to be more straightforward and easier to adjust to different use cases and requirements.
