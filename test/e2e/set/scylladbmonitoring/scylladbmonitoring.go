// Copyright (C) 2022 ScyllaDB

package scylladbmonitoring

import (
	"compress/gzip"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	prometheusappclient "github.com/prometheus/client_golang/api"
	promeheusappv1api "github.com/prometheus/client_golang/api/prometheus/v1"
	configassests "github.com/scylladb/scylla-operator/assets/config"
	grafanav1alpha1assets "github.com/scylladb/scylla-operator/assets/monitoring/grafana/v1alpha1"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	scyllafixture "github.com/scylladb/scylla-operator/test/e2e/fixture/scylla"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	"github.com/scylladb/scylla-operator/test/e2e/utils/grafana"
	scyllaclusterverification "github.com/scylladb/scylla-operator/test/e2e/utils/verification/scyllacluster"
	"github.com/scylladb/scylla-operator/test/e2e/verification"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apimachineryutilintstr "k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/util/cert"
	"k8s.io/klog/v2"
)

type scyllaDBMonitoringEntry struct {
	// Description is a description of the entry.
	Description string

	// ScyllaDBMonitoringModifierFn allows modifying the default ScyllaDBMonitoring before creation.
	// It's optional.
	ScyllaDBMonitoringModifierFn func(*scyllav1alpha1.ScyllaDBMonitoring)

	// PrepareExternalPrometheusFn allows preparing Prometheus installation when it's not managed by the Operator via ScyllaDBMonitoring.
	// It's optional.
	PrepareExternalPrometheusFn func(ctx context.Context, f *framework.Framework, smName string)

	// VerifyPrometheusFn allows verifying Prometheus installation.
	// It's optional.
	VerifyPrometheusFn func(context.Context, *framework.Framework, *scyllav1alpha1.ScyllaDBMonitoring)

	// VerifyGrafanaFn allows verifying Grafana installation (either managed or not).
	VerifyGrafanaFn func(context.Context, *framework.Framework, *scyllav1alpha1.ScyllaDBMonitoring)
}

func describeEntry(e *scyllaDBMonitoringEntry) string {
	return fmt.Sprintf("with %s", e.Description)
}

