# Support for ScyllaDB Alternator

## Summary

This proposal aims to introduce basic support for handling metadata in managed objects by ScyllaClusters.
It aims to make minimal changes to satisfy the use cases presented so far.

## Motivation

ScyllaCluster users need to specify labels and annotations for various reasons including external integrations or object discovery / tracking.

### Goals

- Inherit ScyllaCluster labels and annotation into managed objects (aka default labels)
- Allow specifying explicit labels and annotations for node services
- Allow specifying explicit labels and annotations for pods
- Allow specifying explicit labels and annotations for initial persistent volume claims
- Allow specifying explicit labels and annotations for ingresses

### Non-Goals

- Allow unique metadata for every single object we create 

## Proposal

I'd like to use the existing ScyllaCluster.metadata (labels and annotations) and inherit them in every object we create,
unless there is an override.
In the case when using an override, the ScyllaCluster.metadata won't be used.

I propose to extend the ScyllaCluster API to allow overriding metadata for pods, node services and ingresses.

In case there is a conflict with managed metadata, the managed metadata will win.

These changes are purely intended for metadata and shall not change the selectors used.

### User Stories

#### DataDog metrics
As a user I want to configure [DataDog metrics](https://docs.datadoghq.com/containers/kubernetes/prometheus/?tab=kubernetesadv2#setup) for certain pods.
This is done by adding `ad.datadoghq.com/<CONTAINER_IDENTIFIER>.checks` to Pod's annotations.

#### Third Party Service and Ingress options
As a user I want to configure additional options for my service or ingress.
These third party options are usually controlled by adding annotations or labels, e.g.:
- [`networking.gke.io/load-balancer-type: "Internal"` annotation for GKE](https://cloud.google.com/kubernetes-engine/docs/how-to/internal-load-balancing)
- [`service.beta.kubernetes.io/aws-load-balancer-eip-allocations: eipalloc-xxxxxxxxxxxxxxxxx,eipalloc-yyyyyyyyyyyyyyyyy` annotation for assigning elastic IPs](https://docs.aws.amazon.com/eks/latest/userguide/network-load-balancing.html)
- [Ingress Controller sharding using labels and selectors in OCP](https://docs.openshift.com/container-platform/4.14/networking/ingress-sharding.html)

#### Cost tracking
As a user I want to add a label to all of my resources and use it to track costs.

#### PagerDuty
As a user I want to add extra label to my workload to route dynamic PagerDuty alerts.

#### PVC properties
As a user I want to add extra annotations to configure additional properties for PVCs, like "ebs.csi.aws.com/iops" or "ebs.csi.aws.com/throughput".

### Risks and Mitigations

Not known.

## Design Details

### API changes for ScyllaClusters.scylla.scylladb.com/v1

The following API changes show how we'd approximately extend `scyllaclusters.scylla.scylladb.com/v1.spec` specification. Only new fields are shown for existing structs.

```golang
package v1

type ObjectTemplateMetadata struct {
	// labels is a custom key value map that gets merged with managed object annotations.
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// annotations is a custom key value map that gets merged with managed object annotations.
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
}

type ScyllaClusterSpec struct {
	// podMetadata controls shared metadata for all pods created based on this spec.
	// +optional
	PodMetadata *ObjectTemplateMetadata `json:"podMetadata,omitempty"`

	// ...
}

type RackSpec struct {
	// [ReviewComment] Renames the type from StorageSpec to Storage
	Storage Storage `json:"storage"`
}

type Storage struct {
	// metadata controls shared metadata for the volume claim for this rack.
	// At this point, the values are applied only for the initial claim and is not reconciled during its lifetime.
	// Note that this may get fixed in the future and this behaviour shouldn't be relied on in any way.
	// +optional
	Metadata *ObjectTemplateMetadata `json:"metadata,omitempty"`

	// ...
}

type IngressOptions struct {
	// ObjectTemplateMetadata "replaces" / embeds previous field `annotations`
	ObjectTemplateMetadata `json:",inline"`
	
	// ...
}


type NodeServiceTemplate struct {
    // ObjectTemplateMetadata "replaces" / embeds previous field `annotations`
	ObjectTemplateMetadata `json:",inline"`

	// ...
}
```

### Test Plan

Existing unit tests will be extended to cover the new cases.

### Upgrade / Downgrade Strategy

No specific action for upgrades / downgrades is needed.

### Version Skew Strategy

All new fields are optional. New operator seeing older CRD will ignore the new empty fields. Previous version will ignore the new fields set on the CRD.

## Implementation History

- 2023-12-28: Initial enhancement proposal

## Drawbacks

Not known.

## Alternatives

- Something quite more complex / with less type enforcement.

- Metadata API filed for every object. That would likely be a nightmare to maintain 
  and locked our implementation freedom to using certain APIs.  
