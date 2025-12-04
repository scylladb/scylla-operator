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
	oslices "github.com/scylladb/scylla-operator/pkg/helpers/slices"
	"github.com/scylladb/scylla-operator/pkg/internalapi"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apimachineryutilerrors "k8s.io/apimachinery/pkg/util/errors"
	apimachineryutilvalidation "k8s.io/apimachinery/pkg/util/validation"
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
		IPFamily:                scSpec.IPFamily,
		IPFamilyPolicy:          scSpec.Network.IPFamilyPolicy,
		IPFamilies:              scSpec.Network.IPFamilies,
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
							Volumes: oslices.Filter(rack.Volumes, func(v corev1.Volume) bool {
								return oslices.Contains(rack.VolumeMounts, func(vm corev1.VolumeMount) bool {
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
							Volumes: oslices.Filter(rack.Volumes, func(v corev1.Volume) bool {
								return oslices.Contains(rack.AgentVolumeMounts, func(vm corev1.VolumeMount) bool {
									return vm.Name == v.Name
								})
							}),
							VolumeMounts: rack.AgentVolumeMounts,
						},
						ExposeOptions: func() *scyllav1alpha1.RackExposeOptions {
							if rack.ExposeOptions == nil {
								return nil
							}

							return &scyllav1alpha1.RackExposeOptions{
								NodeService: func() *scyllav1alpha1.RackNodeServiceTemplate {
									if rack.ExposeOptions.NodeService == nil {
										return nil
									}
									return &scyllav1alpha1.RackNodeServiceTemplate{
										ObjectTemplateMetadata: scyllav1alpha1.ObjectTemplateMetadata{
											Labels:      maps.Clone(rack.ExposeOptions.NodeService.Labels),
											Annotations: maps.Clone(rack.ExposeOptions.NodeService.Annotations),
										},
									}
								}(),
							}
						}(),
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
	}, apimachineryutilerrors.NewAggregate(migrateErrs)
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

			if isGlobalScyllaDBManagerIntegrationDisabled(sc) {
				objectMeta.Labels[naming.GlobalScyllaDBManagerRegistrationLabel] = naming.LabelValueFalse
			} else {
				// If the ScyllaDBManager integration is not disabled explicitly, we set the label to true to keep backward compatibility.
				objectMeta.Labels[naming.GlobalScyllaDBManagerRegistrationLabel] = naming.LabelValueTrue
			}

			// Override the ScyllaDB Manager cluster name for backward compatibility.
			objectMeta.Annotations[naming.ScyllaDBManagerClusterRegistrationNameOverrideAnnotation] = naming.ManagerClusterName(sc)

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

	return sdc, upgradeContext, apimachineryutilerrors.NewAggregate(migrateErrs)
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

				rackServices := oslices.Filter(services, func(svc *corev1.Service) bool {
					return svc.Labels[naming.RackNameLabel] == rackStatus.Name
				})

				anyIsDecommissioning := oslices.Contains(rackServices, func(svc *corev1.Service) bool {
					return svc.Labels[naming.DecommissionedLabel] == naming.LabelValueFalse
				})
				if anyIsDecommissioning {
					controllerhelpers.SetRackCondition(&migratedRackStatus, scyllav1.RackConditionTypeMemberDecommissioning)
				}

				anyIsReplacing := oslices.Contains(rackServices, func(svc *corev1.Service) bool {
					_, ok := svc.Labels[naming.ReplaceLabel]
					return ok
				})

				if anyIsReplacing {
					controllerhelpers.SetRackCondition(&migratedRackStatus, scyllav1.RackConditionTypeMemberReplacing)
				}

				anyIsLeaving := oslices.Contains(rackServices, func(svc *corev1.Service) bool {
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

			cm, _, ok := oslices.Find(configMaps, func(cm *corev1.ConfigMap) bool {
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

func MigrateV1ScyllaClusterToV1Alpha1ScyllaDBManagerTasks(sc *scyllav1.ScyllaCluster) ([]*scyllav1alpha1.ScyllaDBManagerTask, error) {
	var scyllaDBManagerTasks []*scyllav1alpha1.ScyllaDBManagerTask
	var errs []error

	// If the annotation disabling global manager integration is set to true, we skip migration of tasks.
	if sc.Annotations != nil && sc.Annotations[naming.DisableGlobalScyllaDBManagerIntegrationAnnotation] == naming.LabelValueTrue {
		return scyllaDBManagerTasks, nil
	}

	for i := range sc.Spec.Backups {
		smt, err := migrateV1BackupTaskSpecToV1Alpha1ScyllaDBManagerTask(sc, &sc.Spec.Backups[i])
		if err != nil {
			errs = append(errs, fmt.Errorf("can't migrate v1.BackupTaskSpec to v1alpha1.ScyllaDBManagerTask: %w", err))
			continue
		}

		scyllaDBManagerTasks = append(scyllaDBManagerTasks, smt)
	}

	for i := range sc.Spec.Repairs {
		smt, err := migrateV1RepairTaskSpecToV1Alpha1ScyllaDBManagerTask(sc, &sc.Spec.Repairs[i])
		if err != nil {
			errs = append(errs, fmt.Errorf("can't migrate v1.RepairTaskSpec to v1alpha1.ScyllaDBManagerTask: %w", err))
			continue
		}

		scyllaDBManagerTasks = append(scyllaDBManagerTasks, smt)
	}

	return scyllaDBManagerTasks, apimachineryutilerrors.NewAggregate(errs)
}

type v1TaskSpecToV1Alpha1ScyllaDBManagerTaskMigrationOptions func(*scyllav1alpha1.ScyllaDBManagerTask)

func withExtraAnnotations(extraAnnotations map[string]string) v1TaskSpecToV1Alpha1ScyllaDBManagerTaskMigrationOptions {
	return func(smt *scyllav1alpha1.ScyllaDBManagerTask) {
		if smt.Annotations == nil {
			smt.Annotations = make(map[string]string)
		}
		maps.Copy(smt.Annotations, extraAnnotations)
	}
}

func withScyllaDBManagerTaskType(taskType scyllav1alpha1.ScyllaDBManagerTaskType) v1TaskSpecToV1Alpha1ScyllaDBManagerTaskMigrationOptions {
	return func(smt *scyllav1alpha1.ScyllaDBManagerTask) {
		smt.Spec.Type = taskType
	}
}

func migrateV1BackupTaskSpecToV1Alpha1ScyllaDBManagerTask(sc *scyllav1.ScyllaCluster, backupTaskSpec *scyllav1.BackupTaskSpec) (*scyllav1alpha1.ScyllaDBManagerTask, error) {
	name, err := scyllaDBManagerTaskName(sc.Name, scyllav1alpha1.ScyllaDBManagerTaskTypeBackup, backupTaskSpec.Name)
	if err != nil {
		return nil, fmt.Errorf("can't generate ScyllaDBManagerTask name: %w", err)
	}

	extraAnnotations := map[string]string{
		naming.ScyllaDBManagerTaskBackupDCNoValidateAnnotation:               "",
		naming.ScyllaDBManagerTaskBackupKeyspaceNoValidateAnnotation:         "",
		naming.ScyllaDBManagerTaskBackupLocationNoValidateAnnotation:         "",
		naming.ScyllaDBManagerTaskBackupRateLimitNoValidateAnnotation:        "",
		naming.ScyllaDBManagerTaskBackupRetentionNoValidateAnnotation:        "",
		naming.ScyllaDBManagerTaskBackupSnapshotParallelNoValidateAnnotation: "",
		naming.ScyllaDBManagerTaskBackupUploadParallelNoValidateAnnotation:   "",
	}

	withBackupOptions := func(smt *scyllav1alpha1.ScyllaDBManagerTask) {
		smt.Spec.Backup = &scyllav1alpha1.ScyllaDBManagerBackupTaskOptions{
			ScyllaDBManagerTaskSchedule: scyllav1alpha1.ScyllaDBManagerTaskSchedule{
				Cron:       backupTaskSpec.TaskSpec.SchedulerTaskSpec.Cron,
				NumRetries: backupTaskSpec.TaskSpec.SchedulerTaskSpec.NumRetries,
				RetryWait:  backupTaskSpec.TaskSpec.SchedulerTaskSpec.RetryWait,
			},
			DC:               backupTaskSpec.DC,
			Keyspace:         backupTaskSpec.Keyspace,
			Location:         backupTaskSpec.Location,
			RateLimit:        backupTaskSpec.RateLimit,
			Retention:        pointer.Ptr(backupTaskSpec.Retention),
			SnapshotParallel: backupTaskSpec.SnapshotParallel,
			UploadParallel:   backupTaskSpec.UploadParallel,
		}
	}

	return migrateV1TaskSpecToV1Alpha1ScyllaDBManagerTask(name, sc, &backupTaskSpec.TaskSpec, []v1TaskSpecToV1Alpha1ScyllaDBManagerTaskMigrationOptions{
		withExtraAnnotations(extraAnnotations),
		withScyllaDBManagerTaskType(scyllav1alpha1.ScyllaDBManagerTaskTypeBackup),
		withBackupOptions,
	}...), nil
}

func migrateV1RepairTaskSpecToV1Alpha1ScyllaDBManagerTask(sc *scyllav1.ScyllaCluster, repairTaskSpec *scyllav1.RepairTaskSpec) (*scyllav1alpha1.ScyllaDBManagerTask, error) {
	name, err := scyllaDBManagerTaskName(sc.Name, scyllav1alpha1.ScyllaDBManagerTaskTypeRepair, repairTaskSpec.Name)
	if err != nil {
		return nil, fmt.Errorf("can't generate ScyllaDBManagerTask name: %w", err)
	}

	extraAnnotations := map[string]string{
		naming.ScyllaDBManagerTaskRepairDCNoValidateAnnotation:                "",
		naming.ScyllaDBManagerTaskRepairKeyspaceNoValidateAnnotation:          "",
		naming.ScyllaDBManagerTaskRepairIntensityOverrideAnnotation:           repairTaskSpec.Intensity,
		naming.ScyllaDBManagerTaskRepairParallelNoValidateAnnotation:          "",
		naming.ScyllaDBManagerTaskRepairSmallTableThresholdOverrideAnnotation: repairTaskSpec.SmallTableThreshold,
	}

	withRepairOptions := func(smt *scyllav1alpha1.ScyllaDBManagerTask) {
		smt.Spec.Repair = &scyllav1alpha1.ScyllaDBManagerRepairTaskOptions{
			ScyllaDBManagerTaskSchedule: scyllav1alpha1.ScyllaDBManagerTaskSchedule{
				Cron:       repairTaskSpec.TaskSpec.SchedulerTaskSpec.Cron,
				NumRetries: repairTaskSpec.TaskSpec.SchedulerTaskSpec.NumRetries,
				RetryWait:  repairTaskSpec.TaskSpec.SchedulerTaskSpec.RetryWait,
			},
			DC:              repairTaskSpec.DC,
			Keyspace:        repairTaskSpec.Keyspace,
			FailFast:        pointer.Ptr(repairTaskSpec.FailFast),
			Host:            repairTaskSpec.Host,
			IgnoreDownHosts: repairTaskSpec.IgnoreDownHosts,
			Parallel:        pointer.Ptr(repairTaskSpec.Parallel),
		}
	}

	return migrateV1TaskSpecToV1Alpha1ScyllaDBManagerTask(name, sc, &repairTaskSpec.TaskSpec, []v1TaskSpecToV1Alpha1ScyllaDBManagerTaskMigrationOptions{
		withExtraAnnotations(extraAnnotations),
		withScyllaDBManagerTaskType(scyllav1alpha1.ScyllaDBManagerTaskTypeRepair),
		withRepairOptions,
	}...), nil
}

func migrateV1TaskSpecToV1Alpha1ScyllaDBManagerTask(name string, sc *scyllav1.ScyllaCluster, taskSpec *scyllav1.TaskSpec, options ...v1TaskSpecToV1Alpha1ScyllaDBManagerTaskMigrationOptions) *scyllav1alpha1.ScyllaDBManagerTask {
	smt := &scyllav1alpha1.ScyllaDBManagerTask{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: sc.Namespace,
			Labels:    naming.ClusterLabelsForScyllaCluster(sc),
			Annotations: func() map[string]string {
				annotations := map[string]string{
					// Override the task name in ScyllaDB Manager state for backward compatibility.
					naming.ScyllaDBManagerTaskNameOverrideAnnotation: taskSpec.Name,

					// Force adoption of the task existing in ScyllaDB Manager state for state retention.
					naming.ScyllaDBManagerTaskMissingOwnerUIDForceAdoptAnnotation: "",
				}

				if taskSpec.SchedulerTaskSpec.StartDate != nil {
					annotations[naming.ScyllaDBManagerTaskScheduleStartDateOverrideAnnotation] = *taskSpec.SchedulerTaskSpec.StartDate
				}
				if taskSpec.SchedulerTaskSpec.Interval != nil {
					annotations[naming.ScyllaDBManagerTaskScheduleIntervalOverrideAnnotation] = *taskSpec.SchedulerTaskSpec.Interval
				}
				if taskSpec.SchedulerTaskSpec.Timezone != nil {
					annotations[naming.ScyllaDBManagerTaskScheduleTimezoneOverrideAnnotation] = *taskSpec.SchedulerTaskSpec.Timezone
				}

				return annotations
			}(),
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(sc, scyllaClusterControllerGVK),
			},
		},
		Spec: scyllav1alpha1.ScyllaDBManagerTaskSpec{
			ScyllaDBClusterRef: scyllav1alpha1.LocalScyllaDBReference{
				Name: sc.Name,
				Kind: scyllav1alpha1.ScyllaDBDatacenterGVK.Kind,
			},
		},
	}

	for _, o := range options {
		o(smt)
	}

	return smt
}

func scyllaDBManagerTaskName(scyllaClusterName string, taskType scyllav1alpha1.ScyllaDBManagerTaskType, taskName string) (string, error) {
	nameSuffix, err := naming.GenerateNameHash(scyllaClusterName, string(taskType), taskName)
	if err != nil {
		return "", fmt.Errorf("can't generate name hash: %w", err)
	}

	fullName := strings.ToLower(fmt.Sprintf("%s-%s-%s", scyllaClusterName, taskType, taskName))
	fullNameWithSuffix := fmt.Sprintf("%s-%s", fullName[:min(len(fullName), apimachineryutilvalidation.DNS1123SubdomainMaxLength-len(nameSuffix)-1)], nameSuffix)
	return fullNameWithSuffix, nil
}

func migrateV1Alpha1ScyllaDBManagerTaskStatusToV1BackupTaskStatus(smt *scyllav1alpha1.ScyllaDBManagerTask, taskName string) (scyllav1.BackupTaskStatus, bool, error) {
	return migrateV1Alpha1ScyllaDBManagerTaskStatusToV1TaskStatus[scyllav1.BackupTaskStatus](smt, func(taskStatus scyllav1.TaskStatus) scyllav1.BackupTaskStatus {
		taskStatus.Name = taskName
		return scyllav1.BackupTaskStatus{
			TaskStatus: taskStatus,
		}
	})
}

func migrateV1Alpha1ScyllaDBManagerTaskStatusToV1RepairTaskStatus(smt *scyllav1alpha1.ScyllaDBManagerTask, taskName string) (scyllav1.RepairTaskStatus, bool, error) {
	return migrateV1Alpha1ScyllaDBManagerTaskStatusToV1TaskStatus[scyllav1.RepairTaskStatus](smt, func(taskStatus scyllav1.TaskStatus) scyllav1.RepairTaskStatus {
		taskStatus.Name = taskName
		return scyllav1.RepairTaskStatus{
			TaskStatus: taskStatus,
		}
	})
}

func migrateV1Alpha1ScyllaDBManagerTaskStatusToV1TaskStatus[T scyllav1.BackupTaskStatus | scyllav1.RepairTaskStatus](
	smt *scyllav1alpha1.ScyllaDBManagerTask,
	newTaskStatusFunc func(scyllav1.TaskStatus) T,
) (T, bool, error) {
	var taskStatusErr *string

	degradedCondition := meta.FindStatusCondition(smt.Status.Conditions, scyllav1alpha1.DegradedCondition)
	if degradedCondition != nil && degradedCondition.Status == metav1.ConditionTrue {
		taskStatusErr = pointer.Ptr(degradedCondition.Message)
	}

	taskStatus := newTaskStatusFunc(scyllav1.TaskStatus{
		ID:    smt.Status.TaskID,
		Error: taskStatusErr,
	})

	scyllaV1TaskStatusAnnotation, ok := smt.Annotations[naming.ScyllaDBManagerTaskStatusAnnotation]
	if ok {
		// Use the existing TaskStatus instance to retain the potential error.
		err := json.NewDecoder(strings.NewReader(scyllaV1TaskStatusAnnotation)).Decode(&taskStatus)
		if err != nil {
			return *new(T), false, fmt.Errorf("can't decode ScyllaDBManagerTask status annotation: %w", err)
		}
	} else if taskStatusErr == nil {
		// Neither task's status nor error have been propagated yet.
		return *new(T), false, nil
	}

	return taskStatus, true, nil
}

func isGlobalScyllaDBManagerIntegrationDisabled(sc *scyllav1.ScyllaCluster) bool {
	if sc.Annotations == nil {
		// If there are no annotations, the default behavior is to enable global ScyllaDB Manager integration.
		return false
	}

	// If the annotation disabling global manager integration is set to true, we skip migration of tasks.
	return sc.Annotations[naming.DisableGlobalScyllaDBManagerIntegrationAnnotation] == naming.LabelValueTrue
}
