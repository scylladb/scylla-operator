package scylladbmonitoring

import (
	"reflect"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	configassests "github.com/scylladb/scylla-operator/assets/config"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	monitoringv1 "github.com/scylladb/scylla-operator/pkg/externalapi/monitoring/v1"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_makeScyllaDBServiceMonitor(t *testing.T) {
	tt := []struct {
		name           string
		sm             *scyllav1alpha1.ScyllaDBMonitoring
		expectedString string
		expectedErr    error
	}{
		{
			name: "empty selector",
			sm: &scyllav1alpha1.ScyllaDBMonitoring{
				ObjectMeta: metav1.ObjectMeta{
					Name: "sm-name",
				},
			},
			expectedString: strings.TrimLeft(`
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: "sm-name-scylladb"
spec:
  selector:
    {}
  jobLabel: scylla/cluster
  endpoints:
  - port: node-exporter
    honorLabels: false
    relabelings:
    - sourceLabels: [__address__]
      regex: '(.*):\d+'
      targetLabel: instance
      replacement: '${1}'
    - sourceLabels: [__address__]
      regex: '([^:]+)'
      targetLabel: instance
      replacement: '${1}'
    - sourceLabels: [instance]
      regex: '(.*)'
      targetLabel: __address__
      replacement: '${1}:9100'
    - sourceLabels: [__meta_kubernetes_service_label_scylla_cluster]
      regex:  '(.+)'
      targetLabel: cluster
      replacement: '${1}'
    - sourceLabels: [__meta_kubernetes_pod_label_scylla_datacenter]
      regex:  '(.+)'
      targetLabel: dc
      replacement: '${1}'
    # Scylla Monitoring OS Metrics dashboard expect node exporter metrics to have 'job=node_exporter'
    - sourceLabels: [__meta_kubernetes_endpoint_port_name]
      regex: '(.+)'
      replacement: 'node_exporter'
      targetLabel: job
  - port: prometheus
    honorLabels: false
    metricRelabelings:
    - sourceLabels: [version]
      regex:  '(.+)'
      targetLabel: CPU
      replacement: 'cpu'
    - sourceLabels: [version]
      regex:  '(.+)'
      targetLabel: CQL
      replacement: 'cql'
    - sourceLabels: [version]
      regex:  '(.+)'
      targetLabel: OS
      replacement: 'os'
    - sourceLabels: [version]
      regex:  '(.+)'
      targetLabel: IO
      replacement: 'io'
    - sourceLabels: [version]
      regex:  '(.+)'
      targetLabel: Errors
      replacement: 'errors'
    - regex: 'help|exported_instance'
      action: labeldrop
    - sourceLabels: [version]
      regex: '([0-9]+\.[0-9]+)(\.?[0-9]*).*'
      replacement: '$1$2'
      targetLabel: svr
    relabelings:
    - sourceLabels: [__address__]
      regex:  '(.*):.+'
      targetLabel: instance
      replacement: '${1}'
    - sourceLabels: [__meta_kubernetes_service_label_scylla_cluster]
      regex:  '(.+)'
      targetLabel: cluster
      replacement: '${1}'
    - sourceLabels: [__meta_kubernetes_pod_label_scylla_datacenter]
      regex:  '(.+)'
      targetLabel: dc
      replacement: '${1}'
`, "\n"),
			expectedErr: nil,
		},
		{
			name: "specific selector",
			sm: &scyllav1alpha1.ScyllaDBMonitoring{
				ObjectMeta: metav1.ObjectMeta{
					Name: "sm-name",
				},
				Spec: scyllav1alpha1.ScyllaDBMonitoringSpec{
					EndpointsSelector: metav1.LabelSelector{
						MatchLabels: map[string]string{
							"foo": "bar",
						},
						MatchExpressions: []metav1.LabelSelectorRequirement{
							{
								Key:      "alpha",
								Operator: metav1.LabelSelectorOpExists,
								Values:   []string{"beta"},
							},
						},
					},
				},
			},
			expectedString: strings.TrimLeft(`
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: "sm-name-scylladb"
spec:
  selector:
    matchExpressions:
    - key: alpha
      operator: Exists
      values:
      - beta
    matchLabels:
      foo: bar
  jobLabel: scylla/cluster
  endpoints:
  - port: node-exporter
    honorLabels: false
    relabelings:
    - sourceLabels: [__address__]
      regex: '(.*):\d+'
      targetLabel: instance
      replacement: '${1}'
    - sourceLabels: [__address__]
      regex: '([^:]+)'
      targetLabel: instance
      replacement: '${1}'
    - sourceLabels: [instance]
      regex: '(.*)'
      targetLabel: __address__
      replacement: '${1}:9100'
    - sourceLabels: [__meta_kubernetes_service_label_scylla_cluster]
      regex:  '(.+)'
      targetLabel: cluster
      replacement: '${1}'
    - sourceLabels: [__meta_kubernetes_pod_label_scylla_datacenter]
      regex:  '(.+)'
      targetLabel: dc
      replacement: '${1}'
    # Scylla Monitoring OS Metrics dashboard expect node exporter metrics to have 'job=node_exporter'
    - sourceLabels: [__meta_kubernetes_endpoint_port_name]
      regex: '(.+)'
      replacement: 'node_exporter'
      targetLabel: job
  - port: prometheus
    honorLabels: false
    metricRelabelings:
    - sourceLabels: [version]
      regex:  '(.+)'
      targetLabel: CPU
      replacement: 'cpu'
    - sourceLabels: [version]
      regex:  '(.+)'
      targetLabel: CQL
      replacement: 'cql'
    - sourceLabels: [version]
      regex:  '(.+)'
      targetLabel: OS
      replacement: 'os'
    - sourceLabels: [version]
      regex:  '(.+)'
      targetLabel: IO
      replacement: 'io'
    - sourceLabels: [version]
      regex:  '(.+)'
      targetLabel: Errors
      replacement: 'errors'
    - regex: 'help|exported_instance'
      action: labeldrop
    - sourceLabels: [version]
      regex: '([0-9]+\.[0-9]+)(\.?[0-9]*).*'
      replacement: '$1$2'
      targetLabel: svr
    relabelings:
    - sourceLabels: [__address__]
      regex:  '(.*):.+'
      targetLabel: instance
      replacement: '${1}'
    - sourceLabels: [__meta_kubernetes_service_label_scylla_cluster]
      regex:  '(.+)'
      targetLabel: cluster
      replacement: '${1}'
    - sourceLabels: [__meta_kubernetes_pod_label_scylla_datacenter]
      regex:  '(.+)'
      targetLabel: dc
      replacement: '${1}'
`, "\n"),
			expectedErr: nil,
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			_, objString, err := makeScyllaDBServiceMonitor(tc.sm)
			if !reflect.DeepEqual(err, tc.expectedErr) {
				t.Errorf("expected and got errors differ:\n%s\nRendered object:\n%s", cmp.Diff(tc.expectedErr, err), objString)
			}

			if objString != tc.expectedString {
				t.Errorf("expected and got strings differ:\n%s", cmp.Diff(
					strings.Split(tc.expectedString, "\n"),
					strings.Split(objString, "\n"),
				))
			}
		})
	}
}

