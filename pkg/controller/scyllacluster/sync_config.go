package scyllacluster

import (
	"context"
	"fmt"

	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/resourceapply"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/errors"
)

func (scc *Controller) syncConfigs(
	ctx context.Context,
	sc *scyllav1.ScyllaCluster,
) ([]metav1.Condition, error) {
	var errs []error
	var progressingConditions []metav1.Condition

	cm, err := MakeManagedScyllaDBConfig(sc)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't make managed scylladb config: %w", err)
	}

	_, changed, err := resourceapply.ApplyConfigMap(ctx, scc.kubeClient.CoreV1(), scc.configMapLister, scc.eventRecorder, cm, resourceapply.ApplyOptions{})
	if changed {
		controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, configControllerProgressingCondition, cm, "apply", sc.Generation)
	}
	if err != nil {
		return progressingConditions, fmt.Errorf("can't apply configmap %q: %w", naming.ObjRef(cm), err)
	}

	return progressingConditions, errors.NewAggregate(errs)
}
