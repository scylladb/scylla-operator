// Copyright (C) 2021 ScyllaDB

package scyllacluster

import (
	"context"
	"errors"
	"fmt"
	"strings"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	scyllaclusterverification "github.com/scylladb/scylla-operator/test/e2e/utils/verification/scyllacluster"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/exec"
)

// scyllaIOPropertiesPath is where the sidecar expects the IO properties file. It's a symlink to the
// cached results on the data volume, dangling until iotune actually produces output.
const scyllaIOPropertiesPath = "/etc/scylla.d/" + naming.ScyllaIOPropertiesName

func IsIOTuneRequested(ctx context.Context, f *framework.Framework, podName string) (bool, error) {
	// IsIOTuneRequested reports whether iotune was requested by the entrypoint for the given Pod.
	// It inspects the arguments the sidecar rendered onto the long-lived /docker-entrypoint.py
	// process (it blocks on supervisord.wait(), so its args are readable for the whole container
	// lifetime by execing into the container). --io-setup=0 and --io-properties-file are passed
	// only when the cached IO properties file already exists, i.e. when iotune was skipped.
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

	// In developer mode iotune is never run, so there's nothing to perform or skip.
	if strings.Contains(entrypointCommand, "--developer-mode=1") {
		return false, nil
	}

	hasIOSetupDisabled := strings.Contains(entrypointCommand, "--io-setup=0")
	hasIOPropertiesFile := strings.Contains(entrypointCommand, "--io-properties-file")
	isIOTuneSkipped := hasIOSetupDisabled && hasIOPropertiesFile

	return !isIOTuneSkipped, nil
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

var _ = g.Describe("ScyllaCluster", framework.SuiteParallel, framework.SuiteParallelOpenShift, func() {
	var f *framework.Framework

	g.BeforeEach(func(ctx context.Context) {
		f = framework.NewFramework(ctx, "scyllacluster")
	})

	g.It("should skip iotune on restart when io properties are cached", func(testCtx context.Context) {
		framework.By("Creating a ScyllaCluster with developer mode disabled")
		sc := f.GetNonDevModeScyllaCluster()
		sc.Spec.Datacenter.Racks[0].Members = 1

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

		framework.By("Verifying that iotune was requested on initial rollout")
		isIOTuneRequested, err := IsIOTuneRequested(testCtx, f, pod.Name)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(isIOTuneRequested).To(o.BeTrue())

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
		isIOTuneRequested, err = IsIOTuneRequested(testCtx, f, pod.Name)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(isIOTuneRequested).To(o.BeFalse())

		framework.By("Verifying that io_properties file was present on restart")
		ioPropertiesFileExists, err = IOPropertiesFileExists(testCtx, f, pod.Name)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(ioPropertiesFileExists).To(o.BeTrue())

	})
})
