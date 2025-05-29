package controllerhelpers

import (
	"context"

	"github.com/scylladb/scylla-operator/pkg/kubeinterfaces"
	"github.com/scylladb/scylla-operator/pkg/resource"
	"github.com/scylladb/scylla-operator/pkg/resourceapply"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apimachineryutilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
)

type PruneControlInterface interface {
	Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error
}

type PruneControlFuncs struct {
	DeleteFunc func(ctx context.Context, name string, opts metav1.DeleteOptions) error
}

func (pcf *PruneControlFuncs) Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	return pcf.DeleteFunc(ctx, name, opts)
}

var _ PruneControlInterface = &PruneControlFuncs{}

func Prune[T kubeinterfaces.ObjectInterface](ctx context.Context, requiredObjects []T, existingObjects map[string]T, control PruneControlInterface, eventRecorder record.EventRecorder) error {
	var errs []error

	for _, existing := range existingObjects {
		if existing.GetDeletionTimestamp() != nil {
			continue
		}

		isRequired := false
		for _, required := range requiredObjects {
			if existing.GetName() == required.GetName() {
				isRequired = true
				break
			}
		}
		if isRequired {
			continue
		}

		uid := existing.GetUID()
		propagationPolicy := metav1.DeletePropagationBackground
		klog.V(2).InfoS("Pruning resource", "GVK", resource.GetObjectGVKOrUnknown(existing), "Ref", klog.KObj(existing))
		err := control.Delete(ctx, existing.GetName(), metav1.DeleteOptions{
			Preconditions: &metav1.Preconditions{
				UID: &uid,
			},
			PropagationPolicy: &propagationPolicy,
		})
		resourceapply.ReportDeleteEvent(eventRecorder, existing, err)
		if err != nil {
			errs = append(errs, err)
			continue
		}
	}

	return apimachineryutilerrors.NewAggregate(errs)
}
