package controllerhelpers

import (
	"fmt"
	"sort"

	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/helpers"
	oslices "github.com/scylladb/scylla-operator/pkg/helpers/slices"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	"github.com/scylladb/scylla-operator/pkg/scyllaclient"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	apimachineryutilerrors "k8s.io/apimachinery/pkg/util/errors"
	corev1listers "k8s.io/client-go/listers/core/v1"
	corev1schedulinghelpers "k8s.io/component-helpers/scheduling/corev1"
	"k8s.io/component-helpers/scheduling/corev1/nodeaffinity"
)

func GetScyllaHost(sdc *scyllav1alpha1.ScyllaDBDatacenter, svc *corev1.Service, pod *corev1.Pod) (string, error) {
	// Assume API's default.
	nodeBroadcastAddressType := scyllav1alpha1.ScyllaDBDatacenterDefaultNodesBroadcastAddressType
	if sdc.Spec.ExposeOptions != nil && sdc.Spec.ExposeOptions.BroadcastOptions != nil {
		nodeBroadcastAddressType = sdc.Spec.ExposeOptions.BroadcastOptions.Nodes.Type
	}

	return GetScyllaBroadcastAddress(nodeBroadcastAddressType, svc, pod)
}

func GetScyllaClientBroadcastHost(sdc *scyllav1alpha1.ScyllaDBDatacenter, svc *corev1.Service, pod *corev1.Pod) (string, error) {
	clientsBroadcastAddressType := scyllav1alpha1.ScyllaDBDatacenterDefaultClientsBroadcastAddressType
	if sdc.Spec.ExposeOptions != nil && sdc.Spec.ExposeOptions.BroadcastOptions != nil {
		clientsBroadcastAddressType = sdc.Spec.ExposeOptions.BroadcastOptions.Clients.Type
	}

	return GetScyllaBroadcastAddress(clientsBroadcastAddressType, svc, pod)
}

func GetScyllaHostForScyllaCluster(sc *scyllav1.ScyllaCluster, svc *corev1.Service, pod *corev1.Pod) (string, error) {
	// Assume API's default.
	nodeBroadcastAddressType := scyllav1.BroadcastAddressTypeServiceClusterIP
	if sc.Spec.ExposeOptions != nil && sc.Spec.ExposeOptions.BroadcastOptions != nil {
		nodeBroadcastAddressType = sc.Spec.ExposeOptions.BroadcastOptions.Nodes.Type
	}

	return GetScyllaBroadcastAddress(scyllav1alpha1.BroadcastAddressType(nodeBroadcastAddressType), svc, pod)
}

