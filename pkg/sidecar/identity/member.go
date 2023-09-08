package identity

import (
	"context"
	"fmt"
	"sort"
	"strconv"

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
	// IP of the Pod
	IP string
	// ClusterIP of the member's Service
	StaticIP      string
	Rack          string
	RackOrdinal   int
	Datacenter    string
	Cluster       string
	ServiceLabels map[string]string
	PodID         string

	Overprovisioned bool
}

func NewMemberFromObjects(service *corev1.Service, pod *corev1.Pod) (*Member, error) {
	rackOrdinalString, ok := pod.Labels[naming.RackOrdinalLabel]
	if !ok {
		return nil, fmt.Errorf("pod %q is missing %q label", naming.ObjRef(pod), naming.RackOrdinalLabel)
	}
	rackOrdinal, err := strconv.Atoi(rackOrdinalString)
	if err != nil {
		return nil, fmt.Errorf("can't get rack ordinal from label %q: %w", rackOrdinalString, err)
	}

	return &Member{
		Namespace:       service.Namespace,
		Name:            service.Name,
		IP:              pod.Status.PodIP,
		StaticIP:        service.Spec.ClusterIP,
		Rack:            pod.Labels[naming.RackNameLabel],
		RackOrdinal:     rackOrdinal,
		Datacenter:      pod.Labels[naming.DatacenterNameLabel],
		Cluster:         pod.Labels[naming.ClusterNameLabel],
		ServiceLabels:   service.Labels,
		PodID:           string(pod.UID),
		Overprovisioned: pod.Status.QOSClass != corev1.PodQOSGuaranteed,
	}, nil
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

		return []string{m.StaticIP}, nil
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
	res = append(res, svc.Spec.ClusterIP)

	return res, nil
}
