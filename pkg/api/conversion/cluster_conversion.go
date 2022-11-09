// Copyright (c) 2022 ScyllaDB.

package conversion

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	scyllav2alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v2alpha1"
	"github.com/scylladb/scylla-operator/pkg/naming"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"k8s.io/utils/pointer"
)

func convertV1ScyllaClusterToV2alpha1(sc *scyllav1.ScyllaCluster) (*scyllav2alpha1.ScyllaCluster, error) {
	annotations := map[string]string{}

	scCopy := sc.DeepCopy()
	delete(scCopy.Annotations, naming.ScyllaClusterV2alpha1ScyllaClusterConditionsAnnotation)
	delete(scCopy.Annotations, naming.ScyllaClusterV2alpha1ScyllaDatacenterStaleAnnotation)
	delete(scCopy.Annotations, naming.ScyllaClusterV2alpha1RemoteKubeClusterConfigRefAnnotation)
	delete(scCopy.Annotations, naming.ScyllaClusterV1Annotation)
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(scCopy); err != nil {
		return nil, fmt.Errorf("can't encode v1.ScyllaCluster in annotation: %w", err)
	}
	annotations[naming.ScyllaClusterV1Annotation] = buf.String()

	spec, err := convertV1ScyllaClusterSpecToV2alpha1(sc, sc.Spec)
	if err != nil {
		return nil, fmt.Errorf("can't convert v1.ScyllaClusterSpec to v2alpha1: %w", err)
	}

	status, err := convertV1ScyllaClusterStatusToV2alpha1(sc)
	if err != nil {
		return nil, fmt.Errorf("can't convert v1.ScyllaClusterStatus to v2alpha1: %w", err)
	}

	if sc.Annotations == nil {
		sc.Annotations = map[string]string{}
	}

	for k, v := range annotations {
		sc.Annotations[k] = v
	}

	delete(sc.Annotations, naming.ScyllaClusterV2alpha1ScyllaClusterConditionsAnnotation)
	delete(sc.Annotations, naming.ScyllaClusterV2alpha1ScyllaDatacenterStaleAnnotation)
	delete(sc.Annotations, naming.ScyllaClusterV2alpha1RemoteKubeClusterConfigRefAnnotation)

	csc := &scyllav2alpha1.ScyllaCluster{
		ObjectMeta: sc.ObjectMeta,
		Spec:       *spec,
		Status:     *status,
	}

	return csc, nil
}

func convertV1ScyllaClusterSpecToV2alpha1(sc *scyllav1.ScyllaCluster, scSpec scyllav1.ScyllaClusterSpec) (*scyllav2alpha1.ScyllaClusterSpec, error) {
	var alternatorOptions *scyllav2alpha1.AlternatorOptions
	if scSpec.Alternator != nil {
		alternatorOptions = &scyllav2alpha1.AlternatorOptions{WriteIsolation: scSpec.Alternator.WriteIsolation}
	}

	var unsupportedScyllaArgs []string
	if len(scSpec.ScyllaArgs) != 0 {
		unsupportedScyllaArgs = strings.Split(scSpec.ScyllaArgs, " ")
	}

	var developerMode *bool
	if scSpec.DeveloperMode {
		developerMode = pointer.Bool(scSpec.DeveloperMode)
	}

	var network *scyllav2alpha1.Network
	if len(scSpec.Network.DNSPolicy) != 0 {
		network = &scyllav2alpha1.Network{
			DNSPolicy: scSpec.Network.DNSPolicy,
		}
	}

	datacenter, err := convertV1DatacenterSpecToV2alpha1(sc, scSpec, scSpec.Datacenter)
	if err != nil {
		return nil, fmt.Errorf("can't convert scyllav1.Datacenter to v2alpha1: %w", err)
	}

	return &scyllav2alpha1.ScyllaClusterSpec{
		Scylla: scyllav2alpha1.Scylla{
			Image:                 fmt.Sprintf("%s:%s", scSpec.Repository, scSpec.Version),
			AlternatorOptions:     alternatorOptions,
			UnsupportedScyllaArgs: unsupportedScyllaArgs,
			EnableDeveloperMode:   developerMode,
		},
		ScyllaManagerAgent: &scyllav2alpha1.ScyllaManagerAgent{
			Image: fmt.Sprintf("%s:%s", scSpec.AgentRepository, scSpec.AgentVersion),
		},
		ImagePullSecrets: scSpec.ImagePullSecrets,
		Network:          network,
		Datacenters:      []scyllav2alpha1.Datacenter{*datacenter},
	}, nil
}

