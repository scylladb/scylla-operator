package scylladbcluster

import (
	"fmt"
	"maps"
	"sort"
	"strings"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	oslices "github.com/scylladb/scylla-operator/pkg/helpers/slices"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	remotelister "github.com/scylladb/scylla-operator/pkg/remoteclient/lister"
	"github.com/scylladb/scylla-operator/pkg/scylla"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	apimachineryutilerrors "k8s.io/apimachinery/pkg/util/errors"
	apimachineryutilsets "k8s.io/apimachinery/pkg/util/sets"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/klog/v2"
)

func MakeRemoteRemoteOwners(sc *scyllav1alpha1.ScyllaDBCluster, dc *scyllav1alpha1.ScyllaDBClusterDatacenter, remoteNamespace *corev1.Namespace, managingClusterDomain string) ([]*scyllav1alpha1.RemoteOwner, error) {
	nameSuffix, err := naming.GenerateNameHash(sc.Namespace, dc.Name)
	if err != nil {
		return nil, fmt.Errorf("can't generate remoteowner name suffix: %w", err)
	}

	requiredRemoteOwners := []*scyllav1alpha1.RemoteOwner{
		{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: remoteNamespace.Name,
				Name:      fmt.Sprintf("%s-%s", sc.Name, nameSuffix),
				Labels:    naming.RemoteOwnerLabels(sc, dc, managingClusterDomain),
			},
		},
	}

	return requiredRemoteOwners, nil
}

func MakeRemoteNamespaces(sc *scyllav1alpha1.ScyllaDBCluster, dc *scyllav1alpha1.ScyllaDBClusterDatacenter, managingClusterDomain string) ([]*corev1.Namespace, error) {
	suffix, err := naming.GenerateNameHash(sc.Namespace, dc.Name)
	if err != nil {
		return nil, fmt.Errorf("can't generate namespace name suffix: %w", err)
	}

	return []*corev1.Namespace{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   fmt.Sprintf("%s-%s", sc.Namespace, suffix),
				Labels: naming.ScyllaDBClusterDatacenterLabels(sc, dc, managingClusterDomain),
			},
		},
	}, nil
}

func MakeRemoteServices(sc *scyllav1alpha1.ScyllaDBCluster, dc *scyllav1alpha1.ScyllaDBClusterDatacenter, remoteNamespace *corev1.Namespace, remoteController metav1.Object, managingClusterDomain string) []*corev1.Service {
	var remoteServices []*corev1.Service

	remoteServices = append(remoteServices, makeSeedServices(sc, dc, remoteNamespace, remoteController, managingClusterDomain)...)

	return remoteServices
}

// makeSeedServices returns a list of seed services to other datacenters than one provided.
func makeSeedServices(sc *scyllav1alpha1.ScyllaDBCluster, dc *scyllav1alpha1.ScyllaDBClusterDatacenter, remoteNamespace *corev1.Namespace, remoteController metav1.Object, managingClusterDomain string) []*corev1.Service {
	var seedServices []*corev1.Service

	for _, otherDC := range sc.Spec.Datacenters {
		if dc.Name == otherDC.Name {
			continue
		}

		seedServices = append(seedServices, &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:            naming.SeedService(sc, &otherDC),
				Namespace:       remoteNamespace.Name,
				Labels:          naming.ScyllaDBClusterDatacenterLabels(sc, dc, managingClusterDomain),
				Annotations:     naming.ScyllaDBClusterDatacenterAnnotations(sc, dc),
				OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(remoteController, remoteControllerGVK)},
			},
			Spec: corev1.ServiceSpec{
				Ports: oslices.ConvertSlice(scyllaDBInterNodeCommunicationPorts, func(port portSpec) corev1.ServicePort {
					return corev1.ServicePort{
						Name:     port.name,
						Protocol: port.protocol,
						Port:     port.port,
					}
				}),
				Selector:  nil,
				ClusterIP: corev1.ClusterIPNone,
				Type:      corev1.ServiceTypeClusterIP,
			},
		})
	}

	return seedServices
}

