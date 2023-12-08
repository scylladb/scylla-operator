// Copyright (c) 2024 ScyllaDB.

package controllerhelpers

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_ConvertEndpointSlicesToEndpoints(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name              string
		endpointSlices    []*discoveryv1.EndpointSlice
		expectedEndpoints []*corev1.Endpoints
	}{
		{
			name: "single EndpointSlice per Service",
			endpointSlices: []*discoveryv1.EndpointSlice{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "basic-dc-rack-0-12345678",
						Namespace: "default",
						Labels: map[string]string{
							"foo":                                    "bar",
							"kubernetes.io/service-name":             "basic-dc-rack-0",
							"endpointslice.kubernetes.io/managed-by": "scylla-operator.scylladb.com/scylla-operator",
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion:         "scylla.scylladb.com/v1",
								Kind:               "ScyllaCluster",
								Name:               "basic",
								UID:                "the-uid",
								Controller:         pointer.Ptr(true),
								BlockOwnerDeletion: pointer.Ptr(true),
							},
						},
					},
					AddressType: discoveryv1.AddressTypeIPv4,
					Endpoints: []discoveryv1.Endpoint{
						{
							Addresses: []string{"1.2.3.4"},
							Conditions: discoveryv1.EndpointConditions{
								Ready:       pointer.Ptr(true),
								Serving:     pointer.Ptr(true),
								Terminating: pointer.Ptr(false),
							},
							TargetRef: &corev1.ObjectReference{
								Kind:      "Pod",
								Namespace: "default",
								Name:      "basic-dc-rack-0",
								UID:       "pod-uid",
							},
							NodeName: pointer.Ptr("node-a"),
						},
					},
					Ports: []discoveryv1.EndpointPort{
						{
							Name:     pointer.Ptr("port-1"),
							Protocol: pointer.Ptr(corev1.ProtocolTCP),
							Port:     pointer.Ptr(int32(777)),
						},
					},
				},
			},
			expectedEndpoints: []*corev1.Endpoints{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "basic-dc-rack-0",
						Namespace: "default",
						Labels: map[string]string{
							"foo":                                     "bar",
							"kubernetes.io/service-name":              "basic-dc-rack-0",
							"endpointslice.kubernetes.io/managed-by":  "scylla-operator.scylladb.com/scylla-operator",
							"endpointslice.kubernetes.io/skip-mirror": "true",
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion:         "scylla.scylladb.com/v1",
								Kind:               "ScyllaCluster",
								Name:               "basic",
								UID:                "the-uid",
								Controller:         pointer.Ptr(true),
								BlockOwnerDeletion: pointer.Ptr(true),
							},
						},
					},
					Subsets: []corev1.EndpointSubset{
						{
							Addresses: []corev1.EndpointAddress{
								{
									IP: "1.2.3.4",
									TargetRef: &corev1.ObjectReference{
										Kind:      "Pod",
										Namespace: "default",
										Name:      "basic-dc-rack-0",
										UID:       "pod-uid",
									},
									NodeName: pointer.Ptr("node-a"),
								},
							},
							NotReadyAddresses: nil,
							Ports: []corev1.EndpointPort{
								{
									Name:     "port-1",
									Port:     777,
									Protocol: "TCP",
								},
							},
						},
					},
				},
			},
		},
		{
			name: "multiple EndpointSlices per Service",
			endpointSlices: []*discoveryv1.EndpointSlice{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "basic-dc-rack-0-abc",
						Namespace: "default",
						Labels: map[string]string{
							"foo":                                    "bar",
							"kubernetes.io/service-name":             "basic-dc-rack-0",
							"endpointslice.kubernetes.io/managed-by": "scylla-operator.scylladb.com/scylla-operator",
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion:         "scylla.scylladb.com/v1",
								Kind:               "ScyllaCluster",
								Name:               "basic",
								UID:                "the-uid",
								Controller:         pointer.Ptr(true),
								BlockOwnerDeletion: pointer.Ptr(true),
							},
						},
					},
					AddressType: discoveryv1.AddressTypeIPv4,
					Endpoints: []discoveryv1.Endpoint{
						{
							Addresses: []string{"1.2.3.4"},
							Conditions: discoveryv1.EndpointConditions{
								Ready:       pointer.Ptr(true),
								Serving:     pointer.Ptr(true),
								Terminating: pointer.Ptr(false),
							},
							TargetRef: &corev1.ObjectReference{
								Kind:      "Pod",
								Namespace: "default",
								Name:      "basic-dc-rack-0",
								UID:       "pod-uid",
							},
							NodeName: pointer.Ptr("node-a"),
						},
					},
					Ports: []discoveryv1.EndpointPort{
						{
							Name:     pointer.Ptr("port-1"),
							Protocol: pointer.Ptr(corev1.ProtocolTCP),
							Port:     pointer.Ptr(int32(666)),
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "basic-dc-rack-0-def",
						Namespace: "default",
						Labels: map[string]string{
							"foo":                                    "bar",
							"kubernetes.io/service-name":             "basic-dc-rack-0",
							"endpointslice.kubernetes.io/managed-by": "scylla-operator.scylladb.com/scylla-operator",
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion:         "scylla.scylladb.com/v1",
								Kind:               "ScyllaCluster",
								Name:               "basic",
								UID:                "the-uid",
								Controller:         pointer.Ptr(true),
								BlockOwnerDeletion: pointer.Ptr(true),
							},
						},
					},
					AddressType: discoveryv1.AddressTypeIPv4,
					Endpoints: []discoveryv1.Endpoint{
						{
							Addresses: []string{"1.2.3.4"},
							Conditions: discoveryv1.EndpointConditions{
								Ready:       pointer.Ptr(false),
								Serving:     pointer.Ptr(false),
								Terminating: pointer.Ptr(false),
							},
							TargetRef: &corev1.ObjectReference{
								Kind:      "Pod",
								Namespace: "default",
								Name:      "basic-dc-rack-0",
								UID:       "pod-uid",
							},
							NodeName: pointer.Ptr("node-a"),
						},
					},
					Ports: []discoveryv1.EndpointPort{
						{
							Name:     pointer.Ptr("port-2"),
							Protocol: pointer.Ptr(corev1.ProtocolTCP),
							Port:     pointer.Ptr(int32(777)),
						},
					},
				},
			},
			expectedEndpoints: []*corev1.Endpoints{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "basic-dc-rack-0",
						Namespace: "default",
						Labels: map[string]string{
							"foo":                                     "bar",
							"kubernetes.io/service-name":              "basic-dc-rack-0",
							"endpointslice.kubernetes.io/managed-by":  "scylla-operator.scylladb.com/scylla-operator",
							"endpointslice.kubernetes.io/skip-mirror": "true",
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion:         "scylla.scylladb.com/v1",
								Kind:               "ScyllaCluster",
								Name:               "basic",
								UID:                "the-uid",
								Controller:         pointer.Ptr(true),
								BlockOwnerDeletion: pointer.Ptr(true),
							},
						},
					},
					Subsets: []corev1.EndpointSubset{
						{
							Addresses: []corev1.EndpointAddress{
								{
									IP: "1.2.3.4",
									TargetRef: &corev1.ObjectReference{
										Kind:      "Pod",
										Namespace: "default",
										Name:      "basic-dc-rack-0",
										UID:       "pod-uid",
									},
									NodeName: pointer.Ptr("node-a"),
								},
							},
							NotReadyAddresses: nil,
							Ports: []corev1.EndpointPort{
								{
									Name:     "port-1",
									Port:     666,
									Protocol: "TCP",
								},
							},
						},
						{
							Addresses: nil,
							NotReadyAddresses: []corev1.EndpointAddress{
								{
									IP: "1.2.3.4",
									TargetRef: &corev1.ObjectReference{
										Kind:      "Pod",
										Namespace: "default",
										Name:      "basic-dc-rack-0",
										UID:       "pod-uid",
									},
									NodeName: pointer.Ptr("node-a"),
								},
							},
							Ports: []corev1.EndpointPort{
								{
									Name:     "port-2",
									Port:     777,
									Protocol: "TCP",
								},
							},
						},
					},
				},
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			endpoints, err := ConvertEndpointSlicesToEndpoints(tc.endpointSlices)
			if err != nil {
				t.Fatal(err)
			}
			if !apiequality.Semantic.DeepEqual(endpoints, tc.expectedEndpoints) {
				t.Errorf("expected and actual Endpoints differ: %s", cmp.Diff(tc.expectedEndpoints, endpoints))
			}
		})
	}
}
