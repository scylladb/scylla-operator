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
	"net/url"
	"path/filepath"
	"strings"
	"time"

	gapi "github.com/grafana/grafana-api-golang-client"
	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	prometheusappclient "github.com/prometheus/client_golang/api"
	promeheusappv1api "github.com/prometheus/client_golang/api/prometheus/v1"
	configassests "github.com/scylladb/scylla-operator/assets/config"
	grafanav1alpha1assets "github.com/scylladb/scylla-operator/assets/monitoring/grafana/v1alpha1"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	scyllafixture "github.com/scylladb/scylla-operator/test/e2e/fixture/scylla"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	scyllaclusterverification "github.com/scylladb/scylla-operator/test/e2e/utils/verification/scyllacluster"
	"github.com/scylladb/scylla-operator/test/e2e/verification"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

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

// expectedPlatformFolderDashboardSearchResponse contains the expected grafana dashboards.
// Platform dashboards come directly from ScyllaDB Monitoring, so we do not know the expected values
// and given the size they are not feasible to be maintained as a duplicate.
// Contrary to our testing practice, in this case we'll just make sure it's not empty and load
// the expected values dynamically.
var expectedPlatformFolderDashboardSearchResponse []gapi.FolderDashboardSearchResponse
var expectedPlatformHomeDashboardUID string

