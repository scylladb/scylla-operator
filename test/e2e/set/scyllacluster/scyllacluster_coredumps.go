// Copyright (C) 2025 ScyllaDB

package scyllacluster

import (
	"context"
	"fmt"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	configassests "github.com/scylladb/scylla-operator/assets/config"
	coredumpsfixture "github.com/scylladb/scylla-operator/examples/gke/coredumps"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	scyllaclusterverification "github.com/scylladb/scylla-operator/test/e2e/utils/verification/scyllacluster"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	daemonSetRolloutTimeout   = 5 * time.Minute
	sysctlPropagationTimeout  = 3 * time.Minute
	coredumpVerifyPodTimeout  = 3 * time.Minute
	coredumpCollectionTimeout = 2 * time.Minute
)

var _ = g.Describe("ScyllaCluster coredumps", framework.Serial, framework.NotSupportedOnKind, func() {
	var f *framework.Framework

	g.BeforeEach(func(ctx context.Context) {
		f = framework.NewFramework(ctx, "scyllacluster")
	})

	g.It("should persist a coredump when ScyllaDB crashes with SIGABRT", func(ctx g.SpecContext) {
		cm := coredumpsfixture.CoredumpConfConfigMap.ReadOrFail()
		ds := coredumpsfixture.SetupSystemdCoredumpDaemonSet.ReadOrFail()
		coredumpSetupNamespace := cm.GetNamespace()

		framework.By("Creating the coredump configuration ConfigMap")
		cm, err := f.KubeAdminClient().CoreV1().ConfigMaps(coredumpSetupNamespace).Create(ctx, cm, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.DeferCleanup(func(ctx context.Context) {
			framework.By("Deleting the coredump configuration ConfigMap")
			err := f.KubeAdminClient().CoreV1().ConfigMaps(coredumpSetupNamespace).Delete(ctx, cm.Name, metav1.DeleteOptions{
				PropagationPolicy: pointer.Ptr(metav1.DeletePropagationForeground),
			})
			o.Expect(err).NotTo(o.HaveOccurred())
		})

		framework.By("Creating the coredump setup DaemonSet")
		ds, err = f.KubeAdminClient().AppsV1().DaemonSets(coredumpSetupNamespace).Create(ctx, ds, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.DeferCleanup(func(ctx context.Context) {
			framework.By("Deleting the coredump setup DaemonSet")
			err := f.KubeAdminClient().AppsV1().DaemonSets(coredumpSetupNamespace).Delete(ctx, ds.Name, metav1.DeleteOptions{
				PropagationPolicy: pointer.Ptr(metav1.DeletePropagationForeground),
			})
			o.Expect(err).NotTo(o.HaveOccurred())

			framework.By("Waiting for the coredump setup DaemonSet to be removed")
			err = framework.WaitForObjectDeletion(
				ctx,
				f.DynamicAdminClient(),
				appsv1.SchemeGroupVersion.WithResource("daemonsets"),
				ds.Namespace,
				ds.Name,
				nil,
			)
			o.Expect(err).NotTo(o.HaveOccurred())
		})

		framework.By("Waiting for the coredump setup DaemonSet to roll out")
		dsRolloutCtx, dsRolloutCtxCancel := context.WithTimeoutCause(ctx, daemonSetRolloutTimeout, fmt.Errorf("timed out waiting for DaemonSet %q to roll out", naming.ObjRef(ds)))
		defer dsRolloutCtxCancel()
		ds, err = controllerhelpers.WaitForDaemonSetState(dsRolloutCtx, f.KubeAdminClient().AppsV1().DaemonSets(coredumpSetupNamespace), ds.Name, controllerhelpers.WaitForStateOptions{}, controllerhelpers.IsDaemonSetRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Creating a privileged host Pod to verify node-level side effects")
		hostPod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "coredump-verify-",
			},
			Spec: corev1.PodSpec{
				HostPID: true,
				Containers: []corev1.Container{
					{
						Name:  "host",
						Image: configassests.Project.OperatorTests.NodeSetupImage,
						Command: []string{
							"/bin/sh",
							"-c",
							"sleep 3600",
						},
						SecurityContext: &corev1.SecurityContext{
							Privileged: pointer.Ptr(true),
						},
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      "host",
								MountPath: "/host",
							},
						},
					},
				},
				NodeSelector: map[string]string{
					"scylla.scylladb.com/node-type": "scylla",
				},
				Tolerations: []corev1.Toleration{
					{
						Key:      "scylla-operator.scylladb.com/dedicated",
						Operator: corev1.TolerationOpEqual,
						Value:    "scyllaclusters",
						Effect:   corev1.TaintEffectNoSchedule,
					},
				},
				Volumes: []corev1.Volume{
					{
						Name: "host",
						VolumeSource: corev1.VolumeSource{
							HostPath: &corev1.HostPathVolumeSource{
								Path: "/",
							},
						},
					},
				},
				TerminationGracePeriodSeconds: pointer.Ptr(int64(1)),
				RestartPolicy:                 corev1.RestartPolicyNever,
			},
		}
		hostPod, err = f.KubeClient().CoreV1().Pods(f.Namespace()).Create(ctx, hostPod, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the host Pod to be running")
		hostPodCtx, hostPodCtxCancel := context.WithTimeoutCause(ctx, coredumpVerifyPodTimeout, fmt.Errorf("timed out waiting for host Pod %q to be running", naming.ObjRef(hostPod)))
		defer hostPodCtxCancel()
		hostPod, err = controllerhelpers.WaitForPodState(hostPodCtx, f.KubeClient().CoreV1().Pods(f.Namespace()), hostPod.Name, controllerhelpers.WaitForStateOptions{}, utils.PodIsRunning)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for kernel.core_pattern to be set to systemd-coredump on node %q", hostPod.Spec.NodeName)
		o.Eventually(func(eg o.Gomega) {
			stdout, stderr, err := utils.ExecWithOptions(ctx, f.ClientConfig(), f.KubeClient().CoreV1(), utils.ExecOptions{
				Command: []string{
					"nsenter",
					"--mount=/host/proc/1/ns/mnt",
					"--uts=/host/proc/1/ns/uts",
					"--",
					"sysctl", "-n", "kernel.core_pattern",
				},
				Namespace:     hostPod.Namespace,
				PodName:       hostPod.Name,
				ContainerName: "host",
				CaptureStdout: true,
				CaptureStderr: true,
			})
			eg.Expect(err).NotTo(o.HaveOccurred(), stdout, stderr)
			eg.Expect(strings.TrimSpace(stdout)).To(o.ContainSubstring("systemd-coredump"))
		}).WithPolling(5 * time.Second).WithTimeout(sysctlPropagationTimeout).Should(o.Succeed())

		sc := f.GetDefaultScyllaCluster()
		o.Expect(sc.Spec.Datacenter.Racks).To(o.HaveLen(1))
		sc.Spec.Datacenter.Racks[0].Members = 1

		framework.By("Creating a ScyllaCluster")
		sc, err = f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Create(ctx, sc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaCluster to roll out (RV=%s)", sc.ResourceVersion)
		waitCtx, waitCtxCancel := utils.ContextForRollout(ctx, sc)
		defer waitCtxCancel()
		sc, err = controllerhelpers.WaitForScyllaClusterState(waitCtx, f.ScyllaClient().ScyllaV1().ScyllaClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		scyllaclusterverification.Verify(ctx, f.KubeClient(), f.ScyllaClient(), sc)

		podName := utils.GetNodeName(sc, 0)

		framework.By("Sending SIGABRT to the scylla process in Pod %q", podName)
		_, _, err = utils.ExecWithOptions(ctx, f.ClientConfig(), f.KubeClient().CoreV1(), utils.ExecOptions{
			Command:       []string{"sh", "-c", "kill -ABRT $(pgrep -x scylla)"},
			Namespace:     f.Namespace(),
			PodName:       podName,
			ContainerName: naming.ScyllaContainerName,
			CaptureStdout: true,
			CaptureStderr: true,
		})
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to send SIGABRT to scylla process in Pod %q", podName)

		framework.By("Waiting for the ScyllaCluster to roll out again after the crash")
		recoverCtx, recoverCtxCancel := utils.ContextForRollout(ctx, sc)
		defer recoverCtxCancel()
		sc, err = controllerhelpers.WaitForScyllaClusterState(recoverCtx, f.ScyllaClient().ScyllaV1().ScyllaClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Verifying coredump was captured using coredumpctl on the host")
		o.Eventually(func(eg o.Gomega) {
			stdout, stderr, err := utils.ExecWithOptions(ctx, f.ClientConfig(), f.KubeClient().CoreV1(), utils.ExecOptions{
				Command: []string{
					"nsenter",
					"--mount=/host/proc/1/ns/mnt",
					"--uts=/host/proc/1/ns/uts",
					"--",
					"coredumpctl", "list", "--no-pager",
				},
				Namespace:     hostPod.Namespace,
				PodName:       hostPod.Name,
				ContainerName: "host",
				CaptureStdout: true,
				CaptureStderr: true,
			})
			eg.Expect(err).NotTo(o.HaveOccurred(), stderr)
			eg.Expect(stdout + stderr).To(o.ContainSubstring("scylla"))
		}).WithPolling(5 * time.Second).WithTimeout(coredumpCollectionTimeout).Should(o.Succeed())
	})
})