var _ = g.Describe("ScyllaDBMonitoring", func() {
	f := framework.NewFramework("scylladbmonitoring")

	g.DescribeTable("should setup monitoring stack", func(ctx g.SpecContext, e *scyllaDBMonitoringEntry) {
		framework.By("Creating a ScyllaCluster with a single node")
		sc := createTestScyllaCluster(ctx, f)

		framework.By("Creating a ScyllaDBMonitoring")
		sm := createScyllaDBMonitoringForScyllaCluster(ctx, f, sc, e.ScyllaDBMonitoringModifierFn)

		framework.By("Waiting for the ScyllaCluster to roll out")
		awaitAndVerifyScyllaClusterRollout(ctx, f, sc)

		if e.PrepareExternalPrometheusFn != nil {
			framework.By("Preparing Prometheus")
			e.PrepareExternalPrometheusFn(ctx, f, sm.Name)
		}

		framework.By("Waiting for the ScyllaDBMonitoring to roll out")
		awaitScyllaDBMonitoringRollout(ctx, f, sm)

		if e.VerifyPrometheusFn != nil {
			framework.By("Verifying that Prometheus is configured correctly")
			e.VerifyPrometheusFn(ctx, f, sm)
		}

		framework.By("Verifying that Grafana is configured correctly")
		e.VerifyGrafanaFn(ctx, f, sm)
	},
		g.Entry(describeEntry, &scyllaDBMonitoringEntry{
			Description: "SAAS type",
			ScyllaDBMonitoringModifierFn: func(sm *scyllav1alpha1.ScyllaDBMonitoring) {
				sm.Spec.Type = pointer.Ptr(scyllav1alpha1.ScyllaDBMonitoringTypeSAAS)
			},
			PrepareExternalPrometheusFn: nil, // Using managed Prometheus.
			VerifyPrometheusFn:          verifyManagedPrometheus,
			VerifyGrafanaFn: verifyManagedGrafanaWithDashboards([]grafana.Dashboard{
				{
					Title:       "CQL Overview",
					Type:        "dash-db",
					FolderTitle: "scylladb-latest",
					Tags:        []string{},
				},
			},
				"cql-overview",
			),
		}, framework.NotSupportedOnOpenShift),
		g.Entry(describeEntry, &scyllaDBMonitoringEntry{
			Description: "Platform type",
			ScyllaDBMonitoringModifierFn: func(sm *scyllav1alpha1.ScyllaDBMonitoring) {
				sm.Spec.Type = pointer.Ptr(scyllav1alpha1.ScyllaDBMonitoringTypePlatform)
			},
			PrepareExternalPrometheusFn: nil, // Using managed Prometheus.
			VerifyPrometheusFn:          verifyManagedPrometheus,
			VerifyGrafanaFn:             verifyManagedGrafanaWithDashboards(getExpectedPlatformDashboards()),
		}, framework.NotSupportedOnOpenShift),
		g.Entry(describeEntry, &scyllaDBMonitoringEntry{
			Description: "Platform type with external Prometheus without TLS",
			ScyllaDBMonitoringModifierFn: func(sm *scyllav1alpha1.ScyllaDBMonitoring) {
				sm.Spec.Type = pointer.Ptr(scyllav1alpha1.ScyllaDBMonitoringTypePlatform)
				sm.Spec.Components.Prometheus.Mode = scyllav1alpha1.PrometheusModeExternal
				sm.Spec.Components.Grafana.Datasources = []scyllav1alpha1.GrafanaDatasourceSpec{
					{
						Name: "prometheus",
						Type: scyllav1alpha1.GrafanaDatasourceTypePrometheus,
						URL:  fmt.Sprintf("http://%s.%s.svc.cluster.local:9090", prometheusNameForScyllaDBMonitoring(sm.Name), f.Namespace()),
						PrometheusOptions: &scyllav1alpha1.GrafanaPrometheusDatasourceOptions{
							TLS: &scyllav1alpha1.GrafanaDatasourceTLSSpec{
								InsecureSkipVerify: true,
							},
							Auth: &scyllav1alpha1.GrafanaPrometheusDatasourceAuthSpec{
								Type: scyllav1alpha1.GrafanaPrometheusDatasourceAuthTypeNoAuthentication,
							},
						},
					},
				}
			},
			PrepareExternalPrometheusFn: prepareExternalPrometheusWithoutTLS,
			VerifyPrometheusFn:          verifyExternalPrometheusWithoutTLS,
			VerifyGrafanaFn:             verifyManagedGrafanaWithDashboards(getExpectedPlatformDashboards()),
		}, framework.NotSupportedOnOpenShift),
		g.Entry(describeEntry, &scyllaDBMonitoringEntry{
			Description: "Platform type with external Prometheus with TLS",
			ScyllaDBMonitoringModifierFn: func(sm *scyllav1alpha1.ScyllaDBMonitoring) {
				sm.Spec.Type = pointer.Ptr(scyllav1alpha1.ScyllaDBMonitoringTypePlatform)
				sm.Spec.Components.Prometheus.Mode = scyllav1alpha1.PrometheusModeExternal
				sm.Spec.Components.Grafana.Datasources = []scyllav1alpha1.GrafanaDatasourceSpec{
					{
						Name: "prometheus",
						Type: scyllav1alpha1.GrafanaDatasourceTypePrometheus,
						URL:  fmt.Sprintf("https://%s.%s.svc.cluster.local:9090", prometheusNameForScyllaDBMonitoring(sm.Name), f.Namespace()),
						PrometheusOptions: &scyllav1alpha1.GrafanaPrometheusDatasourceOptions{
							TLS: &scyllav1alpha1.GrafanaDatasourceTLSSpec{
								InsecureSkipVerify: false,
								CACertConfigMapRef: &scyllav1alpha1.LocalObjectKeySelector{
									Name: prometheusCACertConfigMapNameForScyllaDBMonitoring(sm.Name),
									Key:  "ca-bundle.crt",
								},
							},
						},
					},
				}
			},
			PrepareExternalPrometheusFn: prepareExternalPrometheusWithTLS,
			VerifyPrometheusFn:          verifyExternalPrometheusWithTLS,
			VerifyGrafanaFn:             verifyManagedGrafanaWithDashboards(getExpectedPlatformDashboards()),
		}, framework.NotSupportedOnOpenShift),
		g.Entry(describeEntry, &scyllaDBMonitoringEntry{
			Description: "Platform type with Thanos Querier on OpenShift",
			ScyllaDBMonitoringModifierFn: func(sm *scyllav1alpha1.ScyllaDBMonitoring) {
				sm.Spec.Type = pointer.Ptr(scyllav1alpha1.ScyllaDBMonitoringTypePlatform)
				sm.Spec.Components.Prometheus.Mode = scyllav1alpha1.PrometheusModeExternal
				sm.Spec.Components.Grafana.Datasources = []scyllav1alpha1.GrafanaDatasourceSpec{
					{
						Name: "prometheus",
						Type: scyllav1alpha1.GrafanaDatasourceTypePrometheus,
						URL:  "https://thanos-querier.openshift-monitoring.svc:9091",
						PrometheusOptions: &scyllav1alpha1.GrafanaPrometheusDatasourceOptions{
							TLS: &scyllav1alpha1.GrafanaDatasourceTLSSpec{
								InsecureSkipVerify: false,
								CACertConfigMapRef: &scyllav1alpha1.LocalObjectKeySelector{
									Name: openShiftServiceCAConfigMapName(sm.Name),
									Key:  "service-ca.crt",
								},
							},
							Auth: &scyllav1alpha1.GrafanaPrometheusDatasourceAuthSpec{
								Type: scyllav1alpha1.GrafanaPrometheusDatasourceAuthTypeBearerToken,
								BearerTokenOptions: &scyllav1alpha1.GrafanaPrometheusDatasourceBearerTokenAuthOptions{
									SecretRef: &scyllav1alpha1.LocalObjectKeySelector{
										Name: monitoringAccessServiceAccountNameOnOpenShift(sm.Name),
										Key:  "token",
									},
								},
							},
						},
					},
				}
			},
			PrepareExternalPrometheusFn: prepareOpenShiftMonitoring,
			VerifyPrometheusFn:          nil, // Grafana datasource is enough to verify access to Thanos Querier.
			VerifyGrafanaFn:             verifyManagedGrafanaWithDashboards(getExpectedPlatformDashboards()),
		}, framework.SupportedOnlyOnOpenShift),
	)
})

func createTestScyllaCluster(ctx context.Context, f *framework.Framework) *scyllav1.ScyllaCluster {
	g.GinkgoHelper()

	sc := f.GetDefaultScyllaCluster()
	sc, err := f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Create(
		ctx,
		sc,
		metav1.CreateOptions{
			FieldManager:    f.FieldManager(),
			FieldValidation: metav1.FieldValidationStrict,
		},
	)
	o.Expect(err).NotTo(o.HaveOccurred())

	return sc
}

