package scylladbdatacenter

import (
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/features"
	oslices "github.com/scylladb/scylla-operator/pkg/helpers/slices"
	"github.com/scylladb/scylla-operator/pkg/internalapi"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apimachineryutilintstr "k8s.io/apimachinery/pkg/util/intstr"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/component-base/featuregate"
	featuregatetesting "k8s.io/component-base/featuregate/testing"
)

func TestMemberService(t *testing.T) {
	basicSC := &scyllav1alpha1.ScyllaDBDatacenter{
		ObjectMeta: metav1.ObjectMeta{
			Name: "basic",
			UID:  "the-uid",
			Labels: map[string]string{
				"default-sc-label": "foo",
			},
			Annotations: map[string]string{
				"default-sc-annotation": "bar",
			},
		},
		Spec: scyllav1alpha1.ScyllaDBDatacenterSpec{
			DatacenterName: pointer.Ptr("dc"),
			ClusterName:    "basic",
			RackTemplate:   &scyllav1alpha1.RackTemplate{},
			Racks: []scyllav1alpha1.RackSpec{
				{
					Name: "rack",
				},
			},
		},
		Status: scyllav1alpha1.ScyllaDBDatacenterStatus{
			Racks: []scyllav1alpha1.RackStatus{},
		},
	}
	basicSCOwnerRefs := []metav1.OwnerReference{
		{
			APIVersion:         "scylla.scylladb.com/v1alpha1",
			Kind:               "ScyllaDBDatacenter",
			Name:               "basic",
			UID:                "the-uid",
			Controller:         pointer.Ptr(true),
			BlockOwnerDeletion: pointer.Ptr(true),
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
			"default-sc-label":             "foo",
			"scylla/cluster":               "basic",
			"scylla/datacenter":            "dc",
			"scylla/rack":                  "rack",
			"scylla-operator.scylladb.com/scylla-service-type": "member",
		}
	}
	basicSVCAnnotations := func() map[string]string {
		return map[string]string{
			"default-sc-annotation": "bar",
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
			Name: "cql",
			Port: 9042,
		},
		{
			Name: "cql-ssl",
			Port: 9142,
		},
		{
			Name: "cql-shard-aware",
			Port: 19042,
		},
		{
			Name: "cql-ssl-shard-aware",
			Port: 19142,
		},
		{
			Name: "jmx-monitoring",
			Port: 7199,
		},
		{
			Name: "agent-api",
			Port: 10001,
		},
		{
			Name: "prometheus",
			Port: 9180,
		},
		{
			Name: "agent-prometheus",
			Port: 5090,
		},
		{
			Name: "node-exporter",
			Port: 9100,
		},
		{
			Name: "thrift",
			Port: 9160,
		},
	}

	tt := []struct {
		name               string
		scyllaDBDatacenter *scyllav1alpha1.ScyllaDBDatacenter
		rackName           string
		svcName            string
		oldService         *corev1.Service
		jobs               map[string]*batchv1.Job
		expectedService    *corev1.Service
	}{
		{
			name:               "new service",
			scyllaDBDatacenter: basicSC,
			rackName:           basicRackName,
			svcName:            basicSVCName,
			oldService:         nil,
			jobs:               nil,
			expectedService: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:            basicSVCName,
					Labels:          basicSVCLabels(),
					Annotations:     basicSVCAnnotations(),
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
			name:               "existing service",
			scyllaDBDatacenter: basicSC,
			rackName:           basicRackName,
			svcName:            basicSVCName,
			oldService: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Labels: nil,
				},
			},
			jobs: nil,
			expectedService: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:            basicSVCName,
					Labels:          basicSVCLabels(),
					Annotations:     basicSVCAnnotations(),
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
			name:               "existing service with maintenance mode label, it is not carried over into required object - #1252",
			scyllaDBDatacenter: basicSC,
			rackName:           basicRackName,
			svcName:            basicSVCName,
			oldService: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						naming.NodeMaintenanceLabel: "42",
					},
				},
			},
			jobs: nil,
			expectedService: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:            basicSVCName,
					Labels:          basicSVCLabels(),
					Annotations:     basicSVCAnnotations(),
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
			name:               "last cleaned up annotation is rewritten from current one when it's missing in existing service",
			scyllaDBDatacenter: basicSC,
			rackName:           basicRackName,
			svcName:            basicSVCName,
			oldService: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"internal.scylla-operator.scylladb.com/current-token-ring-hash": "abc",
					},
				},
			}, jobs: nil,
			expectedService: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:   basicSVCName,
					Labels: basicSVCLabels(),
					Annotations: func() map[string]string {
						res := basicSVCAnnotations()
						res["internal.scylla-operator.scylladb.com/last-cleaned-up-token-ring-hash"] = "abc"
						return res
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
			name:               "last cleaned up annotation is added when cleanup job is completed",
			scyllaDBDatacenter: basicSC,
			rackName:           basicRackName,
			svcName:            basicSVCName,
			oldService:         nil,
			jobs: map[string]*batchv1.Job{
				"cleanup-member": {
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							"internal.scylla-operator.scylladb.com/cleanup-token-ring-hash": "abc",
						},
					},
					Status: batchv1.JobStatus{
						CompletionTime: pointer.Ptr(metav1.Now()),
					},
				},
			},
			expectedService: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:   basicSVCName,
					Labels: basicSVCLabels(),
					Annotations: func() map[string]string {
						res := basicSVCAnnotations()
						res["internal.scylla-operator.scylladb.com/last-cleaned-up-token-ring-hash"] = "abc"
						return res
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
			name: "Service properties are taken from ExposeOptions.NodeService",
			scyllaDBDatacenter: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sdc := basicSC.DeepCopy()
				sdc.Spec.ExposeOptions = &scyllav1alpha1.ExposeOptions{
					NodeService: &scyllav1alpha1.NodeServiceTemplate{
						ObjectTemplateMetadata: scyllav1alpha1.ObjectTemplateMetadata{
							Annotations: map[string]string{
								"foo": "bar",
							},
							Labels: map[string]string{
								"user-label": "user-label-value",
							},
						},
						Type:                          scyllav1alpha1.NodeServiceTypeLoadBalancer,
						ExternalTrafficPolicy:         pointer.Ptr(corev1.ServiceExternalTrafficPolicyLocal),
						AllocateLoadBalancerNodePorts: pointer.Ptr(true),
						LoadBalancerClass:             pointer.Ptr("my-lb-class"),
						InternalTrafficPolicy:         pointer.Ptr(corev1.ServiceInternalTrafficPolicyCluster),
					},
				}

				return sdc
			}(),
			rackName:   basicRackName,
			svcName:    basicSVCName,
			oldService: nil,
			jobs:       nil,
			expectedService: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: basicSVCName,
					Labels: map[string]string{
						"app":                          "scylla",
						"app.kubernetes.io/name":       "scylla",
						"app.kubernetes.io/managed-by": "scylla-operator",
						"user-label":                   "user-label-value",
						"scylla/cluster":               "basic",
						"scylla/datacenter":            "dc",
						"scylla/rack":                  "rack",
						"scylla-operator.scylladb.com/scylla-service-type": "member",
					},
					Annotations: map[string]string{
						"foo": "bar",
					},
					OwnerReferences: basicSCOwnerRefs,
				},
				Spec: corev1.ServiceSpec{
					Type:                          corev1.ServiceTypeLoadBalancer,
					Selector:                      basicSVCSelector,
					PublishNotReadyAddresses:      true,
					Ports:                         basicPorts,
					ExternalTrafficPolicy:         corev1.ServiceExternalTrafficPolicyLocal,
					AllocateLoadBalancerNodePorts: pointer.Ptr(true),
					LoadBalancerClass:             pointer.Ptr("my-lb-class"),
					InternalTrafficPolicy:         pointer.Ptr(corev1.ServiceInternalTrafficPolicyCluster),
				},
			},
		},
		{
			name: "headless service type in node service template",
			scyllaDBDatacenter: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sc := basicSC.DeepCopy()
				sc.Spec.ExposeOptions = &scyllav1alpha1.ExposeOptions{
					NodeService: &scyllav1alpha1.NodeServiceTemplate{
						Type: scyllav1alpha1.NodeServiceTypeHeadless,
					},
				}

				return sc
			}(),
			rackName:   basicRackName,
			svcName:    basicSVCName,
			oldService: nil,
			jobs:       nil,
			expectedService: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:            basicSVCName,
					Labels:          basicSVCLabels(),
					Annotations:     basicSVCAnnotations(),
					OwnerReferences: basicSCOwnerRefs,
				},
				Spec: corev1.ServiceSpec{
					Type:                     corev1.ServiceTypeClusterIP,
					Selector:                 basicSVCSelector,
					PublishNotReadyAddresses: true,
					Ports:                    basicPorts,
					ClusterIP:                corev1.ClusterIPNone,
				},
			},
		},
		{
			name: "ClusterIP service type in node service template",
			scyllaDBDatacenter: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sc := basicSC.DeepCopy()
				sc.Spec.ExposeOptions = &scyllav1alpha1.ExposeOptions{
					NodeService: &scyllav1alpha1.NodeServiceTemplate{
						Type: scyllav1alpha1.NodeServiceTypeClusterIP,
					},
				}

				return sc
			}(),
			rackName:   basicRackName,
			svcName:    basicSVCName,
			oldService: nil,
			jobs:       nil,
			expectedService: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:            basicSVCName,
					Labels:          basicSVCLabels(),
					Annotations:     basicSVCAnnotations(),
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
			name: "LoadBalancer service type in node service template",
			scyllaDBDatacenter: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sc := basicSC.DeepCopy()
				sc.Spec.ExposeOptions = &scyllav1alpha1.ExposeOptions{
					NodeService: &scyllav1alpha1.NodeServiceTemplate{
						Type: scyllav1alpha1.NodeServiceTypeLoadBalancer,
					},
				}

				return sc
			}(),
			rackName:   basicRackName,
			svcName:    basicSVCName,
			oldService: nil,
			jobs:       nil,
			expectedService: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:            basicSVCName,
					Labels:          basicSVCLabels(),
					Annotations:     basicSVCAnnotations(),
					OwnerReferences: basicSCOwnerRefs,
				},
				Spec: corev1.ServiceSpec{
					Type:                     corev1.ServiceTypeLoadBalancer,
					Selector:                 basicSVCSelector,
					PublishNotReadyAddresses: true,
					Ports:                    basicPorts,
				},
			},
		},
		{
			name: "rack Service metadata in rack template and rack expose options",
			scyllaDBDatacenter: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sc := basicSC.DeepCopy()
				sc.Spec.RackTemplate.ExposeOptions = &scyllav1alpha1.RackExposeOptions{
					NodeService: &scyllav1alpha1.RackNodeServiceTemplate{
						ObjectTemplateMetadata: scyllav1alpha1.ObjectTemplateMetadata{
							Labels: map[string]string{
								"custom-rack-template-service-label": "custom-rack-template-service-label-value",
							},
							Annotations: map[string]string{
								"custom-rack-template-service-annotation": "custom-rack-template-service-annotation-value",
							},
						},
					},
				}
				sc.Spec.Racks[0].ExposeOptions = &scyllav1alpha1.RackExposeOptions{
					NodeService: &scyllav1alpha1.RackNodeServiceTemplate{
						ObjectTemplateMetadata: scyllav1alpha1.ObjectTemplateMetadata{
							Labels: map[string]string{
								"custom-rack-service-label": "custom-rack-service-label-value",
							},
							Annotations: map[string]string{
								"custom-rack-service-annotation": "custom-rack-service-annotation-value",
							},
						},
					},
				}

				return sc
			}(),
			rackName:   basicRackName,
			svcName:    basicSVCName,
			oldService: nil,
			jobs:       nil,
			expectedService: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: basicSVCName,
					Labels: func() map[string]string {
						labels := basicSVCLabels()
						labels["custom-rack-template-service-label"] = "custom-rack-template-service-label-value"
						labels["custom-rack-service-label"] = "custom-rack-service-label-value"
						return labels
					}(),
					Annotations: func() map[string]string {
						annotations := basicSVCAnnotations()
						annotations["custom-rack-template-service-annotation"] = "custom-rack-template-service-annotation-value"
						annotations["custom-rack-service-annotation"] = "custom-rack-service-annotation-value"
						return annotations
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
			name: "rack spec Service metadata overrides one specified on rack template on collisions",
			scyllaDBDatacenter: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sc := basicSC.DeepCopy()
				sc.Spec.RackTemplate.ExposeOptions = &scyllav1alpha1.RackExposeOptions{
					NodeService: &scyllav1alpha1.RackNodeServiceTemplate{
						ObjectTemplateMetadata: scyllav1alpha1.ObjectTemplateMetadata{
							Labels: map[string]string{
								"custom-rack-service-label": "foo",
							},
							Annotations: map[string]string{
								"custom-rack-service-annotation": "foo",
							},
						},
					},
				}
				sc.Spec.Racks[0].ExposeOptions = &scyllav1alpha1.RackExposeOptions{
					NodeService: &scyllav1alpha1.RackNodeServiceTemplate{
						ObjectTemplateMetadata: scyllav1alpha1.ObjectTemplateMetadata{
							Labels: map[string]string{
								"custom-rack-service-label": "bar",
							},
							Annotations: map[string]string{
								"custom-rack-service-annotation": "bar",
							},
						},
					},
				}

				return sc
			}(),
			rackName:   basicRackName,
			svcName:    basicSVCName,
			oldService: nil,
			jobs:       nil,
			expectedService: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: basicSVCName,
					Labels: func() map[string]string {
						labels := basicSVCLabels()
						labels["custom-rack-service-label"] = "bar"
						return labels
					}(),
					Annotations: func() map[string]string {
						annotations := basicSVCAnnotations()
						annotations["custom-rack-service-annotation"] = "bar"
						return annotations
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
			name: "non-propagated annotations are not propagated",
			scyllaDBDatacenter: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sdc := basicSC.DeepCopy()
				if len(nonPropagatedAnnotationKeys) == 0 {
					panic("nonPropagatedAnnotationKeys can't be empty")
				}
				for _, k := range nonPropagatedAnnotationKeys {
					metav1.SetMetaDataAnnotation(&sdc.ObjectMeta, k, "")
				}
				return sdc
			}(),
			rackName:   basicRackName,
			svcName:    basicSVCName,
			oldService: nil,
			jobs:       nil,
			expectedService: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:            basicSVCName,
					Labels:          basicSVCLabels(),
					Annotations:     basicSVCAnnotations(),
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
			name: "non-propagated labels are not propagated",
			scyllaDBDatacenter: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sdc := basicSC.DeepCopy()
				if len(nonPropagatedLabelKeys) == 0 {
					panic("nonPropagatedLabelKeys can't be empty")
				}
				for _, k := range nonPropagatedLabelKeys {
					metav1.SetMetaDataLabel(&sdc.ObjectMeta, k, "")
				}
				return sdc
			}(),
			rackName:   basicRackName,
			svcName:    basicSVCName,
			oldService: nil,
			jobs:       nil,
			expectedService: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:            basicSVCName,
					Labels:          basicSVCLabels(),
					Annotations:     basicSVCAnnotations(),
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
			got, err := MemberService(tc.scyllaDBDatacenter, tc.rackName, tc.svcName, tc.oldService, tc.jobs)
			if err != nil {
				t.Fatal(err)
			}

			if !apiequality.Semantic.DeepEqual(got, tc.expectedService) {
				t.Errorf("expected and actual services differ: %s", cmp.Diff(tc.expectedService, got))
			}
		})
	}
}

