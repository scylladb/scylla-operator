// Copyright (C) 2021 ScyllaDB

package nodeconfig

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	configassests "github.com/scylladb/scylla-operator/assets/config"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/internalapi"
	"github.com/scylladb/scylla-operator/pkg/naming"
	scyllafixture "github.com/scylladb/scylla-operator/test/e2e/fixture/scylla"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	scyllaclusterverification "github.com/scylladb/scylla-operator/test/e2e/utils/verification/scyllacluster"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
)

const (
	resourceQuotaName = "blocking"
)

// These tests modify global resource affecting global cluster state.
// They must not be run asynchronously with other tests.
var _ = g.Describe("NodeConfig Optimizations", framework.Serial, func() {
	f := framework.NewFramework("nodeconfig")

	ncTemplate := scyllafixture.NodeConfig.ReadOrFail()
	var matchingNodes []*corev1.Node

	g.JustBeforeEach(func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		g.By("Verifying there is at least one scylla node")
		var err error
		matchingNodes, err = utils.GetMatchingNodesForNodeConfig(ctx, f.KubeAdminClient().CoreV1(), ncTemplate)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(matchingNodes).NotTo(o.HaveLen(0))
		framework.Infof("There are %d scylla nodes", len(matchingNodes))
	})

	g.It("should create tuning resources and tune nodes", func() {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		nc := ncTemplate.DeepCopy()
		rc := framework.NewRestoringCleaner(
			ctx,
			f.AdminClientConfig(),
			f.KubeAdminClient(),
			f.DynamicAdminClient(),
			nodeConfigResourceInfo,
			nc.Namespace,
			nc.Name,
			framework.RestoreStrategyRecreate,
		)
		f.AddCleaners(rc)
		rc.DeleteObject(ctx, true)

		g.By("Creating a NodeConfig")
		nc, err := f.ScyllaAdminClient().ScyllaV1alpha1().NodeConfigs().Create(ctx, nc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the NodeConfig to deploy")
		ctx1, ctx1Cancel := context.WithTimeout(ctx, nodeConfigRolloutTimeout)
		defer ctx1Cancel()
		o.Expect(matchingNodes).NotTo(o.BeEmpty())
		nc, err = controllerhelpers.WaitForNodeConfigState(
			ctx1,
			f.ScyllaAdminClient().ScyllaV1alpha1().NodeConfigs(),
			nc.Name,
			controllerhelpers.WaitForStateOptions{TolerateDelete: false},
			utils.IsNodeConfigRolledOut,
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyNodeConfig(ctx, f.KubeAdminClient(), nc)

		// There should be a tuning job for every scylla node.
		nodeJobList, err := f.KubeAdminClient().BatchV1().Jobs(naming.ScyllaOperatorNodeTuningNamespace).List(ctx, metav1.ListOptions{
			LabelSelector: labels.SelectorFromSet(labels.Set{
				naming.NodeConfigNameLabel:    nc.Name,
				naming.NodeConfigJobTypeLabel: string(naming.NodeConfigJobTypeNodePerftune),
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

	g.It("should set Scylla process nofile rlimit to maximum", func(ctx g.SpecContext) {
		const (
			nrOpenVar   = "fs.nr_open"
			nrOpenLimit = "12345678"
		)

		nc := ncTemplate.DeepCopy()
		nc.Spec.Sysctls = []corev1.Sysctl{
			{
				Name:  nrOpenVar,
				Value: nrOpenLimit,
			},
		}

		ncRC := framework.NewRestoringCleaner(
			ctx,
			f.AdminClientConfig(),
			f.KubeAdminClient(),
			f.DynamicAdminClient(),
			nodeConfigResourceInfo,
			nc.Namespace,
			nc.Name,
			framework.RestoreStrategyRecreate,
		)
		f.AddCleaners(ncRC)
		ncRC.DeleteObject(ctx, true)

		framework.By("Creating a NodeConfig with fs.nr_open sysctl configured")
		nc, err := f.ScyllaAdminClient().ScyllaV1alpha1().NodeConfigs().Create(ctx, nc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for NodeConfig to roll out (RV=%s)", nc.ResourceVersion)
		nodeConfigRolloutCtx, nodeConfigRolloutCtxCancel := context.WithTimeout(ctx, nodeConfigRolloutTimeout)
		defer nodeConfigRolloutCtxCancel()
		nc, err = controllerhelpers.WaitForNodeConfigState(nodeConfigRolloutCtx, f.ScyllaAdminClient().ScyllaV1alpha1().NodeConfigs(), nc.Name, controllerhelpers.WaitForStateOptions{TolerateDelete: false}, utils.IsNodeConfigRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		sc := f.GetDefaultScyllaCluster()

		framework.By("Creating a ScyllaCluster")
		sc, err = f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Create(ctx, sc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaCluster to roll out (RV=%s)", sc.ResourceVersion)
		scyllaClusterRolloutCtx, scyllaClusterRolloutCtxCancel := utils.ContextForRollout(ctx, sc)
		defer scyllaClusterRolloutCtxCancel()
		sc, err = controllerhelpers.WaitForScyllaClusterState(scyllaClusterRolloutCtx, f.ScyllaClient().ScyllaV1().ScyllaClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		scyllaclusterverification.Verify(ctx, f.KubeClient(), f.ScyllaClient(), sc)

		podName := fmt.Sprintf("%s-%d", naming.StatefulSetNameForRackForScyllaCluster(sc.Spec.Datacenter.Racks[0], sc), 0)
		for _, rlimit := range []string{"SOFT", "HARD"} {
			framework.By("Validating %s file limit of Scylla process", rlimit)

			ec := &corev1.EphemeralContainer{
				TargetContainerName: naming.ScyllaContainerName,
				EphemeralContainerCommon: corev1.EphemeralContainerCommon{
					Name:            fmt.Sprintf("e2e-prlimits-%s", strings.ToLower(rlimit)),
					Image:           configassests.Project.OperatorTests.NodeSetupImage,
					ImagePullPolicy: corev1.PullIfNotPresent,
					Command:         []string{"bash"},
					Args: []string{
						"-euEo",
						"pipefail",
						"-O",
						"inherit_errexit",
						"-c",
						fmt.Sprintf(`prlimit --pid=$(pidof scylla) --nofile --noheadings --output=%s`, rlimit),
					},
				},
			}

			pod, ecLogs, err := utils.RunEphemeralContainerAndCollectLogs(ctx, f.KubeAdminClient().CoreV1().Pods(sc.Namespace), podName, ec)
			o.Expect(err).NotTo(o.HaveOccurred())
			ephemeralContainerState := controllerhelpers.FindContainerStatus(pod, ec.Name)
			o.Expect(ephemeralContainerState).NotTo(o.BeNil())
			o.Expect(ephemeralContainerState.State.Terminated).NotTo(o.BeNil())
			o.Expect(ephemeralContainerState.State.Terminated.ExitCode).To(o.BeEquivalentTo(0))
			o.Expect(strings.TrimSpace(string(ecLogs))).To(o.Equal(nrOpenLimit))
		}
	})

	g.It("should correctly project state for each scylla pod", func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

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

		rq := &corev1.ResourceQuota{
			ObjectMeta: metav1.ObjectMeta{
				Name:      resourceQuotaName,
				Namespace: naming.ScyllaOperatorNodeTuningNamespace,
			},
			Spec: corev1.ResourceQuotaSpec{
				Hard: corev1.ResourceList{
					corev1.ResourcePods: resource.MustParse("0"),
				},
			},
		}
		rqRC := framework.NewRestoringCleaner(
			ctx,
			f.AdminClientConfig(),
			f.KubeAdminClient(),
			f.DynamicAdminClient(),
			resourceQuotaResourceInfo,
			rq.Namespace,
			rq.Name,
			framework.RestoreStrategyUpdate,
		)
		f.AddCleaners(rqRC)
		rqRC.DeleteObject(ctx, true)

		rq, err = f.KubeAdminClient().CoreV1().ResourceQuotas(naming.ScyllaOperatorNodeTuningNamespace).Create(
			ctx,
			rq,
			metav1.CreateOptions{},
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		nc := ncTemplate.DeepCopy()

		ncRC := framework.NewRestoringCleaner(
			ctx,
			f.AdminClientConfig(),
			f.KubeAdminClient(),
			f.DynamicAdminClient(),
			nodeConfigResourceInfo,
			nc.Namespace,
			nc.Name,
			framework.RestoreStrategyRecreate,
		)
		f.AddCleaners(ncRC)
		ncRC.DeleteObject(ctx, true)

		g.By("Creating a NodeConfig")
		nc, err = f.ScyllaAdminClient().ScyllaV1alpha1().NodeConfigs().Create(ctx, nc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		sc := f.GetDefaultScyllaCluster()
		o.Expect(sc.Spec.Datacenter.Racks).To(o.HaveLen(1))
		sc.Spec.Datacenter.Racks[0].AgentResources = corev1.ResourceRequirements{
			Requests: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceCPU:    resource.MustParse("50m"),
				corev1.ResourceMemory: resource.MustParse("50Mi"),
			},
			Limits: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceCPU:    resource.MustParse("50m"),
				corev1.ResourceMemory: resource.MustParse("50Mi"),
			},
		}
		sc.Spec.Datacenter.Racks[0].Resources = corev1.ResourceRequirements{
			Requests: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceCPU:    resource.MustParse("1"),
				corev1.ResourceMemory: resource.MustParse("1Gi"),
			},
			Limits: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceCPU:    resource.MustParse("1"),
				corev1.ResourceMemory: resource.MustParse("1Gi"),
			},
		}

		framework.By("Creating a ScyllaCluster")
		sc, err = f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Create(ctx, sc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for a ConfigMap to indicate blocking NodeConfig")
		ctx1, ctx1Cancel := context.WithTimeout(ctx, 30*time.Second)
		defer ctx1Cancel()
		podName := fmt.Sprintf("%s-%d", naming.StatefulSetNameForRackForScyllaCluster(sc.Spec.Datacenter.Racks[0], sc), 0)
		pod, err := controllerhelpers.WaitForPodState(ctx1, f.KubeClient().CoreV1().Pods(sc.Namespace), podName, controllerhelpers.WaitForStateOptions{}, func(p *corev1.Pod) (bool, error) {
			return true, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(pod.Status.QOSClass).To(o.Equal(corev1.PodQOSGuaranteed))

		cmName := naming.GetTuningConfigMapNameForPod(pod)
		ctx2, ctx2Cancel := context.WithTimeout(ctx, 30*time.Second)
		defer ctx2Cancel()
		src := &internalapi.SidecarRuntimeConfig{}
		cm, err := controllerhelpers.WaitForConfigMapState(ctx2, f.KubeClient().CoreV1().ConfigMaps(sc.Namespace), cmName, controllerhelpers.WaitForStateOptions{}, func(cm *corev1.ConfigMap) (bool, error) {
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

		framework.By("Verifying the containers are blocked and not ready")
		podSelector := labels.Set(naming.ScyllaDBNodesPodsLabelsForScyllaCluster(sc)).AsSelector()
		scPods, err := f.KubeClient().CoreV1().Pods(sc.Namespace).List(ctx, metav1.ListOptions{
			LabelSelector: podSelector.String(),
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(scPods.Items).NotTo(o.BeEmpty())

		for _, pod := range scPods.Items {
			crm := utils.GetContainerReadinessMap(&pod)
			o.Expect(crm).To(
				o.And(
					o.HaveKeyWithValue(naming.ScyllaContainerName, false),
					o.HaveKeyWithValue(naming.ScyllaDBIgnitionContainerName, false),
					o.HaveKeyWithValue(naming.ScyllaManagerAgentContainerName, false),
				),
				fmt.Sprintf("container(s) in Pod %q don't match the expected state", pod.Name),
			)
		}

		framework.By("Unblocking tuning")
		intermittentArtifactsDir := ""
		if len(f.GetDefaultArtifactsDir()) != 0 {
			intermittentArtifactsDir = filepath.Join(f.GetDefaultArtifactsDir(), "intermittent")
		}
		rqRC.Collect(ctx, intermittentArtifactsDir, f.Namespace())
		rqRC.DeleteObject(ctx, false)

		pod, err = f.KubeClient().CoreV1().Pods(f.Namespace()).Get(
			ctx,
			utils.GetNodeName(sc, 0),
			metav1.GetOptions{},
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		scyllaContainerID, err := controllerhelpers.GetScyllaContainerID(pod)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Bumping tuning DaemonSet sync and waiting for it to become healthy")

		dsList, err := f.KubeAdminClient().AppsV1().DaemonSets(naming.ScyllaOperatorNodeTuningNamespace).List(ctx, metav1.ListOptions{
			LabelSelector: labels.Set{
				"app.kubernetes.io/name": naming.NodeConfigAppName,
			}.AsSelector().String(),
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(dsList.Items).To(o.HaveLen(1), "there should be exactly 1 matching NodeConfig in this test")
		ds := &dsList.Items[0]

		// At this point the DaemonSet controller in kube-controller-manager is notably rate-limited
		// because of resource quota failures. We have to trigger an event to reset its queue rate limiter,
		// or there will be a large delay.
		ds, err = f.KubeAdminClient().AppsV1().DaemonSets(naming.ScyllaOperatorNodeTuningNamespace).Patch(
			ctx,
			ds.Name,
			types.JSONPatchType,
			[]byte(fmt.Sprintf(`[{"op": "add", "path": "/metadata/annotations/e2e-requeue", "value": %q}]`, time.Now())),
			metav1.PatchOptions{},
		)

		ctxTuningDS, ctxTuningDSCancel := utils.ContextForRollout(ctx, sc)
		defer ctxTuningDSCancel()
		ds, err = controllerhelpers.WaitForDaemonSetState(ctxTuningDS, f.KubeAdminClient().AppsV1().DaemonSets(ds.Namespace), ds.Name, controllerhelpers.WaitForStateOptions{}, controllerhelpers.IsDaemonSetRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the NodeConfig to deploy")
		ctx3, ctx3Cancel := context.WithTimeout(ctx, nodeConfigRolloutTimeout)
		defer ctx3Cancel()
		nc, err = controllerhelpers.WaitForNodeConfigState(
			ctx3,
			f.ScyllaAdminClient().ScyllaV1alpha1().NodeConfigs(),
			nc.Name,
			controllerhelpers.WaitForStateOptions{TolerateDelete: false},
			utils.IsNodeConfigRolledOut,
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyNodeConfig(ctx, f.KubeAdminClient(), nc)

		isNodeTunedForScyllaContainer := controllerhelpers.IsNodeTunedForContainer(nc, pod.Spec.NodeName, scyllaContainerID)
		o.Expect(isNodeTunedForScyllaContainer).To(o.BeTrue())

		framework.By("Waiting for the ScyllaCluster to roll out (RV=%s)", sc.ResourceVersion)
		ctx4, ctx4Cancel := utils.ContextForRollout(ctx, sc)
		defer ctx4Cancel()
		sc, err = controllerhelpers.WaitForScyllaClusterState(ctx4, f.ScyllaClient().ScyllaV1().ScyllaClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		scyllaclusterverification.VerifyWithOptions(
			ctx,
			f.KubeClient(),
			f.ScyllaClient(),
			sc,
			scyllaclusterverification.VerifyOptions{
				VerifyStatefulSetOptions: scyllaclusterverification.VerifyStatefulSetOptions{
					PodRestartCountAssertion: func(a o.Assertion, containerName, podName string) {
						// We expect restart(s) from the startup probe, usually 1.
						a.To(o.BeNumerically("<=", 2), fmt.Sprintf("container %q in pod %q should not be restarted by the startup probe more than twice", containerName, podName))
					},
				},
			},
		)

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

	g.It("should configure kernel parameters (sysctls)", func(ctx g.SpecContext) {
		const (
			fsAioMaxNRVar          = "fs.aio-max-nr"
			fsAioMaxNrInitialValue = "2097152"
			fsAioMaxNrTargetValue  = "30000000"
		)

		nc := ncTemplate.DeepCopy()
		nc.Spec.Sysctls = []corev1.Sysctl{
			{
				Name:  fsAioMaxNRVar,
				Value: fsAioMaxNrTargetValue,
			},
		}

		// Delete any existing NodeConfigs to ensure the test doesn't conflict with other jobs trying to configure sysctls.
		existingNCs, err := f.ScyllaAdminClient().ScyllaV1alpha1().NodeConfigs().List(ctx, metav1.ListOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		ncsToClean := existingNCs.Items
		// Ensure we also clean up the NodeConfig we're about to create.
		ncsToClean = append(ncsToClean, *nc)
		for _, ncToClean := range ncsToClean {
			rc := framework.NewRestoringCleaner(
				ctx,
				f.AdminClientConfig(),
				f.KubeAdminClient(),
				f.DynamicAdminClient(),
				nodeConfigResourceInfo,
				ncToClean.Namespace,
				ncToClean.Name,
				framework.RestoreStrategyRecreate,
			)
			f.AddCleaners(rc)
			rc.DeleteObject(ctx, true)
		}

		// Select one of the matching nodes for testing.
		o.Expect(matchingNodes).NotTo(o.BeEmpty())
		nodeUnderTest := matchingNodes[0]
		framework.Infof("Node %s will be used for testing.", nodeUnderTest.GetName())

		framework.By("Creating a client Pod")
		clientPod := newClientPod(nc, nodeUnderTest)
		clientPod, err = f.KubeClient().CoreV1().Pods(f.Namespace()).Create(ctx, clientPod, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		clientPodCreationCtx, clientPodCreationCtxCancel := utils.ContextForPodStartup(ctx)
		defer clientPodCreationCtxCancel()
		clientPod, err = controllerhelpers.WaitForPodState(clientPodCreationCtx, f.KubeClient().CoreV1().Pods(clientPod.Namespace), clientPod.Name, controllerhelpers.WaitForStateOptions{}, utils.PodIsRunning)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Writing an initial value for fs.aio-max-nr sysctl")
		stdout, stderr, err := utils.ExecuteInPod(ctx, f.ClientConfig(), f.KubeClient().CoreV1(), clientPod, "", "sysctl", "-w", fmt.Sprintf("%s=%s", fsAioMaxNRVar, fsAioMaxNrInitialValue))
		o.Expect(err).NotTo(o.HaveOccurred(), stdout, stderr)

		framework.By("Verifying fs.aio-max-nr sysctl is set to the initial value")
		stdout, stderr, err = utils.ExecuteInPod(ctx, f.ClientConfig(), f.KubeClient().CoreV1(), clientPod, "", "sysctl", "-n", fsAioMaxNRVar)
		o.Expect(err).NotTo(o.HaveOccurred(), stdout, stderr)
		o.Expect(strings.TrimSpace(stdout)).To(o.Equal(fsAioMaxNrInitialValue))

		framework.By("Creating a NodeConfig with fs.aio-max-nr sysctl configured")
		nc, err = f.ScyllaAdminClient().ScyllaV1alpha1().NodeConfigs().Create(ctx, nc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for NodeConfig to roll out (RV=%s)", nc.ResourceVersion)
		nodeConfigRolloutCtx, nodeConfigRolloutCtxCancel := context.WithTimeout(ctx, nodeConfigRolloutTimeout)
		defer nodeConfigRolloutCtxCancel()
		nc, err = controllerhelpers.WaitForNodeConfigState(nodeConfigRolloutCtx, f.ScyllaAdminClient().ScyllaV1alpha1().NodeConfigs(), nc.Name, controllerhelpers.WaitForStateOptions{TolerateDelete: false}, utils.IsNodeConfigRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyNodeConfig(ctx, f.KubeAdminClient(), nc)

		framework.By("Verifying fs.aio-max-nr sysctl is set to the target value")
		stdout, stderr, err = utils.ExecuteInPod(ctx, f.ClientConfig(), f.KubeClient().CoreV1(), clientPod, "", "sysctl", "-n", fsAioMaxNRVar)
		o.Expect(err).NotTo(o.HaveOccurred(), stdout, stderr)
		o.Expect(strings.TrimSpace(stdout)).To(o.Equal(fsAioMaxNrTargetValue))
	})
})
