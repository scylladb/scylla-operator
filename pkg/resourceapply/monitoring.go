package resourceapply

import (
	"context"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	monitoringv1listers "github.com/prometheus-operator/prometheus-operator/pkg/client/listers/monitoring/v1"
	monitoringv1client "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned/typed/monitoring/v1"
	"k8s.io/client-go/tools/record"
)

func ApplyPrometheusWithControl(
	ctx context.Context,
	control ApplyControlInterface[*monitoringv1.Prometheus],
	recorder record.EventRecorder,
	required *monitoringv1.Prometheus,
	options ApplyOptions,
) (*monitoringv1.Prometheus, bool, error) {
	return ApplyGeneric[*monitoringv1.Prometheus](ctx, control, recorder, required, options)
}

func ApplyPrometheus(
	ctx context.Context,
	client monitoringv1client.PrometheusesGetter,
	lister monitoringv1listers.PrometheusLister,
	recorder record.EventRecorder,
	required *monitoringv1.Prometheus,
	options ApplyOptions,
) (*monitoringv1.Prometheus, bool, error) {
	return ApplyPrometheusWithControl(
		ctx,
		ApplyControlFuncs[*monitoringv1.Prometheus]{
			GetCachedFunc: lister.Prometheuses(required.Namespace).Get,
			CreateFunc:    client.Prometheuses(required.Namespace).Create,
			UpdateFunc:    client.Prometheuses(required.Namespace).Update,
			DeleteFunc:    client.Prometheuses(required.Namespace).Delete,
		},
		recorder,
		required,
		options,
	)
}

func ApplyPrometheusRuleWithControl(
	ctx context.Context,
	control ApplyControlInterface[*monitoringv1.PrometheusRule],
	recorder record.EventRecorder,
	required *monitoringv1.PrometheusRule,
	options ApplyOptions,
) (*monitoringv1.PrometheusRule, bool, error) {
	return ApplyGeneric[*monitoringv1.PrometheusRule](ctx, control, recorder, required, options)
}

func ApplyPrometheusRule(
	ctx context.Context,
	client monitoringv1client.PrometheusRulesGetter,
	lister monitoringv1listers.PrometheusRuleLister,
	recorder record.EventRecorder,
	required *monitoringv1.PrometheusRule,
	options ApplyOptions,
) (*monitoringv1.PrometheusRule, bool, error) {
	return ApplyPrometheusRuleWithControl(
		ctx,
		ApplyControlFuncs[*monitoringv1.PrometheusRule]{
			GetCachedFunc: lister.PrometheusRules(required.Namespace).Get,
			CreateFunc:    client.PrometheusRules(required.Namespace).Create,
			UpdateFunc:    client.PrometheusRules(required.Namespace).Update,
			DeleteFunc:    client.PrometheusRules(required.Namespace).Delete,
		},
		recorder,
		required,
		options,
	)
}

func ApplyServiceMonitorWithControl(
	ctx context.Context,
	control ApplyControlInterface[*monitoringv1.ServiceMonitor],
	recorder record.EventRecorder,
	required *monitoringv1.ServiceMonitor,
	options ApplyOptions,
) (*monitoringv1.ServiceMonitor, bool, error) {
	return ApplyGeneric[*monitoringv1.ServiceMonitor](ctx, control, recorder, required, options)
}

func ApplyServiceMonitor(
	ctx context.Context,
	client monitoringv1client.ServiceMonitorsGetter,
	lister monitoringv1listers.ServiceMonitorLister,
	recorder record.EventRecorder,
	required *monitoringv1.ServiceMonitor,
	options ApplyOptions,
) (*monitoringv1.ServiceMonitor, bool, error) {
	return ApplyServiceMonitorWithControl(
		ctx,
		ApplyControlFuncs[*monitoringv1.ServiceMonitor]{
			GetCachedFunc: lister.ServiceMonitors(required.Namespace).Get,
			CreateFunc:    client.ServiceMonitors(required.Namespace).Create,
			UpdateFunc:    client.ServiceMonitors(required.Namespace).Update,
			DeleteFunc:    client.ServiceMonitors(required.Namespace).Delete,
		},
		recorder,
		required,
		options,
	)
}
