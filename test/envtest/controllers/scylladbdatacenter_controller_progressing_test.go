//go:build envtest

package controllers

import (
	"context"
	"fmt"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/internalapi"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	"github.com/scylladb/scylla-operator/test/envtest"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
)

// The StatefulSet sync waits for its inputs and reports why, and it degrades on inputs it can't act on. These specs
// pin the reasons and the effects on the rack StatefulSets.
var _ = g.Describe("ScyllaDBDatacenter controller StatefulSet sync progress", func() {
	const rackName = "rack-a"

	var env *envtest.Environment
	g.BeforeEach(func(ctx g.SpecContext) {
		env = envtest.Setup(ctx)
	})

	g.It("should wait for the ScyllaDB node exporter image before creating any StatefulSet", func(ctx g.SpecContext) {
		g.By("Running ScyllaDBDatacenter controller")
		runScyllaDBDatacenterController(ctx, env)

		g.By("Creating ScyllaOperatorConfig singleton without the images in its status")
		createScyllaOperatorConfigWithoutStatus(ctx, env)

		g.By("Creating a ScyllaDBDatacenter with a single rack")
		sdc := makeEnvtestScyllaDBDatacenter(env.Namespace(), []string{rackName})
		sdc, err := env.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(env.Namespace()).Create(ctx, sdc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		rackStatefulSetName := naming.StatefulSetNameForRack(sdc.Spec.Racks[0], sdc)

		g.By("Waiting for the StatefulSet sync to report waiting for the node exporter image")
		waitForStatefulSetControllerProgressingReason(ctx, env, sdc.Name, "WaitingForScyllaDBNodeExporterImage")

		g.By("Verifying the rack StatefulSet is not created")
		o.Consistently(func(co o.Gomega, ctx context.Context) {
			_, err := env.TypedKubeClient().AppsV1().StatefulSets(env.Namespace()).Get(ctx, rackStatefulSetName, metav1.GetOptions{})
			co.Expect(apierrors.IsNotFound(err)).To(o.BeTrue())
		}).WithContext(ctx).WithTimeout(scyllaDBDatacenterControllerDefaultConsistentlyTimeout).WithPolling(100 * time.Millisecond).Should(o.Succeed())

		g.By("Setting the node exporter image in the ScyllaOperatorConfig status")
		setScyllaOperatorConfigScyllaDBNodeExporterImage(ctx, env)

		g.By("Waiting for the rack StatefulSet to be created")
		waitForStatefulSet(ctx, env, rackStatefulSetName, scyllaDBDatacenterControllerDefaultEventuallyTimeout)
	})

	g.It("should wait for the managed config while its ConfigMap is owned by another controller", func(ctx g.SpecContext) {
		g.By("Running ScyllaDBDatacenter controller")
		runScyllaDBDatacenterController(ctx, env)

		g.By("Creating ScyllaOperatorConfig singleton")
		createScyllaOperatorConfig(ctx, env)

		sdc := makeEnvtestScyllaDBDatacenter(env.Namespace(), []string{rackName})
		rackStatefulSetName := naming.StatefulSetNameForRack(sdc.Spec.Racks[0], sdc)
		managedConfigMapName := naming.GetScyllaDBManagedConfigCMName(sdc.Name)

		g.By("Creating a ConfigMap under the managed config name owned by another controller")
		foreignConfigMap, err := env.TypedKubeClient().CoreV1().ConfigMaps(env.Namespace()).Create(ctx, &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:            managedConfigMapName,
				Namespace:       env.Namespace(),
				OwnerReferences: []metav1.OwnerReference{makeEnvtestForeignControllerRef()},
			},
			Data: map[string]string{"foreign": "config"},
		}, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Creating a ScyllaDBDatacenter with a single rack")
		sdc, err = env.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(env.Namespace()).Create(ctx, sdc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Waiting for the config sync to be reported as degraded and the StatefulSet sync to wait for the managed config")
		o.Eventually(func(eo o.Gomega, ctx context.Context) {
			configDegradedCondition := getScyllaDBDatacenterCondition(ctx, env, sdc.Name, internalapi.MakeKindControllerCondition("Config", scyllav1alpha1.DegradedCondition))
			eo.Expect(configDegradedCondition).NotTo(o.BeNil())
			eo.Expect(configDegradedCondition.Status).To(o.Equal(metav1.ConditionTrue))
			eo.Expect(configDegradedCondition.Reason).To(o.Equal(internalapi.ErrorReason))

			progressingCondition := getStatefulSetControllerProgressingCondition(ctx, env, sdc.Name)
			eo.Expect(progressingCondition).NotTo(o.BeNil())
			eo.Expect(progressingCondition.Status).To(o.Equal(metav1.ConditionTrue))
			eo.Expect(progressingCondition.Reason).To(o.Equal("WaitingForManagedConfig"))
		}).WithContext(ctx).WithTimeout(scyllaDBDatacenterControllerDefaultEventuallyTimeout).WithPolling(100 * time.Millisecond).Should(o.Succeed())

		g.By("Verifying the rack StatefulSet is not created and the ConfigMap is left untouched")
		o.Consistently(func(co o.Gomega, ctx context.Context) {
			_, err := env.TypedKubeClient().AppsV1().StatefulSets(env.Namespace()).Get(ctx, rackStatefulSetName, metav1.GetOptions{})
			co.Expect(apierrors.IsNotFound(err)).To(o.BeTrue())

			cm, err := env.TypedKubeClient().CoreV1().ConfigMaps(env.Namespace()).Get(ctx, managedConfigMapName, metav1.GetOptions{})
			co.Expect(err).NotTo(o.HaveOccurred())
			co.Expect(cm.UID).To(o.Equal(foreignConfigMap.UID))
			co.Expect(cm.ResourceVersion).To(o.Equal(foreignConfigMap.ResourceVersion))
		}).WithContext(ctx).WithTimeout(scyllaDBDatacenterControllerDefaultConsistentlyTimeout).WithPolling(100 * time.Millisecond).Should(o.Succeed())
	})

	g.It("should report an invalid rack with a degraded StatefulSet sync", func(ctx g.SpecContext) {
		g.By("Creating ScyllaOperatorConfig singleton")
		createScyllaOperatorConfig(ctx, env)

		g.By("Creating a ScyllaDBDatacenter with a single rack")
		sdc := makeEnvtestScyllaDBDatacenter(env.Namespace(), []string{rackName})
		sdc, err := env.ScyllaClient().ScyllaV1alpha1().ScyllaDBDatacenters(env.Namespace()).Create(ctx, sdc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		rackStatefulSetName := naming.StatefulSetNameForRack(sdc.Spec.Racks[0], sdc)

		// The rack StatefulSet is made from the existing one, whose data volume claim template is immutable. One that
		// lacks it can't be reconciled into a rack.
		g.By("Creating an owned rack StatefulSet without the data volume claim template before the controller runs")
		invalidStatefulSet := makeEnvtestForeignStatefulSet(env.Namespace(), rackStatefulSetName, naming.ScyllaDBDatacenterSelectorLabels(sdc))
		invalidStatefulSet.Labels[naming.RackNameLabel] = rackName
		invalidStatefulSet.OwnerReferences = []metav1.OwnerReference{*metav1.NewControllerRef(sdc, scyllav1alpha1.ScyllaDBDatacenterGVK)}
		invalidStatefulSet, err = env.TypedKubeClient().AppsV1().StatefulSets(env.Namespace()).Create(ctx, invalidStatefulSet, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Running ScyllaDBDatacenter controller")
		runScyllaDBDatacenterController(ctx, env)

		g.By("Waiting for the StatefulSet sync to be reported as degraded")
		o.Eventually(func(eo o.Gomega, ctx context.Context) {
			degradedCondition := getScyllaDBDatacenterCondition(ctx, env, sdc.Name, internalapi.MakeKindControllerCondition("StatefulSet", scyllav1alpha1.DegradedCondition))
			eo.Expect(degradedCondition).NotTo(o.BeNil())
			eo.Expect(degradedCondition.Status).To(o.Equal(metav1.ConditionTrue))
			eo.Expect(degradedCondition.Reason).To(o.Equal(internalapi.ErrorReason))
		}).WithContext(ctx).WithTimeout(scyllaDBDatacenterControllerDefaultEventuallyTimeout).WithPolling(100 * time.Millisecond).Should(o.Succeed())

		g.By("Verifying the invalid StatefulSet is left as is")
		o.Consistently(func(co o.Gomega, ctx context.Context) {
			sts, err := env.TypedKubeClient().AppsV1().StatefulSets(env.Namespace()).Get(ctx, rackStatefulSetName, metav1.GetOptions{})
			co.Expect(err).NotTo(o.HaveOccurred())
			co.Expect(sts.UID).To(o.Equal(invalidStatefulSet.UID))
			co.Expect(sts.Generation).To(o.Equal(invalidStatefulSet.Generation))
			co.Expect(sts.DeletionTimestamp).To(o.BeNil())
		}).WithContext(ctx).WithTimeout(scyllaDBDatacenterControllerDefaultConsistentlyTimeout).WithPolling(100 * time.Millisecond).Should(o.Succeed())
	})

	// A rack's pending scale is applied without waiting for its own StatefulSet to roll out, as a scale-down has to
	// reach a rack whose highest node is the one that isn't ready. The rollout of every other rack is still waited
	// for, so that a scale can't overlap with another rack's rollout.
	g.It("should scale a rack whose StatefulSet is not rolled out, but hold a scale while another rack's is not", func(ctx g.SpecContext) {
		const otherRackName = "rack-b"

		sdc := setupRolledOutRacks(ctx, env, false, []string{rackName, otherRackName}, 1)
		rackStatefulSetName := naming.StatefulSetNameForRack(sdc.Spec.Racks[0], sdc)
		otherRackStatefulSetName := naming.StatefulSetNameForRack(sdc.Spec.Racks[1], sdc)

		g.By(fmt.Sprintf("Marking the %q rack StatefulSet as not rolled out", rackName))
		markStatefulSetAsNotRolledOut(ctx, env.TypedKubeClient().AppsV1().StatefulSets(env.Namespace()), rackStatefulSetName)

		g.By(fmt.Sprintf("Scaling the %q rack up to two nodes", rackName))
		scaleRack(ctx, env, sdc.Name, rackName, 2)

		g.By(fmt.Sprintf("Waiting for the %q rack StatefulSet to be scaled up without rolling out first", rackName))
		waitForStatefulSetReplicas(ctx, env, rackStatefulSetName, 2)

		g.By(fmt.Sprintf("Scaling the %q rack up to two nodes while the %q rack is not rolled out", otherRackName, rackName))
		scaleRack(ctx, env, sdc.Name, otherRackName, 2)

		g.By(fmt.Sprintf("Waiting for the %q rack's rollout to be reported as awaited", rackName))
		o.Eventually(func(eo o.Gomega, ctx context.Context) {
			progressingCondition := getStatefulSetControllerProgressingCondition(ctx, env, sdc.Name)
			eo.Expect(progressingCondition).NotTo(o.BeNil())
			eo.Expect(progressingCondition.Status).To(o.Equal(metav1.ConditionTrue))
			eo.Expect(progressingCondition.Reason).To(o.Equal("WaitingForStatefulSetRollout"))
			eo.Expect(progressingCondition.Message).To(o.ContainSubstring(rackStatefulSetName))
		}).WithContext(ctx).WithTimeout(scyllaDBDatacenterControllerDefaultEventuallyTimeout).WithPolling(100 * time.Millisecond).Should(o.Succeed())

		g.By(fmt.Sprintf("Verifying the %q rack StatefulSet is not scaled while the %q rack is not rolled out", otherRackName, rackName))
		o.Consistently(func(co o.Gomega, ctx context.Context) {
			sts, err := env.TypedKubeClient().AppsV1().StatefulSets(env.Namespace()).Get(ctx, otherRackStatefulSetName, metav1.GetOptions{})
			co.Expect(err).NotTo(o.HaveOccurred())
			co.Expect(*sts.Spec.Replicas).To(o.Equal(int32(1)))
		}).WithContext(ctx).WithTimeout(scyllaDBDatacenterControllerDefaultConsistentlyTimeout).WithPolling(100 * time.Millisecond).Should(o.Succeed())

		g.By(fmt.Sprintf("Marking the %q rack StatefulSet as rolled out", rackName))
		markStatefulSetAsRolledOut(ctx, env.TypedKubeClient().AppsV1().StatefulSets(env.Namespace()), rackStatefulSetName)

		g.By(fmt.Sprintf("Waiting for the %q rack StatefulSet to be scaled up", otherRackName))
		waitForStatefulSetReplicas(ctx, env, otherRackStatefulSetName, 2)
	})
})

// waitForStatefulSetControllerProgressingReason waits for the StatefulSet sync to report progressing with the given
// reason.
func waitForStatefulSetControllerProgressingReason(ctx context.Context, e *envtest.Environment, sdcName, reason string) {
	g.GinkgoHelper()

	o.Eventually(func(eo o.Gomega, ctx context.Context) {
		progressingCondition := getStatefulSetControllerProgressingCondition(ctx, e, sdcName)
		eo.Expect(progressingCondition).NotTo(o.BeNil())
		eo.Expect(progressingCondition.Status).To(o.Equal(metav1.ConditionTrue))
		eo.Expect(progressingCondition.Reason).To(o.Equal(reason))
	}).WithContext(ctx).WithTimeout(scyllaDBDatacenterControllerDefaultEventuallyTimeout).WithPolling(100 * time.Millisecond).Should(o.Succeed())
}

// createScyllaOperatorConfigWithoutStatus creates the ScyllaOperatorConfig singleton with none of the images the
// controllers read from its status, as it is before the ScyllaOperatorConfig controller has resolved them.
func createScyllaOperatorConfigWithoutStatus(ctx context.Context, e *envtest.Environment) *scyllav1alpha1.ScyllaOperatorConfig {
	g.GinkgoHelper()

	soc, err := e.ScyllaClient().ScyllaV1alpha1().ScyllaOperatorConfigs().Create(ctx, &scyllav1alpha1.ScyllaOperatorConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: naming.SingletonName,
		},
		Spec: scyllav1alpha1.ScyllaOperatorConfigSpec{
			ScyllaUtilsImage: "docker.io/scylladb/scylla:6.2.0",
		},
	}, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	return soc
}

// setScyllaOperatorConfigScyllaDBNodeExporterImage sets the node exporter image in the status of the
// ScyllaOperatorConfig singleton, in place of the ScyllaOperatorConfig controller.
func setScyllaOperatorConfigScyllaDBNodeExporterImage(ctx context.Context, e *envtest.Environment) {
	g.GinkgoHelper()

	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		soc, err := e.ScyllaClient().ScyllaV1alpha1().ScyllaOperatorConfigs().Get(ctx, naming.SingletonName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("can't get ScyllaOperatorConfig %q: %w", naming.SingletonName, err)
		}

		soc.Status.ScyllaDBNodeExporterImage = pointer.Ptr("docker.io/scylladb/scylla-operator-node-exporter:latest")
		_, err = e.ScyllaClient().ScyllaV1alpha1().ScyllaOperatorConfigs().UpdateStatus(ctx, soc, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("can't update status of ScyllaOperatorConfig %q: %w", naming.ObjRef(soc), err)
		}

		return nil
	})
	o.Expect(err).NotTo(o.HaveOccurred())
}