var _ = g.BeforeSuite(func() {
	for dashboardFolderName, dashboardFolder := range grafanav1alpha1assets.GrafanaDashboardsPlatform.Get() {
		for _, dashboardString := range dashboardFolder {
			o.Expect(dashboardString).NotTo(o.BeEmpty())

			gd, err := decodeGrafanaDashboardFromGZBase64String(dashboardString)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(gd).NotTo(o.BeZero())

			expectedPlatformFolderDashboardSearchResponse = append(expectedPlatformFolderDashboardSearchResponse, gapi.FolderDashboardSearchResponse{
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

	expectedPlatformHomeDashboardUID = ghd.UID
})

var _ = g.Describe("ScyllaDBMonitoring", func() {
	f := framework.NewFramework("scylladbmonitoring")

	type entry struct {
		Type                     scyllav1alpha1.ScyllaDBMonitoringType
		ExpectedDashboards       *[]gapi.FolderDashboardSearchResponse
		ExpectedHomeDashboardUID *string
	}

	describeEntry := func(e *entry) string {
		return fmt.Sprintf("with %q monitoring type", e.Type)
	}

	// Disabled on OpenShift because of https://github.com/scylladb/scylla-operator/issues/2319#issuecomment-2643287819
	g.DescribeTable("should setup monitoring stack TESTCASE_DISABLED_ON_OPENSHIFT", func(e *entry) {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		defer cancel()

		sc := f.GetDefaultScyllaCluster()
		o.Expect(sc.Spec.Datacenter.Racks).To(o.HaveLen(1))
		sc.Spec.Datacenter.Racks[0].Members = 1

		framework.By("Creating a ScyllaCluster with a single node")
		sc, err := f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Create(
			ctx,
			sc,
			metav1.CreateOptions{
				FieldManager:    f.FieldManager(),
				FieldValidation: metav1.FieldValidationStrict,
			},
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Creating a ScyllaDBMonitoring")
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
		sm.Spec.Type = &e.Type

		sm, err = f.ScyllaClient().ScyllaV1alpha1().ScyllaDBMonitorings(sc.Namespace).Create(
			ctx,
			sm,
			metav1.CreateOptions{
				FieldManager: f.FieldManager(),
				// Disable strict validation until https://github.com/scylladb/scylla-operator/issues/1251 is fixed
				FieldValidation: metav1.FieldValidationWarn,
			},
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaCluster to roll out (RV=%s)", sc.ResourceVersion)
		waitCtx1, waitCtx1Cancel := utils.ContextForRollout(ctx, sc)
		defer waitCtx1Cancel()
		sc, err = controllerhelpers.WaitForScyllaClusterState(waitCtx1, f.ScyllaClient().ScyllaV1().ScyllaClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		scyllaclusterverification.Verify(ctx, f.KubeClient(), f.ScyllaClient(), sc)

		framework.By("Waiting for the ScyllaDBMonitoring to roll out (RV=%s)", sm.ResourceVersion)
		waitCtx2, waitCtx2Cancel := context.WithTimeout(ctx, 5*time.Minute)
		defer waitCtx2Cancel()
		sm, err = controllerhelpers.WaitForScyllaDBMonitoringState(waitCtx2, f.ScyllaClient().ScyllaV1alpha1().ScyllaDBMonitorings(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaDBMonitoringRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		// We need to retry the prometheus and grafana assertion for several reasons, some of them are:
		//  - ingress exposure is asynchronous and some controllers don't report back status to wait for
		//  - prometheus configuration is asynchronous without any acknowledgement
		//  - grafana configuration is asynchronous without any acknowledgement
		// Some of these may be fixable by manually verifying it in the operator sync loop so it can also be
		// consumed by clients, but it's a bigger effort.

		framework.By("Verifying that Prometheus is configured correctly")

		prometheusServingCABundleConfigMap, err := f.KubeClient().CoreV1().ConfigMaps(f.Namespace()).Get(ctx, fmt.Sprintf("%s-prometheus-serving-ca", sm.Name), metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		prometheusServingCACerts, _ := verification.VerifyAndParseCABundle(prometheusServingCABundleConfigMap)
		o.Expect(prometheusServingCACerts).To(o.HaveLen(1))

		prometheusServingCAPool := x509.NewCertPool()
		prometheusServingCAPool.AddCert(prometheusServingCACerts[0])

		prometheusGrafanaClientSecret, err := f.KubeClient().CoreV1().Secrets(f.Namespace()).Get(ctx, fmt.Sprintf("%s-prometheus-client-grafana", sm.Name), metav1.GetOptions{})
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

		promClient := promeheusappv1api.NewAPI(promHTTPClient)

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

		framework.By("Verifying that Grafana is configured correctly")

		grafanaAdminCredentialsSecret, err := f.KubeClient().CoreV1().Secrets(f.Namespace()).Get(ctx, fmt.Sprintf("%s-grafana-admin-credentials", sc.Name), metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(grafanaAdminCredentialsSecret.Data).To(o.HaveLen(2))
		o.Expect(grafanaAdminCredentialsSecret.Data).To(o.HaveKey("username"))
		o.Expect(grafanaAdminCredentialsSecret.Data).To(o.HaveKey("password"))

		grafanaUsername := string(grafanaAdminCredentialsSecret.Data["username"])
		o.Expect(grafanaUsername).NotTo(o.BeEmpty())
		grafanaPassword := string(grafanaAdminCredentialsSecret.Data["password"])
		o.Expect(grafanaPassword).NotTo(o.BeEmpty())

		grafanaServingCABundleConfigMap, err := f.KubeClient().CoreV1().ConfigMaps(f.Namespace()).Get(ctx, fmt.Sprintf("%s-grafana-serving-ca", sc.Name), metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		grafanaServingCACerts, _ := verification.VerifyAndParseCABundle(grafanaServingCABundleConfigMap)
		o.Expect(grafanaServingCACerts).To(o.HaveLen(1))

		grafanaServingCAPool := x509.NewCertPool()
		grafanaServingCAPool.AddCert(grafanaServingCACerts[0])

		o.Expect(sm.Spec.Components.Grafana.ExposeOptions.WebInterface.Ingress.DNSDomains).To(o.HaveLen(1))
		grafanaServerName := sm.Spec.Components.Grafana.ExposeOptions.WebInterface.Ingress.DNSDomains[0]

		httpClient := &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					ServerName: grafanaServerName,
					RootCAs:    grafanaServingCAPool,
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
			Timeout: 15 * time.Second,
		}
		grafanaBaseURL := "https://" + f.GetIngressAddress(grafanaServerName)
		grafanaClient, err := gapi.New(
			grafanaBaseURL,
			gapi.Config{
				BasicAuth: url.UserPassword(grafanaUsername, grafanaPassword),
				Client:    httpClient,
			},
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		var dashboards []gapi.FolderDashboardSearchResponse
		o.Eventually(func(eo o.Gomega) {
			dashboards, err = grafanaClient.Dashboards()
			framework.Infof("Listing grafana dashboards: err: %v, count: %d", err, len(dashboards))
			eo.Expect(err).NotTo(o.HaveOccurred())
			eo.Expect(dashboards).To(o.HaveLen(len(*e.ExpectedDashboards)))
		}).WithTimeout(10 * time.Minute).WithPolling(1 * time.Second).Should(o.Succeed())

		for i := range dashboards {
			d := &dashboards[i]

			// Clear random fields for comparison.
			d.UID = ""
			d.URL = ""
			d.FolderUID = ""
			d.FolderURL = ""

			// Clear order dependent fields for comparison.
			d.ID = 0
			d.FolderID = 0

			// Clear fields we don't want to compare
			d.URI = ""
		}
		o.Expect(dashboards).To(o.ConsistOf(*e.ExpectedDashboards))

		// The home dashboard API is not exposed in the client and OrgPreferences return empty values,
		// so we have to call the API directly here.
		ghdReq, err := http.NewRequestWithContext(ctx, "GET", grafanaBaseURL+"/api/dashboards/home", nil)
		o.Expect(err).NotTo(o.HaveOccurred())
		ghdReq.SetBasicAuth(grafanaUsername, grafanaPassword)
		ghdResp, err := httpClient.Do(ghdReq)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer func() {
			closeErr := ghdResp.Body.Close()
			o.Expect(closeErr).NotTo(o.HaveOccurred())
		}()
		ghdRespBody, err := io.ReadAll(ghdResp.Body)
		o.Expect(err).NotTo(o.HaveOccurred())

		type homeDashboardResponse struct {
			Dashboard grafanaDashboard `json:"dashboard"`
		}
		hdResp := &homeDashboardResponse{}
		err = json.Unmarshal(ghdRespBody, hdResp)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(hdResp.Dashboard.UID).To(o.Equal(*e.ExpectedHomeDashboardUID))
	},
		g.Entry(describeEntry, &entry{
			Type:                     scyllav1alpha1.ScyllaDBMonitoringTypeSAAS,
			ExpectedHomeDashboardUID: pointer.Ptr("cql-overview"),
			ExpectedDashboards: pointer.Ptr([]gapi.FolderDashboardSearchResponse{
				{
					Title:       "CQL Overview",
					Type:        "dash-db",
					FolderTitle: "scylladb-latest",
					Tags:        []string{},
				},
			}),
		}),
		g.Entry(describeEntry, &entry{
			Type:                     scyllav1alpha1.ScyllaDBMonitoringTypePlatform,
			ExpectedHomeDashboardUID: &expectedPlatformHomeDashboardUID,
			ExpectedDashboards:       &expectedPlatformFolderDashboardSearchResponse,
		}),
	)
})
