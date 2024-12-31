// Code generated by client-gen. DO NOT EDIT.

package v1

import (
	context "context"

	monitoringv1 "github.com/scylladb/scylla-operator/pkg/externalapi/monitoring/v1"
	scheme "github.com/scylladb/scylla-operator/pkg/externalclient/monitoring/clientset/versioned/scheme"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	gentype "k8s.io/client-go/gentype"
)

// PodMonitorsGetter has a method to return a PodMonitorInterface.
// A group's client should implement this interface.
type PodMonitorsGetter interface {
	PodMonitors(namespace string) PodMonitorInterface
}

// PodMonitorInterface has methods to work with PodMonitor resources.
type PodMonitorInterface interface {
	Create(ctx context.Context, podMonitor *monitoringv1.PodMonitor, opts metav1.CreateOptions) (*monitoringv1.PodMonitor, error)
	Update(ctx context.Context, podMonitor *monitoringv1.PodMonitor, opts metav1.UpdateOptions) (*monitoringv1.PodMonitor, error)
	Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Get(ctx context.Context, name string, opts metav1.GetOptions) (*monitoringv1.PodMonitor, error)
	List(ctx context.Context, opts metav1.ListOptions) (*monitoringv1.PodMonitorList, error)
	Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (result *monitoringv1.PodMonitor, err error)
	PodMonitorExpansion
}

// podMonitors implements PodMonitorInterface
type podMonitors struct {
	*gentype.ClientWithList[*monitoringv1.PodMonitor, *monitoringv1.PodMonitorList]
}

// newPodMonitors returns a PodMonitors
func newPodMonitors(c *MonitoringV1Client, namespace string) *podMonitors {
	return &podMonitors{
		gentype.NewClientWithList[*monitoringv1.PodMonitor, *monitoringv1.PodMonitorList](
			"podmonitors",
			c.RESTClient(),
			scheme.ParameterCodec,
			namespace,
			func() *monitoringv1.PodMonitor { return &monitoringv1.PodMonitor{} },
			func() *monitoringv1.PodMonitorList { return &monitoringv1.PodMonitorList{} },
		),
	}
}