func GetScyllaBroadcastAddress(broadcastAddressType scyllav1alpha1.BroadcastAddressType, svc *corev1.Service, pod *corev1.Pod, ipFamily ...*corev1.IPFamily) (string, error) {
	switch broadcastAddressType {
	case scyllav1alpha1.BroadcastAddressTypeServiceLoadBalancerIngress:
		if len(svc.Status.LoadBalancer.Ingress) < 1 {
			return "", fmt.Errorf("service %q does not have an ingress status", naming.ObjRef(svc))
		}

		if len(ipFamily) > 0 && ipFamily[0] != nil {
			preferredFamily := *ipFamily[0]

			for _, ingress := range svc.Status.LoadBalancer.Ingress {
				if len(ingress.IP) == 0 {
					continue
				}
				ip, err := helpers.ParseIP(ingress.IP)
				if err != nil {
					continue
				}
				if helpers.GetIPFamily(ip) == preferredFamily {
					if helpers.IsIPv6(ip) {
						return ip.String(), nil
					}
					return ingress.IP, nil
				}
			}

			// If no IP of the preferred family was found, try hostname as fallback
			if len(svc.Status.LoadBalancer.Ingress[0].Hostname) != 0 {
				return svc.Status.LoadBalancer.Ingress[0].Hostname, nil
			}

			return "", fmt.Errorf("service %q does not have a %s ingress IP or hostname", naming.ObjRef(svc), preferredFamily)
		}

		if len(svc.Status.LoadBalancer.Ingress[0].IP) != 0 {
			if ip, err := helpers.ParseIP(svc.Status.LoadBalancer.Ingress[0].IP); err == nil && helpers.IsIPv6(ip) {
				return ip.String(), nil
			}
			return svc.Status.LoadBalancer.Ingress[0].IP, nil
		}

		if len(svc.Status.LoadBalancer.Ingress[0].Hostname) != 0 {
			return svc.Status.LoadBalancer.Ingress[0].Hostname, nil
		}

		return "", fmt.Errorf("service %q does not have an external address", naming.ObjRef(svc))

	case scyllav1alpha1.BroadcastAddressTypeServiceClusterIP:
		if svc.Spec.ClusterIP == corev1.ClusterIPNone {
			return "", fmt.Errorf("service %q does not have a ClusterIP address", naming.ObjRef(svc))
		}

		if ip, err := helpers.ParseIP(svc.Spec.ClusterIP); err == nil && helpers.IsIPv6(ip) {
			return ip.String(), nil
		}
		return svc.Spec.ClusterIP, nil

	case scyllav1alpha1.BroadcastAddressTypePodIP:
		if len(ipFamily) > 0 && ipFamily[0] != nil {
			preferredFamily := *ipFamily[0]

			for _, podIP := range pod.Status.PodIPs {
				ip, err := helpers.ParseIP(podIP.IP)
				if err != nil {
					continue
				}
				if helpers.GetIPFamily(ip) == preferredFamily {
					if helpers.IsIPv6(ip) {
						return ip.String(), nil
					}
					return podIP.IP, nil
				}
			}

			if len(pod.Status.PodIP) == 0 {
				return "", fmt.Errorf("pod %q does not have a PodIP address", naming.ObjRef(pod))
			}

			if ip, err := helpers.ParseIP(pod.Status.PodIP); err == nil && helpers.IsIPv6(ip) {
				return ip.String(), nil
			}
			return pod.Status.PodIP, nil
		}

		if len(pod.Status.PodIP) == 0 {
			return "", fmt.Errorf("pod %q does not have a PodIP address", naming.ObjRef(pod))
		}

		if ip, err := helpers.ParseIP(pod.Status.PodIP); err == nil && helpers.IsIPv6(ip) {
			return ip.String(), nil
		}
		return pod.Status.PodIP, nil

	default:
		return "", fmt.Errorf("unsupported broadcast address type: %q", broadcastAddressType)
	}
}

func GetRequiredScyllaHosts(sdc *scyllav1alpha1.ScyllaDBDatacenter, services map[string]*corev1.Service, podLister corev1listers.PodLister) ([]string, error) {
	var hosts []string
	var errs []error
	for _, rack := range sdc.Spec.Racks {
		rackNodeCount, err := GetRackNodeCount(sdc, rack.Name)
		if err != nil {
			errs = append(errs, fmt.Errorf("can't get rack %q node count of ScyllaDBDatacenter %q: %w", rack.Name, naming.ObjRef(sdc), err))
			continue
		}
		for ord := int32(0); ord < *rackNodeCount; ord++ {
			svcName := naming.MemberServiceName(rack, sdc, int(ord))
			svc, exists := services[svcName]
			if !exists {
				errs = append(errs, fmt.Errorf("service %q does not exist", naming.ManualRef(sdc.Namespace, svcName)))
				continue
			}

			podName := naming.PodNameFromService(svc)
			pod, err := podLister.Pods(sdc.Namespace).Get(podName)
			if err != nil {
				errs = append(errs, fmt.Errorf("can't get pod %q: %w", naming.ManualRef(sdc.Namespace, podName), err))
				continue
			}

			host, err := GetScyllaHost(sdc, svc, pod)
			if err != nil {
				errs = append(errs, fmt.Errorf("can't get scylla host for service %q: %w", naming.ObjRef(svc), err))
				continue
			}

			hosts = append(hosts, host)
		}
	}
	var err error = apimachineryutilerrors.NewAggregate(errs)
	if err != nil {
		return nil, err
	}

	return hosts, nil
}