func runTestStatefulSetForRack(t *testing.T) {
	logEnabledFeatures(t)

	newBasicRack := func() scyllav1alpha1.RackSpec {
		return scyllav1alpha1.RackSpec{
			Name: "rack",
			RackTemplate: scyllav1alpha1.RackTemplate{
				ScyllaDB: &scyllav1alpha1.ScyllaDBTemplate{
					Storage: &scyllav1alpha1.StorageOptions{
						Capacity: "1Gi",
					},
				},
			},
		}
	}

	newBasicScyllaDBDatacenter := func() *scyllav1alpha1.ScyllaDBDatacenter {
		return &scyllav1alpha1.ScyllaDBDatacenter{
			ObjectMeta: metav1.ObjectMeta{
				Name: "basic",
				UID:  "the-uid",
				Labels: map[string]string{
					"default-sc-label": "foo",
				},
				Annotations: map[string]string{
					"default-sc-annotation": "bar",
				},
			},
			Spec: scyllav1alpha1.ScyllaDBDatacenterSpec{
				ClusterName:    "basic",
				DatacenterName: pointer.Ptr("dc"),
				ScyllaDB: scyllav1alpha1.ScyllaDB{
					Image: "scylladb/scylla:latest",
				},
				ScyllaDBManagerAgent: &scyllav1alpha1.ScyllaDBManagerAgent{
					Image: pointer.Ptr("scylladb/scylla-manager-agent:latest"),
				},
				Racks: []scyllav1alpha1.RackSpec{
					newBasicRack(),
				},
			},
			Status: scyllav1alpha1.ScyllaDBDatacenterStatus{
				Racks: []scyllav1alpha1.RackStatus{},
			},
		}
	}

	newBasicStatefulSetLabels := func(ordinal int) map[string]string {
		return map[string]string{
			"app":                                   "scylla",
			"app.kubernetes.io/managed-by":          "scylla-operator",
			"app.kubernetes.io/name":                "scylla",
			"default-sc-label":                      "foo",
			"scylla/cluster":                        "basic",
			"scylla/datacenter":                     "dc",
			"scylla/rack":                           "rack",
			"scylla/scylla-version":                 "latest",
			"scylla/rack-ordinal":                   fmt.Sprintf("%d", ordinal),
			"scylla-operator.scylladb.com/pod-type": "scylladb-node",
		}
	}

	newBasicStatefulSet := func() *appsv1.StatefulSet {
		return &appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "basic-dc-rack",
				Labels: newBasicStatefulSetLabels(0),
				Annotations: map[string]string{
					"default-sc-annotation": "bar",
				},
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion:         "scylla.scylladb.com/v1alpha1",
						Kind:               "ScyllaDBDatacenter",
						Name:               "basic",
						UID:                "the-uid",
						Controller:         pointer.Ptr(true),
						BlockOwnerDeletion: pointer.Ptr(true),
					},
				},
			},
			Spec: appsv1.StatefulSetSpec{
				Replicas: pointer.Ptr(int32(0)),
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app":                          "scylla",
						"app.kubernetes.io/managed-by": "scylla-operator",
						"app.kubernetes.io/name":       "scylla",
						"scylla/cluster":               "basic",
						"scylla/datacenter":            "dc",
						"scylla/rack":                  "rack",
					},
				},
				MinReadySeconds: 5,
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: newBasicStatefulSetLabels(0),
						Annotations: map[string]string{
							"default-sc-annotation":                    "bar",
							"scylla-operator.scylladb.com/inputs-hash": "",
							"prometheus.io/port":                       "9180",
							"prometheus.io/scrape":                     "true",
						},
					},
					Spec: corev1.PodSpec{
						SecurityContext: &corev1.PodSecurityContext{
							RunAsUser:  pointer.Ptr(int64(0)),
							RunAsGroup: pointer.Ptr(int64(0)),
						},
						Volumes: func() []corev1.Volume {
							volumes := []corev1.Volume{
								{
									Name: "shared",
									VolumeSource: corev1.VolumeSource{
										EmptyDir: &corev1.EmptyDirVolumeSource{},
									},
								},
								{
									Name: "scylla-config-volume",
									VolumeSource: corev1.VolumeSource{
										ConfigMap: &corev1.ConfigMapVolumeSource{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "scylla-config",
											},
											Optional: pointer.Ptr(true),
										},
									},
								},
								{
									Name: "scylladb-managed-config",
									VolumeSource: corev1.VolumeSource{
										ConfigMap: &corev1.ConfigMapVolumeSource{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "basic-managed-config",
											},
											Optional: pointer.Ptr(false),
										},
									},
								},
								{
									Name: "scylladb-snitch-config",
									VolumeSource: corev1.VolumeSource{
										ConfigMap: &corev1.ConfigMapVolumeSource{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "basic-rack-snitch-config",
											},
											Optional: pointer.Ptr(false),
										},
									},
								},
								{
									Name: "scylla-managed-agent-config-volume",
									VolumeSource: corev1.VolumeSource{
										ConfigMap: &corev1.ConfigMapVolumeSource{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "basic-manager-agent-config",
											},
											Optional: pointer.Ptr(false),
										},
									},
								},
								{
									Name: "scylla-agent-config-volume",
									VolumeSource: corev1.VolumeSource{
										Secret: &corev1.SecretVolumeSource{
											SecretName: "scylla-agent-config-secret",
											Optional:   pointer.Ptr(true),
										},
									},
								},
								{
									Name: "scylla-client-config-volume",
									VolumeSource: corev1.VolumeSource{
										Secret: &corev1.SecretVolumeSource{
											SecretName: "scylla-client-config-secret",
											Optional:   pointer.Ptr(true),
										},
									},
								},
								{
									Name: "scylla-agent-auth-token-volume",
									VolumeSource: corev1.VolumeSource{
										Secret: &corev1.SecretVolumeSource{
											SecretName: "basic-auth-token",
										},
									},
								},
							}

							if utilfeature.DefaultMutableFeatureGate.Enabled(features.AutomaticTLSCertificates) {
								volumes = append(volumes, []corev1.Volume{
									{
										Name: "scylladb-serving-certs",
										VolumeSource: corev1.VolumeSource{
											Secret: &corev1.SecretVolumeSource{
												SecretName: "basic-local-serving-certs",
											},
										},
									},
									{
										Name: "scylladb-client-ca",
										VolumeSource: corev1.VolumeSource{
											ConfigMap: &corev1.ConfigMapVolumeSource{
												LocalObjectReference: corev1.LocalObjectReference{
													Name: "basic-local-client-ca",
												},
											},
										},
									},
									{
										Name: "scylladb-user-admin",
										VolumeSource: corev1.VolumeSource{
											Secret: &corev1.SecretVolumeSource{
												SecretName: "basic-local-user-admin",
											},
										},
									},
								}...)
							}

							return volumes
						}(),
						InitContainers: func() []corev1.Container {
							initContainers := []corev1.Container{
								{
									Name:            "sidecar-injection",
									ImagePullPolicy: "IfNotPresent",
									Image:           "scylladb/scylla-operator:latest",
									Command: []string{
										"/bin/sh",
										"-c",
										"cp -a /usr/bin/scylla-operator '/mnt/shared'",
									},
									Resources: corev1.ResourceRequirements{
										Limits: corev1.ResourceList{
											corev1.ResourceCPU:    resource.MustParse("10m"),
											corev1.ResourceMemory: resource.MustParse("50Mi"),
										},
										Requests: corev1.ResourceList{
											corev1.ResourceCPU:    resource.MustParse("10m"),
											corev1.ResourceMemory: resource.MustParse("50Mi"),
										},
									},
									VolumeMounts: []corev1.VolumeMount{
										{
											Name:      "shared",
											MountPath: "/mnt/shared",
											ReadOnly:  false,
										},
									},
								},
							}

							if utilfeature.DefaultMutableFeatureGate.Enabled(features.BootstrapSynchronisation) {
								initContainers = append(initContainers, []corev1.Container{
									{
										Name:            "scylladb-bootstrap-barrier",
										Image:           "scylladb/scylla:latest",
										ImagePullPolicy: "IfNotPresent",
										Command: []string{
											"/mnt/shared/scylla-operator",
											"run-bootstrap-barrier",
											"--service-name=$(SERVICE_NAME)",
											"--scylla-data-dir=/var/lib/scylla/data",
											"--selector-label-value=basic",
											"--single-report-allow-non-reporting-host-ids=false",
											"--loglevel=0",
										},
										Resources: corev1.ResourceRequirements{
											Limits: corev1.ResourceList{
												corev1.ResourceCPU:    resource.MustParse("50m"),
												corev1.ResourceMemory: resource.MustParse("100Mi"),
											},
											Requests: corev1.ResourceList{
												corev1.ResourceCPU:    resource.MustParse("50m"),
												corev1.ResourceMemory: resource.MustParse("100Mi"),
											},
										},
										VolumeMounts: []corev1.VolumeMount{
											{
												Name:      "data",
												MountPath: "/var/lib/scylla",
												ReadOnly:  true,
											},
											{
												Name:      "shared",
												MountPath: "/mnt/shared",
												ReadOnly:  true,
											},
										},
										Env: []corev1.EnvVar{
											{
												Name: "SERVICE_NAME",
												ValueFrom: &corev1.EnvVarSource{
													FieldRef: &corev1.ObjectFieldSelector{
														FieldPath: "metadata.name",
													},
												},
											},
										},
									},
								}...)
							}

							return initContainers
						}(),
						Containers: []corev1.Container{
							{
								Name:            "scylla",
								Image:           "scylladb/scylla:latest",
								ImagePullPolicy: corev1.PullIfNotPresent,
								Ports: []corev1.ContainerPort{
									{
										Name:          "intra-node",
										ContainerPort: 7000,
									},
									{
										Name:          "tls-intra-node",
										ContainerPort: 7001,
									},
									{
										Name:          "cql",
										ContainerPort: 9042,
									},
									{
										Name:          "cql-ssl",
										ContainerPort: 9142,
									},
									{
										Name:          "jmx",
										ContainerPort: 7199,
									},
									{
										Name:          "prometheus",
										ContainerPort: 9180,
									},
									{
										Name:          "node-exporter",
										ContainerPort: 9100,
									},
									{
										Name:          "thrift",
										ContainerPort: 9160,
									},
								},
								Command: func() []string {
									return []string{
										"/usr/bin/bash",
										"-euEo",
										"pipefail",
										"-O",
										"inherit_errexit",
										"-c",
										strings.TrimSpace(`
trap 'kill $( jobs -p ); exit 0' TERM

printf 'INFO %s ignition - Waiting for /mnt/shared/ignition.done\n' "$( date '+%Y-%m-%d %H:%M:%S,%3N' )" > /dev/stderr
until [[ -f "/mnt/shared/ignition.done" ]]; do
  sleep 1 &
  wait
done
printf 'INFO %s ignition - Ignited. Starting ScyllaDB...\n' "$( date '+%Y-%m-%d %H:%M:%S,%3N' )" > /dev/stderr

# TODO: This is where we should start ScyllaDB directly after the sidecar split #1942 
exec /mnt/shared/scylla-operator sidecar \
--feature-gates=` + func() string {
											featureGates := []string{"AllAlpha=false", "AllBeta=false"}

											featureGates = append(featureGates, fmt.Sprintf("AutomaticTLSCertificates=%t", utilfeature.DefaultMutableFeatureGate.Enabled(features.AutomaticTLSCertificates)))
											featureGates = append(featureGates, fmt.Sprintf("BootstrapSynchronisation=%t", utilfeature.DefaultMutableFeatureGate.Enabled(features.BootstrapSynchronisation)))

											return strings.Join(featureGates, ",")
										}() + ` \
--nodes-broadcast-address-type=ServiceClusterIP \
--clients-broadcast-address-type=ServiceClusterIP \
--service-name=$(SERVICE_NAME) \
--cpu-count=$(CPU_COUNT) \
--scylla-localhost-address=127.0.0.1 \
--ip-family=IPv4 \
--loglevel=0 \
 -- "$@"
`),
										"--",
										"--developer-mode=0",
									}
								}(),
								Env: []corev1.EnvVar{
									{
										Name: "SERVICE_NAME",
										ValueFrom: &corev1.EnvVarSource{
											FieldRef: &corev1.ObjectFieldSelector{
												FieldPath: "metadata.name",
											},
										},
									},
									{
										Name: "CPU_COUNT",
										ValueFrom: &corev1.EnvVarSource{
											ResourceFieldRef: &corev1.ResourceFieldSelector{
												ContainerName: "scylla",
												Resource:      "limits.cpu",
												Divisor:       resource.MustParse("1"),
											},
										},
									},
								},
								Resources: func() corev1.ResourceRequirements {
									rack := newBasicRack()
									if rack.ScyllaDB != nil && rack.ScyllaDB.Resources != nil {
										return *rack.ScyllaDB.Resources
									}
									return corev1.ResourceRequirements{}
								}(),
								VolumeMounts: func() []corev1.VolumeMount {
									mounts := []corev1.VolumeMount{
										{
											Name:      "data",
											MountPath: "/var/lib/scylla",
										},
										{
											Name:      "shared",
											MountPath: "/mnt/shared",
										},
										{
											Name:      "scylla-config-volume",
											MountPath: "/mnt/scylla-config",
											ReadOnly:  true,
										},
										{
											Name:      "scylladb-managed-config",
											MountPath: "/var/run/configmaps/scylla-operator.scylladb.com/scylladb/managed-config",
											ReadOnly:  true,
										},
										{
											Name:      "scylladb-snitch-config",
											MountPath: "/var/run/configmaps/scylla-operator.scylladb.com/scylladb/snitch-config",
											ReadOnly:  true,
										},
										{
											Name:      "scylla-client-config-volume",
											MountPath: "/mnt/scylla-client-config",
											ReadOnly:  true,
										},
									}

									if utilfeature.DefaultMutableFeatureGate.Enabled(features.AutomaticTLSCertificates) {
										mounts = append(mounts, []corev1.VolumeMount{
											{
												Name:      "scylladb-serving-certs",
												MountPath: "/var/run/secrets/scylla-operator.scylladb.com/scylladb/serving-certs",
												ReadOnly:  true,
											},
											{
												Name:      "scylladb-client-ca",
												MountPath: "/var/run/configmaps/scylla-operator.scylladb.com/scylladb/client-ca",
												ReadOnly:  true,
											},
											{
												Name:      "scylladb-user-admin",
												MountPath: "/var/run/secrets/scylla-operator.scylladb.com/scylladb/user-admin",
												ReadOnly:  true,
											},
										}...)
									}

									return mounts
								}(),
								SecurityContext: &corev1.SecurityContext{
									RunAsUser:  pointer.Ptr(int64(0)),
									RunAsGroup: pointer.Ptr(int64(0)),
									Capabilities: &corev1.Capabilities{
										Add: []corev1.Capability{"SYS_NICE"},
									},
								},
								StartupProbe: &corev1.Probe{
									TimeoutSeconds:   int32(30),
									FailureThreshold: int32(40),
									PeriodSeconds:    int32(10),
									ProbeHandler: corev1.ProbeHandler{
										HTTPGet: &corev1.HTTPGetAction{
											Port: apimachineryutilintstr.FromInt(8080),
											Path: "/healthz",
										},
									},
								},
								LivenessProbe: &corev1.Probe{
									TimeoutSeconds:   int32(10),
									FailureThreshold: int32(12),
									PeriodSeconds:    int32(10),
									ProbeHandler: corev1.ProbeHandler{
										HTTPGet: &corev1.HTTPGetAction{
											Port: apimachineryutilintstr.FromInt(8080),
											Path: "/healthz",
										},
									},
								},
								ReadinessProbe: &corev1.Probe{
									TimeoutSeconds:   int32(30),
									FailureThreshold: int32(1),
									PeriodSeconds:    int32(10),
									ProbeHandler: corev1.ProbeHandler{
										HTTPGet: &corev1.HTTPGetAction{
											Port: apimachineryutilintstr.FromInt(8080),
											Path: "/readyz",
										},
									},
								},
								Lifecycle: &corev1.Lifecycle{
									PreStop: &corev1.LifecycleHandler{
										Exec: &corev1.ExecAction{
											Command: []string{
												"/usr/bin/bash",
												"-euExo",
												"pipefail",
												"-O",
												"inherit_errexit",
												"-c",
												strings.TrimSpace(`
trap 'kill $( jobs -p ); exit 0' TERM
trap 'rm -f /mnt/shared/ignition.done' EXIT

nodetool drain &
sleep 15 &
wait`),
											},
										},
									},
								},
							},
							{
								Name:            "scylladb-api-status-probe",
								Image:           "scylladb/scylla-operator:latest",
								ImagePullPolicy: corev1.PullIfNotPresent,
								Command: []string{
									"/usr/bin/scylla-operator",
									"serve-probes",
									"scylladb-api-status",
									"--port=8080",
									"--service-name=$(SERVICE_NAME)",
									"--scylla-localhost-address=127.0.0.1",
									"--loglevel=0",
								},
								Env: []corev1.EnvVar{
									{
										Name: "SERVICE_NAME",
										ValueFrom: &corev1.EnvVarSource{
											FieldRef: &corev1.ObjectFieldSelector{
												FieldPath: "metadata.name",
											},
										},
									},
								},
								ReadinessProbe: &corev1.Probe{
									TimeoutSeconds:   int32(30),
									FailureThreshold: int32(1),
									PeriodSeconds:    int32(5),
									ProbeHandler: corev1.ProbeHandler{
										TCPSocket: &corev1.TCPSocketAction{
											Port: apimachineryutilintstr.FromInt32(naming.ScyllaDBAPIStatusProbePort),
										},
									},
								},
								Resources: corev1.ResourceRequirements{
									Limits: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("10m"),
										corev1.ResourceMemory: resource.MustParse("40Mi"),
									},
									Requests: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("10m"),
										corev1.ResourceMemory: resource.MustParse("40Mi"),
									},
								},
							},
							{
								Name:            "scylladb-ignition",
								Image:           "scylladb/scylla-operator:latest",
								ImagePullPolicy: corev1.PullIfNotPresent,
								Command: []string{
									"/usr/bin/scylla-operator",
									"run-ignition",
									"--service-name=$(SERVICE_NAME)",
									"--nodes-broadcast-address-type=ServiceClusterIP",
									"--clients-broadcast-address-type=ServiceClusterIP",
									"--loglevel=0",
								},
								Env: []corev1.EnvVar{
									{
										Name: "SERVICE_NAME",
										ValueFrom: &corev1.EnvVarSource{
											FieldRef: &corev1.ObjectFieldSelector{
												FieldPath: "metadata.name",
											},
										},
									},
								},
								ReadinessProbe: &corev1.Probe{
									TimeoutSeconds:   int32(30),
									FailureThreshold: int32(1),
									PeriodSeconds:    int32(5),
									ProbeHandler: corev1.ProbeHandler{
										HTTPGet: &corev1.HTTPGetAction{
											Port: apimachineryutilintstr.FromInt32(42081),
											Path: "/readyz",
										},
									},
								},
								Resources: corev1.ResourceRequirements{
									Limits: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("10m"),
										corev1.ResourceMemory: resource.MustParse("40Mi"),
									},
									Requests: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("10m"),
										corev1.ResourceMemory: resource.MustParse("40Mi"),
									},
								},
								VolumeMounts: []corev1.VolumeMount{
									{
										Name:      "shared",
										MountPath: "/mnt/shared",
										ReadOnly:  false,
									},
								},
							},
							{
								Name:            "scylla-manager-agent",
								Image:           "scylladb/scylla-manager-agent:latest",
								ImagePullPolicy: corev1.PullIfNotPresent,
								Command: []string{
									"/usr/bin/bash",
									"-euEo",
									"pipefail",
									"-O",
									"inherit_errexit",
									"-c",
									strings.TrimSpace(`
trap 'kill $( jobs -p ); exit 0' TERM

printf '{"L":"INFO","T":"%s","M":"Waiting for /mnt/shared/ignition.done"}\n' "$( date -u '+%Y-%m-%dT%H:%M:%S,%3NZ' )" > /dev/stderr
until [[ -f "/mnt/shared/ignition.done" ]]; do
  sleep 1 &
  wait
done
printf '{"L":"INFO","T":"%s","M":"Ignited. Starting ScyllaDB Manager Agent"}\n' "$( date -u '+%Y-%m-%dT%H:%M:%S,%3NZ' )" > /dev/stderr

exec scylla-manager-agent \
-c "/etc/scylla-manager-agent/scylla-manager-agent.yaml" \
-c "/mnt/scylla-managed-agent-config/scylla-manager-agent.yaml" \
-c "/mnt/scylla-agent-config/scylla-manager-agent.yaml" \
-c "/mnt/scylla-agent-config/auth-token.yaml"
`),
								},
								Ports: []corev1.ContainerPort{
									{
										Name:          "agent-rest-api",
										ContainerPort: 10001,
									},
								},
								ReadinessProbe: &corev1.Probe{
									ProbeHandler: corev1.ProbeHandler{
										TCPSocket: &corev1.TCPSocketAction{
											Port: apimachineryutilintstr.FromInt32(10001),
										},
									},
								},
								VolumeMounts: []corev1.VolumeMount{
									{
										Name:      "data",
										MountPath: "/var/lib/scylla",
									},
									{
										Name:      "scylla-managed-agent-config-volume",
										MountPath: "/mnt/scylla-managed-agent-config/scylla-manager-agent.yaml",
										SubPath:   "scylla-manager-agent.yaml",
										ReadOnly:  true,
									},
									{
										Name:      "scylla-agent-config-volume",
										MountPath: "/mnt/scylla-agent-config/scylla-manager-agent.yaml",
										SubPath:   "scylla-manager-agent.yaml",
										ReadOnly:  true,
									},
									{
										Name:      "scylla-agent-auth-token-volume",
										MountPath: "/mnt/scylla-agent-config/auth-token.yaml",
										SubPath:   "auth-token.yaml",
										ReadOnly:  true,
									},
									{
										Name:      "shared",
										MountPath: "/mnt/shared",
										ReadOnly:  true,
									},
								},
								Resources: func() corev1.ResourceRequirements {
									rack := newBasicRack()
									if rack.ScyllaDBManagerAgent != nil && rack.ScyllaDBManagerAgent.Resources != nil {
										return *rack.ScyllaDBManagerAgent.Resources
									}
									return corev1.ResourceRequirements{}
								}(),
							},
						},
						DNSPolicy:                     "ClusterFirstWithHostNet",
						ServiceAccountName:            "basic-member",
						Affinity:                      &corev1.Affinity{},
						TerminationGracePeriodSeconds: pointer.Ptr(int64(900)),
					},
				},
				VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "data",
							Labels: map[string]string{
								"app":                          "scylla",
								"app.kubernetes.io/managed-by": "scylla-operator",
								"app.kubernetes.io/name":       "scylla",
								"scylla/cluster":               "basic",
								"scylla/datacenter":            "dc",
								"scylla/rack":                  "rack",
								"default-sc-label":             "foo",
							},
							Annotations: map[string]string{
								"default-sc-annotation": "bar",
							},
						},
						Spec: corev1.PersistentVolumeClaimSpec{
							AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
							StorageClassName: newBasicRack().ScyllaDB.Storage.StorageClassName,
							Resources: corev1.VolumeResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceStorage: resource.MustParse(newBasicRack().ScyllaDB.Storage.Capacity),
								},
							},
						},
					},
				},
				ServiceName:         "basic-client",
				PodManagementPolicy: appsv1.OrderedReadyPodManagement,
				UpdateStrategy: appsv1.StatefulSetUpdateStrategy{
					Type: appsv1.RollingUpdateStatefulSetStrategyType,
					RollingUpdate: &appsv1.RollingUpdateStatefulSetStrategy{
						Partition: pointer.Ptr(int32(0)),
					},
				},
			},
		}
	}

	newNodeAffinity := func() *corev1.NodeAffinity {
		return &corev1.NodeAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
				NodeSelectorTerms: []corev1.NodeSelectorTerm{
					{
						MatchExpressions: []corev1.NodeSelectorRequirement{
							{
								Key:      "key",
								Operator: corev1.NodeSelectorOpIn,
								Values:   []string{"value"},
							},
						},
					},
				},
			},
		}
	}

	newPodAffinity := func() *corev1.PodAffinity {
		return &corev1.PodAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
				{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"key": "value",
						},
					},
				},
			},
		}
	}

	newPodAntiAffinity := func() *corev1.PodAntiAffinity {
		return &corev1.PodAntiAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
				{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"key": "value",
						},
					},
				},
			},
		}
	}

	const scyllaContainerIndex = 0
	const runBootstrapBarrierInitContainerIndex = 1

	tt := []struct {
		name                string
		rack                scyllav1alpha1.RackSpec
		scyllaDBDatacenter  *scyllav1alpha1.ScyllaDBDatacenter
		existingStatefulSet *appsv1.StatefulSet
		expectedStatefulSet *appsv1.StatefulSet
		expectedError       error
	}{
		{
			name:                "new StatefulSet",
			rack:                newBasicRack(),
			scyllaDBDatacenter:  newBasicScyllaDBDatacenter(),
			existingStatefulSet: nil,
			expectedStatefulSet: newBasicStatefulSet(),
			expectedError:       nil,
		},
		{
			name: "error for invalid Rack storage",
			rack: func() scyllav1alpha1.RackSpec {
				r := newBasicRack()
				r.ScyllaDB = &scyllav1alpha1.ScyllaDBTemplate{
					Storage: &scyllav1alpha1.StorageOptions{
						Capacity: "",
					},
				}
				return r
			}(),
			scyllaDBDatacenter:  newBasicScyllaDBDatacenter(),
			existingStatefulSet: nil,
			expectedStatefulSet: nil,
			expectedError:       fmt.Errorf(`cannot parse storage capacity "": %v`, resource.ErrFormatWrong),
		},
		{
			name: "new StatefulSet with non-nil Tolerations",
			rack: func() scyllav1alpha1.RackSpec {
				r := newBasicRack()
				r.Placement = &scyllav1alpha1.Placement{
					Tolerations: []corev1.Toleration{
						{
							Key:      "key",
							Operator: corev1.TolerationOpEqual,
							Value:    "value",
							Effect:   corev1.TaintEffectNoSchedule,
						},
					},
				}
				return r
			}(),
			scyllaDBDatacenter:  newBasicScyllaDBDatacenter(),
			existingStatefulSet: nil,
			expectedStatefulSet: func() *appsv1.StatefulSet {
				s := newBasicStatefulSet()
				s.Spec.Template.Spec.Tolerations = []corev1.Toleration{
					{
						Key:      "key",
						Operator: corev1.TolerationOpEqual,
						Value:    "value",
						Effect:   corev1.TaintEffectNoSchedule,
					},
				}
				return s
			}(),
			expectedError: nil,
		},
		{
			name: "new StatefulSet with non-nil NodeAffinity",
			rack: func() scyllav1alpha1.RackSpec {
				r := newBasicRack()
				r.Placement = &scyllav1alpha1.Placement{
					NodeAffinity: newNodeAffinity(),
				}
				return r
			}(),
			scyllaDBDatacenter:  newBasicScyllaDBDatacenter(),
			existingStatefulSet: nil,
			expectedStatefulSet: func() *appsv1.StatefulSet {
				s := newBasicStatefulSet()
				s.Spec.Template.Spec.Affinity.NodeAffinity = newNodeAffinity()
				return s
			}(),
			expectedError: nil,
		},
		{
			name: "new StatefulSet with non-nil PodAffinity",
			rack: func() scyllav1alpha1.RackSpec {
				r := newBasicRack()
				r.Placement = &scyllav1alpha1.Placement{
					PodAffinity: newPodAffinity(),
				}
				return r
			}(),
			scyllaDBDatacenter:  newBasicScyllaDBDatacenter(),
			existingStatefulSet: nil,
			expectedStatefulSet: func() *appsv1.StatefulSet {
				s := newBasicStatefulSet()
				s.Spec.Template.Spec.Affinity.PodAffinity = newPodAffinity()
				return s
			}(),
			expectedError: nil,
		},
		{
			name: "new StatefulSet with non-nil PodAntiAffinity",
			rack: func() scyllav1alpha1.RackSpec {
				r := newBasicRack()
				r.Placement = &scyllav1alpha1.Placement{
					PodAntiAffinity: newPodAntiAffinity(),
				}
				return r
			}(),
			scyllaDBDatacenter:  newBasicScyllaDBDatacenter(),
			existingStatefulSet: nil,
			expectedStatefulSet: func() *appsv1.StatefulSet {
				s := newBasicStatefulSet()
				s.Spec.Template.Spec.Affinity.PodAntiAffinity = newPodAntiAffinity()
				return s
			}(),
			expectedError: nil,
		},
		{
			name: "new StatefulSet with non-nil ImagePullSecrets",
			rack: newBasicRack(),
			scyllaDBDatacenter: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sc := newBasicScyllaDBDatacenter()
				sc.Spec.ImagePullSecrets = []corev1.LocalObjectReference{
					{
						Name: "basic-secrets",
					},
				}
				return sc
			}(),
			existingStatefulSet: nil,
			expectedStatefulSet: func() *appsv1.StatefulSet {
				sts := newBasicStatefulSet()
				sts.Spec.Template.Spec.ImagePullSecrets = []corev1.LocalObjectReference{
					{
						Name: "basic-secrets",
					},
				}
				return sts
			}(),
			expectedError: nil,
		},
		{
			name: "new StatefulSet with non-nil ForceRedeploymentReason",
			rack: newBasicRack(),
			scyllaDBDatacenter: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sc := newBasicScyllaDBDatacenter()
				sc.Spec.ForceRedeploymentReason = pointer.Ptr("reason")
				return sc
			}(),
			existingStatefulSet: nil,
			expectedStatefulSet: func() *appsv1.StatefulSet {
				sts := newBasicStatefulSet()
				sts.Spec.Template.Annotations[naming.ForceRedeploymentReasonAnnotation] = "reason"
				return sts
			}(),
			expectedError: nil,
		},
		{
			name: "new StatefulSet with non-empty externalSeeds",
			rack: newBasicRack(),
			scyllaDBDatacenter: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sc := newBasicScyllaDBDatacenter()
				sc.Spec.ScyllaDB.ExternalSeeds = []string{"10.0.1.1", "10.0.1.2", "10.0.1.3"}
				return sc
			}(),
			existingStatefulSet: nil,
			expectedStatefulSet: func() *appsv1.StatefulSet {
				sts := newBasicStatefulSet()

				sts.Spec.Template.Spec.Containers[scyllaContainerIndex].Command[len(sts.Spec.Template.Spec.Containers[scyllaContainerIndex].Command)-3] = strings.Replace(
					sts.Spec.Template.Spec.Containers[scyllaContainerIndex].Command[len(sts.Spec.Template.Spec.Containers[scyllaContainerIndex].Command)-3],
					` -- "$@"`,
					"--external-seeds=10.0.1.1,10.0.1.2,10.0.1.3 -- \"$@\"",
					1,
				)

				if utilfeature.DefaultMutableFeatureGate.Enabled(features.BootstrapSynchronisation) {
					sts.Spec.Template.Spec.InitContainers[runBootstrapBarrierInitContainerIndex].Command[len(sts.Spec.Template.Spec.InitContainers[runBootstrapBarrierInitContainerIndex].Command)-2] = strings.Replace(
						sts.Spec.Template.Spec.InitContainers[runBootstrapBarrierInitContainerIndex].Command[len(sts.Spec.Template.Spec.InitContainers[runBootstrapBarrierInitContainerIndex].Command)-2],
						`--single-report-allow-non-reporting-host-ids=false`,
						`--single-report-allow-non-reporting-host-ids=true`,
						1,
					)
				}

				return sts
			}(),
			expectedError: nil,
		},
		{
			name: "new StatefulSet with non-empty additional scyllaDB arguments",
			rack: newBasicRack(),
			scyllaDBDatacenter: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sc := newBasicScyllaDBDatacenter()
				sc.Spec.ScyllaDB.AdditionalScyllaDBArguments = []string{
					"--batch-size-warn-threshold-in-kb=128",
					"--batch-size-fail-threshold-in-kb",
					"1024",
					`--commitlog-sync="batch"`,
				}
				return sc
			}(),
			existingStatefulSet: nil,
			expectedStatefulSet: func() *appsv1.StatefulSet {
				sts := newBasicStatefulSet()

				sts.Spec.Template.Spec.Containers[scyllaContainerIndex].Command = append(sts.Spec.Template.Spec.Containers[scyllaContainerIndex].Command[:len(sts.Spec.Template.Spec.Containers[scyllaContainerIndex].Command)-1], "--batch-size-warn-threshold-in-kb=128", "--batch-size-fail-threshold-in-kb", "1024", "--commitlog-sync=\"batch\"", "--developer-mode=0")

				return sts
			}(),
			expectedError: nil,
		},
		{
			name: "new StatefulSet with developer mode",
			rack: newBasicRack(),
			scyllaDBDatacenter: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sc := newBasicScyllaDBDatacenter()
				sc.Spec.ScyllaDB.EnableDeveloperMode = pointer.Ptr(true)
				return sc
			}(),
			existingStatefulSet: nil,
			expectedStatefulSet: func() *appsv1.StatefulSet {
				sts := newBasicStatefulSet()

				sts.Spec.Template.Spec.Containers[scyllaContainerIndex].Command[len(sts.Spec.Template.Spec.Containers[scyllaContainerIndex].Command)-1] = "--developer-mode=1"

				return sts
			}(),
			expectedError: nil,
		},
		{
			name: "new StatefulSet with custom pod metadata uses the new values and doesn't inherit from the ScyllaCluster",
			rack: newBasicRack(),
			scyllaDBDatacenter: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sc := newBasicScyllaDBDatacenter()
				sc.Spec.Metadata = &scyllav1alpha1.ObjectTemplateMetadata{
					Annotations: map[string]string{
						"custom-pod-annotation": "custom-pod-annotation-value",
					},
					Labels: map[string]string{
						"custom-pod-label": "custom-pod-label-value",
					},
				}
				return sc
			}(),
			existingStatefulSet: nil,
			expectedStatefulSet: func() *appsv1.StatefulSet {
				sts := newBasicStatefulSet()

				sts.Spec.Template.ObjectMeta.Annotations = map[string]string{
					"custom-pod-annotation":                    "custom-pod-annotation-value",
					"prometheus.io/port":                       "9180",
					"prometheus.io/scrape":                     "true",
					"scylla-operator.scylladb.com/inputs-hash": "",
				}
				sts.Spec.Template.ObjectMeta.Labels = map[string]string{
					"app":                                   "scylla",
					"app.kubernetes.io/managed-by":          "scylla-operator",
					"app.kubernetes.io/name":                "scylla",
					"custom-pod-label":                      "custom-pod-label-value",
					"scylla/cluster":                        "basic",
					"scylla/datacenter":                     "dc",
					"scylla/rack":                           "rack",
					"scylla/rack-ordinal":                   "0",
					"scylla/scylla-version":                 "latest",
					"scylla-operator.scylladb.com/pod-type": "scylladb-node",
				}

				return sts
			}(),
			expectedError: nil,
		},
		{
			name: "new StatefulSet with custom minReadySeconds",
			rack: newBasicRack(),
			scyllaDBDatacenter: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sc := newBasicScyllaDBDatacenter()
				sc.Spec.MinReadySeconds = pointer.Ptr(int32(1234))
				return sc
			}(),
			existingStatefulSet: nil,
			expectedStatefulSet: func() *appsv1.StatefulSet {
				sts := newBasicStatefulSet()
				sts.Spec.MinReadySeconds = 1234

				return sts
			}(),
			expectedError: nil,
		},
		{
			name: "new StatefulSet with custom readiness gates",
			rack: newBasicRack(),
			scyllaDBDatacenter: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sc := newBasicScyllaDBDatacenter()
				sc.Spec.ReadinessGates = []corev1.PodReadinessGate{
					{
						ConditionType: "my-custom-pod-readiness-gate-1",
					},
					{
						ConditionType: "my-custom-pod-readiness-gate-2",
					},
				}
				return sc
			}(),
			existingStatefulSet: nil,
			expectedStatefulSet: func() *appsv1.StatefulSet {
				sts := newBasicStatefulSet()
				sts.Spec.Template.Spec.ReadinessGates = []corev1.PodReadinessGate{
					{
						ConditionType: "my-custom-pod-readiness-gate-1",
					},
					{
						ConditionType: "my-custom-pod-readiness-gate-2",
					},
				}

				return sts
			}(),
			expectedError: nil,
		},
		{
			name: "new StatefulSet with default Alternator enabled",
			rack: newBasicRack(),
			scyllaDBDatacenter: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sc := newBasicScyllaDBDatacenter()
				sc.Spec.ScyllaDB.AlternatorOptions = &scyllav1alpha1.AlternatorOptions{}
				return sc
			}(),
			existingStatefulSet: nil,
			expectedStatefulSet: func() *appsv1.StatefulSet {
				sts := newBasicStatefulSet()

				tmplSpec := &sts.Spec.Template.Spec
				scylladbContainer := &tmplSpec.Containers[scyllaContainerIndex]

				tmplSpec.Volumes = append(tmplSpec.Volumes, corev1.Volume{
					Name: "scylladb-alternator-serving-certs",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName: "basic-alternator-local-serving-certs",
							Optional:   pointer.Ptr(false),
						},
					},
				})

				scylladbContainer.VolumeMounts = append(scylladbContainer.VolumeMounts, corev1.VolumeMount{
					Name:      "scylladb-alternator-serving-certs",
					ReadOnly:  true,
					MountPath: "/var/run/secrets/scylla-operator.scylladb.com/scylladb/alternator-serving-certs",
				})
				scylladbContainer.Ports = append(
					scylladbContainer.Ports,
					corev1.ContainerPort{
						Name:          "alternator-tls",
						ContainerPort: 8043,
					},
				)

				return sts
			}(),
			expectedError: nil,
		},
		{
			name: "new StatefulSet with default Alternator enabled and disabled http",
			rack: newBasicRack(),
			scyllaDBDatacenter: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sdc := newBasicScyllaDBDatacenter()
				sdc.Spec.ScyllaDB.AlternatorOptions = &scyllav1alpha1.AlternatorOptions{}
				return sdc
			}(),
			existingStatefulSet: nil,
			expectedStatefulSet: func() *appsv1.StatefulSet {
				sts := newBasicStatefulSet()

				tmplSpec := &sts.Spec.Template.Spec
				scylladbContainer := &tmplSpec.Containers[scyllaContainerIndex]

				tmplSpec.Volumes = append(tmplSpec.Volumes, corev1.Volume{
					Name: "scylladb-alternator-serving-certs",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName: "basic-alternator-local-serving-certs",
							Optional:   pointer.Ptr(false),
						},
					},
				})

				scylladbContainer.VolumeMounts = append(scylladbContainer.VolumeMounts, corev1.VolumeMount{
					Name:      "scylladb-alternator-serving-certs",
					ReadOnly:  true,
					MountPath: "/var/run/secrets/scylla-operator.scylladb.com/scylladb/alternator-serving-certs",
				})
				scylladbContainer.Ports = append(
					scylladbContainer.Ports,
					corev1.ContainerPort{
						Name:          "alternator-tls",
						ContainerPort: 8043,
					},
				)

				return sts
			}(),
			expectedError: nil,
		},
		{
			name: "non-propagated annotations are not propagated",
			rack: newBasicRack(),
			scyllaDBDatacenter: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sdc := newBasicScyllaDBDatacenter()
				if len(nonPropagatedAnnotationKeys) == 0 {
					panic("nonPropagatedAnnotationKeys can't be empty")
				}
				for _, k := range nonPropagatedAnnotationKeys {
					metav1.SetMetaDataAnnotation(&sdc.ObjectMeta, k, "")
				}
				return sdc
			}(),
			existingStatefulSet: nil,
			expectedStatefulSet: newBasicStatefulSet(),
			expectedError:       nil,
		},
		{
			name: "non-propagated labels are not propagated",
			rack: newBasicRack(),
			scyllaDBDatacenter: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sdc := newBasicScyllaDBDatacenter()
				if len(nonPropagatedLabelKeys) == 0 {
					panic("nonPropagatedLabelKeys can't be empty")
				}
				for _, k := range nonPropagatedLabelKeys {
					metav1.SetMetaDataLabel(&sdc.ObjectMeta, k, "")
				}
				return sdc
			}(),
			existingStatefulSet: nil,
			expectedStatefulSet: newBasicStatefulSet(),
			expectedError:       nil,
		},
		{
			name: "new StatefulSet with Scylla version lower than required for BootstrapSynchronisation feature",
			rack: newBasicRack(),
			scyllaDBDatacenter: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sdc := newBasicScyllaDBDatacenter()
				sdc.Spec.ScyllaDB.Image = "scylla/scylla:2025.1.0"
				return sdc
			}(),
			existingStatefulSet: nil,
			expectedStatefulSet: func() *appsv1.StatefulSet {
				sts := newBasicStatefulSet()

				sts.Labels["scylla/scylla-version"] = "2025.1.0"
				sts.Spec.Template.Labels["scylla/scylla-version"] = "2025.1.0"
				sts.Spec.Template.Spec.Containers[scyllaContainerIndex].Image = "scylla/scylla:2025.1.0"

				if utilfeature.DefaultMutableFeatureGate.Enabled(features.BootstrapSynchronisation) {
					sts.Spec.Template.Spec.InitContainers = append(sts.Spec.Template.Spec.InitContainers[:runBootstrapBarrierInitContainerIndex],
						sts.Spec.Template.Spec.InitContainers[runBootstrapBarrierInitContainerIndex+1:]...)
				}

				return sts
			}(),
			expectedError: nil,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := StatefulSetForRack(tc.rack, tc.scyllaDBDatacenter, tc.existingStatefulSet, "scylladb/scylla-operator:latest", 0, "")

			if !reflect.DeepEqual(err, tc.expectedError) {
				t.Fatalf("expected and actual errors differ: %s",
					cmp.Diff(tc.expectedError, err, cmpopts.EquateErrors()))
			}

			if !apiequality.Semantic.DeepEqual(got, tc.expectedStatefulSet) {
				t.Errorf("expected and actual StatefulSets differ: %s",
					cmp.Diff(tc.expectedStatefulSet, got))
			}
		})

	}
}