// Given DC is part of seed list if it's fully reconciled, or is part of another DC seeds list,
// meaning it was fully reconciled in the past, so DC is part of the cluster.
func calculateSeedsForDatacenter(sc *scyllav1alpha1.ScyllaDBCluster, dc *scyllav1alpha1.ScyllaDBClusterDatacenter, remoteScyllaDBDatacenters map[string]map[string]*scyllav1alpha1.ScyllaDBDatacenter, remoteNamespace *corev1.Namespace) ([]string, error) {
	seedDCNamesSet := apimachineryutilsets.New[string]()
	for _, dcSpec := range sc.Spec.Datacenters {
		sdc, ok := remoteScyllaDBDatacenters[dcSpec.RemoteKubernetesClusterName][naming.ScyllaDBDatacenterName(sc, &dcSpec)]
		if !ok {
			continue

		}
		dcIsRolledOut, err := controllerhelpers.IsScyllaDBDatacenterRolledOut(sdc)
		if err != nil {
			return nil, fmt.Errorf("can't check if %q ScyllaDBDatacenter is rolled out: %w", naming.ObjRef(sdc), err)
		}
		if dcIsRolledOut {
			seedDCNamesSet.Insert(dcSpec.Name)
			continue
		}

		for _, remoteSDCs := range remoteScyllaDBDatacenters {
			for _, remoteSDC := range remoteSDCs {
				isReferencedByOtherDCAsSeed := oslices.Contains(remoteSDC.Spec.ScyllaDB.ExternalSeeds, func(remoteSDCSeed string) bool {
					return naming.DCNameFromSeedServiceAddress(sc, remoteSDCSeed, remoteSDC.Namespace) == dcSpec.Name
				})

				if isReferencedByOtherDCAsSeed {
					seedDCNamesSet.Insert(dcSpec.Name)
				}
			}
		}
	}

	seeds := make([]string, 0, len(sc.Spec.ScyllaDB.ExternalSeeds)+seedDCNamesSet.Len())
	seeds = append(seeds, sc.Spec.ScyllaDB.ExternalSeeds...)

	for _, otherDC := range sc.Spec.Datacenters {
		if otherDC.Name == dc.Name {
			continue
		}

		if seedDCNamesSet.Has(otherDC.Name) {
			seeds = append(seeds, fmt.Sprintf("%s.%s.svc", naming.SeedService(sc, &otherDC), remoteNamespace.Name))
		}
	}

	return seeds, nil
}

