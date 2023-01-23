package scylladbmonitoring

import (
	"reflect"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
