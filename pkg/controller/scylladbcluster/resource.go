package scylladbcluster

import (
	"fmt"
	"maps"
	"sort"
	"strings"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/helpers/slices"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	remotelister "github.com/scylladb/scylla-operator/pkg/remoteclient/lister"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/klog/v2"
)

func MakeRemoteRemoteOwners(sc *scyllav1alpha1.ScyllaDBCluster, remoteNamespaces map[string]*corev1.Namespace, existingRemoteOwners map[string]map[string]*scyllav1alpha1.RemoteOwner, managingClusterDomain string) ([]metav1.Condition, map[string][]*scyllav1alpha1.RemoteOwner, error) {
	var progressingConditions []metav1.Condition
	requiredRemoteOwners := make(map[string][]*scyllav1alpha1.RemoteOwner, len(sc.Spec.Datacenters))

	for _, dc := range sc.Spec.Datacenters {
		ns, ok := remoteNamespaces[dc.RemoteKubernetesClusterName]
		if !ok {
			progressingConditions = append(progressingConditions, metav1.Condition{
				Type:               remoteRemoteOwnerControllerProgressingCondition,
				Status:             metav1.ConditionTrue,
				Reason:             "WaitingForRemoteNamespace",
				Message:            fmt.Sprintf("Waiting for Namespace to be created in %q Cluster", dc.RemoteKubernetesClusterName),
				ObservedGeneration: sc.Generation,
			})
			return progressingConditions, nil, nil
		}

		nameSuffix, err := naming.GenerateNameHash(sc.Namespace, dc.Name)
		if err != nil {
			return nil, nil, fmt.Errorf("can't generate remoteowner name suffix: %w", err)
		}

		ro := &scyllav1alpha1.RemoteOwner{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: ns.Name,
				Name:      fmt.Sprintf("%s-%s", sc.Name, nameSuffix),
				Labels:    naming.RemoteOwnerLabels(sc, &dc, managingClusterDomain),
			},
		}

		requiredRemoteOwners[dc.RemoteKubernetesClusterName] = append(requiredRemoteOwners[dc.RemoteKubernetesClusterName], ro)
	}

	return progressingConditions, requiredRemoteOwners, nil
}

func MakeRemoteNamespaces(sc *scyllav1alpha1.ScyllaDBCluster, existingNamespaces map[string]map[string]*corev1.Namespace, managingClusterDomain string) (map[string][]*corev1.Namespace, error) {
	requiredNamespaces := make(map[string][]*corev1.Namespace)

	for _, dc := range sc.Spec.Datacenters {
		suffix, err := naming.GenerateNameHash(sc.Namespace, dc.Name)
		if err != nil {
			return nil, fmt.Errorf("can't generate namespace name suffix: %w", err)
		}

		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name:   fmt.Sprintf("%s-%s", sc.Namespace, suffix),
				Labels: naming.ScyllaDBClusterDatacenterLabels(sc, &dc, managingClusterDomain),
			},
		}

		requiredNamespaces[dc.RemoteKubernetesClusterName] = append(requiredNamespaces[dc.RemoteKubernetesClusterName], ns)
	}

	return requiredNamespaces, nil
}

func MakeRemoteServices(sc *scyllav1alpha1.ScyllaDBCluster, remoteNamespaces map[string]*corev1.Namespace, remoteControllers map[string]metav1.Object, managingClusterDomain string) ([]metav1.Condition, map[string][]*corev1.Service) {
	var progressingConditions []metav1.Condition
	remoteServices := make(map[string][]*corev1.Service, len(sc.Spec.Datacenters))

	for _, dc := range sc.Spec.Datacenters {
		dcProgressingConditions, remoteNamespace, remoteController := getRemoteNamespaceAndController(
			remoteServiceControllerProgressingCondition,
			sc,
			dc.RemoteKubernetesClusterName,
			remoteNamespaces,
			remoteControllers,
		)
		if len(dcProgressingConditions) > 0 {
			progressingConditions = append(progressingConditions, dcProgressingConditions...)
			continue
		}

		for _, otherDC := range sc.Spec.Datacenters {
			if dc.Name == otherDC.Name {
				continue
			}

			remoteServices[dc.RemoteKubernetesClusterName] = append(remoteServices[dc.RemoteKubernetesClusterName], &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:            naming.SeedService(sc, &otherDC),
					Namespace:       remoteNamespace.Name,
					Labels:          naming.ScyllaDBClusterDatacenterLabels(sc, &dc, managingClusterDomain),
					Annotations:     naming.ScyllaDBClusterDatacenterAnnotations(sc, &dc),
					OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(remoteController, remoteControllerGVK)},
				},
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{
							Name:     "inter-node",
							Protocol: corev1.ProtocolTCP,
							Port:     7000,
						},
						{
							Name:     "inter-node-ssl",
							Protocol: corev1.ProtocolTCP,
							Port:     7001,
						},
						{
							Name:     "cql",
							Protocol: corev1.ProtocolTCP,
							Port:     9042,
						},
						{
							Name:     "cql-ssl",
							Protocol: corev1.ProtocolTCP,
							Port:     9142,
						},
					},
					Selector:  nil,
					ClusterIP: corev1.ClusterIPNone,
					Type:      corev1.ServiceTypeClusterIP,
				},
			})
		}
	}

	return progressingConditions, remoteServices
}

