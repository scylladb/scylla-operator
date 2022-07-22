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

func (smc *Controller) syncSecrets(ctx context.Context, sm *v1alpha1.ScyllaManager, secrets map[string]*v1.Secret) error {
	requiredSecret, err := MakeSecret(sm)
	if err != nil {
		return fmt.Errorf("can't make secret: %v", err)
	}

	var deletionErrors []error
	for _, secret := range secrets {
		if secret.DeletionTimestamp != nil {
			continue
		}

		if secret.Name == requiredSecret.Name {
			continue
		}

		propagationPolicy := metav1.DeletePropagationBackground
		err = smc.kubeClient.CoreV1().ConfigMaps(secret.Namespace).Delete(ctx, secret.Name, metav1.DeleteOptions{
			Preconditions: &metav1.Preconditions{
				UID: &secret.UID,
			},
			PropagationPolicy: &propagationPolicy,
		})
		deletionErrors = append(deletionErrors, err)
	}
	err = utilerrors.NewAggregate(deletionErrors)
	if err != nil {
		return fmt.Errorf("can't delete secret(s): %v", err)
	}

	_, _, err = resourceapply.ApplySecret(ctx, smc.kubeClient.CoreV1(), smc.secretLister, smc.eventRecorder, requiredSecret, true)
	if err != nil {
		return fmt.Errorf("can't apply secret %v", err)
	}

	return nil
}