func createScyllaDBMonitoringForScyllaCluster(ctx context.Context, f *framework.Framework, sc *scyllav1.ScyllaCluster, modifierFn func(*scyllav1alpha1.ScyllaDBMonitoring)) *scyllav1alpha1.ScyllaDBMonitoring {
	g.GinkgoHelper()

	renderArgs := map[string]any{
		"name":              sc.Name,
		"namespace":         sc.Namespace,
		"scyllaClusterName": sc.Name,
		"storageClassName":  framework.TestContext.ScyllaClusterOptions.StorageClassName,
	}
	if framework.TestContext.IngressController != nil {
		renderArgs["ingressClassName"] = framework.TestContext.IngressController.IngressClassName
		renderArgs["ingressCustomAnnotations"] = framework.TestContext.IngressController.CustomAnnotations
	}
	sm, _, err := scyllafixture.ScyllaDBMonitoringTemplate.RenderObject(renderArgs)
	o.Expect(err).NotTo(o.HaveOccurred())
	if modifierFn != nil {
		modifierFn(sm)
	}

	sm, err = f.ScyllaClient().ScyllaV1alpha1().ScyllaDBMonitorings(sc.Namespace).Create(
		ctx,
		sm,
		metav1.CreateOptions{
			FieldManager: f.FieldManager(),
		},
	)
	o.Expect(err).NotTo(o.HaveOccurred())
	return sm
}