func MakeRemoteScyllaDBDatacenters(sc *scyllav1alpha1.ScyllaDBCluster, dc *scyllav1alpha1.ScyllaDBClusterDatacenter, remoteScyllaDBDatacenters map[string]map[string]*scyllav1alpha1.ScyllaDBDatacenter, remoteNamespace *corev1.Namespace, remoteController metav1.Object, managingClusterDomain string) (*scyllav1alpha1.ScyllaDBDatacenter, error) {
	dcSpec := applyDatacenterTemplateOnDatacenter(sc.Spec.DatacenterTemplate, dc)

	seeds, err := calculateSeedsForDatacenter(sc, dc, remoteScyllaDBDatacenters, remoteNamespace)
	if err != nil {
		return nil, fmt.Errorf("can't calculate seeds for datacenter %q: %w", dc.Name, err)
	}

	return &scyllav1alpha1.ScyllaDBDatacenter{
		ObjectMeta: metav1.ObjectMeta{
			Name:            naming.ScyllaDBDatacenterName(sc, dcSpec),
			Namespace:       remoteNamespace.Name,
			Labels:          naming.ScyllaDBClusterDatacenterLabels(sc, dcSpec, managingClusterDomain),
			Annotations:     naming.ScyllaDBClusterDatacenterAnnotations(sc, dcSpec),
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

				if dcSpec.Metadata != nil {
					if metadata == nil {
						metadata = &scyllav1alpha1.ObjectTemplateMetadata{
							Labels:      map[string]string{},
							Annotations: map[string]string{},
						}
					}
					maps.Copy(metadata.Labels, dcSpec.Metadata.Labels)
					maps.Copy(metadata.Annotations, dcSpec.Metadata.Annotations)
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
			DatacenterName: pointer.Ptr(dcSpec.Name),
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

				if sc.Spec.ForceRedeploymentReason != nil && dcSpec.ForceRedeploymentReason != nil {
					sb.WriteByte(',')
				}

				if dcSpec.ForceRedeploymentReason != nil {
					sb.WriteString(*dcSpec.ForceRedeploymentReason)
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
			RackTemplate:                            dcSpec.RackTemplate,
			Racks:                                   dcSpec.Racks,
			DisableAutomaticOrphanedNodeReplacement: pointer.Ptr(sc.Spec.DisableAutomaticOrphanedNodeReplacement),
			MinTerminationGracePeriodSeconds:        sc.Spec.MinTerminationGracePeriodSeconds,
			MinReadySeconds:                         sc.Spec.MinReadySeconds,
			ReadinessGates:                          sc.Spec.ReadinessGates,
			// TODO: not supported yet
			// Ref: https://github.com/scylladb/scylla-operator/issues/2262
			ImagePullSecrets: nil,
			// TODO not supported yet:
			// Ref: https://github.com/scylladb/scylla-operator/issues/2603
			DNSPolicy: nil,
			// TODO not supported yet:
			// Ref: https://github.com/scylladb/scylla-operator/issues/2602
			DNSDomains: nil,
		},
	}, nil
}

func MakeRemoteEndpointSlices(sc *scyllav1alpha1.ScyllaDBCluster, dc *scyllav1alpha1.ScyllaDBClusterDatacenter, remoteNamespace *corev1.Namespace, remoteController metav1.Object, remoteNamespaces map[string]*corev1.Namespace, remoteServiceLister remotelister.GenericClusterLister[corev1listers.ServiceLister], remotePodLister remotelister.GenericClusterLister[corev1listers.PodLister], managingClusterDomain string) ([]metav1.Condition, []*discoveryv1.EndpointSlice, error) {
	var progressingConditions []metav1.Condition
	var remoteEndpointSlices []*discoveryv1.EndpointSlice

	seedServiceEndpointSlicesProgressingConditions, seedServiceEndpointSlices, err := makeEndpointSlicesForSeedService(sc, dc, remoteNamespace, remoteController, remoteNamespaces, remoteServiceLister, remotePodLister, managingClusterDomain)
	progressingConditions = append(progressingConditions, seedServiceEndpointSlicesProgressingConditions...)
	if err != nil {
		return progressingConditions, nil, fmt.Errorf("can't make endpoint slices for seed service: %w", err)
	}

	remoteEndpointSlices = append(remoteEndpointSlices, seedServiceEndpointSlices...)

	return progressingConditions, remoteEndpointSlices, nil
}

// makeEndpointSlicesForSeedService creates EndpointSlice backing seed service, those endpoints are ScyllaDB nodes from other datacenters.
func makeEndpointSlicesForSeedService(sc *scyllav1alpha1.ScyllaDBCluster, dc *scyllav1alpha1.ScyllaDBClusterDatacenter, remoteNamespace *corev1.Namespace, remoteController metav1.Object, remoteNamespaces map[string]*corev1.Namespace, remoteServiceLister remotelister.GenericClusterLister[corev1listers.ServiceLister], remotePodLister remotelister.GenericClusterLister[corev1listers.PodLister], managingClusterDomain string) ([]metav1.Condition, []*discoveryv1.EndpointSlice, error) {
	var progressingConditions []metav1.Condition
	var remoteEndpointSlices []*discoveryv1.EndpointSlice

	nodeBroadcastType := scyllav1alpha1.BroadcastAddressTypePodIP
	if sc.Spec.ExposeOptions != nil && sc.Spec.ExposeOptions.BroadcastOptions != nil {
		nodeBroadcastType = sc.Spec.ExposeOptions.BroadcastOptions.Nodes.Type
	}

	for _, otherDC := range sc.Spec.Datacenters {
		if dc.Name == otherDC.Name {
			continue
		}

		otherDCPodSelector := naming.DatacenterPodsSelector(sc, &otherDC)
		otherDCNamespace, ok := remoteNamespaces[otherDC.RemoteKubernetesClusterName]
		if !ok {
			progressingConditions = append(progressingConditions, metav1.Condition{
				Type:               makeRemoteEndpointSliceControllerDatacenterProgressingCondition(dc.Name),
				Status:             metav1.ConditionTrue,
				Reason:             "WaitingForRemoteNamespace",
				Message:            fmt.Sprintf("Waiting for Namespace to be created in %q Cluster", otherDC.RemoteKubernetesClusterName),
				ObservedGeneration: sc.Generation,
			})
			continue
		}

		dcLabels := naming.ScyllaDBClusterDatacenterEndpointsLabels(sc, dc, managingClusterDomain)
		dcLabels[discoveryv1.LabelServiceName] = naming.SeedService(sc, &otherDC)
		dcLabels[discoveryv1.LabelManagedBy] = naming.OperatorAppNameWithDomain

		endpoints, err := calculateEndpointsForRemoteDCPods(sc, nodeBroadcastType, otherDC, otherDCNamespace, otherDCPodSelector, remotePodLister, remoteServiceLister)
		if err != nil {
			return progressingConditions, nil, fmt.Errorf("can't calculate endpoints to dataceter %q for datacenter %q: %w", otherDC.Name, dc.Name, err)
		}

		remoteEndpointSlices = append(remoteEndpointSlices, &discoveryv1.EndpointSlice{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:       remoteNamespace.Name,
				Name:            naming.SeedService(sc, &otherDC),
				Labels:          dcLabels,
				Annotations:     naming.ScyllaDBClusterDatacenterAnnotations(sc, dc),
				OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(remoteController, remoteControllerGVK)},
			},
			AddressType: discoveryv1.AddressTypeIPv4,
			Endpoints:   endpoints,
			Ports: oslices.ConvertSlice(scyllaDBInterNodeCommunicationPorts, func(port portSpec) discoveryv1.EndpointPort {
				return discoveryv1.EndpointPort{
					Name:     pointer.Ptr(port.name),
					Protocol: pointer.Ptr(port.protocol),
					Port:     pointer.Ptr(port.port),
				}
			}),
		})
	}

	return progressingConditions, remoteEndpointSlices, nil
}

