// Copyright (C) 2021 ScyllaDB

package scyllacluster

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	"github.com/scylladb/scylla-operator/test/e2e/utils/verification"
	scyllaclusterverification "github.com/scylladb/scylla-operator/test/e2e/utils/verification/scyllacluster"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/exec"
)

// scyllaIOPropertiesPath is where the sidecar expects the IO properties file. It's a symlink to the
// cached results on the data volume, dangling until iotune actually produces output.
const scyllaIOPropertiesPath = "/etc/scylla.d/" + naming.ScyllaIOPropertiesName

func addQuantity(lhs resource.Quantity, rhs resource.Quantity) *resource.Quantity {
	res := lhs.DeepCopy()
	res.Add(rhs)

	// Pre-cache the string so DeepEqual works.
	_ = res.String()

	return &res
}

func IsIOTunePerformed(ctx context.Context, f *framework.Framework, podName string, sc *scyllav1.ScyllaCluster) (bool, error) {
	// The sidecar starts /docker-entrypoint.py as a long-lived child process (it blocks on
	// supervisord.wait()), so its arguments are readable for the whole container lifetime by
	// execing into the container. --io-setup=0 and --io-properties-file are only passed to the
	// entrypoint when the cached IO properties file already exists, i.e. when iotune was skipped.
	stdout, stderr, err := utils.ExecWithOptions(ctx, f.ClientConfig(), f.KubeClient().CoreV1(), utils.ExecOptions{
		Command:       []string{"/usr/bin/pgrep", "-af", "docker-entrypoint.py"},
		Namespace:     f.Namespace(),
		PodName:       podName,
		ContainerName: naming.ScyllaContainerName,
		CaptureStdout: true,
		CaptureStderr: true,
	})
	if err != nil {
		return false, fmt.Errorf("can't read entrypoint process args from pod %q: %w (stderr: %q)", podName, err, stderr)
	}

	entrypointCommand := stdout
	if !strings.Contains(entrypointCommand, "docker-entrypoint.py") {
		return false, fmt.Errorf("can't find docker-entrypoint.py process in pod %q", podName)
	}

	hasIOSetupDisabled := strings.Contains(entrypointCommand, "--io-setup=0")
	hasIOPropertiesFile := strings.Contains(entrypointCommand, "--io-properties-file")
	isIOTuneSkipped := hasIOSetupDisabled && hasIOPropertiesFile

	return !sc.Spec.DeveloperMode && !isIOTuneSkipped, nil
}

// IOPropertiesFileExists reports whether the resolved io_properties file exists in the ScyllaDB
// container. The path is a symlink that's always present, but its target only exists once iotune has
// actually produced output, so `test -f` (which follows the symlink) confirms iotune was performed
// rather than merely requested.
func IOPropertiesFileExists(ctx context.Context, f *framework.Framework, podName string) (bool, error) {
	_, stderr, err := utils.ExecWithOptions(ctx, f.ClientConfig(), f.KubeClient().CoreV1(), utils.ExecOptions{
		Command:       []string{"test", "-f", scyllaIOPropertiesPath},
		Namespace:     f.Namespace(),
		PodName:       podName,
		ContainerName: naming.ScyllaContainerName,
		CaptureStdout: true,
		CaptureStderr: true,
	})
	if err != nil {
		var codeExitErr exec.CodeExitError
		if errors.As(err, &codeExitErr) && codeExitErr.Code == 1 {
			// `test -f` exits with 1 when the file doesn't exist (or the symlink is dangling).
			return false, nil
		}
		return false, fmt.Errorf("can't check for io_properties file in pod %q: %w (stderr: %q)", podName, err, stderr)
	}

	return true, nil
}

