// Copyright (c) 2024 ScyllaDB.

package controllerhelpers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"strconv"
	"strings"

	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/helpers/slices"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/errors"
)

func ConvertV1Alpha1ScyllaDBDatacenterToV1ScyllaCluster(sdc *scyllav1alpha1.ScyllaDBDatacenter) (*scyllav1.ScyllaCluster, error) {
	scyllaRepository, scyllaVersion, err := naming.ImageToRepositoryVersion(sdc.Spec.ScyllaDB.Image)
	if err != nil {
		return nil, fmt.Errorf("can't extract repository and version from ScyllaDB image %q: %w", sdc.Spec.ScyllaDB.Image, err)
	}
	var agentRepository, agentVersion string
	if sdc.Spec.ScyllaDBManagerAgent != nil {
		agentRepository, agentVersion, err = naming.ImageToRepositoryVersion(*sdc.Spec.ScyllaDBManagerAgent.Image)
		if err != nil {
			return nil, fmt.Errorf("can't extract repository and version from ScyllaDB image %q: %w", *sdc.Spec.ScyllaDBManagerAgent.Image, err)
		}
	}

	var convertErrs []error
	return &scyllav1.ScyllaCluster{
		ObjectMeta: sdc.ObjectMeta,
		Spec: scyllav1.ScyllaClusterSpec{
			PodMetadata: &scyllav1.ObjectTemplateMetadata{
				Labels:      sdc.Spec.Metadata.Labels,
				Annotations: sdc.Spec.Metadata.Annotations,
			},
			Version:    scyllaVersion,
			Repository: scyllaRepository,
			Alternator: func() *scyllav1.AlternatorSpec {
				if sdc.Spec.ScyllaDB.AlternatorOptions == nil {
					return nil
				}
				return &scyllav1.AlternatorSpec{
					Port: func() int32 {
						alternatorPortAnnotation, ok := sdc.Annotations[naming.TransformScyllaClusterToScyllaDBDatacenterAlternatorPortAnnotation]
						if !ok {
							return 0
						}

						port, err := strconv.Atoi(alternatorPortAnnotation)
						if err != nil {
							convertErrs = append(convertErrs, fmt.Errorf("can't parse alternator port annotation %q: %w", alternatorPortAnnotation, err))
							return 0
						}

						return int32(port)
					}(),
					InsecureEnableHTTP: func() *bool {
						insecureEnableHTTPAnnotation, ok := sdc.Annotations[naming.TransformScyllaClusterToScyllaDBDatacenterInsecureEnableHTTPAnnotation]
						if !ok {
							return nil
						}

						insecureEnableHTTP, err := strconv.ParseBool(insecureEnableHTTPAnnotation)
						if err != nil {
							convertErrs = append(convertErrs, fmt.Errorf("can't parse alternator insecure enable http annotation %q: %w", insecureEnableHTTPAnnotation, err))
							return nil
						}

						return pointer.Ptr(insecureEnableHTTP)
					}(),
					InsecureDisableAuthorization: func() *bool {
						insecureDisableAuthorizationAnnotation, ok := sdc.Annotations[naming.TransformScyllaClusterToScyllaDBDatacenterInsecureDisableAuthorizationAnnotation]
						if !ok {
							return nil
						}

						insecureDisableAuthorization, err := strconv.ParseBool(insecureDisableAuthorizationAnnotation)
						if err != nil {
							convertErrs = append(convertErrs, fmt.Errorf("can't parse alternator insecure disable authorization annotation %q: %w", insecureDisableAuthorizationAnnotation, err))
							return nil
						}

						return pointer.Ptr(insecureDisableAuthorization)
					}(),
					WriteIsolation: sdc.Spec.ScyllaDB.AlternatorOptions.WriteIsolation,
					ServingCertificate: func() *scyllav1.TLSCertificate {
						if sdc.Spec.ScyllaDB.AlternatorOptions.ServingCertificate == nil {
							return nil
						}
						return &scyllav1.TLSCertificate{
							Type: scyllav1.TLSCertificateType(sdc.Spec.ScyllaDB.AlternatorOptions.ServingCertificate.Type),
							UserManagedOptions: func() *scyllav1.UserManagedTLSCertificateOptions {
								if sdc.Spec.ScyllaDB.AlternatorOptions.ServingCertificate.UserManagedOptions == nil {
									return nil
								}
								return &scyllav1.UserManagedTLSCertificateOptions{
									SecretName: sdc.Spec.ScyllaDB.AlternatorOptions.ServingCertificate.UserManagedOptions.SecretName,
								}
							}(),
							OperatorManagedOptions: func() *scyllav1.OperatorManagedTLSCertificateOptions {
								if sdc.Spec.ScyllaDB.AlternatorOptions.ServingCertificate.OperatorManagedOptions == nil {
									return nil
								}
								return &scyllav1.OperatorManagedTLSCertificateOptions{
									AdditionalDNSNames:    sdc.Spec.ScyllaDB.AlternatorOptions.ServingCertificate.OperatorManagedOptions.AdditionalDNSNames,
									AdditionalIPAddresses: sdc.Spec.ScyllaDB.AlternatorOptions.ServingCertificate.OperatorManagedOptions.AdditionalIPAddresses,
								}
							}(),
						}
					}(),
				}
			}(),
			AgentVersion:    agentVersion,
			AgentRepository: agentRepository,
			DeveloperMode: func() bool {
				if sdc.Spec.ScyllaDB.EnableDeveloperMode == nil {
					return false
				}
				return *sdc.Spec.ScyllaDB.EnableDeveloperMode
			}(),
			CpuSet:                       false,
			AutomaticOrphanedNodeCleanup: !sdc.Spec.DisableAutomaticOrphanedNodeReplacement,
			Datacenter: scyllav1.DatacenterSpec{
				Name: sdc.Spec.DatacenterName,
				Racks: func() []scyllav1.RackSpec {
					var racks []scyllav1.RackSpec
					for _, rack := range sdc.Spec.Racks {
						racks = append(racks, scyllav1.RackSpec{
							Name: rack.Name,
							Members: func() int32 {
								if rack.Nodes == nil {
									return 0
								}
								return *rack.Nodes
							}(),
							Placement: func() *scyllav1.PlacementSpec {
								if rack.Placement == nil {
									return nil
								}
								return &scyllav1.PlacementSpec{
									NodeAffinity:    rack.Placement.NodeAffinity,
									PodAffinity:     rack.Placement.PodAffinity,
									PodAntiAffinity: rack.Placement.PodAntiAffinity,
									Tolerations:     rack.Placement.Tolerations,
								}
							}(),
							Resources: *rack.ScyllaDB.Resources,
							Storage: scyllav1.Storage{
								Metadata: &scyllav1.ObjectTemplateMetadata{
									Labels:      rack.ScyllaDB.Storage.Metadata.Labels,
									Annotations: rack.ScyllaDB.Storage.Metadata.Annotations,
								},
								Capacity:         rack.ScyllaDB.Storage.Capacity,
								StorageClassName: rack.ScyllaDB.Storage.StorageClassName,
							},
							ScyllaConfig: func() string {
								if rack.ScyllaDB.CustomConfigMapRef == nil {
									return ""
								}
								return *rack.ScyllaDB.CustomConfigMapRef
							}(),
							Volumes: func() []corev1.Volume {
								var volumes []corev1.Volume

								if rack.ScyllaDB != nil {
									volumes = append(volumes, rack.ScyllaDB.Volumes...)
								}
								if rack.ScyllaDBManagerAgent != nil {
									volumes = append(volumes, rack.ScyllaDBManagerAgent.Volumes...)
								}

								return volumes
							}(),
							VolumeMounts:   rack.ScyllaDB.VolumeMounts,
							AgentResources: *rack.ScyllaDBManagerAgent.Resources,
							ScyllaAgentConfig: func() string {
								if rack.ScyllaDBManagerAgent.CustomConfigSecretRef == nil {
									return ""
								}
								return *rack.ScyllaDBManagerAgent.CustomConfigSecretRef
							}(),
							AgentVolumeMounts: rack.ScyllaDBManagerAgent.VolumeMounts,
						})
					}
					return racks
				}(),
			},
			Sysctls: func() []string {
				var sysctls []string

				sysctlsAnnotation, ok := sdc.Annotations[naming.TransformScyllaClusterToScyllaDBDatacenterSysctlsAnnotation]
				if ok {
					err := json.NewDecoder(strings.NewReader(sysctlsAnnotation)).Decode(&sysctls)
					if err != nil {
						convertErrs = append(convertErrs, fmt.Errorf("can't decode sysctls annotation: %w", err))
						return nil
					}
				}
				return sysctls
			}(),
			ScyllaArgs: strings.Join(sdc.Spec.ScyllaDB.AdditionalScyllaDBArguments, " "),
			Network: scyllav1.Network{
				HostNetworking: func() bool {
					hostNetworkingAnnotation, ok := sdc.Annotations[naming.TransformScyllaClusterToScyllaDBDatacenterHostNetworkingAnnotation]
					if !ok {
						return false
					}
					return hostNetworkingAnnotation == "true"
				}(),
				DNSPolicy: func() corev1.DNSPolicy {
					if sdc.Spec.DNSPolicy == nil {
						return corev1.DNSClusterFirstWithHostNet
					}
					return *sdc.Spec.DNSPolicy
				}(),
			},
			ExposeOptions: func() *scyllav1.ExposeOptions {
				if sdc.Spec.ExposeOptions == nil {
					return nil
				}
				return &scyllav1.ExposeOptions{
					CQL: func() *scyllav1.CQLExposeOptions {
						if sdc.Spec.ExposeOptions.CQL == nil {
							return nil
						}
						return &scyllav1.CQLExposeOptions{
							Ingress: func() *scyllav1.IngressOptions {
								if sdc.Spec.ExposeOptions.CQL.Ingress == nil {
									return nil
								}
								return &scyllav1.IngressOptions{
									ObjectTemplateMetadata: scyllav1.ObjectTemplateMetadata{
										Labels:      sdc.Spec.ExposeOptions.CQL.Ingress.Labels,
										Annotations: sdc.Spec.ExposeOptions.CQL.Ingress.Annotations,
									},
									Disabled:         pointer.Ptr(false),
									IngressClassName: sdc.Spec.ExposeOptions.CQL.Ingress.IngressClassName,
								}
							}(),
						}
					}(),
					NodeService: func() *scyllav1.NodeServiceTemplate {
						if sdc.Spec.ExposeOptions.NodeService == nil {
							return nil
						}
						return &scyllav1.NodeServiceTemplate{
							ObjectTemplateMetadata: scyllav1.ObjectTemplateMetadata{
								Labels:      sdc.Spec.ExposeOptions.NodeService.Labels,
								Annotations: sdc.Spec.ExposeOptions.NodeService.Annotations,
							},
							Type:                          scyllav1.NodeServiceType(sdc.Spec.ExposeOptions.NodeService.Type),
							ExternalTrafficPolicy:         sdc.Spec.ExposeOptions.NodeService.ExternalTrafficPolicy,
							AllocateLoadBalancerNodePorts: sdc.Spec.ExposeOptions.NodeService.AllocateLoadBalancerNodePorts,
							LoadBalancerClass:             sdc.Spec.ExposeOptions.NodeService.LoadBalancerClass,
							InternalTrafficPolicy:         sdc.Spec.ExposeOptions.NodeService.InternalTrafficPolicy,
						}
					}(),
					BroadcastOptions: func() *scyllav1.NodeBroadcastOptions {
						if sdc.Spec.ExposeOptions.BroadcastOptions == nil {
							return nil
						}
						return &scyllav1.NodeBroadcastOptions{
							Nodes: scyllav1.BroadcastOptions{
								Type: scyllav1.BroadcastAddressType(sdc.Spec.ExposeOptions.BroadcastOptions.Nodes.Type),
								PodIP: func() *scyllav1.PodIPAddressOptions {
									if sdc.Spec.ExposeOptions.BroadcastOptions.Nodes.PodIP == nil {
										return nil
									}
									return &scyllav1.PodIPAddressOptions{
										Source: scyllav1.PodIPSourceType(sdc.Spec.ExposeOptions.BroadcastOptions.Nodes.PodIP.Source),
									}
								}(),
							},
							Clients: scyllav1.BroadcastOptions{
								Type: scyllav1.BroadcastAddressType(sdc.Spec.ExposeOptions.BroadcastOptions.Clients.Type),
								PodIP: func() *scyllav1.PodIPAddressOptions {
									if sdc.Spec.ExposeOptions.BroadcastOptions.Clients.PodIP == nil {
										return nil
									}
									return &scyllav1.PodIPAddressOptions{
										Source: scyllav1.PodIPSourceType(sdc.Spec.ExposeOptions.BroadcastOptions.Clients.PodIP.Source),
									}
								}(),
							},
						}
					}(),
				}
			}(),
			ForceRedeploymentReason:          sdc.Spec.ForceRedeploymentReason,
			ImagePullSecrets:                 sdc.Spec.ImagePullSecrets,
			DNSDomains:                       sdc.Spec.DNSDomains,
			ExternalSeeds:                    sdc.Spec.ScyllaDB.ExternalSeeds,
			MinTerminationGracePeriodSeconds: sdc.Spec.MinTerminationGracePeriodSeconds,
			MinReadySeconds:                  sdc.Spec.MinReadySeconds,
			ReadinessGates:                   sdc.Spec.ReadinessGates,
			Repairs:                          nil,
			Backups:                          nil,
			GenericUpgrade:                   nil,
		},
		Status: ConvertV1Alpha1ScyllaDBDatacenterStatusToV1ScyllaClusterStatus(sdc),
	}, errors.NewAggregate(convertErrs)
}