type portSpec struct {
	name     string
	protocol corev1.Protocol
	port     int32
}

var scyllaDBInterNodeCommunicationPorts = []portSpec{
	{
		name:     "inter-node",
		protocol: corev1.ProtocolTCP,
		port:     scylla.DefaultStoragePort,
	},
	{
		name:     "inter-node-ssl",
		protocol: corev1.ProtocolTCP,
		port:     scylla.DefaultStoragePortSSL,
	},
	{
		name:     "cql",
		protocol: corev1.ProtocolTCP,
		port:     scylla.DefaultNativeTransportPort,
	},
	{
		name:     "cql-ssl",
		protocol: corev1.ProtocolTCP,
		port:     scylla.DefaultNativeTransportPortSSL,
	},
}

// calculateEndpointsForRemoteDCPods computes endpoints for remote datacenter pods taking into account how nodes are being exposed.
func calculateEndpointsForRemoteDCPods(sc *scyllav1alpha1.ScyllaDBCluster, nodeBroadcastType scyllav1alpha1.BroadcastAddressType, remoteDC scyllav1alpha1.ScyllaDBClusterDatacenter, remoteDCNamespace *corev1.Namespace, remoteDCPodSelector labels.Selector, remotePodLister remotelister.GenericClusterLister[corev1listers.PodLister], remoteServiceLister remotelister.GenericClusterLister[corev1listers.ServiceLister]) ([]discoveryv1.Endpoint, error) {
	var endpoints []discoveryv1.Endpoint

	switch nodeBroadcastType {
	case scyllav1alpha1.BroadcastAddressTypePodIP:
		dcPods, err := remotePodLister.Cluster(remoteDC.RemoteKubernetesClusterName).Pods(remoteDCNamespace.Name).List(remoteDCPodSelector)
		if err != nil {
			return nil, fmt.Errorf("can't list pods in %q ScyllaCluster %q Datacenter: %w", naming.ObjRef(sc), remoteDC.Name, err)
		}

		klog.V(4).InfoS("Found remote Scylla Pods", "Cluster", klog.KObj(sc), "Datacenter", remoteDC.Name, "Pods", len(dcPods))

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

			endpoints = append(endpoints, discoveryv1.Endpoint{
				Addresses: []string{dcPod.Status.PodIP},
				Conditions: discoveryv1.EndpointConditions{
					Ready:       pointer.Ptr(ready),
					Serving:     pointer.Ptr(serving),
					Terminating: pointer.Ptr(terminating),
				},
			})
		}

	case scyllav1alpha1.BroadcastAddressTypeServiceLoadBalancerIngress:
		dcServices, err := remoteServiceLister.Cluster(remoteDC.RemoteKubernetesClusterName).Services(remoteDCNamespace.Name).List(remoteDCPodSelector)
		if err != nil {
			return nil, fmt.Errorf("can't list services in %q ScyllaDBCluster %q Datacenter: %w", naming.ObjRef(sc), remoteDC.Name, err)
		}

		klog.V(4).InfoS("Found remote ScyllaDB Services", "ScyllaDBCluster", klog.KObj(sc), "Datacenter", remoteDC.Name, "Services", len(dcServices))

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

				endpoints = append(endpoints, ep)
			}
		}

	default:
		return nil, fmt.Errorf("unsupported node broadcast address type %v specified in %q ScyllaDBCluster", nodeBroadcastType, naming.ObjRef(sc))
	}

	return endpoints, nil
}