func NewScyllaClient(cfg *scyllaclient.Config) (*scyllaclient.Client, error) {
	scyllaClient, err := scyllaclient.NewClient(cfg)
	if err != nil {
		return nil, err
	}
	return scyllaClient, nil
}

func NewScyllaClientFromToken(hosts []string, authToken string) (*scyllaclient.Client, error) {
	cfg := scyllaclient.DefaultConfig(authToken, hosts...)
	return NewScyllaClient(cfg)
}

// NewScyllaClientConfigForLocalhost creates a ScyllaDB client configuration for
// localhost with the specified IP family (IPv4 or IPv6).
func NewScyllaClientConfigForLocalhost(ipFamily corev1.IPFamily) *scyllaclient.Config {
	var host string
	switch ipFamily {
	case corev1.IPv6Protocol:
		host = "::1" // IPv6 localhost
	default:
		host = "127.0.0.1" // IPv4 localhost
	}

	cfg := scyllaclient.DefaultConfig("", host)
	cfg.Scheme = "http"
	cfg.Port = fmt.Sprintf("%d", naming.ScyllaAPIPort)
	t := scyllaclient.DefaultTransport()
	t.TLSClientConfig = nil
	cfg.Transport = t
	return cfg
}

// NewScyllaClientForLocalhost creates a ScyllaDB client configured for
// localhost with the specified IP family (IPv4 or IPv6).
func NewScyllaClientForLocalhost(ipFamily corev1.IPFamily) (*scyllaclient.Client, error) {
	cfg := NewScyllaClientConfigForLocalhost(ipFamily)
	return NewScyllaClient(cfg)
}

func SetRackCondition(rackStatus *scyllav1.RackStatus, newCondition scyllav1.RackConditionType) {
	for i := range rackStatus.Conditions {
		if rackStatus.Conditions[i].Type == newCondition {
			rackStatus.Conditions[i].Status = corev1.ConditionTrue
			return
		}
	}
	rackStatus.Conditions = append(
		rackStatus.Conditions,
		scyllav1.RackCondition{Type: newCondition, Status: corev1.ConditionTrue},
	)
}

func FindNodeStatus(nodeStatuses []scyllav1alpha1.NodeConfigNodeStatus, nodeName string) *scyllav1alpha1.NodeConfigNodeStatus {
	for i := range nodeStatuses {
		ns := &nodeStatuses[i]
		if ns.Name == nodeName {
			return ns
		}
	}

	return nil
}

func SetNodeStatus(nodeStatuses []scyllav1alpha1.NodeConfigNodeStatus, status *scyllav1alpha1.NodeConfigNodeStatus) []scyllav1alpha1.NodeConfigNodeStatus {
	for i, ns := range nodeStatuses {
		if ns.Name == status.Name {
			nodeStatuses[i] = *status
			return nodeStatuses
		}
	}

	nodeStatuses = append(nodeStatuses, *status)

	sort.SliceStable(nodeStatuses, func(i, j int) bool {
		return nodeStatuses[i].Name < nodeStatuses[j].Name
	})

	return nodeStatuses
}

func IsNodeConfigSelectingNode(nc *scyllav1alpha1.NodeConfig, node *corev1.Node) (bool, error) {
	// Check nodeSelector.

	if !labels.SelectorFromSet(nc.Spec.Placement.NodeSelector).Matches(labels.Set(node.Labels)) {
		return false, nil
	}

	// Check affinity.

	if nc.Spec.Placement.Affinity.NodeAffinity != nil &&
		nc.Spec.Placement.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution != nil {
		affinityNodeSelector, err := nodeaffinity.NewNodeSelector(
			nc.Spec.Placement.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution,
		)
		if err != nil {
			return false, fmt.Errorf("can't construct node affinity node selector: %w", err)
		}

		if !affinityNodeSelector.Match(node) {
			return false, nil
		}
	}

	// Check taints and tolerations.

	_, isUntolerated := corev1schedulinghelpers.FindMatchingUntoleratedTaint(
		node.Spec.Taints,
		nc.Spec.Placement.Tolerations,
		func(t *corev1.Taint) bool {
			// We are only interested in NoSchedule and NoExecute taints.
			return t.Effect == corev1.TaintEffectNoSchedule || t.Effect == corev1.TaintEffectNoExecute
		},
	)
	if isUntolerated {
		return false, nil
	}

	return true, nil
}

