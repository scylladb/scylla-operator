// Copyright (C) 2021 ScyllaDB

package scyllacluster

import (
	"context"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	"github.com/scylladb/scylla-operator/pkg/naming"
	scyllaclusterfixture "github.com/scylladb/scylla-operator/test/e2e/fixture/scyllacluster"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = g.Describe("ScyllaCluster auto replace", func() {
	defer g.GinkgoRecover()

	f := framework.NewFramework("scyllacluster")

	g.It("should replace a node", func() {
		ctx, cancel := context.WithTimeout(context.Background(), testTimout)
		defer cancel()

		sc := scyllaclusterfixture.BasicScyllaCluster.ReadOrFail()
		sc.Spec.Datacenter.Racks[0].Members = 2

		framework.By("Creating a ScyllaCluster")
		err := framework.SetupScyllaClusterSA(ctx, f.KubeClient().CoreV1(), f.KubeClient().RbacV1(), f.Namespace(), sc.Name)
		o.Expect(err).NotTo(o.HaveOccurred())

		sc, err = f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Create(ctx, sc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaCluster to deploy")
		waitCtx1, waitCtx1Cancel := contextForRollout(ctx, sc)
		defer waitCtx1Cancel()
		sc, err = waitForScyllaClusterState(waitCtx1, f.ScyllaClient().ScyllaV1(), sc.Namespace, sc.Name, scyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyScyllaCluster(ctx, f.KubeClient(), sc)

		framework.By("Removing node #0's pvc")
		pvc, err := f.KubeClient().CoreV1().PersistentVolumeClaims(f.Namespace()).Get(
			ctx,
			naming.PVCNameForPod(getNodeName(sc, 0)),
			metav1.GetOptions{},
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		var gracePeriodSeconds int64 = 0
		var propagationPolicy = metav1.DeletePropagationForeground
		err = f.KubeClient().CoreV1().PersistentVolumeClaims(f.Namespace()).Delete(
			ctx,
			naming.PVCNameForPod(getNodeName(sc, 0)),
			metav1.DeleteOptions{
				GracePeriodSeconds: &gracePeriodSeconds,
				PropagationPolicy:  &propagationPolicy,
				Preconditions: &metav1.Preconditions{
					UID: &pvc.UID,
				},
			},
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Removing node #0's pod")
		pod, err := f.KubeClient().CoreV1().Pods(f.Namespace()).Get(
			ctx,
			getNodeName(sc, 0),
			metav1.GetOptions{},
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		err = f.KubeClient().CoreV1().Pods(f.Namespace()).Delete(
			ctx,
			getNodeName(sc, 0),
			metav1.DeleteOptions{
				GracePeriodSeconds: &gracePeriodSeconds,
				PropagationPolicy:  &propagationPolicy,
				Preconditions: &metav1.Preconditions{
					UID: &pod.UID,
				},
			},
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Awaiting node #0's pvc deletion")
		waitCtx2, waitCtx2Cancel := context.WithCancel(context.Background())
		defer waitCtx2Cancel()
		err = framework.WaitForObjectDeletion(waitCtx2, f.DynamicAdminClient(), corev1.SchemeGroupVersion.WithResource("persistentvolumeclaims"), f.Namespace(), pvc.Name, pvc.UID)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Awaiting node #0's pod deletion")
		waitCtx3, waitCtx3Cancel := context.WithCancel(context.Background())
		defer waitCtx3Cancel()
		err = framework.WaitForObjectDeletion(waitCtx3, f.DynamicAdminClient(), corev1.SchemeGroupVersion.WithResource("pods"), f.Namespace(), pod.Name, pod.UID)
		o.Expect(err).NotTo(o.HaveOccurred())

		// Give the controller some time to observe that the pod is down.
		time.Sleep(10 * time.Second)

		framework.By("Waiting for the ScyllaCluster to re-deploy")
		waitCtx4, waitCtx4Cancel := contextForRollout(ctx, sc)
		defer waitCtx4Cancel()
		sc, err = waitForScyllaClusterState(waitCtx4, f.ScyllaClient().ScyllaV1(), sc.Namespace, sc.Name, scyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyScyllaCluster(ctx, f.KubeClient(), sc)
	})
})
