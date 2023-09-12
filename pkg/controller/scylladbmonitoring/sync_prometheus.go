package scylladbmonitoring

import (
	"context"
	"crypto/x509/pkix"
	"fmt"
	"time"

	prometheusv1assets "github.com/scylladb/scylla-operator/assets/monitoring/prometheus/v1"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	ocrypto "github.com/scylladb/scylla-operator/pkg/crypto"
	monitoringv1 "github.com/scylladb/scylla-operator/pkg/externalapi/monitoring/v1"
	"github.com/scylladb/scylla-operator/pkg/helpers"
	"github.com/scylladb/scylla-operator/pkg/helpers/slices"
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
	kutilerrors "k8s.io/apimachinery/pkg/util/errors"
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
	return prometheusv1assets.PrometheusSATemplate.RenderObject(map[string]any{
		"namespace":              sm.Namespace,
		"scyllaDBMonitoringName": sm.Name,
	})
}

func makePrometheusRoleBinding(sm *scyllav1alpha1.ScyllaDBMonitoring) (*rbacv1.RoleBinding, string, error) {
	return prometheusv1assets.PrometheusRoleBindingTemplate.RenderObject(map[string]any{
		"namespace":              sm.Namespace,
		"scyllaDBMonitoringName": sm.Name,
	})
}

func makePrometheusService(sm *scyllav1alpha1.ScyllaDBMonitoring) (*corev1.Service, string, error) {
	return prometheusv1assets.PrometheusServiceTemplate.RenderObject(map[string]any{
		"namespace":              sm.Namespace,
		"scyllaDBMonitoringName": sm.Name,
	})
}

func makeScyllaDBServiceMonitor(sm *scyllav1alpha1.ScyllaDBMonitoring) (*monitoringv1.ServiceMonitor, string, error) {
	return prometheusv1assets.ScyllaDBServiceMonitorTemplate.RenderObject(map[string]any{
		"scyllaDBMonitoringName": sm.Name,
		"endpointsSelector":      sm.Spec.EndpointsSelector,
	})
}

func makeRecodingPrometheusRule(sm *scyllav1alpha1.ScyllaDBMonitoring) (*monitoringv1.PrometheusRule, string, error) {
	return prometheusv1assets.RecordingPrometheusRuleTemplate.RenderObject(map[string]any{
		"scyllaDBMonitoringName": sm.Name,
	})
}

func makeAlertsPrometheusRule(sm *scyllav1alpha1.ScyllaDBMonitoring) (*monitoringv1.PrometheusRule, string, error) {
	return prometheusv1assets.AlertsPrometheusRuleTemplate.RenderObject(map[string]any{
		"scyllaDBMonitoringName": sm.Name,
	})
}