func awaitAndVerifyScyllaClusterRollout(ctx context.Context, f *framework.Framework, sc *scyllav1.ScyllaCluster) {
	g.GinkgoHelper()

	waitCtx, waitCtxCancel := utils.ContextForRollout(ctx, sc)
	defer waitCtxCancel()
	sc, err := controllerhelpers.WaitForScyllaClusterState(waitCtx, f.ScyllaClient().ScyllaV1().ScyllaClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
	o.Expect(err).NotTo(o.HaveOccurred())

	scyllaclusterverification.Verify(ctx, f.KubeClient(), f.ScyllaClient(), sc)
}

func awaitScyllaDBMonitoringRollout(ctx context.Context, f *framework.Framework, sm *scyllav1alpha1.ScyllaDBMonitoring) {
	g.GinkgoHelper()

	waitCtx, waitCtxCancel := context.WithTimeout(ctx, 5*time.Minute)
	defer waitCtxCancel()
	sm, err := controllerhelpers.WaitForScyllaDBMonitoringState(waitCtx, f.ScyllaClient().ScyllaV1alpha1().ScyllaDBMonitorings(sm.Namespace), sm.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaDBMonitoringRolledOut)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func verifyManagedPrometheus(ctx context.Context, f *framework.Framework, sm *scyllav1alpha1.ScyllaDBMonitoring) {
	g.GinkgoHelper()

	promClient := prepareManagedPrometheusClient(ctx, f, sm)
	verifyPrometheusTargetsAndRules(ctx, promClient)
}

func prepareManagedPrometheusClient(ctx context.Context, f *framework.Framework, sm *scyllav1alpha1.ScyllaDBMonitoring) promeheusappv1api.API {
	g.GinkgoHelper()

	prometheusServingCABundleConfigMapName, err := naming.ManagedPrometheusServingCAConfigMapName(sm.Name)
	o.Expect(err).NotTo(o.HaveOccurred())
	prometheusServingCABundleConfigMap, err := f.KubeClient().CoreV1().ConfigMaps(f.Namespace()).Get(ctx, prometheusServingCABundleConfigMapName, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	prometheusServingCACerts, _ := verification.VerifyAndParseCABundle(prometheusServingCABundleConfigMap)
	o.Expect(prometheusServingCACerts).To(o.HaveLen(1))

	prometheusServingCAPool := x509.NewCertPool()
	prometheusServingCAPool.AddCert(prometheusServingCACerts[0])

	prometheusGrafanaClientSecretName, err := naming.ManagedPrometheusClientGrafanaSecretName(sm.Name)
	o.Expect(err).NotTo(o.HaveOccurred())
	prometheusGrafanaClientSecret, err := f.KubeClient().CoreV1().Secrets(f.Namespace()).Get(ctx, prometheusGrafanaClientSecretName, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	_, prometheusGrafanaClientCertBytes, _, prometheusGrafanaClientKeyBytes := verification.VerifyAndParseTLSCert(prometheusGrafanaClientSecret, verification.TLSCertOptions{
		IsCA:     pointer.Ptr(false),
		KeyUsage: pointer.Ptr(x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature),
	})

	prometheusGrafanaAdminTLSCert, err := tls.X509KeyPair(prometheusGrafanaClientCertBytes, prometheusGrafanaClientKeyBytes)
	o.Expect(err).NotTo(o.HaveOccurred())

	o.Expect(sm.Spec.Components.Prometheus.ExposeOptions.WebInterface.Ingress.DNSDomains).To(o.HaveLen(1))
	prometheusServerName := sm.Spec.Components.Prometheus.ExposeOptions.WebInterface.Ingress.DNSDomains[0]

	promHTTPClient, err := prometheusappclient.NewClient(prometheusappclient.Config{
		Address: "https://" + f.GetIngressAddress(prometheusServerName),
		Client: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					ServerName:   prometheusServerName,
					Certificates: []tls.Certificate{prometheusGrafanaAdminTLSCert},
					RootCAs:      prometheusServingCAPool,
				},
				Proxy: http.ProxyFromEnvironment,
				DialContext: (&net.Dialer{
					Timeout:   30 * time.Second,
					KeepAlive: 30 * time.Second,
				}).DialContext,
				ForceAttemptHTTP2:     true,
				MaxIdleConns:          100,
				IdleConnTimeout:       90 * time.Second,
				TLSHandshakeTimeout:   10 * time.Second,
				ExpectContinueTimeout: 1 * time.Second,
			},
		},
	})
	o.Expect(err).NotTo(o.HaveOccurred())

	return promeheusappv1api.NewAPI(promHTTPClient)
}

func verifyPrometheusTargetsAndRules(ctx context.Context, promClient promeheusappv1api.API) {
	g.GinkgoHelper()

	framework.By("Verifying Prometheus targets and rules")
	// We wait for this to be eventually true as Prometheus may take some time to scrape targets and load rules.
	// This is expected to be eventually consistent and there's no programmatic way to know when it's ready
	// other than querying its API.
	o.Eventually(func(eo o.Gomega) {
		ctxTargets, ctxTargetsCancel := context.WithTimeout(ctx, 15*time.Second)
		defer ctxTargetsCancel()

		targets, err := promClient.Targets(ctxTargets)
		framework.Infof("Listing Prometheus targets: err: %v, active: %d, dropped: %d", err, len(targets.Active), len(targets.Dropped))
		eo.Expect(err).NotTo(o.HaveOccurred())

		// This should match the number of rules in service monitors used. We can possibly extend this to compare those
		// or wait to be able to assess that dropped targets are empty.
		eo.Expect(targets.Active).To(o.HaveLen(2))
		for _, t := range targets.Active {
			eo.Expect(t.Health).To(o.Equal(promeheusappv1api.HealthGood))
		}

		// TODO: There shouldn't be any dropped targets. Currently, /service-discovery contains
		//       "undefined (0 / 54 active targets)" that are in addition to our ServiceMonitor definition.
		//       (Maciek was looking into this, it seems to be a bug in prometheus operator.)
		// o.Expect(targets.Dropped).To(o.HaveLen(0))

		rulesResult, err := promClient.Rules(ctxTargets)
		framework.Infof("Listing Prometheus rules: err: %v, groupCount: %d", err, len(rulesResult.Groups))
		eo.Expect(err).NotTo(o.HaveOccurred())

		eo.Expect(rulesResult.Groups).To(o.HaveLen(3))
		for _, ruleGroup := range rulesResult.Groups {
			eo.Expect(ruleGroup.Name).To(o.Equal("scylla.rules"))
			eo.Expect(ruleGroup.Rules).NotTo(o.BeEmpty())
			for _, rule := range ruleGroup.Rules {
				switch r := rule.(type) {
				case promeheusappv1api.AlertingRule:
					eo.Expect(r.Health).To(o.BeEquivalentTo(promeheusappv1api.RuleHealthGood))

				case promeheusappv1api.RecordingRule:
					eo.Expect(r.Health).To(o.BeEquivalentTo(promeheusappv1api.RuleHealthGood))

				default:
					eo.Expect(fmt.Errorf("unexpected rule type %t", rule)).NotTo(o.HaveOccurred())
				}
			}
		}
	}).WithTimeout(5 * time.Minute).WithPolling(1 * time.Second).Should(o.Succeed())
}

func verifyManagedGrafanaWithDashboards(
	expectedDashboards []grafana.Dashboard,
	expectedHomeDashboardUID string,
) func(context.Context, *framework.Framework, *scyllav1alpha1.ScyllaDBMonitoring) {
	return func(ctx context.Context, f *framework.Framework, sm *scyllav1alpha1.ScyllaDBMonitoring) {
		g.GinkgoHelper()

		grafanaAdminCredentialsSecret, err := f.KubeClient().CoreV1().Secrets(f.Namespace()).Get(ctx, fmt.Sprintf("%s-grafana-admin-credentials", sm.Name), metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(grafanaAdminCredentialsSecret.Data).To(o.HaveLen(2))
		o.Expect(grafanaAdminCredentialsSecret.Data).To(o.HaveKey("username"))
		o.Expect(grafanaAdminCredentialsSecret.Data).To(o.HaveKey("password"))

		grafanaUsername := string(grafanaAdminCredentialsSecret.Data["username"])
		o.Expect(grafanaUsername).NotTo(o.BeEmpty())
		grafanaPassword := string(grafanaAdminCredentialsSecret.Data["password"])
		o.Expect(grafanaPassword).NotTo(o.BeEmpty())

		grafanaServingCABundleConfigMap, err := f.KubeClient().CoreV1().ConfigMaps(f.Namespace()).Get(ctx, fmt.Sprintf("%s-grafana-serving-ca", sm.Name), metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		grafanaServingCACerts, _ := verification.VerifyAndParseCABundle(grafanaServingCABundleConfigMap)
		o.Expect(grafanaServingCACerts).To(o.HaveLen(1))

		grafanaServingCAPool := x509.NewCertPool()
		grafanaServingCAPool.AddCert(grafanaServingCACerts[0])

		o.Expect(sm.Spec.Components.Grafana.ExposeOptions.WebInterface.Ingress.DNSDomains).To(o.HaveLen(1))
		grafanaServerName := sm.Spec.Components.Grafana.ExposeOptions.WebInterface.Ingress.DNSDomains[0]

		grafanaClient, err := grafana.NewClient(grafana.ClientOptions{
			URL:      "https://" + f.GetIngressAddress(grafanaServerName),
			Username: grafanaUsername,
			Password: grafanaPassword,
			TLS: &tls.Config{
				ServerName: grafanaServerName,
				RootCAs:    grafanaServingCAPool,
			},
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		// We wait for these conditions to be eventually true as Grafana may take some time to expose the dashboards
		// and configure the data source.
		// This is expected to be eventually consistent and there's no programmatic way to know when it's ready
		// other than querying its API.
		verifyGrafanaDashboards(grafanaClient, expectedDashboards, expectedHomeDashboardUID)
		verifyPrometheusGrafanaDataSource(grafanaClient)
	}
}

func verifyGrafanaDashboards(grafanaClient *grafana.Client, expectedDashboards []grafana.Dashboard, expectedHomeDashboardUID string) {
	g.GinkgoHelper()

	framework.By("Verifying Grafana dashboards")
	var dashboards []grafana.Dashboard
	o.Eventually(func(eo o.Gomega) {
		var err error
		dashboards, err = grafanaClient.Dashboards()
		framework.Infof("Listing grafana dashboards: err: %v, count: %d", err, len(dashboards))
		eo.Expect(err).NotTo(o.HaveOccurred())
		eo.Expect(dashboards).To(o.HaveLen(len(expectedDashboards)))
	}).WithTimeout(10 * time.Minute).WithPolling(1 * time.Second).Should(o.Succeed())
	o.Expect(dashboards).To(o.ConsistOf(expectedDashboards))

	framework.By("Verifying Grafana home dashboard UID")
	homeDashboardUID, err := grafanaClient.HomeDashboardUID()
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(homeDashboardUID).To(o.Equal(expectedHomeDashboardUID))
}

func verifyPrometheusGrafanaDataSource(grafanaClient *grafana.Client) {
	g.GinkgoHelper()

	framework.By("Verifying 'prometheus' Grafana data source")
	o.Eventually(func(eo o.Gomega) {
		health, err := grafanaClient.DatasourceHealth("prometheus")
		framework.Infof("Checking 'prometheus' grafana data source health: err: %v, health: %v, message: %s", err, health.OK, health.Message)
		eo.Expect(err).NotTo(o.HaveOccurred())
		eo.Expect(health.OK).To(o.Equal(true))
	}).WithTimeout(10 * time.Minute).WithPolling(1 * time.Second).Should(o.Succeed())
}

// getExpectedPlatformDashboards returns the expected grafana dashboards.
// Platform dashboards come directly from ScyllaDB Monitoring, so we do not know the expected values
// and given the size they are not feasible to be maintained as a duplicate.
// Contrary to our testing practice, in this case we'll just make sure it's not empty and load
// the expected values dynamically.
func getExpectedPlatformDashboards() (expectedDashboards []grafana.Dashboard, homeDashboardUID string) {
	g.GinkgoHelper()

	var expectedPlatformFolderDashboardSearchResponse []grafana.Dashboard

	for dashboardFolderName, dashboardFolder := range grafanav1alpha1assets.GrafanaDashboardsPlatform.Get() {
		for _, dashboardString := range dashboardFolder {
			o.Expect(dashboardString).NotTo(o.BeEmpty())

			gd, err := decodeGrafanaDashboardFromGZBase64String(dashboardString)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(gd).NotTo(o.BeZero())

			expectedPlatformFolderDashboardSearchResponse = append(expectedPlatformFolderDashboardSearchResponse, grafana.Dashboard{
				Title:       gd.Title,
				Tags:        gd.Tags,
				Type:        "dash-db",
				FolderTitle: dashboardFolderName,
			})
		}
	}
	o.Expect(expectedPlatformFolderDashboardSearchResponse).NotTo(o.BeEmpty())

	homeDashboardDir, homeDashboardFile := filepath.Split(configassests.Project.Operator.GrafanaDefaultPlatformDashboard)
	homeDashboardDir = strings.TrimSuffix(homeDashboardDir, "/")
	o.Expect(homeDashboardDir).NotTo(o.BeZero())
	o.Expect(homeDashboardDir).NotTo(o.ContainSubstring("/"))
	o.Expect(homeDashboardFile).NotTo(o.BeZero())
	o.Expect(homeDashboardFile).NotTo(o.ContainSubstring("/"))
	homeDashboardFile += ".gz.base64"

	o.Expect(grafanav1alpha1assets.GrafanaDashboardsPlatform.Get()).To(o.HaveKey(homeDashboardDir))
	homeDashboardFolder := grafanav1alpha1assets.GrafanaDashboardsPlatform.Get()[homeDashboardDir]
	o.Expect(homeDashboardFolder).NotTo(o.BeEmpty())
	o.Expect(homeDashboardFolder).To(o.HaveKey(homeDashboardFile))
	homeDashboardString := homeDashboardFolder[homeDashboardFile]
	o.Expect(homeDashboardString).NotTo(o.BeZero())
	ghd, err := decodeGrafanaDashboardFromGZBase64String(homeDashboardString)
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(ghd).NotTo(o.BeZero())
	o.Expect(ghd.UID).NotTo(o.BeZero())

	return expectedPlatformFolderDashboardSearchResponse, ghd.UID
}

type grafanaDashboard struct {
	Title string   `json:"title"`
	Tags  []string `json:"tags"`
	UID   string   `json:"uid"`
}

func decodeGrafanaDashboardFromGZBase64String(s string) (*grafanaDashboard, error) {
	b64Reader := base64.NewDecoder(base64.StdEncoding, strings.NewReader(s))
	zr, err := gzip.NewReader(b64Reader)
	if err != nil {
		return nil, fmt.Errorf("can't create gzip reader: %w", err)
	}
	defer func() {
		closeErr := zr.Close()
		if closeErr != nil {
			klog.ErrorS(err, "can't close gzip writer")
		}
	}()

	data, err := io.ReadAll(zr)
	if err != nil {
		return nil, fmt.Errorf("can't read data from gzip reader: %w", err)
	}

	res := &grafanaDashboard{}
	err = json.Unmarshal(data, &res)
	if err != nil {
		return nil, fmt.Errorf("can't unmarshal grafana dashboard: %w", err)
	}

	return res, nil
}

func prepareExternalPrometheusWithoutTLS(ctx context.Context, f *framework.Framework, smName string) {
	g.GinkgoHelper()

	framework.By("Creating a ServiceAccount for external Prometheus")
	sa := createExternalPrometheusServiceAccountWithClusterRole(ctx, f, smName)

	framework.By("Creating a Service for Prometheus to be used by external Prometheus")
	svc := createServiceForPrometheus(ctx, f, smName)

	framework.By("Creating a Prometheus instance to be used as external Prometheus")
	createExternalPrometheusInstanceWithoutTLS(ctx, f, prometheusInstanceOptions{
		PrometheusName:         prometheusNameForScyllaDBMonitoring(smName),
		ScyllaDBMonitoringName: smName,
		ServiceAccountName:     sa.Name,
		ServiceName:            svc.Name,
	})
}

func prepareExternalPrometheusWithTLS(ctx context.Context, f *framework.Framework, smName string) {
	g.GinkgoHelper()

	framework.By("Creating a ServiceAccount for external Prometheus")
	sa := createExternalPrometheusServiceAccountWithClusterRole(ctx, f, smName)

	framework.By("Creating a Service for Prometheus to be used by external Prometheus")
	svc := createServiceForPrometheus(ctx, f, smName)

	framework.By("Creating a TLS Secret and ConfigMap for Prometheus")
	createPrometheusTLSSecretAndConfigMap(ctx, f, prometheusTLSSecretAndConfigMapOptions{
		CACertConfigMapName: prometheusCACertConfigMapNameForScyllaDBMonitoring(smName),
		PrometheusName:      prometheusNameForScyllaDBMonitoring(smName),
		TLSSecretName:       prometheusTLSSecretNameForScyllaDBMonitoring(smName),
	})

	framework.By("Creating a Prometheus instance to be used as external Prometheus")
	createExternalPrometheusInstanceWithTLS(ctx, f, prometheusInstanceWithTLSOptions{
		prometheusInstanceOptions: prometheusInstanceOptions{
			PrometheusName:         prometheusNameForScyllaDBMonitoring(smName),
			ScyllaDBMonitoringName: smName,
			ServiceAccountName:     sa.Name,
			ServiceName:            svc.Name,
		},
		TLSSecretName: prometheusTLSSecretNameForScyllaDBMonitoring(smName),
	})
}

func createExternalPrometheusServiceAccountWithClusterRole(ctx context.Context, f *framework.Framework, smName string) *corev1.ServiceAccount {
	g.GinkgoHelper()

	framework.By("Creating a ServiceAccount for external Prometheus")
	prometheusServiceAccountName := fmt.Sprintf("%s-prometheus", smName)
	sa, err := f.KubeAdminClient().CoreV1().ServiceAccounts(f.Namespace()).Create(ctx, &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      prometheusServiceAccountName,
			Namespace: f.Namespace(),
		},
	}, metav1.CreateOptions{
		FieldManager: f.FieldManager(),
	})
	o.Expect(err).NotTo(o.HaveOccurred())

	framework.By("Creating a ClusterRole for external Prometheus")
	prometheusClusterRoleName := fmt.Sprintf("%s-prometheus", smName)
	_, err = f.KubeAdminClient().RbacV1().ClusterRoles().Create(ctx, &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: prometheusClusterRoleName,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{
					"nodes",
					"nodes/metrics",
					"services",
					"endpoints",
					"pods",
				},
				Verbs: []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"configmaps"},
				Verbs:     []string{"get"},
			},
			{
				APIGroups: []string{"discovery.k8s.io"},
				Resources: []string{"endpointslices"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{"networking.k8s.io"},
				Resources: []string{"ingresses"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				NonResourceURLs: []string{"/metrics"},
				Verbs:           []string{"get"},
			},
		},
	}, metav1.CreateOptions{
		FieldManager: f.FieldManager(),
	})
	o.Expect(err).NotTo(o.HaveOccurred())

	framework.By("Creating a ClusterRoleBinding for external Prometheus")
	_, err = f.KubeAdminClient().RbacV1().ClusterRoleBindings().Create(ctx, &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s-prometheus", smName),
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      prometheusServiceAccountName,
				Namespace: f.Namespace(),
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     prometheusClusterRoleName,
		},
	}, metav1.CreateOptions{
		FieldManager: f.FieldManager(),
	})
	o.Expect(err).NotTo(o.HaveOccurred())

	return sa
}