func TestStatefulSetForRack(t *testing.T) {
	for _, f := range features.Features {
		for _, enabled := range []bool{true, false} {
			t.Run("", func(t *testing.T) {
				featuregatetesting.SetFeatureGateDuringTest(
					t,
					utilfeature.DefaultFeatureGate,
					f,
					enabled,
				)

				runTestStatefulSetForRack(t)
			})
		}
	}
}

func TestMakeIngresses(t *testing.T) {
	basicScyllaCluster := &scyllav1alpha1.ScyllaDBDatacenter{
		ObjectMeta: metav1.ObjectMeta{
			Name: "basic",
			UID:  "the-uid",
			Annotations: map[string]string{
				"default-sc-annotation": "bar",
			},
			Labels: map[string]string{
				"default-sc-label": "foo",
			},
		},
		Spec: scyllav1alpha1.ScyllaDBDatacenterSpec{
			ClusterName:    "basic",
			DatacenterName: pointer.Ptr("dc"),
			Racks: []scyllav1alpha1.RackSpec{
				{
					Name: "rack",
					RackTemplate: scyllav1alpha1.RackTemplate{
						ScyllaDB: &scyllav1alpha1.ScyllaDBTemplate{
							Storage: &scyllav1alpha1.StorageOptions{
								Capacity: "1Gi",
							},
						},
					},
				},
			},
		},
	}

	newIdentityService := func(name string) *corev1.Service {
		return &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
				Annotations: map[string]string{
					"default-sc-annotation": "bar",
				},
				Labels: map[string]string{
					"default-sc-label":            "foo",
					naming.ScyllaServiceTypeLabel: string(naming.ScyllaServiceTypeIdentity),
				},
			},
		}
	}

	newMemberService := func(name, hostID string) *corev1.Service {
		return &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
				Annotations: map[string]string{
					"default-sc-annotation":                         "bar",
					"internal.scylla-operator.scylladb.com/host-id": hostID,
				},
				Labels: map[string]string{
					"default-sc-label": "foo",
					"scylla-operator.scylladb.com/scylla-service-type": "member",
				},
			},
		}
	}

	pathTypePrefix := networkingv1.PathTypePrefix

	tt := []struct {
		name               string
		scyllaDBDatacenter *scyllav1alpha1.ScyllaDBDatacenter
		services           map[string]*corev1.Service
		expectedIngresses  []*networkingv1.Ingress
	}{
		{
			name:               "no ingresses when cluster isn't exposed",
			scyllaDBDatacenter: basicScyllaCluster,
			services:           map[string]*corev1.Service{},
			expectedIngresses:  nil,
		},
		{
			name: "no ingresses when ingresses are explicitly disabled",
			scyllaDBDatacenter: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sdc := basicScyllaCluster.DeepCopy()
				sdc.Spec.DNSDomains = []string{"public.scylladb.com", "private.scylladb.com"}
				sdc.Spec.ExposeOptions = &scyllav1alpha1.ExposeOptions{
					CQL: &scyllav1alpha1.CQLExposeOptions{
						Ingress: nil,
					},
				}

				return sdc
			}(),
			services:          map[string]*corev1.Service{},
			expectedIngresses: nil,
		},
		{
			name: "ingress objects are generated for every domain",
			services: map[string]*corev1.Service{
				"any":    newIdentityService("any"),
				"node-1": newMemberService("node-1", "host-id-1"),
				"node-2": newMemberService("node-2", "host-id-2"),
				"node-3": newMemberService("node-3", "host-id-3"),
			},
			scyllaDBDatacenter: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sdc := basicScyllaCluster.DeepCopy()
				sdc.Spec.DNSDomains = []string{"public.scylladb.com", "private.scylladb.com"}
				sdc.Spec.ExposeOptions = &scyllav1alpha1.ExposeOptions{
					CQL: &scyllav1alpha1.CQLExposeOptions{
						Ingress: &scyllav1alpha1.CQLExposeIngressOptions{
							IngressClassName: "cql-ingress-class",
							ObjectTemplateMetadata: scyllav1alpha1.ObjectTemplateMetadata{
								Annotations: map[string]string{
									"my-cql-custom-annotation": "my-cql-custom-annotation-value",
								},
							},
						},
					},
				}

				return sdc
			}(),
			expectedIngresses: []*networkingv1.Ingress{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "any-cql",
						Labels: map[string]string{
							"app":                          "scylla",
							"app.kubernetes.io/name":       "scylla",
							"app.kubernetes.io/managed-by": "scylla-operator",
							"default-sc-label":             "foo",
							"scylla/cluster":               "basic",
							"scylla-operator.scylladb.com/scylla-ingress-type": "AnyNode",
						},
						Annotations: map[string]string{
							"my-cql-custom-annotation": "my-cql-custom-annotation-value",
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion:         "scylla.scylladb.com/v1alpha1",
								Kind:               "ScyllaDBDatacenter",
								Name:               "basic",
								UID:                "the-uid",
								Controller:         pointer.Ptr(true),
								BlockOwnerDeletion: pointer.Ptr(true),
							},
						},
					},
					Spec: networkingv1.IngressSpec{
						IngressClassName: pointer.Ptr("cql-ingress-class"),
						Rules: []networkingv1.IngressRule{
							{
								Host: "cql.public.scylladb.com",
								IngressRuleValue: networkingv1.IngressRuleValue{
									HTTP: &networkingv1.HTTPIngressRuleValue{
										Paths: []networkingv1.HTTPIngressPath{
											{
												Path:     "/",
												PathType: &pathTypePrefix,
												Backend: networkingv1.IngressBackend{
													Service: &networkingv1.IngressServiceBackend{
														Name: "any",
														Port: networkingv1.ServiceBackendPort{
															Name: "cql-ssl",
														},
													},
												},
											},
										},
									},
								},
							},
							{
								Host: "cql.private.scylladb.com",
								IngressRuleValue: networkingv1.IngressRuleValue{
									HTTP: &networkingv1.HTTPIngressRuleValue{
										Paths: []networkingv1.HTTPIngressPath{
											{
												Path:     "/",
												PathType: &pathTypePrefix,
												Backend: networkingv1.IngressBackend{
													Service: &networkingv1.IngressServiceBackend{
														Name: "any",
														Port: networkingv1.ServiceBackendPort{
															Name: "cql-ssl",
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node-1-cql",
						Labels: map[string]string{
							"app":                          "scylla",
							"app.kubernetes.io/name":       "scylla",
							"app.kubernetes.io/managed-by": "scylla-operator",
							"default-sc-label":             "foo",
							"scylla/cluster":               "basic",
							"scylla-operator.scylladb.com/scylla-ingress-type": "Node",
						},
						Annotations: map[string]string{
							"my-cql-custom-annotation": "my-cql-custom-annotation-value",
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion:         "scylla.scylladb.com/v1alpha1",
								Kind:               "ScyllaDBDatacenter",
								Name:               "basic",
								UID:                "the-uid",
								Controller:         pointer.Ptr(true),
								BlockOwnerDeletion: pointer.Ptr(true),
							},
						},
					},
					Spec: networkingv1.IngressSpec{
						IngressClassName: pointer.Ptr("cql-ingress-class"),
						Rules: []networkingv1.IngressRule{
							{
								Host: "host-id-1.cql.public.scylladb.com",
								IngressRuleValue: networkingv1.IngressRuleValue{
									HTTP: &networkingv1.HTTPIngressRuleValue{
										Paths: []networkingv1.HTTPIngressPath{
											{
												Path:     "/",
												PathType: &pathTypePrefix,
												Backend: networkingv1.IngressBackend{
													Service: &networkingv1.IngressServiceBackend{
														Name: "node-1",
														Port: networkingv1.ServiceBackendPort{
															Name: "cql-ssl",
														},
													},
												},
											},
										},
									},
								},
							},
							{
								Host: "host-id-1.cql.private.scylladb.com",
								IngressRuleValue: networkingv1.IngressRuleValue{
									HTTP: &networkingv1.HTTPIngressRuleValue{
										Paths: []networkingv1.HTTPIngressPath{
											{
												Path:     "/",
												PathType: &pathTypePrefix,
												Backend: networkingv1.IngressBackend{
													Service: &networkingv1.IngressServiceBackend{
														Name: "node-1",
														Port: networkingv1.ServiceBackendPort{
															Name: "cql-ssl",
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node-2-cql",
						Labels: map[string]string{
							"app":                          "scylla",
							"app.kubernetes.io/name":       "scylla",
							"app.kubernetes.io/managed-by": "scylla-operator",
							"default-sc-label":             "foo",
							"scylla/cluster":               "basic",
							"scylla-operator.scylladb.com/scylla-ingress-type": "Node",
						},
						Annotations: map[string]string{
							"my-cql-custom-annotation": "my-cql-custom-annotation-value",
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion:         "scylla.scylladb.com/v1alpha1",
								Kind:               "ScyllaDBDatacenter",
								Name:               "basic",
								UID:                "the-uid",
								Controller:         pointer.Ptr(true),
								BlockOwnerDeletion: pointer.Ptr(true),
							},
						},
					},
					Spec: networkingv1.IngressSpec{
						IngressClassName: pointer.Ptr("cql-ingress-class"),
						Rules: []networkingv1.IngressRule{
							{
								Host: "host-id-2.cql.public.scylladb.com",
								IngressRuleValue: networkingv1.IngressRuleValue{
									HTTP: &networkingv1.HTTPIngressRuleValue{
										Paths: []networkingv1.HTTPIngressPath{
											{
												Path:     "/",
												PathType: &pathTypePrefix,
												Backend: networkingv1.IngressBackend{
													Service: &networkingv1.IngressServiceBackend{
														Name: "node-2",
														Port: networkingv1.ServiceBackendPort{
															Name: "cql-ssl",
														},
													},
												},
											},
										},
									},
								},
							},
							{
								Host: "host-id-2.cql.private.scylladb.com",
								IngressRuleValue: networkingv1.IngressRuleValue{
									HTTP: &networkingv1.HTTPIngressRuleValue{
										Paths: []networkingv1.HTTPIngressPath{
											{
												Path:     "/",
												PathType: &pathTypePrefix,
												Backend: networkingv1.IngressBackend{
													Service: &networkingv1.IngressServiceBackend{
														Name: "node-2",
														Port: networkingv1.ServiceBackendPort{
															Name: "cql-ssl",
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node-3-cql",
						Labels: map[string]string{
							"app":                          "scylla",
							"app.kubernetes.io/name":       "scylla",
							"app.kubernetes.io/managed-by": "scylla-operator",
							"default-sc-label":             "foo",
							"scylla/cluster":               "basic",
							"scylla-operator.scylladb.com/scylla-ingress-type": "Node",
						},
						Annotations: map[string]string{
							"my-cql-custom-annotation": "my-cql-custom-annotation-value",
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion:         "scylla.scylladb.com/v1alpha1",
								Kind:               "ScyllaDBDatacenter",
								Name:               "basic",
								UID:                "the-uid",
								Controller:         pointer.Ptr(true),
								BlockOwnerDeletion: pointer.Ptr(true),
							},
						},
					},
					Spec: networkingv1.IngressSpec{
						IngressClassName: pointer.Ptr("cql-ingress-class"),
						Rules: []networkingv1.IngressRule{
							{
								Host: "host-id-3.cql.public.scylladb.com",
								IngressRuleValue: networkingv1.IngressRuleValue{
									HTTP: &networkingv1.HTTPIngressRuleValue{
										Paths: []networkingv1.HTTPIngressPath{
											{
												Path:     "/",
												PathType: &pathTypePrefix,
												Backend: networkingv1.IngressBackend{
													Service: &networkingv1.IngressServiceBackend{
														Name: "node-3",
														Port: networkingv1.ServiceBackendPort{
															Name: "cql-ssl",
														},
													},
												},
											},
										},
									},
								},
							},
							{
								Host: "host-id-3.cql.private.scylladb.com",
								IngressRuleValue: networkingv1.IngressRuleValue{
									HTTP: &networkingv1.HTTPIngressRuleValue{
										Paths: []networkingv1.HTTPIngressPath{
											{
												Path:     "/",
												PathType: &pathTypePrefix,
												Backend: networkingv1.IngressBackend{
													Service: &networkingv1.IngressServiceBackend{
														Name: "node-3",
														Port: networkingv1.ServiceBackendPort{
															Name: "cql-ssl",
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "ingress objects inherit scylladbdatacenter labels and annotations if none are specified",
			services: map[string]*corev1.Service{
				"any":    newIdentityService("any"),
				"node-1": newMemberService("node-1", "host-id-1"),
			},
			scyllaDBDatacenter: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sdc := basicScyllaCluster.DeepCopy()
				sdc.Spec.ExposeOptions = &scyllav1alpha1.ExposeOptions{
					CQL: &scyllav1alpha1.CQLExposeOptions{
						Ingress: &scyllav1alpha1.CQLExposeIngressOptions{
							IngressClassName: "cql-ingress-class",
						},
					},
				}

				return sdc
			}(),
			expectedIngresses: []*networkingv1.Ingress{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "any-cql",
						Labels: map[string]string{
							"app":                          "scylla",
							"app.kubernetes.io/name":       "scylla",
							"app.kubernetes.io/managed-by": "scylla-operator",
							"default-sc-label":             "foo",
							"scylla/cluster":               "basic",
							"scylla-operator.scylladb.com/scylla-ingress-type": "AnyNode",
						},
						Annotations: map[string]string{
							"default-sc-annotation": "bar",
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion:         "scylla.scylladb.com/v1alpha1",
								Kind:               "ScyllaDBDatacenter",
								Name:               "basic",
								UID:                "the-uid",
								Controller:         pointer.Ptr(true),
								BlockOwnerDeletion: pointer.Ptr(true),
							},
						},
					},
					Spec: networkingv1.IngressSpec{
						IngressClassName: pointer.Ptr("cql-ingress-class"),
						Rules:            []networkingv1.IngressRule{},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node-1-cql",
						Labels: map[string]string{
							"app":                          "scylla",
							"app.kubernetes.io/name":       "scylla",
							"app.kubernetes.io/managed-by": "scylla-operator",
							"default-sc-label":             "foo",
							"scylla/cluster":               "basic",
							"scylla-operator.scylladb.com/scylla-ingress-type": "Node",
						},
						Annotations: map[string]string{
							"default-sc-annotation": "bar",
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion:         "scylla.scylladb.com/v1alpha1",
								Kind:               "ScyllaDBDatacenter",
								Name:               "basic",
								UID:                "the-uid",
								Controller:         pointer.Ptr(true),
								BlockOwnerDeletion: pointer.Ptr(true),
							},
						},
					},
					Spec: networkingv1.IngressSpec{
						IngressClassName: pointer.Ptr("cql-ingress-class"),
						Rules:            []networkingv1.IngressRule{},
					},
				},
			},
		},
		{
			name: "non-propagated annotations are not propagated",
			services: map[string]*corev1.Service{
				"any":    newIdentityService("any"),
				"node-1": newMemberService("node-1", "host-id-1"),
			},
			scyllaDBDatacenter: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sdc := basicScyllaCluster.DeepCopy()
				sdc.Spec.ExposeOptions = &scyllav1alpha1.ExposeOptions{
					CQL: &scyllav1alpha1.CQLExposeOptions{
						Ingress: &scyllav1alpha1.CQLExposeIngressOptions{
							IngressClassName: "cql-ingress-class",
						},
					},
				}
				if len(nonPropagatedAnnotationKeys) == 0 {
					panic("nonPropagatedAnnotationKeys can't be empty")
				}
				for _, k := range nonPropagatedAnnotationKeys {
					metav1.SetMetaDataAnnotation(&sdc.ObjectMeta, k, "")
				}
				return sdc
			}(),
			expectedIngresses: []*networkingv1.Ingress{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "any-cql",
						Labels: map[string]string{
							"app":                          "scylla",
							"app.kubernetes.io/name":       "scylla",
							"app.kubernetes.io/managed-by": "scylla-operator",
							"default-sc-label":             "foo",
							"scylla/cluster":               "basic",
							"scylla-operator.scylladb.com/scylla-ingress-type": "AnyNode",
						},
						Annotations: map[string]string{
							"default-sc-annotation": "bar",
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion:         "scylla.scylladb.com/v1alpha1",
								Kind:               "ScyllaDBDatacenter",
								Name:               "basic",
								UID:                "the-uid",
								Controller:         pointer.Ptr(true),
								BlockOwnerDeletion: pointer.Ptr(true),
							},
						},
					},
					Spec: networkingv1.IngressSpec{
						IngressClassName: pointer.Ptr("cql-ingress-class"),
						Rules:            []networkingv1.IngressRule{},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node-1-cql",
						Labels: map[string]string{
							"app":                          "scylla",
							"app.kubernetes.io/name":       "scylla",
							"app.kubernetes.io/managed-by": "scylla-operator",
							"default-sc-label":             "foo",
							"scylla/cluster":               "basic",
							"scylla-operator.scylladb.com/scylla-ingress-type": "Node",
						},
						Annotations: map[string]string{
							"default-sc-annotation": "bar",
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion:         "scylla.scylladb.com/v1alpha1",
								Kind:               "ScyllaDBDatacenter",
								Name:               "basic",
								UID:                "the-uid",
								Controller:         pointer.Ptr(true),
								BlockOwnerDeletion: pointer.Ptr(true),
							},
						},
					},
					Spec: networkingv1.IngressSpec{
						IngressClassName: pointer.Ptr("cql-ingress-class"),
						Rules:            []networkingv1.IngressRule{},
					},
				},
			},
		},
		{
			name: "non-propagated labels are not propagated",
			services: map[string]*corev1.Service{
				"any":    newIdentityService("any"),
				"node-1": newMemberService("node-1", "host-id-1"),
			},
			scyllaDBDatacenter: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sdc := basicScyllaCluster.DeepCopy()
				sdc.Spec.ExposeOptions = &scyllav1alpha1.ExposeOptions{
					CQL: &scyllav1alpha1.CQLExposeOptions{
						Ingress: &scyllav1alpha1.CQLExposeIngressOptions{
							IngressClassName: "cql-ingress-class",
						},
					},
				}
				if len(nonPropagatedLabelKeys) == 0 {
					panic("nonPropagatedLabelKeys can't be empty")
				}
				for _, k := range nonPropagatedLabelKeys {
					metav1.SetMetaDataLabel(&sdc.ObjectMeta, k, "")
				}
				return sdc
			}(),
			expectedIngresses: []*networkingv1.Ingress{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "any-cql",
						Labels: map[string]string{
							"app":                          "scylla",
							"app.kubernetes.io/name":       "scylla",
							"app.kubernetes.io/managed-by": "scylla-operator",
							"default-sc-label":             "foo",
							"scylla/cluster":               "basic",
							"scylla-operator.scylladb.com/scylla-ingress-type": "AnyNode",
						},
						Annotations: map[string]string{
							"default-sc-annotation": "bar",
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion:         "scylla.scylladb.com/v1alpha1",
								Kind:               "ScyllaDBDatacenter",
								Name:               "basic",
								UID:                "the-uid",
								Controller:         pointer.Ptr(true),
								BlockOwnerDeletion: pointer.Ptr(true),
							},
						},
					},
					Spec: networkingv1.IngressSpec{
						IngressClassName: pointer.Ptr("cql-ingress-class"),
						Rules:            []networkingv1.IngressRule{},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node-1-cql",
						Labels: map[string]string{
							"app":                          "scylla",
							"app.kubernetes.io/name":       "scylla",
							"app.kubernetes.io/managed-by": "scylla-operator",
							"default-sc-label":             "foo",
							"scylla/cluster":               "basic",
							"scylla-operator.scylladb.com/scylla-ingress-type": "Node",
						},
						Annotations: map[string]string{
							"default-sc-annotation": "bar",
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion:         "scylla.scylladb.com/v1alpha1",
								Kind:               "ScyllaDBDatacenter",
								Name:               "basic",
								UID:                "the-uid",
								Controller:         pointer.Ptr(true),
								BlockOwnerDeletion: pointer.Ptr(true),
							},
						},
					},
					Spec: networkingv1.IngressSpec{
						IngressClassName: pointer.Ptr("cql-ingress-class"),
						Rules:            []networkingv1.IngressRule{},
					},
				},
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := MakeIngresses(tc.scyllaDBDatacenter, tc.services)
			if !apiequality.Semantic.DeepEqual(got, tc.expectedIngresses) {
				t.Errorf("expected and actual Ingresses differ: %s", cmp.Diff(tc.expectedIngresses, got))
			}
		})
	}
}

func TestMakeJobs(t *testing.T) {
	basicScyllaDBDatacenter := func() *scyllav1alpha1.ScyllaDBDatacenter {
		return &scyllav1alpha1.ScyllaDBDatacenter{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "basic",
				Namespace: "default",
				UID:       "the-uid",
				Labels: map[string]string{
					"default-sc-label": "foo",
				},
				Annotations: map[string]string{
					"default-sc-annotation": "bar",
				},
			},
			Spec: scyllav1alpha1.ScyllaDBDatacenterSpec{
				ClusterName:    "basic",
				DatacenterName: pointer.Ptr("dc"),
				Racks: []scyllav1alpha1.RackSpec{
					{
						Name: "rack",
						RackTemplate: scyllav1alpha1.RackTemplate{
							ScyllaDB: &scyllav1alpha1.ScyllaDBTemplate{
								Storage: &scyllav1alpha1.StorageOptions{
									Capacity: "1Gi",
								},
							},
							Nodes: pointer.Ptr[int32](1),
						},
					},
				},
			},
		}
	}

	newPod := func(name string, podIP string) *corev1.Pod {
		return &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: "default",
			},
			Status: corev1.PodStatus{
				PodIP: podIP,
			},
		}
	}

	newMemberService := func(name string, annotations map[string]string, clusterIP string) *corev1.Service {
		return &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
				Labels: map[string]string{
					"default-sc-label": "foo",
				},
				Annotations: annotations,
			},
			Spec: corev1.ServiceSpec{
				ClusterIP: clusterIP,
			},
		}
	}

	tt := []struct {
		name               string
		scyllaDBDatacenter *scyllav1alpha1.ScyllaDBDatacenter
		services           map[string]*corev1.Service
		pods               []*corev1.Pod
		expectedJobs       []*batchv1.Job
		expectedConditions []metav1.Condition
	}{
		{
			name:               "progressing condition rack member service is not present",
			scyllaDBDatacenter: basicScyllaDBDatacenter(),
			services:           map[string]*corev1.Service{},
			expectedJobs:       nil,
			expectedConditions: []metav1.Condition{
				{
					Type:    "JobControllerProgressing",
					Status:  "True",
					Reason:  "WaitingForService",
					Message: `Waiting for Service "default/basic-dc-rack-0"`,
				},
			},
		},
		{
			name:               "progressing condition when member service doesn't have current token ring hash annotation",
			scyllaDBDatacenter: basicScyllaDBDatacenter(),
			services: map[string]*corev1.Service{
				"basic-dc-rack-0": newMemberService("basic-dc-rack-0", map[string]string{}, "1.1.1.1"),
			},
			expectedJobs: nil,
			expectedConditions: []metav1.Condition{
				{
					Type:    "JobControllerProgressing",
					Status:  "True",
					Reason:  "WaitingForServiceState",
					Message: `Service "basic-dc-rack-0" is missing current token ring hash annotation`,
				},
			},
		},
		{
			name:               "progressing condition when member service current token ring hash annotation is empty",
			scyllaDBDatacenter: basicScyllaDBDatacenter(),
			services: map[string]*corev1.Service{
				"basic-dc-rack-0": newMemberService("basic-dc-rack-0", map[string]string{
					"internal.scylla-operator.scylladb.com/current-token-ring-hash": "",
				}, "1.1.1.1"),
			},
			expectedJobs: nil,
			expectedConditions: []metav1.Condition{
				{
					Type:    "JobControllerProgressing",
					Status:  "True",
					Reason:  "UnexpectedServiceState",
					Message: `Service "basic-dc-rack-0" has unexpected empty current token ring hash annotation, can't create cleanup Job`,
				},
			},
		},
		{
			name:               "progressing condition when member service doesn't have latest token ring hash annotation",
			scyllaDBDatacenter: basicScyllaDBDatacenter(),
			services: map[string]*corev1.Service{
				"basic-dc-rack-0": newMemberService("basic-dc-rack-0", map[string]string{
					"internal.scylla-operator.scylladb.com/current-token-ring-hash": "abc",
				}, "1.1.1.1"),
			},
			expectedJobs: nil,
			expectedConditions: []metav1.Condition{
				{
					Type:    "JobControllerProgressing",
					Status:  "True",
					Reason:  "WaitingForServiceState",
					Message: `Service "basic-dc-rack-0" is missing last cleaned up token ring hash annotation`,
				},
			},
		},
		{
			name:               "progressing condition when member service last cleaned up token ring hash annotation is empty",
			scyllaDBDatacenter: basicScyllaDBDatacenter(),
			services: map[string]*corev1.Service{
				"basic-dc-rack-0": newMemberService("basic-dc-rack-0", map[string]string{
					"internal.scylla-operator.scylladb.com/current-token-ring-hash":         "abc",
					"internal.scylla-operator.scylladb.com/last-cleaned-up-token-ring-hash": "",
				}, "1.1.1.1"),
			},
			expectedJobs: nil,
			expectedConditions: []metav1.Condition{
				{
					Type:    "JobControllerProgressing",
					Status:  "True",
					Reason:  "UnexpectedServiceState",
					Message: `Service "basic-dc-rack-0" has unexpected empty last cleaned up token ring hash annotation, can't create cleanup Job`,
				},
			},
		},
		{
			name:               "no cleanup jobs when member service token ring hash annotations are equal",
			scyllaDBDatacenter: basicScyllaDBDatacenter(),
			services: map[string]*corev1.Service{
				"basic-dc-rack-0": newMemberService("basic-dc-rack-0", map[string]string{
					"internal.scylla-operator.scylladb.com/current-token-ring-hash":         "abc",
					"internal.scylla-operator.scylladb.com/last-cleaned-up-token-ring-hash": "abc",
				}, "1.1.1.1"),
			},
			expectedJobs:       nil,
			expectedConditions: nil,
		},
		{
			name:               "cleanup job when member service token ring hash annotations differ",
			scyllaDBDatacenter: basicScyllaDBDatacenter(),
			services: func() map[string]*corev1.Service {
				return map[string]*corev1.Service{
					"basic-dc-rack-0": newMemberService("basic-dc-rack-0", map[string]string{
						"internal.scylla-operator.scylladb.com/current-token-ring-hash":         "abc",
						"internal.scylla-operator.scylladb.com/last-cleaned-up-token-ring-hash": "def",
					}, "1.1.1.1"),
				}
			}(),
			pods: func() []*corev1.Pod {
				return []*corev1.Pod{
					newPod("basic-dc-rack-0", "2.2.2.2"),
				}
			}(),
			expectedJobs: []*batchv1.Job{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cleanup-basic-dc-rack-0",
						Namespace: "default",
						Annotations: map[string]string{
							"default-sc-annotation": "bar",
							"internal.scylla-operator.scylladb.com/cleanup-token-ring-hash": "abc",
						},
						Labels: map[string]string{
							"default-sc-label":                           "foo",
							"scylla/cluster":                             "basic",
							"scylla-operator.scylladb.com/node-job":      "basic-dc-rack-0",
							"scylla-operator.scylladb.com/node-job-type": "Cleanup",
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion:         "scylla.scylladb.com/v1alpha1",
								Kind:               "ScyllaDBDatacenter",
								Name:               "basic",
								UID:                "the-uid",
								Controller:         pointer.Ptr(true),
								BlockOwnerDeletion: pointer.Ptr(true),
							},
						},
					},
					Spec: batchv1.JobSpec{
						Selector:       nil,
						ManualSelector: pointer.Ptr(false),
						Template: corev1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Annotations: map[string]string{
									"default-sc-annotation": "bar",
									"internal.scylla-operator.scylladb.com/cleanup-token-ring-hash": "abc",
								},
								Labels: map[string]string{
									"default-sc-label":                           "foo",
									"scylla/cluster":                             "basic",
									"scylla-operator.scylladb.com/node-job":      "basic-dc-rack-0",
									"scylla-operator.scylladb.com/node-job-type": "Cleanup",
									"scylla-operator.scylladb.com/pod-type":      "cleanup-job",
								},
							},
							Spec: corev1.PodSpec{
								RestartPolicy: corev1.RestartPolicyOnFailure,
								Containers: []corev1.Container{
									{
										Name:            naming.CleanupContainerName,
										Image:           "scylladb/scylla-operator:latest",
										ImagePullPolicy: corev1.PullIfNotPresent,
										Args: []string{
											"cleanup-job",
											"--manager-auth-config-path=/etc/scylla-cleanup-job/auth-token.yaml",
											"--node-address=1.1.1.1",
										},
										VolumeMounts: []corev1.VolumeMount{
											{
												Name:      "scylla-manager-agent-token",
												ReadOnly:  true,
												MountPath: "/etc/scylla-cleanup-job/auth-token.yaml",
												SubPath:   "auth-token.yaml",
											},
										},
									},
								},
								Volumes: []corev1.Volume{
									{
										Name: "scylla-manager-agent-token",
										VolumeSource: corev1.VolumeSource{
											Secret: &corev1.SecretVolumeSource{
												SecretName: "basic-auth-token",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expectedConditions: nil,
		},
		{
			name: "cleanup job has the same placement requirements as ScyllaCluster",
			scyllaDBDatacenter: func() *scyllav1alpha1.ScyllaDBDatacenter {
				scyllaDBDatacenter := basicScyllaDBDatacenter()
				scyllaDBDatacenter.Spec.Racks[0].Placement = &scyllav1alpha1.Placement{
					NodeAffinity: &corev1.NodeAffinity{
						RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
							NodeSelectorTerms: []corev1.NodeSelectorTerm{
								{
									MatchExpressions: []corev1.NodeSelectorRequirement{
										{
											Key:      "topology.kubernetes.io/zone",
											Operator: corev1.NodeSelectorOpIn,
											Values:   []string{"zone-1", "zone-2"},
										},
									},
								},
							},
						},
					},
					PodAffinity: &corev1.PodAffinity{
						RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
							{
								LabelSelector: &metav1.LabelSelector{
									MatchLabels: map[string]string{
										"app.kubernetes.io/name": "scylla",
										"scylla/rack":            "rack",
									},
								},
								TopologyKey: "kubernetes.io/hostname",
							},
						},
					},
					PodAntiAffinity: &corev1.PodAntiAffinity{
						RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
							{
								LabelSelector: &metav1.LabelSelector{
									MatchLabels: map[string]string{
										"scylla-operator.scylladb.com/node-job-type": "Cleanup",
									},
								},
								TopologyKey: "kubernetes.io/hostname",
							},
						},
					},
					Tolerations: []corev1.Toleration{
						{
							Key:      "role",
							Operator: corev1.TolerationOpEqual,
							Value:    "scylla-clusters",
							Effect:   corev1.TaintEffectNoSchedule,
						},
					},
				}
				scyllaDBDatacenter.Spec.Racks = append(scyllaDBDatacenter.Spec.Racks,
					scyllav1alpha1.RackSpec{
						Name: "rack-2",
						RackTemplate: scyllav1alpha1.RackTemplate{
							ScyllaDB: &scyllav1alpha1.ScyllaDBTemplate{
								Storage: &scyllav1alpha1.StorageOptions{
									Capacity: "1Gi",
								},
							},
							Nodes: pointer.Ptr[int32](1),
							Placement: &scyllav1alpha1.Placement{
								NodeAffinity: &corev1.NodeAffinity{
									RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
										NodeSelectorTerms: []corev1.NodeSelectorTerm{
											{
												MatchExpressions: []corev1.NodeSelectorRequirement{
													{
														Key:      "topology.kubernetes.io/zone",
														Operator: corev1.NodeSelectorOpIn,
														Values:   []string{"zone-3", "zone-4"},
													},
												},
											},
										},
									},
								},
								PodAffinity: &corev1.PodAffinity{
									RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
										{
											LabelSelector: &metav1.LabelSelector{
												MatchLabels: map[string]string{
													"app.kubernetes.io/name": "scylla",
													"scylla/rack":            "rack-2",
												},
											},
											TopologyKey: "kubernetes.io/hostname",
										},
									},
								},
								PodAntiAffinity: &corev1.PodAntiAffinity{
									RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
										{
											LabelSelector: &metav1.LabelSelector{
												MatchLabels: map[string]string{
													"scylla-operator.scylladb.com/node-job-type": "Cleanup",
													"scylla/rack-ordinal":                        "0",
												},
											},
											TopologyKey: "kubernetes.io/hostname",
										},
									},
								},
								Tolerations: []corev1.Toleration{
									{
										Key:      "role",
										Operator: corev1.TolerationOpEqual,
										Value:    "scylla-clusters-rack-2",
										Effect:   corev1.TaintEffectNoSchedule,
									},
								},
							},
						},
					},
				)

				return scyllaDBDatacenter
			}(),
			pods: []*corev1.Pod{
				newPod("basic-dc-rack-0", "2.2.2.2"),
				newPod("basic-dc-rack-2-0", "2.2.2.3"),
			},
			services: func() map[string]*corev1.Service {
				return map[string]*corev1.Service{
					"basic-dc-rack-0": newMemberService("basic-dc-rack-0", map[string]string{
						"internal.scylla-operator.scylladb.com/current-token-ring-hash":         "abc",
						"internal.scylla-operator.scylladb.com/last-cleaned-up-token-ring-hash": "def",
					}, "1.1.1.1"),
					"basic-dc-rack-2-0": newMemberService("basic-dc-rack-2-0", map[string]string{
						"internal.scylla-operator.scylladb.com/current-token-ring-hash":         "abc",
						"internal.scylla-operator.scylladb.com/last-cleaned-up-token-ring-hash": "def",
					}, "1.1.1.2"),
				}
			}(),
			expectedJobs: []*batchv1.Job{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cleanup-basic-dc-rack-0",
						Namespace: "default",
						Annotations: map[string]string{
							"default-sc-annotation": "bar",
							"internal.scylla-operator.scylladb.com/cleanup-token-ring-hash": "abc",
						},
						Labels: map[string]string{
							"default-sc-label":                           "foo",
							"scylla/cluster":                             "basic",
							"scylla-operator.scylladb.com/node-job":      "basic-dc-rack-0",
							"scylla-operator.scylladb.com/node-job-type": "Cleanup",
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion:         "scylla.scylladb.com/v1alpha1",
								Kind:               "ScyllaDBDatacenter",
								Name:               "basic",
								UID:                "the-uid",
								Controller:         pointer.Ptr(true),
								BlockOwnerDeletion: pointer.Ptr(true),
							},
						},
					},
					Spec: batchv1.JobSpec{
						Selector:       nil,
						ManualSelector: pointer.Ptr(false),
						Template: corev1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Annotations: map[string]string{
									"default-sc-annotation": "bar",
									"internal.scylla-operator.scylladb.com/cleanup-token-ring-hash": "abc",
								},
								Labels: map[string]string{
									"default-sc-label":                           "foo",
									"scylla/cluster":                             "basic",
									"scylla-operator.scylladb.com/node-job":      "basic-dc-rack-0",
									"scylla-operator.scylladb.com/node-job-type": "Cleanup",
									"scylla-operator.scylladb.com/pod-type":      "cleanup-job",
								},
							},
							Spec: corev1.PodSpec{
								RestartPolicy: corev1.RestartPolicyOnFailure,
								Tolerations: []corev1.Toleration{
									{
										Key:      "role",
										Operator: corev1.TolerationOpEqual,
										Value:    "scylla-clusters",
										Effect:   corev1.TaintEffectNoSchedule,
									},
								},
								Affinity: &corev1.Affinity{
									NodeAffinity: &corev1.NodeAffinity{
										RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
											NodeSelectorTerms: []corev1.NodeSelectorTerm{
												{
													MatchExpressions: []corev1.NodeSelectorRequirement{
														{
															Key:      "topology.kubernetes.io/zone",
															Operator: corev1.NodeSelectorOpIn,
															Values:   []string{"zone-1", "zone-2"},
														},
													},
												},
											},
										},
									},
									PodAffinity: &corev1.PodAffinity{
										RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
											{
												LabelSelector: &metav1.LabelSelector{
													MatchLabels: map[string]string{
														"app.kubernetes.io/name": "scylla",
														"scylla/rack":            "rack",
													},
												},
												TopologyKey: "kubernetes.io/hostname",
											},
										},
									},
									PodAntiAffinity: &corev1.PodAntiAffinity{
										RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
											{
												LabelSelector: &metav1.LabelSelector{
													MatchLabels: map[string]string{
														"scylla-operator.scylladb.com/node-job-type": "Cleanup",
													},
												},
												TopologyKey: "kubernetes.io/hostname",
											},
										},
									},
								},
								Containers: []corev1.Container{
									{
										Name:            naming.CleanupContainerName,
										Image:           "scylladb/scylla-operator:latest",
										ImagePullPolicy: corev1.PullIfNotPresent,
										Args: []string{
											"cleanup-job",
											"--manager-auth-config-path=/etc/scylla-cleanup-job/auth-token.yaml",
											"--node-address=1.1.1.1",
										},
										VolumeMounts: []corev1.VolumeMount{
											{
												Name:      "scylla-manager-agent-token",
												ReadOnly:  true,
												MountPath: "/etc/scylla-cleanup-job/auth-token.yaml",
												SubPath:   "auth-token.yaml",
											},
										},
									},
								},
								Volumes: []corev1.Volume{
									{
										Name: "scylla-manager-agent-token",
										VolumeSource: corev1.VolumeSource{
											Secret: &corev1.SecretVolumeSource{
												SecretName: "basic-auth-token",
											},
										},
									},
								},
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cleanup-basic-dc-rack-2-0",
						Namespace: "default",
						Annotations: map[string]string{
							"default-sc-annotation": "bar",
							"internal.scylla-operator.scylladb.com/cleanup-token-ring-hash": "abc",
						},
						Labels: map[string]string{
							"default-sc-label":                           "foo",
							"scylla/cluster":                             "basic",
							"scylla-operator.scylladb.com/node-job":      "basic-dc-rack-2-0",
							"scylla-operator.scylladb.com/node-job-type": "Cleanup",
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion:         "scylla.scylladb.com/v1alpha1",
								Kind:               "ScyllaDBDatacenter",
								Name:               "basic",
								UID:                "the-uid",
								Controller:         pointer.Ptr(true),
								BlockOwnerDeletion: pointer.Ptr(true),
							},
						},
					},
					Spec: batchv1.JobSpec{
						Selector:       nil,
						ManualSelector: pointer.Ptr(false),
						Template: corev1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Annotations: map[string]string{
									"default-sc-annotation": "bar",
									"internal.scylla-operator.scylladb.com/cleanup-token-ring-hash": "abc",
								},
								Labels: map[string]string{
									"default-sc-label":                           "foo",
									"scylla/cluster":                             "basic",
									"scylla-operator.scylladb.com/node-job":      "basic-dc-rack-2-0",
									"scylla-operator.scylladb.com/node-job-type": "Cleanup",
									"scylla-operator.scylladb.com/pod-type":      "cleanup-job",
								},
							},
							Spec: corev1.PodSpec{
								RestartPolicy: corev1.RestartPolicyOnFailure,
								Tolerations: []corev1.Toleration{
									{
										Key:      "role",
										Operator: corev1.TolerationOpEqual,
										Value:    "scylla-clusters-rack-2",
										Effect:   corev1.TaintEffectNoSchedule,
									},
								},
								Affinity: &corev1.Affinity{
									NodeAffinity: &corev1.NodeAffinity{
										RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
											NodeSelectorTerms: []corev1.NodeSelectorTerm{
												{
													MatchExpressions: []corev1.NodeSelectorRequirement{
														{
															Key:      "topology.kubernetes.io/zone",
															Operator: corev1.NodeSelectorOpIn,
															Values:   []string{"zone-3", "zone-4"},
														},
													},
												},
											},
										},
									},
									PodAffinity: &corev1.PodAffinity{
										RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
											{
												LabelSelector: &metav1.LabelSelector{
													MatchLabels: map[string]string{
														"app.kubernetes.io/name": "scylla",
														"scylla/rack":            "rack-2",
													},
												},
												TopologyKey: "kubernetes.io/hostname",
											},
										},
									},
									PodAntiAffinity: &corev1.PodAntiAffinity{
										RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
											{
												LabelSelector: &metav1.LabelSelector{
													MatchLabels: map[string]string{
														"scylla-operator.scylladb.com/node-job-type": "Cleanup",
														"scylla/rack-ordinal":                        "0",
													},
												},
												TopologyKey: "kubernetes.io/hostname",
											},
										},
									},
								},
								Containers: []corev1.Container{
									{
										Name:            naming.CleanupContainerName,
										Image:           "scylladb/scylla-operator:latest",
										ImagePullPolicy: corev1.PullIfNotPresent,
										Args: []string{
											"cleanup-job",
											"--manager-auth-config-path=/etc/scylla-cleanup-job/auth-token.yaml",
											"--node-address=1.1.1.2",
										},
										VolumeMounts: []corev1.VolumeMount{
											{
												Name:      "scylla-manager-agent-token",
												ReadOnly:  true,
												MountPath: "/etc/scylla-cleanup-job/auth-token.yaml",
												SubPath:   "auth-token.yaml",
											},
										},
									},
								},
								Volumes: []corev1.Volume{
									{
										Name: "scylla-manager-agent-token",
										VolumeSource: corev1.VolumeSource{
											Secret: &corev1.SecretVolumeSource{
												SecretName: "basic-auth-token",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expectedConditions: nil,
		},
		{
			name: "cleanup job connects through PodIP when ScyllaDBDatacenter is exposed via PodIP to clients",
			scyllaDBDatacenter: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sdc := basicScyllaDBDatacenter()
				sdc.Spec.ExposeOptions = &scyllav1alpha1.ExposeOptions{
					BroadcastOptions: &scyllav1alpha1.NodeBroadcastOptions{
						Clients: scyllav1alpha1.BroadcastOptions{
							Type: scyllav1alpha1.BroadcastAddressTypePodIP,
						},
					},
				}

				return sdc
			}(),
			services: func() map[string]*corev1.Service {
				return map[string]*corev1.Service{
					"basic-dc-rack-0": newMemberService("basic-dc-rack-0", map[string]string{
						"internal.scylla-operator.scylladb.com/current-token-ring-hash":         "abc",
						"internal.scylla-operator.scylladb.com/last-cleaned-up-token-ring-hash": "def",
					}, "1.1.1.1"),
				}
			}(),
			pods: func() []*corev1.Pod {
				return []*corev1.Pod{
					newPod("basic-dc-rack-0", "2.2.2.2"),
				}
			}(),
			expectedJobs: []*batchv1.Job{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cleanup-basic-dc-rack-0",
						Namespace: "default",
						Annotations: map[string]string{
							"default-sc-annotation": "bar",
							"internal.scylla-operator.scylladb.com/cleanup-token-ring-hash": "abc",
						},
						Labels: map[string]string{
							"default-sc-label":                           "foo",
							"scylla/cluster":                             "basic",
							"scylla-operator.scylladb.com/node-job":      "basic-dc-rack-0",
							"scylla-operator.scylladb.com/node-job-type": "Cleanup",
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion:         "scylla.scylladb.com/v1alpha1",
								Kind:               "ScyllaDBDatacenter",
								Name:               "basic",
								UID:                "the-uid",
								Controller:         pointer.Ptr(true),
								BlockOwnerDeletion: pointer.Ptr(true),
							},
						},
					},
					Spec: batchv1.JobSpec{
						Selector:       nil,
						ManualSelector: pointer.Ptr(false),
						Template: corev1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Annotations: map[string]string{
									"default-sc-annotation": "bar",
									"internal.scylla-operator.scylladb.com/cleanup-token-ring-hash": "abc",
								},
								Labels: map[string]string{
									"default-sc-label":                           "foo",
									"scylla/cluster":                             "basic",
									"scylla-operator.scylladb.com/node-job":      "basic-dc-rack-0",
									"scylla-operator.scylladb.com/node-job-type": "Cleanup",
									"scylla-operator.scylladb.com/pod-type":      "cleanup-job",
								},
							},
							Spec: corev1.PodSpec{
								RestartPolicy: corev1.RestartPolicyOnFailure,
								Containers: []corev1.Container{
									{
										Name:            naming.CleanupContainerName,
										Image:           "scylladb/scylla-operator:latest",
										ImagePullPolicy: corev1.PullIfNotPresent,
										Args: []string{
											"cleanup-job",
											"--manager-auth-config-path=/etc/scylla-cleanup-job/auth-token.yaml",
											"--node-address=2.2.2.2",
										},
										VolumeMounts: []corev1.VolumeMount{
											{
												Name:      "scylla-manager-agent-token",
												ReadOnly:  true,
												MountPath: "/etc/scylla-cleanup-job/auth-token.yaml",
												SubPath:   "auth-token.yaml",
											},
										},
									},
								},
								Volumes: []corev1.Volume{
									{
										Name: "scylla-manager-agent-token",
										VolumeSource: corev1.VolumeSource{
											Secret: &corev1.SecretVolumeSource{
												SecretName: "basic-auth-token",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expectedConditions: nil,
		},
		{
			name: "cleanup job connects through LoadBalancer external IP address when ScyllaDBDatacenter is exposed via LoadBalancer Service to clients",
			scyllaDBDatacenter: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sdc := basicScyllaDBDatacenter()
				sdc.Spec.ExposeOptions = &scyllav1alpha1.ExposeOptions{
					BroadcastOptions: &scyllav1alpha1.NodeBroadcastOptions{
						Clients: scyllav1alpha1.BroadcastOptions{
							Type: scyllav1alpha1.BroadcastAddressTypeServiceLoadBalancerIngress,
						},
					},
				}

				return sdc
			}(),
			services: func() map[string]*corev1.Service {
				svc := newMemberService("basic-dc-rack-0", map[string]string{
					"internal.scylla-operator.scylladb.com/current-token-ring-hash":         "abc",
					"internal.scylla-operator.scylladb.com/last-cleaned-up-token-ring-hash": "def",
				}, "1.1.1.1")

				svc.Spec.Type = corev1.ServiceTypeLoadBalancer
				svc.Status.LoadBalancer.Ingress = []corev1.LoadBalancerIngress{
					{
						IP: "3.3.3.3",
					},
				}

				return map[string]*corev1.Service{
					"basic-dc-rack-0": svc,
				}
			}(),
			pods: func() []*corev1.Pod {
				return []*corev1.Pod{
					newPod("basic-dc-rack-0", "2.2.2.2"),
				}
			}(),
			expectedJobs: []*batchv1.Job{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cleanup-basic-dc-rack-0",
						Namespace: "default",
						Annotations: map[string]string{
							"default-sc-annotation": "bar",
							"internal.scylla-operator.scylladb.com/cleanup-token-ring-hash": "abc",
						},
						Labels: map[string]string{
							"default-sc-label":                           "foo",
							"scylla/cluster":                             "basic",
							"scylla-operator.scylladb.com/node-job":      "basic-dc-rack-0",
							"scylla-operator.scylladb.com/node-job-type": "Cleanup",
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion:         "scylla.scylladb.com/v1alpha1",
								Kind:               "ScyllaDBDatacenter",
								Name:               "basic",
								UID:                "the-uid",
								Controller:         pointer.Ptr(true),
								BlockOwnerDeletion: pointer.Ptr(true),
							},
						},
					},
					Spec: batchv1.JobSpec{
						Selector:       nil,
						ManualSelector: pointer.Ptr(false),
						Template: corev1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Annotations: map[string]string{
									"default-sc-annotation": "bar",
									"internal.scylla-operator.scylladb.com/cleanup-token-ring-hash": "abc",
								},
								Labels: map[string]string{
									"default-sc-label":                           "foo",
									"scylla/cluster":                             "basic",
									"scylla-operator.scylladb.com/node-job":      "basic-dc-rack-0",
									"scylla-operator.scylladb.com/node-job-type": "Cleanup",
									"scylla-operator.scylladb.com/pod-type":      "cleanup-job",
								},
							},
							Spec: corev1.PodSpec{
								RestartPolicy: corev1.RestartPolicyOnFailure,
								Containers: []corev1.Container{
									{
										Name:            naming.CleanupContainerName,
										Image:           "scylladb/scylla-operator:latest",
										ImagePullPolicy: corev1.PullIfNotPresent,
										Args: []string{
											"cleanup-job",
											"--manager-auth-config-path=/etc/scylla-cleanup-job/auth-token.yaml",
											"--node-address=3.3.3.3",
										},
										VolumeMounts: []corev1.VolumeMount{
											{
												Name:      "scylla-manager-agent-token",
												ReadOnly:  true,
												MountPath: "/etc/scylla-cleanup-job/auth-token.yaml",
												SubPath:   "auth-token.yaml",
											},
										},
									},
								},
								Volumes: []corev1.Volume{
									{
										Name: "scylla-manager-agent-token",
										VolumeSource: corev1.VolumeSource{
											Secret: &corev1.SecretVolumeSource{
												SecretName: "basic-auth-token",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expectedConditions: nil,
		},
		{
			name: "cleanup job connects through LoadBalancer external IP address when ScyllaDBDatacenter is exposed via LoadBalancer Service to clients",
			scyllaDBDatacenter: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sdc := basicScyllaDBDatacenter()
				sdc.Spec.ExposeOptions = &scyllav1alpha1.ExposeOptions{
					BroadcastOptions: &scyllav1alpha1.NodeBroadcastOptions{
						Clients: scyllav1alpha1.BroadcastOptions{
							Type: scyllav1alpha1.BroadcastAddressTypeServiceLoadBalancerIngress,
						},
					},
				}

				return sdc
			}(),
			services: func() map[string]*corev1.Service {
				svc := newMemberService("basic-dc-rack-0", map[string]string{
					"internal.scylla-operator.scylladb.com/current-token-ring-hash":         "abc",
					"internal.scylla-operator.scylladb.com/last-cleaned-up-token-ring-hash": "def",
				}, "1.1.1.1")

				svc.Spec.Type = corev1.ServiceTypeLoadBalancer
				svc.Status.LoadBalancer.Ingress = []corev1.LoadBalancerIngress{
					{
						Hostname: "external.lb.address.com",
					},
				}

				return map[string]*corev1.Service{
					"basic-dc-rack-0": svc,
				}
			}(),
			pods: func() []*corev1.Pod {
				return []*corev1.Pod{
					newPod("basic-dc-rack-0", "2.2.2.2"),
				}
			}(),
			expectedJobs: []*batchv1.Job{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cleanup-basic-dc-rack-0",
						Namespace: "default",
						Annotations: map[string]string{
							"default-sc-annotation": "bar",
							"internal.scylla-operator.scylladb.com/cleanup-token-ring-hash": "abc",
						},
						Labels: map[string]string{
							"default-sc-label":                           "foo",
							"scylla/cluster":                             "basic",
							"scylla-operator.scylladb.com/node-job":      "basic-dc-rack-0",
							"scylla-operator.scylladb.com/node-job-type": "Cleanup",
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion:         "scylla.scylladb.com/v1alpha1",
								Kind:               "ScyllaDBDatacenter",
								Name:               "basic",
								UID:                "the-uid",
								Controller:         pointer.Ptr(true),
								BlockOwnerDeletion: pointer.Ptr(true),
							},
						},
					},
					Spec: batchv1.JobSpec{
						Selector:       nil,
						ManualSelector: pointer.Ptr(false),
						Template: corev1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Annotations: map[string]string{
									"default-sc-annotation": "bar",
									"internal.scylla-operator.scylladb.com/cleanup-token-ring-hash": "abc",
								},
								Labels: map[string]string{
									"default-sc-label":                           "foo",
									"scylla/cluster":                             "basic",
									"scylla-operator.scylladb.com/node-job":      "basic-dc-rack-0",
									"scylla-operator.scylladb.com/node-job-type": "Cleanup",
									"scylla-operator.scylladb.com/pod-type":      "cleanup-job",
								},
							},
							Spec: corev1.PodSpec{
								RestartPolicy: corev1.RestartPolicyOnFailure,
								Containers: []corev1.Container{
									{
										Name:            naming.CleanupContainerName,
										Image:           "scylladb/scylla-operator:latest",
										ImagePullPolicy: corev1.PullIfNotPresent,
										Args: []string{
											"cleanup-job",
											"--manager-auth-config-path=/etc/scylla-cleanup-job/auth-token.yaml",
											"--node-address=external.lb.address.com",
										},
										VolumeMounts: []corev1.VolumeMount{
											{
												Name:      "scylla-manager-agent-token",
												ReadOnly:  true,
												MountPath: "/etc/scylla-cleanup-job/auth-token.yaml",
												SubPath:   "auth-token.yaml",
											},
										},
									},
								},
								Volumes: []corev1.Volume{
									{
										Name: "scylla-manager-agent-token",
										VolumeSource: corev1.VolumeSource{
											Secret: &corev1.SecretVolumeSource{
												SecretName: "basic-auth-token",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expectedConditions: nil,
		},
		{
			name:               "non-propagated labels are not propagated",
			scyllaDBDatacenter: basicScyllaDBDatacenter(),
			pods: []*corev1.Pod{
				newPod("basic-dc-rack-0", "2.2.2.2"),
			},
			services: func() map[string]*corev1.Service {
				return map[string]*corev1.Service{
					"basic-dc-rack-0": newMemberService("basic-dc-rack-0", map[string]string{
						"internal.scylla-operator.scylladb.com/current-token-ring-hash":         "abc",
						"internal.scylla-operator.scylladb.com/last-cleaned-up-token-ring-hash": "def",
					}, "1.1.1.1"),
				}
			}(),
			expectedJobs: []*batchv1.Job{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cleanup-basic-dc-rack-0",
						Namespace: "default",
						Annotations: map[string]string{
							"default-sc-annotation": "bar",
							"internal.scylla-operator.scylladb.com/cleanup-token-ring-hash": "abc",
						},
						Labels: map[string]string{
							"default-sc-label":                           "foo",
							"scylla/cluster":                             "basic",
							"scylla-operator.scylladb.com/node-job":      "basic-dc-rack-0",
							"scylla-operator.scylladb.com/node-job-type": "Cleanup",
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion:         "scylla.scylladb.com/v1alpha1",
								Kind:               "ScyllaDBDatacenter",
								Name:               "basic",
								UID:                "the-uid",
								Controller:         pointer.Ptr(true),
								BlockOwnerDeletion: pointer.Ptr(true),
							},
						},
					},
					Spec: batchv1.JobSpec{
						Selector:       nil,
						ManualSelector: pointer.Ptr(false),
						Template: corev1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Annotations: map[string]string{
									"default-sc-annotation": "bar",
									"internal.scylla-operator.scylladb.com/cleanup-token-ring-hash": "abc",
								},
								Labels: map[string]string{
									"default-sc-label":                           "foo",
									"scylla/cluster":                             "basic",
									"scylla-operator.scylladb.com/node-job":      "basic-dc-rack-0",
									"scylla-operator.scylladb.com/node-job-type": "Cleanup",
									"scylla-operator.scylladb.com/pod-type":      "cleanup-job",
								},
							},
							Spec: corev1.PodSpec{
								RestartPolicy: corev1.RestartPolicyOnFailure,
								Containers: []corev1.Container{
									{
										Name:            naming.CleanupContainerName,
										Image:           "scylladb/scylla-operator:latest",
										ImagePullPolicy: corev1.PullIfNotPresent,
										Args: []string{
											"cleanup-job",
											"--manager-auth-config-path=/etc/scylla-cleanup-job/auth-token.yaml",
											"--node-address=1.1.1.1",
										},
										VolumeMounts: []corev1.VolumeMount{
											{
												Name:      "scylla-manager-agent-token",
												ReadOnly:  true,
												MountPath: "/etc/scylla-cleanup-job/auth-token.yaml",
												SubPath:   "auth-token.yaml",
											},
										},
									},
								},
								Volumes: []corev1.Volume{
									{
										Name: "scylla-manager-agent-token",
										VolumeSource: corev1.VolumeSource{
											Secret: &corev1.SecretVolumeSource{
												SecretName: "basic-auth-token",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expectedConditions: nil,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			podCache := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
			for _, obj := range tc.pods {
				err := podCache.Add(obj)
				if err != nil {
					t.Fatal(err)
				}
			}

			podLister := corev1listers.NewPodLister(podCache)

			gotJobs, gotConditions, err := MakeJobs(tc.scyllaDBDatacenter, tc.services, podLister, "scylladb/scylla-operator:latest")
			if err != nil {
				t.Errorf("expected nil err, got: %v", err)
			}
			if !apiequality.Semantic.DeepEqual(gotJobs, tc.expectedJobs) {
				t.Errorf("expected and actual Job differ: %s", cmp.Diff(tc.expectedJobs, gotJobs))
			}
			if !reflect.DeepEqual(gotConditions, tc.expectedConditions) {
				t.Fatalf("expected and actual conditions differ: %s", cmp.Diff(tc.expectedConditions, gotConditions))
			}
		})
	}
}

func Test_MakeManagedScyllaDBConfig(t *testing.T) {
	newBasicScyllaDBDatacenter := func() *scyllav1alpha1.ScyllaDBDatacenter {
		return &scyllav1alpha1.ScyllaDBDatacenter{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:   "foo-ns",
				Name:        "foo",
				UID:         "uid-42",
				Annotations: map[string]string{},
				Labels: map[string]string{
					"user-label": "user-label-value",
				},
			},
			Spec: scyllav1alpha1.ScyllaDBDatacenterSpec{
				ClusterName: "foo-cluster",
			},
		}
	}

	tt := []struct {
		name                 string
		sdc                  *scyllav1alpha1.ScyllaDBDatacenter
		enableTLSFeatureGate bool
		expectedCM           *corev1.ConfigMap
		expectedErr          error
	}{
		{
			name:                 "no TLS config when the feature is disabled",
			sdc:                  newBasicScyllaDBDatacenter(),
			enableTLSFeatureGate: false,
			expectedCM: &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ConfigMap",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace:   "foo-ns",
					Name:        "foo-managed-config",
					Annotations: map[string]string{},
					Labels: map[string]string{
						"app":                          "scylla",
						"app.kubernetes.io/managed-by": "scylla-operator",
						"app.kubernetes.io/name":       "scylla",
						"scylla/cluster":               "foo",
						"user-label":                   "user-label-value",
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "scylla.scylladb.com/v1alpha1",
							Kind:               "ScyllaDBDatacenter",
							Name:               "foo",
							UID:                "uid-42",
							Controller:         pointer.Ptr(true),
							BlockOwnerDeletion: pointer.Ptr(true),
						},
					},
				},
				Data: map[string]string{
					"scylladb-managed-config.yaml": strings.TrimPrefix(`
cluster_name: "foo-cluster"
rpc_address: "0.0.0.0"
api_address: "127.0.0.1"
listen_address: "0.0.0.0"
seed_provider:
  - class_name: org.apache.cassandra.locator.SimpleSeedProvider
    parameters:
      - seeds: "127.0.0.1"
endpoint_snitch: "GossipingPropertyFileSnitch"
internode_compression: "all"
`, "\n"),
				},
			},
			expectedErr: nil,
		},
		{
			name:                 "TLS config present when the feature is enabled",
			sdc:                  newBasicScyllaDBDatacenter(),
			enableTLSFeatureGate: true,
			expectedCM: &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ConfigMap",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace:   "foo-ns",
					Name:        "foo-managed-config",
					Annotations: map[string]string{},
					Labels: map[string]string{
						"app":                          "scylla",
						"app.kubernetes.io/managed-by": "scylla-operator",
						"app.kubernetes.io/name":       "scylla",
						"scylla/cluster":               "foo",
						"user-label":                   "user-label-value",
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "scylla.scylladb.com/v1alpha1",
							Kind:               "ScyllaDBDatacenter",
							Name:               "foo",
							UID:                "uid-42",
							Controller:         pointer.Ptr(true),
							BlockOwnerDeletion: pointer.Ptr(true),
						},
					},
				},
				Data: map[string]string{
					"scylladb-managed-config.yaml": strings.TrimPrefix(`
cluster_name: "foo-cluster"
rpc_address: "0.0.0.0"
api_address: "127.0.0.1"
listen_address: "0.0.0.0"
seed_provider:
  - class_name: org.apache.cassandra.locator.SimpleSeedProvider
    parameters:
      - seeds: "127.0.0.1"
endpoint_snitch: "GossipingPropertyFileSnitch"
internode_compression: "all"
native_transport_port_ssl: 9142
native_shard_aware_transport_port_ssl: 19142
client_encryption_options:
  enabled: true
  optional: false
  certificate: "/var/run/secrets/scylla-operator.scylladb.com/scylladb/serving-certs/tls.crt"
  keyfile: "/var/run/secrets/scylla-operator.scylladb.com/scylladb/serving-certs/tls.key"
  require_client_auth: true
  truststore: "/var/run/configmaps/scylla-operator.scylladb.com/scylladb/client-ca/ca-bundle.crt"
`, "\n"),
				},
			},
			expectedErr: nil,
		},
		{
			name: "alternator is setup on TLS-only with authorization by default",
			sdc: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sdc := newBasicScyllaDBDatacenter()
				sdc.Spec.ScyllaDB.AlternatorOptions = &scyllav1alpha1.AlternatorOptions{}
				return sdc
			}(),
			enableTLSFeatureGate: true,
			expectedCM: &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ConfigMap",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo-ns",
					Name:      "foo-managed-config",
					Labels: map[string]string{
						"app":                          "scylla",
						"app.kubernetes.io/managed-by": "scylla-operator",
						"app.kubernetes.io/name":       "scylla",
						"scylla/cluster":               "foo",
						"user-label":                   "user-label-value",
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "scylla.scylladb.com/v1alpha1",
							Kind:               "ScyllaDBDatacenter",
							Name:               "foo",
							UID:                "uid-42",
							Controller:         pointer.Ptr(true),
							BlockOwnerDeletion: pointer.Ptr(true),
						},
					},
				},
				Data: map[string]string{
					"scylladb-managed-config.yaml": strings.TrimPrefix(`
cluster_name: "foo-cluster"
rpc_address: "0.0.0.0"
api_address: "127.0.0.1"
listen_address: "0.0.0.0"
seed_provider:
  - class_name: org.apache.cassandra.locator.SimpleSeedProvider
    parameters:
      - seeds: "127.0.0.1"
endpoint_snitch: "GossipingPropertyFileSnitch"
internode_compression: "all"
native_transport_port_ssl: 9142
native_shard_aware_transport_port_ssl: 19142
client_encryption_options:
  enabled: true
  optional: false
  certificate: "/var/run/secrets/scylla-operator.scylladb.com/scylladb/serving-certs/tls.crt"
  keyfile: "/var/run/secrets/scylla-operator.scylladb.com/scylladb/serving-certs/tls.key"
  require_client_auth: true
  truststore: "/var/run/configmaps/scylla-operator.scylladb.com/scylladb/client-ca/ca-bundle.crt"
alternator_write_isolation: always_use_lwt
alternator_enforce_authorization: true
alternator_https_port: 8043
alternator_encryption_options:
  certificate: "/var/run/secrets/scylla-operator.scylladb.com/scylladb/alternator-serving-certs/tls.crt"
  keyfile: "/var/run/secrets/scylla-operator.scylladb.com/scylladb/alternator-serving-certs/tls.key"
`, "\n"),
				},
			},
			expectedErr: nil,
		},
		{
			name: "alternator is setup on TLS-only when insecure port is disabled",
			sdc: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sdc := newBasicScyllaDBDatacenter()
				metav1.SetMetaDataAnnotation(&sdc.ObjectMeta, "internal.scylla-operator.scylladb.com/alternator-insecure-enable-http", "false")
				sdc.Spec.ScyllaDB.AlternatorOptions = &scyllav1alpha1.AlternatorOptions{}
				return sdc
			}(),
			enableTLSFeatureGate: true,
			expectedCM: &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ConfigMap",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo-ns",
					Name:      "foo-managed-config",
					Labels: map[string]string{
						"app":                          "scylla",
						"app.kubernetes.io/managed-by": "scylla-operator",
						"app.kubernetes.io/name":       "scylla",
						"scylla/cluster":               "foo",
						"user-label":                   "user-label-value",
					},
					Annotations: map[string]string{
						"internal.scylla-operator.scylladb.com/alternator-insecure-enable-http": "false",
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "scylla.scylladb.com/v1alpha1",
							Kind:               "ScyllaDBDatacenter",
							Name:               "foo",
							UID:                "uid-42",
							Controller:         pointer.Ptr(true),
							BlockOwnerDeletion: pointer.Ptr(true),
						},
					},
				},
				Data: map[string]string{
					"scylladb-managed-config.yaml": strings.TrimPrefix(`
cluster_name: "foo-cluster"
rpc_address: "0.0.0.0"
api_address: "127.0.0.1"
listen_address: "0.0.0.0"
seed_provider:
  - class_name: org.apache.cassandra.locator.SimpleSeedProvider
    parameters:
      - seeds: "127.0.0.1"
endpoint_snitch: "GossipingPropertyFileSnitch"
internode_compression: "all"
native_transport_port_ssl: 9142
native_shard_aware_transport_port_ssl: 19142
client_encryption_options:
  enabled: true
  optional: false
  certificate: "/var/run/secrets/scylla-operator.scylladb.com/scylladb/serving-certs/tls.crt"
  keyfile: "/var/run/secrets/scylla-operator.scylladb.com/scylladb/serving-certs/tls.key"
  require_client_auth: true
  truststore: "/var/run/configmaps/scylla-operator.scylladb.com/scylladb/client-ca/ca-bundle.crt"
alternator_write_isolation: always_use_lwt
alternator_enforce_authorization: true
alternator_https_port: 8043
alternator_encryption_options:
  certificate: "/var/run/secrets/scylla-operator.scylladb.com/scylladb/alternator-serving-certs/tls.crt"
  keyfile: "/var/run/secrets/scylla-operator.scylladb.com/scylladb/alternator-serving-certs/tls.key"
`, "\n"),
				},
			},
			expectedErr: nil,
		},
		{
			name: "alternator is setup both on TLS and insecure when insecure port is enabled",
			sdc: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sdc := newBasicScyllaDBDatacenter()
				metav1.SetMetaDataAnnotation(&sdc.ObjectMeta, "internal.scylla-operator.scylladb.com/alternator-insecure-enable-http", "true")
				sdc.Spec.ScyllaDB.AlternatorOptions = &scyllav1alpha1.AlternatorOptions{}
				return sdc
			}(),
			enableTLSFeatureGate: true,
			expectedCM: &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ConfigMap",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo-ns",
					Name:      "foo-managed-config",
					Labels: map[string]string{
						"app":                          "scylla",
						"app.kubernetes.io/managed-by": "scylla-operator",
						"app.kubernetes.io/name":       "scylla",
						"scylla/cluster":               "foo",
						"user-label":                   "user-label-value",
					},
					Annotations: map[string]string{
						"internal.scylla-operator.scylladb.com/alternator-insecure-enable-http": "true",
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "scylla.scylladb.com/v1alpha1",
							Kind:               "ScyllaDBDatacenter",
							Name:               "foo",
							UID:                "uid-42",
							Controller:         pointer.Ptr(true),
							BlockOwnerDeletion: pointer.Ptr(true),
						},
					},
				},
				Data: map[string]string{
					"scylladb-managed-config.yaml": strings.TrimPrefix(`
cluster_name: "foo-cluster"
rpc_address: "0.0.0.0"
api_address: "127.0.0.1"
listen_address: "0.0.0.0"
seed_provider:
  - class_name: org.apache.cassandra.locator.SimpleSeedProvider
    parameters:
      - seeds: "127.0.0.1"
endpoint_snitch: "GossipingPropertyFileSnitch"
internode_compression: "all"
native_transport_port_ssl: 9142
native_shard_aware_transport_port_ssl: 19142
client_encryption_options:
  enabled: true
  optional: false
  certificate: "/var/run/secrets/scylla-operator.scylladb.com/scylladb/serving-certs/tls.crt"
  keyfile: "/var/run/secrets/scylla-operator.scylladb.com/scylladb/serving-certs/tls.key"
  require_client_auth: true
  truststore: "/var/run/configmaps/scylla-operator.scylladb.com/scylladb/client-ca/ca-bundle.crt"
alternator_write_isolation: always_use_lwt
alternator_enforce_authorization: true
alternator_port: 8000
alternator_https_port: 8043
alternator_encryption_options:
  certificate: "/var/run/secrets/scylla-operator.scylladb.com/scylladb/alternator-serving-certs/tls.crt"
  keyfile: "/var/run/secrets/scylla-operator.scylladb.com/scylladb/alternator-serving-certs/tls.key"
`, "\n"),
				},
			},
			expectedErr: nil,
		},
		{
			name: "alternator is setup without authorization when manual port is specified for backwards compatibility",
			sdc: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sdc := newBasicScyllaDBDatacenter()
				metav1.SetMetaDataAnnotation(&sdc.ObjectMeta, "internal.scylla-operator.scylladb.com/alternator-port", "42")
				sdc.Spec.ScyllaDB.AlternatorOptions = &scyllav1alpha1.AlternatorOptions{}
				return sdc
			}(),
			enableTLSFeatureGate: true,
			expectedCM: &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ConfigMap",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo-ns",
					Name:      "foo-managed-config",
					Labels: map[string]string{
						"app":                          "scylla",
						"app.kubernetes.io/managed-by": "scylla-operator",
						"app.kubernetes.io/name":       "scylla",
						"scylla/cluster":               "foo",
						"user-label":                   "user-label-value",
					},
					Annotations: map[string]string{
						"internal.scylla-operator.scylladb.com/alternator-port": "42",
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "scylla.scylladb.com/v1alpha1",
							Kind:               "ScyllaDBDatacenter",
							Name:               "foo",
							UID:                "uid-42",
							Controller:         pointer.Ptr(true),
							BlockOwnerDeletion: pointer.Ptr(true),
						},
					},
				},
				Data: map[string]string{
					"scylladb-managed-config.yaml": strings.TrimPrefix(`
cluster_name: "foo-cluster"
rpc_address: "0.0.0.0"
api_address: "127.0.0.1"
listen_address: "0.0.0.0"
seed_provider:
  - class_name: org.apache.cassandra.locator.SimpleSeedProvider
    parameters:
      - seeds: "127.0.0.1"
endpoint_snitch: "GossipingPropertyFileSnitch"
internode_compression: "all"
native_transport_port_ssl: 9142
native_shard_aware_transport_port_ssl: 19142
client_encryption_options:
  enabled: true
  optional: false
  certificate: "/var/run/secrets/scylla-operator.scylladb.com/scylladb/serving-certs/tls.crt"
  keyfile: "/var/run/secrets/scylla-operator.scylladb.com/scylladb/serving-certs/tls.key"
  require_client_auth: true
  truststore: "/var/run/configmaps/scylla-operator.scylladb.com/scylladb/client-ca/ca-bundle.crt"
alternator_write_isolation: always_use_lwt
alternator_enforce_authorization: false
alternator_port: 42
alternator_https_port: 8043
alternator_encryption_options:
  certificate: "/var/run/secrets/scylla-operator.scylladb.com/scylladb/alternator-serving-certs/tls.crt"
  keyfile: "/var/run/secrets/scylla-operator.scylladb.com/scylladb/alternator-serving-certs/tls.key"
`, "\n"),
				},
			},
			expectedErr: nil,
		},
		{
			name: "alternator is setup with authorization when manual port is specified and authorization is enabled",
			sdc: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sdc := newBasicScyllaDBDatacenter()
				metav1.SetMetaDataAnnotation(&sdc.ObjectMeta, "internal.scylla-operator.scylladb.com/alternator-port", "42")
				metav1.SetMetaDataAnnotation(&sdc.ObjectMeta, "internal.scylla-operator.scylladb.com/alternator-insecure-disable-authorization", "false")
				sdc.Spec.ScyllaDB.AlternatorOptions = &scyllav1alpha1.AlternatorOptions{}
				return sdc
			}(),
			enableTLSFeatureGate: true,
			expectedCM: &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ConfigMap",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo-ns",
					Name:      "foo-managed-config",
					Labels: map[string]string{
						"app":                          "scylla",
						"app.kubernetes.io/managed-by": "scylla-operator",
						"app.kubernetes.io/name":       "scylla",
						"scylla/cluster":               "foo",
						"user-label":                   "user-label-value",
					},
					Annotations: map[string]string{
						"internal.scylla-operator.scylladb.com/alternator-port":                           "42",
						"internal.scylla-operator.scylladb.com/alternator-insecure-disable-authorization": "false",
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "scylla.scylladb.com/v1alpha1",
							Kind:               "ScyllaDBDatacenter",
							Name:               "foo",
							UID:                "uid-42",
							Controller:         pointer.Ptr(true),
							BlockOwnerDeletion: pointer.Ptr(true),
						},
					},
				},
				Data: map[string]string{
					"scylladb-managed-config.yaml": strings.TrimPrefix(`
cluster_name: "foo-cluster"
rpc_address: "0.0.0.0"
api_address: "127.0.0.1"
listen_address: "0.0.0.0"
seed_provider:
  - class_name: org.apache.cassandra.locator.SimpleSeedProvider
    parameters:
      - seeds: "127.0.0.1"
endpoint_snitch: "GossipingPropertyFileSnitch"
internode_compression: "all"
native_transport_port_ssl: 9142
native_shard_aware_transport_port_ssl: 19142
client_encryption_options:
  enabled: true
  optional: false
  certificate: "/var/run/secrets/scylla-operator.scylladb.com/scylladb/serving-certs/tls.crt"
  keyfile: "/var/run/secrets/scylla-operator.scylladb.com/scylladb/serving-certs/tls.key"
  require_client_auth: true
  truststore: "/var/run/configmaps/scylla-operator.scylladb.com/scylladb/client-ca/ca-bundle.crt"
alternator_write_isolation: always_use_lwt
alternator_enforce_authorization: true
alternator_port: 42
alternator_https_port: 8043
alternator_encryption_options:
  certificate: "/var/run/secrets/scylla-operator.scylladb.com/scylladb/alternator-serving-certs/tls.crt"
  keyfile: "/var/run/secrets/scylla-operator.scylladb.com/scylladb/alternator-serving-certs/tls.key"
`, "\n"),
				},
			},
			expectedErr: nil,
		},
		{
			name: "alternator is setup without authorization when it's disabled and using manual port",
			sdc: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sdc := newBasicScyllaDBDatacenter()
				metav1.SetMetaDataAnnotation(&sdc.ObjectMeta, "internal.scylla-operator.scylladb.com/alternator-port", "42")
				metav1.SetMetaDataAnnotation(&sdc.ObjectMeta, "internal.scylla-operator.scylladb.com/alternator-insecure-disable-authorization", "true")
				sdc.Spec.ScyllaDB.AlternatorOptions = &scyllav1alpha1.AlternatorOptions{}
				return sdc
			}(),
			enableTLSFeatureGate: true,
			expectedCM: &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ConfigMap",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo-ns",
					Name:      "foo-managed-config",
					Labels: map[string]string{
						"app":                          "scylla",
						"app.kubernetes.io/managed-by": "scylla-operator",
						"app.kubernetes.io/name":       "scylla",
						"scylla/cluster":               "foo",
						"user-label":                   "user-label-value",
					},
					Annotations: map[string]string{
						"internal.scylla-operator.scylladb.com/alternator-port":                           "42",
						"internal.scylla-operator.scylladb.com/alternator-insecure-disable-authorization": "true",
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "scylla.scylladb.com/v1alpha1",
							Kind:               "ScyllaDBDatacenter",
							Name:               "foo",
							UID:                "uid-42",
							Controller:         pointer.Ptr(true),
							BlockOwnerDeletion: pointer.Ptr(true),
						},
					},
				},
				Data: map[string]string{
					"scylladb-managed-config.yaml": strings.TrimPrefix(`
cluster_name: "foo-cluster"
rpc_address: "0.0.0.0"
api_address: "127.0.0.1"
listen_address: "0.0.0.0"
seed_provider:
  - class_name: org.apache.cassandra.locator.SimpleSeedProvider
    parameters:
      - seeds: "127.0.0.1"
endpoint_snitch: "GossipingPropertyFileSnitch"
internode_compression: "all"
native_transport_port_ssl: 9142
native_shard_aware_transport_port_ssl: 19142
client_encryption_options:
  enabled: true
  optional: false
  certificate: "/var/run/secrets/scylla-operator.scylladb.com/scylladb/serving-certs/tls.crt"
  keyfile: "/var/run/secrets/scylla-operator.scylladb.com/scylladb/serving-certs/tls.key"
  require_client_auth: true
  truststore: "/var/run/configmaps/scylla-operator.scylladb.com/scylladb/client-ca/ca-bundle.crt"
alternator_write_isolation: always_use_lwt
alternator_enforce_authorization: false
alternator_port: 42
alternator_https_port: 8043
alternator_encryption_options:
  certificate: "/var/run/secrets/scylla-operator.scylladb.com/scylladb/alternator-serving-certs/tls.crt"
  keyfile: "/var/run/secrets/scylla-operator.scylladb.com/scylladb/alternator-serving-certs/tls.key"
`, "\n"),
				},
			},
			expectedErr: nil,
		},
		{
			name: "alternator is setup without authorization when it's disabled",
			sdc: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sdc := newBasicScyllaDBDatacenter()
				metav1.SetMetaDataAnnotation(&sdc.ObjectMeta, "internal.scylla-operator.scylladb.com/alternator-insecure-disable-authorization", "true")
				sdc.Spec.ScyllaDB.AlternatorOptions = &scyllav1alpha1.AlternatorOptions{}
				return sdc
			}(),
			enableTLSFeatureGate: true,
			expectedCM: &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ConfigMap",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo-ns",
					Name:      "foo-managed-config",
					Labels: map[string]string{
						"app":                          "scylla",
						"app.kubernetes.io/managed-by": "scylla-operator",
						"app.kubernetes.io/name":       "scylla",
						"scylla/cluster":               "foo",
						"user-label":                   "user-label-value",
					},
					Annotations: map[string]string{
						"internal.scylla-operator.scylladb.com/alternator-insecure-disable-authorization": "true",
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "scylla.scylladb.com/v1alpha1",
							Kind:               "ScyllaDBDatacenter",
							Name:               "foo",
							UID:                "uid-42",
							Controller:         pointer.Ptr(true),
							BlockOwnerDeletion: pointer.Ptr(true),
						},
					},
				},
				Data: map[string]string{
					"scylladb-managed-config.yaml": strings.TrimPrefix(`
cluster_name: "foo-cluster"
rpc_address: "0.0.0.0"
api_address: "127.0.0.1"
listen_address: "0.0.0.0"
seed_provider:
  - class_name: org.apache.cassandra.locator.SimpleSeedProvider
    parameters:
      - seeds: "127.0.0.1"
endpoint_snitch: "GossipingPropertyFileSnitch"
internode_compression: "all"
native_transport_port_ssl: 9142
native_shard_aware_transport_port_ssl: 19142
client_encryption_options:
  enabled: true
  optional: false
  certificate: "/var/run/secrets/scylla-operator.scylladb.com/scylladb/serving-certs/tls.crt"
  keyfile: "/var/run/secrets/scylla-operator.scylladb.com/scylladb/serving-certs/tls.key"
  require_client_auth: true
  truststore: "/var/run/configmaps/scylla-operator.scylladb.com/scylladb/client-ca/ca-bundle.crt"
alternator_write_isolation: always_use_lwt
alternator_enforce_authorization: false
alternator_https_port: 8043
alternator_encryption_options:
  certificate: "/var/run/secrets/scylla-operator.scylladb.com/scylladb/alternator-serving-certs/tls.crt"
  keyfile: "/var/run/secrets/scylla-operator.scylladb.com/scylladb/alternator-serving-certs/tls.key"
`, "\n"),
				},
			},
			expectedErr: nil,
		},
		{
			name: "non-propagated annotations are not propagated",
			sdc: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sdc := newBasicScyllaDBDatacenter()
				if len(nonPropagatedAnnotationKeys) == 0 {
					panic("nonPropagatedAnnotationKeys can't be empty")
				}
				for _, k := range nonPropagatedAnnotationKeys {
					metav1.SetMetaDataAnnotation(&sdc.ObjectMeta, k, "")
				}
				return sdc
			}(),
			enableTLSFeatureGate: false,
			expectedCM: &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ConfigMap",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace:   "foo-ns",
					Name:        "foo-managed-config",
					Annotations: map[string]string{},
					Labels: map[string]string{
						"app":                          "scylla",
						"app.kubernetes.io/managed-by": "scylla-operator",
						"app.kubernetes.io/name":       "scylla",
						"scylla/cluster":               "foo",
						"user-label":                   "user-label-value",
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "scylla.scylladb.com/v1alpha1",
							Kind:               "ScyllaDBDatacenter",
							Name:               "foo",
							UID:                "uid-42",
							Controller:         pointer.Ptr(true),
							BlockOwnerDeletion: pointer.Ptr(true),
						},
					},
				},
				Data: map[string]string{
					"scylladb-managed-config.yaml": strings.TrimPrefix(`
cluster_name: "foo-cluster"
rpc_address: "0.0.0.0"
api_address: "127.0.0.1"
listen_address: "0.0.0.0"
seed_provider:
  - class_name: org.apache.cassandra.locator.SimpleSeedProvider
    parameters:
      - seeds: "127.0.0.1"
endpoint_snitch: "GossipingPropertyFileSnitch"
internode_compression: "all"
`, "\n"),
				},
			},
			expectedErr: nil,
		},
		{
			name: "non-propagated labels are not propagated",
			sdc: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sdc := newBasicScyllaDBDatacenter()
				if len(nonPropagatedLabelKeys) == 0 {
					panic("nonPropagatedLabelKeys can't be empty")
				}
				for _, k := range nonPropagatedLabelKeys {
					metav1.SetMetaDataLabel(&sdc.ObjectMeta, k, "")
				}
				return sdc
			}(),
			enableTLSFeatureGate: false,
			expectedCM: &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ConfigMap",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace:   "foo-ns",
					Name:        "foo-managed-config",
					Annotations: map[string]string{},
					Labels: map[string]string{
						"app":                          "scylla",
						"app.kubernetes.io/managed-by": "scylla-operator",
						"app.kubernetes.io/name":       "scylla",
						"scylla/cluster":               "foo",
						"user-label":                   "user-label-value",
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "scylla.scylladb.com/v1alpha1",
							Kind:               "ScyllaDBDatacenter",
							Name:               "foo",
							UID:                "uid-42",
							Controller:         pointer.Ptr(true),
							BlockOwnerDeletion: pointer.Ptr(true),
						},
					},
				},
				Data: map[string]string{
					"scylladb-managed-config.yaml": strings.TrimPrefix(`
cluster_name: "foo-cluster"
rpc_address: "0.0.0.0"
api_address: "127.0.0.1"
listen_address: "0.0.0.0"
seed_provider:
  - class_name: org.apache.cassandra.locator.SimpleSeedProvider
    parameters:
      - seeds: "127.0.0.1"
endpoint_snitch: "GossipingPropertyFileSnitch"
internode_compression: "all"
`, "\n"),
				},
			},
			expectedErr: nil,
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			featuregatetesting.SetFeatureGateDuringTest(
				t,
				utilfeature.DefaultMutableFeatureGate,
				features.AutomaticTLSCertificates,
				tc.enableTLSFeatureGate,
			)

			got, err := MakeManagedScyllaDBConfig(tc.sdc)
			if !reflect.DeepEqual(err, tc.expectedErr) {
				t.Errorf("expected and actual errors differ: %s", cmp.Diff(tc.expectedErr, err))
			}

			if !apiequality.Semantic.DeepEqual(got, tc.expectedCM) {
				t.Errorf("expected and actual configmaps differ:\n%s", cmp.Diff(tc.expectedCM, got))
			}
		})
	}
}