func MakeRemoteScyllaDBDatacenters(sc *scyllav1alpha1.ScyllaDBCluster, remoteScyllaDBDatacenters map[string]map[string]*scyllav1alpha1.ScyllaDBDatacenter, remoteNamespaces map[string]*corev1.Namespace, remoteControllers map[string]metav1.Object, managingClusterDomain string) ([]metav1.Condition, map[string][]*scyllav1alpha1.ScyllaDBDatacenter, error) {
	var progressingConditions []metav1.Condition
	requiredRemoteScyllaDBDatacenters := make(map[string][]*scyllav1alpha1.ScyllaDBDatacenter, len(sc.Spec.Datacenters))

	// Given DC is part of seed list if it's fully reconciled, or is part of another DC seeds list,
	// meaning it was fully reconciled in the past, so DC is part of the cluster.
	seedDCNamesSet := sets.New[string]()
	for _, dc := range sc.Spec.Datacenters {
		sdc, ok := remoteScyllaDBDatacenters[dc.RemoteKubernetesClusterName][naming.ScyllaDBDatacenterName(sc, &dc)]
		if !ok {
			continue

		}
		dcIsRolledOut, err := controllerhelpers.IsScyllaDBDatacenterRolledOut(sdc)
		if err != nil {
			return progressingConditions, nil, fmt.Errorf("can't check if %q ScyllaDBDatacenter is rolled out: %w", naming.ObjRef(sdc), err)
		}
		if dcIsRolledOut {
			seedDCNamesSet.Insert(dc.Name)
			continue
		}

		for _, remoteSDCs := range remoteScyllaDBDatacenters {
			for _, remoteSDC := range remoteSDCs {
				isReferencedByOtherDCAsSeed := slices.Contains(remoteSDC.Spec.ScyllaDB.ExternalSeeds, func(remoteSDCSeed string) bool {
					return naming.DCNameFromSeedServiceAddress(sc, remoteSDCSeed, remoteSDC.Namespace) == dc.Name
				})

				if isReferencedByOtherDCAsSeed {
					seedDCNamesSet.Insert(dc.Name)
				}
			}
		}
	}

	for _, dcSpec := range sc.Spec.Datacenters {
		dcProgressingConditions, remoteNamespace, remoteController := getRemoteNamespaceAndController(
			remoteScyllaDBDatacenterControllerProgressingCondition,
			sc,
			dcSpec.RemoteKubernetesClusterName,
			remoteNamespaces,
			remoteControllers,
		)
		if len(dcProgressingConditions) > 0 {
			progressingConditions = append(progressingConditions, dcProgressingConditions...)
			continue
		}

		dc := applyDatacenterTemplateOnDatacenter(sc.Spec.DatacenterTemplate, &dcSpec)

		seeds := make([]string, 0, len(sc.Spec.ScyllaDB.ExternalSeeds)+seedDCNamesSet.Len())
		seeds = append(seeds, sc.Spec.ScyllaDB.ExternalSeeds...)

		for _, otherDC := range sc.Spec.Datacenters {
			if otherDC.Name == dcSpec.Name {
				continue
			}

			if seedDCNamesSet.Has(otherDC.Name) {
				seeds = append(seeds, fmt.Sprintf("%s.%s.svc", naming.SeedService(sc, &otherDC), remoteNamespace.Name))
			}
		}

		requiredRemoteScyllaDBDatacenters[dc.RemoteKubernetesClusterName] = append(requiredRemoteScyllaDBDatacenters[dc.RemoteKubernetesClusterName], &scyllav1alpha1.ScyllaDBDatacenter{
			ObjectMeta: metav1.ObjectMeta{
				Name:            naming.ScyllaDBDatacenterName(sc, dc),
				Namespace:       remoteNamespace.Name,
				Labels:          naming.ScyllaDBClusterDatacenterLabels(sc, dc, managingClusterDomain),
				Annotations:     naming.ScyllaDBClusterDatacenterAnnotations(sc, dc),
				OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(remoteController, remoteControllerGVK)},
			},
			Spec: scyllav1alpha1.ScyllaDBDatacenterSpec{
				Metadata: func() *scyllav1alpha1.ObjectTemplateMetadata {
					var metadata *scyllav1alpha1.ObjectTemplateMetadata
					if sc.Spec.Metadata != nil {
						metadata = &scyllav1alpha1.ObjectTemplateMetadata{
							Labels:      map[string]string{},
							Annotations: map[string]string{},
						}
						maps.Copy(metadata.Labels, sc.Spec.Metadata.Labels)
						maps.Copy(metadata.Annotations, sc.Spec.Metadata.Annotations)
					}

					if dc.Metadata != nil {
						if metadata == nil {
							metadata = &scyllav1alpha1.ObjectTemplateMetadata{
								Labels:      map[string]string{},
								Annotations: map[string]string{},
							}
						}
						maps.Copy(metadata.Labels, dc.Metadata.Labels)
						maps.Copy(metadata.Annotations, dc.Metadata.Annotations)
					}

					if metadata == nil {
						metadata = &scyllav1alpha1.ObjectTemplateMetadata{
							Labels:      map[string]string{},
							Annotations: map[string]string{},
						}
					}

					metadata.Labels[naming.ParentClusterNameLabel] = sc.Name
					metadata.Labels[naming.ParentClusterNamespaceLabel] = sc.Namespace

					return metadata
				}(),
				ClusterName:    sc.Name,
				DatacenterName: pointer.Ptr(dc.Name),
				ScyllaDB: scyllav1alpha1.ScyllaDB{
					Image:                       sc.Spec.ScyllaDB.Image,
					ExternalSeeds:               seeds,
					AlternatorOptions:           sc.Spec.ScyllaDB.AlternatorOptions,
					AdditionalScyllaDBArguments: sc.Spec.ScyllaDB.AdditionalScyllaDBArguments,
					EnableDeveloperMode:         sc.Spec.ScyllaDB.EnableDeveloperMode,
				},
				ScyllaDBManagerAgent: &scyllav1alpha1.ScyllaDBManagerAgent{
					Image: func() *string {
						var image *string
						if sc.Spec.ScyllaDBManagerAgent != nil {
							image = sc.Spec.ScyllaDBManagerAgent.Image
						}
						return image
					}(),
				},
				ForceRedeploymentReason: func() *string {
					sb := strings.Builder{}
					if sc.Spec.ForceRedeploymentReason != nil {
						sb.WriteString(*sc.Spec.ForceRedeploymentReason)
					}

					if sc.Spec.ForceRedeploymentReason != nil && dc.ForceRedeploymentReason != nil {
						sb.WriteByte(',')
					}

					if dc.ForceRedeploymentReason != nil {
						sb.WriteString(*dc.ForceRedeploymentReason)
					}
					if sb.Len() == 0 {
						return nil
					}
					return pointer.Ptr(sb.String())
				}(),
				ExposeOptions: func() *scyllav1alpha1.ExposeOptions {
					exposeOptions := &scyllav1alpha1.ExposeOptions{
						// TODO: not supported yet
						// Ref: https://github.com/scylladb/scylla-operator-enterprise/issues/55
						CQL: nil,
						NodeService: &scyllav1alpha1.NodeServiceTemplate{
							Type: scyllav1alpha1.NodeServiceTypeHeadless,
						},
						BroadcastOptions: &scyllav1alpha1.NodeBroadcastOptions{
							Nodes: scyllav1alpha1.BroadcastOptions{
								Type: scyllav1alpha1.BroadcastAddressTypePodIP,
							},
							Clients: scyllav1alpha1.BroadcastOptions{
								Type: scyllav1alpha1.BroadcastAddressTypePodIP,
							},
						},
					}

					if sc.Spec.ExposeOptions != nil {
						if sc.Spec.ExposeOptions.NodeService != nil {
							exposeOptions.NodeService = sc.Spec.ExposeOptions.NodeService
						}

						if sc.Spec.ExposeOptions.BroadcastOptions != nil {
							exposeOptions.BroadcastOptions.Nodes = scyllav1alpha1.BroadcastOptions{
								Type:  sc.Spec.ExposeOptions.BroadcastOptions.Nodes.Type,
								PodIP: sc.Spec.ExposeOptions.BroadcastOptions.Nodes.PodIP,
							}
							exposeOptions.BroadcastOptions.Clients = scyllav1alpha1.BroadcastOptions{
								Type:  sc.Spec.ExposeOptions.BroadcastOptions.Clients.Type,
								PodIP: sc.Spec.ExposeOptions.BroadcastOptions.Clients.PodIP,
							}
						}
						return exposeOptions
					}

					return exposeOptions
				}(),
				RackTemplate:                            dc.RackTemplate,
				Racks:                                   dc.Racks,
				DisableAutomaticOrphanedNodeReplacement: pointer.Ptr(sc.Spec.DisableAutomaticOrphanedNodeReplacement),
				MinTerminationGracePeriodSeconds:        sc.Spec.MinTerminationGracePeriodSeconds,
				MinReadySeconds:                         sc.Spec.MinReadySeconds,
				ReadinessGates:                          sc.Spec.ReadinessGates,
				// TODO: not supported yet
				// Ref: https://github.com/scylladb/scylla-operator-enterprise/issues/56
				ImagePullSecrets: nil,
				// TODO not supported yet:
				// Ref: https://github.com/scylladb/scylla-operator-enterprise/issues/57
				DNSPolicy: nil,
				// TODO not supported yet:
				// Ref: https://github.com/scylladb/scylla-operator-enterprise/issues/55
				DNSDomains: nil,
			},
		})
	}

	return progressingConditions, requiredRemoteScyllaDBDatacenters, nil
}