// createServiceForPrometheus creates a headless Service for Prometheus to be used by external Prometheus.
func createServiceForPrometheus(ctx context.Context, f *framework.Framework, smName string) *corev1.Service {
	g.GinkgoHelper()

	framework.By("Creating a Service for Prometheus")
	prometheusName := prometheusNameForScyllaDBMonitoring(smName)
	svc, err := f.KubeAdminClient().CoreV1().Services(f.Namespace()).Create(ctx, &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      prometheusName,
			Namespace: f.Namespace(),
		},
		Spec: corev1.ServiceSpec{
			ClusterIP: corev1.ClusterIPNone,
			Selector: map[string]string{
				"app.kubernetes.io/managed-by": "prometheus-operator",
				"app.kubernetes.io/name":       "prometheus",
				"app.kubernetes.io/instance":   prometheusName,
				"operator.prometheus.io/name":  prometheusName,
				"prometheus":                   prometheusName,
			},
			Ports: []corev1.ServicePort{
				{
					Name:     "web",
					Port:     9090,
					Protocol: corev1.ProtocolTCP,
				},
			},
		},
	}, metav1.CreateOptions{
		FieldManager: f.FieldManager(),
	})
	o.Expect(err).NotTo(o.HaveOccurred())

	return svc
}

type prometheusInstanceOptions struct {
	PrometheusName         string
	ScyllaDBMonitoringName string
	ServiceAccountName     string
	ServiceName            string
}