func TestMakeManagedScyllaDBManagerAgentConfig(t *testing.T) {
	tt := []struct {
		name        string
		sdc         *scyllav1alpha1.ScyllaDBDatacenter
		expectedErr error
		expectedCM  *corev1.ConfigMap
	}{
		{
			name: "manager agent config with IPv4 (default)",
			sdc: &scyllav1alpha1.ScyllaDBDatacenter{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo-ns",
					Name:      "foo",
					UID:       "uid-42",
					Labels: map[string]string{
						"user-label": "user-label-value",
					},
					Annotations: map[string]string{
						"user-annotation": "user-annotation-value",
					},
				},
				Spec: scyllav1alpha1.ScyllaDBDatacenterSpec{
					ClusterName: "foo-cluster",
				},
			},
			expectedCM: &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ConfigMap",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo-ns",
					Name:      "foo-manager-agent-config",
					Labels: map[string]string{
						"app":                          "scylla",
						"app.kubernetes.io/managed-by": "scylla-operator",
						"app.kubernetes.io/name":       "scylla",
						"scylla/cluster":               "foo",
						"user-label":                   "user-label-value",
					},
					Annotations: map[string]string{
						"user-annotation": "user-annotation-value",
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "scylla.scylladb.com/v1alpha1",
							Kind:               "ScyllaDBDatacenter",
							Name:               "foo",
							UID:                "uid-42",
							Controller:         pointer.Ptr(true),
							BlockOwnerDeletion: pointer.Ptr(true),
						},
					},
				},
				Data: map[string]string{
					"scylla-manager-agent.yaml": strings.TrimLeft(`
# ScyllaDB API endpoint configuration based on IP family
scylla:
  api_address: "127.0.0.1"
  api_port: 10000

# Default ScyllaDB Manager Agent configuration
# Additional configuration can be provided via customConfigSecretRef
`, "\n"),
				},
			},
			expectedErr: nil,
		},
		{
			name: "manager agent config with IPv6",
			sdc: &scyllav1alpha1.ScyllaDBDatacenter{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo-ns",
					Name:      "foo",
					UID:       "uid-42",
					Labels: map[string]string{
						"user-label": "user-label-value",
					},
					Annotations: map[string]string{
						"user-annotation": "user-annotation-value",
					},
				},
				Spec: scyllav1alpha1.ScyllaDBDatacenterSpec{
					ClusterName: "foo-cluster",
					IPFamily:    pointer.Ptr(corev1.IPv6Protocol),
				},
			},
			expectedCM: &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ConfigMap",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo-ns",
					Name:      "foo-manager-agent-config",
					Labels: map[string]string{
						"app":                          "scylla",
						"app.kubernetes.io/managed-by": "scylla-operator",
						"app.kubernetes.io/name":       "scylla",
						"scylla/cluster":               "foo",
						"user-label":                   "user-label-value",
					},
					Annotations: map[string]string{
						"user-annotation": "user-annotation-value",
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "scylla.scylladb.com/v1alpha1",
							Kind:               "ScyllaDBDatacenter",
							Name:               "foo",
							UID:                "uid-42",
							Controller:         pointer.Ptr(true),
							BlockOwnerDeletion: pointer.Ptr(true),
						},
					},
				},
				Data: map[string]string{
					"scylla-manager-agent.yaml": strings.TrimLeft(`
# ScyllaDB API endpoint configuration based on IP family
scylla:
  api_address: "::1"
  api_port: 10000

# Default ScyllaDB Manager Agent configuration
# Additional configuration can be provided via customConfigSecretRef
`, "\n"),
				},
			},
			expectedErr: nil,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			got, err := MakeManagedScyllaDBManagerAgentConfig(tc.sdc)
			if !reflect.DeepEqual(err, tc.expectedErr) {
				t.Errorf("expected and actual errors differ: %s", cmp.Diff(tc.expectedErr, err))
			}

			if !apiequality.Semantic.DeepEqual(got, tc.expectedCM) {
				t.Errorf("expected and actual configmaps differ:\n%s", cmp.Diff(tc.expectedCM, got))
			}
		})
	}
}