func MakeRemoteEndpointSlices(sc *scyllav1alpha1.ScyllaDBCluster, remoteNamespaces map[string]*corev1.Namespace, remoteControllers map[string]metav1.Object, remoteServiceLister remotelister.GenericClusterLister[corev1listers.ServiceLister], remotePodLister remotelister.GenericClusterLister[corev1listers.PodLister], managingClusterDomain string) ([]metav1.Condition, map[string][]*discoveryv1.EndpointSlice, error) {
	var progressingConditions []metav1.Condition
	remoteEndpointSlices := make(map[string][]*discoveryv1.EndpointSlice, len(sc.Spec.Datacenters))

	nodeBroadcastType := scyllav1alpha1.BroadcastAddressTypePodIP
	if sc.Spec.ExposeOptions != nil && sc.Spec.ExposeOptions.BroadcastOptions != nil {
		nodeBroadcastType = sc.Spec.ExposeOptions.BroadcastOptions.Nodes.Type
	}

	for _, dc := range sc.Spec.Datacenters {
		dcProgressingConditions, remoteNamespace, remoteController := getRemoteNamespaceAndController(
			remoteEndpointSliceControllerProgressingCondition,
			sc,
			dc.RemoteKubernetesClusterName,
			remoteNamespaces,
			remoteControllers,
		)
		if len(dcProgressingConditions) > 0 {
			progressingConditions = append(progressingConditions, dcProgressingConditions...)
			continue
		}

		for _, otherDC := range sc.Spec.Datacenters {
			if dc.Name == otherDC.Name {
				continue
			}

			dcLabels := naming.ScyllaDBClusterDatacenterEndpointsLabels(sc, dc, managingClusterDomain)
			dcLabels[discoveryv1.LabelServiceName] = naming.SeedService(sc, &otherDC)
			dcLabels[discoveryv1.LabelManagedBy] = naming.OperatorAppNameWithDomain

			dcEs := &discoveryv1.EndpointSlice{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:       remoteNamespace.Name,
					Name:            naming.SeedService(sc, &otherDC),
					Labels:          dcLabels,
					Annotations:     naming.ScyllaDBClusterDatacenterAnnotations(sc, &dc),
					OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(remoteController, remoteControllerGVK)},
				},
				AddressType: discoveryv1.AddressTypeIPv4,
				Ports: []discoveryv1.EndpointPort{
					{
						Name:     pointer.Ptr("inter-node"),
						Protocol: pointer.Ptr(corev1.ProtocolTCP),
						Port:     pointer.Ptr(int32(7000)),
					},
					{
						Name:     pointer.Ptr("inter-node-ssl"),
						Protocol: pointer.Ptr(corev1.ProtocolTCP),
						Port:     pointer.Ptr(int32(7001)),
					},
					{
						Name:     pointer.Ptr("cql"),
						Protocol: pointer.Ptr(corev1.ProtocolTCP),
						Port:     pointer.Ptr(int32(9042)),
					},
					{
						Name:     pointer.Ptr("cql-ssl"),
						Protocol: pointer.Ptr(corev1.ProtocolTCP),
						Port:     pointer.Ptr(int32(9142)),
					},
				},
			}

			otherDCPodSelector := naming.DatacenterPodsSelector(sc, &otherDC)
			otherDCNamespace, ok := remoteNamespaces[otherDC.RemoteKubernetesClusterName]
			if !ok {
				progressingConditions = append(progressingConditions, metav1.Condition{
					Type:               remoteEndpointSliceControllerProgressingCondition,
					Status:             metav1.ConditionTrue,
					Reason:             "WaitingForRemoteNamespace",
					Message:            fmt.Sprintf("Waiting for Namespace to be created in %q Cluster", otherDC.RemoteKubernetesClusterName),
					ObservedGeneration: sc.Generation,
				})
				continue
			}

			switch nodeBroadcastType {
			case scyllav1alpha1.BroadcastAddressTypePodIP:
				dcPods, err := remotePodLister.Cluster(otherDC.RemoteKubernetesClusterName).Pods(otherDCNamespace.Name).List(otherDCPodSelector)
				if err != nil {
					return progressingConditions, nil, fmt.Errorf("can't list pods in %q ScyllaCluster %q Datacenter: %w", naming.ObjRef(sc), otherDC.Name, err)
				}

				klog.V(4).InfoS("Found remote Scylla Pods", "Cluster", klog.KObj(sc), "Datacenter", otherDC.Name, "Pods", len(dcPods))

				// Sort pods to have stable list of endpoints
				sort.Slice(dcPods, func(i, j int) bool {
					return dcPods[i].Name < dcPods[j].Name
				})
				for _, dcPod := range dcPods {
					if len(dcPod.Status.PodIP) == 0 {
						continue
					}

					ready := controllerhelpers.IsPodReady(dcPod)
					terminating := dcPod.DeletionTimestamp != nil
					serving := ready && !terminating

					dcEs.Endpoints = append(dcEs.Endpoints, discoveryv1.Endpoint{
						Addresses: []string{dcPod.Status.PodIP},
						Conditions: discoveryv1.EndpointConditions{
							Ready:       pointer.Ptr(ready),
							Serving:     pointer.Ptr(serving),
							Terminating: pointer.Ptr(terminating),
						},
					})
				}

			case scyllav1alpha1.BroadcastAddressTypeServiceLoadBalancerIngress:
				dcServices, err := remoteServiceLister.Cluster(otherDC.RemoteKubernetesClusterName).Services(otherDCNamespace.Name).List(otherDCPodSelector)
				if err != nil {
					return progressingConditions, nil, fmt.Errorf("can't list services in %q ScyllaDBCluster %q Datacenter: %w", naming.ObjRef(sc), otherDC.Name, err)
				}

				klog.V(4).InfoS("Found remote ScyllaDB Services", "ScyllaDBCluster", klog.KObj(sc), "Datacenter", otherDC.Name, "Services", len(dcServices))

				// Sort objects to have stable list of endpoints
				sort.Slice(dcServices, func(i, j int) bool {
					return dcServices[i].Name < dcServices[j].Name
				})

				for _, dcService := range dcServices {
					if dcService.Labels[naming.ScyllaServiceTypeLabel] != string(naming.ScyllaServiceTypeMember) {
						continue
					}

					if len(dcService.Status.LoadBalancer.Ingress) < 1 {
						continue
					}

					for _, ingress := range dcService.Status.LoadBalancer.Ingress {
						ep := discoveryv1.Endpoint{
							Conditions: discoveryv1.EndpointConditions{
								Terminating: pointer.Ptr(dcService.DeletionTimestamp != nil),
							},
						}
						if len(ingress.IP) != 0 {
							ep.Addresses = append(ep.Addresses, ingress.IP)
						}

						if len(ingress.Hostname) != 0 {
							ep.Addresses = append(ep.Addresses, ingress.Hostname)
						}

						if len(ep.Addresses) > 0 {
							// LoadBalancer services are external to Kubernetes, and they don't report their readiness.
							// Assume that if the address is there, it's ready and serving.
							ep.Conditions.Ready = pointer.Ptr(true)
							ep.Conditions.Serving = pointer.Ptr(true)
						}

						dcEs.Endpoints = append(dcEs.Endpoints, ep)
					}
				}

			default:
				return progressingConditions, nil, fmt.Errorf("unsupported node broadcast address type %v specified in %q ScyllaDBCluster", nodeBroadcastType, naming.ObjRef(sc))
			}

			remoteEndpointSlices[dc.RemoteKubernetesClusterName] = append(remoteEndpointSlices[dc.RemoteKubernetesClusterName], dcEs)
		}
	}

	return progressingConditions, remoteEndpointSlices, nil
}