func createExternalPrometheusInstanceWithoutTLS(ctx context.Context, f *framework.Framework, opts prometheusInstanceOptions) *monitoringv1.Prometheus {
	g.GinkgoHelper()

	framework.By("Creating a Prometheus instance")
	prom, err := f.PrometheusOperatorAdminClient().MonitoringV1().Prometheuses(f.Namespace()).Create(ctx, &monitoringv1.Prometheus{
		ObjectMeta: metav1.ObjectMeta{
			Name:      opts.PrometheusName,
			Namespace: f.Namespace(),
		},
		Spec: monitoringv1.PrometheusSpec{
			CommonPrometheusFields: monitoringv1.CommonPrometheusFields{
				ServiceAccountName: opts.ServiceAccountName,
				ServiceName:        pointer.Ptr(opts.ServiceAccountName),
				SecurityContext: &corev1.PodSecurityContext{
					RunAsNonRoot: pointer.Ptr(true),
					RunAsUser:    pointer.Ptr[int64](65534),
					FSGroup:      pointer.Ptr[int64](65534),
				},
				ServiceMonitorSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"scylla-operator.scylladb.com/scylladbmonitoring-name": opts.ScyllaDBMonitoringName,
					},
				},
				Web: &monitoringv1.PrometheusWebSpec{
					PageTitle: pointer.Ptr("ScyllaDB Prometheus"),
				},
			},
			Alerting: &monitoringv1.AlertingSpec{
				Alertmanagers: []monitoringv1.AlertmanagerEndpoints{
					{
						Name: "scylla-monitoring",
						Port: apimachineryutilintstr.FromString("web"),
					},
				},
			},
			RuleSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"scylla-operator.scylladb.com/scylladbmonitoring-name": opts.ScyllaDBMonitoringName,
				},
			},
		},
	}, metav1.CreateOptions{
		FieldManager: f.FieldManager(),
	})
	o.Expect(err).NotTo(o.HaveOccurred())

	return prom
}