func convertV1DatacenterSpecToV2alpha1(sc *scyllav1.ScyllaCluster, scSpec scyllav1.ScyllaClusterSpec, datacenter scyllav1.DatacenterSpec) (*scyllav2alpha1.Datacenter, error) {
	var nodesPerRack *int32
	var scylla *scyllav2alpha1.ScyllaOverrides
	var scyllaManagerAgent *scyllav2alpha1.ScyllaManagerAgentOverrides

	if len(datacenter.Racks) > 0 {
		rack := datacenter.Racks[0]
		nodesPerRack = pointer.Int32(rack.Members)

		storage := &scyllav2alpha1.Storage{
			Resources: corev1.ResourceRequirements{
				Requests: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceStorage: resource.MustParse(rack.Storage.Capacity),
				},
			},
			StorageClassName: rack.Storage.StorageClassName,
		}

		var customConfigMapRef *corev1.LocalObjectReference
		if len(rack.ScyllaConfig) > 0 {
			customConfigMapRef = &corev1.LocalObjectReference{Name: rack.ScyllaConfig}
		}

		scylla = &scyllav2alpha1.ScyllaOverrides{
			Resources:          rack.Resources.DeepCopy(),
			Storage:            storage,
			CustomConfigMapRef: customConfigMapRef,
		}

		var customSecretRef *corev1.LocalObjectReference
		if len(rack.ScyllaAgentConfig) > 0 {
			customSecretRef = &corev1.LocalObjectReference{Name: rack.ScyllaAgentConfig}
		}

		scyllaManagerAgent = &scyllav2alpha1.ScyllaManagerAgentOverrides{
			Resources:             rack.AgentResources.DeepCopy(),
			CustomConfigSecretRef: customSecretRef,
		}
	}

	var remoteKubeClusterConfigRef *scyllav2alpha1.RemoteKubeClusterConfigRef
	if v, ok := sc.Annotations[naming.ScyllaClusterV2alpha1RemoteKubeClusterConfigRefAnnotation]; ok {
		remoteKubeClusterConfigRef = &scyllav2alpha1.RemoteKubeClusterConfigRef{Name: v}
	}

	var racks []scyllav2alpha1.RackSpec
	for _, r := range datacenter.Racks {
		var placement *scyllav2alpha1.Placement
		if r.Placement != nil {
			placement = &scyllav2alpha1.Placement{
				NodeAffinity:    r.Placement.NodeAffinity,
				PodAffinity:     r.Placement.PodAffinity,
				PodAntiAffinity: r.Placement.PodAntiAffinity,
				Tolerations:     r.Placement.Tolerations,
			}
		}

		racks = append(racks, scyllav2alpha1.RackSpec{
			Name:      r.Name,
			Placement: placement,
		})
	}

	var exposeOptions *scyllav2alpha1.ExposeOptions
	if sc.Spec.ExposeOptions != nil {
		exposeOptions = &scyllav2alpha1.ExposeOptions{}
		if sc.Spec.ExposeOptions.CQL != nil {
			exposeOptions.CQL = &scyllav2alpha1.CQLExposeOptions{}
			if sc.Spec.ExposeOptions.CQL.Ingress != nil {
				exposeOptions.CQL.Ingress = &scyllav2alpha1.IngressOptions{
					Disabled:         sc.Spec.ExposeOptions.CQL.Ingress.Disabled,
					IngressClassName: sc.Spec.ExposeOptions.CQL.Ingress.IngressClassName,
					Annotations:      sc.Spec.ExposeOptions.CQL.Ingress.Annotations,
				}
			}
		}
	}

	return &scyllav2alpha1.Datacenter{
		Name: datacenter.Name,

		DNSDomains:    sc.Spec.DNSDomains,
		ExposeOptions: exposeOptions,

		RemoteKubeClusterConfigRef: remoteKubeClusterConfigRef,
		ForceRedeploymentReason:    scSpec.ForceRedeploymentReason,
		NodesPerRack:               nodesPerRack,
		Scylla:                     scylla,
		ScyllaManagerAgent:         scyllaManagerAgent,
		Racks:                      racks,
		Placement:                  nil,
	}, nil
}

