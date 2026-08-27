// Copyright (C) 2026 ScyllaDB

package scylladbdatacenter

import (
	"context"
	"fmt"
	"strings"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	oslices "github.com/scylladb/scylla-operator/pkg/helpers/slices"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	utilsv1alpha1 "github.com/scylladb/scylla-operator/test/e2e/utils/v1alpha1"
	"github.com/scylladb/scylla-operator/test/e2e/utils/verification"
	scylladbdatacenterverification "github.com/scylladb/scylla-operator/test/e2e/utils/verification/scylladbdatacenter"
	corev1 "k8s.io/api/core/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	// statefulSetControllerProgressingConditionType is the progressing condition of the StatefulSet controller of the
	// ScyllaDBDatacenter controller.
	statefulSetControllerProgressingConditionType = "StatefulSetControllerProgressing"
	// deferringRackNodeCountChangeReason is reported while a node count change waits for the decommissioning nodes.
	deferringRackNodeCountChangeReason = "DeferringRackNodeCountChange"
)

var _ = g.Describe("ScyllaDBDatacenter", framework.SuiteParallel, framework.SuiteParallelOpenShift, framework.SuiteKindFast, func() {
	var f *framework.Framework

	g.BeforeEach(func(ctx context.Context) {
		f = framework.NewFramework(ctx, "scylladbdatacenter")
	})

	g.It("should finish an ongoing decommission before applying a node count raised back mid-decommission", func(ctx g.SpecContext) {
		const (
			initialNodes = int32(3)
			// leavingOrdinal is the highest ordinal, which is the node a scale-down by one removes.
			leavingOrdinal = initialNodes - 1
		)

		ns, nsClient, ok := f.DefaultNamespaceIfAny()
		o.Expect(ok).To(o.BeTrue())

		sdc := f.GetDefaultScyllaDBDatacenter()
		sdc.Spec.RackTemplate.Nodes = new(initialNodes)
		o.Expect(sdc.Spec.Racks).To(o.HaveLen(1))
		rack := sdc.Spec.Racks[0]

		framework.By("Creating a ScyllaDBDatacenter with %d nodes", initialNodes)
		sdc, err := nsClient.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(ns.Name).Create(ctx, sdc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaDBDatacenter to roll out (RV=%s)", sdc.ResourceVersion)
		initialRolloutCtx, initialRolloutCtxCancel := utilsv1alpha1.ContextForRollout(ctx, sdc)
		defer initialRolloutCtxCancel()
		sdc, err = controllerhelpers.WaitForScyllaDBDatacenterState(initialRolloutCtx, nsClient.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(ns.Name), sdc.Name, controllerhelpers.WaitForStateOptions{}, utilsv1alpha1.IsScyllaDBDatacenterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())
		scylladbdatacenterverification.Verify(ctx, nsClient.KubeClient(), nsClient.ScyllaClient(), sdc)
		scylladbdatacenterverification.WaitForFullQuorum(ctx, nsClient.KubeClient().CoreV1(), sdc)

		initialHostIDs, err := utilsv1alpha1.GetHostIDs(ctx, nsClient.KubeClient().CoreV1(), sdc)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(initialHostIDs).To(o.HaveLen(int(initialNodes)))

		// The data is replicated to the nodes that stay, so that the leaving node can be decommissioned.
		framework.By("Inserting data replicated to the %d staying nodes", initialNodes-1)
		var stayingHosts []string
		for ordinal := int32(0); ordinal < leavingOrdinal; ordinal++ {
			svc, err := nsClient.KubeClient().CoreV1().Services(ns.Name).Get(ctx, naming.MemberServiceName(rack, sdc, int(ordinal)), metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			host, err := utilsv1alpha1.GetBroadcastRPCAddress(ctx, nsClient.KubeClient().CoreV1(), sdc, svc)
			o.Expect(err).NotTo(o.HaveOccurred())
			stayingHosts = append(stayingHosts, host)
		}
		di := verification.InsertAndVerifyCQLData(ctx, stayingHosts)
		defer di.Close()

		leavingServiceName := naming.MemberServiceName(rack, sdc, int(leavingOrdinal))
		leavingService, err := nsClient.KubeClient().CoreV1().Services(ns.Name).Get(ctx, leavingServiceName, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Scaling the ScyllaDBDatacenter down to %d nodes", initialNodes-1)
		sdc, err = nsClient.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(ns.Name).Patch(
			ctx,
			sdc.Name,
			types.JSONPatchType,
			[]byte(fmt.Sprintf(`[{"op": "replace", "path": "/spec/rackTemplate/nodes", "value": %d}]`, initialNodes-1)),
			metav1.PatchOptions{},
		)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(*sdc.Spec.RackTemplate.Nodes).To(o.Equal(initialNodes - 1))

		framework.By("Waiting for the decommission of node %q to be requested", leavingServiceName)
		decommissionRequestCtx, decommissionRequestCtxCancel := context.WithTimeout(ctx, utils.SyncTimeout)
		defer decommissionRequestCtxCancel()
		_, err = controllerhelpers.WaitForServiceState(decommissionRequestCtx, nsClient.KubeClient().CoreV1().Services(ns.Name), leavingServiceName, controllerhelpers.WaitForStateOptions{}, func(svc *corev1.Service) (bool, error) {
			return svc.Labels[naming.DecommissionedLabel] == naming.LabelValueFalse, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Raising the node count back to %d while node %q is still decommissioning", initialNodes, leavingServiceName)
		sdc, err = nsClient.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(ns.Name).Patch(
			ctx,
			sdc.Name,
			types.JSONPatchType,
			[]byte(fmt.Sprintf(`[{"op": "replace", "path": "/spec/rackTemplate/nodes", "value": %d}]`, initialNodes)),
			metav1.PatchOptions{},
		)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(*sdc.Spec.RackTemplate.Nodes).To(o.Equal(initialNodes))

		framework.By("Waiting for the raised node count to be reported as deferred")
		deferredCtx, deferredCtxCancel := context.WithTimeout(ctx, utils.SyncTimeout)
		defer deferredCtxCancel()
		sdc, err = controllerhelpers.WaitForScyllaDBDatacenterState(deferredCtx, nsClient.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(ns.Name), sdc.Name, controllerhelpers.WaitForStateOptions{}, func(sdc *scyllav1alpha1.ScyllaDBDatacenter) (bool, error) {
			if sdc.Status.ObservedGeneration == nil || *sdc.Status.ObservedGeneration < sdc.Generation {
				return false, nil
			}

			progressingCondition := apimeta.FindStatusCondition(sdc.Status.Conditions, statefulSetControllerProgressingConditionType)
			if progressingCondition == nil || progressingCondition.Status != metav1.ConditionTrue {
				return false, nil
			}

			// The reasons of the aggregated condition are comma-separated.
			return oslices.Contains(strings.Split(progressingCondition.Reason, ","), func(reason string) bool {
				return reason == deferringRackNodeCountChangeReason
			}), nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		rackStatus, _, ok := oslices.Find(sdc.Status.Racks, func(rackStatus scyllav1alpha1.RackStatus) bool {
			return rackStatus.Name == rack.Name
		})
		o.Expect(ok).To(o.BeTrue())
		o.Expect(rackStatus.DecommissioningNodes).To(o.Equal([]scyllav1alpha1.DecommissioningNodeStatus{
			{Name: leavingServiceName},
		}))

		framework.By("Waiting for the ScyllaDBDatacenter to roll out (RV=%s)", sdc.ResourceVersion)
		// The rollout removes the leaving node and bootstraps a fresh one in its place.
		finalRolloutCtx, finalRolloutCtxCancel := utilsv1alpha1.ContextForRollout(ctx, sdc)
		defer finalRolloutCtxCancel()
		sdc, err = controllerhelpers.WaitForScyllaDBDatacenterState(finalRolloutCtx, nsClient.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(ns.Name), sdc.Name, controllerhelpers.WaitForStateOptions{}, utilsv1alpha1.IsScyllaDBDatacenterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())
		scylladbdatacenterverification.Verify(ctx, nsClient.KubeClient(), nsClient.ScyllaClient(), sdc)
		scylladbdatacenterverification.WaitForFullQuorum(ctx, nsClient.KubeClient().CoreV1(), sdc)

		framework.By("Verifying the leaving node was replaced by a fresh one")
		rackStatus, _, ok = oslices.Find(sdc.Status.Racks, func(rackStatus scyllav1alpha1.RackStatus) bool {
			return rackStatus.Name == rack.Name
		})
		o.Expect(ok).To(o.BeTrue())
		o.Expect(rackStatus.DecommissioningNodes).To(o.BeEmpty())

		freshService, err := nsClient.KubeClient().CoreV1().Services(ns.Name).Get(ctx, leavingServiceName, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(freshService.UID).NotTo(o.Equal(leavingService.UID))
		o.Expect(freshService.Labels).NotTo(o.HaveKey(naming.DecommissionedLabel))

		hostIDs, err := utilsv1alpha1.GetHostIDs(ctx, nsClient.KubeClient().CoreV1(), sdc)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(hostIDs).To(o.HaveLen(int(initialNodes)))
		// The staying nodes keep their identity, while the leaving node's identity is gone for good.
		stayingHostIDs := oslices.Filter(hostIDs, func(hostID string) bool {
			return oslices.Contains(initialHostIDs, func(initialHostID string) bool {
				return initialHostID == hostID
			})
		})
		o.Expect(stayingHostIDs).To(o.HaveLen(int(initialNodes - 1)))

		verification.VerifyCQLData(ctx, di)
	})
})