func TestMakeManagedScyllaDBSnitchConfig(t *testing.T) {
	tt := []struct {
		name        string
		sdc         *scyllav1alpha1.ScyllaDBDatacenter
		expectedErr error
		expectedCMs []*corev1.ConfigMap
	}{
		{
			name: "snitch config per dc rack",
			sdc: &scyllav1alpha1.ScyllaDBDatacenter{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo-ns",
					Name:      "foo",
					UID:       "uid-42",
					Labels: map[string]string{
						"user-label": "user-label-value",
					},
					Annotations: map[string]string{
						"user-annotation": "user-annotation-value",
					},
				},
				Spec: scyllav1alpha1.ScyllaDBDatacenterSpec{
					ClusterName: "foo-cluster",
					Racks: []scyllav1alpha1.RackSpec{
						{
							Name: "rack0",
						},
						{
							Name: "rack1",
						},
					},
				},
			},
			expectedCMs: []*corev1.ConfigMap{
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       "ConfigMap",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "foo-ns",
						Name:      "foo-rack0-snitch-config",
						Labels: map[string]string{
							"app":                          "scylla",
							"app.kubernetes.io/managed-by": "scylla-operator",
							"app.kubernetes.io/name":       "scylla",
							"scylla/cluster":               "foo",
							"user-label":                   "user-label-value",
						},
						Annotations: map[string]string{
							"user-annotation": "user-annotation-value",
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion:         "scylla.scylladb.com/v1alpha1",
								Kind:               "ScyllaDBDatacenter",
								Name:               "foo",
								UID:                "uid-42",
								Controller:         pointer.Ptr(true),
								BlockOwnerDeletion: pointer.Ptr(true),
							},
						},
					},
					Data: map[string]string{
						"cassandra-rackdc.properties": strings.TrimLeft(`
dc=foo
rack=rack0
prefer_local=false
`, "\n"),
					},
				},
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       "ConfigMap",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "foo-ns",
						Name:      "foo-rack1-snitch-config",
						Labels: map[string]string{
							"app":                          "scylla",
							"app.kubernetes.io/managed-by": "scylla-operator",
							"app.kubernetes.io/name":       "scylla",
							"scylla/cluster":               "foo",
							"user-label":                   "user-label-value",
						},
						Annotations: map[string]string{
							"user-annotation": "user-annotation-value",
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion:         "scylla.scylladb.com/v1alpha1",
								Kind:               "ScyllaDBDatacenter",
								Name:               "foo",
								UID:                "uid-42",
								Controller:         pointer.Ptr(true),
								BlockOwnerDeletion: pointer.Ptr(true),
							},
						},
					},
					Data: map[string]string{
						"cassandra-rackdc.properties": strings.TrimLeft(`
dc=foo
rack=rack1
prefer_local=false
`, "\n"),
					},
				},
			},
			expectedErr: nil,
		},
		{
			name: "snitch datacenter name is taken from scylladbdatacenter.spec.datacenterName when it's provided",
			sdc: &scyllav1alpha1.ScyllaDBDatacenter{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo-ns",
					Name:      "foo",
					UID:       "uid-42",
					Labels: map[string]string{
						"user-label": "user-label-value",
					},
					Annotations: map[string]string{
						"user-annotation": "user-annotation-value",
					},
				},
				Spec: scyllav1alpha1.ScyllaDBDatacenterSpec{
					ClusterName:    "foo-cluster",
					DatacenterName: pointer.Ptr("dc0"),
					Racks: []scyllav1alpha1.RackSpec{
						{
							Name: "rack0",
						},
					},
				},
			},
			expectedCMs: []*corev1.ConfigMap{
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       "ConfigMap",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "foo-ns",
						Name:      "foo-rack0-snitch-config",
						Labels: map[string]string{
							"app":                          "scylla",
							"app.kubernetes.io/managed-by": "scylla-operator",
							"app.kubernetes.io/name":       "scylla",
							"scylla/cluster":               "foo",
							"user-label":                   "user-label-value",
						},
						Annotations: map[string]string{
							"user-annotation": "user-annotation-value",
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion:         "scylla.scylladb.com/v1alpha1",
								Kind:               "ScyllaDBDatacenter",
								Name:               "foo",
								UID:                "uid-42",
								Controller:         pointer.Ptr(true),
								BlockOwnerDeletion: pointer.Ptr(true),
							},
						},
					},
					Data: map[string]string{
						"cassandra-rackdc.properties": strings.TrimLeft(`
dc=dc0
rack=rack0
prefer_local=false
`, "\n"),
					},
				},
			},
			expectedErr: nil,
		},
		{
			name: "non-propagated annotations are not propagated",
			sdc: &scyllav1alpha1.ScyllaDBDatacenter{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo-ns",
					Name:      "foo",
					UID:       "uid-42",
					Labels: map[string]string{
						"user-label": "user-label-value",
					},
					Annotations: func() map[string]string {
						annotations := map[string]string{
							"user-annotation": "user-annotation-value",
						}
						if len(nonPropagatedAnnotationKeys) == 0 {
							panic("nonPropagatedAnnotationKeys can't be empty")
						}
						for _, k := range nonPropagatedAnnotationKeys {
							annotations[k] = ""
						}
						return annotations
					}(),
				},
				Spec: scyllav1alpha1.ScyllaDBDatacenterSpec{
					ClusterName: "foo-cluster",
					Racks: []scyllav1alpha1.RackSpec{
						{
							Name: "rack0",
						},
						{
							Name: "rack1",
						},
					},
				},
			},
			expectedCMs: []*corev1.ConfigMap{
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       "ConfigMap",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "foo-ns",
						Name:      "foo-rack0-snitch-config",
						Labels: map[string]string{
							"app":                          "scylla",
							"app.kubernetes.io/managed-by": "scylla-operator",
							"app.kubernetes.io/name":       "scylla",
							"scylla/cluster":               "foo",
							"user-label":                   "user-label-value",
						},
						Annotations: map[string]string{
							"user-annotation": "user-annotation-value",
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion:         "scylla.scylladb.com/v1alpha1",
								Kind:               "ScyllaDBDatacenter",
								Name:               "foo",
								UID:                "uid-42",
								Controller:         pointer.Ptr(true),
								BlockOwnerDeletion: pointer.Ptr(true),
							},
						},
					},
					Data: map[string]string{
						"cassandra-rackdc.properties": strings.TrimLeft(`
dc=foo
rack=rack0
prefer_local=false
`, "\n"),
					},
				},
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       "ConfigMap",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "foo-ns",
						Name:      "foo-rack1-snitch-config",
						Labels: map[string]string{
							"app":                          "scylla",
							"app.kubernetes.io/managed-by": "scylla-operator",
							"app.kubernetes.io/name":       "scylla",
							"scylla/cluster":               "foo",
							"user-label":                   "user-label-value",
						},
						Annotations: map[string]string{
							"user-annotation": "user-annotation-value",
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion:         "scylla.scylladb.com/v1alpha1",
								Kind:               "ScyllaDBDatacenter",
								Name:               "foo",
								UID:                "uid-42",
								Controller:         pointer.Ptr(true),
								BlockOwnerDeletion: pointer.Ptr(true),
							},
						},
					},
					Data: map[string]string{
						"cassandra-rackdc.properties": strings.TrimLeft(`
dc=foo
rack=rack1
prefer_local=false
`, "\n"),
					},
				},
			},
			expectedErr: nil,
		},
		{
			name: "non-propagated labels are not propagated",
			sdc: &scyllav1alpha1.ScyllaDBDatacenter{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo-ns",
					Name:      "foo",
					UID:       "uid-42",
					Labels: func() map[string]string {
						labels := map[string]string{
							"user-label": "user-label-value",
						}
						if len(nonPropagatedLabelKeys) == 0 {
							panic("nonPropagatedLabelKeys can't be empty")
						}
						for _, k := range nonPropagatedLabelKeys {
							labels[k] = ""
						}
						return labels
					}(),
					Annotations: map[string]string{
						"user-annotation": "user-annotation-value",
					},
				},
				Spec: scyllav1alpha1.ScyllaDBDatacenterSpec{
					ClusterName: "foo-cluster",
					Racks: []scyllav1alpha1.RackSpec{
						{
							Name: "rack0",
						},
						{
							Name: "rack1",
						},
					},
				},
			},
			expectedCMs: []*corev1.ConfigMap{
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       "ConfigMap",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "foo-ns",
						Name:      "foo-rack0-snitch-config",
						Labels: map[string]string{
							"app":                          "scylla",
							"app.kubernetes.io/managed-by": "scylla-operator",
							"app.kubernetes.io/name":       "scylla",
							"scylla/cluster":               "foo",
							"user-label":                   "user-label-value",
						},
						Annotations: map[string]string{
							"user-annotation": "user-annotation-value",
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion:         "scylla.scylladb.com/v1alpha1",
								Kind:               "ScyllaDBDatacenter",
								Name:               "foo",
								UID:                "uid-42",
								Controller:         pointer.Ptr(true),
								BlockOwnerDeletion: pointer.Ptr(true),
							},
						},
					},
					Data: map[string]string{
						"cassandra-rackdc.properties": strings.TrimLeft(`
dc=foo
rack=rack0
prefer_local=false
`, "\n"),
					},
				},
				{
					TypeMeta: metav1.TypeMeta{
						Kind:       "ConfigMap",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "foo-ns",
						Name:      "foo-rack1-snitch-config",
						Labels: map[string]string{
							"app":                          "scylla",
							"app.kubernetes.io/managed-by": "scylla-operator",
							"app.kubernetes.io/name":       "scylla",
							"scylla/cluster":               "foo",
							"user-label":                   "user-label-value",
						},
						Annotations: map[string]string{
							"user-annotation": "user-annotation-value",
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion:         "scylla.scylladb.com/v1alpha1",
								Kind:               "ScyllaDBDatacenter",
								Name:               "foo",
								UID:                "uid-42",
								Controller:         pointer.Ptr(true),
								BlockOwnerDeletion: pointer.Ptr(true),
							},
						},
					},
					Data: map[string]string{
						"cassandra-rackdc.properties": strings.TrimLeft(`
dc=foo
rack=rack1
prefer_local=false
`, "\n"),
					},
				},
			},
			expectedErr: nil,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			got, err := MakeManagedScyllaDBSnitchConfig(tc.sdc)
			if !reflect.DeepEqual(err, tc.expectedErr) {
				t.Errorf("expected and actual errors differ: %s", cmp.Diff(tc.expectedErr, err))
			}

			if !apiequality.Semantic.DeepEqual(got, tc.expectedCMs) {
				t.Errorf("expected and actual configmaps differ:\n%s", cmp.Diff(tc.expectedCMs, got))
			}
		})
	}
}

