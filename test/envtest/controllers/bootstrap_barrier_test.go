//go:build envtest

package controllers

import (
	"testing"
	"time"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controller/bootstrapbarrier"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	"github.com/scylladb/scylla-operator/test/envtest"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestBootstrapBarrierController(t *testing.T) {
	t.Parallel()

	const (
		nodeServiceName                      = "dc1-rack1-1"
		nodeOrdinal                          = 1
		nodesStatusReportSelector            = "test-selector"
		singleReportAllowNonReportingHostIDs = true
		rackName                             = "rack1"
		dcName                               = "dc1"
		hostID                               = "host1"
	)

	testService := func() *corev1.Service {
		return &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name: nodeServiceName,
				Labels: map[string]string{
					naming.DatacenterNameLabel: dcName,
					naming.RackNameLabel:       rackName,
				},
				Annotations: map[string]string{},
			},
			Spec: corev1.ServiceSpec{
				ClusterIP: "None",
			},
		}
	}

	testCases := []struct {
		name                  string
		objects               []client.Object
		expectPreconditionMet bool
	}{
		{
			name:                  "no service",
			objects:               nil,
			expectPreconditionMet: false,
		},
		{
			name: "single status report with all nodes up",
			objects: []client.Object{
				testService(),
				&scyllav1alpha1.ScyllaDBDatacenterNodesStatusReport{
					ObjectMeta: metav1.ObjectMeta{
						Name: "report-1",
						Labels: map[string]string{
							naming.ScyllaDBDatacenterNodesStatusReportSelectorLabel: nodesStatusReportSelector,
						},
					},
					DatacenterName: dcName,
					Racks: []scyllav1alpha1.RackNodesStatusReport{
						{
							Name: rackName,
							Nodes: []scyllav1alpha1.NodeStatusReport{
								{
									Ordinal: 0,
									HostID:  pointer.Ptr("host0"),
									ObservedNodes: []scyllav1alpha1.ObservedNodeStatus{
										{HostID: "host0", Status: scyllav1alpha1.NodeStatusUp},
										{HostID: hostID, Status: scyllav1alpha1.NodeStatusUp},
									},
								},
								{
									Ordinal: nodeOrdinal,
									HostID:  pointer.Ptr(hostID),
									ObservedNodes: []scyllav1alpha1.ObservedNodeStatus{
										{HostID: "host0", Status: scyllav1alpha1.NodeStatusUp},
										{HostID: hostID, Status: scyllav1alpha1.NodeStatusUp},
									},
								},
							},
						},
					},
				},
			},
			expectPreconditionMet: true,
		},
		{
			name: "single status report with one node down",
			objects: []client.Object{
				testService(),
				&scyllav1alpha1.ScyllaDBDatacenterNodesStatusReport{
					ObjectMeta: metav1.ObjectMeta{
						Name: "report-1",
						Labels: map[string]string{
							naming.ScyllaDBDatacenterNodesStatusReportSelectorLabel: nodesStatusReportSelector,
						},
					},
					DatacenterName: dcName,
					Racks: []scyllav1alpha1.RackNodesStatusReport{
						{
							Name: rackName,
							Nodes: []scyllav1alpha1.NodeStatusReport{
								{
									Ordinal: 0,
									HostID:  pointer.Ptr("host0"),
									ObservedNodes: []scyllav1alpha1.ObservedNodeStatus{
										{HostID: "host0", Status: scyllav1alpha1.NodeStatusUp},
										{HostID: hostID, Status: scyllav1alpha1.NodeStatusDown},
									},
								},
								{
									Ordinal: nodeOrdinal,
									HostID:  pointer.Ptr(hostID),
									ObservedNodes: []scyllav1alpha1.ObservedNodeStatus{
										{HostID: "host0", Status: scyllav1alpha1.NodeStatusUp},
										{HostID: hostID, Status: scyllav1alpha1.NodeStatusDown},
									},
								},
							},
						},
					},
				},
			},
			expectPreconditionMet: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			env := envtest.Setup(t)

			t.Logf("Creating test objects")
			for _, obj := range tc.objects {
				if ok, err := env.KubeClient().IsObjectNamespaced(obj); err == nil && ok {
					obj.SetNamespace(env.Namespace())
				}

				err := env.KubeClient().Create(t.Context(), obj)
				if err != nil {
					t.Fatalf("Failed to create object %T: %v", obj, err)
				}
				t.Logf("Created object %T/%s", obj, obj.GetName())
			}

			controllerOpts := controllerOpts{
				nodeServiceName:                      nodeServiceName,
				nodesStatusReportSelector:            nodesStatusReportSelector,
				singleReportAllowNonReportingHostIDs: singleReportAllowNonReportingHostIDs,
				bootstrapPreconditionSatisfiedCh:     make(chan struct{}),
			}
			runBoostrapBarrierController(t, env, controllerOpts)

			if tc.expectPreconditionMet {
				timeout := time.After(30 * time.Second)
				select {
				case <-timeout:
					t.Fatalf("Timed out waiting for bootstrap precondition to be met")
				case <-t.Context().Done():
					t.Fatalf("Test context done while waiting for bootstrap precondition to be met")
				case <-controllerOpts.bootstrapPreconditionSatisfiedCh:
					t.Logf("Bootstrap precondition met as expected")
				}
			} else {
				timeout := time.After(10 * time.Second)
				select {
				case <-timeout:
					t.Logf("Bootstrap precondition not met as expected")
				case <-t.Context().Done():
					t.Fatalf("Test context done while waiting for bootstrap precondition to not be met")
				case <-controllerOpts.bootstrapPreconditionSatisfiedCh:
					t.Fatalf("Bootstrap precondition met unexpectedly")
				}
			}
		})
	}
}

type controllerOpts struct {
	nodeServiceName                      string
	nodesStatusReportSelector            string
	singleReportAllowNonReportingHostIDs bool
	bootstrapPreconditionSatisfiedCh     chan struct{}
}

func runBoostrapBarrierController(t *testing.T, envTest *envtest.Environment, opts controllerOpts) {
	informerFactory := bootstrapbarrier.NewInformerFactory(
		envTest.TypedKubeClient(),
		envTest.ScyllaClient(),
		bootstrapbarrier.InformerFactoryOptions{
			ServiceName:        opts.nodeServiceName,
			SelectorLabelValue: opts.nodesStatusReportSelector,
			Namespace:          envTest.Namespace(),
		},
	)

	bootstrapBarrierController, err := bootstrapbarrier.NewController(
		envTest.Namespace(),
		opts.nodeServiceName,
		opts.nodesStatusReportSelector,
		opts.singleReportAllowNonReportingHostIDs,
		opts.bootstrapPreconditionSatisfiedCh,
		envTest.TypedKubeClient(),
		informerFactory,
	)
	if err != nil {
		t.Fatalf("Failed to create bootstrap barrier controller: %v", err)
	}

	go informerFactory.Start(t.Context().Done())
	t.Logf("Started informers")

	go bootstrapBarrierController.Run(t.Context())
	t.Logf("Started bootstrap barrier controller")
}