func mergeScyllaV1Alpha1Placement(placementGetters ...func() *scyllav1alpha1.Placement) *scyllav1alpha1.Placement {
	placementGetters = oslices.FilterOut(placementGetters, func(getter func() *scyllav1alpha1.Placement) bool {
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
	scyllaDBTemplateGetters = oslices.FilterOut(scyllaDBTemplateGetters, func(getter func() *scyllav1alpha1.ScyllaDBTemplate) bool {
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
	scyllaDBManagerAgentTemplateGetters = oslices.FilterOut(scyllaDBManagerAgentTemplateGetters, func(getter func() *scyllav1alpha1.ScyllaDBManagerAgentTemplate) bool {
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

func MakeRemoteConfigMaps(sc *scyllav1alpha1.ScyllaDBCluster, dc *scyllav1alpha1.ScyllaDBClusterDatacenter, remoteNamespace *corev1.Namespace, remoteController metav1.Object, localConfigMapLister corev1listers.ConfigMapLister, managingClusterDomain string) ([]metav1.Condition, []*corev1.ConfigMap, error) {
	var requiredRemoteConfigMaps []*corev1.ConfigMap

	progressingConditions, mirroredConfigMaps, err := makeMirroredRemoteConfigMaps(sc, dc, remoteNamespace, remoteController, localConfigMapLister, managingClusterDomain)
	if err != nil {
		return progressingConditions, nil, fmt.Errorf("can't make mirrored configmaps: %w", err)
	}

	requiredRemoteConfigMaps = append(requiredRemoteConfigMaps, mirroredConfigMaps...)

	return progressingConditions, requiredRemoteConfigMaps, nil
}

func makeMirroredRemoteConfigMaps(sc *scyllav1alpha1.ScyllaDBCluster, dc *scyllav1alpha1.ScyllaDBClusterDatacenter, remoteNamespace *corev1.Namespace, remoteController metav1.Object, localConfigMapLister corev1listers.ConfigMapLister, managingClusterDomain string) ([]metav1.Condition, []*corev1.ConfigMap, error) {
	var errs []error
	var progressingConditions []metav1.Condition
	var requiredRemoteConfigMaps []*corev1.ConfigMap

	var configMapsToMirror []string

	if sc.Spec.DatacenterTemplate != nil && sc.Spec.DatacenterTemplate.ScyllaDB != nil && sc.Spec.DatacenterTemplate.ScyllaDB.CustomConfigMapRef != nil {
		configMapsToMirror = append(configMapsToMirror, *sc.Spec.DatacenterTemplate.ScyllaDB.CustomConfigMapRef)
	}

	if sc.Spec.DatacenterTemplate != nil && sc.Spec.DatacenterTemplate.RackTemplate != nil && sc.Spec.DatacenterTemplate.RackTemplate.ScyllaDB != nil && sc.Spec.DatacenterTemplate.RackTemplate.ScyllaDB.CustomConfigMapRef != nil {
		configMapsToMirror = append(configMapsToMirror, *sc.Spec.DatacenterTemplate.RackTemplate.ScyllaDB.CustomConfigMapRef)
	}

	if dc.ScyllaDB != nil && dc.ScyllaDB.CustomConfigMapRef != nil {
		configMapsToMirror = append(configMapsToMirror, *dc.ScyllaDB.CustomConfigMapRef)
	}

	if dc.RackTemplate != nil && dc.RackTemplate.ScyllaDB != nil && dc.RackTemplate.ScyllaDB.CustomConfigMapRef != nil {
		configMapsToMirror = append(configMapsToMirror, *dc.RackTemplate.ScyllaDB.CustomConfigMapRef)
	}

	for _, rack := range dc.Racks {
		if rack.ScyllaDB != nil && rack.ScyllaDB.CustomConfigMapRef != nil {
			configMapsToMirror = append(configMapsToMirror, *rack.ScyllaDB.CustomConfigMapRef)
		}
	}

	for _, cmName := range configMapsToMirror {
		localConfigMap, err := localConfigMapLister.ConfigMaps(sc.Namespace).Get(cmName)
		if err != nil {
			if apierrors.IsNotFound(err) {
				progressingConditions = append(progressingConditions, metav1.Condition{
					Type:               makeRemoteConfigMapControllerDatacenterProgressingCondition(dc.Name),
					Status:             metav1.ConditionTrue,
					Reason:             "WaitingForConfigMap",
					Message:            fmt.Sprintf("Waiting for ConfigMap %q to exist.", naming.ManualRef(sc.Namespace, cmName)),
					ObservedGeneration: sc.Generation,
				})
				continue
			}
			errs = append(errs, fmt.Errorf("can't get %q ConfigMap: %w", naming.ManualRef(sc.Namespace, cmName), err))
			continue
		}

		requiredRemoteConfigMaps = append(requiredRemoteConfigMaps, &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:            cmName,
				Namespace:       remoteNamespace.Name,
				Labels:          naming.ScyllaDBClusterDatacenterLabels(sc, dc, managingClusterDomain),
				Annotations:     naming.ScyllaDBClusterDatacenterAnnotations(sc, dc),
				OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(remoteController, remoteControllerGVK)},
			},
			Immutable:  localConfigMap.Immutable,
			Data:       maps.Clone(localConfigMap.Data),
			BinaryData: maps.Clone(localConfigMap.BinaryData),
		})
	}

	err := apimachineryutilerrors.NewAggregate(errs)
	if err != nil {
		return progressingConditions, requiredRemoteConfigMaps, fmt.Errorf("can't make mirrored ConfigMaps: %w", err)
	}

	return progressingConditions, requiredRemoteConfigMaps, nil
}

func MakeRemoteSecrets(sc *scyllav1alpha1.ScyllaDBCluster, dc *scyllav1alpha1.ScyllaDBClusterDatacenter, remoteNamespace *corev1.Namespace, remoteController metav1.Object, localSecretLister corev1listers.SecretLister, managingClusterDomain string) ([]metav1.Condition, []*corev1.Secret, error) {
	var requiredRemoteSecrets []*corev1.Secret

	progressingConditions, mirroredSecrets, err := makeMirroredRemoteSecrets(sc, dc, remoteNamespace, remoteController, localSecretLister, managingClusterDomain)
	if err != nil {
		return progressingConditions, nil, fmt.Errorf("can't make mirrored secrets: %w", err)
	}

	requiredRemoteSecrets = append(requiredRemoteSecrets, mirroredSecrets...)

	return progressingConditions, requiredRemoteSecrets, nil
}

func makeMirroredRemoteSecrets(sc *scyllav1alpha1.ScyllaDBCluster, dc *scyllav1alpha1.ScyllaDBClusterDatacenter, remoteNamespace *corev1.Namespace, remoteController metav1.Object, localSecretLister corev1listers.SecretLister, managingClusterDomain string) ([]metav1.Condition, []*corev1.Secret, error) {
	var progressingConditions []metav1.Condition
	var requiredRemoteSecrets []*corev1.Secret

	var secretsToMirror []string

	if sc.Spec.DatacenterTemplate != nil && sc.Spec.DatacenterTemplate.ScyllaDBManagerAgent != nil && sc.Spec.DatacenterTemplate.ScyllaDBManagerAgent.CustomConfigSecretRef != nil {
		secretsToMirror = append(secretsToMirror, *sc.Spec.DatacenterTemplate.ScyllaDBManagerAgent.CustomConfigSecretRef)
	}

	if sc.Spec.DatacenterTemplate != nil && sc.Spec.DatacenterTemplate.RackTemplate != nil && sc.Spec.DatacenterTemplate.RackTemplate.ScyllaDBManagerAgent != nil && sc.Spec.DatacenterTemplate.RackTemplate.ScyllaDBManagerAgent.CustomConfigSecretRef != nil {
		secretsToMirror = append(secretsToMirror, *sc.Spec.DatacenterTemplate.RackTemplate.ScyllaDBManagerAgent.CustomConfigSecretRef)
	}

	if dc.ScyllaDBManagerAgent != nil && dc.ScyllaDBManagerAgent.CustomConfigSecretRef != nil {
		secretsToMirror = append(secretsToMirror, *dc.ScyllaDBManagerAgent.CustomConfigSecretRef)
	}

	if dc.RackTemplate != nil && dc.RackTemplate.ScyllaDBManagerAgent != nil && dc.RackTemplate.ScyllaDBManagerAgent.CustomConfigSecretRef != nil {
		secretsToMirror = append(secretsToMirror, *dc.RackTemplate.ScyllaDBManagerAgent.CustomConfigSecretRef)
	}

	for _, rack := range dc.Racks {
		if rack.ScyllaDBManagerAgent != nil && rack.ScyllaDBManagerAgent.CustomConfigSecretRef != nil {
			secretsToMirror = append(secretsToMirror, *rack.ScyllaDBManagerAgent.CustomConfigSecretRef)
		}
	}

	var errs []error
	for _, secretName := range secretsToMirror {
		localSecret, err := localSecretLister.Secrets(sc.Namespace).Get(secretName)
		if err != nil {
			if apierrors.IsNotFound(err) {
				progressingConditions = append(progressingConditions, metav1.Condition{
					Type:               makeRemoteSecretControllerDatacenterProgressingCondition(dc.Name),
					Status:             metav1.ConditionTrue,
					Reason:             "WaitingForSecret",
					Message:            fmt.Sprintf("Waiting for Secret %q to exist.", naming.ManualRef(sc.Namespace, secretName)),
					ObservedGeneration: sc.Generation,
				})
				continue
			}
			errs = append(errs, fmt.Errorf("can't get %q Secret: %w", naming.ManualRef(sc.Namespace, secretName), err))
			continue
		}

		requiredRemoteSecrets = append(requiredRemoteSecrets, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:            secretName,
				Namespace:       remoteNamespace.Name,
				Labels:          naming.ScyllaDBClusterDatacenterLabels(sc, dc, managingClusterDomain),
				Annotations:     naming.ScyllaDBClusterDatacenterAnnotations(sc, dc),
				OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(remoteController, remoteControllerGVK)},
			},
			Immutable: localSecret.Immutable,
			Data:      maps.Clone(localSecret.Data),
			Type:      localSecret.Type,
		})
	}

	err := apimachineryutilerrors.NewAggregate(errs)
	if err != nil {
		return progressingConditions, requiredRemoteSecrets, fmt.Errorf("can't make mirrored Secrets: %w", err)
	}

	return progressingConditions, requiredRemoteSecrets, nil
}
