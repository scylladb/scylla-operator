package resource

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/naming"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

func TestMemberService(t *testing.T) {
	basicSC := &scyllav1.ScyllaCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "basic",
			UID:  "the-uid",
		},
		Spec: scyllav1.ScyllaClusterSpec{
			Datacenter: scyllav1.DatacenterSpec{
				Name: "dc",
			},
		},
		Status: scyllav1.ScyllaClusterStatus{
			Racks: map[string]scyllav1.RackStatus{},
		},
	}
	basicSCOwnerRefs := []metav1.OwnerReference{
		{
			APIVersion:         "scylla.scylladb.com/v1",
			Kind:               "ScyllaCluster",
			Name:               "basic",
			UID:                "the-uid",
			Controller:         pointer.BoolPtr(true),
			BlockOwnerDeletion: pointer.BoolPtr(true),
		},
	}
	basicRackName := "rack"
	basicSVCName := "member"
	basicSVCSelector := map[string]string{
		"statefulset.kubernetes.io/pod-name": "member",
	}
	basicSVCLabels := func() map[string]string {
		return map[string]string{
			"app":                          "scylla",
			"app.kubernetes.io/name":       "scylla",
			"app.kubernetes.io/managed-by": "scylla-operator",
			"scylla/cluster":               "basic",
			"scylla/datacenter":            "dc",
			"scylla/rack":                  "rack",
		}
	}
	basicPorts := []corev1.ServicePort{
		{
			Name: "inter-node-communication",
			Port: 7000,
		},
		{
			Name: "ssl-inter-node-communication",
			Port: 7001,
		},
		{
			Name: "jmx-monitoring",
			Port: 7199,
		},
		{
			Name: "cql",
			Port: 9042,
		},
		{
			Name: "cql-ssl",
			Port: 9142,
		}, {
			Name: "agent-api",
			Port: 10001,
		},
		{
			Name: "thrift",
			Port: 9160,
		},
	}

	tt := []struct {
		name            string
		scyllaCLuster   *scyllav1.ScyllaCluster
		rackName        string
		svcName         string
		oldService      *corev1.Service
		expectedService *corev1.Service
	}{
		{
			name:          "new service",
			scyllaCLuster: basicSC,
			rackName:      basicRackName,
			svcName:       basicSVCName,
			oldService:    nil,
			expectedService: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:            basicSVCName,
					Labels:          basicSVCLabels(),
					OwnerReferences: basicSCOwnerRefs,
				},
				Spec: corev1.ServiceSpec{
					Type:                     corev1.ServiceTypeClusterIP,
					Selector:                 basicSVCSelector,
					PublishNotReadyAddresses: true,
					Ports:                    basicPorts,
				},
			},
		},
		{
			name: "new service with saved IP",
			scyllaCLuster: func() *scyllav1.ScyllaCluster {
				sc := basicSC.DeepCopy()
				sc.Status.Racks[basicRackName] = scyllav1.RackStatus{
					ReplaceAddressFirstBoot: map[string]string{
						basicSVCName: "10.0.0.1",
					},
				}
				return sc
			}(),
			rackName:   basicRackName,
			svcName:    basicSVCName,
			oldService: nil,
			expectedService: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: basicSVCName,
					Labels: func() map[string]string {
						labels := basicSVCLabels()
						labels[naming.ReplaceLabel] = "10.0.0.1"
						return labels
					}(),
					OwnerReferences: basicSCOwnerRefs,
				},
				Spec: corev1.ServiceSpec{
					Type:                     corev1.ServiceTypeClusterIP,
					Selector:                 basicSVCSelector,
					PublishNotReadyAddresses: true,
					Ports:                    basicPorts,
				},
			},
		},
		{
			name: "new service with saved IP and existing replace address",
			scyllaCLuster: func() *scyllav1.ScyllaCluster {
				sc := basicSC.DeepCopy()
				sc.Status.Racks[basicRackName] = scyllav1.RackStatus{
					ReplaceAddressFirstBoot: map[string]string{
						basicSVCName: "10.0.0.1",
					},
				}
				return sc
			}(),
			rackName: basicRackName,
			svcName:  basicSVCName,
			oldService: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						naming.ReplaceLabel: "10.0.0.1",
					},
				},
			},
			expectedService: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: basicSVCName,
					Labels: func() map[string]string {
						labels := basicSVCLabels()
						labels[naming.ReplaceLabel] = "10.0.0.1"
						return labels
					}(),
					OwnerReferences: basicSCOwnerRefs,
				},
				Spec: corev1.ServiceSpec{
					Type:                     corev1.ServiceTypeClusterIP,
					Selector:                 basicSVCSelector,
					PublishNotReadyAddresses: true,
					Ports:                    basicPorts,
				},
			},
		},
		{
			name:          "new service with unsaved IP and existing replace address",
			scyllaCLuster: basicSC,
			rackName:      basicRackName,
			svcName:       basicSVCName,
			oldService: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						naming.ReplaceLabel: "10.0.0.1",
					},
				},
			},
			expectedService: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: basicSVCName,
					Labels: func() map[string]string {
						labels := basicSVCLabels()
						labels[naming.ReplaceLabel] = "10.0.0.1"
						return labels
					}(),
					OwnerReferences: basicSCOwnerRefs,
				},
				Spec: corev1.ServiceSpec{
					Type:                     corev1.ServiceTypeClusterIP,
					Selector:                 basicSVCSelector,
					PublishNotReadyAddresses: true,
					Ports:                    basicPorts,
				},
			},
		},
		{
			name:          "existing initial service",
			scyllaCLuster: basicSC,
			rackName:      basicRackName,
			svcName:       basicSVCName,
			oldService: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						naming.ReplaceLabel: "",
					},
				},
			},
			expectedService: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: basicSVCName,
					Labels: func() map[string]string {
						labels := basicSVCLabels()
						labels[naming.ReplaceLabel] = ""
						return labels
					}(),
					OwnerReferences: basicSCOwnerRefs,
				},
				Spec: corev1.ServiceSpec{
					Type:                     corev1.ServiceTypeClusterIP,
					Selector:                 basicSVCSelector,
					PublishNotReadyAddresses: true,
					Ports:                    basicPorts,
				},
			},
		},
		{
			name: "existing initial service with IP",
			scyllaCLuster: func() *scyllav1.ScyllaCluster {
				sc := basicSC.DeepCopy()
				sc.Status.Racks[basicRackName] = scyllav1.RackStatus{
					ReplaceAddressFirstBoot: map[string]string{
						basicSVCName: "10.0.0.1",
					},
				}
				return sc
			}(),
			rackName: basicRackName,
			svcName:  basicSVCName,
			oldService: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						naming.ReplaceLabel: "",
					},
				},
			},
			expectedService: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: basicSVCName,
					Labels: func() map[string]string {
						labels := basicSVCLabels()
						labels[naming.ReplaceLabel] = ""
						return labels
					}(),
					OwnerReferences: basicSCOwnerRefs,
				},
				Spec: corev1.ServiceSpec{
					Type:                     corev1.ServiceTypeClusterIP,
					Selector:                 basicSVCSelector,
					PublishNotReadyAddresses: true,
					Ports:                    basicPorts,
				},
			},
		},
		{
			name:          "existing service",
			scyllaCLuster: basicSC,
			rackName:      basicRackName,
			svcName:       basicSVCName,
			oldService: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Labels: nil,
				},
			},
			expectedService: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:            basicSVCName,
					Labels:          basicSVCLabels(),
					OwnerReferences: basicSCOwnerRefs,
				},
				Spec: corev1.ServiceSpec{
					Type:                     corev1.ServiceTypeClusterIP,
					Selector:                 basicSVCSelector,
					PublishNotReadyAddresses: true,
					Ports:                    basicPorts,
				},
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			got := MemberService(tc.scyllaCLuster, tc.rackName, tc.svcName, tc.oldService)

			if !apiequality.Semantic.DeepEqual(got, tc.expectedService) {
				t.Errorf("expected and actual services differ: %s", cmp.Diff(tc.expectedService, got))
			}
		})
	}
}
