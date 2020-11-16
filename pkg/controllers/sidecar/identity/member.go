package identity

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"
	"github.com/scylladb/scylla-operator/pkg/naming"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
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

func (m *Member) GetSeeds(ctx context.Context, kubeClient kubernetes.Interface) ([]string, error) {
	var services *corev1.ServiceList
	var err error

	sel := fmt.Sprintf("%s,%s=%s", naming.SeedLabel, naming.ClusterNameLabel, m.Cluster)

	const maxRetryCount = 5
	for retryCount := 0; ; retryCount++ {
		services, err = kubeClient.CoreV1().Services(m.Namespace).List(ctx, metav1.ListOptions{LabelSelector: sel})
		if err == nil && len(services.Items) > 0 {
			break
		}
		if retryCount > 5 {
			return nil, errors.New(fmt.Sprintf("failed to get seeds, error: %+v, len(services): %d", err, len(services.Items)))
		}
		time.Sleep(1000 * time.Millisecond)
	}

	seeds := []string{}
	for _, svc := range services.Items {
		seeds = append(seeds, svc.Spec.ClusterIP)
	}
	return seeds, nil
}
