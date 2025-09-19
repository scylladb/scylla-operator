package scylladbmonitoring

import (
	"context"
	"crypto/x509/pkix"
	"fmt"
	"time"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	prometheusv1assets "github.com/scylladb/scylla-operator/assets/monitoring/prometheus/v1"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	ocrypto "github.com/scylladb/scylla-operator/pkg/crypto"
	"github.com/scylladb/scylla-operator/pkg/helpers"
	oslices "github.com/scylladb/scylla-operator/pkg/helpers/slices"
	okubecrypto "github.com/scylladb/scylla-operator/pkg/kubecrypto"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	"github.com/scylladb/scylla-operator/pkg/resource"
	"github.com/scylladb/scylla-operator/pkg/resourceapply"
	"github.com/scylladb/scylla-operator/pkg/resourcemerge"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	apimachineryutilerrors "k8s.io/apimachinery/pkg/util/errors"
)

func getPrometheusLabels(sm *scyllav1alpha1.ScyllaDBMonitoring) labels.Set {
	return helpers.MergeMaps(
		getLabels(sm),
		labels.Set{
			naming.ControllerNameLabel: "prometheus",
		},
	)
}

func getPrometheusSelector(sm *scyllav1alpha1.ScyllaDBMonitoring) labels.Selector {
	return labels.SelectorFromSet(getPrometheusLabels(sm))
}

func getPrometheusSpec(sm *scyllav1alpha1.ScyllaDBMonitoring) *scyllav1alpha1.PrometheusSpec {
	if sm.Spec.Components != nil {
		return sm.Spec.Components.Prometheus
	}

	return nil
}

func getPrometheusIngressOptions(sm *scyllav1alpha1.ScyllaDBMonitoring) *scyllav1alpha1.IngressOptions {
	spec := getPrometheusSpec(sm)
	if spec != nil &&
		spec.ExposeOptions != nil &&
		spec.ExposeOptions.WebInterface != nil {
		return spec.ExposeOptions.WebInterface.Ingress
	}

	return nil
}

func getPrometheusIngressDomains(sm *scyllav1alpha1.ScyllaDBMonitoring) []string {
	ingressOptions := getPrometheusIngressOptions(sm)
	if ingressOptions != nil {
		return ingressOptions.DNSDomains
	}

	return nil
}

func makePrometheusSA(sm *scyllav1alpha1.ScyllaDBMonitoring) (*corev1.ServiceAccount, string, error) {
	return prometheusv1assets.PrometheusSATemplate.Get().RenderObject(map[string]any{
		"namespace":              sm.Namespace,
		"scyllaDBMonitoringName": sm.Name,
	})
}

func makePrometheusRoleBinding(sm *scyllav1alpha1.ScyllaDBMonitoring) (*rbacv1.RoleBinding, string, error) {
	return prometheusv1assets.PrometheusRoleBindingTemplate.Get().RenderObject(map[string]any{
		"namespace":              sm.Namespace,
		"scyllaDBMonitoringName": sm.Name,
	})
}

func makePrometheusService(sm *scyllav1alpha1.ScyllaDBMonitoring) (*corev1.Service, string, error) {
	return prometheusv1assets.PrometheusServiceTemplate.Get().RenderObject(map[string]any{
		"namespace":              sm.Namespace,
		"scyllaDBMonitoringName": sm.Name,
	})
}

func makeScyllaDBServiceMonitor(sm *scyllav1alpha1.ScyllaDBMonitoring) (*monitoringv1.ServiceMonitor, string, error) {
	return prometheusv1assets.ScyllaDBServiceMonitorTemplate.Get().RenderObject(map[string]any{
		"scyllaDBMonitoringName": sm.Name,
		"endpointsSelector":      sm.Spec.EndpointsSelector,
	})
}

func makeLatencyPrometheusRule(sm *scyllav1alpha1.ScyllaDBMonitoring) (*monitoringv1.PrometheusRule, string, error) {
	const latencyRulesFile = "prometheus.latency.rules.yml"
	latencyRules, found := prometheusv1assets.PrometheusRules.Get()[latencyRulesFile]
	if !found {
		return nil, "", fmt.Errorf("can't find latency rules file %q in the assets", latencyRulesFile)
	}

	return prometheusv1assets.LatencyPrometheusRuleTemplate.Get().RenderObject(map[string]any{
		"scyllaDBMonitoringName": sm.Name,
		"groups":                 latencyRules.Get(),
	})
}

