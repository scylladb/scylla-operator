package scyllacluster

import (
	"reflect"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

func Test_makeScyllaConnectionConfig(t *testing.T) {
	tt := []struct {
		name            string
		sc              *scyllav1.ScyllaCluster
		secrets         map[string]*corev1.Secret
		configMaps      map[string]*corev1.ConfigMap
		cqlsIngressPort int
		expected        *corev1.Secret
		expectedError   error
	}{
		{
			name: "single domain with port will generate bundle using explicit port",
			sc: &scyllav1.ScyllaCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo-ns",
					Name:      "bar",
				},
				Spec: scyllav1.ScyllaClusterSpec{
					DNSDomains: []string{
						"my-domain",
					},
					Datacenter: scyllav1.DatacenterSpec{
						Name: "us-east-1",
					},
				},
			},
			secrets: map[string]*corev1.Secret{
				"bar-local-user-admin": {
					Data: map[string][]byte{
						"tls.crt": []byte("admin-certificate-data"),
						"tls.key": []byte("admin-certificate-key"),
					},
				},
			},
			configMaps: map[string]*corev1.ConfigMap{
				"bar-local-serving-ca": {
					Data: map[string]string{
						"ca-bundle.crt": "serving-certificate-data",
					},
				},
			},
			cqlsIngressPort: 9142,
			expected: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo-ns",
					Name:      "bar-local-cql-connection-configs-admin",
					Labels: map[string]string{
						"app":                          "scylla",
						"app.kubernetes.io/managed-by": "scylla-operator",
						"app.kubernetes.io/name":       "scylla",
						"scylla/cluster":               "bar",
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "scylla.scylladb.com/v1",
							Kind:               "ScyllaCluster",
							Name:               "bar",
							Controller:         pointer.Bool(true),
							BlockOwnerDeletion: pointer.Bool(true),
						},
					},
				},
				Type: "Opaque",
				Data: map[string][]byte{
					"my-domain": []byte(strings.TrimPrefix(`
apiVersion: cqlclient.scylla.scylladb.com/v1alpha1
authInfos:
  admin:
    clientCertificateData: YWRtaW4tY2VydGlmaWNhdGUtZGF0YQ==
    clientKeyData: YWRtaW4tY2VydGlmaWNhdGUta2V5
    password: cassandra
    username: cassandra
contexts:
  default:
    authInfoName: admin
    datacenterName: us-east-1
currentContext: default
datacenters:
  us-east-1:
    certificateAuthorityData: c2VydmluZy1jZXJ0aWZpY2F0ZS1kYXRh
    nodeDomain: cql.my-domain
    server: cql.my-domain:9142
kind: CQLConnectionConfig
parameters:
  defaultConsistency: QUORUM
  defaultSerialConsistency: SERIAL
`, "\n")),
				},
			},
			expectedError: nil,
		},
		{
			name: "multi domain will generate multiple bundles",
			sc: &scyllav1.ScyllaCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo-ns",
					Name:      "bar",
				},
				Spec: scyllav1.ScyllaClusterSpec{
					DNSDomains: []string{
						"my-domain",
						"my-private-domain",
					},
					Datacenter: scyllav1.DatacenterSpec{
						Name: "us-east-1",
					},
				},
			},
			secrets: map[string]*corev1.Secret{
				"bar-local-user-admin": {
					Data: map[string][]byte{
						"tls.crt": []byte("admin-certificate-data"),
						"tls.key": []byte("admin-certificate-key"),
					},
				},
			},
			configMaps: map[string]*corev1.ConfigMap{
				"bar-local-serving-ca": {
					Data: map[string]string{
						"ca-bundle.crt": "serving-certificate-data",
					},
				},
			},
			cqlsIngressPort: 0,
			expected: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo-ns",
					Name:      "bar-local-cql-connection-configs-admin",
					Labels: map[string]string{
						"app":                          "scylla",
						"app.kubernetes.io/managed-by": "scylla-operator",
						"app.kubernetes.io/name":       "scylla",
						"scylla/cluster":               "bar",
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "scylla.scylladb.com/v1",
							Kind:               "ScyllaCluster",
							Name:               "bar",
							Controller:         pointer.Bool(true),
							BlockOwnerDeletion: pointer.Bool(true),
						},
					},
				},
				Type: "Opaque",
				Data: map[string][]byte{
					"my-domain": []byte(strings.TrimPrefix(`
apiVersion: cqlclient.scylla.scylladb.com/v1alpha1
authInfos:
  admin:
    clientCertificateData: YWRtaW4tY2VydGlmaWNhdGUtZGF0YQ==
    clientKeyData: YWRtaW4tY2VydGlmaWNhdGUta2V5
    password: cassandra
    username: cassandra
contexts:
  default:
    authInfoName: admin
    datacenterName: us-east-1
currentContext: default
datacenters:
  us-east-1:
    certificateAuthorityData: c2VydmluZy1jZXJ0aWZpY2F0ZS1kYXRh
    nodeDomain: cql.my-domain
    server: cql.my-domain
kind: CQLConnectionConfig
parameters:
  defaultConsistency: QUORUM
  defaultSerialConsistency: SERIAL
`, "\n")),
					"my-private-domain": []byte(strings.TrimPrefix(`
apiVersion: cqlclient.scylla.scylladb.com/v1alpha1
authInfos:
  admin:
    clientCertificateData: YWRtaW4tY2VydGlmaWNhdGUtZGF0YQ==
    clientKeyData: YWRtaW4tY2VydGlmaWNhdGUta2V5
    password: cassandra
    username: cassandra
contexts:
  default:
    authInfoName: admin
    datacenterName: us-east-1
currentContext: default
datacenters:
  us-east-1:
    certificateAuthorityData: c2VydmluZy1jZXJ0aWZpY2F0ZS1kYXRh
    nodeDomain: cql.my-private-domain
    server: cql.my-private-domain
kind: CQLConnectionConfig
parameters:
  defaultConsistency: QUORUM
  defaultSerialConsistency: SERIAL
`, "\n")),
				},
			},
			expectedError: nil,
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			got, err := makeScyllaConnectionConfig(tc.sc, tc.secrets, tc.configMaps, tc.cqlsIngressPort)
			if !reflect.DeepEqual(err, tc.expectedError) {
				t.Errorf("expected error %#v, got %#v", tc.expectedError, err)
			}

			if !reflect.DeepEqual(got, tc.expected) {
				t.Errorf("expected and actual connection configs differ: %s", cmp.Diff(tc.expected, got))
			}
		})
	}
}