func convertV1ScyllaClusterStatusToV2alpha1(sc *scyllav1.ScyllaCluster) (*scyllav2alpha1.ScyllaClusterStatus, error) {
	var nodes, updatedNodes, readyNodes int32
	var datacenters []scyllav2alpha1.DatacenterStatus

	if len(sc.Status.Racks) > 0 {
		var upgradeStatus *scyllav2alpha1.UpgradeStatus
		if sc.Status.Upgrade != nil {
			upgradeStatus = &scyllav2alpha1.UpgradeStatus{
				State:             sc.Status.Upgrade.State,
				FromVersion:       sc.Status.Upgrade.FromVersion,
				ToVersion:         sc.Status.Upgrade.ToVersion,
				SystemSnapshotTag: sc.Status.Upgrade.SystemSnapshotTag,
				DataSnapshotTag:   sc.Status.Upgrade.DataSnapshotTag,
			}
		}
		var dcStale *bool
		if v, ok := sc.Annotations[naming.ScyllaClusterV2alpha1ScyllaDatacenterStaleAnnotation]; ok {
			dcStale = pointer.Bool(v == "true")
		}
		var dcNodes, dcNodesUpdated, dcNodesReady int32
		var rackStatuses []scyllav2alpha1.RackStatus
		for _, rack := range sc.Spec.Datacenter.Racks {
			rackStatus, ok := sc.Status.Racks[rack.Name]
			if !ok {
				continue
			}

			if rackStatus.UpdatedMembers != nil {
				dcNodesUpdated += *rackStatus.UpdatedMembers
			}
			dcNodes += rackStatus.Members
			dcNodesReady += rackStatus.ReadyMembers

			var conditions []metav1.Condition
			for _, condition := range rackStatus.Conditions {
				c := metav1.Condition{
					Status: metav1.ConditionStatus(condition.Status),
				}
				switch condition.Type {
				case scyllav1.RackConditionTypeUpgrading:
					c.Type = scyllav2alpha1.RackConditionTypeUpgrading
				case scyllav1.RackConditionTypeMemberDecommissioning:
					c.Type = scyllav2alpha1.RackConditionTypeNodeDecommissioning
				case scyllav1.RackConditionTypeMemberReplacing:
					c.Type = scyllav2alpha1.RackConditionTypeNodeReplacing
				case scyllav1.RackConditionTypeMemberLeaving:
					c.Type = scyllav2alpha1.RackConditionTypeNodeLeaving
				default:
					klog.Warningf("Unsupported rack condition found during v1.ScyllaCluster to scyllav2alpha1.ScyllaCluster conversion %s", condition.Type)
				}

				conditions = append(conditions, c)
			}

			var currentVersion string
			if len(rackStatus.Version) != 0 {
				currentVersion = fmt.Sprintf("%s:%s", sc.Spec.Repository, rackStatus.Version)
			}

			rackStatuses = append(rackStatuses, scyllav2alpha1.RackStatus{
				Name:                    rack.Name,
				CurrentVersion:          currentVersion,
				Nodes:                   pointer.Int32(rackStatus.Members),
				UpdatedNodes:            rackStatus.UpdatedMembers,
				ReadyNodes:              pointer.Int32(rackStatus.ReadyMembers),
				Stale:                   rackStatus.Stale,
				Conditions:              conditions,
				ReplaceAddressFirstBoot: rackStatus.ReplaceAddressFirstBoot,
			})
		}

		currentVersion := sc.Status.Racks[sc.Spec.Datacenter.Racks[len(sc.Spec.Datacenter.Racks)-1].Name].Version
		if len(currentVersion) != 0 {
			currentVersion = fmt.Sprintf("%s:%s", sc.Spec.Repository, currentVersion)
		}

		conditions := make([]metav1.Condition, 0, len(sc.Status.Conditions))
		for _, cond := range sc.Status.Conditions {
			conditions = append(conditions, *cond.DeepCopy())
		}

		datacenters = append(datacenters, scyllav2alpha1.DatacenterStatus{
			Name:           sc.Spec.Datacenter.Name,
			CurrentVersion: currentVersion,
			Nodes:          pointer.Int32(dcNodes),
			UpdatedNodes:   pointer.Int32(dcNodesUpdated),
			ReadyNodes:     pointer.Int32(dcNodesReady),
			Stale:          dcStale,
			Conditions:     conditions,
			Racks:          rackStatuses,
			Upgrade:        upgradeStatus,
		})
	}

	if datacenters != nil {
		dc := datacenters[0]
		nodes = *dc.Nodes
		updatedNodes = *dc.UpdatedNodes
		readyNodes = *dc.ReadyNodes
	}

	var clusterConditions []metav1.Condition
	if v, ok := sc.Annotations[naming.ScyllaClusterV2alpha1ScyllaClusterConditionsAnnotation]; ok {
		if err := json.Unmarshal([]byte(v), &clusterConditions); err != nil {
			return nil, fmt.Errorf("can't unmarshal scylla cluster conditions from annotation %q: %w", v, err)
		}
	}

	return &scyllav2alpha1.ScyllaClusterStatus{
		ObservedGeneration: sc.Status.ObservedGeneration,
		Nodes:              pointer.Int32(nodes),
		UpdatedNodes:       pointer.Int32(updatedNodes),
		ReadyNodes:         pointer.Int32(readyNodes),
		Datacenters:        datacenters,
		Conditions:         clusterConditions,
	}, nil
}