func ConvertV1Alpha1ScyllaDBDatacenterStatusToV1ScyllaClusterStatus(sdc *scyllav1alpha1.ScyllaDBDatacenter) scyllav1.ScyllaClusterStatus {
	return scyllav1.ScyllaClusterStatus{
		ObservedGeneration: sdc.Status.ObservedGeneration,
		Conditions:         sdc.Status.Conditions,
		Members:            sdc.Status.Nodes,
		ReadyMembers:       sdc.Status.ReadyNodes,
		AvailableMembers:   sdc.Status.AvailableNodes,
		RackCount:          pointer.Ptr(int32(len(sdc.Status.Racks))),
		Racks: func() map[string]scyllav1.RackStatus {
			rackStatuses := make(map[string]scyllav1.RackStatus)
			for _, rackStatus := range sdc.Status.Racks {
				rackStatuses[rackStatus.Name] = scyllav1.RackStatus{
					Version: rackStatus.CurrentVersion,
					Members: func() int32 {
						if rackStatus.Nodes != nil {
							return *rackStatus.Nodes
						}
						return 0
					}(),
					ReadyMembers: func() int32 {
						if rackStatus.ReadyNodes != nil {
							return *rackStatus.ReadyNodes
						}
						return 0
					}(),
					AvailableMembers: rackStatus.AvailableNodes,
					UpdatedMembers:   rackStatus.UpdatedNodes,
					Stale:            rackStatus.Stale,
					Conditions: func() []scyllav1.RackCondition {
						// TODO
						return nil
					}(),
					ReplaceAddressFirstBoot: nil,
				}
			}
			return rackStatuses
		}(),
		// TODO: rewrite from ConfigMap
		Upgrade:   nil,
		Repairs:   nil,
		Backups:   nil,
		ManagerID: nil,
	}
}

