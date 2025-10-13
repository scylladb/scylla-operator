// Copyright (C) 2025 ScyllaDB

package bootstrapbarrier

import (
	"testing"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/pointer"
)

func Test_isBootstrapPreconditionSatisfied(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name                                 string
		scyllaDBDatacenterNodesStatusReports []*scyllav1alpha1.ScyllaDBDatacenterNodesStatusReport
		selfDC                               string
		selfRack                             string
		selfOrdinal                          int
		expected                             bool
	}{
		{
			name:                                 "no reports",
			scyllaDBDatacenterNodesStatusReports: []*scyllav1alpha1.ScyllaDBDatacenterNodesStatusReport{},
			selfDC:                               "dc1",
			selfRack:                             "rack1",
			selfOrdinal:                          0,
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
			selfDC:      "dc1",
			selfRack:    "rack1",
			selfOrdinal: 2,
			expected:    true,
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
			selfDC:      "dc1",
			selfRack:    "rack1",
			selfOrdinal: 2,
			expected:    false,
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
			selfDC:      "dc1",
			selfRack:    "rack1",
			selfOrdinal: 1,
			expected:    false,
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
			selfDC:      "dc1",
			selfRack:    "rack1",
			selfOrdinal: 2,
			expected:    false,
		},
		// In non-automated multi-datacenter deployments, we expect nodes from external DCs to appear in the status report as reportees only.
		// They are expected to be present in all reports and UP, but they are not expected to report their own status.
		{
			name: "single report, additional up node reported",
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
			selfDC:      "dc1",
			selfRack:    "rack1",
			selfOrdinal: 2,
			expected:    true,
		},
		{
			name: "single report, additional down node reported",
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
			selfDC:      "dc1",
			selfRack:    "rack1",
			selfOrdinal: 2,
			expected:    false,
		},
		{
			name: "single report, additional up node reported but not by all",
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
			selfDC:      "dc1",
			selfRack:    "rack1",
			selfOrdinal: 2,
			expected:    false,
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
			selfDC:      "dc3",
			selfRack:    "rack1",
			selfOrdinal: 0,
			expected:    true,
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
			selfDC:      "dc3",
			selfRack:    "rack1",
			selfOrdinal: 0,
			expected:    false,
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
			selfDC:      "dc3",
			selfRack:    "rack1",
			selfOrdinal: 0,
			expected:    false,
		},
		// With multiple reports, we do not allow for foreign nodes to appear in a report without having reported their own status.
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
			selfDC:      "dc2",
			selfRack:    "rack1",
			selfOrdinal: 0,
			expected:    false,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := isBootstrapPreconditionSatisfied(tc.scyllaDBDatacenterNodesStatusReports, tc.selfDC, tc.selfRack, tc.selfOrdinal)
			if got != tc.expected {
				t.Errorf("expected %v, got %v", tc.expected, got)
			}
		})
	}
}
