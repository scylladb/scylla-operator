//go:build envtest

package controllers

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	scyllaversionedclient "github.com/scylladb/scylla-operator/pkg/client/scylla/clientset/versioned"
	scyllainformers "github.com/scylladb/scylla-operator/pkg/client/scylla/informers/externalversions"
	"github.com/scylladb/scylla-operator/pkg/controller/scyllacluster"
	"github.com/scylladb/scylla-operator/pkg/internalapi"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	"github.com/scylladb/scylla-operator/test/envtest"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
)

var _ = g.Describe("ScyllaClusterController condition aggregation", func() {
	var env *envtest.Environment

	g.BeforeEach(func(ctx g.SpecContext) {
		env = envtest.Setup(ctx)
	})

	g.It("propagates Progressing=False,Degraded=False from SDC when SDC reports healthy at the current generation", func(ctx g.SpecContext) {
		g.By("Starting ScyllaCluster controller")
		runScyllaClusterController(ctx, env)

		sc := newBasicScyllaCluster("basic", env.Namespace())
		sc, sdc := createScyllaClusterAndWaitForScyllaDBDatacenterToExist(ctx, env, sc)
		_ = mockHealthyScyllaDBDatacenterConditions(ctx, env.ScyllaClient(), sdc)
		_ = waitForScyllaClusterToSettleWithHealthyConditions(ctx, env.ScyllaClient(), sc.Namespace, sc.Name)
	})

	g.It("propagates Progressing=True from SDC when SDC reports progressing", func(ctx g.SpecContext) {
		g.By("Starting ScyllaCluster controller")
		runScyllaClusterController(ctx, env)

		sc := newBasicScyllaCluster("progressing", env.Namespace())
		sc, sdc := createScyllaClusterAndWaitForScyllaDBDatacenterToExist(ctx, env, sc)

		g.By("Patching SDC status with Progressing=True")
		sdcCopy := sdc.DeepCopy()
		sdcCopy.Status.Conditions = []metav1.Condition{
			// Aggregate conditions.
			newCondition(scyllav1alpha1.ProgressingCondition, metav1.ConditionTrue, "RollingOut", "Datacenter is rolling out", sdc.Generation),
			newCondition(scyllav1alpha1.DegradedCondition, metav1.ConditionFalse, internalapi.AsExpectedReason, "", sdc.Generation),
			// Representative partial conditions from the SDC controller.
			newCondition("StatefulSetControllerProgressing", metav1.ConditionTrue, "RollingOut", "StatefulSet is rolling out", sdc.Generation),
			newCondition("StatefulSetControllerDegraded", metav1.ConditionFalse, internalapi.AsExpectedReason, "", sdc.Generation),
			newCondition("StatefulSetControllerAvailable", metav1.ConditionFalse, "NotYetAvailable", "", sdc.Generation),
		}
		_, err := env.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(env.Namespace()).UpdateStatus(ctx, sdcCopy, metav1.UpdateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Expecting ScyllaCluster to report Progressing=True")
		o.Eventually(func(g o.Gomega) {
			sc, err = env.ScyllaClient().ScyllaV1().ScyllaClusters(env.Namespace()).Get(ctx, sc.Name, metav1.GetOptions{})
			g.Expect(err).NotTo(o.HaveOccurred())

			progressingCond := apimeta.FindStatusCondition(sc.Status.Conditions, scyllav1.ProgressingCondition)
			g.Expect(progressingCond).NotTo(o.BeNil(), "Progressing condition should be present")
			g.Expect(progressingCond.Status).To(o.Equal(metav1.ConditionTrue), "Progressing should be True")

			degradedCond := apimeta.FindStatusCondition(sc.Status.Conditions, scyllav1.DegradedCondition)
			g.Expect(degradedCond).NotTo(o.BeNil(), "Degraded condition should be present")
			g.Expect(degradedCond.Status).To(o.Equal(metav1.ConditionFalse), "Degraded should be False")
		}).WithTimeout(30 * time.Second).WithContext(ctx).Should(o.Succeed())
	})

	g.It("propagates Degraded=True from SDC when SDC reports degraded", func(ctx g.SpecContext) {
		g.By("Starting ScyllaCluster controller")
		runScyllaClusterController(ctx, env)

		sc := newBasicScyllaCluster("degraded", env.Namespace())
		sc, sdc := createScyllaClusterAndWaitForScyllaDBDatacenterToExist(ctx, env, sc)

		g.By("Patching SDC status with Degraded=True")
		sdcCopy := sdc.DeepCopy()
		sdcCopy.Status.Conditions = []metav1.Condition{
			// Aggregate conditions.
			newCondition(scyllav1alpha1.ProgressingCondition, metav1.ConditionFalse, internalapi.AsExpectedReason, "", sdc.Generation),
			newCondition(scyllav1alpha1.DegradedCondition, metav1.ConditionTrue, "NodeFailed", "A node has failed", sdc.Generation),
			newCondition(scyllav1alpha1.AvailableCondition, metav1.ConditionFalse, "NodeFailed", "A node has failed", sdc.Generation),
			// Representative partial conditions from the SDC controller.
			newCondition("StatefulSetControllerProgressing", metav1.ConditionFalse, internalapi.AsExpectedReason, "", sdc.Generation),
			newCondition("StatefulSetControllerDegraded", metav1.ConditionTrue, "NodeFailed", "A node has failed", sdc.Generation),
			newCondition("StatefulSetControllerAvailable", metav1.ConditionFalse, "NodeFailed", "A node has failed", sdc.Generation),
		}
		_, err := env.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(env.Namespace()).UpdateStatus(ctx, sdcCopy, metav1.UpdateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Expecting ScyllaCluster to report Degraded=True")
		o.Eventually(func(g o.Gomega) {
			sc, err := env.ScyllaClient().ScyllaV1().ScyllaClusters(env.Namespace()).Get(ctx, sc.Name, metav1.GetOptions{})
			g.Expect(err).NotTo(o.HaveOccurred())

			degradedCond := apimeta.FindStatusCondition(sc.Status.Conditions, scyllav1.DegradedCondition)
			g.Expect(degradedCond).NotTo(o.BeNil(), "Degraded condition should be present")
			g.Expect(degradedCond.Status).To(o.Equal(metav1.ConditionTrue), "Degraded should be True")

			progressingCond := apimeta.FindStatusCondition(sc.Status.Conditions, scyllav1.ProgressingCondition)
			g.Expect(progressingCond).NotTo(o.BeNil(), "Progressing condition should be present")
			g.Expect(progressingCond.Status).To(o.Equal(metav1.ConditionFalse), "Progressing should be False")
		}).WithTimeout(30 * time.Second).WithContext(ctx).Should(o.Succeed())
	})

	g.It("reports Progressing=Unknown,Degraded=Unknown before SDC status is available", func(ctx g.SpecContext) {
		g.By("Starting ScyllaCluster controller")
		runScyllaClusterController(ctx, env)

		sc := newBasicScyllaCluster("awaiting", env.Namespace())
		sc, _ = createScyllaClusterAndWaitForScyllaDBDatacenterToExist(ctx, env, sc)

		// On the first sync the controller applies the SDC, which sets ScyllaDBDatacenterControllerProgressing=True,
		// causing the aggregate to be True.
		// We wait for the state to settle to Progressing=Unknown,Degraded=Unknown before proceeding with the assertion to avoid flakes from racing with the first sync.
		g.By("Waiting for ScyllaCluster to report Progressing=Unknown, Degraded=Unknown (SDC stable, no status yet)")
		o.Eventually(func(g o.Gomega) {
			sc, err := env.ScyllaClient().ScyllaV1().ScyllaClusters(env.Namespace()).Get(ctx, sc.Name, metav1.GetOptions{})
			g.Expect(err).NotTo(o.HaveOccurred())

			progressingCond := apimeta.FindStatusCondition(sc.Status.Conditions, scyllav1.ProgressingCondition)
			g.Expect(progressingCond).NotTo(o.BeNil(), "Progressing condition should be present")
			g.Expect(progressingCond.Status).To(o.Equal(metav1.ConditionUnknown), "Progressing should be Unknown while awaiting SDC status")

			degradedCond := apimeta.FindStatusCondition(sc.Status.Conditions, scyllav1.DegradedCondition)
			g.Expect(degradedCond).NotTo(o.BeNil(), "Degraded condition should be present")
			g.Expect(degradedCond.Status).To(o.Equal(metav1.ConditionUnknown), "Degraded should be Unknown while awaiting SDC status")
		}).WithTimeout(30 * time.Second).WithContext(ctx).Should(o.Succeed())

		g.By("Consistently expecting ScyllaCluster to remain Progressing=Unknown, Degraded=Unknown (no SDC status)")
		o.Consistently(func(g o.Gomega) {
			sc, err := env.ScyllaClient().ScyllaV1().ScyllaClusters(env.Namespace()).Get(ctx, sc.Name, metav1.GetOptions{})
			g.Expect(err).NotTo(o.HaveOccurred())

			progressingCond := apimeta.FindStatusCondition(sc.Status.Conditions, scyllav1.ProgressingCondition)
			g.Expect(progressingCond).NotTo(o.BeNil(), "Progressing condition should be present")
			g.Expect(progressingCond.Status).To(o.Equal(metav1.ConditionUnknown), "Progressing should be Unknown while awaiting SDC status")

			degradedCond := apimeta.FindStatusCondition(sc.Status.Conditions, scyllav1.DegradedCondition)
			g.Expect(degradedCond).NotTo(o.BeNil(), "Degraded condition should be present")
			g.Expect(degradedCond.Status).To(o.Equal(metav1.ConditionUnknown), "Degraded should be Unknown while awaiting SDC status")
		}).WithTimeout(10 * time.Second).WithContext(ctx).Should(o.Succeed())
	})

	// This test verifies the generation skew compensation.
	//
	// When sc.Spec.Sysctls changes, sc.Generation increments but sdc.Generation does not, because the SC spec update only translated to an annotation update in SDC.
	// The controller compensates by offsetting all SDC conditions' ObservedGeneration.
	//
	// This test verifies that after the sysctl update:
	// - Progressing and Degraded remain False (not Unknown), proving the shift worked.
	// - Their ObservedGeneration on the SC status equals the new sc.Generation.
	g.It("adjusts SDC condition ObservedGenerations by the generation skew (SC > SDC)", func(ctx g.SpecContext) {
		g.By("Starting ScyllaCluster controller")
		runScyllaClusterController(ctx, env)

		sc := newBasicScyllaCluster("sysctl-skew", env.Namespace())
		sc, sdc := createScyllaClusterAndWaitForScyllaDBDatacenterToExist(ctx, env, sc)
		sdcGen := sdc.Generation
		_ = mockHealthyScyllaDBDatacenterConditions(ctx, env.ScyllaClient(), sdc)
		sc = waitForScyllaClusterToSettleWithHealthyConditions(ctx, env.ScyllaClient(), sc.Namespace, sc.Name)

		// Update sc.Spec.Sysctls: this bumps sc.Generation by 1 but leaves sdc.Generation
		// unchanged because sysctls are stored only as an SDC annotation, not in sdc.Spec.
		g.By("Updating sc.Spec.Sysctls to introduce a generation skew (sc.Generation > sdc.Generation)")
		sc, err := env.ScyllaClient().ScyllaV1().ScyllaClusters(env.Namespace()).Get(ctx, sc.Name, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		scCopy := sc.DeepCopy()
		scCopy.Spec.Sysctls = []string{"fs.aio-max-nr=1048576"}
		sc, err = env.ScyllaClient().ScyllaV1().ScyllaClusters(env.Namespace()).Update(ctx, scCopy, metav1.UpdateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Verifying that the SDC annotation is updated by the controller but sdc.Generation stays the same")
		o.Eventually(func(g o.Gomega) {
			sdc, err := env.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(env.Namespace()).Get(ctx, sc.Name, metav1.GetOptions{})
			g.Expect(err).NotTo(o.HaveOccurred())
			g.Expect(sdc.Annotations).To(o.HaveKey(naming.TransformScyllaClusterToScyllaDBDatacenterSysctlsAnnotation),
				"SDC annotation for sysctls should be set by the controller")
			g.Expect(sdc.Generation).To(o.Equal(sdcGen),
				"sdc.Generation must not change because sysctls are stored in annotations, not spec")
		}).WithTimeout(30 * time.Second).WithContext(ctx).Should(o.Succeed())

		g.By("Expecting SC to report healthy conditions with generation skew compensation")
		_ = waitForScyllaClusterToSettleWithHealthyConditions(ctx, env.ScyllaClient(), sc.Namespace, sc.Name)
	})

	// This test verifies the generation skew compensation in the opposite, simulating a situation where a user interacts directly with SDC owned by SC.
	//
	// When the SDC spec is updated directly, sdc.Generation increments but sc.Generation does not.
	// The SDC controller then reconciles and reports conditions at the new sdc.Generation.
	// The SC controller compensates by shifting SDC condition ObservedGenerations back into SC's generation space.
	//
	// This test verifies that after the SDC spec update and SDC controller reconciliation:
	// - Progressing and Degraded remain False (not Unknown), proving the negative shift worked.
	// - Their ObservedGeneration on the SC status equals sc.Generation.
	g.It("adjusts SDC condition ObservedGenerations by the generation skew (SDC > SC)", func(ctx g.SpecContext) {
		g.By("Starting ScyllaCluster controller")
		runScyllaClusterController(ctx, env)

		basicSC := newBasicScyllaCluster("sdc-skew", env.Namespace())
		basicSC.Spec.MinReadySeconds = pointer.Ptr(int32(30))
		sc, sdc := createScyllaClusterAndWaitForScyllaDBDatacenterToExist(ctx, env, basicSC)

		scGeneration := sc.Generation
		_ = mockHealthyScyllaDBDatacenterConditions(ctx, env.ScyllaClient(), sdc)
		sc = waitForScyllaClusterToSettleWithHealthyConditions(ctx, env.ScyllaClient(), sc.Namespace, sc.Name)

		g.By("Bumping sdc.Generation with a direct update of SDC spec")
		sdc, err := env.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(env.Namespace()).Get(ctx, sc.Name, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		sdcCopy := sdc.DeepCopy()
		sdcCopy.Spec.MinReadySeconds = pointer.Ptr(int32(31))
		sdc, err = env.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(env.Namespace()).Update(ctx, sdcCopy, metav1.UpdateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		// NOTE: we do not verify that the spec change has been reconciled back to the declared value by the SC controller
		// as the apply operation is only executed on a managed hash annotation change.

		o.Expect(sdc.Generation).To(o.BeNumerically(">", scGeneration),
			"sdc.Generation must exceed sc.Generation")

		g.By("Patching SDC status with healthy conditions at the final sdc.Generation, simulating SDC controller reconciliation")
		sdc = mockHealthyScyllaDBDatacenterConditions(ctx, env.ScyllaClient(), sdc)

		g.By("Expecting SC to report healthy conditions with generation skew compensation")
		_ = waitForScyllaClusterToSettleWithHealthyConditions(ctx, env.ScyllaClient(), sc.Namespace, sc.Name)
	})

	g.It("reports Degraded=True when ScyllaDBDatacenter apply is rejected", func(ctx g.SpecContext) {
		var rejectSDC atomic.Bool

		g.By("Installing validating webhook (initially allowing)")
		envtest.SetupMockValidatingWebhook(ctx, env, func(ar *admissionv1.AdmissionReview) ([]string, error) {
			if rejectSDC.Load() && ar.Request.Resource.Resource == "scylladbdatacenters" {
				return nil, fmt.Errorf("injected SDC webhook rejection")
			}
			return nil, nil
		})

		g.By("Starting ScyllaCluster controller")
		runScyllaClusterController(ctx, env)

		sc := newBasicScyllaCluster("webhook-degraded", env.Namespace())
		sc, sdc := createScyllaClusterAndWaitForScyllaDBDatacenterToExist(ctx, env, sc)
		_ = mockHealthyScyllaDBDatacenterConditions(ctx, env.ScyllaClient(), sdc)
		sc = waitForScyllaClusterToSettleWithHealthyConditions(ctx, env.ScyllaClient(), sc.Namespace, sc.Name)

		g.By("Enabling webhook rejection and triggering an SDC re-apply via a sysctl change")
		rejectSDC.Store(true)

		sc, err := env.ScyllaClient().ScyllaV1().ScyllaClusters(env.Namespace()).Get(ctx, sc.Name, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		scCopy := sc.DeepCopy()
		scCopy.Spec.Sysctls = []string{"fs.aio-max-nr=1048576"}
		_, err = env.ScyllaClient().ScyllaV1().ScyllaClusters(env.Namespace()).Update(ctx, scCopy, metav1.UpdateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Expecting ScyllaCluster to report Degraded=True due to rejected SDC apply")
		o.Eventually(func(g o.Gomega) {
			sc, err = env.ScyllaClient().ScyllaV1().ScyllaClusters(env.Namespace()).Get(ctx, sc.Name, metav1.GetOptions{})
			g.Expect(err).NotTo(o.HaveOccurred())

			degradedCond := apimeta.FindStatusCondition(sc.Status.Conditions, scyllav1.DegradedCondition)
			g.Expect(degradedCond).NotTo(o.BeNil(), "Degraded condition should be present")
			g.Expect(degradedCond.Status).To(o.Equal(metav1.ConditionTrue),
				"Degraded should be True because the webhook rejected the SDC apply")
		}).WithTimeout(30 * time.Second).WithContext(ctx).Should(o.Succeed())
	})

	g.It("sets Degraded=True when ScyllaDBManagerTask apply is rejected", func(ctx g.SpecContext) {
		var rejectSMT atomic.Bool

		g.By("Installing validating webhook (initially allowing all)")
		envtest.SetupMockValidatingWebhook(ctx, env, func(ar *admissionv1.AdmissionReview) ([]string, error) {
			if rejectSMT.Load() && ar.Request.Resource.Resource == "scylladbmanagertasks" {
				return nil, fmt.Errorf("injected SMT webhook rejection")
			}
			return nil, nil
		})

		g.By("Starting ScyllaCluster controller")
		runScyllaClusterController(ctx, env)

		sc := newBasicScyllaCluster("smt-degraded", env.Namespace())
		sc, sdc := createScyllaClusterAndWaitForScyllaDBDatacenterToExist(ctx, env, sc)
		_ = mockHealthyScyllaDBDatacenterConditions(ctx, env.ScyllaClient(), sdc)
		sc = waitForScyllaClusterToSettleWithHealthyConditions(ctx, env.ScyllaClient(), sc.Namespace, sc.Name)

		g.By("Enabling SMT webhook rejection and adding a repair task to trigger an SMT apply")
		rejectSMT.Store(true)

		sc, err := env.ScyllaClient().ScyllaV1().ScyllaClusters(env.Namespace()).Get(ctx, sc.Name, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		scCopy := sc.DeepCopy()
		scCopy.Spec.Repairs = []scyllav1.RepairTaskSpec{
			{
				TaskSpec: scyllav1.TaskSpec{
					Name: "weekly",
				},
			},
		}
		_, err = env.ScyllaClient().ScyllaV1().ScyllaClusters(env.Namespace()).Update(ctx, scCopy, metav1.UpdateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Expecting ScyllaCluster to report Degraded=True due to rejected SMT apply")
		o.Eventually(func(g o.Gomega) {
			sc, err = env.ScyllaClient().ScyllaV1().ScyllaClusters(env.Namespace()).Get(ctx, sc.Name, metav1.GetOptions{})
			g.Expect(err).NotTo(o.HaveOccurred())

			degradedCond := apimeta.FindStatusCondition(sc.Status.Conditions, scyllav1.DegradedCondition)
			g.Expect(degradedCond).NotTo(o.BeNil(), "Degraded condition should be present")
			g.Expect(degradedCond.Status).To(o.Equal(metav1.ConditionTrue),
				"Degraded should be True because the webhook rejected the SMT apply")
		}).WithTimeout(30 * time.Second).WithContext(ctx).Should(o.Succeed())
	})

})

// createScyllaClusterAndWaitForScyllaDBDatacenterToExist creates the given ScyllaCluster and polls until the controller
// has created the corresponding ScyllaDBDatacenter. Returns the latest SC and SDC objects.
func createScyllaClusterAndWaitForScyllaDBDatacenterToExist(ctx context.Context, e *envtest.Environment, sc *scyllav1.ScyllaCluster) (*scyllav1.ScyllaCluster, *scyllav1alpha1.ScyllaDBDatacenter) {
	g.GinkgoHelper()

	g.By("Creating ScyllaCluster")
	var err error
	sc, err = e.ScyllaClient().ScyllaV1().ScyllaClusters(e.Namespace()).Create(ctx, sc, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	g.By("Waiting for ScyllaDBDatacenter to be created by the controller")
	var sdc *scyllav1alpha1.ScyllaDBDatacenter
	o.Eventually(func(eg o.Gomega) {
		sdc, err = e.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(e.Namespace()).Get(ctx, sc.Name, metav1.GetOptions{})
		eg.Expect(err).NotTo(o.HaveOccurred())
	}).WithTimeout(30 * time.Second).WithContext(ctx).Should(o.Succeed())

	return sc, sdc
}

// mockHealthyScyllaDBDatacenterConditions patches the given SDC's status with healthy conditions
// (Progressing=False, Degraded=False, Available=True) at the SDC's current generation,
// mocking a healthy SDC controller reconciliation. Returns the latest SDC object.
func mockHealthyScyllaDBDatacenterConditions(ctx context.Context, scyllaClient scyllaversionedclient.Interface, sdc *scyllav1alpha1.ScyllaDBDatacenter) *scyllav1alpha1.ScyllaDBDatacenter {
	g.GinkgoHelper()

	g.By("Mocking SDC status with Progressing=False, Degraded=False, Available=True")
	sdcCopy := sdc.DeepCopy()
	sdcCopy.Status.Conditions = []metav1.Condition{
		newCondition(scyllav1alpha1.ProgressingCondition, metav1.ConditionFalse, internalapi.AsExpectedReason, "", sdc.Generation),
		newCondition(scyllav1alpha1.DegradedCondition, metav1.ConditionFalse, internalapi.AsExpectedReason, "", sdc.Generation),
		newCondition(scyllav1alpha1.AvailableCondition, metav1.ConditionTrue, internalapi.AsExpectedReason, "", sdc.Generation),
		newCondition("StatefulSetControllerProgressing", metav1.ConditionFalse, internalapi.AsExpectedReason, "", sdc.Generation),
		newCondition("StatefulSetControllerDegraded", metav1.ConditionFalse, internalapi.AsExpectedReason, "", sdc.Generation),
		newCondition("StatefulSetControllerAvailable", metav1.ConditionTrue, internalapi.AsExpectedReason, "", sdc.Generation),
	}
	sdc, err := scyllaClient.ScyllaV1alpha1().ScyllaDBDatacenters(sdc.Namespace).UpdateStatus(ctx, sdcCopy, metav1.UpdateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	return sdc
}

// waitForScyllaClusterToSettleWithHealthyConditions waits for the SC to settle with Progressing=False, Degraded=False,
// Available=True and conditions' ObservedGenerations equal to SC's generation.
// Returns the latest SC object.
func waitForScyllaClusterToSettleWithHealthyConditions(ctx context.Context, scyllaClient scyllaversionedclient.Interface, namespace, name string) *scyllav1.ScyllaCluster {
	g.GinkgoHelper()

	g.By("Waiting for ScyllaCluster to settle with Progressing=False, Degraded=False, Available=True with conditions' ObservedGeneration equal to SC's generation")
	var sc *scyllav1.ScyllaCluster
	o.Eventually(func(eo o.Gomega) {
		var err error
		sc, err = scyllaClient.ScyllaV1().ScyllaClusters(namespace).Get(ctx, name, metav1.GetOptions{})
		eo.Expect(err).NotTo(o.HaveOccurred())

		progressingCond := apimeta.FindStatusCondition(sc.Status.Conditions, scyllav1.ProgressingCondition)
		eo.Expect(progressingCond).NotTo(o.BeNil())
		eo.Expect(progressingCond.Status).To(o.Equal(metav1.ConditionFalse))
		eo.Expect(progressingCond.ObservedGeneration).To(o.Equal(sc.Generation))

		degradedCond := apimeta.FindStatusCondition(sc.Status.Conditions, scyllav1.DegradedCondition)
		eo.Expect(degradedCond).NotTo(o.BeNil())
		eo.Expect(degradedCond.Status).To(o.Equal(metav1.ConditionFalse))
		eo.Expect(degradedCond.ObservedGeneration).To(o.Equal(sc.Generation))

		availableCond := apimeta.FindStatusCondition(sc.Status.Conditions, scyllav1.AvailableCondition)
		eo.Expect(availableCond).NotTo(o.BeNil())
		eo.Expect(availableCond.Status).To(o.Equal(metav1.ConditionTrue))
		eo.Expect(availableCond.ObservedGeneration).To(o.Equal(sc.Generation))
	}).WithTimeout(30 * time.Second).WithContext(ctx).Should(o.Succeed())

	return sc
}

// newBasicScyllaCluster returns a minimal valid *scyllav1.ScyllaCluster for use in envtest.
func newBasicScyllaCluster(name, namespace string) *scyllav1.ScyllaCluster {
	g.GinkgoHelper()

	return &scyllav1.ScyllaCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: scyllav1.ScyllaClusterSpec{
			Version:         "6.2.0",
			Repository:      "docker.io/scylladb/scylla",
			AgentVersion:    "3.3.2",
			AgentRepository: "docker.io/scylladb/scylla-manager-agent",
			Datacenter: scyllav1.DatacenterSpec{
				Name: "dc1",
				Racks: []scyllav1.RackSpec{
					{
						Name:    "rack1",
						Members: 1,
						Storage: scyllav1.Storage{
							Capacity: "10Gi",
						},
						Resources: corev1.ResourceRequirements{
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("1"),
								corev1.ResourceMemory: resource.MustParse("1Gi"),
							},
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("1"),
								corev1.ResourceMemory: resource.MustParse("1Gi"),
							},
						},
					},
				},
			},
		},
	}
}

// newCondition returns a metav1.Condition with LastTransitionTime set to now.
func newCondition(condType string, status metav1.ConditionStatus, reason, message string, observedGeneration int64) metav1.Condition {
	g.GinkgoHelper()

	return metav1.Condition{
		Type:               condType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: observedGeneration,
		LastTransitionTime: metav1.Now(),
	}
}

// runScyllaClusterController creates and starts a ScyllaCluster controller against the given envtest environment.
// The controller is stopped automatically when ctx is cancelled. Informer factories are shut down and the
// controller goroutine is drained via DeferCleanup before the envtest environment is torn down.
func runScyllaClusterController(ctx context.Context, e *envtest.Environment) {
	g.GinkgoHelper()

	const resyncPeriod = 12 * time.Hour

	kubeInformers := informers.NewSharedInformerFactoryWithOptions(
		e.TypedKubeClient(),
		resyncPeriod,
		informers.WithNamespace(e.Namespace()),
	)
	scyllaInformers := scyllainformers.NewSharedInformerFactoryWithOptions(
		e.ScyllaClient(),
		resyncPeriod,
		scyllainformers.WithNamespace(e.Namespace()),
	)

	scc, err := scyllacluster.NewController(
		e.TypedKubeClient(),
		e.ScyllaClient(),
		kubeInformers.Core().V1().Services(),
		kubeInformers.Core().V1().Secrets(),
		kubeInformers.Core().V1().ConfigMaps(),
		kubeInformers.Core().V1().ServiceAccounts(),
		kubeInformers.Rbac().V1().RoleBindings(),
		kubeInformers.Apps().V1().StatefulSets(),
		kubeInformers.Policy().V1().PodDisruptionBudgets(),
		kubeInformers.Networking().V1().Ingresses(),
		kubeInformers.Batch().V1().Jobs(),
		scyllaInformers.Scylla().V1().ScyllaClusters(),
		scyllaInformers.Scylla().V1alpha1().ScyllaDBDatacenters(),
		scyllaInformers.Scylla().V1alpha1().ScyllaDBManagerClusterRegistrations(),
		scyllaInformers.Scylla().V1alpha1().ScyllaDBManagerTasks(),
	)
	o.Expect(err).NotTo(o.HaveOccurred(), "Failed to create ScyllaCluster controller")

	kubeInformers.Start(ctx.Done())
	scyllaInformers.Start(ctx.Done())

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		scc.Run(ctx, 1)
	}()

	g.DeferCleanup(func() {
		kubeInformers.Shutdown()
		scyllaInformers.Shutdown()
		wg.Wait()
	})
}