func mergeScyllaV1Alpha1Placement(placementGetters ...func() *scyllav1alpha1.Placement) *scyllav1alpha1.Placement {
	placementGetters = slices.FilterOut(placementGetters, func(getter func() *scyllav1alpha1.Placement) bool {
		return getter() == nil
	})
	if len(placementGetters) == 0 {
		return nil
	}

	placement := &scyllav1alpha1.Placement{}

	for _, pg := range placementGetters {
		placementTemplate := pg()

		if placementTemplate.NodeAffinity != nil {
			if placement.NodeAffinity == nil {
				placement.NodeAffinity = &corev1.NodeAffinity{}
			}

			if placementTemplate.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution != nil {
				if placement.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution == nil {
					placement.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution = &corev1.NodeSelector{}
				}

				placement.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms = append(placement.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms, placementTemplate.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms...)
			}
			if placementTemplate.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution != nil {
				placement.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution = append(placement.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution, placementTemplate.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution...)
			}
		}

		if placementTemplate.PodAffinity != nil {
			if placement.PodAffinity == nil {
				placement.PodAffinity = &corev1.PodAffinity{}
			}

			if placementTemplate.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution != nil {
				placement.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution = append(placement.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution, placementTemplate.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution...)
			}
			if placementTemplate.PodAffinity.PreferredDuringSchedulingIgnoredDuringExecution != nil {
				placement.PodAffinity.PreferredDuringSchedulingIgnoredDuringExecution = append(placement.PodAffinity.PreferredDuringSchedulingIgnoredDuringExecution, placementTemplate.PodAffinity.PreferredDuringSchedulingIgnoredDuringExecution...)
			}
		}

		if placementTemplate.PodAntiAffinity != nil {
			if placement.PodAntiAffinity == nil {
				placement.PodAntiAffinity = &corev1.PodAntiAffinity{}
			}
			if placementTemplate.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution != nil {
				placement.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution = append(placement.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution, placementTemplate.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution...)
			}
			if placementTemplate.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution != nil {
				placement.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution = append(placement.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution, placementTemplate.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution...)
			}
		}

		if placementTemplate.Tolerations != nil {
			placement.Tolerations = append(placement.Tolerations, placementTemplate.Tolerations...)
		}
	}

	return placement
}

