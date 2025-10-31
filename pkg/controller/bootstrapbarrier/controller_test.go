// Copyright (C) 2025 ScyllaDB

package bootstrapbarrier

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_shouldProceedWithBootstrap(t *testing.T) {
	t.Parallel()

	newService := func() *corev1.Service {
		return &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "basic-dc1-rack1-0",
				Namespace: "default",
				Labels: map[string]string{
					naming.DatacenterNameLabel: "dc1",
					naming.RackNameLabel:       "rack1",
				},
				Annotations: map[string]string{},
			},
			Spec: corev1.ServiceSpec{},
		}
	}

	trueMockIsBootstrapPreconditionSatisfied := func(_ []*scyllav1alpha1.ScyllaDBDatacenterNodesStatusReport, _, _ string, _ int) bool {
		return true
	}

	falseMockIsBootstrapPreconditionSatisfied := func(_ []*scyllav1alpha1.ScyllaDBDatacenterNodesStatusReport, _, _ string, _ int) bool {
		return false
	}

	tt := []struct {
		name                                 string
		service                              *corev1.Service
		scyllaDBDatacenterNodesStatusReports []*scyllav1alpha1.ScyllaDBDatacenterNodesStatusReport
		isBootstrapPreconditionSatisfied     func([]*scyllav1alpha1.ScyllaDBDatacenterNodesStatusReport, string, string, int) bool
		expected                             bool
		expectedErrorString                  string
	}{
		{
			name: "service is missing datacenter label",
			service: func() *corev1.Service {
				return &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "basic-dc1-rack1-0",
						Namespace: "default",
						Labels: map[string]string{
							naming.RackNameLabel: "rack1",
						},
						Annotations: map[string]string{},
					},
					Spec: corev1.ServiceSpec{},
				}
			}(),
			scyllaDBDatacenterNodesStatusReports: []*scyllav1alpha1.ScyllaDBDatacenterNodesStatusReport{},
			isBootstrapPreconditionSatisfied:     trueMockIsBootstrapPreconditionSatisfied,
			expected:                             false,
			expectedErrorString:                  `service "default/basic-dc1-rack1-0" is missing label "scylla/datacenter"`,
		},
		{
			name: "service is missing rack label",
			service: func() *corev1.Service {
				return &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "basic-dc1-rack1-0",
						Namespace: "default",
						Labels: map[string]string{
							naming.DatacenterNameLabel: "dc1",
						},
						Annotations: map[string]string{},
					},
					Spec: corev1.ServiceSpec{},
				}
			}(),
			scyllaDBDatacenterNodesStatusReports: []*scyllav1alpha1.ScyllaDBDatacenterNodesStatusReport{},
			isBootstrapPreconditionSatisfied:     trueMockIsBootstrapPreconditionSatisfied,
			expected:                             false,
			expectedErrorString:                  `service "default/basic-dc1-rack1-0" is missing label "scylla/rack"`,
		},
		{
			name: "service name is invalid for ordinal extraction",
			service: func() *corev1.Service {
				svc := newService()
				svc.Name = "basic-dc1-rack1-ordinal"
				return svc
			}(),
			scyllaDBDatacenterNodesStatusReports: []*scyllav1alpha1.ScyllaDBDatacenterNodesStatusReport{},
			isBootstrapPreconditionSatisfied:     trueMockIsBootstrapPreconditionSatisfied,
			expected:                             false,
			expectedErrorString:                  `can't get ordinal from name of service "default/basic-dc1-rack1-ordinal": couldn't convert 'ordinal' to a number`,
		},
		{
			name: "service with force proceed to bootstrap annotation set to true",
			service: func() *corev1.Service {
				svc := newService()
				svc.Annotations[naming.ForceProceedToBootstrapAnnotation] = "true"
				return svc
			}(),
			scyllaDBDatacenterNodesStatusReports: []*scyllav1alpha1.ScyllaDBDatacenterNodesStatusReport{},
			isBootstrapPreconditionSatisfied:     falseMockIsBootstrapPreconditionSatisfied,
			expected:                             true,
			expectedErrorString:                  "",
		},
		{
			name: "service with force proceed to bootstrap annotation set to an unsupported value",
			service: func() *corev1.Service {
				svc := newService()
				svc.Annotations[naming.ForceProceedToBootstrapAnnotation] = ""
				return svc
			}(),
			scyllaDBDatacenterNodesStatusReports: []*scyllav1alpha1.ScyllaDBDatacenterNodesStatusReport{},
			isBootstrapPreconditionSatisfied:     falseMockIsBootstrapPreconditionSatisfied,
			expected:                             false,
			expectedErrorString:                  `service "default/basic-dc1-rack1-0" has an unsupported value for annotation "scylla-operator.scylladb.com/force-proceed-to-bootstrap": ""`,
		},
		{
			name: "service with node replacement label",
			service: func() *corev1.Service {
				svc := newService()
				svc.Labels[naming.ReplacingNodeHostIDLabel] = "host-id"
				return svc
			}(),
			scyllaDBDatacenterNodesStatusReports: []*scyllav1alpha1.ScyllaDBDatacenterNodesStatusReport{},
			isBootstrapPreconditionSatisfied:     falseMockIsBootstrapPreconditionSatisfied,
			expected:                             true,
			expectedErrorString:                  "",
		},
		{
			name:                                 "bootstrap precondition satisfied",
			service:                              newService(),
			scyllaDBDatacenterNodesStatusReports: []*scyllav1alpha1.ScyllaDBDatacenterNodesStatusReport{},
			isBootstrapPreconditionSatisfied:     trueMockIsBootstrapPreconditionSatisfied,
			expected:                             true,
			expectedErrorString:                  "",
		},
		{
			name:                                 "bootstrap precondition not satisfied",
			service:                              newService(),
			scyllaDBDatacenterNodesStatusReports: []*scyllav1alpha1.ScyllaDBDatacenterNodesStatusReport{},
			isBootstrapPreconditionSatisfied:     falseMockIsBootstrapPreconditionSatisfied,
			expected:                             false,
			expectedErrorString:                  "",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := shouldProceedWithBootstrap(tc.service, tc.scyllaDBDatacenterNodesStatusReports, tc.isBootstrapPreconditionSatisfied)

			errStr := ""
			if err != nil {
				errStr = err.Error()
			}
			if !cmp.Equal(errStr, tc.expectedErrorString) {
				t.Fatalf("expected and got error strings differ: %s", cmp.Diff(tc.expectedErrorString, errStr))
			}

			if got != tc.expected {
				t.Errorf("expected %v, got %v", tc.expected, got)
			}
		})
	}
}