func Test_cloneMapExcludingKeysOrEmpty(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name         string
		m            map[string]string
		excludedKeys []string
		expected     map[string]string
	}{
		{
			name:         "return empty map on nil input",
			m:            nil,
			excludedKeys: []string{"key1"},
			expected:     map[string]string{},
		},
		{
			name:         "return empty map on empty input",
			m:            map[string]string{},
			excludedKeys: []string{"key1"},
			expected:     map[string]string{},
		},
		{
			name:         "return full copy on empty excluded keys",
			m:            map[string]string{"key1": "value1", "key2": "value2"},
			excludedKeys: nil,
			expected:     map[string]string{"key1": "value1", "key2": "value2"},
		},
		{
			name:         "return full copy on excluded keys not in input",
			m:            map[string]string{"key1": "value1", "key2": "value2"},
			excludedKeys: []string{"key3"},
			expected:     map[string]string{"key1": "value1", "key2": "value2"},
		},
		{
			name:         "return filtered copy on excluded keys in input",
			m:            map[string]string{"key1": "value1", "key2": "value2", "key3": "value3"},
			excludedKeys: []string{"key1", "key3"},
			expected:     map[string]string{"key2": "value2"},
		},
		{
			name:         "return empty map on all excluded keys in input",
			m:            map[string]string{"key1": "value1", "key2": "value2"},
			excludedKeys: []string{"key1", "key2"},
			expected:     map[string]string{},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := cloneMapExcludingKeysOrEmpty(tc.m, tc.excludedKeys)
			if !reflect.DeepEqual(got, tc.expected) {
				t.Errorf("expected and got differ:\n%s\n", cmp.Diff(tc.expected, got))
			}
		})
	}
}

