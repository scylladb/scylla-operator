package scylladbmonitoring

import (
	"cmp"
	"context"
	"crypto/x509/pkix"
	"fmt"
	"slices"
	"time"

	configassests "github.com/scylladb/scylla-operator/assets/config"
	grafanav1alpha1assets "github.com/scylladb/scylla-operator/assets/monitoring/grafana/v1alpha1"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/controllertools"
	ocrypto "github.com/scylladb/scylla-operator/pkg/crypto"
	"github.com/scylladb/scylla-operator/pkg/helpers"
	oslices "github.com/scylladb/scylla-operator/pkg/helpers/slices"
	okubecrypto "github.com/scylladb/scylla-operator/pkg/kubecrypto"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	"github.com/scylladb/scylla-operator/pkg/resource"
	"github.com/scylladb/scylla-operator/pkg/resourceapply"
	"github.com/scylladb/scylla-operator/pkg/resourcemerge"
	"github.com/scylladb/scylla-operator/pkg/util/hash"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	kutilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/rand"
)

const (
	grafanaPasswordLength = 20
)

func getGrafanaLabels(sm *scyllav1alpha1.ScyllaDBMonitoring) labels.Set {
	return helpers.MergeMaps(
		getLabels(sm),
		labels.Set{
			naming.ControllerNameLabel: "grafana",
		},
	)
}

func getGrafanaSelector(sm *scyllav1alpha1.ScyllaDBMonitoring) labels.Selector {
	return labels.SelectorFromSet(getGrafanaLabels(sm))
}

func getGrafanaSpec(sm *scyllav1alpha1.ScyllaDBMonitoring) *scyllav1alpha1.GrafanaSpec {
	if sm.Spec.Components != nil {
		return sm.Spec.Components.Grafana
	}

	return nil
}

func getGrafanaIngressOptions(sm *scyllav1alpha1.ScyllaDBMonitoring) *scyllav1alpha1.IngressOptions {
	spec := getGrafanaSpec(sm)
	if spec != nil &&
		spec.ExposeOptions != nil &&
		spec.ExposeOptions.WebInterface != nil {
		return spec.ExposeOptions.WebInterface.Ingress
	}

	return nil
}

func getGrafanaIngressDomains(sm *scyllav1alpha1.ScyllaDBMonitoring) []string {
	ingressOptions := getGrafanaIngressOptions(sm)
	if ingressOptions != nil {
		return ingressOptions.DNSDomains
	}

	return nil
}

func makeGrafanaDeployment(sm *scyllav1alpha1.ScyllaDBMonitoring, soc *scyllav1alpha1.ScyllaOperatorConfig, grafanaServingCertSecretName string, dashboardsCMs []*corev1.ConfigMap, restartTriggerHash string) (*appsv1.Deployment, string, error) {
	spec := getGrafanaSpec(sm)

	var affinity corev1.Affinity
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

	if soc.Status.GrafanaImage == nil {
		return nil, "", controllertools.NewNonRetriable("scyllaoperatorconfig doesn't yet contain grafana image in the status")
	}
	grafanaImage := soc.Status.GrafanaImage

	if soc.Status.BashToolsImage == nil {
		return nil, "", controllertools.NewNonRetriable("scyllaoperatorconfig doesn't yet contain bash tools image in the status")
	}
	bashToolsImage := soc.Status.BashToolsImage

	if len(dashboardsCMs) == 0 {
		return nil, "", fmt.Errorf("dashboardsCMs can't be empty")
	}

	return grafanav1alpha1assets.GrafanaDeploymentTemplate.RenderObject(map[string]any{
		"grafanaImage":           grafanaImage,
		"bashToolsImage":         bashToolsImage,
		"scyllaDBMonitoringName": sm.Name,
		"servingCertSecretName":  grafanaServingCertSecretName,
		"affinity":               affinity,
		"tolerations":            tolerations,
		"resources":              resources,
		"restartTriggerHash":     restartTriggerHash,
		"dashboardsCMs":          dashboardsCMs,
	})
}