func Test_isBootstrapPreconditionSatisfied(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name                                 string
		scyllaDBDatacenterNodesStatusReports []*scyllav1alpha1.ScyllaDBDatacenterNodesStatusReport
		selfDC                               string
		selfRack                             string
		selfOrdinal                          int
		singleReportAllowNonReportingHostIDs bool
		expected                             bool
	}{
		{
			name:                                 "no reports",
			scyllaDBDatacenterNodesStatusReports: []*scyllav1alpha1.ScyllaDBDatacenterNodesStatusReport{},
			selfDC:                               "dc1",
			selfRack:                             "rack1",
			selfOrdinal:                          0,
			singleReportAllowNonReportingHostIDs: false,
			expected:                             true,
		},
		{
			name: "single report, all nodes up",
			scyllaDBDatacenterNodesStatusReports: []*scyllav1alpha1.ScyllaDBDatacenterNodesStatusReport{
				{
					DatacenterName: "dc1",
					Racks: []scyllav1alpha1.RackNodesStatusReport{
						{
							Name: "rack1",
							Nodes: []scyllav1alpha1.NodeStatusReport{
								{
									Ordinal: 0,
									HostID:  pointer.Ptr("host0"),
									ObservedNodes: []scyllav1alpha1.ObservedNodeStatus{
										{HostID: "host0", Status: scyllav1alpha1.NodeStatusUp},
										{HostID: "host1", Status: scyllav1alpha1.NodeStatusUp},
									},
								},
								{
									Ordinal: 1,
									HostID:  pointer.Ptr("host1"),
									ObservedNodes: []scyllav1alpha1.ObservedNodeStatus{
										{HostID: "host0", Status: scyllav1alpha1.NodeStatusUp},
										{HostID: "host1", Status: scyllav1alpha1.NodeStatusUp},
									},
								},
							},
						},
					},
				},
			},
			selfDC:                               "dc1",
			selfRack:                             "rack1",
			selfOrdinal:                          2,
			singleReportAllowNonReportingHostIDs: false,
			expected:                             true,
		},
		{
			name: "single report, a node is down",
			scyllaDBDatacenterNodesStatusReports: []*scyllav1alpha1.ScyllaDBDatacenterNodesStatusReport{
				{
					DatacenterName: "dc1",
					Racks: []scyllav1alpha1.RackNodesStatusReport{
						{
							Name: "rack1",
							Nodes: []scyllav1alpha1.NodeStatusReport{
								{
									Ordinal: 0,
									HostID:  pointer.Ptr("host0"),
									ObservedNodes: []scyllav1alpha1.ObservedNodeStatus{
										{HostID: "host0", Status: scyllav1alpha1.NodeStatusUp},
										{HostID: "host1", Status: scyllav1alpha1.NodeStatusDown},
									},
								},
								{
									Ordinal: 1,
									HostID:  pointer.Ptr("host1"),
									ObservedNodes: []scyllav1alpha1.ObservedNodeStatus{
										{HostID: "host0", Status: scyllav1alpha1.NodeStatusUp},
										{HostID: "host1", Status: scyllav1alpha1.NodeStatusUp},
									},
								},
							},
						},
					},
				},
			},
			selfDC:                               "dc1",
			selfRack:                             "rack1",
			selfOrdinal:                          2,
			singleReportAllowNonReportingHostIDs: false,
			expected:                             false,
		},
		{
			name: "single report, a node is missing a host ID",
			scyllaDBDatacenterNodesStatusReports: []*scyllav1alpha1.ScyllaDBDatacenterNodesStatusReport{
				{
					DatacenterName: "dc1",
					Racks: []scyllav1alpha1.RackNodesStatusReport{
						{
							Name: "rack1",
							Nodes: []scyllav1alpha1.NodeStatusReport{
								{
									Ordinal: 0,
									HostID:  nil,
									ObservedNodes: []scyllav1alpha1.ObservedNodeStatus{
										{HostID: "host0", Status: scyllav1alpha1.NodeStatusUp},
									},
								},
							},
						},
					},
				},
			},
			selfDC:                               "dc1",
			selfRack:                             "rack1",
			selfOrdinal:                          1,
			singleReportAllowNonReportingHostIDs: false,
			expected:                             false,
		},
		{
			name: "single report, a node is missing a status report",
			scyllaDBDatacenterNodesStatusReports: []*scyllav1alpha1.ScyllaDBDatacenterNodesStatusReport{
				{
					DatacenterName: "dc1",
					Racks: []scyllav1alpha1.RackNodesStatusReport{
						{
							Name: "rack1",
							Nodes: []scyllav1alpha1.NodeStatusReport{
								{
									Ordinal: 0,
									HostID:  pointer.Ptr("host0"),
									ObservedNodes: []scyllav1alpha1.ObservedNodeStatus{
										{HostID: "host0", Status: scyllav1alpha1.NodeStatusUp},
										{HostID: "host1", Status: scyllav1alpha1.NodeStatusUp},
									},
								},
								{
									Ordinal:       1,
									HostID:        pointer.Ptr("host1"),
									ObservedNodes: []scyllav1alpha1.ObservedNodeStatus{},
								},
							},
						},
					},
				},
			},
			selfDC:                               "dc1",
			selfRack:                             "rack1",
			selfOrdinal:                          2,
			singleReportAllowNonReportingHostIDs: false,
			expected:                             false,
		},
		{
			name: "single report, additional up node reported, non-reporting nodes not allowed",
			scyllaDBDatacenterNodesStatusReports: []*scyllav1alpha1.ScyllaDBDatacenterNodesStatusReport{
				{
					DatacenterName: "dc1",
					Racks: []scyllav1alpha1.RackNodesStatusReport{
						{
							Name: "rack1",
							Nodes: []scyllav1alpha1.NodeStatusReport{
								{
									Ordinal: 0,
									HostID:  pointer.Ptr("host0"),
									ObservedNodes: []scyllav1alpha1.ObservedNodeStatus{
										{HostID: "host0", Status: scyllav1alpha1.NodeStatusUp},
										{HostID: "host1", Status: scyllav1alpha1.NodeStatusUp},
										{HostID: "other-host0", Status: scyllav1alpha1.NodeStatusUp},
									},
								},
								{
									Ordinal: 1,
									HostID:  pointer.Ptr("host1"),
									ObservedNodes: []scyllav1alpha1.ObservedNodeStatus{
										{HostID: "host0", Status: scyllav1alpha1.NodeStatusUp},
										{HostID: "host1", Status: scyllav1alpha1.NodeStatusUp},
										{HostID: "other-host0", Status: scyllav1alpha1.NodeStatusUp},
									},
								},
							},
						},
					},
				},
			},
			selfDC:                               "dc1",
			selfRack:                             "rack1",
			selfOrdinal:                          2,
			singleReportAllowNonReportingHostIDs: false,
			expected:                             false,
		},
		// In non-automated multi-datacenter deployments, we expect nodes from external DCs to appear in the status report as reportees only.
		// They are expected to be present in all reports and UP, but they are not expected to report their own status.
		{
			name: "single report, additional up node reported, non-reporting nodes allowed",
			scyllaDBDatacenterNodesStatusReports: []*scyllav1alpha1.ScyllaDBDatacenterNodesStatusReport{
				{
					DatacenterName: "dc1",
					Racks: []scyllav1alpha1.RackNodesStatusReport{
						{
							Name: "rack1",
							Nodes: []scyllav1alpha1.NodeStatusReport{
								{
									Ordinal: 0,
									HostID:  pointer.Ptr("host0"),
									ObservedNodes: []scyllav1alpha1.ObservedNodeStatus{
										{HostID: "host0", Status: scyllav1alpha1.NodeStatusUp},
										{HostID: "host1", Status: scyllav1alpha1.NodeStatusUp},
										{HostID: "other-host0", Status: scyllav1alpha1.NodeStatusUp},
									},
								},
								{
									Ordinal: 1,
									HostID:  pointer.Ptr("host1"),
									ObservedNodes: []scyllav1alpha1.ObservedNodeStatus{
										{HostID: "host0", Status: scyllav1alpha1.NodeStatusUp},
										{HostID: "host1", Status: scyllav1alpha1.NodeStatusUp},
										{HostID: "other-host0", Status: scyllav1alpha1.NodeStatusUp},
									},
								},
							},
						},
					},
				},
			},
			selfDC:                               "dc1",
			selfRack:                             "rack1",
			selfOrdinal:                          2,
			singleReportAllowNonReportingHostIDs: true,
			expected:                             true,
		},
		{
			name: "single report, additional down node reported, non-reporting nodes allowed",
			scyllaDBDatacenterNodesStatusReports: []*scyllav1alpha1.ScyllaDBDatacenterNodesStatusReport{
				{
					DatacenterName: "dc1",
					Racks: []scyllav1alpha1.RackNodesStatusReport{
						{
							Name: "rack1",
							Nodes: []scyllav1alpha1.NodeStatusReport{
								{
									Ordinal: 0,
									HostID:  pointer.Ptr("host0"),
									ObservedNodes: []scyllav1alpha1.ObservedNodeStatus{
										{HostID: "host0", Status: scyllav1alpha1.NodeStatusUp},
										{HostID: "host1", Status: scyllav1alpha1.NodeStatusUp},
										{HostID: "other-host0", Status: scyllav1alpha1.NodeStatusDown},
									},
								},
								{
									Ordinal: 1,
									HostID:  pointer.Ptr("host1"),
									ObservedNodes: []scyllav1alpha1.ObservedNodeStatus{
										{HostID: "host0", Status: scyllav1alpha1.NodeStatusUp},
										{HostID: "host1", Status: scyllav1alpha1.NodeStatusUp},
										{HostID: "other-host0", Status: scyllav1alpha1.NodeStatusUp},
									},
								},
							},
						},
					},
				},
			},
			selfDC:                               "dc1",
			selfRack:                             "rack1",
			selfOrdinal:                          2,
			singleReportAllowNonReportingHostIDs: true,
			expected:                             false,
		},
		{
			name: "single report, additional up node reported but not by all, non-reporting nodes allowed",
			scyllaDBDatacenterNodesStatusReports: []*scyllav1alpha1.ScyllaDBDatacenterNodesStatusReport{
				{
					DatacenterName: "dc1",
					Racks: []scyllav1alpha1.RackNodesStatusReport{
						{
							Name: "rack1",
							Nodes: []scyllav1alpha1.NodeStatusReport{
								{
									Ordinal: 0,
									HostID:  pointer.Ptr("host0"),
									ObservedNodes: []scyllav1alpha1.ObservedNodeStatus{
										{HostID: "host0", Status: scyllav1alpha1.NodeStatusUp},
										{HostID: "host1", Status: scyllav1alpha1.NodeStatusUp},
										{HostID: "other-host0", Status: scyllav1alpha1.NodeStatusUp},
									},
								},
								{
									Ordinal: 1,
									HostID:  pointer.Ptr("host1"),
									ObservedNodes: []scyllav1alpha1.ObservedNodeStatus{
										{HostID: "host0", Status: scyllav1alpha1.NodeStatusUp},
										{HostID: "host1", Status: scyllav1alpha1.NodeStatusUp},
									},
								},
							},
						},
					},
				},
			},
			selfDC:                               "dc1",
			selfRack:                             "rack1",
			selfOrdinal:                          2,
			singleReportAllowNonReportingHostIDs: true,
			expected:                             false,
		},
		{
			name: "multiple reports, all nodes up",
			scyllaDBDatacenterNodesStatusReports: []*scyllav1alpha1.ScyllaDBDatacenterNodesStatusReport{
				{
					DatacenterName: "dc1",
					Racks: []scyllav1alpha1.RackNodesStatusReport{
						{
							Name: "rack1",
							Nodes: []scyllav1alpha1.NodeStatusReport{
								{
									Ordinal: 0,
									HostID:  pointer.Ptr("dc1-host0"),
									ObservedNodes: []scyllav1alpha1.ObservedNodeStatus{
										{HostID: "dc1-host0", Status: scyllav1alpha1.NodeStatusUp},
										{HostID: "dc2-host0", Status: scyllav1alpha1.NodeStatusUp},
									},
								},
							},
						},
					},
				},
				{
					DatacenterName: "dc2",
					Racks: []scyllav1alpha1.RackNodesStatusReport{
						{
							Name: "rack1",
							Nodes: []scyllav1alpha1.NodeStatusReport{
								{
									Ordinal: 0,
									HostID:  pointer.Ptr("dc2-host0"),
									ObservedNodes: []scyllav1alpha1.ObservedNodeStatus{
										{HostID: "dc1-host0", Status: scyllav1alpha1.NodeStatusUp},
										{HostID: "dc2-host0", Status: scyllav1alpha1.NodeStatusUp},
									},
								},
							},
						},
					},
				},
			},
			selfDC:                               "dc3",
			selfRack:                             "rack1",
			selfOrdinal:                          0,
			singleReportAllowNonReportingHostIDs: true,
			expected:                             true,
		},
		{
			name: "multiple reports, a node is down",
			scyllaDBDatacenterNodesStatusReports: []*scyllav1alpha1.ScyllaDBDatacenterNodesStatusReport{
				{
					DatacenterName: "dc1",
					Racks: []scyllav1alpha1.RackNodesStatusReport{
						{
							Name: "rack1",
							Nodes: []scyllav1alpha1.NodeStatusReport{
								{
									Ordinal: 0,
									HostID:  pointer.Ptr("dc1-host0"),
									ObservedNodes: []scyllav1alpha1.ObservedNodeStatus{
										{HostID: "dc1-host0", Status: scyllav1alpha1.NodeStatusUp},
										{HostID: "dc2-host0", Status: scyllav1alpha1.NodeStatusDown},
									},
								},
							},
						},
					},
				},
				{
					DatacenterName: "dc2",
					Racks: []scyllav1alpha1.RackNodesStatusReport{
						{
							Name: "rack1",
							Nodes: []scyllav1alpha1.NodeStatusReport{
								{
									Ordinal: 0,
									HostID:  pointer.Ptr("dc2-host0"),
									ObservedNodes: []scyllav1alpha1.ObservedNodeStatus{
										{HostID: "dc1-host0", Status: scyllav1alpha1.NodeStatusUp},
										{HostID: "dc2-host0", Status: scyllav1alpha1.NodeStatusUp},
									},
								},
							},
						},
					},
				},
			},
			selfDC:                               "dc3",
			selfRack:                             "rack1",
			selfOrdinal:                          0,
			singleReportAllowNonReportingHostIDs: true,
			expected:                             false,
		},
		{
			name: "multiple reports, node missing host ID in other dc",
			scyllaDBDatacenterNodesStatusReports: []*scyllav1alpha1.ScyllaDBDatacenterNodesStatusReport{
				{
					DatacenterName: "dc1",
					Racks: []scyllav1alpha1.RackNodesStatusReport{
						{
							Name: "rack1",
							Nodes: []scyllav1alpha1.NodeStatusReport{
								{
									Ordinal: 0,
									HostID:  pointer.Ptr("dc1-host0"),
									ObservedNodes: []scyllav1alpha1.ObservedNodeStatus{
										{HostID: "dc1-host0", Status: scyllav1alpha1.NodeStatusUp},
									},
								},
							},
						},
					},
				},
				{
					DatacenterName: "dc2",
					Racks: []scyllav1alpha1.RackNodesStatusReport{
						{
							Name: "rack1",
							Nodes: []scyllav1alpha1.NodeStatusReport{
								{
									Ordinal:       0,
									HostID:        nil,
									ObservedNodes: []scyllav1alpha1.ObservedNodeStatus{},
								},
							},
						},
					},
				},
			},
			selfDC:                               "dc3",
			selfRack:                             "rack1",
			selfOrdinal:                          0,
			singleReportAllowNonReportingHostIDs: true,
			expected:                             false,
		},
		// With multiple reports, we do not allow for foreign nodes to appear in a report without having reported their own status, regardless of the singleReportAllowNonReportingHostIDs parameter.
		{
			name: "multiple reports, foreign up node reported",
			scyllaDBDatacenterNodesStatusReports: []*scyllav1alpha1.ScyllaDBDatacenterNodesStatusReport{
				{
					DatacenterName: "dc1",
					Racks: []scyllav1alpha1.RackNodesStatusReport{
						{
							Name: "rack1",
							Nodes: []scyllav1alpha1.NodeStatusReport{
								{
									Ordinal: 0,
									HostID:  pointer.Ptr("dc1-host0"),
									ObservedNodes: []scyllav1alpha1.ObservedNodeStatus{
										{HostID: "dc1-host0", Status: scyllav1alpha1.NodeStatusUp},
										{HostID: "dc2-host0", Status: scyllav1alpha1.NodeStatusUp},
										{HostID: "other-host0", Status: scyllav1alpha1.NodeStatusUp},
									},
								},
							},
						},
					},
				},
				{
					DatacenterName: "dc2",
					Racks: []scyllav1alpha1.RackNodesStatusReport{
						{
							Name: "rack1",
							Nodes: []scyllav1alpha1.NodeStatusReport{
								{
									Ordinal: 0,
									HostID:  pointer.Ptr("dc2-host0"),
									ObservedNodes: []scyllav1alpha1.ObservedNodeStatus{
										{HostID: "dc1-host0", Status: scyllav1alpha1.NodeStatusUp},
										{HostID: "dc2-host0", Status: scyllav1alpha1.NodeStatusUp},
										{HostID: "other-host0", Status: scyllav1alpha1.NodeStatusUp},
									},
								},
							},
						},
					},
				},
			},
			selfDC:                               "dc2",
			selfRack:                             "rack1",
			selfOrdinal:                          0,
			singleReportAllowNonReportingHostIDs: true,
			expected:                             false,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := isBootstrapPreconditionSatisfied(tc.scyllaDBDatacenterNodesStatusReports, tc.selfDC, tc.selfRack, tc.selfOrdinal, tc.singleReportAllowNonReportingHostIDs)
			if got != tc.expected {
				t.Errorf("expected %v, got %v", tc.expected, got)
			}
		})
	}
}