func ScyllaClusterFromV1ToV2Alpha1(sc *scyllav1.ScyllaCluster) (*scyllav2alpha1.ScyllaCluster, error) {
	return convertV1ScyllaClusterToV2alpha1(sc.DeepCopy())
}

func convertV2alpha1ScyllaClusterSpecToV1(sc *scyllav2alpha1.ScyllaCluster, scSpec scyllav2alpha1.ScyllaClusterSpec) (*scyllav1.ScyllaClusterSpec, map[string]string, error) {
	var scyllaRepository, scyllaVersion string
	scyllaImageParts := strings.Split(scSpec.Scylla.Image, ":")
	if len(scyllaImageParts) != 2 {
		return nil, nil, fmt.Errorf("unsupported scylla image %q", scSpec.Scylla.Image)
	}
	scyllaRepository, scyllaVersion = scyllaImageParts[0], scyllaImageParts[1]

	var agentRepository, agentVersion string
	if scSpec.ScyllaManagerAgent != nil {
		agentImageParts := strings.Split(scSpec.ScyllaManagerAgent.Image, ":")
		if len(agentImageParts) != 2 {
			return nil, nil, fmt.Errorf("unsupported agent image %q", scSpec.ScyllaManagerAgent.Image)
		}
		agentRepository, agentVersion = agentImageParts[0], agentImageParts[1]
	}

	var alternatorSpec *scyllav1.AlternatorSpec
	if scSpec.Scylla.AlternatorOptions != nil {
		alternatorSpec = &scyllav1.AlternatorSpec{WriteIsolation: scSpec.Scylla.AlternatorOptions.WriteIsolation}
	}

	developerMode := false
	if scSpec.Scylla.EnableDeveloperMode != nil {
		developerMode = *scSpec.Scylla.EnableDeveloperMode
	}

	var dnsPolicy corev1.DNSPolicy
	if scSpec.Network != nil {
		dnsPolicy = scSpec.Network.DNSPolicy
	}

	forceRedeploymentReason := ""
	if len(scSpec.Datacenters) > 0 {
		forceRedeploymentReason = scSpec.Datacenters[0].ForceRedeploymentReason
	}

	var dnsDomains []string
	var exposeOptions *scyllav1.ExposeOptions
	if len(scSpec.Datacenters) > 0 {
		dc := scSpec.Datacenters[0]

		dnsDomains = dc.DNSDomains
		if dc.ExposeOptions != nil {
			exposeOptions = &scyllav1.ExposeOptions{}
			if dc.ExposeOptions.CQL != nil {
				exposeOptions.CQL = &scyllav1.CQLExposeOptions{}
				if dc.ExposeOptions.CQL.Ingress != nil {
					exposeOptions.CQL.Ingress = &scyllav1.IngressOptions{
						Disabled:         dc.ExposeOptions.CQL.Ingress.Disabled,
						IngressClassName: dc.ExposeOptions.CQL.Ingress.IngressClassName,
						Annotations:      dc.ExposeOptions.CQL.Ingress.Annotations,
					}
				}
			}
		}
	}

	var scv1 *scyllav1.ScyllaCluster
	if v, ok := sc.Annotations[naming.ScyllaClusterV1Annotation]; ok {
		scv1 = &scyllav1.ScyllaCluster{}
		if err := json.NewDecoder(bytes.NewBufferString(v)).Decode(scv1); err != nil {
			return nil, nil, fmt.Errorf("can't decode v1.ScyllaCluster from annotation: %w", err)
		}
	}

	var automaticOrphanedNodeCleanup bool
	var hostNetworking bool
	var sysctls []string
	var repairs []scyllav1.RepairTaskSpec
	var backups []scyllav1.BackupTaskSpec

	if scv1 != nil {
		repairs = scv1.Spec.Repairs
		backups = scv1.Spec.Backups
		hostNetworking = scv1.Spec.Network.HostNetworking
		automaticOrphanedNodeCleanup = scv1.Spec.AutomaticOrphanedNodeCleanup
		sysctls = scv1.Spec.Sysctls
	}

	dcSpec, customAnnotations, err := convertV2alpha1DatacenterToV1(sc, scSpec)
	if err != nil {
		return nil, nil, fmt.Errorf("can't convert scyllav2alpha1.Datacenter into v1: %w", err)
	}

	return &scyllav1.ScyllaClusterSpec{
		Version:                      scyllaVersion,
		Repository:                   scyllaRepository,
		Alternator:                   alternatorSpec,
		AgentVersion:                 agentVersion,
		AgentRepository:              agentRepository,
		DeveloperMode:                developerMode,
		CpuSet:                       false,
		AutomaticOrphanedNodeCleanup: automaticOrphanedNodeCleanup,
		GenericUpgrade:               nil,
		Datacenter:                   *dcSpec,
		Sysctls:                      sysctls,
		ScyllaArgs:                   strings.Join(scSpec.Scylla.UnsupportedScyllaArgs, " "),
		Network: scyllav1.Network{
			HostNetworking: hostNetworking,
			DNSPolicy:      dnsPolicy,
		},
		Repairs:                 repairs,
		Backups:                 backups,
		ForceRedeploymentReason: forceRedeploymentReason,
		ImagePullSecrets:        scSpec.ImagePullSecrets,
		DNSDomains:              dnsDomains,
		ExposeOptions:           exposeOptions,
	}, customAnnotations, nil
}

