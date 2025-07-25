package resourceapply

import (
	"context"
	"fmt"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/scylladb/scylla-operator/pkg/kubeinterfaces"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
)

type ApplyConfigUntyped struct {
	Required kubeinterfaces.ObjectInterface
	Options  ApplyOptions
	Control  ApplyControlUntypedInterface
}

type ApplyConfig[T kubeinterfaces.ObjectInterface] struct {
	Required T
	Options  ApplyOptions
	Control  ApplyControlFuncs[T]
}

func (ac ApplyConfig[T]) ToUntyped() ApplyConfigUntyped {
	return ApplyConfigUntyped{
		Required: ac.Required,
		Options:  ac.Options,
		Control:  ac.Control.ToUntyped(),
	}
}

func ApplyFromConfig(
	ctx context.Context,
	cfg ApplyConfigUntyped,
	recorder record.EventRecorder,
) (kubeinterfaces.ObjectInterface, bool, error) {
	return Apply(
		ctx,
		cfg.Required,
		cfg.Control,
		cfg.Options,
		recorder,
	)
}

func Apply(
	ctx context.Context,
	required kubeinterfaces.ObjectInterface,
	control ApplyControlUntypedInterface,
	options ApplyOptions,
	recorder record.EventRecorder,
) (kubeinterfaces.ObjectInterface, bool, error) {
	switch metav1.Object(required).(type) {
	case *corev1.Service:
		return ApplyServiceWithControl(
			ctx,
			TypeApplyControlInterface[*corev1.Service](control),
			recorder,
			required.(*corev1.Service),
			options,
		)

	case *corev1.ConfigMap:
		return ApplyConfigMapWithControl(
			ctx,
			TypeApplyControlInterface[*corev1.ConfigMap](control),
			recorder,
			required.(*corev1.ConfigMap),
			options,
		)

	case *corev1.Secret:
		return ApplySecretWithControl(
			ctx,
			TypeApplyControlInterface[*corev1.Secret](control),
			recorder,
			required.(*corev1.Secret),
			options,
		)

	case *corev1.ServiceAccount:
		return ApplyServiceAccountWithControl(
			ctx,
			TypeApplyControlInterface[*corev1.ServiceAccount](control),
			recorder,
			required.(*corev1.ServiceAccount),
			options,
		)

	case *rbacv1.RoleBinding:
		return ApplyRoleBindingWithControl(
			ctx,
			TypeApplyControlInterface[*rbacv1.RoleBinding](control),
			recorder,
			required.(*rbacv1.RoleBinding),
			options,
		)

	case *appsv1.Deployment:
		return ApplyDeploymentWithControl(
			ctx,
			TypeApplyControlInterface[*appsv1.Deployment](control),
			recorder,
			required.(*appsv1.Deployment),
			options,
		)

	case *networkingv1.Ingress:
		return ApplyIngressWithControl(
			ctx,
			TypeApplyControlInterface[*networkingv1.Ingress](control),
			recorder,
			required.(*networkingv1.Ingress),
			options,
		)

	case *monitoringv1.Prometheus:
		return ApplyPrometheusWithControl(
			ctx,
			TypeApplyControlInterface[*monitoringv1.Prometheus](control),
			recorder,
			required.(*monitoringv1.Prometheus),
			options,
		)

	case *monitoringv1.PrometheusRule:
		return ApplyPrometheusRuleWithControl(
			ctx,
			TypeApplyControlInterface[*monitoringv1.PrometheusRule](control),
			recorder,
			required.(*monitoringv1.PrometheusRule),
			options,
		)

	case *monitoringv1.ServiceMonitor:
		return ApplyServiceMonitorWithControl(
			ctx,
			TypeApplyControlInterface[*monitoringv1.ServiceMonitor](control),
			recorder,
			required.(*monitoringv1.ServiceMonitor),
			options,
		)

	default:
		return nil, false, fmt.Errorf("no apply method matched for type %T", required)
	}
}