func makeGrafanaAdminCredentials(sm *scyllav1alpha1.ScyllaDBMonitoring, secrets map[string]*corev1.Secret) (*corev1.Secret, string, error) {
	var existingPassword []byte

	secretName := sm.Name + "-grafana-admin-credentials"
	existingSecret, found := secrets[secretName]
	if found {
		existingPassword = existingSecret.Data["password"]
	}

	if len(existingPassword) == 0 {
		existingPassword = []byte(rand.String(grafanaPasswordLength))
	}

	return grafanav1alpha1assets.GrafanaAdminCredentialsSecretTemplate.RenderObject(map[string]any{
		"name":     secretName,
		"password": existingPassword,
	})
}

func makeGrafanaSA(sm *scyllav1alpha1.ScyllaDBMonitoring) (*corev1.ServiceAccount, string, error) {
	return grafanav1alpha1assets.GrafanaSATemplate.RenderObject(map[string]any{
		"namespace":              sm.Namespace,
		"scyllaDBMonitoringName": sm.Name,
	})
}

func makeGrafanaConfigs(sm *scyllav1alpha1.ScyllaDBMonitoring) (*corev1.ConfigMap, string, error) {
	enableAnonymousAccess := false
	spec := getGrafanaSpec(sm)
	if spec != nil {
		enableAnonymousAccess = spec.Authentication.InsecureEnableAnonymousAccess
	}

	var defaultDashboard string
	switch t := sm.Spec.GetType(); t {
	case scyllav1alpha1.ScyllaDBMonitoringTypePlatform:
		defaultDashboard = configassests.Project.Operator.GrafanaDefaultPlatformDashboard
	case scyllav1alpha1.ScyllaDBMonitoringTypeSAAS:
		defaultDashboard = "scylladb-latest/overview.json"
	default:
		return nil, "", fmt.Errorf("unkown monitoring type: %q", t)
	}

	return grafanav1alpha1assets.GrafanaConfigsTemplate.RenderObject(map[string]any{
		"scyllaDBMonitoringName": sm.Name,
		"enableAnonymousAccess":  enableAnonymousAccess,
		"defaultDashboard":       defaultDashboard,
	})
}

func makeGrafanaDashboards(sm *scyllav1alpha1.ScyllaDBMonitoring) ([]*corev1.ConfigMap, error) {
	var dashboardsFoldersMap grafanav1alpha1assets.GrafanaDashboardsFoldersMap
	switch t := sm.Spec.GetType(); t {
	case scyllav1alpha1.ScyllaDBMonitoringTypePlatform:
		dashboardsFoldersMap = grafanav1alpha1assets.GrafanaDashboardsPlatform
	case scyllav1alpha1.ScyllaDBMonitoringTypeSAAS:
		dashboardsFoldersMap = grafanav1alpha1assets.GrafanaDashboardsSAAS
	default:
		return nil, fmt.Errorf("unkown monitoring type: %q", t)
	}

	var cms []*corev1.ConfigMap
	for name, folder := range dashboardsFoldersMap {
		cm, _, err := grafanav1alpha1assets.GrafanaDashboardsConfigMapTemplate.RenderObject(map[string]any{
			"scyllaDBMonitoringName": sm.Name,
			"dashboardsName":         name,
			"dashboards":             folder,
		})
		if err != nil {
			return nil, err
		}

		cms = append(cms, cm)
	}

	slices.SortStableFunc(cms, func(lhs, rhs *corev1.ConfigMap) int {
		return cmp.Compare(lhs.Name, rhs.Name)
	})

	return cms, nil
}

func makeGrafanaProvisionings(sm *scyllav1alpha1.ScyllaDBMonitoring) (*corev1.ConfigMap, string, error) {
	return grafanav1alpha1assets.GrafanaProvisioningConfigMapTemplate.RenderObject(map[string]any{
		"scyllaDBMonitoringName": sm.Name,
	})
}

func makeGrafanaService(sm *scyllav1alpha1.ScyllaDBMonitoring) (*corev1.Service, string, error) {
	return grafanav1alpha1assets.GrafanaServiceTemplate.RenderObject(map[string]any{
		"scyllaDBMonitoringName": sm.Name,
	})
}