func IsNodeTunedForContainer(nc *scyllav1alpha1.NodeConfig, nodeName string, containerID string) bool {
	ns := FindNodeStatus(nc.Status.NodeStatuses, nodeName)
	if ns == nil {
		return false
	}

	if !ns.TunedNode {
		return false
	}

	for _, cid := range ns.TunedContainers {
		if cid == containerID {
			return true
		}
	}

	return false
}

func IsPodTunable(pod *corev1.Pod) bool {
	return pod.Status.QOSClass == corev1.PodQOSGuaranteed
}

func IsScyllaPod(pod *corev1.Pod) bool {
	if pod.Labels == nil {
		return false
	}

	if !labels.SelectorFromSet(naming.ScyllaDBNodePodLabels()).Matches(labels.Set(pod.Labels)) {
		return false
	}

	return true
}

func GetRackNodeCount(sdc *scyllav1alpha1.ScyllaDBDatacenter, rackName string) (*int32, error) {
	rackSpec, _, ok := oslices.Find(sdc.Spec.Racks, func(spec scyllav1alpha1.RackSpec) bool {
		return spec.Name == rackName
	})
	if !ok {
		return nil, fmt.Errorf("can't find rack %q in rack spec of ScyllaDBDatacenter %q", rackName, naming.ObjRef(sdc))
	}

	if rackSpec.Nodes != nil {
		return rackSpec.Nodes, nil
	}

	if sdc.Spec.RackTemplate != nil && sdc.Spec.RackTemplate.Nodes != nil {
		return sdc.Spec.RackTemplate.Nodes, nil
	}

	// TODO: support scale subresource, until it's missing, mimic default value of rack members from v1.ScyllaCluster
	return pointer.Ptr[int32](0), nil
}

func IsScyllaDBDatacenterRolledOut(sdc *scyllav1alpha1.ScyllaDBDatacenter) (bool, error) {
	if !helpers.IsStatusConditionPresentAndTrue(sdc.Status.Conditions, scyllav1alpha1.AvailableCondition, sdc.Generation) {
		return false, nil
	}

	if !helpers.IsStatusConditionPresentAndFalse(sdc.Status.Conditions, scyllav1alpha1.ProgressingCondition, sdc.Generation) {
		return false, nil
	}

	if !helpers.IsStatusConditionPresentAndFalse(sdc.Status.Conditions, scyllav1alpha1.DegradedCondition, sdc.Generation) {
		return false, nil
	}

	return true, nil
}

func GetScyllaDBClusterNodeCount(sc *scyllav1alpha1.ScyllaDBCluster) int32 {
	nodes := int32(0)
	for _, dc := range sc.Spec.Datacenters {
		nodes += GetScyllaDBClusterDatacenterNodeCount(sc, dc)
	}

	return nodes
}

func GetScyllaDBClusterDatacenterNodeCount(sc *scyllav1alpha1.ScyllaDBCluster, dc scyllav1alpha1.ScyllaDBClusterDatacenter) int32 {
	var racks []scyllav1alpha1.RackSpec
	nodes := int32(0)
	rackNodes := int32(0)

	if sc.Spec.DatacenterTemplate != nil {
		if sc.Spec.DatacenterTemplate.RackTemplate != nil && sc.Spec.DatacenterTemplate.RackTemplate.Nodes != nil {
			rackNodes = *sc.Spec.DatacenterTemplate.RackTemplate.Nodes
		}
		if sc.Spec.DatacenterTemplate.Racks != nil {
			racks = sc.Spec.DatacenterTemplate.Racks
		}
	}

	if dc.RackTemplate != nil && dc.RackTemplate.Nodes != nil {
		rackNodes = *dc.RackTemplate.Nodes
	}

	if dc.Racks != nil {
		racks = dc.Racks
	}

	for _, rack := range racks {
		if rack.Nodes != nil {
			nodes += *rack.Nodes
		} else {
			nodes += rackNodes
		}
	}

	return nodes
}
