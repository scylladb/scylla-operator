package identity

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/pkg/errors"
	"github.com/scylladb/scylla-operator/pkg/controller/helpers"
	"github.com/scylladb/scylla-operator/pkg/naming"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
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
	Datacenter    string
	Cluster       string
	ServiceLabels map[string]string
}

func Retrieve(ctx context.Context, name, namespace string, kubeclient kubernetes.Interface) (*Member, error) {
	// Get the member's service
	var memberService *corev1.Service
	var err error
	const maxRetryCount = 5
	for retryCount := 0; ; retryCount++ {
		memberService, err = kubeclient.CoreV1().Services(namespace).Get(ctx, name, metav1.GetOptions{})
		if err == nil {
			break
		}
		if retryCount > maxRetryCount {
			return nil, errors.Wrap(err, "failed to get memberservice")
		}
		time.Sleep(500 * time.Millisecond)
	}

	// Get the pod's ip
	pod, err := kubeclient.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "failed to get pod")
	}

	return &Member{
		Name:          name,
		Namespace:     namespace,
		IP:            pod.Status.PodIP,
		StaticIP:      memberService.Spec.ClusterIP,
		Rack:          pod.Labels[naming.RackNameLabel],
		Datacenter:    pod.Labels[naming.DatacenterNameLabel],
		Cluster:       pod.Labels[naming.ClusterNameLabel],
		ServiceLabels: memberService.Labels,
	}, nil
}

func (m *Member) GetSeed(ctx context.Context, coreClient v1.CoreV1Interface) (string, error) {
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
		if helpers.IsPodReady(otherPods[i]) && !helpers.IsPodReady(otherPods[j]) {
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