func makeAlertsPrometheusRule(sm *scyllav1alpha1.ScyllaDBMonitoring) (*monitoringv1.PrometheusRule, string, error) {
	const alertsRulesFile = "prometheus.rules.yml"
	rule, found := prometheusv1assets.PrometheusRules.Get()[alertsRulesFile]
	if !found {
		return nil, "", fmt.Errorf("can't find alerts rules file %q in the assets", alertsRulesFile)
	}

	return prometheusv1assets.AlertsPrometheusRuleTemplate.Get().RenderObject(map[string]any{
		"scyllaDBMonitoringName": sm.Name,
		"groups":                 rule.Get(),
	})
}

func makeTablePrometheusRule(sm *scyllav1alpha1.ScyllaDBMonitoring) (*monitoringv1.PrometheusRule, string, error) {
	const tableRulesFile = "prometheus.table.yml"
	rule, found := prometheusv1assets.PrometheusRules.Get()[tableRulesFile]
	if !found {
		return nil, "", fmt.Errorf("can't find table rules file %q in the assets", tableRulesFile)
	}

	return prometheusv1assets.TablePrometheusRuleTemplate.Get().RenderObject(map[string]any{
		"scyllaDBMonitoringName": sm.Name,
		"groups":                 rule.Get(),
	})
}

func makePrometheus(sm *scyllav1alpha1.ScyllaDBMonitoring, soc *scyllav1alpha1.ScyllaOperatorConfig) (*monitoringv1.Prometheus, string, error) {
	spec := getPrometheusSpec(sm)

	var volumeClaimTemplate *monitoringv1.EmbeddedPersistentVolumeClaim
	if spec != nil && spec.Storage != nil {
		volumeClaimTemplate = &monitoringv1.EmbeddedPersistentVolumeClaim{
			EmbeddedObjectMetadata: monitoringv1.EmbeddedObjectMetadata{
				Name:        fmt.Sprintf("%s-prometheus", sm.Name),
				Labels:      spec.Storage.VolumeClaimTemplate.Labels,
				Annotations: spec.Storage.VolumeClaimTemplate.Annotations,
			},
			Spec: spec.Storage.VolumeClaimTemplate.Spec,
		}

	}

	affinity := corev1.Affinity{}
	var tolerations []corev1.Toleration
	if spec != nil && spec.Placement != nil {
		affinity.NodeAffinity = spec.Placement.NodeAffinity
		affinity.PodAffinity = spec.Placement.PodAffinity
		affinity.PodAntiAffinity = spec.Placement.PodAntiAffinity

		tolerations = spec.Placement.Tolerations
	}

	var resources corev1.ResourceRequirements
	if spec != nil {
		resources = spec.Resources
	}

	return prometheusv1assets.PrometheusTemplate.Get().RenderObject(map[string]any{
		"prometheusVersion":      soc.Status.PrometheusVersion,
		"namespace":              sm.Namespace,
		"scyllaDBMonitoringName": sm.Name,
		"volumeClaimTemplate":    volumeClaimTemplate,
		"affinity":               affinity,
		"tolerations":            tolerations,
		"resources":              resources,
	})
}

func makePrometheusIngress(sm *scyllav1alpha1.ScyllaDBMonitoring) (*networkingv1.Ingress, string, error) {
	ingressOptions := getPrometheusIngressOptions(sm)
	if ingressOptions == nil {
		return nil, "", nil
	}

	if ingressOptions.Disabled != nil && *ingressOptions.Disabled == true {
		return nil, "", nil
	}

	if len(ingressOptions.DNSDomains) == 0 {
		return nil, "", nil
	}

	return prometheusv1assets.PrometheusIngressTemplate.Get().RenderObject(map[string]any{
		"scyllaDBMonitoringName": sm.Name,
		"dnsDomains":             ingressOptions.DNSDomains,
		"ingressAnnotations":     ingressOptions.Annotations,
		"ingressClassName":       ingressOptions.IngressClassName,
	})
}