func mergeScyllaV1Alpha1ScyllaDB(scyllaDBTemplateGetters ...func() *scyllav1alpha1.ScyllaDBTemplate) *scyllav1alpha1.ScyllaDBTemplate {
	scyllaDBTemplateGetters = slices.FilterOut(scyllaDBTemplateGetters, func(getter func() *scyllav1alpha1.ScyllaDBTemplate) bool {
		return getter() == nil
	})
	if len(scyllaDBTemplateGetters) == 0 {
		return nil
	}

	scyllaDBTemplate := &scyllav1alpha1.ScyllaDBTemplate{}

	for _, templateGetter := range scyllaDBTemplateGetters {
		template := templateGetter()

		if template.Resources != nil {
			if scyllaDBTemplate.Resources == nil {
				scyllaDBTemplate.Resources = &corev1.ResourceRequirements{
					Limits:   make(corev1.ResourceList),
					Requests: make(corev1.ResourceList),
				}
			}

			maps.Copy(scyllaDBTemplate.Resources.Limits, template.Resources.Limits)
			maps.Copy(scyllaDBTemplate.Resources.Requests, template.Resources.Requests)
		}

		if template.Storage != nil {
			if scyllaDBTemplate.Storage == nil {
				scyllaDBTemplate.Storage = &scyllav1alpha1.StorageOptions{}
			}

			if template.Storage.Metadata != nil {
				if scyllaDBTemplate.Storage.Metadata == nil {
					scyllaDBTemplate.Storage.Metadata = &scyllav1alpha1.ObjectTemplateMetadata{
						Labels:      make(map[string]string),
						Annotations: make(map[string]string),
					}
				}

				maps.Copy(scyllaDBTemplate.Storage.Metadata.Labels, template.Storage.Metadata.Labels)
				maps.Copy(scyllaDBTemplate.Storage.Metadata.Annotations, template.Storage.Metadata.Annotations)
			}

			if template.Storage.StorageClassName != nil {
				scyllaDBTemplate.Storage.StorageClassName = template.Storage.StorageClassName
			}

			if len(template.Storage.Capacity) > 0 {
				scyllaDBTemplate.Storage.Capacity = template.Storage.Capacity
			}
		}

		if template.CustomConfigMapRef != nil {
			scyllaDBTemplate.CustomConfigMapRef = template.CustomConfigMapRef
		}

		scyllaDBTemplate.Volumes = append(scyllaDBTemplate.Volumes, template.Volumes...)
		scyllaDBTemplate.VolumeMounts = append(scyllaDBTemplate.VolumeMounts, template.VolumeMounts...)
	}

	return scyllaDBTemplate
}