func convertV2alpha1DatacenterToV1(sc *scyllav2alpha1.ScyllaCluster, scSpec scyllav2alpha1.ScyllaClusterSpec) (*scyllav1.DatacenterSpec, map[string]string, error) {
	var annotations map[string]string
	var racks []scyllav1.RackSpec
	var name string

	if len(scSpec.Datacenters) > 0 {
		dc := scSpec.Datacenters[0]
		name = dc.Name
		nodesPerRack := *dc.NodesPerRack

		if dc.RemoteKubeClusterConfigRef != nil {
			if annotations == nil {
				annotations = map[string]string{}
			}
			annotations[naming.ScyllaClusterV2alpha1RemoteKubeClusterConfigRefAnnotation] = dc.RemoteKubeClusterConfigRef.Name
		}

		var scyllaResources, agentResources corev1.ResourceRequirements
		var scyllaConfigMapName, agentSecretName string
		if dc.Scylla != nil {
			scyllaResources = *dc.Scylla.Resources.DeepCopy()
			if dc.Scylla.CustomConfigMapRef != nil {
				scyllaConfigMapName = dc.Scylla.CustomConfigMapRef.Name
			}
		}
		if dc.ScyllaManagerAgent != nil {
			agentResources = *dc.ScyllaManagerAgent.Resources.DeepCopy()
			if dc.ScyllaManagerAgent.CustomConfigSecretRef != nil {
				agentSecretName = dc.ScyllaManagerAgent.CustomConfigSecretRef.Name
			}
		}

		var placementSpec *scyllav1.PlacementSpec
		if dc.Placement != nil {
			placementSpec = &scyllav1.PlacementSpec{
				NodeAffinity:    dc.Placement.NodeAffinity,
				PodAffinity:     dc.Placement.PodAffinity,
				PodAntiAffinity: dc.Placement.PodAntiAffinity,
				Tolerations:     dc.Placement.Tolerations,
			}
		}

		var storageSpec scyllav1.StorageSpec
		if dc.Scylla != nil && dc.Scylla.Storage != nil {
			capacity := ""
			if v, ok := dc.Scylla.Storage.Resources.Requests[corev1.ResourceStorage]; ok {
				capacity = v.String()
			}
			if v, ok := dc.Scylla.Storage.Resources.Limits[corev1.ResourceStorage]; ok {
				capacity = v.String()
			}
			storageSpec = scyllav1.StorageSpec{
				Capacity:         capacity,
				StorageClassName: dc.Scylla.Storage.StorageClassName,
			}
		}

		var scv1 *scyllav1.ScyllaCluster
		if v, ok := sc.Annotations[naming.ScyllaClusterV1Annotation]; ok {
			scv1 = &scyllav1.ScyllaCluster{}
			if err := json.NewDecoder(bytes.NewBufferString(v)).Decode(scv1); err != nil {
				return nil, nil, fmt.Errorf("can't decode v1.ScyllaCluster from annotation: %w", err)
			}
		}

		for i, rack := range dc.Racks {
			var volumes []corev1.Volume
			var scyllaVolumeMounts, agentVolumeMounts []corev1.VolumeMount

			if rack.Placement != nil {
				placementSpec = &scyllav1.PlacementSpec{
					NodeAffinity:    rack.Placement.NodeAffinity,
					PodAffinity:     rack.Placement.PodAffinity,
					PodAntiAffinity: rack.Placement.PodAntiAffinity,
					Tolerations:     rack.Placement.Tolerations,
				}
			}

			if scv1 != nil && len(scv1.Spec.Datacenter.Racks) > i {
				volumes = scv1.Spec.Datacenter.Racks[i].Volumes
				scyllaVolumeMounts = scv1.Spec.Datacenter.Racks[i].VolumeMounts
				agentVolumeMounts = scv1.Spec.Datacenter.Racks[i].AgentVolumeMounts

				if i > 0 {
					scyllaResources = scv1.Spec.Datacenter.Racks[i].Resources
					agentResources = scv1.Spec.Datacenter.Racks[i].AgentResources
					scyllaConfigMapName = scv1.Spec.Datacenter.Racks[i].ScyllaConfig
					agentSecretName = scv1.Spec.Datacenter.Racks[i].ScyllaAgentConfig
					placementSpec = scv1.Spec.Datacenter.Racks[i].Placement
					storageSpec = scv1.Spec.Datacenter.Racks[i].Storage
					nodesPerRack = scv1.Spec.Datacenter.Racks[i].Members
				}
			}

			racks = append(racks, scyllav1.RackSpec{
				Name:              rack.Name,
				Members:           nodesPerRack,
				Storage:           storageSpec,
				Placement:         placementSpec,
				Resources:         scyllaResources,
				AgentResources:    agentResources,
				Volumes:           volumes,
				VolumeMounts:      scyllaVolumeMounts,
				AgentVolumeMounts: agentVolumeMounts,
				ScyllaConfig:      scyllaConfigMapName,
				ScyllaAgentConfig: agentSecretName,
			})
		}
	}

	return &scyllav1.DatacenterSpec{
		Name:  name,
		Racks: racks,
	}, annotations, nil

}