var _ = g.Describe("ScyllaCluster", framework.SuiteParallel, framework.SuiteParallelOpenShift, framework.SuiteKindFast, func() {
	var f *framework.Framework

	g.BeforeEach(func(ctx context.Context) {
		f = framework.NewFramework(ctx, "scyllacluster")
	})

	g.It("should rolling restart cluster when forceRedeploymentReason is changed", func() {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		framework.By("Creating a ScyllaCluster")
		sc := f.GetDefaultScyllaCluster()
		sc.Spec.Datacenter.Racks[0].Members = 1

		sc, err := f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Create(ctx, sc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaCluster to roll out (RV=%s)", sc.ResourceVersion)
		waitCtx1, waitCtx1Cancel := utils.ContextForRollout(ctx, sc)
		defer waitCtx1Cancel()
		sc, err = controllerhelpers.WaitForScyllaClusterState(waitCtx1, f.ScyllaClient().ScyllaV1().ScyllaClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		scyllaclusterverification.Verify(ctx, f.KubeClient(), f.ScyllaClient(), sc)
		scyllaclusterverification.WaitForFullQuorum(ctx, f.KubeClient().CoreV1(), sc)

		pod, err := f.KubeClient().CoreV1().Pods(f.Namespace()).Get(
			ctx,
			utils.GetNodeName(sc, 0),
			metav1.GetOptions{},
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		initialPodUID := pod.UID
		framework.Infof("Initial pod %q UID is %q", pod.Name, initialPodUID)

		sc, err = f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Patch(
			ctx,
			sc.Name,
			types.JSONPatchType,
			[]byte(`[{"op": "replace", "path": "/spec/forceRedeploymentReason", "value": "foo"}]`),
			metav1.PatchOptions{},
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaCluster to redeploy")
		waitCtx2, waitCtx2Cancel := utils.ContextForRollout(ctx, sc)
		defer waitCtx2Cancel()
		sc, err = controllerhelpers.WaitForScyllaClusterState(waitCtx2, f.ScyllaClient().ScyllaV1().ScyllaClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		scyllaclusterverification.Verify(ctx, f.KubeClient(), f.ScyllaClient(), sc)
		scyllaclusterverification.WaitForFullQuorum(ctx, f.KubeClient().CoreV1(), sc)

		pod, err = f.KubeClient().CoreV1().Pods(f.Namespace()).Get(
			ctx,
			utils.GetNodeName(sc, 0),
			metav1.GetOptions{},
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		o.Expect(pod.UID).NotTo(o.Equal(initialPodUID))
	})

	g.It("should reconcile resource changes", func() {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		framework.By("Creating a ScyllaCluster")
		sc := f.GetDefaultScyllaCluster()
		o.Expect(sc.Spec.Datacenter.Racks).To(o.HaveLen(1))
		o.Expect(sc.Spec.Datacenter.Racks[0].Members).To(o.BeEquivalentTo(1))

		sc, err := f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Create(ctx, sc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaCluster to roll out (RV=%s)", sc.ResourceVersion)
		waitCtx1, waitCtx1Cancel := utils.ContextForRollout(ctx, sc)
		defer waitCtx1Cancel()
		sc, err = controllerhelpers.WaitForScyllaClusterState(waitCtx1, f.ScyllaClient().ScyllaV1().ScyllaClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		scyllaclusterverification.Verify(ctx, f.KubeClient(), f.ScyllaClient(), sc)
		scyllaclusterverification.WaitForFullQuorum(ctx, f.KubeClient().CoreV1(), sc)

		hosts, hostIDs, err := utils.GetBroadcastRPCAddressesAndUUIDs(ctx, f.KubeClient().CoreV1(), sc)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(hosts).To(o.HaveLen(1))
		di := verification.InsertAndVerifyCQLData(ctx, hosts)
		defer di.Close()

		framework.By("Changing pod resources")
		oldResources := *sc.Spec.Datacenter.Racks[0].Resources.DeepCopy()
		newResources := *oldResources.DeepCopy()
		o.Expect(oldResources.Requests).To(o.HaveKey(corev1.ResourceCPU))
		o.Expect(oldResources.Requests).To(o.HaveKey(corev1.ResourceMemory))
		o.Expect(oldResources.Limits).To(o.HaveKey(corev1.ResourceCPU))
		o.Expect(oldResources.Limits).To(o.HaveKey(corev1.ResourceMemory))

		newResources.Requests[corev1.ResourceCPU] = *addQuantity(newResources.Requests[corev1.ResourceCPU], resource.MustParse("1m"))
		newResources.Requests[corev1.ResourceMemory] = *addQuantity(newResources.Requests[corev1.ResourceMemory], resource.MustParse("1Mi"))
		o.Expect(newResources.Requests).NotTo(o.BeEquivalentTo(oldResources.Requests))

		newResources.Limits[corev1.ResourceCPU] = *addQuantity(newResources.Limits[corev1.ResourceCPU], resource.MustParse("1m"))
		newResources.Limits[corev1.ResourceMemory] = *addQuantity(newResources.Limits[corev1.ResourceMemory], resource.MustParse("1Mi"))
		o.Expect(newResources.Limits).NotTo(o.BeEquivalentTo(oldResources.Limits))

		o.Expect(newResources).NotTo(o.BeEquivalentTo(oldResources))

		newResourcesJSON, err := json.Marshal(newResources)
		o.Expect(err).NotTo(o.HaveOccurred())

		sc, err = f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Patch(
			ctx,
			sc.Name,
			types.JSONPatchType,
			[]byte(fmt.Sprintf(
				`[{"op": "replace", "path": "/spec/datacenter/racks/0/resources", "value": %s}]`,
				newResourcesJSON,
			)),
			metav1.PatchOptions{},
		)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(sc.Spec.Datacenter.Racks[0].Resources).To(o.BeEquivalentTo(newResources))

		framework.By("Waiting for the ScyllaCluster to redeploy")
		waitCtx2, waitCtx2Cancel := utils.ContextForRollout(ctx, sc)
		defer waitCtx2Cancel()
		sc, err = controllerhelpers.WaitForScyllaClusterState(waitCtx2, f.ScyllaClient().ScyllaV1().ScyllaClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		scyllaclusterverification.Verify(ctx, f.KubeClient(), f.ScyllaClient(), sc)
		scyllaclusterverification.WaitForFullQuorum(ctx, f.KubeClient().CoreV1(), sc)

		oldHosts := hosts
		oldHostIDs := hostIDs
		hosts, hostIDs, err = utils.GetBroadcastRPCAddressesAndUUIDs(ctx, f.KubeClient().CoreV1(), sc)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(hosts).To(o.HaveLen(len(oldHosts)))
		o.Expect(hostIDs).To(o.ConsistOf(oldHostIDs))

		// Reset hosts as the client won't be able to discover a single node after rollout.
		err = di.SetClientEndpoints(hosts)
		o.Expect(err).NotTo(o.HaveOccurred())
		verification.VerifyCQLData(ctx, di)

		framework.By("Scaling the ScyllaCluster up to create a new replica")
		oldMembers := sc.Spec.Datacenter.Racks[0].Members
		newMebmers := oldMembers + 1
		sc, err = f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Patch(
			ctx,
			sc.Name,
			types.JSONPatchType,
			[]byte(fmt.Sprintf(
				`[{"op": "replace", "path": "/spec/datacenter/racks/0/members", "value": %d}]`,
				newMebmers,
			)),
			metav1.PatchOptions{},
		)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(sc.Spec.Datacenter.Racks[0].Members).To(o.Equal(newMebmers))

		framework.By("Waiting for the ScyllaCluster to redeploy")
		waitCtx4, waitCtx4Cancel := utils.ContextForRollout(ctx, sc)
		defer waitCtx4Cancel()
		sc, err = controllerhelpers.WaitForScyllaClusterState(waitCtx4, f.ScyllaClient().ScyllaV1().ScyllaClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		scyllaclusterverification.Verify(ctx, f.KubeClient(), f.ScyllaClient(), sc)
		scyllaclusterverification.WaitForFullQuorum(ctx, f.KubeClient().CoreV1(), sc)

		oldHostIDs = hostIDs
		_, hostIDs, err = utils.GetBroadcastRPCAddressesAndUUIDs(ctx, f.KubeClient().CoreV1(), sc)

		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(oldHostIDs).To(o.HaveLen(int(oldMembers)))
		o.Expect(hostIDs).To(o.HaveLen(int(newMebmers)))
		o.Expect(hostIDs).To(o.ContainElements(oldHostIDs))
		verification.VerifyCQLData(ctx, di)
	})
})

var _ = g.Describe("ScyllaCluster", framework.SuiteParallel, framework.SuiteParallelOpenShift, func() {
	var f *framework.Framework

	g.BeforeEach(func(ctx context.Context) {
		f = framework.NewFramework(ctx, "scyllacluster")
	})

	g.It("should skip iotune on restart when io properties are cached", func(testCtx context.Context) {
		framework.By("Creating a ScyllaCluster with developer mode disabled")
		sc := f.GetNonDevModeScyllaCluster()
		sc.Spec.Datacenter.Racks[0].Members = 1
		var isIOTunePerformed bool

		sc, err := f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Create(testCtx, sc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaCluster to roll out (RV=%s)", sc.ResourceVersion)
		initialRolloutCtx, initialRolloutCtxCancel := utils.ContextForRollout(testCtx, sc)
		defer initialRolloutCtxCancel()
		sc, err = controllerhelpers.WaitForScyllaClusterState(initialRolloutCtx, f.ScyllaClient().ScyllaV1().ScyllaClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		scyllaclusterverification.Verify(testCtx, f.KubeClient(), f.ScyllaClient(), sc)

		pod, err := f.KubeClient().CoreV1().Pods(f.Namespace()).Get(
			testCtx,
			utils.GetNodeName(sc, 0),
			metav1.GetOptions{},
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Verifying that iotune was performed on initial rollout")
		isIOTunePerformed, err = IsIOTunePerformed(testCtx, f, pod.Name, sc)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(isIOTunePerformed).To(o.BeTrue())

		framework.By("Verifying that iotune produced the io_properties file")
		ioPropertiesFileExists, err := IOPropertiesFileExists(testCtx, f, pod.Name)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(ioPropertiesFileExists).To(o.BeTrue())

		initialPodUID := pod.UID
		framework.Infof("Initial pod %q UID is %q", pod.Name, initialPodUID)

		framework.By("Restarting the ScyllaDB Pod")
		err = f.KubeClient().CoreV1().Pods(f.Namespace()).Delete(testCtx, pod.Name, metav1.DeleteOptions{
			Preconditions: &metav1.Preconditions{
				UID: &initialPodUID,
			},
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the old ScyllaDB Pod to be deleted")
		deletionCtx, deletionCtxCancel := context.WithTimeoutCause(
			testCtx,
			utils.ScyllaDBTerminationTimeout,
			fmt.Errorf("pod %q has not finished termination in time", naming.ObjRef(pod)),
		)
		defer deletionCtxCancel()
		err = framework.WaitForObjectDeletion(
			deletionCtx,
			f.DynamicClient(),
			corev1.SchemeGroupVersion.WithResource("pods"),
			pod.Namespace,
			pod.Name,
			pointer.Ptr(initialPodUID),
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaCluster to roll out (RV=%s)", sc.ResourceVersion)
		restartRolloutCtx, restartRolloutCtxCancel := utils.ContextForRollout(testCtx, sc)
		defer restartRolloutCtxCancel()
		sc, err = controllerhelpers.WaitForScyllaClusterState(restartRolloutCtx, f.ScyllaClient().ScyllaV1().ScyllaClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Verifying that iotune was skipped on restart")
		isIOTunePerformed, err = IsIOTunePerformed(testCtx, f, pod.Name, sc)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(isIOTunePerformed).To(o.BeFalse())
	})
})
