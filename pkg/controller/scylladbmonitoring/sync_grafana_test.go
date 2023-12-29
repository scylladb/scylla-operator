package scylladbmonitoring

import (
	"crypto/sha512"
	"encoding/base64"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	"github.com/scylladb/scylla-operator/pkg/scheme"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
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

func Test_makeGrafanaDashboards(t *testing.T) {
	tt := []struct {
		name           string
		sm             *scyllav1alpha1.ScyllaDBMonitoring
		expectedString string
		expectedErr    error
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
			expectedString: strings.TrimLeft(`
apiVersion: v1
data:
  overview.json.gz.base64: '<12812 bytes, hash: "wX2HNiQFcoq3ZHp/fxfzwHDcBs4L14XT5Ainng==">'
kind: ConfigMap
metadata:
  creationTimestamp: null
  name: sm-name-grafana-scylladb-dashboards
`, "\n"),
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
			expectedString: strings.TrimLeft(`
apiVersion: v1
data:
  advanced.json.gz.base64: '<10636 bytes, hash: "2FtFj4vQKwaZldf3EpV/lquuiKnceSFkKCvs/A==">'
  alternator.json.gz.base64: '<29564 bytes, hash: "kG1P7l18/b7pqlBGM1J085vpzK5rn3RpaJSsYA==">'
  cql.json.gz.base64: '<26344 bytes, hash: "JTpUOpCmNdZJ/2OcOPwSdqAkIpllDu/7VsRziw==">'
  detailed.json.gz.base64: '<32768 bytes, hash: "vD71wxAYWcAKgKeelKh70O0X2zevBjM7TILEVA==">'
  ks.json.gz.base64: '<19488 bytes, hash: "uiuutDdzdfGODAwu7/g84QpwwDsxbtfzt9RDOA==">'
  manager.json.gz.base64: '<19420 bytes, hash: "mbcqlRXTOcSVJbjutBrQZ4DzKqXCIiHtkD3jxA==">'
  os.json.gz.base64: '<19788 bytes, hash: "UIjUH/4tsJHHOZZ+ZbVOopQ8ScMY8KkYIRb2ZQ==">'
  overview.json.gz.base64: '<14204 bytes, hash: "dYi0WNJhWxsZNIf9ZChTqo9YRFiq4Zu2gNMAFQ==">'
kind: ConfigMap
metadata:
  creationTimestamp: null
  name: sm-name-grafana-scylladb-dashboards
`, "\n"),
			expectedErr: nil,
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			cm, _, err := makeGrafanaDashboards(tc.sm)
			if !reflect.DeepEqual(err, tc.expectedErr) {
				t.Errorf("expected and got errors differ:\n%s", cmp.Diff(tc.expectedErr, err))
			}

			// To avoid embedding thousands of lines we make a sum.
			totalSize := 0
			for k, v := range cm.Data {
				hash := sha512.Sum512_224([]byte(v))
				cm.Data[k] = fmt.Sprintf("<%d bytes, hash: %q>", len(v), base64.StdEncoding.EncodeToString(hash[:]))
				totalSize += len(v)
			}

			t.Logf("ConfigMap %q contains %d byte(s) of data", cm.Name, totalSize)

			objStringBytes, err := runtime.Encode(scheme.DefaultYamlSerializer, cm)
			if err != nil {
				t.Fatal(err)
			}

			objString := string(objStringBytes)
			if objString != tc.expectedString {
				t.Errorf("expected and got strings differ:\n%s", cmp.Diff(
					strings.Split(tc.expectedString, "\n"),
					strings.Split(objString, "\n"),
				))
			}

			// Make sure we don't outgrow max CM size.
			if totalSize > corev1.MaxSecretSize {
				t.Errorf("configmap size too large: size %d exceeded max size %d", totalSize, corev1.MaxSecretSize)
			}
		})
	}
}
