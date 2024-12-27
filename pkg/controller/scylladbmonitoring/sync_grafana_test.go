package scylladbmonitoring

import (
	"cmp"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"

	gcmp "github.com/google/go-cmp/cmp"
	grafanav1alpha1assets "github.com/scylladb/scylla-operator/assets/monitoring/grafana/v1alpha1"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/assets"
	oslices "github.com/scylladb/scylla-operator/pkg/helpers/slices"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apimachineryvalidation "k8s.io/apimachinery/pkg/api/validation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

func Test_makeGrafanaIngress(t *testing.T) {
	tt := []struct {
		name           string
		sm             *scyllav1alpha1.ScyllaDBMonitoring
		expectedString string
		expectedErr    error
	}{
		{
			name: "empty annotations",
			sm: &scyllav1alpha1.ScyllaDBMonitoring{
				ObjectMeta: metav1.ObjectMeta{
					Name: "sm-name",
				},
				Spec: scyllav1alpha1.ScyllaDBMonitoringSpec{
					Components: &scyllav1alpha1.Components{
						Grafana: &scyllav1alpha1.GrafanaSpec{
							ExposeOptions: &scyllav1alpha1.GrafanaExposeOptions{
								WebInterface: &scyllav1alpha1.HTTPSExposeOptions{
									Ingress: &scyllav1alpha1.IngressOptions{
										DNSDomains: []string{"grafana.localhost"},
									},
								},
							},
						},
					},
				},
			},
			expectedString: strings.TrimLeft(`
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: "sm-name-grafana"
  annotations:
    null
spec:
  ingressClassName: null
  rules:
  - host: "grafana.localhost"
    http:
      paths:
      - backend:
          service:
            name: "sm-name-grafana"
            port:
              number: 3000
        path: /
        pathType: Prefix
`, "\n"),
			expectedErr: nil,
		},
		{
			name: "supplied annotations",
			sm: &scyllav1alpha1.ScyllaDBMonitoring{
				ObjectMeta: metav1.ObjectMeta{
					Name: "sm-name",
				},
				Spec: scyllav1alpha1.ScyllaDBMonitoringSpec{
					Components: &scyllav1alpha1.Components{
						Grafana: &scyllav1alpha1.GrafanaSpec{
							ExposeOptions: &scyllav1alpha1.GrafanaExposeOptions{
								WebInterface: &scyllav1alpha1.HTTPSExposeOptions{
									Ingress: &scyllav1alpha1.IngressOptions{
										Annotations: map[string]string{
											"ann1": "ann1val",
											"ann2": "ann2val",
										},
										DNSDomains: []string{"grafana.localhost"},
									},
								},
							},
						},
					},
				},
			},
			expectedString: strings.TrimLeft(`
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: "sm-name-grafana"
  annotations:
    ann1: ann1val
    ann2: ann2val
spec:
  ingressClassName: null
  rules:
  - host: "grafana.localhost"
    http:
      paths:
      - backend:
          service:
            name: "sm-name-grafana"
            port:
              number: 3000
        path: /
        pathType: Prefix
`, "\n"),
			expectedErr: nil,
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			_, objString, err := makeGrafanaIngress(tc.sm)
			if !reflect.DeepEqual(err, tc.expectedErr) {
				t.Errorf("expected and got errors differ:\n%s\nRendered object:\n%s", gcmp.Diff(tc.expectedErr, err), objString)
			}

			if objString != tc.expectedString {
				t.Errorf("expected and got strings differ:\n%s", gcmp.Diff(
					strings.Split(tc.expectedString, "\n"),
					strings.Split(objString, "\n"),
				))
			}
		})
	}
}

