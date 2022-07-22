package managerv2

import (
	"context"
	"fmt"

	"github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/resourceapply"
	v1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

func (smc *Controller) syncDeployments(
	ctx context.Context,
	sm *v1alpha1.ScyllaManager,
	deployments map[string]*v1.Deployment,
) error {
	var err error

	requiredDeployment := MakeDeployment(sm)

	var deletionErrors []error
	for _, deployment := range deployments {
		if deployment.DeletionTimestamp != nil {
			continue
		}

		if deployment.Name == requiredDeployment.Name {
			continue
		}

		propagationPolicy := metav1.DeletePropagationBackground
		err = smc.kubeClient.AppsV1().Deployments(deployment.Namespace).Delete(ctx, deployment.Name, metav1.DeleteOptions{
			Preconditions: &metav1.Preconditions{
				UID: &deployment.UID,
			},
			PropagationPolicy: &propagationPolicy,
		})
		deletionErrors = append(deletionErrors, err)
	}
	err = utilerrors.NewAggregate(deletionErrors)
	if err != nil {
		return fmt.Errorf("can't delete deployment(s): %v", err)
	}

	_, _, err = resourceapply.ApplyDeployment(ctx, smc.kubeClient.AppsV1(), smc.deploymentLister, smc.eventRecorder, requiredDeployment)
	if err != nil {
		return fmt.Errorf("can't apply deployment %v", err)
	}

	return nil
}