func ConvertV1ScyllaClusterToV1Alpha1ScyllaDBDatacenter(sc *scyllav1.ScyllaCluster) (*scyllav1alpha1.ScyllaDBDatacenter, error) {
	var convertErrs []error
	return &scyllav1alpha1.ScyllaDBDatacenter{
		ObjectMeta: func() metav1.ObjectMeta {
			objectMeta := metav1.ObjectMeta{
				Name:        sc.Name,
				Namespace:   sc.Namespace,
				Labels:      make(map[string]string),
				Annotations: make(map[string]string),
			}

			maps.Copy(objectMeta.Labels, sc.Labels)
			maps.Copy(objectMeta.Annotations, sc.Annotations)

			if sc.Spec.Network.HostNetworking {
				objectMeta.Annotations[naming.TransformScyllaClusterToScyllaDBDatacenterHostNetworkingAnnotation] = "true"
			}

			if len(sc.Spec.Sysctls) > 0 {
				buf := bytes.Buffer{}
				err := json.NewEncoder(&buf).Encode(sc.Spec.Sysctls)
				if err != nil {
					convertErrs = append(convertErrs, fmt.Errorf("can't encode sysctls: %w", err))
				}
				objectMeta.Annotations[naming.TransformScyllaClusterToScyllaDBDatacenterSysctlsAnnotation] = buf.String()
			}

			if sc.Spec.Alternator != nil {
				if sc.Spec.Alternator.Port != 0 {
					objectMeta.Annotations[naming.TransformScyllaClusterToScyllaDBDatacenterAlternatorPortAnnotation] = fmt.Sprintf("%d", sc.Spec.Alternator.Port)
				}

				if sc.Spec.Alternator.InsecureDisableAuthorization != nil {
					objectMeta.Annotations[naming.TransformScyllaClusterToScyllaDBDatacenterInsecureDisableAuthorizationAnnotation] = fmt.Sprintf("%v", *sc.Spec.Alternator.InsecureDisableAuthorization)
				}

				if sc.Spec.Alternator.InsecureEnableHTTP != nil {
					objectMeta.Annotations[naming.TransformScyllaClusterToScyllaDBDatacenterInsecureEnableHTTPAnnotation] = fmt.Sprintf("%v", *sc.Spec.Alternator.InsecureEnableHTTP)
				}
			}

			return objectMeta

		}(),
		Spec: scyllav1alpha1.ScyllaDBDatacenterSpec{
			Metadata: func() scyllav1alpha1.ObjectTemplateMetadata {
				if sc.Spec.PodMetadata == nil {
					return scyllav1alpha1.ObjectTemplateMetadata{}
				}
				return scyllav1alpha1.ObjectTemplateMetadata{
					Labels:      sc.Spec.PodMetadata.Labels,
					Annotations: sc.Spec.PodMetadata.Annotations,
				}
			}(),
			ClusterName:    sc.Name,
			DatacenterName: sc.Spec.Datacenter.Name,
			ScyllaDB: scyllav1alpha1.ScyllaDB{
				// v1.ScyllaCluster doesn't provide API to specify cluster/datacenter wide properties of scylladb.
				ScyllaDBTemplate: scyllav1alpha1.ScyllaDBTemplate{},
				Image:            fmt.Sprintf("%s:%s", sc.Spec.Repository, sc.Spec.Version),
				ExternalSeeds:    sc.Spec.ExternalSeeds,
				AlternatorOptions: func() *scyllav1alpha1.AlternatorOptions {
					if sc.Spec.Alternator == nil {
						return nil
					}

					return &scyllav1alpha1.AlternatorOptions{
						WriteIsolation: sc.Spec.Alternator.WriteIsolation,
						ServingCertificate: func() *scyllav1alpha1.TLSCertificate {
							if sc.Spec.Alternator.ServingCertificate == nil {
								return nil
							}
							return &scyllav1alpha1.TLSCertificate{
								Type: scyllav1alpha1.TLSCertificateType(sc.Spec.Alternator.ServingCertificate.Type),
								UserManagedOptions: func() *scyllav1alpha1.UserManagedTLSCertificateOptions {
									if sc.Spec.Alternator.ServingCertificate.UserManagedOptions == nil {
										return nil
									}
									return &scyllav1alpha1.UserManagedTLSCertificateOptions{
										SecretName: sc.Spec.Alternator.ServingCertificate.UserManagedOptions.SecretName,
									}
								}(),
								OperatorManagedOptions: func() *scyllav1alpha1.OperatorManagedTLSCertificateOptions {
									if sc.Spec.Alternator.ServingCertificate.OperatorManagedOptions == nil {
										return nil
									}
									return &scyllav1alpha1.OperatorManagedTLSCertificateOptions{
										AdditionalDNSNames:    sc.Spec.Alternator.ServingCertificate.OperatorManagedOptions.AdditionalDNSNames,
										AdditionalIPAddresses: sc.Spec.Alternator.ServingCertificate.OperatorManagedOptions.AdditionalIPAddresses,
									}
								}(),
							}
						}(),
					}
				}(),
				AdditionalScyllaDBArguments: strings.Split(sc.Spec.ScyllaArgs, " "),
				EnableDeveloperMode:         pointer.Ptr(sc.Spec.DeveloperMode),
			},
			ScyllaDBManagerAgent: &scyllav1alpha1.ScyllaDBManagerAgent{
				// v1.ScyllaCluster doesn't provide API to specify cluster/datacenter wide properties of scylladb manager agent.
				ScyllaDBManagerAgentTemplate: scyllav1alpha1.ScyllaDBManagerAgentTemplate{},
				Image:                        pointer.Ptr(fmt.Sprintf("%s:%s", sc.Spec.AgentRepository, sc.Spec.AgentVersion)),
			},
			ImagePullSecrets:        sc.Spec.ImagePullSecrets,
			DNSPolicy:               pointer.Ptr(sc.Spec.Network.GetDNSPolicy()),
			DNSDomains:              sc.Spec.DNSDomains,
			ForceRedeploymentReason: sc.Spec.ForceRedeploymentReason,
			ExposeOptions: func() *scyllav1alpha1.ExposeOptions {
				if sc.Spec.ExposeOptions == nil {
					return nil
				}

				return &scyllav1alpha1.ExposeOptions{
					CQL: func() *scyllav1alpha1.CQLExposeOptions {
						if sc.Spec.ExposeOptions.CQL == nil {
							return nil
						}
						return &scyllav1alpha1.CQLExposeOptions{
							Ingress: func() *scyllav1alpha1.CQLExposeIngressOptions {
								if sc.Spec.ExposeOptions.CQL.Ingress == nil {
									return nil
								}
								return &scyllav1alpha1.CQLExposeIngressOptions{
									ObjectTemplateMetadata: scyllav1alpha1.ObjectTemplateMetadata{
										Labels:      sc.Spec.ExposeOptions.CQL.Ingress.Labels,
										Annotations: sc.Spec.ExposeOptions.CQL.Ingress.Annotations,
									},
									IngressClassName: sc.Spec.ExposeOptions.CQL.Ingress.IngressClassName,
								}
							}(),
						}
					}(),
					NodeService: func() *scyllav1alpha1.NodeServiceTemplate {
						if sc.Spec.ExposeOptions.NodeService == nil {
							return nil
						}
						return &scyllav1alpha1.NodeServiceTemplate{
							ObjectTemplateMetadata: scyllav1alpha1.ObjectTemplateMetadata{
								Labels:      sc.Spec.ExposeOptions.NodeService.Labels,
								Annotations: sc.Spec.ExposeOptions.NodeService.Annotations,
							},
							Type:                          scyllav1alpha1.NodeServiceType(sc.Spec.ExposeOptions.NodeService.Type),
							ExternalTrafficPolicy:         sc.Spec.ExposeOptions.NodeService.ExternalTrafficPolicy,
							AllocateLoadBalancerNodePorts: sc.Spec.ExposeOptions.NodeService.AllocateLoadBalancerNodePorts,
							LoadBalancerClass:             sc.Spec.ExposeOptions.NodeService.LoadBalancerClass,
							InternalTrafficPolicy:         sc.Spec.ExposeOptions.NodeService.InternalTrafficPolicy,
						}
					}(),
					BroadcastOptions: func() *scyllav1alpha1.NodeBroadcastOptions {
						if sc.Spec.ExposeOptions.BroadcastOptions == nil {
							return nil
						}

						return &scyllav1alpha1.NodeBroadcastOptions{
							Nodes: scyllav1alpha1.BroadcastOptions{
								Type: scyllav1alpha1.BroadcastAddressType(sc.Spec.ExposeOptions.BroadcastOptions.Nodes.Type),
								PodIP: func() *scyllav1alpha1.PodIPAddressOptions {
									if sc.Spec.ExposeOptions.BroadcastOptions.Nodes.PodIP == nil {
										return nil
									}
									return &scyllav1alpha1.PodIPAddressOptions{
										Source: scyllav1alpha1.PodIPSourceType(sc.Spec.ExposeOptions.BroadcastOptions.Nodes.PodIP.Source),
									}
								}(),
							},
							Clients: scyllav1alpha1.BroadcastOptions{
								Type: scyllav1alpha1.BroadcastAddressType(sc.Spec.ExposeOptions.BroadcastOptions.Clients.Type),
								PodIP: func() *scyllav1alpha1.PodIPAddressOptions {
									if sc.Spec.ExposeOptions.BroadcastOptions.Clients.PodIP == nil {
										return nil
									}
									return &scyllav1alpha1.PodIPAddressOptions{
										Source: scyllav1alpha1.PodIPSourceType(sc.Spec.ExposeOptions.BroadcastOptions.Clients.PodIP.Source),
									}
								}(),
							},
						}
					}(),
				}
			}(),
			// v1.ScyllaCluster doesn't provide API to specify cluster/datacenter wide properties of scylladb.
			RackTemplate: nil,
			Racks: func() []scyllav1alpha1.RackSpec {
				var racks []scyllav1alpha1.RackSpec

				for _, rack := range sc.Spec.Datacenter.Racks {
					racks = append(racks, scyllav1alpha1.RackSpec{
						RackTemplate: scyllav1alpha1.RackTemplate{
							Nodes: pointer.Ptr(rack.Members),
							Placement: func() *scyllav1alpha1.Placement {
								if rack.Placement == nil {
									return nil
								}
								return &scyllav1alpha1.Placement{
									NodeAffinity:    rack.Placement.NodeAffinity,
									PodAffinity:     rack.Placement.PodAffinity,
									PodAntiAffinity: rack.Placement.PodAntiAffinity,
									Tolerations:     rack.Placement.Tolerations,
								}
							}(),
							TopologyLabelSelector: nil,
							ScyllaDB: &scyllav1alpha1.ScyllaDBTemplate{
								Resources: pointer.Ptr(rack.Resources),
								Storage: &scyllav1alpha1.StorageOptions{
									Metadata: func() scyllav1alpha1.ObjectTemplateMetadata {
										if rack.Storage.Metadata == nil {
											return scyllav1alpha1.ObjectTemplateMetadata{}
										}
										return scyllav1alpha1.ObjectTemplateMetadata{
											Labels:      rack.Storage.Metadata.Labels,
											Annotations: rack.Storage.Metadata.Annotations,
										}
									}(),
									Capacity:         rack.Storage.Capacity,
									StorageClassName: rack.Storage.StorageClassName,
								},
								CustomConfigMapRef: func() *string {
									if len(rack.ScyllaConfig) > 0 {
										return pointer.Ptr(rack.ScyllaConfig)
									}
									return nil
								}(),
								Volumes: slices.Filter(rack.Volumes, func(v corev1.Volume) bool {
									return slices.Contains(rack.VolumeMounts, func(vm corev1.VolumeMount) bool {
										return vm.Name == v.Name
									})
								}),
								VolumeMounts: rack.VolumeMounts,
							},
							ScyllaDBManagerAgent: &scyllav1alpha1.ScyllaDBManagerAgentTemplate{
								Resources: pointer.Ptr(rack.AgentResources),
								CustomConfigSecretRef: func() *string {
									if len(rack.ScyllaAgentConfig) > 0 {
										return pointer.Ptr(rack.ScyllaAgentConfig)
									}
									return nil
								}(),
								Volumes: slices.Filter(rack.Volumes, func(v corev1.Volume) bool {
									return slices.Contains(rack.AgentVolumeMounts, func(vm corev1.VolumeMount) bool {
										return vm.Name == v.Name
									})
								}),
								VolumeMounts: rack.AgentVolumeMounts,
							},
						},
						Name: rack.Name,
					})
				}

				return racks
			}(),
			DisableAutomaticOrphanedNodeReplacement: !sc.Spec.AutomaticOrphanedNodeCleanup,
			MinTerminationGracePeriodSeconds:        sc.Spec.MinTerminationGracePeriodSeconds,
			MinReadySeconds:                         sc.Spec.MinReadySeconds,
			ReadinessGates:                          sc.Spec.ReadinessGates,
		},
		// Status is reconciled by the controllers
		Status: scyllav1alpha1.ScyllaDBDatacenterStatus{},
	}, errors.NewAggregate(convertErrs)
}