func convertV2alpha1ScyllaClusterStatusToV1(sc *scyllav2alpha1.ScyllaCluster) (*scyllav1.ScyllaClusterStatus, map[string]string, error) {
	var annotations map[string]string
	var upgradeStatus *scyllav1.UpgradeStatus
	var rackStatuses map[string]scyllav1.RackStatus
	var conditions []metav1.Condition

	if len(sc.Status.Datacenters) > 0 {
		dcStatus := sc.Status.Datacenters[0]

		if dcStatus.Stale != nil {
			if annotations == nil {
				annotations = map[string]string{}
			}
			annotations[naming.ScyllaClusterV2alpha1ScyllaDatacenterStaleAnnotation] = fmt.Sprintf("%v", *dcStatus.Stale)
		}

		for _, cond := range dcStatus.Conditions {
			conditions = append(conditions, *cond.DeepCopy())
		}

		if dcStatus.Upgrade != nil {
			upgradeStatus = &scyllav1.UpgradeStatus{
				State:             dcStatus.Upgrade.State,
				FromVersion:       dcStatus.Upgrade.FromVersion,
				ToVersion:         dcStatus.Upgrade.ToVersion,
				SystemSnapshotTag: dcStatus.Upgrade.SystemSnapshotTag,
				DataSnapshotTag:   dcStatus.Upgrade.DataSnapshotTag,
			}
		}
	}

	for _, dcSpec := range sc.Spec.Datacenters {
		for _, dcStatus := range sc.Status.Datacenters {
			if dcStatus.Name != dcSpec.Name {
				continue
			}

			for i, rackStatus := range dcStatus.Racks {
				var rackConditions []scyllav1.RackCondition
				for _, rackCondition := range rackStatus.Conditions {
					rc := scyllav1.RackCondition{
						Status: corev1.ConditionStatus(rackCondition.Status),
					}
					switch rackCondition.Type {
					case scyllav2alpha1.RackConditionTypeNodeLeaving:
						rc.Type = scyllav1.RackConditionTypeMemberLeaving
					case scyllav2alpha1.RackConditionTypeUpgrading:
						rc.Type = scyllav1.RackConditionTypeUpgrading
					case scyllav2alpha1.RackConditionTypeNodeReplacing:
						rc.Type = scyllav1.RackConditionTypeMemberReplacing
					case scyllav2alpha1.RackConditionTypeNodeDecommissioning:
						rc.Type = scyllav1.RackConditionTypeMemberDecommissioning
					default:
						return nil, nil, fmt.Errorf("unknown rack condition: %q", rackCondition)
					}
					rackConditions = append(rackConditions, rc)
				}

				if rackStatuses == nil {
					rackStatuses = map[string]scyllav1.RackStatus{}
				}

				version := rackStatus.CurrentVersion
				versionParts := strings.Split(rackStatus.CurrentVersion, ":")
				if len(versionParts) == 2 {
					version = versionParts[1]
				}

				var rackStale *bool
				if rackStatus.Stale != nil {
					rackStale = pointer.Bool(*rackStatus.Stale)
				}
				if dcStatus.Stale != nil {
					rackStale = pointer.Bool((rackStale != nil && *rackStale) || *dcStatus.Stale)
				}

				rs := scyllav1.RackStatus{
					Version:                 version,
					UpdatedMembers:          rackStatus.UpdatedNodes,
					Stale:                   rackStale,
					Conditions:              rackConditions,
					ReplaceAddressFirstBoot: rackStatus.ReplaceAddressFirstBoot,
				}
				if rackStatus.Nodes != nil {
					rs.Members = *rackStatus.Nodes
				}
				if rackStatus.ReadyNodes != nil {
					rs.ReadyMembers = *rackStatus.ReadyNodes
				}
				rackStatuses[dcSpec.Racks[i].Name] = rs
			}
		}
	}

	var repairsStatus []scyllav1.RepairTaskStatus
	var backupsStatus []scyllav1.BackupTaskStatus
	var managerID *string

	var scv1 *scyllav1.ScyllaCluster
	if v, ok := sc.Annotations[naming.ScyllaClusterV1Annotation]; ok {
		scv1 = &scyllav1.ScyllaCluster{}
		if err := json.NewDecoder(bytes.NewBufferString(v)).Decode(scv1); err != nil {
			return nil, nil, fmt.Errorf("can't decode v1.ScyllaCluster from annotation: %w", err)
		}
	}

	if scv1 != nil {
		repairsStatus = scv1.Status.Repairs
		backupsStatus = scv1.Status.Backups
		managerID = scv1.Status.ManagerID
	}

	if sc.Status.Conditions != nil {
		if annotations == nil {
			annotations = map[string]string{}
		}

		buf, err := json.Marshal(sc.Status.Conditions)
		if err != nil {
			return nil, nil, fmt.Errorf("can't encode v2alpha1.ScyllaCluster status conditions: %w", err)
		}
		annotations[naming.ScyllaClusterV2alpha1ScyllaClusterConditionsAnnotation] = string(buf)
	}

	return &scyllav1.ScyllaClusterStatus{
		ObservedGeneration: sc.Status.ObservedGeneration,
		Racks:              rackStatuses,
		ManagerID:          managerID,
		Repairs:            repairsStatus,
		Backups:            backupsStatus,
		Upgrade:            upgradeStatus,
		Conditions:         conditions,
	}, annotations, nil
}

