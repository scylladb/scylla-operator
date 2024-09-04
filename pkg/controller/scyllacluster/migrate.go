// Copyright (c) 2024 ScyllaDB.

package scyllacluster

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"strings"

	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/helpers/slices"
	"github.com/scylladb/scylla-operator/pkg/internalapi"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
)

func MigrateV1ScyllaClusterSpecToV1Alpha1ScyllaDBDatacenterSpec(scName string, scSpec scyllav1.ScyllaClusterSpec) (scyllav1alpha1.ScyllaDBDatacenterSpec, error) {
	var migrateErrs []error

	return scyllav1alpha1.ScyllaDBDatacenterSpec{
		Metadata: func() *scyllav1alpha1.ObjectTemplateMetadata {
			if scSpec.PodMetadata == nil {
				return nil
			}
			return &scyllav1alpha1.ObjectTemplateMetadata{
				Labels:      scSpec.PodMetadata.Labels,
				Annotations: scSpec.PodMetadata.Annotations,
			}
		}(),
		ClusterName:    scName,
		DatacenterName: pointer.Ptr(scSpec.Datacenter.Name),
		ScyllaDB: scyllav1alpha1.ScyllaDB{
			// v1.ScyllaCluster doesn't provide API to specify cluster/datacenter wide properties of scylladb.
			ScyllaDBTemplate: scyllav1alpha1.ScyllaDBTemplate{},
			Image: func() string {
				if len(scSpec.Repository) == 0 {
					migrateErrs = append(migrateErrs, fmt.Errorf("v1alpha1.ScyllaCluster ScyllaDB repository cannot be empty"))
					return ""
				}
				if len(scSpec.Version) == 0 {
					migrateErrs = append(migrateErrs, fmt.Errorf("v1alpha1.ScyllaCluster ScyllaDB version cannot be empty"))
					return ""
				}
				return fmt.Sprintf("%s:%s", scSpec.Repository, scSpec.Version)
			}(),
			ExternalSeeds: scSpec.ExternalSeeds,
			AlternatorOptions: func() *scyllav1alpha1.AlternatorOptions {
				if scSpec.Alternator == nil {
					return nil
				}

				return &scyllav1alpha1.AlternatorOptions{
					WriteIsolation: scSpec.Alternator.WriteIsolation,
					ServingCertificate: func() *scyllav1alpha1.TLSCertificate {
						if scSpec.Alternator.ServingCertificate == nil {
							return nil
						}
						return &scyllav1alpha1.TLSCertificate{
							Type: scyllav1alpha1.TLSCertificateType(scSpec.Alternator.ServingCertificate.Type),
							UserManagedOptions: func() *scyllav1alpha1.UserManagedTLSCertificateOptions {
								if scSpec.Alternator.ServingCertificate.UserManagedOptions == nil {
									return nil
								}
								return &scyllav1alpha1.UserManagedTLSCertificateOptions{
									SecretName: scSpec.Alternator.ServingCertificate.UserManagedOptions.SecretName,
								}
							}(),
							OperatorManagedOptions: func() *scyllav1alpha1.OperatorManagedTLSCertificateOptions {
								if scSpec.Alternator.ServingCertificate.OperatorManagedOptions == nil {
									return nil
								}
								return &scyllav1alpha1.OperatorManagedTLSCertificateOptions{
									AdditionalDNSNames:    scSpec.Alternator.ServingCertificate.OperatorManagedOptions.AdditionalDNSNames,
									AdditionalIPAddresses: scSpec.Alternator.ServingCertificate.OperatorManagedOptions.AdditionalIPAddresses,
								}
							}(),
						}
					}(),
				}
			}(),
			AdditionalScyllaDBArguments: strings.Split(scSpec.ScyllaArgs, " "),
			EnableDeveloperMode:         pointer.Ptr(scSpec.DeveloperMode),
		},
		ScyllaDBManagerAgent: &scyllav1alpha1.ScyllaDBManagerAgent{
			// v1.ScyllaCluster doesn't provide API to specify cluster/datacenter wide properties of scylladb manager agent.
			ScyllaDBManagerAgentTemplate: scyllav1alpha1.ScyllaDBManagerAgentTemplate{},
			Image: func() *string {
				if len(scSpec.AgentRepository) == 0 {
					migrateErrs = append(migrateErrs, fmt.Errorf("v1alpha1.ScyllaCluster ScyllaDB Manager Agent repository cannot be empty"))
					return nil
				}
				if len(scSpec.AgentVersion) == 0 {
					migrateErrs = append(migrateErrs, fmt.Errorf("v1alpha1.ScyllaCluster ScyllaDB Manager Agent version cannot be empty"))
					return nil
				}
				return pointer.Ptr(fmt.Sprintf("%s:%s", scSpec.AgentRepository, scSpec.AgentVersion))
			}(),
		},
		ImagePullSecrets:        scSpec.ImagePullSecrets,
		DNSPolicy:               pointer.Ptr(scSpec.Network.GetDNSPolicy()),
		DNSDomains:              scSpec.DNSDomains,
		ForceRedeploymentReason: pointer.Ptr(scSpec.ForceRedeploymentReason),
		ExposeOptions: func() *scyllav1alpha1.ExposeOptions {
			if scSpec.ExposeOptions == nil {
				return nil
			}

			return &scyllav1alpha1.ExposeOptions{
				CQL: func() *scyllav1alpha1.CQLExposeOptions {
					if scSpec.ExposeOptions.CQL == nil {
						return nil
					}
					return &scyllav1alpha1.CQLExposeOptions{
						Ingress: func() *scyllav1alpha1.CQLExposeIngressOptions {
							if scSpec.ExposeOptions.CQL.Ingress == nil {
								return nil
							}
							return &scyllav1alpha1.CQLExposeIngressOptions{
								ObjectTemplateMetadata: scyllav1alpha1.ObjectTemplateMetadata{
									Labels:      scSpec.ExposeOptions.CQL.Ingress.Labels,
									Annotations: scSpec.ExposeOptions.CQL.Ingress.Annotations,
								},
								IngressClassName: scSpec.ExposeOptions.CQL.Ingress.IngressClassName,
							}
						}(),
					}
				}(),
				NodeService: func() *scyllav1alpha1.NodeServiceTemplate {
					if scSpec.ExposeOptions.NodeService == nil {
						return nil
					}
					return &scyllav1alpha1.NodeServiceTemplate{
						ObjectTemplateMetadata: scyllav1alpha1.ObjectTemplateMetadata{
							Labels:      scSpec.ExposeOptions.NodeService.Labels,
							Annotations: scSpec.ExposeOptions.NodeService.Annotations,
						},
						Type:                          scyllav1alpha1.NodeServiceType(scSpec.ExposeOptions.NodeService.Type),
						ExternalTrafficPolicy:         scSpec.ExposeOptions.NodeService.ExternalTrafficPolicy,
						AllocateLoadBalancerNodePorts: scSpec.ExposeOptions.NodeService.AllocateLoadBalancerNodePorts,
						LoadBalancerClass:             scSpec.ExposeOptions.NodeService.LoadBalancerClass,
						InternalTrafficPolicy:         scSpec.ExposeOptions.NodeService.InternalTrafficPolicy,
					}
				}(),
				BroadcastOptions: func() *scyllav1alpha1.NodeBroadcastOptions {
					if scSpec.ExposeOptions.BroadcastOptions == nil {
						return nil
					}

					return &scyllav1alpha1.NodeBroadcastOptions{
						Nodes: scyllav1alpha1.BroadcastOptions{
							Type: scyllav1alpha1.BroadcastAddressType(scSpec.ExposeOptions.BroadcastOptions.Nodes.Type),
							PodIP: func() *scyllav1alpha1.PodIPAddressOptions {
								if scSpec.ExposeOptions.BroadcastOptions.Nodes.PodIP == nil {
									return nil
								}
								return &scyllav1alpha1.PodIPAddressOptions{
									Source: scyllav1alpha1.PodIPSourceType(scSpec.ExposeOptions.BroadcastOptions.Nodes.PodIP.Source),
								}
							}(),
						},
						Clients: scyllav1alpha1.BroadcastOptions{
							Type: scyllav1alpha1.BroadcastAddressType(scSpec.ExposeOptions.BroadcastOptions.Clients.Type),
							PodIP: func() *scyllav1alpha1.PodIPAddressOptions {
								if scSpec.ExposeOptions.BroadcastOptions.Clients.PodIP == nil {
									return nil
								}
								return &scyllav1alpha1.PodIPAddressOptions{
									Source: scyllav1alpha1.PodIPSourceType(scSpec.ExposeOptions.BroadcastOptions.Clients.PodIP.Source),
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

			for _, rack := range scSpec.Datacenter.Racks {
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
								Metadata: func() *scyllav1alpha1.ObjectTemplateMetadata {
									if rack.Storage.Metadata == nil {
										return nil
									}
									return &scyllav1alpha1.ObjectTemplateMetadata{
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
		DisableAutomaticOrphanedNodeReplacement: pointer.Ptr(!scSpec.AutomaticOrphanedNodeCleanup),
		MinTerminationGracePeriodSeconds:        scSpec.MinTerminationGracePeriodSeconds,
		MinReadySeconds:                         scSpec.MinReadySeconds,
		ReadinessGates:                          scSpec.ReadinessGates,
	}, errors.NewAggregate(migrateErrs)
}

func MigrateV1ScyllaClusterToV1Alpha1ScyllaDBDatacenter(sc *scyllav1.ScyllaCluster) (*scyllav1alpha1.ScyllaDBDatacenter, *internalapi.DatacenterUpgradeContext, error) {
	var migrateErrs []error

	sdcSpec, err := MigrateV1ScyllaClusterSpecToV1Alpha1ScyllaDBDatacenterSpec(sc.Name, sc.Spec)
	if err != nil {
		migrateErrs = append(migrateErrs, fmt.Errorf("can't migrate v1.ScyllaClusterSpec to v1alpha1.ScyllaDBDatacenterSpec: %w", err))
	}

	sdc := &scyllav1alpha1.ScyllaDBDatacenter{
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
					migrateErrs = append(migrateErrs, fmt.Errorf("can't encode sysctls: %w", err))
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
		Spec: sdcSpec,
		// Status is reconciled by the controllers
		Status: scyllav1alpha1.ScyllaDBDatacenterStatus{},
	}

	var upgradeContext *internalapi.DatacenterUpgradeContext

	if sc.Status.Upgrade != nil {
		upgradeContext = &internalapi.DatacenterUpgradeContext{
			State: func() internalapi.UpgradePhase {
				switch sc.Status.Upgrade.State {
				case "PreHooks":
					return internalapi.PreHooksUpgradePhase
				case "RolloutInit":
					return internalapi.RolloutRunUpgradePhase
				case "RolloutRun":
					return internalapi.RolloutRunUpgradePhase
				case "PostHooks":
					return internalapi.PostHooksUpgradePhase
				default:
					migrateErrs = append(migrateErrs, fmt.Errorf("unknown upgrade state %q in v1.ScyllaCluster %q status", sc.Status.Upgrade.State, naming.ObjRef(sc)))
					return ""
				}
			}(),
			FromVersion:       sc.Status.Upgrade.FromVersion,
			ToVersion:         sc.Status.Upgrade.ToVersion,
			SystemSnapshotTag: sc.Status.Upgrade.SystemSnapshotTag,
			DataSnapshotTag:   sc.Status.Upgrade.DataSnapshotTag,
		}
	}

	return sdc, upgradeContext, errors.NewAggregate(migrateErrs)
}

func migrateV1Alpha1ScyllaDBDatacenterStatusToV1ScyllaClusterStatus(sdc *scyllav1alpha1.ScyllaDBDatacenter, configMaps []*corev1.ConfigMap, services []*corev1.Service) scyllav1.ScyllaClusterStatus {
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
				migratedRackStatus := scyllav1.RackStatus{
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
					Conditions:       nil,
				}

				rackServices := slices.Filter(services, func(svc *corev1.Service) bool {
					return svc.Labels[naming.RackNameLabel] == rackStatus.Name
				})

				anyIsDecommissioning := slices.Contains(rackServices, func(svc *corev1.Service) bool {
					return svc.Labels[naming.DecommissionedLabel] == naming.LabelValueFalse
				})
				if anyIsDecommissioning {
					controllerhelpers.SetRackCondition(&migratedRackStatus, scyllav1.RackConditionTypeMemberDecommissioning)
				}

				anyIsReplacing := slices.Contains(rackServices, func(svc *corev1.Service) bool {
					_, ok := svc.Labels[naming.ReplaceLabel]
					return ok
				})

				if anyIsReplacing {
					controllerhelpers.SetRackCondition(&migratedRackStatus, scyllav1.RackConditionTypeMemberReplacing)
				}

				anyIsLeaving := slices.Contains(rackServices, func(svc *corev1.Service) bool {
					_, ok := svc.Labels[naming.DecommissionedLabel]
					return ok
				})

				if anyIsLeaving {
					controllerhelpers.SetRackCondition(&migratedRackStatus, scyllav1.RackConditionTypeMemberLeaving)
				}

				if rackStatus.CurrentVersion != rackStatus.UpdatedVersion {
					controllerhelpers.SetRackCondition(&migratedRackStatus, scyllav1.RackConditionTypeUpgrading)
				}

				rackStatuses[rackStatus.Name] = migratedRackStatus
			}
			return rackStatuses
		}(),
		Upgrade: func() *scyllav1.UpgradeStatus {
			cmName := naming.UpgradeContextConfigMapName(sdc)

			cm, _, ok := slices.Find(configMaps, func(cm *corev1.ConfigMap) bool {
				return cm.Name == cmName
			})
			if !ok {
				// Upgrade isn't in progress.
				return nil
			}

			ucRaw, ok := cm.Data[naming.UpgradeContextConfigMapKey]
			if !ok {
				klog.ErrorS(fmt.Errorf("upgrade context configmap is missing required key"), " ConfigMap", cmName, "Key", naming.UpgradeContextConfigMapKey)
				return nil
			}

			uc := &internalapi.DatacenterUpgradeContext{}
			err := uc.Decode(strings.NewReader(ucRaw))
			if err != nil {
				klog.ErrorS(err, "can't decode upgrade context", "ConfigMap", cmName, "Key", naming.UpgradeContextConfigMapKey)
				return nil
			}

			return &scyllav1.UpgradeStatus{
				State:             string(uc.State),
				FromVersion:       uc.FromVersion,
				ToVersion:         uc.ToVersion,
				SystemSnapshotTag: uc.SystemSnapshotTag,
				DataSnapshotTag:   uc.DataSnapshotTag,
			}
		}(),
		// These are not handled by v1alpha1.ScyllaDBDatacenter.
		Repairs:   nil,
		Backups:   nil,
		ManagerID: nil,
	}
}