func makeGrafanaIngress(sm *scyllav1alpha1.ScyllaDBMonitoring) (*networkingv1.Ingress, string, error) {
	ingressOptions := getGrafanaIngressOptions(sm)
	if ingressOptions == nil {
		return nil, "", nil
	}

	if ingressOptions.Disabled != nil && *ingressOptions.Disabled == true {
		return nil, "", nil
	}

	if len(ingressOptions.DNSDomains) == 0 {
		return nil, "", nil
	}

	return grafanav1alpha1assets.GrafanaIngressTemplate.RenderObject(map[string]any{
		"scyllaDBMonitoringName": sm.Name,
		"dnsDomains":             ingressOptions.DNSDomains,
		"ingressAnnotations":     ingressOptions.Annotations,
		"ingressClassName":       ingressOptions.IngressClassName,
	})
}

func (smc *Controller) syncGrafana(
	ctx context.Context,
	sm *scyllav1alpha1.ScyllaDBMonitoring,
	soc *scyllav1alpha1.ScyllaOperatorConfig,
	configMaps map[string]*corev1.ConfigMap,
	secrets map[string]*corev1.Secret,
	services map[string]*corev1.Service,
	serviceAccounts map[string]*corev1.ServiceAccount,
	deployments map[string]*appsv1.Deployment,
	ingresses map[string]*networkingv1.Ingress,
) ([]metav1.Condition, error) {
	var progressingConditions []metav1.Condition

	grafanaServingCertChainConfig := &okubecrypto.CertChainConfig{
		CAConfig: &okubecrypto.CAConfig{
			MetaConfig: okubecrypto.MetaConfig{
				Name:   fmt.Sprintf("%s-grafana-serving-ca", sm.Name),
				Labels: getGrafanaLabels(sm),
			},
			Validity: 10 * 365 * 24 * time.Hour,
			Refresh:  8 * 365 * 24 * time.Hour,
		},
		CABundleConfig: &okubecrypto.CABundleConfig{
			MetaConfig: okubecrypto.MetaConfig{
				Name:   fmt.Sprintf("%s-grafana-serving-ca", sm.Name),
				Labels: getGrafanaLabels(sm),
			},
		},
		CertConfigs: []*okubecrypto.CertificateConfig{
			{
				MetaConfig: okubecrypto.MetaConfig{
					Name:   fmt.Sprintf("%s-grafana-serving-certs", sm.Name),
					Labels: getGrafanaLabels(sm),
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
							sm.Name + "-grafana",
						},
						getGrafanaIngressDomains(sm)...,
					),
				}).ToCreator(),
			},
		},
	}

	var certChainConfigs okubecrypto.CertChainConfigs

	spec := getGrafanaSpec(sm)

	var grafanaServingCertSecretName string
	if spec != nil {
		grafanaServingCertSecretName = spec.ServingCertSecretName
	}

	if len(grafanaServingCertSecretName) == 0 {
		grafanaServingCertSecretName = grafanaServingCertChainConfig.CertConfigs[0].Name
		certChainConfigs = append(certChainConfigs, grafanaServingCertChainConfig)
	}

	// Render manifests.
	var renderErrors []error

	requiredGrafanaSA, _, err := makeGrafanaSA(sm)
	renderErrors = append(renderErrors, err)

	requiredConfigsCM, _, err := makeGrafanaConfigs(sm)
	renderErrors = append(renderErrors, err)

	requiredDahsboardsCMs, err := makeGrafanaDashboards(sm)
	renderErrors = append(renderErrors, err)

	requiredProvisioningsCM, _, err := makeGrafanaProvisionings(sm)
	renderErrors = append(renderErrors, err)

	requiredAdminCredentialsSecret, _, err := makeGrafanaAdminCredentials(sm, secrets)
	renderErrors = append(renderErrors, err)

	var requiredDeployment *appsv1.Deployment
	// Trigger restart for inputs that are not live reloaded.
	grafanaRestartHash, hashErr := hash.HashObjects(requiredConfigsCM, requiredProvisioningsCM, requiredDahsboardsCMs)
	if hashErr != nil {
		renderErrors = append(renderErrors, hashErr)
	} else {
		requiredDeployment, _, err = makeGrafanaDeployment(sm, soc, grafanaServingCertSecretName, requiredDahsboardsCMs, grafanaRestartHash)
		renderErrors = append(renderErrors, err)
	}

	requiredService, _, err := makeGrafanaService(sm)
	renderErrors = append(renderErrors, err)

	requiredIngress, _, err := makeGrafanaIngress(sm)
	renderErrors = append(renderErrors, err)

	renderError := kutilerrors.NewAggregate(renderErrors)
	if renderError != nil {
		return progressingConditions, renderError
	}

	// Prune objects.
	var pruneErrors []error

	err = controllerhelpers.Prune(
		ctx,
		oslices.ToSlice(requiredGrafanaSA),
		serviceAccounts,
		&controllerhelpers.PruneControlFuncs{
			DeleteFunc: smc.kubeClient.CoreV1().ServiceAccounts(sm.Namespace).Delete,
		},
		smc.eventRecorder,
	)
	pruneErrors = append(pruneErrors, err)

	allCMs := []*corev1.ConfigMap{
		requiredConfigsCM,
		requiredProvisioningsCM,
	}
	allCMs = append(allCMs, requiredDahsboardsCMs...)
	allCMs = append(allCMs, certChainConfigs.GetMetaConfigMaps()...)
	err = controllerhelpers.Prune(
		ctx,
		allCMs,
		configMaps,
		&controllerhelpers.PruneControlFuncs{
			DeleteFunc: smc.kubeClient.CoreV1().ConfigMaps(sm.Namespace).Delete,
		},
		smc.eventRecorder,
	)
	pruneErrors = append(pruneErrors, err)

	err = controllerhelpers.Prune(
		ctx,
		append([]*corev1.Secret{requiredAdminCredentialsSecret}, certChainConfigs.GetMetaSecrets()...),
		secrets,
		&controllerhelpers.PruneControlFuncs{
			DeleteFunc: smc.kubeClient.CoreV1().Secrets(sm.Namespace).Delete,
		},
		smc.eventRecorder,
	)
	pruneErrors = append(pruneErrors, err)

	err = controllerhelpers.Prune(
		ctx,
		oslices.ToSlice(requiredService),
		services,
		&controllerhelpers.PruneControlFuncs{
			DeleteFunc: smc.kubeClient.CoreV1().Services(sm.Namespace).Delete,
		},
		smc.eventRecorder,
	)
	pruneErrors = append(pruneErrors, err)

	err = controllerhelpers.Prune(
		ctx,
		oslices.ToSlice(requiredDeployment),
		deployments,
		&controllerhelpers.PruneControlFuncs{
			DeleteFunc: smc.kubeClient.AppsV1().Deployments(sm.Namespace).Delete,
		},
		smc.eventRecorder,
	)
	pruneErrors = append(pruneErrors, err)

	err = controllerhelpers.Prune(
		ctx,
		oslices.FilterOutNil(oslices.ToSlice(requiredIngress)),
		ingresses,
		&controllerhelpers.PruneControlFuncs{
			DeleteFunc: smc.kubeClient.NetworkingV1().Ingresses(sm.Namespace).Delete,
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
			Required: requiredGrafanaSA,
			Control: resourceapply.ApplyControlFuncs[*corev1.ServiceAccount]{
				GetCachedFunc: smc.serviceAccountLister.ServiceAccounts(sm.Namespace).Get,
				CreateFunc:    smc.kubeClient.CoreV1().ServiceAccounts(sm.Namespace).Create,
				UpdateFunc:    smc.kubeClient.CoreV1().ServiceAccounts(sm.Namespace).Update,
				DeleteFunc:    smc.kubeClient.CoreV1().ServiceAccounts(sm.Namespace).Delete,
			},
		}.ToUntyped(),
		resourceapply.ApplyConfig[*corev1.ConfigMap]{
			Required: requiredConfigsCM,
			Control: resourceapply.ApplyControlFuncs[*corev1.ConfigMap]{
				GetCachedFunc: smc.configMapLister.ConfigMaps(sm.Namespace).Get,
				CreateFunc:    smc.kubeClient.CoreV1().ConfigMaps(sm.Namespace).Create,
				UpdateFunc:    smc.kubeClient.CoreV1().ConfigMaps(sm.Namespace).Update,
				DeleteFunc:    smc.kubeClient.CoreV1().ConfigMaps(sm.Namespace).Delete,
			},
		}.ToUntyped(),
		resourceapply.ApplyConfig[*corev1.ConfigMap]{
			Required: requiredProvisioningsCM,
			Control: resourceapply.ApplyControlFuncs[*corev1.ConfigMap]{
				GetCachedFunc: smc.configMapLister.ConfigMaps(sm.Namespace).Get,
				CreateFunc:    smc.kubeClient.CoreV1().ConfigMaps(sm.Namespace).Create,
				UpdateFunc:    smc.kubeClient.CoreV1().ConfigMaps(sm.Namespace).Update,
				DeleteFunc:    smc.kubeClient.CoreV1().ConfigMaps(sm.Namespace).Delete,
			},
		}.ToUntyped(),
		resourceapply.ApplyConfig[*corev1.Secret]{
			Required: requiredAdminCredentialsSecret,
			Control: resourceapply.ApplyControlFuncs[*corev1.Secret]{
				GetCachedFunc: smc.secretLister.Secrets(sm.Namespace).Get,
				CreateFunc:    smc.kubeClient.CoreV1().Secrets(sm.Namespace).Create,
				UpdateFunc:    smc.kubeClient.CoreV1().Secrets(sm.Namespace).Update,
				DeleteFunc:    smc.kubeClient.CoreV1().Secrets(sm.Namespace).Delete,
			},
		}.ToUntyped(),
		resourceapply.ApplyConfig[*appsv1.Deployment]{
			Required: requiredDeployment,
			Control: resourceapply.ApplyControlFuncs[*appsv1.Deployment]{
				GetCachedFunc: smc.deploymentLister.Deployments(sm.Namespace).Get,
				CreateFunc:    smc.kubeClient.AppsV1().Deployments(sm.Namespace).Create,
				UpdateFunc:    smc.kubeClient.AppsV1().Deployments(sm.Namespace).Update,
				DeleteFunc:    smc.kubeClient.AppsV1().Deployments(sm.Namespace).Delete,
			},
		}.ToUntyped(),
		resourceapply.ApplyConfig[*corev1.Service]{
			Required: requiredService,
			Control: resourceapply.ApplyControlFuncs[*corev1.Service]{
				GetCachedFunc: smc.serviceLister.Services(sm.Namespace).Get,
				CreateFunc:    smc.kubeClient.CoreV1().Services(sm.Namespace).Create,
				UpdateFunc:    smc.kubeClient.CoreV1().Services(sm.Namespace).Update,
				DeleteFunc:    smc.kubeClient.CoreV1().Services(sm.Namespace).Delete,
			},
		}.ToUntyped(),
	}
	for _, cm := range requiredDahsboardsCMs {
		applyConfigurations = append(
			applyConfigurations,
			resourceapply.ApplyConfig[*corev1.ConfigMap]{
				Required: cm,
				Control: resourceapply.ApplyControlFuncs[*corev1.ConfigMap]{
					GetCachedFunc: smc.configMapLister.ConfigMaps(sm.Namespace).Get,
					CreateFunc:    smc.kubeClient.CoreV1().ConfigMaps(sm.Namespace).Create,
					UpdateFunc:    smc.kubeClient.CoreV1().ConfigMaps(sm.Namespace).Update,
					DeleteFunc:    smc.kubeClient.CoreV1().ConfigMaps(sm.Namespace).Delete,
				},
			}.ToUntyped(),
		)
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
			cfg.Required.SetLabels(getGrafanaLabels(sm))
		} else {
			resourcemerge.MergeMapInPlaceWithoutRemovalKeys(cfg.Required.GetLabels(), getGrafanaLabels(sm))
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
			controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, grafanaControllerProgressingCondition, cfg.Required, "apply", sm.Generation)
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
		err := cm.ManageCertificateChain(
			ctx,
			time.Now,
			&sm.ObjectMeta,
			scylladbMonitoringControllerGVK,
			ccc,
			secrets,
			configMaps,
		)
		if err != nil {
			applyErrors = append(applyErrors, err)
		}
	}

	applyError := kutilerrors.NewAggregate(applyErrors)
	if applyError != nil {
		return progressingConditions, applyError
	}

	return progressingConditions, nil
}