type prometheusInstanceWithTLSOptions struct {
	prometheusInstanceOptions
	TLSSecretName string
}

func createExternalPrometheusInstanceWithTLS(ctx context.Context, f *framework.Framework, opts prometheusInstanceWithTLSOptions) *monitoringv1.Prometheus {
	g.GinkgoHelper()

	framework.By("Creating a Prometheus instance with TLS")
	prom, err := f.PrometheusOperatorAdminClient().MonitoringV1().Prometheuses(f.Namespace()).Create(ctx, &monitoringv1.Prometheus{
		ObjectMeta: metav1.ObjectMeta{
			Name:      opts.PrometheusName,
			Namespace: f.Namespace(),
		},
		Spec: monitoringv1.PrometheusSpec{
			CommonPrometheusFields: monitoringv1.CommonPrometheusFields{
				ServiceAccountName: opts.ServiceAccountName,
				ServiceName:        pointer.Ptr(opts.ServiceName),
				SecurityContext: &corev1.PodSecurityContext{
					RunAsNonRoot: pointer.Ptr(true),
					RunAsUser:    pointer.Ptr[int64](65534),
					FSGroup:      pointer.Ptr[int64](65534),
				},
				ServiceMonitorSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"scylla-operator.scylladb.com/scylladbmonitoring-name": opts.ScyllaDBMonitoringName,
					},
				},
				Web: &monitoringv1.PrometheusWebSpec{
					PageTitle: pointer.Ptr("ScyllaDB Prometheus"),
					WebConfigFileFields: monitoringv1.WebConfigFileFields{
						TLSConfig: &monitoringv1.WebTLSConfig{
							Cert: monitoringv1.SecretOrConfigMap{
								Secret: &corev1.SecretKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: opts.TLSSecretName,
									},
									Key: "tls.crt",
								},
							},
							KeySecret: corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: opts.TLSSecretName,
								},
								Key: "tls.key",
							},
						},
					},
				},
			},
			Alerting: &monitoringv1.AlertingSpec{
				Alertmanagers: []monitoringv1.AlertmanagerEndpoints{
					{
						Name: "scylla-monitoring",
						Port: apimachineryutilintstr.FromString("web"),
					},
				},
			},
			RuleSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"scylla-operator.scylladb.com/scylladbmonitoring-name": opts.ScyllaDBMonitoringName,
				},
			},
		},
	}, metav1.CreateOptions{
		FieldManager: f.FieldManager(),
	})
	o.Expect(err).NotTo(o.HaveOccurred())

	return prom
}

func prepareOpenShiftMonitoring(ctx context.Context, f *framework.Framework, smName string) {
	g.GinkgoHelper()

	framework.By("Creating a ServiceAccount for monitoring access on OpenShift")
	sa := createMonitoringAccessServiceAccountOnOpenShift(ctx, f, smName)

	framework.By("Binding cluster-monitoring-view ClusterRole to the ServiceAccount")
	bindClusterMonitoringViewClusterRoleToServiceAccount(ctx, f, sa)

	framework.By("Creating a Secret with the ServiceAccount token")
	createServiceAccountTokenSecret(ctx, f, sa)

	framework.By("Creating OpenShift Service CA ConfigMap")
	createOpenShiftServiceCAConfigMap(ctx, f, smName)
}

func createMonitoringAccessServiceAccountOnOpenShift(ctx context.Context, f *framework.Framework, smName string) *corev1.ServiceAccount {
	sa, err := f.KubeAdminClient().CoreV1().ServiceAccounts(f.Namespace()).Create(ctx, &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      monitoringAccessServiceAccountNameOnOpenShift(smName),
			Namespace: f.Namespace(),
		},
	}, metav1.CreateOptions{
		FieldManager: f.FieldManager(),
	})
	o.Expect(err).NotTo(o.HaveOccurred())
	return sa
}

func bindClusterMonitoringViewClusterRoleToServiceAccount(ctx context.Context, f *framework.Framework, sa *corev1.ServiceAccount) {
	_, err := f.KubeAdminClient().RbacV1().ClusterRoleBindings().Create(ctx, &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: sa.Name,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      sa.Name,
				Namespace: sa.Namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "cluster-monitoring-view",
		},
	}, metav1.CreateOptions{
		FieldManager: f.FieldManager(),
	})
	o.Expect(err).NotTo(o.HaveOccurred())
}

