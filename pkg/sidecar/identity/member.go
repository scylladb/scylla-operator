package identity

import (
	"context"
	"fmt"
	"sort"

	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/scyllaclient"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/klog/v2"
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
	Datacenter    string
	Cluster       string
	ServiceLabels map[string]string
	PodID         string

	Overprovisioned bool
}

func NewMemberFromObjects(service *corev1.Service, pod *corev1.Pod) *Member {
	return &Member{
		Namespace:       service.Namespace,
		Name:            service.Name,
		IP:              pod.Status.PodIP,
		StaticIP:        service.Spec.ClusterIP,
		Rack:            pod.Labels[naming.RackNameLabel],
		Datacenter:      pod.Labels[naming.DatacenterNameLabel],
		Cluster:         pod.Labels[naming.ClusterNameLabel],
		ServiceLabels:   service.Labels,
		PodID:           string(pod.UID),
		Overprovisioned: pod.Status.QOSClass != corev1.PodQOSGuaranteed,
	}
}

const ManualInitSeed = "scylla-init-seed"
const SeedKey = "seed"

func (m *Member) GetSeed(ctx context.Context, coreClient v1.CoreV1Interface, checkSeed func(ctx context.Context, seed string) (scyllaclient.NodeStatusInfoSlice, error)) (string, error) {
	// check manual seed assigment by configmap
	seedConfig, err := coreClient.ConfigMaps(m.Namespace).Get(ctx, ManualInitSeed, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		klog.InfoS("manual seed assigment by scylla-init-seed configmap is not found, try list pods")
		goto pods
	}
	if err != nil {
		return "", err
	}
	if seed := seedConfig.Data[SeedKey]; seed == "" {
		klog.InfoS("manual seed assigment by scylla-init-seed configmap is empty, try list pods")
	} else if _, err = checkSeed(ctx, seed); err != nil {
		klog.InfoS("manual seed by scylla-init-seed configmap is failed, try list pods", "err", err)
	} else {
		return seed, nil
	}

pods:

	podList, err := coreClient.Pods(m.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(labels.Set{
			naming.ClusterNameLabel: m.Cluster,
		}).String(),
	})
	if err != nil {
		return "", err
	}

	if len(podList.Items) == 0 {
		return "", fmt.Errorf("internal error: can't find any pod for this cluster, including itself")
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
		return m.StaticIP, nil
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
		return "", err
	}

	return svc.Spec.ClusterIP, nil
}