func mergeScyllaV1Alpha1ScyllaDBManagerAgent(scyllaDBManagerAgentTemplateGetters ...func() *scyllav1alpha1.ScyllaDBManagerAgentTemplate) *scyllav1alpha1.ScyllaDBManagerAgentTemplate {
	scyllaDBManagerAgentTemplateGetters = slices.FilterOut(scyllaDBManagerAgentTemplateGetters, func(getter func() *scyllav1alpha1.ScyllaDBManagerAgentTemplate) bool {
		return getter() == nil
	})
	if len(scyllaDBManagerAgentTemplateGetters) == 0 {
		return nil
	}

	scyllaDBManagerAgentTemplate := &scyllav1alpha1.ScyllaDBManagerAgentTemplate{}

	for _, templateGetter := range scyllaDBManagerAgentTemplateGetters {
		template := templateGetter()

		if template.Resources != nil {
			if scyllaDBManagerAgentTemplate.Resources == nil {
				scyllaDBManagerAgentTemplate.Resources = &corev1.ResourceRequirements{
					Limits:   make(corev1.ResourceList),
					Requests: make(corev1.ResourceList),
				}
			}

			maps.Copy(scyllaDBManagerAgentTemplate.Resources.Limits, template.Resources.Limits)
			maps.Copy(scyllaDBManagerAgentTemplate.Resources.Requests, template.Resources.Requests)
		}

		if template.CustomConfigSecretRef != nil {
			scyllaDBManagerAgentTemplate.CustomConfigSecretRef = template.CustomConfigSecretRef
		}

		scyllaDBManagerAgentTemplate.Volumes = append(scyllaDBManagerAgentTemplate.Volumes, template.Volumes...)
		scyllaDBManagerAgentTemplate.VolumeMounts = append(scyllaDBManagerAgentTemplate.VolumeMounts, template.VolumeMounts...)
	}

	return scyllaDBManagerAgentTemplate
}