func createServiceAccountTokenSecret(ctx context.Context, f *framework.Framework, sa *corev1.ServiceAccount) {
	_, err := f.KubeAdminClient().CoreV1().Secrets(f.Namespace()).Create(ctx, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      sa.Name,
			Namespace: f.Namespace(),
			Annotations: map[string]string{
				"kubernetes.io/service-account.name": sa.Name,
			},
		},
		Type: corev1.SecretTypeServiceAccountToken,
	}, metav1.CreateOptions{
		FieldManager: f.FieldManager(),
	})
	o.Expect(err).NotTo(o.HaveOccurred())
}

func verifyExternalPrometheusWithoutTLS(ctx context.Context, f *framework.Framework, sm *scyllav1alpha1.ScyllaDBMonitoring) {
	g.GinkgoHelper()

	promClient := makePrometheusClientWithConfig(prometheusappclient.Config{
		Address: fmt.Sprintf("http://%s.%s.svc.cluster.local:9090", prometheusNameForScyllaDBMonitoring(sm.Name), f.Namespace()),
	})
	verifyPrometheusTargetsAndRules(ctx, promClient)
}

func verifyExternalPrometheusWithTLS(ctx context.Context, f *framework.Framework, sm *scyllav1alpha1.ScyllaDBMonitoring) {
	g.GinkgoHelper()

	rootCAs := x509.NewCertPool()
	prometheusCACertConfigMap, err := f.KubeClient().CoreV1().ConfigMaps(f.Namespace()).Get(ctx, prometheusCACertConfigMapNameForScyllaDBMonitoring(sm.Name), metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	prometheusCACerts, _ := verification.VerifyAndParseCABundle(prometheusCACertConfigMap)
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(prometheusCACerts).To(o.HaveLen(2))
	rootCAs.AddCert(prometheusCACerts[1]) // The CA cert is the second cert in the bundle.

	promClient := makePrometheusClientWithConfig(prometheusappclient.Config{
		Address: fmt.Sprintf("https://%s.%s.svc.cluster.local:9090", prometheusNameForScyllaDBMonitoring(sm.Name), f.Namespace()),
		Client: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: false,
					RootCAs:            rootCAs,
				},
			},
		},
	})
	verifyPrometheusTargetsAndRules(ctx, promClient)
}

func makePrometheusClientWithConfig(cfg prometheusappclient.Config) promeheusappv1api.API {
	g.GinkgoHelper()

	promHTTPClient, err := prometheusappclient.NewClient(cfg)
	o.Expect(err).NotTo(o.HaveOccurred())
	return promeheusappv1api.NewAPI(promHTTPClient)
}

type prometheusTLSSecretAndConfigMapOptions struct {
	CACertConfigMapName string
	PrometheusName      string
	TLSSecretName       string
}

func createPrometheusTLSSecretAndConfigMap(ctx context.Context, f *framework.Framework, opts prometheusTLSSecretAndConfigMapOptions) {
	crt, key, err := cert.GenerateSelfSignedCertKey(fmt.Sprintf("%s.%s.svc.cluster.local", opts.PrometheusName, f.Namespace()), nil, nil)
	o.Expect(err).NotTo(o.HaveOccurred())

	framework.By("Creating a TLS secret for Prometheus")
	_, err = f.KubeAdminClient().CoreV1().Secrets(f.Namespace()).Create(ctx, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      opts.TLSSecretName,
			Namespace: f.Namespace(),
		},
		Type: corev1.SecretTypeTLS,
		StringData: map[string]string{
			"tls.crt": string(crt),
			"tls.key": string(key),
		},
	}, metav1.CreateOptions{
		FieldManager: f.FieldManager(),
	})
	o.Expect(err).NotTo(o.HaveOccurred())

	framework.By("Creating a CA cert config map for Prometheus")
	_, err = f.KubeAdminClient().CoreV1().ConfigMaps(f.Namespace()).Create(ctx, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      opts.CACertConfigMapName,
			Namespace: f.Namespace(),
		},
		Data: map[string]string{
			"ca-bundle.crt": string(crt),
		},
	}, metav1.CreateOptions{
		FieldManager: f.FieldManager(),
	})
	o.Expect(err).NotTo(o.HaveOccurred())
}

func prometheusNameForScyllaDBMonitoring(smName string) string {
	return fmt.Sprintf("%s-prometheus", smName)
}

// createOpenShiftServiceCAConfigMap creates a ConfigMap containing the OpenShift Service CA certificate.
// See https://docs.redhat.com/en/documentation/openshift_container_platform/4.20/html/security_and_compliance/certificate-types-and-descriptions#cert-types-service-ca-certificates.
func createOpenShiftServiceCAConfigMap(ctx context.Context, f *framework.Framework, smName string) {
	_, err := f.KubeAdminClient().CoreV1().ConfigMaps(f.Namespace()).Create(ctx, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      openShiftServiceCAConfigMapName(smName),
			Namespace: f.Namespace(),
			Annotations: map[string]string{
				// This annotation will make OpenShift inject the Service CA bundle into this ConfigMap under the "ca-bundle.crt" key.
				"service.beta.openshift.io/inject-cabundle": "true",
			},
		},
	}, metav1.CreateOptions{
		FieldManager: f.FieldManager(),
	})
	o.Expect(err).NotTo(o.HaveOccurred())
}

func prometheusCACertConfigMapNameForScyllaDBMonitoring(smName string) string {
	return fmt.Sprintf("%s-prometheus-tls-ca", smName)
}

func prometheusTLSSecretNameForScyllaDBMonitoring(smName string) string {
	return fmt.Sprintf("%s-prometheus-tls", smName)
}

func monitoringAccessServiceAccountNameOnOpenShift(smName string) string {
	return fmt.Sprintf("%s-monitoring-access", smName)
}

func openShiftServiceCAConfigMapName(smName string) string {
	return fmt.Sprintf("%s-openshift-service-ca", smName)
}
