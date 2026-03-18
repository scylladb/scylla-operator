//go:build envtest

package controllers

import (
	"context"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controller/bootstrapbarrier"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	"github.com/scylladb/scylla-operator/test/envtest"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = g.Describe("BootstrapBarrierController", func() {
	const (
		nodeServiceName                      = "dc1-rack1-1"
		nodeOrdinal                          = 1
		nodesStatusReportSelector            = "test-selector"
		singleReportAllowNonReportingHostIDs = true
		rackName                             = "rack1"
		dcName                               = "dc1"
		hostID                               = "host1"
	)

	var env *envtest.Environment
	g.BeforeEach(func(ctx g.SpecContext) {
		env = envtest.Setup(ctx)
	})

	type testCase struct {
		objects               []client.Object
		expectPreconditionMet bool
	}
	g.DescribeTable("Bootstrap precondition evaluation", func(ctx g.SpecContext, tc testCase) {
		g.By("Creating test objects")
		for _, obj := range tc.objects {
			if ok, err := env.KubeClient().IsObjectNamespaced(obj); err == nil && ok {
				obj.SetNamespace(env.Namespace())
			}

			err := env.KubeClient().Create(ctx, obj)
			o.Expect(err).NotTo(o.HaveOccurred(), "Failed to create object %T", obj)
		}

		g.By("Running the BootstrapBarrierController")
		controllerOpts := controllerOpts{
			nodeServiceName:                      nodeServiceName,
			nodesStatusReportSelector:            nodesStatusReportSelector,
			singleReportAllowNonReportingHostIDs: singleReportAllowNonReportingHostIDs,
			bootstrapPreconditionSatisfiedCh:     make(chan struct{}),
		}
		runBoostrapBarrierController(ctx, env, controllerOpts)

		if tc.expectPreconditionMet {
			g.By("Waiting for the bootstrap precondition to be met")
			o.Eventually(controllerOpts.bootstrapPreconditionSatisfiedCh).
				WithTimeout(30*time.Second).
				WithContext(ctx).
				Should(o.BeClosed(), "Bootstrap precondition should be met")
		} else {
			g.By("Ensuring the bootstrap precondition is not met")
			o.Consistently(controllerOpts.bootstrapPreconditionSatisfiedCh).
				WithTimeout(10*time.Second).
				WithContext(ctx).
				ShouldNot(o.BeClosed(), "Bootstrap precondition should not be met")
		}
	},
		g.Entry("When no matching service is created", testCase{
			objects:               nil,
			expectPreconditionMet: false,
		}),
		g.Entry("When a single status report with all nodes up is created", testCase{
			objects: []client.Object{
				testNodeService(nodeServiceName, dcName, rackName),
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
		}),
		g.Entry("When a single status report with one node down is created", testCase{
			objects: []client.Object{
				testNodeService(nodeServiceName, dcName, rackName),
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
		}),
	)
})

type controllerOpts struct {
	nodeServiceName                      string
	nodesStatusReportSelector            string
	singleReportAllowNonReportingHostIDs bool
	bootstrapPreconditionSatisfiedCh     chan struct{}
}

func runBoostrapBarrierController(ctx context.Context, envTest *envtest.Environment, opts controllerOpts) {
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
	o.Expect(err).NotTo(o.HaveOccurred(), "Failed to create bootstrap barrier controller")

	go informerFactory.Start(ctx.Done())
	g.GinkgoWriter.Println("Started informers")

	go bootstrapBarrierController.Run(ctx)
	g.GinkgoWriter.Println("Started bootstrap barrier controller")
}

func testNodeService(nodeServiceName, dcName, rackName string) *corev1.Service {
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
