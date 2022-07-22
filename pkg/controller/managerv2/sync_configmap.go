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

func (smc *Controller) syncConfigMaps(ctx context.Context, sm *v1alpha1.ScyllaManager, configMaps map[string]*v1.ConfigMap) error {
	requiredConfigMap, err := MakeConfigMap(sm)
	if err != nil {
		return fmt.Errorf("can't make configMap: %v", err)
	}

	var deletionErrors []error
	for _, configMap := range configMaps {
		if configMap.DeletionTimestamp != nil {
			continue
		}

		if configMap.Name == requiredConfigMap.Name {
			continue
		}

		propagationPolicy := metav1.DeletePropagationBackground
		err = smc.kubeClient.CoreV1().ConfigMaps(configMap.Namespace).Delete(ctx, configMap.Name, metav1.DeleteOptions{
			Preconditions: &metav1.Preconditions{
				UID: &configMap.UID,
			},
			PropagationPolicy: &propagationPolicy,
		})
		deletionErrors = append(deletionErrors, err)
	}
	err = utilerrors.NewAggregate(deletionErrors)
	if err != nil {
		return fmt.Errorf("can't delete configmap(s): %v", err)
	}

	_, _, err = resourceapply.ApplyConfigMap(ctx, smc.kubeClient.CoreV1(), smc.configMapLister, smc.eventRecorder, requiredConfigMap)
	if err != nil {
		return fmt.Errorf("can't apply configmap %v", err)
	}

	return nil
}
