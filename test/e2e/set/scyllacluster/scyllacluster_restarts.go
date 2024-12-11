// Copyright (C) 2024 ScyllaDB

package scyllacluster

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	scyllaclusterverification "github.com/scylladb/scylla-operator/test/e2e/utils/verification/scyllacluster"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// terminationTimeout is the amount of time that the pod needs to terminate gracefully.
	// In addition to process termination, this needs to account for kubelet sending signals, state propagation
	// and generally being busy in the CI.
	terminationTimeout = 5 * time.Minute
	// A high enough grace period so it can never terminate gracefully in an e2e run.
	gracePeriod = terminationTimeout + (7 * 24 * time.Hour)
)

var _ = g.Describe("ScyllaCluster graceful termination", func() {
	f := framework.NewFramework("scyllacluster")

	// This test verifies correct signal handling in the bash wait routine
	// which oscillates between a sleep (external program) and a file check.
	// The "problematic" external program (sleep) runs a majority of the time, but not all.
	// To be reasonably sure, we should run this multiple times in a single test run
	// and avoid an accidental merge that could bork the test / CI.
	g.It("should work while waiting for ignition", g.MustPassRepeatedly(3), func() {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		sc := f.GetDefaultScyllaCluster()
		o.Expect(sc.Spec.Datacenter.Racks).To(o.HaveLen(1))
		sc.Spec.Datacenter.Racks[0].Members = 1
		if sc.Spec.ExposeOptions == nil {
			sc.Spec.ExposeOptions = &scyllav1.ExposeOptions{}
		}
		if sc.Spec.ExposeOptions.NodeService == nil {
			sc.Spec.ExposeOptions.NodeService = &scyllav1.NodeServiceTemplate{}
		}
		if sc.Spec.ExposeOptions.NodeService.Annotations == nil {
			sc.Spec.ExposeOptions.NodeService.Annotations = map[string]string{}
		}
		sc.Spec.ExposeOptions.NodeService.Annotations[naming.ForceIgnitionValueAnnotation] = "false"

		framework.By("Creating a ScyllaCluster with blocked ignition")
		sc, err := f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Create(ctx, sc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaCluster Pod to be running")
		podName := naming.PodNameForScyllaCluster(sc.Spec.Datacenter.Racks[0], sc, 0)
		waitRunningCtx, waitRunningCtxCancel := context.WithTimeoutCause(ctx, 10*time.Minute, fmt.Errorf("timed out waiting for ScyllaDB Pod to be running"))
		defer waitRunningCtxCancel()
		pod, err := controllerhelpers.WaitForPodState(waitRunningCtx, f.KubeClient().CoreV1().Pods(f.Namespace()), podName, controllerhelpers.WaitForStateOptions{}, utils.PodIsRunning)
		o.Expect(err).NotTo(o.HaveOccurred())

		execCtx, execCtxCancel := context.WithTimeoutCause(ctx, 2*time.Minute, errors.New("ignition probe didn't return expected status in time"))
		defer execCtxCancel()
		framework.By("Executing into %q container and verifying it's not ignited using the probe", naming.ScyllaDBIgnitionContainerName)
		stdout, stderr, err := utils.ExecWithOptions(execCtx, f.ClientConfig(), f.KubeClient().CoreV1(), utils.ExecOptions{
			Command: []string{
				"/usr/bin/timeout",
				"1m",
				"/usr/bin/bash",
				"-euxEo",
				"pipefail",
				"-O",
				"inherit_errexit",
				"-c",
				strings.TrimSpace(`
while [[ "$( curl -s -o /dev/null -w '%{http_code}' -G http://localhost:42081/readyz )" != "503" ]]; do
  sleep 1 &
  wait;
done
				`),
			},
			Namespace:     pod.Namespace,
			PodName:       pod.Name,
			ContainerName: naming.ScyllaDBIgnitionContainerName,
			CaptureStdout: true,
			CaptureStderr: true,
		})
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("* stdout:\n%q\n* stderr:\n%s\n* Cause: %s", stdout, stderr, context.Cause(execCtx)))

		sleepTime := 30 * time.Second
		framework.By("Sleeping for %v to give kubelet time to update the probes", sleepTime)
		// The sleep also waits for nodeconfigpod controller to unblock tuning, in case the override would be broken.
		time.Sleep(sleepTime)

		framework.By("Validating container readiness")
		pod, err = f.KubeClient().CoreV1().Pods(f.Namespace()).Get(ctx, podName, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		crm := utils.GetContainerReadinessMap(pod)
		o.Expect(crm).To(
			o.And(
				o.HaveKeyWithValue(naming.ScyllaContainerName, false),
				o.HaveKeyWithValue(naming.ScyllaDBIgnitionContainerName, false),
				o.HaveKeyWithValue(naming.ScyllaManagerAgentContainerName, false),
			),
			fmt.Sprintf("container(s) in Pod %q don't match the expected state", pod.Name),
		)

		framework.By("Deleting the ScyllaDB Pod")
		err = f.KubeClient().CoreV1().Pods(f.Namespace()).Delete(
			ctx,
			pod.Name,
			metav1.DeleteOptions{
				Preconditions: &metav1.Preconditions{
					UID: pointer.Ptr(pod.UID),
				},
				GracePeriodSeconds: pointer.Ptr(int64(gracePeriod.Seconds())),
			},
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaDB Pod to be deleted")
		deletionCtx, deletionCtxCancel := context.WithTimeoutCause(
			ctx,
			terminationTimeout,
			fmt.Errorf("pod %q has not finished termination in time", naming.ObjRef(pod)),
		)
		defer deletionCtxCancel()
		err = framework.WaitForObjectDeletion(
			deletionCtx,
			f.DynamicClient(),
			corev1.SchemeGroupVersion.WithResource("pods"),
			pod.Namespace,
			pod.Name,
			pointer.Ptr(pod.UID),
		)
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.It("should work when a cluster is fully rolled out", func() {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		sc := f.GetDefaultScyllaCluster()
		o.Expect(sc.Spec.Datacenter.Racks).To(o.HaveLen(1))
		sc.Spec.Datacenter.Racks[0].Members = 1

		framework.By("Creating a ScyllaCluster")
		sc, err := f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Create(ctx, sc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaCluster to roll out (RV=%s)", sc.ResourceVersion)
		waitCtx1, waitCtx1Cancel := utils.ContextForRollout(ctx, sc)
		defer waitCtx1Cancel()
		sc, err = controllerhelpers.WaitForScyllaClusterState(waitCtx1, f.ScyllaClient().ScyllaV1().ScyllaClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		scyllaclusterverification.Verify(ctx, f.KubeClient(), f.ScyllaClient(), sc)

		framework.By("Fetching the Pod and validating container readiness")
		podName := naming.PodNameForScyllaCluster(sc.Spec.Datacenter.Racks[0], sc, 0)
		pod, err := f.KubeClient().CoreV1().Pods(f.Namespace()).Get(ctx, podName, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		crm := utils.GetContainerReadinessMap(pod)
		o.Expect(crm).To(
			o.And(
				o.HaveKeyWithValue(naming.ScyllaContainerName, true),
				o.HaveKeyWithValue(naming.ScyllaDBIgnitionContainerName, true),
				o.HaveKeyWithValue(naming.ScyllaManagerAgentContainerName, true),
			),
			fmt.Sprintf("container(s) in Pod %q don't match the expected state", pod.Name),
		)

		framework.By("Deleting the ScyllaDB Pod")
		err = f.KubeClient().CoreV1().Pods(f.Namespace()).Delete(
			ctx,
			pod.Name,
			metav1.DeleteOptions{
				Preconditions: &metav1.Preconditions{
					UID: pointer.Ptr(pod.UID),
				},
				GracePeriodSeconds: pointer.Ptr(int64(gracePeriod.Seconds())),
			},
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaDB Pod to be deleted")
		deletionCtx, deletionCtxCancel := context.WithTimeoutCause(
			ctx,
			terminationTimeout,
			fmt.Errorf("pod %q has not finished termination in time", naming.ObjRef(pod)),
		)
		defer deletionCtxCancel()
		err = framework.WaitForObjectDeletion(
			deletionCtx,
			f.DynamicClient(),
			corev1.SchemeGroupVersion.WithResource("pods"),
			pod.Namespace,
			pod.Name,
			pointer.Ptr(pod.UID),
		)
		o.Expect(err).NotTo(o.HaveOccurred())
	})
})