func convertV2alpha1ScyllaClusterToV1(sc *scyllav2alpha1.ScyllaCluster) (*scyllav1.ScyllaCluster, error) {
	spec, customAnnotations, err := convertV2alpha1ScyllaClusterSpecToV1(sc, sc.Spec)
	if err != nil {
		return nil, fmt.Errorf("can't convert v2alpha1.ScyllaClusterSpec to v1: %w", err)
	}

	status, customStatusAnnotations, err := convertV2alpha1ScyllaClusterStatusToV1(sc)
	if err != nil {
		return nil, fmt.Errorf("can't convert v2alpha1.ScyllaClusterStatus to v1: %w", err)
	}

	for _, annotations := range []map[string]string{customAnnotations, customStatusAnnotations} {
		if len(annotations) != 0 && sc.Annotations == nil {
			sc.Annotations = map[string]string{}
		}
		for k, v := range annotations {
			sc.Annotations[k] = v
		}
	}

	return &scyllav1.ScyllaCluster{
		ObjectMeta: sc.ObjectMeta,
		Spec:       *spec,
		Status:     *status,
	}, nil
}

func ScyllaClusterFromV2Alpha1oV1(sc *scyllav2alpha1.ScyllaCluster) (*scyllav1.ScyllaCluster, error) {
	return convertV2alpha1ScyllaClusterToV1(sc.DeepCopy())
}
