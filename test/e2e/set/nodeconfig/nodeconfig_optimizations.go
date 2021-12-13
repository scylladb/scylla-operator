// Copyright (C) 2021 ScyllaDB

package nodeconfig

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/internalapi"
	"github.com/scylladb/scylla-operator/pkg/naming"
	scyllafixture "github.com/scylladb/scylla-operator/test/e2e/fixture/scylla"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/tools"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

const (
	resourceQuotaName = "blocking"
)

// These tests modify global resource affecting global cluster state.
// They must not be run asynchronously with other tests.
var _ = g.Describe("NodeConfig Optimizations", framework.Serial, func() {
	defer g.GinkgoRecover()

	f := framework.NewFramework("nodeconfig")

	ncTemplate := scyllafixture.NodeConfig.ReadOrFail()
	var matchingNodes []*corev1.Node

	preconditionSuccessful := false
	g.JustBeforeEach(func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Make sure the NodeConfig is not present.
		framework.By("Making sure NodeConfig %q, doesn't exist", naming.ObjRef(ncTemplate))
		_, err := f.ScyllaAdminClient().ScyllaV1alpha1().NodeConfigs().Get(ctx, ncTemplate.Name, metav1.GetOptions{})
		if err == nil {
			framework.Failf("NodeConfig %q can't be present before running this test", naming.ObjRef(ncTemplate))
		} else if !apierrors.IsNotFound(err) {
			framework.Failf("Can't get NodeConfig %q: %v", naming.ObjRef(ncTemplate), err)
		}

		preconditionSuccessful = true

		g.By("Verifying there is at least one scylla node")
		matchingNodes, err = utils.GetMatchingNodesForNodeConfig(ctx, f.KubeAdminClient().CoreV1(), ncTemplate)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(matchingNodes).NotTo(o.HaveLen(0))
		framework.Infof("There are %d scylla nodes", len(matchingNodes))
	})

	g.JustAfterEach(func() {
		if !preconditionSuccessful {
			return
		}

		framework.By("Deleting NodeConfig %q, if it exists", naming.ObjRef(ncTemplate))
		err := f.ScyllaAdminClient().ScyllaV1alpha1().NodeConfigs().Delete(context.Background(), ncTemplate.Name, metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			framework.Failf("Can't delete NodeConfig %q: %v", naming.ObjRef(ncTemplate), err)
		}
		if !apierrors.IsNotFound(err) {
			err = framework.WaitForObjectDeletion(context.Background(), f.DynamicAdminClient(), scyllav1alpha1.GroupVersion.WithResource("nodeconfigs"), ncTemplate.Namespace, ncTemplate.Name, nil)
			o.Expect(err).NotTo(o.HaveOccurred())
		}

		framework.By("Deleting ResourceQuota %q, if it exists", naming.ObjRef(ncTemplate))
		err = f.KubeAdminClient().CoreV1().ResourceQuotas(naming.ScyllaOperatorNodeTuningNamespace).Delete(context.Background(), resourceQuotaName, metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			framework.Failf("Can't delete ResourceQuota %q: %v", naming.ManualRef(naming.ScyllaOperatorNodeTuningNamespace, resourceQuotaName), err)
		}
		if !apierrors.IsNotFound(err) {
			err = framework.WaitForObjectDeletion(context.Background(), f.DynamicAdminClient(), corev1.SchemeGroupVersion.WithResource("resourcequota"), naming.ScyllaOperatorNodeTuningNamespace, resourceQuotaName, nil)
			o.Expect(err).NotTo(o.HaveOccurred())
		}
	})

	g.It("should apply user provided kernel parameters on tuned nodes", func() {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		const (
			// This has to be synced with Scylla image used in ScyllaOperatorConfig.
			// This parameter wasn't changed for over 2 years, so hopefully it's not going to require frequent syncing.
			parameterFromScyllaImageKey   = "vm.swappiness"
			parameterFromScyllaImageValue = 1

			customParameterCollidingWithScyllaImageKey   = "fs.inotify.max_user_instances"
			customParameterCollidingWithScyllaImageValue = 600

			scalableParameterKey   = "fs.aio-max-nr"
			scalableParameterValue = 5578536
		)

		nc := ncTemplate.DeepCopy()
		nc.Spec.KernelParameters.NodeMultitenancy = 2
		nc.Spec.KernelParameters.CustomKeyValues = []string{
			fmt.Sprintf("%s=%d", customParameterCollidingWithScyllaImageKey, customParameterCollidingWithScyllaImageValue),
			fmt.Sprintf("%s=%d", scalableParameterKey, scalableParameterValue),
		}
		nc.Spec.KernelParameters.TenantScalableKeys = []string{
			scalableParameterKey,
		}

		g.By("Creating a NodeConfig")
		nc, err := f.ScyllaAdminClient().ScyllaV1alpha1().NodeConfigs().Create(ctx, nc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the NodeConfig to deploy")
		waitCtx1, waitCtx1Cancel := context.WithTimeout(ctx, nodeConfigRolloutTimeout)
		defer waitCtx1Cancel()
		nc, err = utils.WaitForNodeConfigState(
			waitCtx1,
			f.ScyllaAdminClient().ScyllaV1alpha1().NodeConfigs(),
			nc.Name,
			utils.WaitForStateOptions{TolerateDelete: false},
			utils.IsNodeConfigRolledOut,
			utils.IsNodeConfigDoneWithNodeTuningFunc(matchingNodes),
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyNodeConfig(ctx, f.KubeAdminClient(), nc)

		// There should be a tuning job for every scylla node.
		nodeJobList, err := f.KubeAdminClient().BatchV1().Jobs(naming.ScyllaOperatorNodeTuningNamespace).List(ctx, metav1.ListOptions{
			LabelSelector: labels.SelectorFromSet(labels.Set{
				naming.NodeConfigNameLabel:    nc.Name,
				naming.NodeConfigJobTypeLabel: string(naming.NodeConfigJobTypeNode),
			}).String(),
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(nodeJobList.Items).To(o.HaveLen(len(matchingNodes)))

		framework.By("Creating a ScyllaCluster")
		sc := scyllafixture.BasicScyllaCluster.ReadOrFail().DeepCopy()
		sc.Spec.Datacenter.Racks[0].Members = 1

		sc, err = f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Create(ctx, sc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaCluster to deploy")
		waitCtx2, waitCtx2Cancel := utils.ContextForRollout(ctx, sc)
		defer waitCtx2Cancel()
		sc, err = utils.WaitForScyllaClusterState(waitCtx2, f.ScyllaClient().ScyllaV1(), sc.Namespace, sc.Name, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Checking the sysctl value")
		expectedSysctls := map[string]int{
			parameterFromScyllaImageKey:                parameterFromScyllaImageValue,
			customParameterCollidingWithScyllaImageKey: customParameterCollidingWithScyllaImageValue,
			scalableParameterKey:                       scalableParameterValue * int(nc.Spec.KernelParameters.NodeMultitenancy),
		}
		podName := fmt.Sprintf("%s-0", naming.StatefulSetNameForRack(sc.Spec.Datacenter.Racks[0], sc))
		for key, value := range expectedSysctls {
			stdout, stderr, err := tools.PodExec(
				f.KubeClient().CoreV1().RESTClient(),
				f.ClientConfig(),
				f.Namespace(),
				podName,
				"scylla",
				[]string{"/usr/sbin/sysctl", "--values", key},
				nil,
			)
			o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("sysctl failed for %q key: %q", key, stderr))
			o.Expect(stderr).To(o.BeEmpty())
			o.Expect(stdout).To(o.Equal(fmt.Sprintf("%d\n", value)))
		}
	})

	g.It("should create tuning resources and tune nodes", func() {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		nc := ncTemplate.DeepCopy()

		g.By("Creating a NodeConfig")
		nc, err := f.ScyllaAdminClient().ScyllaV1alpha1().NodeConfigs().Create(ctx, nc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the NodeConfig to deploy")
		ctx1, ctx1Cancel := context.WithTimeout(ctx, nodeConfigRolloutTimeout)
		defer ctx1Cancel()
		nc, err = utils.WaitForNodeConfigState(
			ctx1,
			f.ScyllaAdminClient().ScyllaV1alpha1().NodeConfigs(),
			nc.Name,
			utils.WaitForStateOptions{TolerateDelete: false},
			utils.IsNodeConfigRolledOut,
			utils.IsNodeConfigDoneWithNodeTuningFunc(matchingNodes),
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyNodeConfig(ctx, f.KubeAdminClient(), nc)

		// There should be a tuning job for every scylla node.
		nodeJobList, err := f.KubeAdminClient().BatchV1().Jobs(naming.ScyllaOperatorNodeTuningNamespace).List(ctx, metav1.ListOptions{
			LabelSelector: labels.SelectorFromSet(labels.Set{
				naming.NodeConfigNameLabel:    nc.Name,
				naming.NodeConfigJobTypeLabel: string(naming.NodeConfigJobTypeNode),
			}).String(),
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(nodeJobList.Items).To(o.HaveLen(len(matchingNodes)))

		var jobNodeNames []string
		for _, j := range nodeJobList.Items {
			o.Expect(j.Annotations).NotTo(o.BeNil())
			nodeName, found := j.Annotations[naming.NodeConfigJobForNodeKey]
			o.Expect(found).To(o.BeTrue())

			o.Expect(nodeName).NotTo(o.BeEmpty())
			jobNodeNames = append(jobNodeNames, nodeName)
		}
		sort.Strings(jobNodeNames)

		var matchingNodeNames []string
		for _, node := range matchingNodes {
			matchingNodeNames = append(matchingNodeNames, node.Name)
		}
		sort.Strings(matchingNodeNames)

		o.Expect(jobNodeNames).To(o.BeEquivalentTo(matchingNodeNames))
	})

	g.It("should correctly project state for each scylla pod", func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		nc := ncTemplate.DeepCopy()

		g.By("Blocking node tuning")
		// We have to make sure the namespace exists.
		_, err := f.KubeAdminClient().CoreV1().Namespaces().Create(
			ctx,
			&corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: naming.ScyllaOperatorNodeTuningNamespace,
				},
			},
			metav1.CreateOptions{},
		)
		if err != nil && !apierrors.IsAlreadyExists(err) {
			o.Expect(err).NotTo(o.HaveOccurred())
		}

		rq, err := f.KubeAdminClient().CoreV1().ResourceQuotas(naming.ScyllaOperatorNodeTuningNamespace).Create(
			ctx,
			&corev1.ResourceQuota{
				ObjectMeta: metav1.ObjectMeta{
					Name: resourceQuotaName,
				},
				Spec: corev1.ResourceQuotaSpec{
					Hard: corev1.ResourceList{
						corev1.ResourcePods: resource.MustParse("0"),
					},
				},
			},
			metav1.CreateOptions{},
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Creating a NodeConfig")
		nc, err = f.ScyllaAdminClient().ScyllaV1alpha1().NodeConfigs().Create(ctx, nc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		sc := scyllafixture.BasicScyllaCluster.ReadOrFail()

		framework.By("Creating a ScyllaCluster")
		sc, err = f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Create(ctx, sc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for a ConfigMap to indicate blocking NodeConfig")
		ctx1, ctx1Cancel := context.WithTimeout(ctx, apiCallTimeout)
		defer ctx1Cancel()
		podName := fmt.Sprintf("%s-%d", naming.StatefulSetNameForRack(sc.Spec.Datacenter.Racks[0], sc), 0)
		pod, err := utils.WaitForPodState(ctx1, f.KubeClient().CoreV1(), sc.Namespace, podName, func(p *corev1.Pod) (bool, error) {
			return true, nil
		}, utils.WaitForStateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		cmName := naming.GetTuningConfigMapNameForPod(pod)
		ctx2, ctx2Cancel := context.WithTimeout(ctx, 30*time.Second)
		defer ctx2Cancel()
		src := &internalapi.SidecarRuntimeConfig{}
		cm, err := utils.WaitForConfigMapState(ctx2, f.KubeClient().CoreV1().ConfigMaps(sc.Namespace), cmName, utils.WaitForStateOptions{}, func(cm *corev1.ConfigMap) (bool, error) {
			if cm.Data == nil {
				return false, nil
			}

			srcData, found := cm.Data[naming.ScyllaRuntimeConfigKey]
			if !found {
				return false, nil
			}

			err = json.Unmarshal([]byte(srcData), src)
			if err != nil {
				return false, err
			}

			return len(src.BlockingNodeConfigs) > 0, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(src.ContainerID).To(o.BeEmpty())
		o.Expect(src.MatchingNodeConfigs).NotTo(o.BeEmpty())

		waitTime := utils.RolloutTimeoutForScyllaCluster(sc)
		framework.By("Sleeping for %v", waitTime)
		time.Sleep(waitTime)

		framework.By("Verifying ScyllaCluster is still not rolled out")
		sc, err = f.ScyllaClient().ScyllaV1().ScyllaClusters(sc.Namespace).Get(ctx, sc.Name, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(utils.IsScyllaClusterRolledOut(sc)).To(o.BeFalse())

		framework.By("Unblocking tuning")
		err = f.KubeAdminClient().CoreV1().ResourceQuotas(rq.Namespace).Delete(
			ctx,
			rq.Name,
			metav1.DeleteOptions{},
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the NodeConfig to deploy")
		ctx3, ctx3Cancel := context.WithTimeout(ctx, nodeConfigRolloutTimeout)
		defer ctx3Cancel()
		nc, err = utils.WaitForNodeConfigState(
			ctx3,
			f.ScyllaAdminClient().ScyllaV1alpha1().NodeConfigs(),
			nc.Name,
			utils.WaitForStateOptions{TolerateDelete: false},
			utils.IsNodeConfigRolledOut,
			utils.IsNodeConfigDoneWithNodeTuningFunc(matchingNodes),
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyNodeConfig(ctx, f.KubeAdminClient(), nc)

		framework.By("Waiting for the ScyllaCluster to deploy")
		ctx4, ctx4Cancel := utils.ContextForRollout(ctx, sc)
		defer ctx4Cancel()
		sc, err = utils.WaitForScyllaClusterState(ctx4, f.ScyllaClient().ScyllaV1(), sc.Namespace, sc.Name, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Verifying ConfigMap content")
		ctx5, ctx5Cancel := context.WithTimeout(ctx, apiCallTimeout)
		defer ctx5Cancel()
		cm, err = f.KubeClient().CoreV1().ConfigMaps(sc.Namespace).Get(ctx5, cmName, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		o.Expect(cm.Data).NotTo(o.BeNil())

		o.Expect(cm.Data).To(o.HaveKey(naming.ScyllaRuntimeConfigKey))
		srcData := cm.Data[naming.ScyllaRuntimeConfigKey]

		err = json.Unmarshal([]byte(srcData), src)
		o.Expect(err).NotTo(o.HaveOccurred())

		o.Expect(src.ContainerID).NotTo(o.BeEmpty())
		o.Expect(src.MatchingNodeConfigs).NotTo(o.BeEmpty())
		o.Expect(src.BlockingNodeConfigs).To(o.BeEmpty())
	})
})