func (smc *Controller) syncPrometheus(
	ctx context.Context,
	sm *scyllav1alpha1.ScyllaDBMonitoring,
	soc *scyllav1alpha1.ScyllaOperatorConfig,
	configMaps map[string]*corev1.ConfigMap,
	secrets map[string]*corev1.Secret,
	services map[string]*corev1.Service,
	serviceAccounts map[string]*corev1.ServiceAccount,
	roleBindings map[string]*rbacv1.RoleBinding,
	ingresses map[string]*networkingv1.Ingress,
	prometheuses map[string]*monitoringv1.Prometheus,
	prometheusRules map[string]*monitoringv1.PrometheusRule,
	serviceMonitors map[string]*monitoringv1.ServiceMonitor,
) ([]metav1.Condition, error) {
	var progressingConditions []metav1.Condition

	managedPrometheusServiceCAConfigMapName, err := naming.ManagedPrometheusServingCAConfigMapName(sm)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't get managed Prometheus serving CA config map name: %w", err)
	}

	prometheusServingCertChainConfig := &okubecrypto.CertChainConfig{
		CAConfig: &okubecrypto.CAConfig{
			MetaConfig: okubecrypto.MetaConfig{
				Name:   managedPrometheusServiceCAConfigMapName,
				Labels: getPrometheusLabels(sm),
			},
			Validity: 10 * 365 * 24 * time.Hour,
			Refresh:  8 * 365 * 24 * time.Hour,
		},
		CABundleConfig: &okubecrypto.CABundleConfig{
			MetaConfig: okubecrypto.MetaConfig{
				Name:   managedPrometheusServiceCAConfigMapName,
				Labels: getPrometheusLabels(sm),
			},
		},
		CertConfigs: []*okubecrypto.CertificateConfig{
			{
				MetaConfig: okubecrypto.MetaConfig{
					Name:   fmt.Sprintf("%s-prometheus-serving-certs", sm.Name),
					Labels: getPrometheusLabels(sm),
				},
				Validity: 30 * 24 * time.Hour,
				Refresh:  20 * 24 * time.Hour,
				CertCreator: (&ocrypto.ServingCertCreatorConfig{
					Subject: pkix.Name{
						CommonName: "",
					},
					IPAddresses: nil,
					DNSNames: append(
						[]string{
							fmt.Sprintf("%s-prometheus", sm.Name),
							fmt.Sprintf("%s-prometheus.%s.svc", sm.Name, sm.Namespace),
						},
						getPrometheusIngressDomains(sm)...,
					),
				}).ToCreator(),
			},
		},
	}

	managedPrometheusClientGrafanaSecretName, err := naming.ManagedPrometheusClientGrafanaSecretName(sm)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't get managed Prometheus client Grafana secret name: %w", err)
	}

	prometheusClientCertChainConfig := &okubecrypto.CertChainConfig{
		CAConfig: &okubecrypto.CAConfig{
			MetaConfig: okubecrypto.MetaConfig{
				Name:   fmt.Sprintf("%s-prometheus-client-ca", sm.Name),
				Labels: getPrometheusLabels(sm),
			},
			Validity: 10 * 365 * 24 * time.Hour,
			Refresh:  8 * 365 * 24 * time.Hour,
		},
		CABundleConfig: &okubecrypto.CABundleConfig{
			MetaConfig: okubecrypto.MetaConfig{
				Name:   fmt.Sprintf("%s-prometheus-client-ca", sm.Name),
				Labels: getPrometheusLabels(sm),
			},
		},
		CertConfigs: []*okubecrypto.CertificateConfig{
			{
				MetaConfig: okubecrypto.MetaConfig{
					Name:   managedPrometheusClientGrafanaSecretName,
					Labels: getPrometheusLabels(sm),
				},
				Validity: 10 * 365 * 24 * time.Hour,
				Refresh:  8 * 365 * 24 * time.Hour,
				CertCreator: (&ocrypto.ClientCertCreatorConfig{
					Subject: pkix.Name{
						CommonName: "",
					},
					DNSNames: []string{"grafana"},
				}).ToCreator(),
			},
		},
	}

	certChainConfigs := okubecrypto.CertChainConfigs{
		prometheusServingCertChainConfig,
		prometheusClientCertChainConfig,
	}

	// Render manifests.
	requiredResources, err := makeRequiredPrometheusResources(sm, soc)
	if err != nil {
		return progressingConditions, err
	}

	// Prune objects.
	var pruneErrors []error

	err = controllerhelpers.Prune(
		ctx,
		oslices.FilterOutNil(oslices.ToSlice(requiredResources.ServiceAccount)),
		serviceAccounts,
		&controllerhelpers.PruneControlFuncs{
			DeleteFunc: smc.kubeClient.CoreV1().ServiceAccounts(sm.Namespace).Delete,
		},
		smc.eventRecorder,
	)
	pruneErrors = append(pruneErrors, err)

	err = controllerhelpers.Prune(
		ctx,
		oslices.FilterOutNil(oslices.ToSlice(requiredResources.Service)),
		services,
		&controllerhelpers.PruneControlFuncs{
			DeleteFunc: smc.kubeClient.CoreV1().Services(sm.Namespace).Delete,
		},
		smc.eventRecorder,
	)
	pruneErrors = append(pruneErrors, err)

	err = controllerhelpers.Prune(
		ctx,
		oslices.FilterOutNil(oslices.ToSlice(requiredResources.RoleBinding)),
		roleBindings,
		&controllerhelpers.PruneControlFuncs{
			DeleteFunc: smc.kubeClient.RbacV1().RoleBindings(sm.Namespace).Delete,
		},
		smc.eventRecorder,
	)
	pruneErrors = append(pruneErrors, err)

	err = controllerhelpers.Prune(
		ctx,
		oslices.FilterOutNil(oslices.ToSlice(requiredResources.Prometheus)),
		prometheuses,
		&controllerhelpers.PruneControlFuncs{
			DeleteFunc: smc.monitoringClient.Prometheuses(sm.Namespace).Delete,
		},
		smc.eventRecorder,
	)
	pruneErrors = append(pruneErrors, err)

	err = controllerhelpers.Prune(
		ctx,
		oslices.FilterOutNil(oslices.ToSlice(requiredResources.Ingress)),
		ingresses,
		&controllerhelpers.PruneControlFuncs{
			DeleteFunc: smc.kubeClient.NetworkingV1().Ingresses(sm.Namespace).Delete,
		},
		smc.eventRecorder,
	)
	pruneErrors = append(pruneErrors, err)

	err = controllerhelpers.Prune(
		ctx,
		oslices.FilterOutNil(oslices.ToSlice(requiredResources.AlertsPrometheusRule, requiredResources.LatencyPrometheusRule, requiredResources.TablePrometheusRule)),
		prometheusRules,
		&controllerhelpers.PruneControlFuncs{
			DeleteFunc: smc.monitoringClient.PrometheusRules(sm.Namespace).Delete,
		},
		smc.eventRecorder,
	)
	pruneErrors = append(pruneErrors, err)

	err = controllerhelpers.Prune(
		ctx,
		oslices.FilterOutNil(oslices.ToSlice(requiredResources.ScyllaDBServiceMonitor)),
		serviceMonitors,
		&controllerhelpers.PruneControlFuncs{
			DeleteFunc: smc.monitoringClient.ServiceMonitors(sm.Namespace).Delete,
		},
		smc.eventRecorder,
	)
	pruneErrors = append(pruneErrors, err)

	err = controllerhelpers.Prune(
		ctx,
		oslices.FilterOutNil(certChainConfigs.GetMetaSecrets()),
		secrets,
		&controllerhelpers.PruneControlFuncs{
			DeleteFunc: smc.kubeClient.CoreV1().Secrets(sm.Namespace).Delete,
		},
		smc.eventRecorder,
	)
	pruneErrors = append(pruneErrors, err)

	err = controllerhelpers.Prune(
		ctx,
		oslices.FilterOutNil(certChainConfigs.GetMetaConfigMaps()),
		configMaps,
		&controllerhelpers.PruneControlFuncs{
			DeleteFunc: smc.kubeClient.CoreV1().ConfigMaps(sm.Namespace).Delete,
		},
		smc.eventRecorder,
	)
	pruneErrors = append(pruneErrors, err)

	pruneError := apimachineryutilerrors.NewAggregate(pruneErrors)
	if pruneError != nil {
		return progressingConditions, pruneError
	}

	// Apply required objects.
	var applyErrors []error
	var applyConfigurations []resourceapply.ApplyConfigUntyped
	if requiredResources.ServiceAccount != nil {
		applyConfigurations = append(applyConfigurations, resourceapply.ApplyConfig[*corev1.ServiceAccount]{
			Required: requiredResources.ServiceAccount,
			Control: resourceapply.ApplyControlFuncs[*corev1.ServiceAccount]{
				GetCachedFunc: smc.serviceAccountLister.ServiceAccounts(sm.Namespace).Get,
				CreateFunc:    smc.kubeClient.CoreV1().ServiceAccounts(sm.Namespace).Create,
				UpdateFunc:    smc.kubeClient.CoreV1().ServiceAccounts(sm.Namespace).Update,
				DeleteFunc:    smc.kubeClient.CoreV1().ServiceAccounts(sm.Namespace).Delete,
			},
		}.ToUntyped())
	}
	if requiredResources.Service != nil {
		applyConfigurations = append(applyConfigurations, resourceapply.ApplyConfig[*corev1.Service]{
			Required: requiredResources.Service,
			Control: resourceapply.ApplyControlFuncs[*corev1.Service]{
				GetCachedFunc: smc.serviceLister.Services(sm.Namespace).Get,
				CreateFunc:    smc.kubeClient.CoreV1().Services(sm.Namespace).Create,
				UpdateFunc:    smc.kubeClient.CoreV1().Services(sm.Namespace).Update,
			},
		}.ToUntyped())
	}
	if requiredResources.RoleBinding != nil {
		requiredPrometheusRoleBinding := requiredResources.RoleBinding
		applyConfigurations = append(applyConfigurations, resourceapply.ApplyConfig[*rbacv1.RoleBinding]{
			Required: requiredPrometheusRoleBinding,
			Control: resourceapply.ApplyControlFuncs[*rbacv1.RoleBinding]{
				GetCachedFunc: smc.roleBindingLister.RoleBindings(sm.Namespace).Get,
				CreateFunc:    smc.kubeClient.RbacV1().RoleBindings(sm.Namespace).Create,
				UpdateFunc:    smc.kubeClient.RbacV1().RoleBindings(sm.Namespace).Update,
				DeleteFunc:    smc.kubeClient.RbacV1().RoleBindings(sm.Namespace).Delete,
			},
		}.ToUntyped())
	}
	if requiredResources.Prometheus != nil {
		requiredPrometheus := requiredResources.Prometheus
		applyConfigurations = append(applyConfigurations, resourceapply.ApplyConfig[*monitoringv1.Prometheus]{
			Required: requiredPrometheus,
			Control: resourceapply.ApplyControlFuncs[*monitoringv1.Prometheus]{
				GetCachedFunc: smc.prometheusLister.Prometheuses(sm.Namespace).Get,
				CreateFunc:    smc.monitoringClient.Prometheuses(sm.Namespace).Create,
				UpdateFunc:    smc.monitoringClient.Prometheuses(sm.Namespace).Update,
				DeleteFunc:    smc.monitoringClient.Prometheuses(sm.Namespace).Delete,
			},
		}.ToUntyped())
	}
	if requiredResources.ScyllaDBServiceMonitor != nil {
		requiredScyllaDBServiceMonitor := requiredResources.ScyllaDBServiceMonitor
		applyConfigurations = append(applyConfigurations, resourceapply.ApplyConfig[*monitoringv1.ServiceMonitor]{
			Required: requiredScyllaDBServiceMonitor,
			Control: resourceapply.ApplyControlFuncs[*monitoringv1.ServiceMonitor]{
				GetCachedFunc: smc.serviceMonitorLister.ServiceMonitors(sm.Namespace).Get,
				CreateFunc:    smc.monitoringClient.ServiceMonitors(sm.Namespace).Create,
				UpdateFunc:    smc.monitoringClient.ServiceMonitors(sm.Namespace).Update,
				DeleteFunc:    smc.monitoringClient.ServiceMonitors(sm.Namespace).Delete,
			},
		}.ToUntyped())
	}
	if requiredResources.LatencyPrometheusRule != nil {
		requiredLatencyPrometheusRule := requiredResources.LatencyPrometheusRule
		applyConfigurations = append(applyConfigurations, resourceapply.ApplyConfig[*monitoringv1.PrometheusRule]{
			Required: requiredLatencyPrometheusRule,
			Control: resourceapply.ApplyControlFuncs[*monitoringv1.PrometheusRule]{
				GetCachedFunc: smc.prometheusRuleLister.PrometheusRules(sm.Namespace).Get,
				CreateFunc:    smc.monitoringClient.PrometheusRules(sm.Namespace).Create,
				UpdateFunc:    smc.monitoringClient.PrometheusRules(sm.Namespace).Update,
				DeleteFunc:    smc.monitoringClient.PrometheusRules(sm.Namespace).Delete,
			},
		}.ToUntyped())
	}
	if requiredResources.AlertsPrometheusRule != nil {
		requiredAlertsPrometheusRule := requiredResources.AlertsPrometheusRule
		applyConfigurations = append(applyConfigurations, resourceapply.ApplyConfig[*monitoringv1.PrometheusRule]{
			Required: requiredAlertsPrometheusRule,
			Control: resourceapply.ApplyControlFuncs[*monitoringv1.PrometheusRule]{
				GetCachedFunc: smc.prometheusRuleLister.PrometheusRules(sm.Namespace).Get,
				CreateFunc:    smc.monitoringClient.PrometheusRules(sm.Namespace).Create,
				UpdateFunc:    smc.monitoringClient.PrometheusRules(sm.Namespace).Update,
				DeleteFunc:    smc.monitoringClient.PrometheusRules(sm.Namespace).Delete,
			},
		}.ToUntyped())
	}
	if requiredResources.TablePrometheusRule != nil {
		requiredTablePrometheusRule := requiredResources.TablePrometheusRule
		applyConfigurations = append(applyConfigurations, resourceapply.ApplyConfig[*monitoringv1.PrometheusRule]{
			Required: requiredTablePrometheusRule,
			Control: resourceapply.ApplyControlFuncs[*monitoringv1.PrometheusRule]{
				GetCachedFunc: smc.prometheusRuleLister.PrometheusRules(sm.Namespace).Get,
				CreateFunc:    smc.monitoringClient.PrometheusRules(sm.Namespace).Create,
				UpdateFunc:    smc.monitoringClient.PrometheusRules(sm.Namespace).Update,
				DeleteFunc:    smc.monitoringClient.PrometheusRules(sm.Namespace).Delete,
			},
		}.ToUntyped())
	}
	if requiredResources.Ingress != nil {
		requiredIngress := requiredResources.Ingress
		applyConfigurations = append(applyConfigurations, resourceapply.ApplyConfig[*networkingv1.Ingress]{
			Required: requiredIngress,
			Control: resourceapply.ApplyControlFuncs[*networkingv1.Ingress]{
				GetCachedFunc: smc.ingressLister.Ingresses(sm.Namespace).Get,
				CreateFunc:    smc.kubeClient.NetworkingV1().Ingresses(sm.Namespace).Create,
				UpdateFunc:    smc.kubeClient.NetworkingV1().Ingresses(sm.Namespace).Update,
				DeleteFunc:    smc.kubeClient.NetworkingV1().Ingresses(sm.Namespace).Delete,
			},
		}.ToUntyped())
	}

	for _, cfg := range applyConfigurations {
		// Enforce namespace.
		cfg.Required.SetNamespace(sm.Namespace)

		// Enforce labels for selection.
		if cfg.Required.GetLabels() == nil {
			cfg.Required.SetLabels(getPrometheusLabels(sm))
		} else {
			resourcemerge.MergeMapInPlaceWithoutRemovalKeys(cfg.Required.GetLabels(), getPrometheusLabels(sm))
		}

		// Set ControllerRef.
		cfg.Required.SetOwnerReferences([]metav1.OwnerReference{
			{
				APIVersion:         scylladbMonitoringControllerGVK.GroupVersion().String(),
				Kind:               scylladbMonitoringControllerGVK.Kind,
				Name:               sm.Name,
				UID:                sm.UID,
				Controller:         pointer.Ptr(true),
				BlockOwnerDeletion: pointer.Ptr(true),
			},
		})

		// Apply required object.
		_, changed, err := resourceapply.ApplyFromConfig(ctx, cfg, smc.eventRecorder)
		if changed {
			controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, prometheusControllerProgressingCondition, cfg.Required, "apply", sm.Generation)
		}
		if err != nil {
			gvk := resource.GetObjectGVKOrUnknown(cfg.Required)
			applyErrors = append(applyErrors, fmt.Errorf("can't apply %s: %w", gvk, err))
		}
	}

	cm := okubecrypto.NewCertificateManager(
		smc.keyGetter,
		smc.kubeClient.CoreV1(),
		smc.secretLister,
		smc.kubeClient.CoreV1(),
		smc.configMapLister,
		smc.eventRecorder,
	)
	for _, ccc := range certChainConfigs {
		applyErrors = append(applyErrors, cm.ManageCertificateChain(
			ctx,
			time.Now,
			&sm.ObjectMeta,
			scylladbMonitoringControllerGVK,
			ccc,
			secrets,
			configMaps,
		))
	}

	applyError := apimachineryutilerrors.NewAggregate(applyErrors)
	if applyError != nil {
		return progressingConditions, applyError
	}

	return progressingConditions, nil
}

