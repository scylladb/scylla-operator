// Copyright (C) 2021 ScyllaDB

package scyllacluster

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	v1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = g.Describe("ScyllaCluster graceful termination", func() {
	f := framework.NewFramework("scyllacluster")

	g.It("should work while waiting for ignition", func() {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		sc := f.GetDefaultScyllaCluster()
		o.Expect(sc.Spec.Datacenter.Racks).To(o.HaveLen(1))
		sc.Spec.Datacenter.Racks[0].Members = 1
		if sc.Spec.ExposeOptions == nil {
			sc.Spec.ExposeOptions = &v1.ExposeOptions{}
		}
		if sc.Spec.ExposeOptions.NodeService == nil {
			sc.Spec.ExposeOptions.NodeService = &v1.NodeServiceTemplate{}
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
		waitCtx1, waitCtx1Cancel := utils.ContextForPodStartup(ctx)
		defer waitCtx1Cancel()
		pod, err := controllerhelpers.WaitForPodState(waitCtx1, f.KubeClient().CoreV1().Pods(f.Namespace()), podName, controllerhelpers.WaitForStateOptions{}, utils.PodIsRunning)
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
				GracePeriodSeconds: pointer.Ptr[int64](7 * 24 * 60 * 60),
			},
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaDB Pod to be deleted")
		deletionCtx, deletionCtxCancel := context.WithTimeoutCause(
			ctx,
			1*time.Minute,
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