func Test_makeGrafanaDashboards(t *testing.T) {
	// Platform dashboards come directly from ScyllaDB Monitoring, so we do not know the expected values
	// and given the size they are not feasible to be maintained as a duplicate.
	// Contrary to our testing practice, in this case we'll just make sure it's not empty and load
	// the expected values dynamically.
	getExpectedPlatformConfigMaps := func(smName string) []*corev1.ConfigMap {
		t.Helper()

		var expectedPlatformConfigMaps []*corev1.ConfigMap
		for dashboardFolderName, dashboardFolder := range grafanav1alpha1assets.GrafanaDashboardsPlatform.Get() {
			cm := &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ConfigMap",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: fmt.Sprintf("%s-grafana-scylladb-dashboards-%s", smName, assets.SanitizeDNSSubdomain(dashboardFolderName)),
					Annotations: map[string]string{
						"internal.scylla-operator.scylladb.com/dashboard-name": dashboardFolderName,
					},
				},
				Data: map[string]string{},
			}

			for dashboardFileName, dashboardString := range dashboardFolder {
				if len(dashboardString) == 0 {
					t.Fatalf("dashboard string can't be empty")
				}

				cm.Data[dashboardFileName] = "<non_empty>"
			}

			expectedPlatformConfigMaps = append(expectedPlatformConfigMaps, cm)
		}

		if len(expectedPlatformConfigMaps) == 0 {
			t.Fatalf("platform dashboard cases can't be empty")
		}

		slices.SortStableFunc(expectedPlatformConfigMaps, func(lhs, rhs *corev1.ConfigMap) int {
			return cmp.Compare(lhs.Name, rhs.Name)
		})

		return expectedPlatformConfigMaps
	}

	type testEntry struct {
		name               string
		sm                 *scyllav1alpha1.ScyllaDBMonitoring
		expectedConfigMaps []*corev1.ConfigMap
		expectedErr        error
	}
	tt := []testEntry{
		{
			name: "renders data for default SaaS type",
			sm: &scyllav1alpha1.ScyllaDBMonitoring{
				ObjectMeta: metav1.ObjectMeta{
					Name: "sm-name",
				},
				Spec: scyllav1alpha1.ScyllaDBMonitoringSpec{
					Type: nil,
				},
			},
			expectedConfigMaps: []*corev1.ConfigMap{
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       "ConfigMap",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "sm-name-grafana-scylladb-dashboards-scylladb-latest",
						Annotations: map[string]string{
							"internal.scylla-operator.scylladb.com/dashboard-name": "scylladb-latest",
						},
					},
					Data: map[string]string{
						"overview.json.gz.base64": "<non_empty>",
					},
				},
			},
			expectedErr: nil,
		},
		{
			name: "renders data for platform type",
			sm: &scyllav1alpha1.ScyllaDBMonitoring{
				ObjectMeta: metav1.ObjectMeta{
					Name: "sm-name",
				},
				Spec: scyllav1alpha1.ScyllaDBMonitoringSpec{
					Type: pointer.Ptr(scyllav1alpha1.ScyllaDBMonitoringTypePlatform),
				},
			},
			expectedConfigMaps: getExpectedPlatformConfigMaps("sm-name"),
			expectedErr:        nil,
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			cms, err := makeGrafanaDashboards(tc.sm)
			if !reflect.DeepEqual(err, tc.expectedErr) {
				t.Errorf("expected and got errors differ:\n%s", gcmp.Diff(tc.expectedErr, err))
			}

			if err == nil {
				for _, cm := range cms {
					errMessages := apimachineryvalidation.NameIsDNSSubdomain(cm.Name, false)
					validationErr := utilerrors.NewAggregate(oslices.ConvertSlice[error](errMessages, func(s string) error {
						return errors.New(s)
					}))

					if validationErr != nil {
						t.Errorf("invalid name for config map %q: %v", cm.Name, validationErr)
					}
				}
			}

			// We don't want to compare the large dashboards content that comes from a different repo and can change.
			for _, cm := range cms {
				for k, v := range cm.Data {
					if len(v) > 0 {
						cm.Data[k] = "<non_empty>"
					}
				}
			}

			if !equality.Semantic.DeepEqual(tc.expectedConfigMaps, cms) {
				t.Errorf("expected and got configmaps differ:\n%s", gcmp.Diff(tc.expectedConfigMaps, cms))
			}
		})
	}
}

