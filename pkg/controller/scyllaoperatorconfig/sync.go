package scyllaoperatorconfig

import (
	"context"

	"github.com/scylladb/scylla-operator/pkg/controller/scyllaoperatorconfig/resource"
	"github.com/scylladb/scylla-operator/pkg/naming"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

func (opc *Controller) sync(ctx context.Context, key string) error {
	_, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		klog.ErrorS(err, "Failed to split meta namespace cache key", "cacheKey", key)
		return err
	}

	if name != naming.ScyllaOperatorName {
		klog.InfoS("ScyllaOperatorConfig is a singleton, please use/modify the default one", "ScyllaOperatorConfig", naming.ScyllaOperatorName)
		return nil
	}

	_, err = opc.scyllaOperatorConfigLister.Get(name)
	if apierrors.IsNotFound(err) {
		klog.InfoS("ScyllaOperatorConfig has been deleted, creating a default one")
		_, err := opc.scyllaClient.ScyllaOperatorConfigs().Create(ctx, resource.DefaultScyllaOperatorConfig(), metav1.CreateOptions{})
		if err != nil {
			klog.ErrorS(err, "Failed to create default ScyllaOperatorConfig")
			return err
		}

		return nil
	}
	if err != nil {
		return err
	}

	// So far nothing to reconcile.
	return nil
}