// requiredPrometheusResources holds all resources required for Prometheus deployment.
// Some of them may be nil, depending on the Prometheus mode.
type requiredPrometheusResources struct {
	ServiceAccount         *corev1.ServiceAccount
	RoleBinding            *rbacv1.RoleBinding
	Service                *corev1.Service
	Ingress                *networkingv1.Ingress
	Prometheus             *monitoringv1.Prometheus
	LatencyPrometheusRule  *monitoringv1.PrometheusRule
	AlertsPrometheusRule   *monitoringv1.PrometheusRule
	TablePrometheusRule    *monitoringv1.PrometheusRule
	ScyllaDBServiceMonitor *monitoringv1.ServiceMonitor
}

func makeRequiredPrometheusResources(sm *scyllav1alpha1.ScyllaDBMonitoring, soc *scyllav1alpha1.ScyllaOperatorConfig) (requiredPrometheusResources, error) {
	var renderErrors []error
	var resources requiredPrometheusResources

	var err error
	switch prometheusMode(sm) {
	case scyllav1alpha1.PrometheusModeManaged:
		resources.ServiceAccount, _, err = makePrometheusSA(sm)
		renderErrors = append(renderErrors, err)

		resources.RoleBinding, _, err = makePrometheusRoleBinding(sm)
		renderErrors = append(renderErrors, err)

		resources.Service, _, err = makePrometheusService(sm)
		renderErrors = append(renderErrors, err)

		resources.Ingress, _, err = makePrometheusIngress(sm)
		renderErrors = append(renderErrors, err)

		resources.Prometheus, _, err = makePrometheus(sm, soc)
		renderErrors = append(renderErrors, err)
	case scyllav1alpha1.PrometheusModeExternal:
		// No resources required.
	default:
		return requiredPrometheusResources{}, fmt.Errorf("unknown Prometheus mode %q", prometheusMode(sm))
	}

	resources.LatencyPrometheusRule, _, err = makeLatencyPrometheusRule(sm)
	renderErrors = append(renderErrors, err)

	resources.AlertsPrometheusRule, _, err = makeAlertsPrometheusRule(sm)
	renderErrors = append(renderErrors, err)

	resources.TablePrometheusRule, _, err = makeTablePrometheusRule(sm)
	renderErrors = append(renderErrors, err)

	resources.ScyllaDBServiceMonitor, _, err = makeScyllaDBServiceMonitor(sm)
	renderErrors = append(renderErrors, err)

	return resources, apimachineryutilerrors.NewAggregate(renderErrors)
}

func prometheusMode(sm *scyllav1alpha1.ScyllaDBMonitoring) scyllav1alpha1.PrometheusMode {
	if sm.Spec.Components.Prometheus != nil {
		return sm.Spec.Components.Prometheus.Mode
	}

	// By default, Prometheus is managed by the ScyllaDB Operator.
	return scyllav1alpha1.PrometheusModeManaged
}