func Test_makeScyllaDBDatacenterNodesStatusReport(t *testing.T) {
	t.Parallel()

	basicScyllaDBDatacenter := func() *scyllav1alpha1.ScyllaDBDatacenter {
		return &scyllav1alpha1.ScyllaDBDatacenter{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "basic",
				Namespace: "default",
				UID:       "uid",
				Labels: map[string]string{
					"default-sc-label": "foo",
				},
				Annotations: map[string]string{
					"default-sc-annotation": "bar",
				},
			},
			Spec: scyllav1alpha1.ScyllaDBDatacenterSpec{
				ClusterName:    "basic",
				DatacenterName: pointer.Ptr("dc"),
				Racks: []scyllav1alpha1.RackSpec{
					{
						Name: "a",
						RackTemplate: scyllav1alpha1.RackTemplate{
							Nodes: pointer.Ptr[int32](3),
						},
					},
				},
			},
		}
	}

	newPod := func(t *testing.T, name string, statusReport *internalapi.NodeStatusReport) *corev1.Pod {
		t.Helper()

		statusReportBytes, err := statusReport.Encode()
		if err != nil {
			t.Fatalf("failed to encode status report: %v", err)
		}

		return &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: "default",
				Annotations: map[string]string{
					"default-sc-annotation": "bar",
					"internal.scylla.scylladb.com/scylladb-node-status-report": string(statusReportBytes),
				},
				Labels: map[string]string{
					"default-sc-label": "foo",
				},
			},
		}
	}

	newMemberService := func(name, hostID string) *corev1.Service {
		return &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: "default",
				Annotations: map[string]string{
					"default-sc-annotation":                         "bar",
					"internal.scylla-operator.scylladb.com/host-id": hostID,
				},
				Labels: map[string]string{
					"default-sc-label": "foo",
					"scylla-operator.scylladb.com/scylla-service-type": "member",
				},
			},
		}
	}

	tt := []struct {
		name        string
		sdc         *scyllav1alpha1.ScyllaDBDatacenter
		services    map[string]*corev1.Service
		pods        []*corev1.Pod
		expected    *scyllav1alpha1.ScyllaDBDatacenterNodesStatusReport
		expectedErr error
	}{
		{
			name:     "no status, no pod and no service",
			sdc:      basicScyllaDBDatacenter(),
			services: map[string]*corev1.Service{},
			pods:     []*corev1.Pod{},
			expected: &scyllav1alpha1.ScyllaDBDatacenterNodesStatusReport{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "basic-12wmr",
					Namespace: "default",
					Labels: map[string]string{
						"default-sc-label":             "foo",
						"app":                          "scylla",
						"app.kubernetes.io/managed-by": "scylla-operator",
						"app.kubernetes.io/name":       "scylla",
						"scylla-operator.scylladb.com/scylladb-datacenter-nodes-status-report-selector": "basic",
						"scylla/cluster": "basic",
					},
					Annotations: map[string]string{
						"default-sc-annotation": "bar",
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "scylla.scylladb.com/v1alpha1",
							Kind:               "ScyllaDBDatacenter",
							Name:               "basic",
							UID:                "uid",
							Controller:         pointer.Ptr(true),
							BlockOwnerDeletion: pointer.Ptr(true),
						},
					},
				},
				DatacenterName: "dc",
				Racks: []scyllav1alpha1.RackNodesStatusReport{
					{
						Name:  "a",
						Nodes: []scyllav1alpha1.NodeStatusReport{},
					},
				},
			},
			expectedErr: nil,
		},
		{
			name: "status with no current nodes, no pod and no service",
			sdc: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sdc := basicScyllaDBDatacenter()

				sdc.Status.Racks = []scyllav1alpha1.RackStatus{
					{
						Name:  "a",
						Nodes: pointer.Ptr[int32](3),
					},
				}

				return sdc
			}(),
			services: map[string]*corev1.Service{},
			pods:     []*corev1.Pod{},
			expected: &scyllav1alpha1.ScyllaDBDatacenterNodesStatusReport{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "basic-12wmr",
					Namespace: "default",
					Labels: map[string]string{
						"default-sc-label":             "foo",
						"app":                          "scylla",
						"app.kubernetes.io/managed-by": "scylla-operator",
						"app.kubernetes.io/name":       "scylla",
						"scylla-operator.scylladb.com/scylladb-datacenter-nodes-status-report-selector": "basic",
						"scylla/cluster": "basic",
					},
					Annotations: map[string]string{
						"default-sc-annotation": "bar",
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "scylla.scylladb.com/v1alpha1",
							Kind:               "ScyllaDBDatacenter",
							Name:               "basic",
							UID:                "uid",
							Controller:         pointer.Ptr(true),
							BlockOwnerDeletion: pointer.Ptr(true),
						},
					},
				},
				DatacenterName: "dc",
				Racks: []scyllav1alpha1.RackNodesStatusReport{
					{
						Name:  "a",
						Nodes: []scyllav1alpha1.NodeStatusReport{},
					},
				},
			},
			expectedErr: nil,
		},
		{
			name: "status with zero current nodes, no pod and no service",
			sdc: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sdc := basicScyllaDBDatacenter()

				sdc.Status.Racks = []scyllav1alpha1.RackStatus{
					{
						Name:         "a",
						Nodes:        pointer.Ptr[int32](3),
						CurrentNodes: pointer.Ptr[int32](0),
					},
				}

				return sdc
			}(),
			services: map[string]*corev1.Service{},
			pods:     []*corev1.Pod{},
			expected: &scyllav1alpha1.ScyllaDBDatacenterNodesStatusReport{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "basic-12wmr",
					Namespace: "default",
					Labels: map[string]string{
						"default-sc-label":             "foo",
						"app":                          "scylla",
						"app.kubernetes.io/managed-by": "scylla-operator",
						"app.kubernetes.io/name":       "scylla",
						"scylla-operator.scylladb.com/scylladb-datacenter-nodes-status-report-selector": "basic",
						"scylla/cluster": "basic",
					},
					Annotations: map[string]string{
						"default-sc-annotation": "bar",
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "scylla.scylladb.com/v1alpha1",
							Kind:               "ScyllaDBDatacenter",
							Name:               "basic",
							UID:                "uid",
							Controller:         pointer.Ptr(true),
							BlockOwnerDeletion: pointer.Ptr(true),
						},
					},
				},
				DatacenterName: "dc",
				Racks: []scyllav1alpha1.RackNodesStatusReport{
					{
						Name:  "a",
						Nodes: []scyllav1alpha1.NodeStatusReport{},
					},
				},
			},
			expectedErr: nil,
		},
		{
			name: "status with one current node, no pod and no service",
			sdc: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sdc := basicScyllaDBDatacenter()

				sdc.Status.Racks = []scyllav1alpha1.RackStatus{
					{
						Name:         "a",
						Nodes:        pointer.Ptr[int32](3),
						CurrentNodes: pointer.Ptr[int32](1),
					},
				}

				return sdc
			}(),
			services: map[string]*corev1.Service{},
			pods:     []*corev1.Pod{},
			expected: &scyllav1alpha1.ScyllaDBDatacenterNodesStatusReport{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "basic-12wmr",
					Namespace: "default",
					Labels: map[string]string{
						"default-sc-label":             "foo",
						"app":                          "scylla",
						"app.kubernetes.io/managed-by": "scylla-operator",
						"app.kubernetes.io/name":       "scylla",
						"scylla-operator.scylladb.com/scylladb-datacenter-nodes-status-report-selector": "basic",
						"scylla/cluster": "basic",
					},
					Annotations: map[string]string{
						"default-sc-annotation": "bar",
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "scylla.scylladb.com/v1alpha1",
							Kind:               "ScyllaDBDatacenter",
							Name:               "basic",
							UID:                "uid",
							Controller:         pointer.Ptr(true),
							BlockOwnerDeletion: pointer.Ptr(true),
						},
					},
				},
				DatacenterName: "dc",
				Racks: []scyllav1alpha1.RackNodesStatusReport{
					{
						Name: "a",
						Nodes: []scyllav1alpha1.NodeStatusReport{
							{
								Ordinal: 0,
							},
						},
					},
				},
			},
			expectedErr: nil,
		},
		{
			name: "status with one current node, service with no host ID, no pod",
			sdc: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sdc := basicScyllaDBDatacenter()

				sdc.Status.Racks = []scyllav1alpha1.RackStatus{
					{
						Name:         "a",
						Nodes:        pointer.Ptr[int32](3),
						CurrentNodes: pointer.Ptr[int32](1),
					},
				}

				return sdc
			}(),
			services: map[string]*corev1.Service{
				"basic-dc-a-0": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "basic-dc-a-0",
					},
				},
			},
			pods: []*corev1.Pod{},
			expected: &scyllav1alpha1.ScyllaDBDatacenterNodesStatusReport{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "basic-12wmr",
					Namespace: "default",
					Labels: map[string]string{
						"default-sc-label":             "foo",
						"app":                          "scylla",
						"app.kubernetes.io/managed-by": "scylla-operator",
						"app.kubernetes.io/name":       "scylla",
						"scylla-operator.scylladb.com/scylladb-datacenter-nodes-status-report-selector": "basic",
						"scylla/cluster": "basic",
					},
					Annotations: map[string]string{
						"default-sc-annotation": "bar",
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "scylla.scylladb.com/v1alpha1",
							Kind:               "ScyllaDBDatacenter",
							Name:               "basic",
							UID:                "uid",
							Controller:         pointer.Ptr(true),
							BlockOwnerDeletion: pointer.Ptr(true),
						},
					},
				},
				DatacenterName: "dc",
				Racks: []scyllav1alpha1.RackNodesStatusReport{
					{
						Name: "a",
						Nodes: []scyllav1alpha1.NodeStatusReport{
							{
								Ordinal: 0,
							},
						},
					},
				},
			},
			expectedErr: nil,
		},
		{
			name: "status with one current node, service with host ID, no pod",
			sdc: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sdc := basicScyllaDBDatacenter()

				sdc.Status.Racks = []scyllav1alpha1.RackStatus{
					{
						Name:         "a",
						Nodes:        pointer.Ptr[int32](3),
						CurrentNodes: pointer.Ptr[int32](1),
					},
				}

				return sdc
			}(),
			services: map[string]*corev1.Service{
				"basic-dc-a-0": newMemberService("basic-dc-a-0", "host-id-0"),
			},
			pods: []*corev1.Pod{},
			expected: &scyllav1alpha1.ScyllaDBDatacenterNodesStatusReport{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "basic-12wmr",
					Namespace: "default",
					Labels: map[string]string{
						"default-sc-label":             "foo",
						"app":                          "scylla",
						"app.kubernetes.io/managed-by": "scylla-operator",
						"app.kubernetes.io/name":       "scylla",
						"scylla-operator.scylladb.com/scylladb-datacenter-nodes-status-report-selector": "basic",
						"scylla/cluster": "basic",
					},
					Annotations: map[string]string{
						"default-sc-annotation": "bar",
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "scylla.scylladb.com/v1alpha1",
							Kind:               "ScyllaDBDatacenter",
							Name:               "basic",
							UID:                "uid",
							Controller:         pointer.Ptr(true),
							BlockOwnerDeletion: pointer.Ptr(true),
						},
					},
				},
				DatacenterName: "dc",
				Racks: []scyllav1alpha1.RackNodesStatusReport{
					{
						Name: "a",
						Nodes: []scyllav1alpha1.NodeStatusReport{
							{
								Ordinal: 0,
								HostID:  pointer.Ptr("host-id-0"),
							},
						},
					},
				},
			},
			expectedErr: nil,
		},
		{
			name: "status with one current node, service with host ID, pod without status report",
			sdc: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sdc := basicScyllaDBDatacenter()

				sdc.Status.Racks = []scyllav1alpha1.RackStatus{
					{
						Name:         "a",
						Nodes:        pointer.Ptr[int32](3),
						CurrentNodes: pointer.Ptr[int32](1),
					},
				}

				return sdc
			}(),
			services: map[string]*corev1.Service{
				"basic-dc-a-0": newMemberService("basic-dc-a-0", "host-id-0"),
			},
			pods: []*corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "basic-dc-a-0",
						Namespace: "default",
					},
				},
			},
			expected: &scyllav1alpha1.ScyllaDBDatacenterNodesStatusReport{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "basic-12wmr",
					Namespace: "default",
					Labels: map[string]string{
						"default-sc-label":             "foo",
						"app":                          "scylla",
						"app.kubernetes.io/managed-by": "scylla-operator",
						"app.kubernetes.io/name":       "scylla",
						"scylla-operator.scylladb.com/scylladb-datacenter-nodes-status-report-selector": "basic",
						"scylla/cluster": "basic",
					},
					Annotations: map[string]string{
						"default-sc-annotation": "bar",
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "scylla.scylladb.com/v1alpha1",
							Kind:               "ScyllaDBDatacenter",
							Name:               "basic",
							UID:                "uid",
							Controller:         pointer.Ptr(true),
							BlockOwnerDeletion: pointer.Ptr(true),
						},
					},
				},
				DatacenterName: "dc",
				Racks: []scyllav1alpha1.RackNodesStatusReport{
					{
						Name: "a",
						Nodes: []scyllav1alpha1.NodeStatusReport{
							{
								Ordinal: 0,
								HostID:  pointer.Ptr("host-id-0"),
							},
						},
					},
				},
			},
			expectedErr: nil,
		},
		{
			name: "status with one current node, service with host ID, pod with status report error",
			sdc: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sdc := basicScyllaDBDatacenter()

				sdc.Status.Racks = []scyllav1alpha1.RackStatus{
					{
						Name:         "a",
						Nodes:        pointer.Ptr[int32](3),
						CurrentNodes: pointer.Ptr[int32](1),
					},
				}

				return sdc
			}(),
			services: map[string]*corev1.Service{
				"basic-dc-a-0": newMemberService("basic-dc-a-0", "host-id-0"),
			},
			pods: []*corev1.Pod{
				newPod(t, "basic-dc-a-0", &internalapi.NodeStatusReport{
					Error: pointer.Ptr("error"),
				}),
			},
			expected: &scyllav1alpha1.ScyllaDBDatacenterNodesStatusReport{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "basic-12wmr",
					Namespace: "default",
					Labels: map[string]string{
						"default-sc-label":             "foo",
						"app":                          "scylla",
						"app.kubernetes.io/managed-by": "scylla-operator",
						"app.kubernetes.io/name":       "scylla",
						"scylla-operator.scylladb.com/scylladb-datacenter-nodes-status-report-selector": "basic",
						"scylla/cluster": "basic",
					},
					Annotations: map[string]string{
						"default-sc-annotation": "bar",
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "scylla.scylladb.com/v1alpha1",
							Kind:               "ScyllaDBDatacenter",
							Name:               "basic",
							UID:                "uid",
							Controller:         pointer.Ptr(true),
							BlockOwnerDeletion: pointer.Ptr(true),
						},
					},
				},
				DatacenterName: "dc",
				Racks: []scyllav1alpha1.RackNodesStatusReport{
					{
						Name: "a",
						Nodes: []scyllav1alpha1.NodeStatusReport{
							{
								Ordinal: 0,
								HostID:  pointer.Ptr("host-id-0"),
							},
						},
					},
				},
			},
			expectedErr: nil,
		},
		{
			name: "status with one current node, service with host ID, pod with status report",
			sdc: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sdc := basicScyllaDBDatacenter()

				sdc.Status.Racks = []scyllav1alpha1.RackStatus{
					{
						Name:         "a",
						Nodes:        pointer.Ptr[int32](3),
						CurrentNodes: pointer.Ptr[int32](1),
					},
				}

				return sdc
			}(),
			services: map[string]*corev1.Service{
				"basic-dc-a-0": newMemberService("basic-dc-a-0", "host-id-0"),
			},
			pods: []*corev1.Pod{
				newPod(t, "basic-dc-a-0", &internalapi.NodeStatusReport{
					ObservedNodes: []scyllav1alpha1.ObservedNodeStatus{
						{
							HostID: "host-id-0",
							Status: scyllav1alpha1.NodeStatusUp,
						},
					},
				}),
			},
			expected: &scyllav1alpha1.ScyllaDBDatacenterNodesStatusReport{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "basic-12wmr",
					Namespace: "default",
					Labels: map[string]string{
						"default-sc-label":             "foo",
						"app":                          "scylla",
						"app.kubernetes.io/managed-by": "scylla-operator",
						"app.kubernetes.io/name":       "scylla",
						"scylla-operator.scylladb.com/scylladb-datacenter-nodes-status-report-selector": "basic",
						"scylla/cluster": "basic",
					},
					Annotations: map[string]string{
						"default-sc-annotation": "bar",
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "scylla.scylladb.com/v1alpha1",
							Kind:               "ScyllaDBDatacenter",
							Name:               "basic",
							UID:                "uid",
							Controller:         pointer.Ptr(true),
							BlockOwnerDeletion: pointer.Ptr(true),
						},
					},
				},
				DatacenterName: "dc",
				Racks: []scyllav1alpha1.RackNodesStatusReport{
					{
						Name: "a",
						Nodes: []scyllav1alpha1.NodeStatusReport{
							{
								Ordinal: 0,
								HostID:  pointer.Ptr("host-id-0"),
								ObservedNodes: []scyllav1alpha1.ObservedNodeStatus{
									{
										HostID: "host-id-0",
										Status: scyllav1alpha1.NodeStatusUp,
									},
								},
							},
						},
					},
				},
			},
			expectedErr: nil,
		},
		{
			name: "status with no current nodes, service with host ID, no pod",
			sdc: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sdc := basicScyllaDBDatacenter()

				sdc.Status.Racks = []scyllav1alpha1.RackStatus{
					{
						Name:         "a",
						Nodes:        pointer.Ptr[int32](3),
						CurrentNodes: pointer.Ptr[int32](0),
					},
				}

				return sdc
			}(),
			services: map[string]*corev1.Service{
				"basic-dc-a-0": newMemberService("basic-dc-a-0", "host-id-0"),
			},
			pods: []*corev1.Pod{},
			expected: &scyllav1alpha1.ScyllaDBDatacenterNodesStatusReport{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "basic-12wmr",
					Namespace: "default",
					Labels: map[string]string{
						"default-sc-label":             "foo",
						"app":                          "scylla",
						"app.kubernetes.io/managed-by": "scylla-operator",
						"app.kubernetes.io/name":       "scylla",
						"scylla-operator.scylladb.com/scylladb-datacenter-nodes-status-report-selector": "basic",
						"scylla/cluster": "basic",
					},
					Annotations: map[string]string{
						"default-sc-annotation": "bar",
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "scylla.scylladb.com/v1alpha1",
							Kind:               "ScyllaDBDatacenter",
							Name:               "basic",
							UID:                "uid",
							Controller:         pointer.Ptr(true),
							BlockOwnerDeletion: pointer.Ptr(true),
						},
					},
				},
				DatacenterName: "dc",
				Racks: []scyllav1alpha1.RackNodesStatusReport{
					{
						Name: "a",
						Nodes: []scyllav1alpha1.NodeStatusReport{
							{
								Ordinal: 0,
								HostID:  pointer.Ptr("host-id-0"),
							},
						},
					},
				},
			},
			expectedErr: nil,
		},
		{
			name: "status with one current node, service without host ID and pod with status report",
			sdc: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sdc := basicScyllaDBDatacenter()

				sdc.Status.Racks = []scyllav1alpha1.RackStatus{
					{
						Name:         "a",
						Nodes:        pointer.Ptr[int32](3),
						CurrentNodes: pointer.Ptr[int32](1),
					},
				}

				return sdc
			}(),
			services: map[string]*corev1.Service{
				"basic-dc-a-0": {
					ObjectMeta: metav1.ObjectMeta{
						Name:      "basic-dc-a-0",
						Namespace: "default",
						Annotations: map[string]string{
							"default-sc-annotation": "bar",
						},
						Labels: map[string]string{
							"default-sc-label": "foo",
							"scylla-operator.scylladb.com/scylla-service-type": "member",
						},
					},
				},
			},
			pods: []*corev1.Pod{
				newPod(t, "basic-dc-a-0", &internalapi.NodeStatusReport{
					ObservedNodes: []scyllav1alpha1.ObservedNodeStatus{
						{
							HostID: "host-id-0",
							Status: scyllav1alpha1.NodeStatusUp,
						},
					},
				}),
			},
			expected: &scyllav1alpha1.ScyllaDBDatacenterNodesStatusReport{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "basic-12wmr",
					Namespace: "default",
					Labels: map[string]string{
						"default-sc-label":             "foo",
						"app":                          "scylla",
						"app.kubernetes.io/managed-by": "scylla-operator",
						"app.kubernetes.io/name":       "scylla",
						"scylla-operator.scylladb.com/scylladb-datacenter-nodes-status-report-selector": "basic",
						"scylla/cluster": "basic",
					},
					Annotations: map[string]string{
						"default-sc-annotation": "bar",
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "scylla.scylladb.com/v1alpha1",
							Kind:               "ScyllaDBDatacenter",
							Name:               "basic",
							UID:                "uid",
							Controller:         pointer.Ptr(true),
							BlockOwnerDeletion: pointer.Ptr(true),
						},
					},
				},
				DatacenterName: "dc",
				Racks: []scyllav1alpha1.RackNodesStatusReport{
					{
						Name: "a",
						Nodes: []scyllav1alpha1.NodeStatusReport{
							{
								Ordinal: 0,
							},
						},
					},
				},
			},
			expectedErr: nil,
		},
		{
			name: "status with two current nodes, one service with host ID and one pod with status report",
			sdc: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sdc := basicScyllaDBDatacenter()

				sdc.Status.Racks = []scyllav1alpha1.RackStatus{
					{
						Name:         "a",
						Nodes:        pointer.Ptr[int32](3),
						CurrentNodes: pointer.Ptr[int32](2),
					},
				}

				return sdc
			}(),
			services: map[string]*corev1.Service{
				"basic-dc-a-0": newMemberService("basic-dc-a-0", "host-id-0"),
			},
			pods: []*corev1.Pod{
				newPod(t, "basic-dc-a-0", &internalapi.NodeStatusReport{
					ObservedNodes: []scyllav1alpha1.ObservedNodeStatus{
						{
							HostID: "host-id-0",
							Status: scyllav1alpha1.NodeStatusUp,
						},
					},
				}),
			},
			expected: &scyllav1alpha1.ScyllaDBDatacenterNodesStatusReport{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "basic-12wmr",
					Namespace: "default",
					Labels: map[string]string{
						"default-sc-label":             "foo",
						"app":                          "scylla",
						"app.kubernetes.io/managed-by": "scylla-operator",
						"app.kubernetes.io/name":       "scylla",
						"scylla-operator.scylladb.com/scylladb-datacenter-nodes-status-report-selector": "basic",
						"scylla/cluster": "basic",
					},
					Annotations: map[string]string{
						"default-sc-annotation": "bar",
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "scylla.scylladb.com/v1alpha1",
							Kind:               "ScyllaDBDatacenter",
							Name:               "basic",
							UID:                "uid",
							Controller:         pointer.Ptr(true),
							BlockOwnerDeletion: pointer.Ptr(true),
						},
					},
				},
				DatacenterName: "dc",
				Racks: []scyllav1alpha1.RackNodesStatusReport{
					{
						Name: "a",
						Nodes: []scyllav1alpha1.NodeStatusReport{
							{
								Ordinal: 0,
								HostID:  pointer.Ptr("host-id-0"),
								ObservedNodes: []scyllav1alpha1.ObservedNodeStatus{
									{
										HostID: "host-id-0",
										Status: scyllav1alpha1.NodeStatusUp,
									},
								},
							},
							{
								Ordinal: 1,
							},
						},
					},
				},
			},
			expectedErr: nil,
		},
		{
			name: "status with three current nodes, all services with host IDs and all pods with status reports",
			sdc: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sdc := basicScyllaDBDatacenter()

				sdc.Status.Racks = []scyllav1alpha1.RackStatus{
					{
						Name:         "a",
						Nodes:        pointer.Ptr[int32](3),
						CurrentNodes: pointer.Ptr[int32](3),
					},
				}

				return sdc
			}(),
			services: map[string]*corev1.Service{
				"basic-dc-a-0": newMemberService("basic-dc-a-0", "host-id-0"),
				"basic-dc-a-1": newMemberService("basic-dc-a-1", "host-id-1"),
				"basic-dc-a-2": newMemberService("basic-dc-a-2", "host-id-2"),
			},
			pods: []*corev1.Pod{
				newPod(t, "basic-dc-a-0", &internalapi.NodeStatusReport{
					ObservedNodes: []scyllav1alpha1.ObservedNodeStatus{
						{
							HostID: "host-id-0",
							Status: scyllav1alpha1.NodeStatusUp,
						},
						{
							HostID: "host-id-1",
							Status: scyllav1alpha1.NodeStatusUp,
						},
						{
							HostID: "host-id-2",
							Status: scyllav1alpha1.NodeStatusUp,
						},
					},
				}),
				newPod(t, "basic-dc-a-1", &internalapi.NodeStatusReport{
					ObservedNodes: []scyllav1alpha1.ObservedNodeStatus{
						{
							HostID: "host-id-0",
							Status: scyllav1alpha1.NodeStatusUp,
						},
						{
							HostID: "host-id-1",
							Status: scyllav1alpha1.NodeStatusUp,
						},
						{
							HostID: "host-id-2",
							Status: scyllav1alpha1.NodeStatusUp,
						},
					},
				}),
				newPod(t, "basic-dc-a-2", &internalapi.NodeStatusReport{
					ObservedNodes: []scyllav1alpha1.ObservedNodeStatus{
						{
							HostID: "host-id-0",
							Status: scyllav1alpha1.NodeStatusUp,
						},
						{
							HostID: "host-id-1",
							Status: scyllav1alpha1.NodeStatusUp,
						},
						{
							HostID: "host-id-2",
							Status: scyllav1alpha1.NodeStatusUp,
						},
					},
				}),
			},
			expected: &scyllav1alpha1.ScyllaDBDatacenterNodesStatusReport{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "basic-12wmr",
					Namespace: "default",
					Labels: map[string]string{
						"default-sc-label":             "foo",
						"app":                          "scylla",
						"app.kubernetes.io/managed-by": "scylla-operator",
						"app.kubernetes.io/name":       "scylla",
						"scylla-operator.scylladb.com/scylladb-datacenter-nodes-status-report-selector": "basic",
						"scylla/cluster": "basic",
					},
					Annotations: map[string]string{
						"default-sc-annotation": "bar",
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "scylla.scylladb.com/v1alpha1",
							Kind:               "ScyllaDBDatacenter",
							Name:               "basic",
							UID:                "uid",
							Controller:         pointer.Ptr(true),
							BlockOwnerDeletion: pointer.Ptr(true),
						},
					},
				},
				DatacenterName: "dc",
				Racks: []scyllav1alpha1.RackNodesStatusReport{
					{
						Name: "a",
						Nodes: []scyllav1alpha1.NodeStatusReport{
							{
								Ordinal: 0,
								HostID:  pointer.Ptr("host-id-0"),
								ObservedNodes: []scyllav1alpha1.ObservedNodeStatus{
									{
										HostID: "host-id-0",
										Status: scyllav1alpha1.NodeStatusUp,
									},
									{
										HostID: "host-id-1",
										Status: scyllav1alpha1.NodeStatusUp,
									},
									{
										HostID: "host-id-2",
										Status: scyllav1alpha1.NodeStatusUp,
									},
								},
							},
							{
								Ordinal: 1,
								HostID:  pointer.Ptr("host-id-1"),
								ObservedNodes: []scyllav1alpha1.ObservedNodeStatus{
									{
										HostID: "host-id-0",
										Status: scyllav1alpha1.NodeStatusUp,
									},
									{
										HostID: "host-id-1",
										Status: scyllav1alpha1.NodeStatusUp,
									},
									{
										HostID: "host-id-2",
										Status: scyllav1alpha1.NodeStatusUp,
									},
								},
							},
							{
								Ordinal: 2,
								HostID:  pointer.Ptr("host-id-2"),
								ObservedNodes: []scyllav1alpha1.ObservedNodeStatus{
									{
										HostID: "host-id-0",
										Status: scyllav1alpha1.NodeStatusUp,
									},
									{
										HostID: "host-id-1",
										Status: scyllav1alpha1.NodeStatusUp,
									},
									{
										HostID: "host-id-2",
										Status: scyllav1alpha1.NodeStatusUp,
									},
								},
							},
						},
					},
				},
			},
			expectedErr: nil,
		},
		{
			name: "status with multiple racks, all services with host IDs and all pods with status reports",
			sdc: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sdc := basicScyllaDBDatacenter()

				sdc.Spec.Racks = []scyllav1alpha1.RackSpec{
					{
						Name: "a",
						RackTemplate: scyllav1alpha1.RackTemplate{
							Nodes: pointer.Ptr[int32](2),
						},
					},
					{
						Name: "b",
						RackTemplate: scyllav1alpha1.RackTemplate{
							Nodes: pointer.Ptr[int32](2),
						},
					},
				}

				sdc.Status.Racks = []scyllav1alpha1.RackStatus{
					{
						Name:         "a",
						Nodes:        pointer.Ptr[int32](2),
						CurrentNodes: pointer.Ptr[int32](2),
					},
					{
						Name:         "b",
						Nodes:        pointer.Ptr[int32](2),
						CurrentNodes: pointer.Ptr[int32](2),
					},
				}

				return sdc
			}(),
			services: map[string]*corev1.Service{
				"basic-dc-a-0": newMemberService("basic-dc-a-0", "host-id-a-0"),
				"basic-dc-a-1": newMemberService("basic-dc-a-1", "host-id-a-1"),
				"basic-dc-b-0": newMemberService("basic-dc-b-0", "host-id-b-0"),
				"basic-dc-b-1": newMemberService("basic-dc-b-1", "host-id-b-1"),
			},
			pods: []*corev1.Pod{
				newPod(t, "basic-dc-a-0", &internalapi.NodeStatusReport{
					ObservedNodes: []scyllav1alpha1.ObservedNodeStatus{
						{
							HostID: "host-id-a-0",
							Status: scyllav1alpha1.NodeStatusUp,
						},
						{
							HostID: "host-id-a-1",
							Status: scyllav1alpha1.NodeStatusUp,
						},
						{
							HostID: "host-id-b-0",
							Status: scyllav1alpha1.NodeStatusUp,
						},
						{
							HostID: "host-id-b-1",
							Status: scyllav1alpha1.NodeStatusUp,
						},
					},
				}),
				newPod(t, "basic-dc-a-1", &internalapi.NodeStatusReport{
					ObservedNodes: []scyllav1alpha1.ObservedNodeStatus{
						{
							HostID: "host-id-a-0",
							Status: scyllav1alpha1.NodeStatusUp,
						},
						{
							HostID: "host-id-a-1",
							Status: scyllav1alpha1.NodeStatusUp,
						},
						{
							HostID: "host-id-b-0",
							Status: scyllav1alpha1.NodeStatusUp,
						},
						{
							HostID: "host-id-b-1",
							Status: scyllav1alpha1.NodeStatusUp,
						},
					},
				}),
				newPod(t, "basic-dc-b-0", &internalapi.NodeStatusReport{
					ObservedNodes: []scyllav1alpha1.ObservedNodeStatus{
						{
							HostID: "host-id-a-0",
							Status: scyllav1alpha1.NodeStatusUp,
						},
						{
							HostID: "host-id-a-1",
							Status: scyllav1alpha1.NodeStatusUp,
						},
						{
							HostID: "host-id-b-0",
							Status: scyllav1alpha1.NodeStatusUp,
						},
						{
							HostID: "host-id-b-1",
							Status: scyllav1alpha1.NodeStatusUp,
						},
					},
				}),
				newPod(t, "basic-dc-b-1", &internalapi.NodeStatusReport{
					ObservedNodes: []scyllav1alpha1.ObservedNodeStatus{
						{
							HostID: "host-id-a-0",
							Status: scyllav1alpha1.NodeStatusUp,
						},
						{
							HostID: "host-id-a-1",
							Status: scyllav1alpha1.NodeStatusUp,
						},
						{
							HostID: "host-id-b-0",
							Status: scyllav1alpha1.NodeStatusUp,
						},
						{
							HostID: "host-id-b-1",
							Status: scyllav1alpha1.NodeStatusUp,
						},
					},
				}),
			},
			expected: &scyllav1alpha1.ScyllaDBDatacenterNodesStatusReport{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "basic-12wmr",
					Namespace: "default",
					Labels: map[string]string{
						"default-sc-label":             "foo",
						"app":                          "scylla",
						"app.kubernetes.io/managed-by": "scylla-operator",
						"app.kubernetes.io/name":       "scylla",
						"scylla-operator.scylladb.com/scylladb-datacenter-nodes-status-report-selector": "basic",
						"scylla/cluster": "basic",
					},
					Annotations: map[string]string{
						"default-sc-annotation": "bar",
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "scylla.scylladb.com/v1alpha1",
							Kind:               "ScyllaDBDatacenter",
							Name:               "basic",
							UID:                "uid",
							Controller:         pointer.Ptr(true),
							BlockOwnerDeletion: pointer.Ptr(true),
						},
					},
				},
				DatacenterName: "dc",
				Racks: []scyllav1alpha1.RackNodesStatusReport{
					{
						Name: "a",
						Nodes: []scyllav1alpha1.NodeStatusReport{
							{
								Ordinal: 0,
								HostID:  pointer.Ptr("host-id-a-0"),
								ObservedNodes: []scyllav1alpha1.ObservedNodeStatus{
									{
										HostID: "host-id-a-0",
										Status: scyllav1alpha1.NodeStatusUp,
									},
									{
										HostID: "host-id-a-1",
										Status: scyllav1alpha1.NodeStatusUp,
									},
									{
										HostID: "host-id-b-0",
										Status: scyllav1alpha1.NodeStatusUp,
									},
									{
										HostID: "host-id-b-1",
										Status: scyllav1alpha1.NodeStatusUp,
									},
								},
							},
							{
								Ordinal: 1,
								HostID:  pointer.Ptr("host-id-a-1"),
								ObservedNodes: []scyllav1alpha1.ObservedNodeStatus{
									{
										HostID: "host-id-a-0",
										Status: scyllav1alpha1.NodeStatusUp,
									},
									{
										HostID: "host-id-a-1",
										Status: scyllav1alpha1.NodeStatusUp,
									},
									{
										HostID: "host-id-b-0",
										Status: scyllav1alpha1.NodeStatusUp,
									},
									{
										HostID: "host-id-b-1",
										Status: scyllav1alpha1.NodeStatusUp,
									},
								},
							},
						},
					},
					{
						Name: "b",
						Nodes: []scyllav1alpha1.NodeStatusReport{
							{
								Ordinal: 0,
								HostID:  pointer.Ptr("host-id-b-0"),
								ObservedNodes: []scyllav1alpha1.ObservedNodeStatus{
									{
										HostID: "host-id-a-0",
										Status: scyllav1alpha1.NodeStatusUp,
									},
									{
										HostID: "host-id-a-1",
										Status: scyllav1alpha1.NodeStatusUp,
									},
									{
										HostID: "host-id-b-0",
										Status: scyllav1alpha1.NodeStatusUp,
									},
									{
										HostID: "host-id-b-1",
										Status: scyllav1alpha1.NodeStatusUp,
									},
								},
							},
							{
								Ordinal: 1,
								HostID:  pointer.Ptr("host-id-b-1"),
								ObservedNodes: []scyllav1alpha1.ObservedNodeStatus{
									{
										HostID: "host-id-a-0",
										Status: scyllav1alpha1.NodeStatusUp,
									},
									{
										HostID: "host-id-a-1",
										Status: scyllav1alpha1.NodeStatusUp,
									},
									{
										HostID: "host-id-b-0",
										Status: scyllav1alpha1.NodeStatusUp,
									},
									{
										HostID: "host-id-b-1",
										Status: scyllav1alpha1.NodeStatusUp,
									},
								},
							},
						},
					},
				},
			},
			expectedErr: nil,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			podCache := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
			for _, obj := range tc.pods {
				err := podCache.Add(obj)
				if err != nil {
					t.Fatal(err)
				}
			}

			podLister := corev1listers.NewPodLister(podCache)

			got, err := makeScyllaDBDatacenterNodesStatusReport(tc.sdc, tc.services, podLister)
			if !reflect.DeepEqual(err, tc.expectedErr) {
				t.Fatalf("expected and actual errors differ: %s", cmp.Diff(tc.expectedErr, err, cmpopts.EquateErrors()))
			}

			if !apiequality.Semantic.DeepEqual(got, tc.expected) {
				t.Errorf("expected and actual objects differ:\n%s", cmp.Diff(tc.expected, got))
			}
		})
	}
}

func logEnabledFeatures(t *testing.T) {
	t.Helper()

	fs := slices.Collect(maps.Keys(utilfeature.DefaultMutableFeatureGate.GetAll()))
	t.Logf("Running TestStatefulSetForRack with features enabled: %s", strings.Join(oslices.ConvertSlice(fs, func(f featuregate.Feature) string {
		return fmt.Sprintf("%s=%t", f, utilfeature.DefaultMutableFeatureGate.Enabled(f))
	}), ","))
}
