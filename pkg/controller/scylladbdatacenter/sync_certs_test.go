package scylladbdatacenter

import (
	"reflect"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_makeScyllaConnectionConfig(t *testing.T) {
	tt := []struct {
		name            string
		sdc             *scyllav1alpha1.ScyllaDBDatacenter
		secrets         map[string]*corev1.Secret
		configMaps      map[string]*corev1.ConfigMap
		cqlsIngressPort int
		expected        *corev1.Secret
		expectedError   error
	}{
		{
			name: "single domain with port will generate bundle using explicit port",
			sdc: &scyllav1alpha1.ScyllaDBDatacenter{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo-ns",
					Name:      "bar",
				},
				Spec: scyllav1alpha1.ScyllaDBDatacenterSpec{
					ClusterName: "bar",
					DNSDomains: []string{
						"my-domain",
					},
					DatacenterName: pointer.Ptr("us-east-1"),
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
							APIVersion:         "scylla.scylladb.com/v1alpha1",
							Kind:               "ScyllaDBDatacenter",
							Name:               "bar",
							Controller:         pointer.Ptr(true),
							BlockOwnerDeletion: pointer.Ptr(true),
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
			sdc: &scyllav1alpha1.ScyllaDBDatacenter{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo-ns",
					Name:      "bar",
				},
				Spec: scyllav1alpha1.ScyllaDBDatacenterSpec{
					ClusterName: "bar",
					DNSDomains: []string{
						"my-domain",
						"my-private-domain",
					},
					DatacenterName: pointer.Ptr("us-east-1"),
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
							APIVersion:         "scylla.scylladb.com/v1alpha1",
							Kind:               "ScyllaDBDatacenter",
							Name:               "bar",
							Controller:         pointer.Ptr(true),
							BlockOwnerDeletion: pointer.Ptr(true),
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
			got, err := makeScyllaConnectionConfig(tc.sdc, tc.secrets, tc.configMaps, tc.cqlsIngressPort)
			if !reflect.DeepEqual(err, tc.expectedError) {
				t.Errorf("expected error %#v, got %#v", tc.expectedError, err)
			}

			if !reflect.DeepEqual(got, tc.expected) {
				t.Errorf("expected and actual connection configs differ: %s", cmp.Diff(tc.expected, got))
			}
		})
	}
}

func Test_getMemberServiceHostIDs(t *testing.T) {
	t.Parallel()

	newService := func(name, serviceType string, annotations map[string]string) *corev1.Service {
		return &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "foo-ns",
				Name:      name,
				Labels: map[string]string{
					naming.ScyllaServiceTypeLabel: serviceType,
				},
				Annotations: annotations,
			},
		}
	}

	tt := []struct {
		name                        string
		serviceMap                  map[string]*corev1.Service
		expectedHostIDs             []string
		expectedProgressingMessages []string
	}{
		{
			name:                        "no services",
			serviceMap:                  map[string]*corev1.Service{},
			expectedHostIDs:             nil,
			expectedProgressingMessages: nil,
		},
		{
			name: "non-member services are ignored",
			serviceMap: map[string]*corev1.Service{
				"identity": newService("identity", string(naming.ScyllaServiceTypeIdentity), nil),
			},
			expectedHostIDs:             nil,
			expectedProgressingMessages: nil,
		},
		{
			name: "hostIDs and messages are sorted regardless of the map order",
			serviceMap: map[string]*corev1.Service{
				"member-2": newService("member-2", string(naming.ScyllaServiceTypeMember), map[string]string{
					naming.HostIDAnnotation: "b-host-id",
				}),
				"member-0": newService("member-0", string(naming.ScyllaServiceTypeMember), map[string]string{
					naming.HostIDAnnotation: "a-host-id",
				}),
				"member-3": newService("member-3", string(naming.ScyllaServiceTypeMember), nil),
				"member-1": newService("member-1", string(naming.ScyllaServiceTypeMember), map[string]string{
					naming.HostIDAnnotation: "",
				}),
			},
			expectedHostIDs: []string{"a-host-id", "b-host-id"},
			expectedProgressingMessages: []string{
				`waiting for service "foo-ns/member-1" to have a hostID set`,
				`waiting for service "foo-ns/member-3" to have a hostID set`,
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Run it multiple times so that the map iteration order gets a chance to differ.
			for range 10 {
				hostIDs, progressingMessages := getMemberServiceHostIDs(tc.serviceMap)
				if !reflect.DeepEqual(hostIDs, tc.expectedHostIDs) {
					t.Errorf("expected hostIDs %v, got %v", tc.expectedHostIDs, hostIDs)
				}
				if !reflect.DeepEqual(progressingMessages, tc.expectedProgressingMessages) {
					t.Errorf("expected progressing messages %v, got %v", tc.expectedProgressingMessages, progressingMessages)
				}
			}
		})
	}
}
