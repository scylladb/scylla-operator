package bootstrapbarrier

import (
	"time"

	clientset "github.com/scylladb/scylla-operator/pkg/client/scylla/clientset/versioned"
	scyllainformers "github.com/scylladb/scylla-operator/pkg/client/scylla/informers/externalversions"
	scyllav1alpha1informers "github.com/scylladb/scylla-operator/pkg/client/scylla/informers/externalversions/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/naming"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	corev1informers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
)

type InformerFactory struct {
	identity                            informers.SharedInformerFactory
	scyllaDBDatacenterNodesStatusReport scyllainformers.SharedInformerFactory
}

type InformerFactoryOptions struct {
	ServiceName        string
	SelectorLabelValue string
	Namespace          string
}

func NewInformerFactory(
	kubeClient kubernetes.Interface,
	scyllaClient clientset.Interface,
	opts InformerFactoryOptions,
) *InformerFactory {
	identityKubeInformers := informers.NewSharedInformerFactoryWithOptions(
		kubeClient,
		12*time.Hour,
		informers.WithNamespace(opts.Namespace),
		informers.WithTweakListOptions(
			func(options *metav1.ListOptions) {
				options.FieldSelector = fields.OneTermEqualSelector("metadata.name", opts.ServiceName).String()
			},
		),
	)

	scyllaDBDatacenterNodesStatusReportKubeInformers := scyllainformers.NewSharedInformerFactoryWithOptions(
		scyllaClient,
		12*time.Hour,
		scyllainformers.WithNamespace(opts.Namespace),
		scyllainformers.WithTweakListOptions(
			func(options *metav1.ListOptions) {
				options.LabelSelector = labels.Set{
					naming.ScyllaDBDatacenterNodesStatusReportSelectorLabel: opts.SelectorLabelValue,
				}.String()
			},
		),
	)

	return &InformerFactory{
		identity:                            identityKubeInformers,
		scyllaDBDatacenterNodesStatusReport: scyllaDBDatacenterNodesStatusReportKubeInformers,
	}
}

func (b *InformerFactory) Services() corev1informers.ServiceInformer {
	return b.identity.Core().V1().Services()
}

func (b *InformerFactory) ScyllaDBDatacenterNodesStatusReports() scyllav1alpha1informers.ScyllaDBDatacenterNodesStatusReportInformer {
	return b.scyllaDBDatacenterNodesStatusReport.Scylla().V1alpha1().ScyllaDBDatacenterNodesStatusReports()
}

func (b *InformerFactory) Start(stopCh <-chan struct{}) {
	b.identity.Start(stopCh)
	b.scyllaDBDatacenterNodesStatusReport.Start(stopCh)
}
