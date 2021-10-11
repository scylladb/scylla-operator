// Copyright (C) 2021 ScyllaDB

package scyllanodeconfig

import (
	"context"
	"fmt"
	"strings"

	g "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	o "github.com/onsi/gomega"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controller/scyllanodeconfig/resource"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/test/e2e/fixture/scyllacluster"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
)

// These tests modify global resource affecting global cluster state.
// They must not be run asynchronously with other tests.
var _ = g.Describe("ScyllaNodeConfig Optimizations [Serial]", func() {
	defer g.GinkgoRecover()

	f := framework.NewFramework("scyllanodeconfig")

	var givenNodeConfigRunningOnAllHosts = func(ctx context.Context, name string) *scyllav1alpha1.ScyllaNodeConfig {
		snc, err := f.ScyllaAdminClient().ScyllaV1alpha1().ScyllaNodeConfigs().Create(ctx, &scyllav1alpha1.ScyllaNodeConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: naming.ScyllaOperatorNodeTuningNamespace,
			},
			Spec: scyllav1alpha1.ScyllaNodeConfigSpec{
				Placement: scyllav1alpha1.ScyllaNodeConfigPlacement{
					NodeSelector: map[string]string{},
				},
				DisableOptimizations: false,
			}}, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		return snc.DeepCopy()
	}

	var givenNodeConfigNotRunningOnAnyHost = func(ctx context.Context, name string) *scyllav1alpha1.ScyllaNodeConfig {
		snc, err := f.ScyllaAdminClient().ScyllaV1alpha1().ScyllaNodeConfigs().Patch(
			ctx,
			name,
			types.JSONPatchType,
			[]byte(`[{"op":"replace", "path": "/spec/placement/nodeSelector", "value": {"node-type": "quantum"} }]`),
			metav1.PatchOptions{},
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		return snc.DeepCopy()
	}

	var whenNodeConfigRollsOut = func(ctx context.Context, snc *scyllav1alpha1.ScyllaNodeConfig) {
		waitCtx, cancel := utils.ContextForNodeConfigRollout(ctx)
		defer cancel()
		_, err := utils.WaitForScyllaNodeConfigRollout(waitCtx, f.ScyllaAdminClient().ScyllaV1alpha1(), snc.Name)
		o.Expect(err).NotTo(o.HaveOccurred())
	}

	var configMapOwnership = func(ownership types.UID) func(*corev1.ConfigMap) (bool, error) {
		return func(cm *corev1.ConfigMap) (bool, error) {
			for _, ownerRef := range cm.OwnerReferences {
				if ownerRef.UID == ownership {
					return true, nil
				}
			}
			return false, nil
		}
	}

	var configMapHasBinaryKey = func(key string) func(configMap *corev1.ConfigMap) (bool, error) {
		return func(cm *corev1.ConfigMap) (bool, error) {
			_, ok := cm.BinaryData[key]
			return ok, nil
		}
	}

	var configMapDoesNotHaveBinaryKey = func(key string) func(configMap *corev1.ConfigMap) (bool, error) {
		return func(cm *corev1.ConfigMap) (bool, error) {
			_, ok := cm.BinaryData[key]
			return !ok, nil
		}
	}

	var thenTuningConfigMapHasState = func(ctx context.Context, sc *scyllav1.ScyllaCluster, opts utils.WaitForStateOptions, conditions ...func(cm *corev1.ConfigMap) (bool, error)) {
		ctx, cancel := utils.ContextForNodeConfigRollout(ctx)
		defer cancel()

		scyllaPod, err := f.KubeClient().CoreV1().Pods(f.Namespace()).Get(ctx, fmt.Sprintf("%s-0", naming.StatefulSetNameForRack(sc.Spec.Datacenter.Racks[0], sc)), metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		_, err = utils.WaitForConfigMap(ctx, f.KubeClient().CoreV1(), f.Namespace(), naming.PerftuneResultName(string(scyllaPod.UID)), opts, conditions...)
		o.Expect(err).NotTo(o.HaveOccurred())
	}

	g.AfterEach(func() {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		g.By("Restoring default ScyllaNodeConfig")
		snc, err := f.ScyllaAdminClient().ScyllaV1alpha1().ScyllaNodeConfigs().Get(ctx, resource.DefaultScyllaNodeConfig().Name, metav1.GetOptions{})
		o.Expect(err).ToNot(o.HaveOccurred())

		defaultSnc := snc.DeepCopy()
		defaultSnc.Spec = resource.DefaultScyllaNodeConfig().Spec
		_, err = f.ScyllaAdminClient().ScyllaV1alpha1().ScyllaNodeConfigs().Update(ctx, defaultSnc, metav1.UpdateOptions{})
		o.Expect(err).ToNot(o.HaveOccurred())
	})

	g.It("Default ScyllaNodeConfig", func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		g.By("Default NodeConfig is available")
		snc, err := f.ScyllaAdminClient().ScyllaV1alpha1().ScyllaNodeConfigs().Get(ctx, resource.DefaultScyllaNodeConfig().Name, metav1.GetOptions{})
		o.Expect(err).ToNot(o.HaveOccurred())

		g.By("Admin is able to modify values and these are not overwritten")
		_, err = f.ScyllaAdminClient().ScyllaV1alpha1().ScyllaNodeConfigs().Patch(
			ctx,
			snc.Name,
			types.JSONPatchType,
			[]byte(`[{"op":"replace", "path": "/spec/optimizationsDisabled", "value": true }]`),
			metav1.PatchOptions{},
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		snc, err = utils.WaitForScyllaNodeConfigRollout(ctx, f.ScyllaAdminClient().ScyllaV1alpha1(), snc.Name)
		o.Expect(err).NotTo(o.HaveOccurred())

		snc, err = f.ScyllaAdminClient().ScyllaV1alpha1().ScyllaNodeConfigs().Get(ctx, snc.Name, metav1.GetOptions{})
		o.Expect(err).ToNot(o.HaveOccurred())
		o.Expect(snc.Spec.DisableOptimizations).To(o.BeTrue())
	})

	type entry struct {
		ScyllaCluster               func() *scyllav1.ScyllaCluster
		OptimizedConfigMapCondition func(*corev1.ConfigMap) (bool, error)
	}

	table.DescribeTable("Tuning ConfigMap ownership and content", func(e entry) {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		framework.By("Disabling default ScyllaNodeConfig on every node")
		snc := givenNodeConfigNotRunningOnAnyHost(ctx, resource.DefaultScyllaNodeConfig().Name)

		framework.By("Waiting for ScyllaNodeConfig to rollout")
		whenNodeConfigRollsOut(ctx, snc)

		sc := e.ScyllaCluster()

		err := framework.SetupScyllaClusterSA(ctx, f.KubeClient().CoreV1(), f.KubeClient().RbacV1(), f.Namespace(), sc.Name)
		o.Expect(err).NotTo(o.HaveOccurred())

		sc, err = f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Create(ctx, sc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaCluster to deploy")
		waitCtx2, waitCtx2Cancel := utils.ContextForRollout(ctx, sc)
		defer waitCtx2Cancel()
		sc, err = utils.WaitForScyllaClusterState(waitCtx2, f.ScyllaClient().ScyllaV1(), sc.Namespace, sc.Name, utils.ScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		scyllaPods, err := f.KubeClient().CoreV1().Pods(f.Namespace()).List(ctx, metav1.ListOptions{LabelSelector: labels.SelectorFromSet(naming.ClusterLabels(sc)).String()})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(scyllaPods).To(o.HaveLen(1))
		scyllaPod := scyllaPods.Items[0]

		framework.By("Checking ownership and content of perftune ConfigMap")

		tolerateDelete := utils.WaitForStateOptions{TolerateDelete: true}
		thenTuningConfigMapHasState(ctx, sc, tolerateDelete, configMapOwnership(scyllaPod.UID), configMapDoesNotHaveBinaryKey(naming.PerftuneCommandName))

		framework.By("Allowing NodeConfig to run on host")
		snc = givenNodeConfigRunningOnAllHosts(ctx, "custom-node-config")
		defer func() {
			err := f.ScyllaAdminClient().ScyllaV1alpha1().ScyllaNodeConfigs().Delete(ctx, "custom-node-config", metav1.DeleteOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
		}()
		whenNodeConfigRollsOut(ctx, snc)

		framework.By("Checking if ownership and content of perftune ConfigMap")
		thenTuningConfigMapHasState(ctx, sc, tolerateDelete, configMapOwnership(scyllaPod.UID), e.OptimizedConfigMapCondition)

		framework.By("Restarting ScyllaCluster")
		_, err = f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Patch(
			ctx,
			sc.Name,
			types.MergePatchType,
			[]byte(fmt.Sprintf(`{"spec": {"forceRedeploymentReason": "%s"}}`, "optimizations are enabled")),
			metav1.PatchOptions{},
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		scyllaPods, err = f.KubeClient().CoreV1().Pods(f.Namespace()).List(ctx, metav1.ListOptions{LabelSelector: labels.SelectorFromSet(naming.ClusterLabels(sc)).String()})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(scyllaPods).To(o.HaveLen(1))
		scyllaPod = scyllaPods.Items[0]

		framework.By("Waiting for the ScyllaCluster to rollout")

		waitCtx3, waitCtx3Cancel := utils.ContextForRollout(ctx, sc)
		defer waitCtx3Cancel()
		sc, err = utils.WaitForScyllaClusterState(waitCtx3, f.ScyllaClient().ScyllaV1(), sc.Namespace, sc.Name, utils.ScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Checking ownership and content of perftune ConfigMap")
		thenTuningConfigMapHasState(ctx, sc, tolerateDelete, configMapOwnership(scyllaPod.UID), e.OptimizedConfigMapCondition)

		framework.By("Disabling optimizations")
		snc, err = f.ScyllaAdminClient().ScyllaV1alpha1().ScyllaNodeConfigs().Patch(
			ctx,
			snc.Name,
			types.JSONPatchType,
			[]byte(`[{"op":"replace", "path": "/spec/optimizationsDisabled", "value": true }]`),
			metav1.PatchOptions{},
		)
		o.Expect(err).NotTo(o.HaveOccurred())
		whenNodeConfigRollsOut(ctx, snc)

		framework.By("Checking ownership and content of perftune ConfigMap")
		thenTuningConfigMapHasState(ctx, sc, tolerateDelete, configMapOwnership(scyllaPod.UID), configMapDoesNotHaveBinaryKey(naming.PerftuneCommandName))

		framework.By("Disabling custom ScyllaNodeConfig on every node")
		snc = givenNodeConfigNotRunningOnAnyHost(ctx, snc.Name)
		whenNodeConfigRollsOut(ctx, snc)

		framework.By("Checking ownership and content of perftune ConfigMap")
		thenTuningConfigMapHasState(ctx, sc, tolerateDelete, configMapOwnership(scyllaPod.UID), configMapDoesNotHaveBinaryKey(naming.PerftuneCommandName))

	},
		table.Entry("guaranteed cluster", entry{
			ScyllaCluster: func() *scyllav1.ScyllaCluster {
				sc := scyllacluster.GuaranteedQoSScyllaCluster.ReadOrFail()
				sc.Spec.Datacenter.Racks[0].Members = 1
				return sc
			},
			OptimizedConfigMapCondition: configMapHasBinaryKey(naming.PerftuneCommandName),
		}),
		table.Entry("burstable cluster", entry{
			ScyllaCluster: func() *scyllav1.ScyllaCluster {
				sc := scyllacluster.BasicScyllaCluster.ReadOrFail()
				sc.Spec.Datacenter.Racks[0].Members = 1
				return sc
			},
			OptimizedConfigMapCondition: configMapDoesNotHaveBinaryKey(naming.PerftuneCommandName),
		}),
	)

	g.It("Tuning image is controlled by default ScyllaOperatorConfig", func() {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		sc := scyllacluster.GuaranteedQoSScyllaCluster.ReadOrFail()
		sc.Spec.Datacenter.Racks[0].Members = 1

		err := framework.SetupScyllaClusterSA(ctx, f.KubeClient().CoreV1(), f.KubeClient().RbacV1(), f.Namespace(), sc.Name)
		o.Expect(err).NotTo(o.HaveOccurred())

		sc, err = f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Create(ctx, sc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaCluster to deploy")
		waitCtx1, waitCtx1Cancel := utils.ContextForRollout(ctx, sc)
		defer waitCtx1Cancel()
		sc, err = utils.WaitForScyllaClusterState(waitCtx1, f.ScyllaClient().ScyllaV1(), sc.Namespace, sc.Name, utils.ScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Checking tuning container image")
		scyllaPods, err := f.KubeClient().CoreV1().Pods(sc.Namespace).List(ctx, metav1.ListOptions{LabelSelector: labels.SelectorFromSet(naming.ClusterLabels(sc)).String()})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(scyllaPods.Items).To(o.HaveLen(1))
		scyllaPod := scyllaPods.Items[0]

		tuningJob, err := f.KubeAdminClient().BatchV1().Jobs(naming.ScyllaOperatorNodeTuningNamespace).Get(ctx, naming.PerftuneJobName(scyllaPod.Spec.NodeName), metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		soc, err := f.ScyllaAdminClient().ScyllaV1alpha1().ScyllaOperatorConfigs().Get(ctx, naming.ScyllaOperatorName, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		o.Expect(tuningJob.Spec.Template.Spec.Containers[0].Image).To(o.Equal(soc.Spec.ScyllaUtilsImage))

		framework.By("Changing tuning container image")
		// Rollback the change after test
		defer func(img string) {
			_, err = f.ScyllaAdminClient().ScyllaV1alpha1().ScyllaOperatorConfigs().Patch(
				ctx,
				soc.Name,
				types.JSONPatchType,
				[]byte(fmt.Sprintf(`[{"op":"replace", "path": "/spec/tuningContainerImage", "value": "%s" }]`, img)),
				metav1.PatchOptions{},
			)
			o.Expect(err).NotTo(o.HaveOccurred())
		}(soc.Spec.ScyllaUtilsImage)

		_, err = f.ScyllaAdminClient().ScyllaV1alpha1().ScyllaOperatorConfigs().Patch(
			ctx,
			soc.Name,
			types.JSONPatchType,
			[]byte(fmt.Sprintf(`[{"op":"replace", "path": "/spec/tuningContainerImage", "value": "%s" }]`, strings.TrimPrefix(soc.Spec.ScyllaUtilsImage, "docker.io/"))),
			metav1.PatchOptions{},
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Checking tuning container image")
		waitCtx2, waitCtx2Cancel := utils.ContextForNodeConfigRollout(ctx)
		defer waitCtx2Cancel()
		_, err = utils.WaitForJob(waitCtx2, f.KubeAdminClient().BatchV1(), naming.ScyllaOperatorNodeTuningNamespace, naming.PerftuneJobName(scyllaPod.Spec.NodeName), utils.WaitForStateOptions{TolerateDelete: true}, func(job *batchv1.Job) (bool, error) {
			return job.Spec.Template.Spec.Containers[0].Image == soc.Spec.ScyllaUtilsImage, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())
	})
})
