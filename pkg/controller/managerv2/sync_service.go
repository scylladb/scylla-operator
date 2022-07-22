package managerv2

import (
	"context"
	"fmt"

	"github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/resourceapply"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

func (smc *Controller) syncServices(
	ctx context.Context,
	sm *v1alpha1.ScyllaManager,
	services map[string]*v1.Service,
) error {
	var err error

	requiredService := MakeService(sm)

	var deletionErrors []error
	for _, service := range services {
		if service.DeletionTimestamp != nil {
			continue
		}

		if service.Name == requiredService.Name {
			continue
		}

		propagationPolicy := metav1.DeletePropagationBackground
		err = smc.kubeClient.CoreV1().Services(service.Namespace).Delete(ctx, service.Name, metav1.DeleteOptions{
			Preconditions: &metav1.Preconditions{
				UID: &service.UID,
			},
			PropagationPolicy: &propagationPolicy,
		})
		deletionErrors = append(deletionErrors, err)
	}
	err = utilerrors.NewAggregate(deletionErrors)
	if err != nil {
		return fmt.Errorf("can't delete service(s): %v", err)
	}

	_, _, err = resourceapply.ApplyService(ctx, smc.kubeClient.CoreV1(), smc.serviceLister, smc.eventRecorder, requiredService, true)
	if err != nil {
		return fmt.Errorf("can't apply service %v", err)
	}

	return nil
}
