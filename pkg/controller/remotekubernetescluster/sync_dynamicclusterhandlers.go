package remotekubernetescluster

import (
	"context"
	"fmt"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/naming"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apimachineryutilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
)

func (rkcc *Controller) syncDynamicClusterHandlers(ctx context.Context, rkc *scyllav1alpha1.RemoteKubernetesCluster) ([]metav1.Condition, error) {
	var progressingConditions []metav1.Condition

	kubeConfigSecret, err := rkcc.secretLister.Secrets(rkc.Spec.KubeconfigSecretRef.Namespace).Get(rkc.Spec.KubeconfigSecretRef.Name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			klog.V(4).InfoS("Secret referenced by RemoteKubernetesCluster doesn't exists", "Secret", naming.ManualRef(rkc.Spec.KubeconfigSecretRef.Namespace, rkc.Spec.KubeconfigSecretRef.Name), "RemoteKubernetesCluster", rkc.Name)
			progressingConditions = append(progressingConditions, metav1.Condition{
				Type:               dynamicClusterHandlersControllerProgressingCondition,
				Status:             metav1.ConditionTrue,
				Reason:             "AwaitingSecret",
				Message:            fmt.Sprintf("Secret %q referenced by RemoteKubernetesCluster %q does not exist yet", naming.ManualRef(rkc.Spec.KubeconfigSecretRef.Namespace, rkc.Spec.KubeconfigSecretRef.Name), rkc.Name),
				ObservedGeneration: rkc.Generation,
			})

			return progressingConditions, nil
		}

		return progressingConditions, fmt.Errorf("can't get secret %q: %w", naming.ManualRef(rkc.Spec.KubeconfigSecretRef.Namespace, rkc.Spec.KubeconfigSecretRef.Name), err)

	}

	config, ok := kubeConfigSecret.Data[naming.KubeConfigSecretKey]
	if !ok {
		return progressingConditions, fmt.Errorf("secret %q referenced by RemoteKubernetesCluster %q doesn't have required key %q", naming.ManualRef(rkc.Spec.KubeconfigSecretRef.Namespace, rkc.Spec.KubeconfigSecretRef.Name), naming.ObjRef(rkc), naming.KubeConfigSecretKey)
	}

	var errs []error
	for _, dch := range rkcc.dynamicClusterHandlers {
		if err := dch.UpdateCluster(rkc.Name, config); err != nil {
			klog.ErrorS(err, "Failed to update dynamic cluster handler about RemoteKubernetesCluster update", "RemoteKubernetesCluster", klog.KObj(rkc))
			errs = append(errs, err)
		}
	}

	return progressingConditions, apimachineryutilerrors.NewAggregate(errs)
}
