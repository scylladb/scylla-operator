package identity

import (
	"context"
	"fmt"
	"sort"
	"strconv"

	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/naming"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

// Member encapsulates the identity for a single member
// of a Scylla Cluster.
type Member struct {
	// Name of the Pod
	Name string
	// Namespace of the Pod
	Namespace string
	// PodIP IP of the Pod
	PodIP string
	// ExternalAddress is an external IP or hostname of a LoadBalancer Service
	ExternalAddress string
	// ClusterIP of the member's Service
	ClusterIP     string
	Rack          string
	RackOrdinal   int
	Datacenter    string
	Cluster       string
	ServiceLabels map[string]string
	PodID         string

	Overprovisioned     bool
	BroadcastRPCAddress string
	BroadcastAddress    string

	NodesBroadcastAddressType scyllav1.BroadcastAddressType
}

func NewMember(service *corev1.Service, pod *corev1.Pod, nodesAddressType, clientAddressType scyllav1.BroadcastAddressType) (*Member, error) {
	rackOrdinalString, ok := pod.Labels[naming.RackOrdinalLabel]
	if !ok {
		return nil, fmt.Errorf("pod %q is missing %q label", naming.ObjRef(pod), naming.RackOrdinalLabel)
	}
	rackOrdinal, err := strconv.Atoi(rackOrdinalString)
	if err != nil {
		return nil, fmt.Errorf("can't get rack ordinal from label %q: %w", rackOrdinalString, err)
	}

	var externalIP string
	if service.Spec.Type == corev1.ServiceTypeLoadBalancer {
		if len(service.Status.LoadBalancer.Ingress) == 0 {
			return nil, fmt.Errorf("LoadBalancer Service %q is missing endpoints in status", naming.ObjRef(service))
		}

		firstIngress := service.Status.LoadBalancer.Ingress[0]
		if len(firstIngress.Hostname) != 0 {
			externalIP = firstIngress.Hostname
		}

		if len(firstIngress.IP) != 0 {
			externalIP = firstIngress.IP
		}

		if len(externalIP) == 0 {
			return nil, fmt.Errorf("service first ingress status has empty external address")
		}
	}

	m := &Member{
		Namespace:                 service.Namespace,
		Name:                      service.Name,
		PodIP:                     pod.Status.PodIP,
		ClusterIP:                 service.Spec.ClusterIP,
		ExternalAddress:           externalIP,
		Rack:                      pod.Labels[naming.RackNameLabel],
		RackOrdinal:               rackOrdinal,
		Datacenter:                pod.Labels[naming.DatacenterNameLabel],
		Cluster:                   pod.Labels[naming.ClusterNameLabel],
		ServiceLabels:             service.Labels,
		PodID:                     string(pod.UID),
		Overprovisioned:           pod.Status.QOSClass != corev1.PodQOSGuaranteed,
		NodesBroadcastAddressType: nodesAddressType,
	}

	switch nodesAddressType {
	case scyllav1.BroadcastAddressTypePodIP:
		m.BroadcastAddress = m.PodIP
	case scyllav1.BroadcastAddressTypeServiceClusterIP:
		m.BroadcastAddress = m.ClusterIP
	case scyllav1.BroadcastAddressTypeServiceLoadBalancerIngress:
		m.BroadcastAddress = m.ExternalAddress
	default:
		return nil, fmt.Errorf("unsupported nodes address type: %q", nodesAddressType)
	}

	switch clientAddressType {
	case scyllav1.BroadcastAddressTypePodIP:
		m.BroadcastRPCAddress = m.PodIP
	case scyllav1.BroadcastAddressTypeServiceClusterIP:
		m.BroadcastRPCAddress = m.ClusterIP
	case scyllav1.BroadcastAddressTypeServiceLoadBalancerIngress:
		m.BroadcastRPCAddress = m.ExternalAddress
	default:
		return nil, fmt.Errorf("unsupported clients address type: %q", clientAddressType)
	}

	return m, nil
}

func (m *Member) GetSeeds(ctx context.Context, coreClient v1.CoreV1Interface, externalSeeds []string) ([]string, error) {
	clusterLabels := naming.ScyllaLabels()
	clusterLabels[naming.ClusterNameLabel] = m.Cluster

	podList, err := coreClient.Pods(m.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(clusterLabels).String(),
	})
	if err != nil {
		return nil, err
	}

	if len(podList.Items) == 0 {
		return nil, fmt.Errorf("internal error: can't find any pod for this cluster, including itself")
	}

	var otherPods []*corev1.Pod
	for i := range podList.Items {
		pod := &podList.Items[i]
		if pod.Name != m.Name {
			otherPods = append(otherPods, pod)
		}
	}

	if len(otherPods) == 0 {
		// We are the only one, assuming first bootstrap.

		podOrdinal, err := naming.IndexFromName(m.Name)
		if err != nil {
			return nil, fmt.Errorf("can't get pod index from name: %w", err)
		}

		if m.RackOrdinal != 0 || podOrdinal != 0 {
			return nil, fmt.Errorf("pod is not first in the cluster, but there are no other pods")
		}

		if len(externalSeeds) > 0 {
			return externalSeeds, nil
		}

		return []string{m.BroadcastAddress}, nil
	}

	sort.Slice(otherPods, func(i, j int) bool {
		if controllerhelpers.IsPodReady(otherPods[i]) && !controllerhelpers.IsPodReady(otherPods[j]) {
			return true
		}

		return podList.Items[i].ObjectMeta.CreationTimestamp.Before(&podList.Items[j].ObjectMeta.CreationTimestamp)
	})

	pod := otherPods[0]

	svc, err := coreClient.Services(m.Namespace).Get(ctx, pod.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	res := make([]string, 0, len(externalSeeds)+1)
	res = append(res, externalSeeds...)

	// Assume nodes share broadcast address type, and it's immutable.
	switch m.NodesBroadcastAddressType {
	case scyllav1.BroadcastAddressTypeServiceLoadBalancerIngress:
		if len(svc.Status.LoadBalancer.Ingress) < 1 {
			return nil, fmt.Errorf("service %q does not have ingress status, despite external address being set as broadcasted", naming.ObjRef(svc))
		}
		if len(svc.Status.LoadBalancer.Ingress[0].IP) != 0 {
			res = append(res, svc.Status.LoadBalancer.Ingress[0].IP)
		} else if len(svc.Status.LoadBalancer.Ingress[0].Hostname) != 0 {
			res = append(res, svc.Status.LoadBalancer.Ingress[0].Hostname)
		} else {
			return nil, fmt.Errorf("service %q does not have external address, despite being set as broadcasted", naming.ObjRef(svc))
		}
	case scyllav1.BroadcastAddressTypeServiceClusterIP:
		if svc.Spec.ClusterIP == corev1.ClusterIPNone {
			return nil, fmt.Errorf("service %q does not have ClusterIP address, despite being set as broadcasted", naming.ObjRef(svc))
		}
		res = append(res, svc.Spec.ClusterIP)
	case scyllav1.BroadcastAddressTypePodIP:
		if len(pod.Status.PodIP) == 0 {
			return nil, fmt.Errorf("pod %q does not have IP address, despite being set as broadcasted", naming.ObjRef(pod))
		}
		res = append(res, pod.Status.PodIP)
	default:
		return nil, fmt.Errorf("unsupported node broadcast address type %q", m.NodesBroadcastAddressType)
	}

	return res, nil
}
