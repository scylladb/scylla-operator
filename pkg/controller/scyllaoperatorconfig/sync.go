package scyllaoperatorconfig

import (
	"context"
	"fmt"

	"github.com/scylladb/scylla-operator/pkg/naming"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

func (opc *Controller) sync(ctx context.Context) error {
	soc, err := opc.scyllaOperatorConfigLister.Get(naming.SingletonName)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("can't get ScyllaOperatorConfig %q: %w", naming.SingletonName, err)
		}

		klog.V(2).InfoS("ScyllaOperatorConfig missing, creating a default one")

		_, err := opc.scyllaClient.ScyllaOperatorConfigs().Create(ctx, DefaultScyllaOperatorConfig(), metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("can't create scyllaoperatorconfig %q: %w", naming.SingletonName, err)
		}

		klog.V(2).InfoS("Create ScyllaOperatorConfig", "ScyllaOperatorConfig", klog.KObj(soc))

		// We need to wait for caches to see the new object.
		return nil
	}

	// TODO: Report status, like which scylla images / version are used in this cluster, ...

	return nil
}