func applyDatacenterTemplateOnDatacenter(dcTemplate *scyllav1alpha1.ScyllaDBClusterDatacenterTemplate, dc *scyllav1alpha1.ScyllaDBClusterDatacenter) *scyllav1alpha1.ScyllaDBClusterDatacenter {
	if dcTemplate == nil {
		return dc
	}

	return &scyllav1alpha1.ScyllaDBClusterDatacenter{
		ScyllaDBClusterDatacenterTemplate: scyllav1alpha1.ScyllaDBClusterDatacenterTemplate{
			Metadata: func() *scyllav1alpha1.ObjectTemplateMetadata {
				var metadata *scyllav1alpha1.ObjectTemplateMetadata

				if dcTemplate.Metadata != nil {
					if metadata == nil {
						metadata = &scyllav1alpha1.ObjectTemplateMetadata{
							Labels:      map[string]string{},
							Annotations: map[string]string{},
						}
					}
					maps.Copy(metadata.Labels, dcTemplate.Metadata.Labels)
					maps.Copy(metadata.Annotations, dcTemplate.Metadata.Annotations)
				}

				if dc.Metadata != nil {
					if metadata == nil {
						metadata = &scyllav1alpha1.ObjectTemplateMetadata{
							Labels:      map[string]string{},
							Annotations: map[string]string{},
						}
					}
					maps.Copy(metadata.Labels, dc.Metadata.Labels)
					maps.Copy(metadata.Annotations, dc.Metadata.Annotations)
				}

				return metadata
			}(),
			Placement: func() *scyllav1alpha1.Placement {
				placement := mergeScyllaV1Alpha1Placement(
					func() *scyllav1alpha1.Placement {
						return dcTemplate.Placement
					},
					func() *scyllav1alpha1.Placement {
						return dc.Placement
					},
				)

				if placement != nil {
					return placement
				}

				topologyLabelSelector := make(map[string]string)
				maps.Copy(topologyLabelSelector, dcTemplate.TopologyLabelSelector)
				maps.Copy(topologyLabelSelector, dc.TopologyLabelSelector)

				if len(topologyLabelSelector) == 0 {
					return nil
				}

				return &scyllav1alpha1.Placement{
					NodeAffinity: &corev1.NodeAffinity{
						RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
							NodeSelectorTerms: []corev1.NodeSelectorTerm{
								{
									MatchExpressions: func() []corev1.NodeSelectorRequirement {
										var reqs []corev1.NodeSelectorRequirement
										for k, v := range topologyLabelSelector {
											reqs = append(reqs, corev1.NodeSelectorRequirement{
												Key:      k,
												Operator: corev1.NodeSelectorOpIn,
												Values:   []string{v},
											})
										}
										return reqs
									}(),
								},
							},
						},
					},
				}
			}(),
			ScyllaDB: mergeScyllaV1Alpha1ScyllaDB(
				func() *scyllav1alpha1.ScyllaDBTemplate {
					return dcTemplate.ScyllaDB
				},
				func() *scyllav1alpha1.ScyllaDBTemplate {
					return dc.ScyllaDB
				},
			),
			ScyllaDBManagerAgent: mergeScyllaV1Alpha1ScyllaDBManagerAgent(
				func() *scyllav1alpha1.ScyllaDBManagerAgentTemplate {
					return dcTemplate.ScyllaDBManagerAgent
				},
				func() *scyllav1alpha1.ScyllaDBManagerAgentTemplate {
					return dc.ScyllaDBManagerAgent
				},
			),
			RackTemplate: func() *scyllav1alpha1.RackTemplate {
				if dcTemplate.RackTemplate == nil && dc.RackTemplate == nil {
					return nil
				}
				return &scyllav1alpha1.RackTemplate{
					Nodes: func() *int32 {
						if dc.RackTemplate != nil && dc.RackTemplate.Nodes != nil {
							return dc.RackTemplate.Nodes
						}
						if dcTemplate.RackTemplate != nil && dcTemplate.RackTemplate.Nodes != nil {
							return dcTemplate.RackTemplate.Nodes
						}
						return nil
					}(),
					Placement: mergeScyllaV1Alpha1Placement(
						func() *scyllav1alpha1.Placement {
							return dcTemplate.Placement
						},
						func() *scyllav1alpha1.Placement {
							if dcTemplate.RackTemplate != nil {
								return dcTemplate.RackTemplate.Placement
							}
							return nil
						},
						func() *scyllav1alpha1.Placement {
							return dc.Placement
						},
						func() *scyllav1alpha1.Placement {
							if dc.RackTemplate != nil {
								return dc.RackTemplate.Placement
							}
							return nil
						},
					),
					TopologyLabelSelector: func() map[string]string {
						topologyLabelSelector := map[string]string{}

						if dcTemplate.RackTemplate != nil && dcTemplate.RackTemplate.TopologyLabelSelector != nil {
							maps.Copy(topologyLabelSelector, dcTemplate.RackTemplate.TopologyLabelSelector)
						}

						if dcTemplate.TopologyLabelSelector != nil {
							maps.Copy(topologyLabelSelector, dcTemplate.TopologyLabelSelector)
						}

						if dc.TopologyLabelSelector != nil {
							maps.Copy(topologyLabelSelector, dc.TopologyLabelSelector)
						}

						if dc.RackTemplate != nil && dc.RackTemplate.TopologyLabelSelector != nil {
							maps.Copy(topologyLabelSelector, dc.RackTemplate.TopologyLabelSelector)
						}

						return topologyLabelSelector
					}(),
					ScyllaDB: mergeScyllaV1Alpha1ScyllaDB(
						func() *scyllav1alpha1.ScyllaDBTemplate {
							return dcTemplate.ScyllaDB
						},
						func() *scyllav1alpha1.ScyllaDBTemplate {
							if dcTemplate.RackTemplate != nil {
								return dcTemplate.RackTemplate.ScyllaDB
							}
							return nil
						},
						func() *scyllav1alpha1.ScyllaDBTemplate {
							return dc.ScyllaDB
						},
						func() *scyllav1alpha1.ScyllaDBTemplate {
							if dc.RackTemplate != nil {
								return dc.RackTemplate.ScyllaDB
							}
							return nil
						},
					),
					ScyllaDBManagerAgent: mergeScyllaV1Alpha1ScyllaDBManagerAgent(
						func() *scyllav1alpha1.ScyllaDBManagerAgentTemplate {
							return dcTemplate.ScyllaDBManagerAgent
						},
						func() *scyllav1alpha1.ScyllaDBManagerAgentTemplate {
							if dcTemplate.RackTemplate != nil {
								return dcTemplate.RackTemplate.ScyllaDBManagerAgent
							}
							return nil
						},
						func() *scyllav1alpha1.ScyllaDBManagerAgentTemplate {
							return dc.ScyllaDBManagerAgent
						},
						func() *scyllav1alpha1.ScyllaDBManagerAgentTemplate {
							if dc.RackTemplate != nil {
								return dc.RackTemplate.ScyllaDBManagerAgent
							}
							return nil
						},
					),
				}
			}(),
			Racks: func() []scyllav1alpha1.RackSpec {
				var racks []scyllav1alpha1.RackSpec

				if dcTemplate.Racks != nil {
					racks = dcTemplate.Racks
				}

				if dc.Racks != nil {
					racks = dc.Racks
				}

				return racks
			}(),
		},
		Name:                        dc.Name,
		RemoteKubernetesClusterName: dc.RemoteKubernetesClusterName,
		ForceRedeploymentReason:     dc.ForceRedeploymentReason,
	}
}