func makePrometheus(sm *scyllav1alpha1.ScyllaDBMonitoring) (*monitoringv1.Prometheus, string, error) {
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

	return prometheusv1assets.PrometheusTemplate.RenderObject(map[string]any{
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

	return prometheusv1assets.PrometheusIngressTemplate.RenderObject(map[string]any{
		"scyllaDBMonitoringName": sm.Name,
		"dnsDomains":             ingressOptions.DNSDomains,
		"ingressAnnotations":     ingressOptions.Annotations,
		"ingressClassName":       ingressOptions.IngressClassName,
	})
}

func (smc *Controller) syncPrometheus(
	ctx context.Context,
	sm *scyllav1alpha1.ScyllaDBMonitoring,
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

	prometheusServingCertChainConfig := &okubecrypto.CertChainConfig{
		CAConfig: &okubecrypto.CAConfig{
			MetaConfig: okubecrypto.MetaConfig{
				Name:   fmt.Sprintf("%s-prometheus-serving-ca", sm.Name),
				Labels: getPrometheusLabels(sm),
			},
			Validity: 10 * 365 * 24 * time.Hour,
			Refresh:  8 * 365 * 24 * time.Hour,
		},
		CABundleConfig: &okubecrypto.CABundleConfig{
			MetaConfig: okubecrypto.MetaConfig{
				Name:   fmt.Sprintf("%s-prometheus-serving-ca", sm.Name),
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
					Name:   fmt.Sprintf("%s-prometheus-client-grafana", sm.Name),
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
	var renderErrors []error

	requiredPrometheusSA, _, err := makePrometheusSA(sm)
	renderErrors = append(renderErrors, err)

	requiredPrometheusRoleBinding, _, err := makePrometheusRoleBinding(sm)
	renderErrors = append(renderErrors, err)

	requiredPrometheusService, _, err := makePrometheusService(sm)
	renderErrors = append(renderErrors, err)

	requiredIngress, _, err := makePrometheusIngress(sm)
	renderErrors = append(renderErrors, err)

	requiredPrometheus, _, err := makePrometheus(sm)
	renderErrors = append(renderErrors, err)

	requiredRecodingPrometheusRule, _, err := makeRecodingPrometheusRule(sm)
	renderErrors = append(renderErrors, err)

	requiredAlertsPrometheusRule, _, err := makeAlertsPrometheusRule(sm)
	renderErrors = append(renderErrors, err)

	requiredScyllaDBServiceMonitor, _, err := makeScyllaDBServiceMonitor(sm)
	renderErrors = append(renderErrors, err)

	renderError := kutilerrors.NewAggregate(renderErrors)
	if renderError != nil {
		return progressingConditions, renderError
	}

	// Prune objects.
	var pruneErrors []error

	err = controllerhelpers.Prune(
		ctx,
		slices.ToSlice(requiredPrometheusSA),
		serviceAccounts,
		&controllerhelpers.PruneControlFuncs{
			DeleteFunc: smc.kubeClient.CoreV1().ServiceAccounts(sm.Namespace).Delete,
		},
		smc.eventRecorder,
	)
	pruneErrors = append(pruneErrors, err)

	err = controllerhelpers.Prune(
		ctx,
		slices.ToSlice(requiredPrometheusService),
		services,
		&controllerhelpers.PruneControlFuncs{
			DeleteFunc: smc.kubeClient.CoreV1().Services(sm.Namespace).Delete,
		},
		smc.eventRecorder,
	)
	pruneErrors = append(pruneErrors, err)

	err = controllerhelpers.Prune(
		ctx,
		slices.ToSlice(requiredPrometheusRoleBinding),
		roleBindings,
		&controllerhelpers.PruneControlFuncs{
			DeleteFunc: smc.kubeClient.RbacV1().RoleBindings(sm.Namespace).Delete,
		},
		smc.eventRecorder,
	)
	pruneErrors = append(pruneErrors, err)

	err = controllerhelpers.Prune(
		ctx,
		slices.ToSlice(requiredPrometheus),
		prometheuses,
		&controllerhelpers.PruneControlFuncs{
			DeleteFunc: smc.monitoringClient.Prometheuses(sm.Namespace).Delete,
		},
		smc.eventRecorder,
	)
	pruneErrors = append(pruneErrors, err)

	err = controllerhelpers.Prune(
		ctx,
		slices.FilterOutNil(slices.ToSlice(requiredIngress)),
		ingresses,
		&controllerhelpers.PruneControlFuncs{
			DeleteFunc: smc.kubeClient.NetworkingV1().Ingresses(sm.Namespace).Delete,
		},
		smc.eventRecorder,
	)
	pruneErrors = append(pruneErrors, err)

	err = controllerhelpers.Prune(
		ctx,
		slices.ToSlice(requiredRecodingPrometheusRule, requiredAlertsPrometheusRule),
		prometheusRules,
		&controllerhelpers.PruneControlFuncs{
			DeleteFunc: smc.monitoringClient.PrometheusRules(sm.Namespace).Delete,
		},
		smc.eventRecorder,
	)
	pruneErrors = append(pruneErrors, err)

	err = controllerhelpers.Prune(
		ctx,
		slices.ToSlice(requiredScyllaDBServiceMonitor),
		serviceMonitors,
		&controllerhelpers.PruneControlFuncs{
			DeleteFunc: smc.monitoringClient.ServiceMonitors(sm.Namespace).Delete,
		},
		smc.eventRecorder,
	)
	pruneErrors = append(pruneErrors, err)

	err = controllerhelpers.Prune(
		ctx,
		certChainConfigs.GetMetaSecrets(),
		secrets,
		&controllerhelpers.PruneControlFuncs{
			DeleteFunc: smc.kubeClient.CoreV1().Secrets(sm.Namespace).Delete,
		},
		smc.eventRecorder,
	)
	pruneErrors = append(pruneErrors, err)

	err = controllerhelpers.Prune(
		ctx,
		certChainConfigs.GetMetaConfigMaps(),
		configMaps,
		&controllerhelpers.PruneControlFuncs{
			DeleteFunc: smc.kubeClient.CoreV1().ConfigMaps(sm.Namespace).Delete,
		},
		smc.eventRecorder,
	)
	pruneErrors = append(pruneErrors, err)

	pruneError := kutilerrors.NewAggregate(pruneErrors)
	if pruneError != nil {
		return progressingConditions, pruneError
	}

	// Apply required objects.
	var applyErrors []error
	applyConfigurations := []resourceapply.ApplyConfigUntyped{
		resourceapply.ApplyConfig[*corev1.ServiceAccount]{
			Required: requiredPrometheusSA,
			Control: resourceapply.ApplyControlFuncs[*corev1.ServiceAccount]{
				GetCachedFunc: smc.serviceAccountLister.ServiceAccounts(sm.Namespace).Get,
				CreateFunc:    smc.kubeClient.CoreV1().ServiceAccounts(sm.Namespace).Create,
				UpdateFunc:    smc.kubeClient.CoreV1().ServiceAccounts(sm.Namespace).Update,
				DeleteFunc:    smc.kubeClient.CoreV1().ServiceAccounts(sm.Namespace).Delete,
			},
		}.ToUntyped(),
		resourceapply.ApplyConfig[*corev1.Service]{
			Required: requiredPrometheusService,
			Control: resourceapply.ApplyControlFuncs[*corev1.Service]{
				GetCachedFunc: smc.serviceLister.Services(sm.Namespace).Get,
				CreateFunc:    smc.kubeClient.CoreV1().Services(sm.Namespace).Create,
				UpdateFunc:    smc.kubeClient.CoreV1().Services(sm.Namespace).Update,
			},
		}.ToUntyped(),
		resourceapply.ApplyConfig[*rbacv1.RoleBinding]{
			Required: requiredPrometheusRoleBinding,
			Control: resourceapply.ApplyControlFuncs[*rbacv1.RoleBinding]{
				GetCachedFunc: smc.roleBindingLister.RoleBindings(sm.Namespace).Get,
				CreateFunc:    smc.kubeClient.RbacV1().RoleBindings(sm.Namespace).Create,
				UpdateFunc:    smc.kubeClient.RbacV1().RoleBindings(sm.Namespace).Update,
				DeleteFunc:    smc.kubeClient.RbacV1().RoleBindings(sm.Namespace).Delete,
			},
		}.ToUntyped(),
		resourceapply.ApplyConfig[*monitoringv1.Prometheus]{
			Required: requiredPrometheus,
			Control: resourceapply.ApplyControlFuncs[*monitoringv1.Prometheus]{
				GetCachedFunc: smc.prometheusLister.Prometheuses(sm.Namespace).Get,
				CreateFunc:    smc.monitoringClient.Prometheuses(sm.Namespace).Create,
				UpdateFunc:    smc.monitoringClient.Prometheuses(sm.Namespace).Update,
				DeleteFunc:    smc.monitoringClient.Prometheuses(sm.Namespace).Delete,
			},
		}.ToUntyped(),
		resourceapply.ApplyConfig[*monitoringv1.ServiceMonitor]{
			Required: requiredScyllaDBServiceMonitor,
			Control: resourceapply.ApplyControlFuncs[*monitoringv1.ServiceMonitor]{
				GetCachedFunc: smc.serviceMonitorLister.ServiceMonitors(sm.Namespace).Get,
				CreateFunc:    smc.monitoringClient.ServiceMonitors(sm.Namespace).Create,
				UpdateFunc:    smc.monitoringClient.ServiceMonitors(sm.Namespace).Update,
				DeleteFunc:    smc.monitoringClient.ServiceMonitors(sm.Namespace).Delete,
			},
		}.ToUntyped(),
		resourceapply.ApplyConfig[*monitoringv1.PrometheusRule]{
			Required: requiredRecodingPrometheusRule,
			Control: resourceapply.ApplyControlFuncs[*monitoringv1.PrometheusRule]{
				GetCachedFunc: smc.prometheusRuleLister.PrometheusRules(sm.Namespace).Get,
				CreateFunc:    smc.monitoringClient.PrometheusRules(sm.Namespace).Create,
				UpdateFunc:    smc.monitoringClient.PrometheusRules(sm.Namespace).Update,
				DeleteFunc:    smc.monitoringClient.PrometheusRules(sm.Namespace).Delete,
			},
		}.ToUntyped(),
		resourceapply.ApplyConfig[*monitoringv1.PrometheusRule]{
			Required: requiredAlertsPrometheusRule,
			Control: resourceapply.ApplyControlFuncs[*monitoringv1.PrometheusRule]{
				GetCachedFunc: smc.prometheusRuleLister.PrometheusRules(sm.Namespace).Get,
				CreateFunc:    smc.monitoringClient.PrometheusRules(sm.Namespace).Create,
				UpdateFunc:    smc.monitoringClient.PrometheusRules(sm.Namespace).Update,
				DeleteFunc:    smc.monitoringClient.PrometheusRules(sm.Namespace).Delete,
			},
		}.ToUntyped(),
	}

	if requiredIngress != nil {
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

	applyError := kutilerrors.NewAggregate(applyErrors)
	if applyError != nil {
		return progressingConditions, applyError
	}

	return progressingConditions, nil
}