func Test_makeGrafanaDeployment(t *testing.T) {
	defaultSOC := &scyllav1alpha1.ScyllaOperatorConfig{
		Status: scyllav1alpha1.ScyllaOperatorConfigStatus{
			GrafanaImage:   pointer.Ptr("grafana-image"),
			BashToolsImage: pointer.Ptr("bash-tools-image"),
		},
	}

	tt := []struct {
		name                         string
		sm                           *scyllav1alpha1.ScyllaDBMonitoring
		soc                          *scyllav1alpha1.ScyllaOperatorConfig
		grafanaServingCertSecretName string
		dashboardsCMs                []*corev1.ConfigMap
		restartTriggerHash           string
		expectedString               string
		expectedErr                  error
	}{
		{
			name: "renders data for default SaaS type",
			sm: &scyllav1alpha1.ScyllaDBMonitoring{
				ObjectMeta: metav1.ObjectMeta{
					Name: "sm-name",
				},
				Spec: scyllav1alpha1.ScyllaDBMonitoringSpec{
					Type: nil,
				},
			},
			soc:                          defaultSOC,
			grafanaServingCertSecretName: "serving-secret",
			dashboardsCMs: []*corev1.ConfigMap{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "sm-name-grafana-scylladb-dashboards-scylladb-latest",
						Annotations: map[string]string{
							"internal.scylla-operator.scylladb.com/dashboard-name": "scylladb-latest",
						},
					},
				},
			},
			restartTriggerHash: "restart-trigger-hash",
			expectedString: strings.TrimLeft(`
apiVersion: apps/v1
kind: Deployment
metadata:
  name: "sm-name-grafana"
spec:
  selector:
    matchLabels:
      scylla-operator.scylladb.com/deployment-name: "sm-name-grafana"
  strategy:
    type: RollingUpdate
  template:
    metadata:
      annotations:
        scylla-operator.scylladb.com/inputs-hash: "restart-trigger-hash"
      labels:
        scylla-operator.scylladb.com/deployment-name: "sm-name-grafana"
    spec:
      serviceAccountName: "sm-name-grafana"
      affinity:
        {}
      tolerations:
        null
      initContainers:
      - name: gzip
        image: "bash-tools-image"
        command:
        - /usr/bin/bash
        - -euExo
        - pipefail
        - -O
        - inherit_errexit
        - -c
        args:
        - |
          mkdir /var/run/decompressed-configmaps/grafana-scylladb-dashboards
          find /var/run/configmaps -mindepth 2 -maxdepth 2 -type d | while read -r d; do
            tp="/var/run/decompressed-configmaps/${d#"/var/run/configmaps/"}"
            mkdir "${tp}"
            find "${d}" -mindepth 1 -maxdepth 1 -name '*.gz.base64' -exec cp -L -t "${tp}" {} +
          done
          find /var/run/decompressed-configmaps -name '*.gz.base64' | while read -r f; do
            base64 -d "${f}" > "${f%.base64}"
            rm "${f}"
          done
          find /var/run/decompressed-configmaps -name '*.gz' -exec gzip -d {} +
        volumeMounts:
        - name: decompressed-configmaps
          mountPath: /var/run/decompressed-configmaps
        - name: "sm-name-grafana-scylladb-dashboards-scylladb-latest"
          mountPath: "/var/run/configmaps/grafana-scylladb-dashboards/scylladb-latest"
      containers:
      - name: grafana
        image: "grafana-image"
        command:
        - grafana-server
        - --packaging=docker
        - --homepath=/usr/share/grafana
        - --config=/var/run/configmaps/grafana-configs/grafana.ini
        env:
        - name: GF_PATHS_PROVISIONING
        - name: GF_PATHS_HOME
        - name: GF_PATHS_DATA
        - name: GF_PATHS_LOGS
        - name: GF_PATHS_PLUGINS
        - name: GF_PATHS_CONFIG
        ports:
        - containerPort: 3000
          name: grafana
          protocol: TCP
        readinessProbe:
          initialDelaySeconds: 10
          periodSeconds: 30
          timeoutSeconds: 5
          successThreshold: 1
          failureThreshold: 1
          httpGet:
            path: /api/health
            port: 3000
            scheme: HTTPS
        livenessProbe:
          initialDelaySeconds: 30
          periodSeconds: 30
          timeoutSeconds: 5
          successThreshold: 1
          failureThreshold: 10
          httpGet:
            path: /api/health
            port: 3000
            scheme: HTTPS
        resources:
          {}
        volumeMounts:
        - name: grafana-configs
          mountPath: /var/run/configmaps/grafana-configs
        - name: decompressed-configmaps
          mountPath: /var/run/dashboards/scylladb
          subPath: grafana-scylladb-dashboards
        - name: grafana-provisioning
          mountPath: /var/run/configmaps/grafana-provisioning/access-control/access-control.yaml
          subPath: access-control.yaml
        - name: grafana-provisioning
          mountPath: /var/run/configmaps/grafana-provisioning/alerting/alerting.yaml
          subPath: alerting.yaml
        - name: grafana-provisioning
          mountPath: /var/run/configmaps/grafana-provisioning/dashboards/dashboards.yaml
          subPath: dashboards.yaml
        - name: grafana-provisioning
          mountPath: /var/run/configmaps/grafana-provisioning/datasources/datasources.yaml
          subPath: datasources.yaml
        - name: grafana-provisioning
          mountPath: /var/run/configmaps/grafana-provisioning/notifiers/notifiers.yaml
          subPath: notifiers.yaml
        - name: grafana-provisioning
          mountPath: /var/run/configmaps/grafana-provisioning/plugins/plugins.yaml
          subPath: plugins.yaml
        - name: grafana-admin-credentials
          mountPath: /var/run/secrets/grafana-admin-credentials
        - name: grafana-serving-certs
          mountPath: /var/run/secrets/grafana-serving-certs
        - name: prometheus-client-certs
          mountPath: /var/run/secrets/prometheus-client-certs
        - name: prometheus-serving-ca
          mountPath: /var/run/configmaps/prometheus-serving-ca
        - name: grafana-storage
          mountPath: /var/lib/grafana
        securityContext:
          allowPrivilegeEscalation: false
          privileged: false
          runAsNonRoot: true
          runAsUser: 472
          runAsGroup: 472
          capabilities:
            drop:
            - ALL
      volumes:
      - name: decompressed-configmaps
        emptyDir:
          sizeLimit: 50Mi
      - name: grafana-configs
        configMap:
          name: "sm-name-grafana-configs"
      - name: "sm-name-grafana-scylladb-dashboards-scylladb-latest"
        configMap:
          name: "sm-name-grafana-scylladb-dashboards-scylladb-latest"
      - name: grafana-provisioning
        configMap:
          name: "sm-name-grafana-provisioning"
      - name: grafana-admin-credentials
        secret:
          secretName: "sm-name-grafana-admin-credentials"
      - name: grafana-serving-certs
        secret:
          secretName: "serving-secret"
      - name: prometheus-client-certs
        secret:
          secretName: "sm-name-prometheus-client-grafana"
      - name: prometheus-serving-ca
        configMap:
          name: "sm-name-prometheus-serving-ca"
      - name: grafana-storage
        emptyDir:
          sizeLimit: 100Mi
      securityContext:
        runAsNonRoot: true
        runAsUser: 472
        runAsGroup: 472
        fsGroup: 472
        seccompProfile:
          type: RuntimeDefault
`, "\n"),
			expectedErr: nil,
		},
		{
			name: "renders data for Platform type",
			sm: &scyllav1alpha1.ScyllaDBMonitoring{
				ObjectMeta: metav1.ObjectMeta{
					Name: "sm-name",
				},
				Spec: scyllav1alpha1.ScyllaDBMonitoringSpec{
					Type: pointer.Ptr(scyllav1alpha1.ScyllaDBMonitoringTypePlatform),
				},
			},
			soc:                          defaultSOC,
			grafanaServingCertSecretName: "serving-secret",
			dashboardsCMs: []*corev1.ConfigMap{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "sm-name-grafana-scylladb-dashboards-scylladb-6.0",
						Annotations: map[string]string{
							"internal.scylla-operator.scylladb.com/dashboard-name": "scylladb-6.0",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "sm-name-grafana-scylladb-dashboards-scylladb-6.1",
						Annotations: map[string]string{
							"internal.scylla-operator.scylladb.com/dashboard-name": "scylladb-6.1",
						},
					},
				},
			},
			restartTriggerHash: "restart-trigger-hash",
			expectedString: strings.TrimLeft(`
apiVersion: apps/v1
kind: Deployment
metadata:
  name: "sm-name-grafana"
spec:
  selector:
    matchLabels:
      scylla-operator.scylladb.com/deployment-name: "sm-name-grafana"
  strategy:
    type: RollingUpdate
  template:
    metadata:
      annotations:
        scylla-operator.scylladb.com/inputs-hash: "restart-trigger-hash"
      labels:
        scylla-operator.scylladb.com/deployment-name: "sm-name-grafana"
    spec:
      serviceAccountName: "sm-name-grafana"
      affinity:
        {}
      tolerations:
        null
      initContainers:
      - name: gzip
        image: "bash-tools-image"
        command:
        - /usr/bin/bash
        - -euExo
        - pipefail
        - -O
        - inherit_errexit
        - -c
        args:
        - |
          mkdir /var/run/decompressed-configmaps/grafana-scylladb-dashboards
          find /var/run/configmaps -mindepth 2 -maxdepth 2 -type d | while read -r d; do
            tp="/var/run/decompressed-configmaps/${d#"/var/run/configmaps/"}"
            mkdir "${tp}"
            find "${d}" -mindepth 1 -maxdepth 1 -name '*.gz.base64' -exec cp -L -t "${tp}" {} +
          done
          find /var/run/decompressed-configmaps -name '*.gz.base64' | while read -r f; do
            base64 -d "${f}" > "${f%.base64}"
            rm "${f}"
          done
          find /var/run/decompressed-configmaps -name '*.gz' -exec gzip -d {} +
        volumeMounts:
        - name: decompressed-configmaps
          mountPath: /var/run/decompressed-configmaps
        - name: "sm-name-grafana-scylladb-dashboards-scylladb-6.0"
          mountPath: "/var/run/configmaps/grafana-scylladb-dashboards/scylladb-6.0"
        - name: "sm-name-grafana-scylladb-dashboards-scylladb-6.1"
          mountPath: "/var/run/configmaps/grafana-scylladb-dashboards/scylladb-6.1"
      containers:
      - name: grafana
        image: "grafana-image"
        command:
        - grafana-server
        - --packaging=docker
        - --homepath=/usr/share/grafana
        - --config=/var/run/configmaps/grafana-configs/grafana.ini
        env:
        - name: GF_PATHS_PROVISIONING
        - name: GF_PATHS_HOME
        - name: GF_PATHS_DATA
        - name: GF_PATHS_LOGS
        - name: GF_PATHS_PLUGINS
        - name: GF_PATHS_CONFIG
        ports:
        - containerPort: 3000
          name: grafana
          protocol: TCP
        readinessProbe:
          initialDelaySeconds: 10
          periodSeconds: 30
          timeoutSeconds: 5
          successThreshold: 1
          failureThreshold: 1
          httpGet:
            path: /api/health
            port: 3000
            scheme: HTTPS
        livenessProbe:
          initialDelaySeconds: 30
          periodSeconds: 30
          timeoutSeconds: 5
          successThreshold: 1
          failureThreshold: 10
          httpGet:
            path: /api/health
            port: 3000
            scheme: HTTPS
        resources:
          {}
        volumeMounts:
        - name: grafana-configs
          mountPath: /var/run/configmaps/grafana-configs
        - name: decompressed-configmaps
          mountPath: /var/run/dashboards/scylladb
          subPath: grafana-scylladb-dashboards
        - name: grafana-provisioning
          mountPath: /var/run/configmaps/grafana-provisioning/access-control/access-control.yaml
          subPath: access-control.yaml
        - name: grafana-provisioning
          mountPath: /var/run/configmaps/grafana-provisioning/alerting/alerting.yaml
          subPath: alerting.yaml
        - name: grafana-provisioning
          mountPath: /var/run/configmaps/grafana-provisioning/dashboards/dashboards.yaml
          subPath: dashboards.yaml
        - name: grafana-provisioning
          mountPath: /var/run/configmaps/grafana-provisioning/datasources/datasources.yaml
          subPath: datasources.yaml
        - name: grafana-provisioning
          mountPath: /var/run/configmaps/grafana-provisioning/notifiers/notifiers.yaml
          subPath: notifiers.yaml
        - name: grafana-provisioning
          mountPath: /var/run/configmaps/grafana-provisioning/plugins/plugins.yaml
          subPath: plugins.yaml
        - name: grafana-admin-credentials
          mountPath: /var/run/secrets/grafana-admin-credentials
        - name: grafana-serving-certs
          mountPath: /var/run/secrets/grafana-serving-certs
        - name: prometheus-client-certs
          mountPath: /var/run/secrets/prometheus-client-certs
        - name: prometheus-serving-ca
          mountPath: /var/run/configmaps/prometheus-serving-ca
        - name: grafana-storage
          mountPath: /var/lib/grafana
        securityContext:
          allowPrivilegeEscalation: false
          privileged: false
          runAsNonRoot: true
          runAsUser: 472
          runAsGroup: 472
          capabilities:
            drop:
            - ALL
      volumes:
      - name: decompressed-configmaps
        emptyDir:
          sizeLimit: 50Mi
      - name: grafana-configs
        configMap:
          name: "sm-name-grafana-configs"
      - name: "sm-name-grafana-scylladb-dashboards-scylladb-6.0"
        configMap:
          name: "sm-name-grafana-scylladb-dashboards-scylladb-6.0"
      - name: "sm-name-grafana-scylladb-dashboards-scylladb-6.1"
        configMap:
          name: "sm-name-grafana-scylladb-dashboards-scylladb-6.1"
      - name: grafana-provisioning
        configMap:
          name: "sm-name-grafana-provisioning"
      - name: grafana-admin-credentials
        secret:
          secretName: "sm-name-grafana-admin-credentials"
      - name: grafana-serving-certs
        secret:
          secretName: "serving-secret"
      - name: prometheus-client-certs
        secret:
          secretName: "sm-name-prometheus-client-grafana"
      - name: prometheus-serving-ca
        configMap:
          name: "sm-name-prometheus-serving-ca"
      - name: grafana-storage
        emptyDir:
          sizeLimit: 100Mi
      securityContext:
        runAsNonRoot: true
        runAsUser: 472
        runAsGroup: 472
        fsGroup: 472
        seccompProfile:
          type: RuntimeDefault
`, "\n"),
			expectedErr: nil,
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			deployment, deploymentString, err := makeGrafanaDeployment(tc.sm, tc.soc, tc.grafanaServingCertSecretName, tc.dashboardsCMs, tc.restartTriggerHash)
			if !reflect.DeepEqual(err, tc.expectedErr) {
				t.Fatalf("expected and got errors differ:\n%s\nRendered object:\n%s", gcmp.Diff(tc.expectedErr, err), deploymentString)
			}

			if err == nil {
				for _, v := range deployment.Spec.Template.Spec.Volumes {
					errMessages := apimachineryvalidation.NameIsDNSSubdomain(v.Name, false)
					validationErr := utilerrors.NewAggregate(oslices.ConvertSlice[error](errMessages, func(s string) error {
						return errors.New(s)
					}))

					if validationErr != nil {
						t.Errorf("invalid volume name %q: %v", v.Name, validationErr)
					}
				}
			}

			if deploymentString != tc.expectedString {
				t.Errorf("expected and got strings differ:\n%s", gcmp.Diff(
					strings.Split(tc.expectedString, "\n"),
					strings.Split(deploymentString, "\n"),
				))
			}
		})
	}
}