func getRemoteNamespaceAndController(controllerProgressingType string, sc *scyllav1alpha1.ScyllaDBCluster, clusterName string, remoteNamespaces map[string]*corev1.Namespace, remoteControllers map[string]metav1.Object) ([]metav1.Condition, *corev1.Namespace, metav1.Object) {
	var progressingConditions []metav1.Condition
	remoteNamespace, ok := remoteNamespaces[clusterName]
	if !ok {
		progressingConditions = append(progressingConditions, metav1.Condition{
			Type:               controllerProgressingType,
			Status:             metav1.ConditionTrue,
			Reason:             "WaitingForRemoteNamespace",
			Message:            fmt.Sprintf("Waiting for Namespace to be created in %q Cluster", clusterName),
			ObservedGeneration: sc.Generation,
		})
		return progressingConditions, nil, nil
	}

	remoteController, ok := remoteControllers[clusterName]
	if !ok {
		progressingConditions = append(progressingConditions, metav1.Condition{
			Type:               controllerProgressingType,
			Status:             metav1.ConditionTrue,
			Reason:             "WaitingForRemoteController",
			Message:            fmt.Sprintf("Waiting for controller object to be created in %q Cluster", clusterName),
			ObservedGeneration: sc.Generation,
		})
		return progressingConditions, nil, nil
	}

	return progressingConditions, remoteNamespace, remoteController
}