func Test_makePrometheus(t *testing.T) {
	tt := []struct {
		name           string
		sm             *scyllav1alpha1.ScyllaDBMonitoring
		expectedString string
		expectedErr    error
	}{
		{
			name: "no storage",
			sm: &scyllav1alpha1.ScyllaDBMonitoring{
				ObjectMeta: metav1.ObjectMeta{
					Name: "sm-name",
				},
			},
			expectedString: strings.TrimLeft(`
apiVersion: monitoring.coreos.com/v1
kind: Prometheus
metadata:
  name: "sm-name"
spec:
  version: "`+configassests.Project.Operator.PrometheusVersion+`"
  serviceAccountName: "sm-name-prometheus"
  securityContext:
    runAsNonRoot: true
    runAsUser: 65534
    fsGroup: 65534
  web:
    pageTitle: "ScyllaDB Prometheus"
    tlsConfig:
      cert:
        secret:
          name: "sm-name-prometheus-serving-certs"
          key: "tls.crt"
      keySecret:
        name: "sm-name-prometheus-serving-certs"
        key: "tls.key"
#      clientAuthType: "RequireAndVerifyClientCert"
#      TODO: we need the prometheus-operator not to require certs only for /-/readyz or to do exec probes that can read certs
      clientAuthType: "RequestClientCert"
      client_ca:
        configMap:
          name: "sm-name-prometheus-client-ca"
          key: "ca-bundle.crt"
    httpConfig:
      http2: true
  serviceMonitorSelector:
    matchLabels:
      scylla-operator.scylladb.com/scylladbmonitoring-name: "sm-name"
  affinity:
    {}
  tolerations:
    null
  resources:
    {}
  alerting:
    alertmanagers:
    - namespace: ""
      name: "sm-name"
      port: web
  ruleSelector:
    matchLabels:
      scylla-operator.scylladb.com/scylladbmonitoring-name: "sm-name"
`, "\n"),
			expectedErr: nil,
		},
		{
			name: "with prometheus pvc template",
			sm: &scyllav1alpha1.ScyllaDBMonitoring{
				ObjectMeta: metav1.ObjectMeta{
					Name: "sm-name",
				},
				Spec: scyllav1alpha1.ScyllaDBMonitoringSpec{
					Components: &scyllav1alpha1.Components{
						Prometheus: &scyllav1alpha1.PrometheusSpec{
							Storage: &scyllav1alpha1.Storage{
								VolumeClaimTemplate: corev1.PersistentVolumeClaimTemplate{
									ObjectMeta: metav1.ObjectMeta{},
									Spec: corev1.PersistentVolumeClaimSpec{
										StorageClassName: pointer.Ptr("pv-class"),
										Resources: corev1.VolumeResourceRequirements{
											Requests: map[corev1.ResourceName]resource.Quantity{
												corev1.ResourceStorage: resource.MustParse("5Gi"),
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expectedString: strings.TrimLeft(`
apiVersion: monitoring.coreos.com/v1
kind: Prometheus
metadata:
  name: "sm-name"
spec:
  version: "`+configassests.Project.Operator.PrometheusVersion+`"
  serviceAccountName: "sm-name-prometheus"
  securityContext:
    runAsNonRoot: true
    runAsUser: 65534
    fsGroup: 65534
  web:
    pageTitle: "ScyllaDB Prometheus"
    tlsConfig:
      cert:
        secret:
          name: "sm-name-prometheus-serving-certs"
          key: "tls.crt"
      keySecret:
        name: "sm-name-prometheus-serving-certs"
        key: "tls.key"
#      clientAuthType: "RequireAndVerifyClientCert"
#      TODO: we need the prometheus-operator not to require certs only for /-/readyz or to do exec probes that can read certs
      clientAuthType: "RequestClientCert"
      client_ca:
        configMap:
          name: "sm-name-prometheus-client-ca"
          key: "ca-bundle.crt"
    httpConfig:
      http2: true
  serviceMonitorSelector:
    matchLabels:
      scylla-operator.scylladb.com/scylladbmonitoring-name: "sm-name"
  affinity:
    {}
  tolerations:
    null
  resources:
    {}
  alerting:
    alertmanagers:
    - namespace: ""
      name: "sm-name"
      port: web
  ruleSelector:
    matchLabels:
      scylla-operator.scylladb.com/scylladbmonitoring-name: "sm-name"
  storage:
    volumeClaimTemplate:
      metadata:
        name: sm-name-prometheus
      spec:
        resources:
          requests:
            storage: 5Gi
        storageClassName: pv-class
      status: {}
`, "\n"),
			expectedErr: nil,
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			_, objString, err := makePrometheus(tc.sm, &scyllav1alpha1.ScyllaOperatorConfig{
				Status: scyllav1alpha1.ScyllaOperatorConfigStatus{
					PrometheusVersion: pointer.Ptr(configassests.Project.Operator.PrometheusVersion),
				},
			})
			if !reflect.DeepEqual(err, tc.expectedErr) {
				t.Errorf("expected and got errors differ:\n%s\nRendered object:\n%s", cmp.Diff(tc.expectedErr, err), objString)
			}

			if objString != tc.expectedString {
				t.Errorf("expected and got strings differ:\n%s", cmp.Diff(
					strings.Split(tc.expectedString, "\n"),
					strings.Split(objString, "\n"),
				))
			}
		})
	}
}

func Test_PrometheusRules(t *testing.T) {
	tt := []struct {
		name         string
		genFunc      func(sm *scyllav1alpha1.ScyllaDBMonitoring) (*monitoringv1.PrometheusRule, string, error)
		sm           *scyllav1alpha1.ScyllaDBMonitoring
		expectedRule *monitoringv1.PrometheusRule
		expectedErr  error
	}{
		{
			name:    "latency rule renders correctly",
			genFunc: makeLatencyPrometheusRule,
			sm: &scyllav1alpha1.ScyllaDBMonitoring{
				ObjectMeta: metav1.ObjectMeta{
					Name: "sm-name",
				},
			},
			expectedRule: &monitoringv1.PrometheusRule{
				TypeMeta: metav1.TypeMeta{
					Kind:       "PrometheusRule",
					APIVersion: monitoringv1.SchemeGroupVersion.String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "sm-name-latency",
					Labels: map[string]string{
						"scylla-operator.scylladb.com/scylladbmonitoring-name": "sm-name",
					},
				},
			},
		},
		{
			name:    "alerts rule renders correctly",
			genFunc: makeAlertsPrometheusRule,
			sm: &scyllav1alpha1.ScyllaDBMonitoring{
				ObjectMeta: metav1.ObjectMeta{
					Name: "sm-name",
				},
			},
			expectedRule: &monitoringv1.PrometheusRule{
				TypeMeta: metav1.TypeMeta{
					Kind:       "PrometheusRule",
					APIVersion: monitoringv1.SchemeGroupVersion.String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "sm-name-alerts",
					Labels: map[string]string{
						"scylla-operator.scylladb.com/scylladbmonitoring-name": "sm-name",
					},
				},
			},
		},
		{
			name:    "table rule renders correctly",
			genFunc: makeTablePrometheusRule,
			sm: &scyllav1alpha1.ScyllaDBMonitoring{
				ObjectMeta: metav1.ObjectMeta{
					Name: "sm-name",
				},
			},
			expectedRule: &monitoringv1.PrometheusRule{
				TypeMeta: metav1.TypeMeta{
					Kind:       "PrometheusRule",
					APIVersion: monitoringv1.SchemeGroupVersion.String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "sm-name-table",
					Labels: map[string]string{
						"scylla-operator.scylladb.com/scylladbmonitoring-name": "sm-name",
					},
				},
			},
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			promRule, _, err := tc.genFunc(tc.sm)

			if !reflect.DeepEqual(err, tc.expectedErr) {
				t.Fatalf("expected and got errors differ:\n%s", cmp.Diff(tc.expectedErr, err))
			}

			if len(promRule.Spec.Groups) == 0 {
				t.Errorf("each prometheus rule should have at least one group")
			}

			promRule.Spec = monitoringv1.PrometheusRuleSpec{}
			if !apiequality.Semantic.DeepEqual(tc.expectedRule, promRule) {
				t.Fatalf("expected and got prometheus rules differ:\n%s", cmp.Diff(tc.expectedRule, promRule))
			}
		})
	}
}
