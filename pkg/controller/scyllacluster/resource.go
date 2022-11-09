// Copyright (c) 2022 ScyllaDB.

package scyllacluster

import (
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	scyllav2alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v2alpha1"
	"github.com/scylladb/scylla-operator/pkg/naming"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func MakeScyllaDatacenters(sc *scyllav2alpha1.ScyllaCluster) map[string]*scyllav1alpha1.ScyllaDatacenter {
	remoteScyllaDatacenters := make(map[string]*scyllav1alpha1.ScyllaDatacenter)

	for _, dc := range sc.Spec.Datacenters {
		var ownerReferences []metav1.OwnerReference
		var namespace string
		var remoteName string

		if dc.RemoteKubeClusterConfigRef == nil {
			ownerReferences = []metav1.OwnerReference{
				*metav1.NewControllerRef(sc, scyllaClusterControllerGVK),
			}
			namespace = sc.Namespace
		} else {
			namespace = naming.RemoteNamespace(sc, dc)
			remoteName = dc.RemoteKubeClusterConfigRef.Name
		}

		var scyllaManagerAgent *scyllav1alpha1.ScyllaManagerAgent
		if sc.Spec.ScyllaManagerAgent != nil {
			scyllaManagerAgent = &scyllav1alpha1.ScyllaManagerAgent{
				Image: sc.Spec.ScyllaManagerAgent.Image,
			}
		}

		remoteScyllaDatacenters[remoteName] = &scyllav1alpha1.ScyllaDatacenter{
			ObjectMeta: metav1.ObjectMeta{
				Name:      sc.Name,
				Namespace: namespace,
				Labels: map[string]string{
					naming.ParentClusterNameLabel:           sc.Name,
					naming.ParentClusterNamespaceLabel:      sc.Namespace,
					naming.ParentClusterDatacenterNameLabel: dc.Name,
				},
				OwnerReferences: ownerReferences,
			},
			Spec: scyllav1alpha1.ScyllaDatacenterSpec{
				Scylla: scyllav1alpha1.Scylla{
					Image: sc.Spec.Scylla.Image,
					AlternatorOptions: func() *scyllav1alpha1.AlternatorOptions {
						var alternatorOptions *scyllav1alpha1.AlternatorOptions
						if sc.Spec.Scylla.AlternatorOptions != nil {
							alternatorOptions = &scyllav1alpha1.AlternatorOptions{
								Enabled:        sc.Spec.Scylla.AlternatorOptions.Enabled,
								WriteIsolation: sc.Spec.Scylla.AlternatorOptions.WriteIsolation,
							}
						}
						return alternatorOptions
					}(),
					UnsupportedScyllaArgsOverrides: sc.Spec.Scylla.UnsupportedScyllaArgs,
					EnableDeveloperMode:            sc.Spec.Scylla.EnableDeveloperMode,
				},
				ScyllaManagerAgent: scyllaManagerAgent,
				NodesPerRack:       dc.NodesPerRack,
				DatacenterName:     dc.Name,
				DNSDomains:         dc.DNSDomains,
				ExposeOptions: func() *scyllav1alpha1.ExposeOptions {
					var exposeOptions *scyllav1alpha1.ExposeOptions
					if dc.ExposeOptions != nil {
						exposeOptions = &scyllav1alpha1.ExposeOptions{}
						if dc.ExposeOptions.CQL != nil {
							exposeOptions.CQL = &scyllav1alpha1.CQLExposeOptions{}
							if dc.ExposeOptions.CQL.Ingress != nil {
								exposeOptions.CQL.Ingress = &scyllav1alpha1.IngressOptions{
									Disabled:         dc.ExposeOptions.CQL.Ingress.Disabled,
									IngressClassName: dc.ExposeOptions.CQL.Ingress.IngressClassName,
									Annotations:      dc.ExposeOptions.CQL.Ingress.Annotations,
								}
							}
						}
					}
					return exposeOptions
				}(),
				Racks: func() []scyllav1alpha1.RackSpec {
					var racks []scyllav1alpha1.RackSpec
					for _, r := range dc.Racks {
						var placement *scyllav1alpha1.Placement
						rackPlacement := override(dc.Placement, r.Placement)
						if rackPlacement != nil {
							placement = &scyllav1alpha1.Placement{
								NodeAffinity:    rackPlacement.NodeAffinity,
								PodAffinity:     rackPlacement.PodAffinity,
								PodAntiAffinity: rackPlacement.PodAntiAffinity,
								Tolerations:     rackPlacement.Tolerations,
							}
						}

						scyllaOverrides := &scyllav1alpha1.ScyllaOverrides{
							Resources: dc.Scylla.Resources,
							Storage: func() *scyllav1alpha1.Storage {
								var storage *scyllav1alpha1.Storage
								if dc.Scylla.Storage != nil {
									storage = &scyllav1alpha1.Storage{
										Resources:        dc.Scylla.Storage.Resources,
										StorageClassName: dc.Scylla.Storage.StorageClassName,
									}
								}
								return storage
							}(),
							CustomConfigMapRef: dc.Scylla.CustomConfigMapRef,
						}

						var agentOverrides *scyllav1alpha1.ScyllaManagerAgentOverrides
						if dc.ScyllaManagerAgent != nil {
							agentOverrides = &scyllav1alpha1.ScyllaManagerAgentOverrides{
								Resources:             dc.ScyllaManagerAgent.Resources,
								CustomConfigSecretRef: dc.ScyllaManagerAgent.CustomConfigSecretRef,
							}
						}

						racks = append(racks, scyllav1alpha1.RackSpec{
							Name:               r.Name,
							Nodes:              dc.NodesPerRack,
							Scylla:             scyllaOverrides,
							ScyllaManagerAgent: agentOverrides,
							Placement:          placement,
						})
					}

					return racks
				}(),
				ForceRedeploymentReason: func() string {
					return sc.Spec.ForceRedeploymentReason + dc.ForceRedeploymentReason
				}(),
				ImagePullSecrets: sc.Spec.ImagePullSecrets,
				Network: func() *scyllav1alpha1.Network {
					var network *scyllav1alpha1.Network
					if sc.Spec.Network != nil {
						network = &scyllav1alpha1.Network{
							DNSPolicy: sc.Spec.Network.DNSPolicy,
						}
					}
					return network
				}(),
				Placement: func() *scyllav1alpha1.Placement {
					var placement *scyllav1alpha1.Placement
					if dc.Placement != nil {
						placement = &scyllav1alpha1.Placement{
							NodeAffinity:    dc.Placement.NodeAffinity,
							PodAffinity:     dc.Placement.PodAffinity,
							PodAntiAffinity: dc.Placement.PodAntiAffinity,
							Tolerations:     dc.Placement.Tolerations,
						}
					}
					return placement
				}(),
			},
		}
	}

	if v, ok := sc.Annotations[naming.ScyllaClusterV1Annotation]; ok {
		var remoteName string
		if sc.Spec.Datacenters[0].RemoteKubeClusterConfigRef != nil {
			remoteName = sc.Spec.Datacenters[0].RemoteKubeClusterConfigRef.Name
		}
		if remoteScyllaDatacenters[remoteName].Annotations == nil {
			remoteScyllaDatacenters[remoteName].Annotations = map[string]string{}
		}
		remoteScyllaDatacenters[remoteName].Annotations[naming.ScyllaClusterV1Annotation] = v
	}

	return remoteScyllaDatacenters
}

func MakeNamespaces(sc *scyllav2alpha1.ScyllaCluster) map[string]*corev1.Namespace {
	requiredNamespaces := make(map[string]*corev1.Namespace, len(sc.Spec.Datacenters))

	for _, dc := range sc.Spec.Datacenters {
		// Do not manage namespaces in local cluster.
		if dc.RemoteKubeClusterConfigRef == nil {
			continue
		}

		remoteNamespace := naming.RemoteNamespace(sc, dc)
		requiredNamespaces[dc.RemoteKubeClusterConfigRef.Name] = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: remoteNamespace,
				Labels: map[string]string{
					naming.ParentClusterNameLabel:           sc.Name,
					naming.ParentClusterNamespaceLabel:      sc.Namespace,
					naming.ParentClusterDatacenterNameLabel: dc.Name,
				},
			},
		}
	}

	return requiredNamespaces
}

// TODO: change to merge
func override[T any](a, b *T) *T {
	if b != nil {
		return b
	}
	return a
}
