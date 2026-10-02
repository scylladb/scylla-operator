// Copyright (c) 2026 ScyllaDB.

package scyllacluster

import (
	"context"
	"fmt"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	scyllaclusterverification "github.com/scylladb/scylla-operator/test/e2e/utils/verification/scyllacluster"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = g.Describe("ScyllaCluster", framework.SuiteParallel, framework.SuiteParallelOpenShift, framework.SuiteKindFast, func() {
	var f *framework.Framework

	g.BeforeEach(func(ctx context.Context) {
		f = framework.NewFramework(ctx, "scyllacluster")
	})

	g.It("should restart the ScyllaDB container right away when the ScyllaDB process dies", func() {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		sc := f.GetDefaultScyllaCluster()
		sc.Spec.Datacenter.Racks[0].Members = 1

		framework.By("Creating a ScyllaCluster with 1 member")
		sc, err := f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Create(ctx, sc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaCluster to roll out (RV=%s)", sc.ResourceVersion)
		waitCtx1, waitCtx1Cancel := utils.ContextForRollout(ctx, sc)
		defer waitCtx1Cancel()
		sc, err = controllerhelpers.WaitForScyllaClusterState(waitCtx1, f.ScyllaClient().ScyllaV1().ScyllaClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		scyllaclusterverification.Verify(ctx, f.KubeClient(), f.ScyllaClient(), sc)

		podName := naming.PodNameForScyllaCluster(sc.Spec.Datacenter.Racks[0], sc, 0)

		// The sidecar is the main process of the container and its only child runs ScyllaDB: the entrypoint waiting on
		// supervisord in older images, ScyllaDB itself in images without supervisord. Killing the child is a crash in
		// both cases, which supervisord can't recover from. The kill is deferred so that the exec session finishes
		// before the container goes away.
		framework.By("Killing the process the sidecar runs ScyllaDB in")
		stdout, stderr, err := utils.ExecWithOptions(ctx, f.ClientConfig(), f.KubeClient().CoreV1(), utils.ExecOptions{
			Command: []string{
				"/usr/bin/bash",
				"-euEo",
				"pipefail",
				"-O",
				"inherit_errexit",
				"-c",
				strings.TrimSpace(`
pid="$( pgrep -P 1 )"
[[ "$( wc -w <<< "${pid}" )" -eq 1 ]]
( sleep 1; kill -KILL "${pid}" ) > /dev/null 2>&1 &
`),
			},
			Namespace:     f.Namespace(),
			PodName:       podName,
			ContainerName: naming.ScyllaContainerName,
			CaptureStdout: true,
			CaptureStderr: true,
		})
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("* stdout:\n%q\n* stderr:\n%s", stdout, stderr))

		// The liveness probe takes minutes to restart a container with a dead ScyllaDB, so a restart within this
		// timeout can only come from the container's main process exiting.
		framework.By("Waiting for the ScyllaDB container to be restarted with the exit code of the killed process")
		restartCtx, restartCtxCancel := context.WithTimeoutCause(ctx, time.Minute, fmt.Errorf("ScyllaDB container in pod %q wasn't restarted in time", podName))
		defer restartCtxCancel()
		_, err = controllerhelpers.WaitForPodState(restartCtx, f.KubeClient().CoreV1().Pods(f.Namespace()), podName, controllerhelpers.WaitForStateOptions{}, func(pod *corev1.Pod) (bool, error) {
			cs := controllerhelpers.FindContainerStatus(pod, naming.ScyllaContainerName)
			if cs == nil || cs.RestartCount == 0 {
				return false, nil
			}

			if cs.LastTerminationState.Terminated == nil {
				return false, nil
			}

			exitCode := cs.LastTerminationState.Terminated.ExitCode
			if exitCode != 137 {
				return true, fmt.Errorf("expected ScyllaDB container to exit with 137, got %d", exitCode)
			}

			return true, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaDB container to become ready again")
		readyCtx, readyCtxCancel := context.WithTimeoutCause(ctx, 5*time.Minute, fmt.Errorf("ScyllaDB container in pod %q didn't become ready in time", podName))
		defer readyCtxCancel()
		_, err = controllerhelpers.WaitForPodState(readyCtx, f.KubeClient().CoreV1().Pods(f.Namespace()), podName, controllerhelpers.WaitForStateOptions{}, func(pod *corev1.Pod) (bool, error) {
			cs := controllerhelpers.FindContainerStatus(pod, naming.ScyllaContainerName)
			return cs != nil && cs.Ready, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())
	})
})
