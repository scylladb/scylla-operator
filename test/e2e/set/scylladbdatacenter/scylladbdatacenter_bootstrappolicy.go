// Copyright (C) 2026 ScyllaDB

package scylladbdatacenter

import (
	"context"
	"fmt"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	utilsv1alpha1 "github.com/scylladb/scylla-operator/test/e2e/utils/v1alpha1"
	scylladbdatacenterverification "github.com/scylladb/scylla-operator/test/e2e/utils/verification/scylladbdatacenter"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = g.Describe("ScyllaDBDatacenter", framework.SuiteParallel, framework.SuiteParallelOpenShift, framework.SuiteKindFast, func() {
	var f *framework.Framework

	g.BeforeEach(func(ctx context.Context) {
		f = framework.NewFramework(ctx, "scylladbdatacenter")
	})

	// getRackStatefulSetUIDs returns the UIDs of the rack StatefulSets, keyed by rack name.
	getRackStatefulSetUIDs := func(statefulSets map[string]*appsv1.StatefulSet) map[string]types.UID {
		uids := map[string]types.UID{}
		for rackName, sts := range statefulSets {
			uids[rackName] = sts.UID
		}

		return uids
	}

	// getRackPodUIDs returns the UIDs of the Pods of every rack StatefulSet, keyed by Pod name.
	getRackPodUIDs := func(ctx context.Context, client framework.Client, statefulSets map[string]*appsv1.StatefulSet) map[string]types.UID {
		uids := map[string]types.UID{}
		for _, sts := range statefulSets {
			pods, err := utilsv1alpha1.GetPodsForStatefulSet(ctx, client.KubeClient().CoreV1(), sts)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(pods).NotTo(o.BeEmpty())

			for podName, pod := range pods {
				uids[podName] = pod.UID
			}
		}

		return uids
	}

	type entry struct {
		initialBootstrapPolicy scyllav1alpha1.BootstrapPolicy
		targetBootstrapPolicy  scyllav1alpha1.BootstrapPolicy
	}

	describeEntry := func(e *entry) string {
		return fmt.Sprintf("from %s to %s", e.initialBootstrapPolicy, e.targetBootstrapPolicy)
	}

	podManagementPolicyForBootstrapPolicy := func(bootstrapPolicy scyllav1alpha1.BootstrapPolicy) appsv1.PodManagementPolicyType {
		if bootstrapPolicy == scyllav1alpha1.BootstrapPolicyParallel {
			return appsv1.ParallelPodManagement
		}

		return appsv1.OrderedReadyPodManagement
	}

	g.DescribeTable("should recreate StatefulSets without disrupting nodes when the bootstrap policy is changed",
		func(ctx g.SpecContext, e *entry) {
			ns, nsClient, ok := f.DefaultNamespaceIfAny()
			o.Expect(ok).To(o.BeTrue())

			sdc := f.GetDefaultScyllaDBDatacenter()
			sdc.Spec.BootstrapPolicy = &e.initialBootstrapPolicy

			framework.By("Creating a ScyllaDBDatacenter with the %s bootstrap policy", e.initialBootstrapPolicy)
			sdc, err := nsClient.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(ns.Name).Create(ctx, sdc, metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			framework.By("Waiting for the ScyllaDBDatacenter to roll out (RV=%s)", sdc.ResourceVersion)
			initialRolloutCtx, initialRolloutCtxCancel := utilsv1alpha1.ContextForRollout(ctx, sdc)
			defer initialRolloutCtxCancel()
			sdc, err = controllerhelpers.WaitForScyllaDBDatacenterState(initialRolloutCtx, nsClient.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(ns.Name), sdc.Name, controllerhelpers.WaitForStateOptions{}, utilsv1alpha1.IsScyllaDBDatacenterRolledOut)
			o.Expect(err).NotTo(o.HaveOccurred())
			scylladbdatacenterverification.Verify(ctx, nsClient.KubeClient(), nsClient.ScyllaClient(), sdc)
			scylladbdatacenterverification.WaitForFullQuorum(ctx, nsClient.KubeClient().CoreV1(), sdc)

			framework.By("Verifying the rack StatefulSets are created with the %s pod management policy", podManagementPolicyForBootstrapPolicy(e.initialBootstrapPolicy))
			statefulSets, err := utilsv1alpha1.GetStatefulSetsForScyllaDBDatacenter(ctx, nsClient.KubeClient().AppsV1(), sdc)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(statefulSets).To(o.HaveLen(len(sdc.Spec.Racks)))
			for _, sts := range statefulSets {
				o.Expect(sts.Spec.PodManagementPolicy).To(o.Equal(podManagementPolicyForBootstrapPolicy(e.initialBootstrapPolicy)))
			}

			initialStatefulSetUIDs := getRackStatefulSetUIDs(statefulSets)
			initialPodUIDs := getRackPodUIDs(ctx, nsClient, statefulSets)

			framework.By("Changing the bootstrap policy to %s", e.targetBootstrapPolicy)
			sdc, err = nsClient.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(ns.Name).Patch(
				ctx,
				sdc.Name,
				types.MergePatchType,
				[]byte(fmt.Sprintf(`{"spec":{"bootstrapPolicy":%q}}`, e.targetBootstrapPolicy)),
				metav1.PatchOptions{},
			)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(sdc.Spec.BootstrapPolicy).NotTo(o.BeNil())
			o.Expect(*sdc.Spec.BootstrapPolicy).To(o.Equal(e.targetBootstrapPolicy))

			framework.By("Waiting for the ScyllaDBDatacenter to roll out (RV=%s)", sdc.ResourceVersion)
			patchRolloutCtx, patchRolloutCtxCancel := utilsv1alpha1.ContextForRollout(ctx, sdc)
			defer patchRolloutCtxCancel()
			sdc, err = controllerhelpers.WaitForScyllaDBDatacenterState(patchRolloutCtx, nsClient.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(ns.Name), sdc.Name, controllerhelpers.WaitForStateOptions{}, utilsv1alpha1.IsScyllaDBDatacenterRolledOut)
			o.Expect(err).NotTo(o.HaveOccurred())
			scylladbdatacenterverification.Verify(ctx, nsClient.KubeClient(), nsClient.ScyllaClient(), sdc)
			scylladbdatacenterverification.WaitForFullQuorum(ctx, nsClient.KubeClient().CoreV1(), sdc)

			framework.By("Verifying the rack StatefulSets are recreated with the %s pod management policy", podManagementPolicyForBootstrapPolicy(e.targetBootstrapPolicy))
			statefulSets, err = utilsv1alpha1.GetStatefulSetsForScyllaDBDatacenter(ctx, nsClient.KubeClient().AppsV1(), sdc)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(statefulSets).To(o.HaveLen(len(sdc.Spec.Racks)))
			for _, sts := range statefulSets {
				o.Expect(sts.Spec.PodManagementPolicy).To(o.Equal(podManagementPolicyForBootstrapPolicy(e.targetBootstrapPolicy)))
			}

			// podManagementPolicy is immutable, so the StatefulSets have to be recreated for the change to take effect.
			for rackName, uid := range getRackStatefulSetUIDs(statefulSets) {
				o.Expect(uid).NotTo(o.Equal(initialStatefulSetUIDs[rackName]), fmt.Sprintf("StatefulSet of rack %q should have been recreated", rackName))
			}

			framework.By("Verifying the nodes were not disrupted")
			// The StatefulSets are deleted with an orphan propagation policy, so their Pods survive the recreate and
			// are adopted back. Any Pod being recreated means the running nodes were disrupted.
			o.Expect(getRackPodUIDs(ctx, nsClient, statefulSets)).To(o.Equal(initialPodUIDs))
		},
		g.Entry(describeEntry, &entry{
			initialBootstrapPolicy: scyllav1alpha1.BootstrapPolicySequential,
			targetBootstrapPolicy:  scyllav1alpha1.BootstrapPolicyParallel,
		}),
		g.Entry(describeEntry, &entry{
			initialBootstrapPolicy: scyllav1alpha1.BootstrapPolicyParallel,
			targetBootstrapPolicy:  scyllav1alpha1.BootstrapPolicySequential,
		}),
	)
})
