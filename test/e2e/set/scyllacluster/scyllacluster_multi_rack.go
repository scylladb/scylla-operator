// Copyright (c) 2021 ScyllaDB

package scyllacluster

import (
	"context"
	"fmt"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/controller/helpers"
	"github.com/scylladb/scylla-operator/pkg/naming"
	scyllaclusterfixture "github.com/scylladb/scylla-operator/test/e2e/fixture/scyllacluster"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = g.Describe("ScyllaCluster with multiple racks", func() {
	defer g.GinkgoRecover()

	f := framework.NewFramework("scyllacluster")

	g.It("should deploy", func() {
		ctx, cancel := context.WithTimeout(context.Background(), testTimout)
		defer cancel()

		sc := scyllaclusterfixture.MultiRackScyllaCluster.ReadOrFail()

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

		di, err := NewDataInserter(ctx, f.KubeClient().CoreV1(), sc, getMemberCount(sc))
		o.Expect(err).NotTo(o.HaveOccurred())
		defer di.Close()

		err = di.Insert()
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyScyllaCluster(ctx, f.KubeClient(), sc, di)
	})

	g.It("should not deploy a second Rack until first is ready", func() {
		ctx, cancel := context.WithTimeout(context.Background(), testTimout)
		defer cancel()

		sc := scyllaclusterfixture.MultiRackScyllaCluster.ReadOrFail()

		multiRack := sc.Spec.Datacenter.Racks
		sc.Spec.Datacenter.Racks = sc.Spec.Datacenter.Racks[:1]

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

		di, err := NewDataInserter(ctx, f.KubeClient().CoreV1(), sc, getMemberCount(sc))
		o.Expect(err).NotTo(o.HaveOccurred())
		defer di.Close()

		err = di.Insert()
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyScyllaCluster(ctx, f.KubeClient(), sc, di)

		framework.By("Enabling maintenance mode for Rack 0")
		stsName := naming.StatefulSetNameForRack(sc.Spec.Datacenter.Racks[0], sc)
		podName := fmt.Sprintf("%s-0", stsName)
		pod, err := f.KubeClient().CoreV1().Pods(f.Namespace()).Get(ctx, podName, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		svc, err := f.KubeClient().CoreV1().Services(f.Namespace()).Get(ctx, naming.ServiceNameFromPod(pod), metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		_, err = f.KubeClient().CoreV1().Services(svc.Namespace).Patch(
			ctx,
			svc.Name,
			types.StrategicMergePatchType,
			[]byte(fmt.Sprintf(`{"metadata": {"labels":{"%s": ""}}}`, naming.NodeMaintenanceLabel)),
			metav1.PatchOptions{},
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for Rack 0 to become unready")
		waitCtx2, waitCtx2Cancel := context.WithTimeout(ctx, 1*time.Minute)
		defer waitCtx2Cancel()
		_, err = waitForStatefulSetState(waitCtx2, f.KubeClient().AppsV1(), f.Namespace(), stsName, func(sts *appsv1.StatefulSet) (bool, error) {
			return sts.Status.ObservedGeneration >= sts.Generation &&
				sts.Status.CurrentRevision == sts.Status.UpdateRevision &&
				sts.Status.ReadyReplicas != *sts.Spec.Replicas, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Adding Rack 1 to ScyllaCluster")
		modified := sc.DeepCopy()
		modified.Spec.Datacenter.Racks = multiRack

		patchData, err := helpers.GenerateMergePatch(sc, modified)
		o.Expect(err).NotTo(o.HaveOccurred())

		sc, err = f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Patch(ctx, sc.Name, types.MergePatchType, patchData, metav1.PatchOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for ScyllaCluster to patch")
		waitCtx3, waitCtx3Cancel := context.WithTimeout(ctx, 1*time.Minute)
		defer waitCtx3Cancel()
		sc, err = waitForScyllaClusterState(waitCtx3, f.ScyllaClient().ScyllaV1(), sc.Namespace, sc.Name, func(sc *scyllav1.ScyllaCluster) (bool, error) {
			return sc.Status.ObservedGeneration != nil && *sc.Status.ObservedGeneration >= sc.Generation, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Verifying that StatefulSet for Rack 1 has not been created")
		_, err = f.KubeClient().AppsV1().StatefulSets(f.Namespace()).Get(ctx, naming.StatefulSetNameForRack(sc.Spec.Datacenter.Racks[1], sc), metav1.GetOptions{})
		o.Expect(errors.IsNotFound(err)).To(o.BeTrue())

		framework.By("Disabling maintenance mode for Rack 0")
		_, err = f.KubeClient().CoreV1().Services(svc.Namespace).Patch(
			ctx,
			svc.Name,
			types.StrategicMergePatchType,
			[]byte(fmt.Sprintf(`{"metadata": {"labels":{"%s": null}}}`, naming.NodeMaintenanceLabel)),
			metav1.PatchOptions{},
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaCluster to roll out")
		waitCtx4, waitCtx4Cancel := contextForRollout(ctx, sc)
		defer waitCtx4Cancel()
		sc, err = waitForScyllaClusterState(waitCtx4, f.ScyllaClient().ScyllaV1(), sc.Namespace, sc.Name, scyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyScyllaCluster(ctx, f.KubeClient(), sc, di)
	})

	g.It("should allow for decommissioning of one Rack while the other is not ready", func() {
		ctx, cancel := context.WithTimeout(context.Background(), testTimout)
		defer cancel()

		sc := scyllaclusterfixture.MultiRackScyllaCluster.ReadOrFail()

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

		di, err := NewDataInserter(ctx, f.KubeClient().CoreV1(), sc, getMemberCount(sc))
		o.Expect(err).NotTo(o.HaveOccurred())
		defer di.Close()

		err = di.Insert()
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyScyllaCluster(ctx, f.KubeClient(), sc, di)

		framework.By("Enabling maintenance mode for Rack 0")
		stsName := naming.StatefulSetNameForRack(sc.Spec.Datacenter.Racks[0], sc)
		podName := fmt.Sprintf("%s-0", stsName)
		pod, err := f.KubeClient().CoreV1().Pods(f.Namespace()).Get(ctx, podName, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		svc, err := f.KubeClient().CoreV1().Services(f.Namespace()).Get(ctx, naming.ServiceNameFromPod(pod), metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		_, err = f.KubeClient().CoreV1().Services(svc.Namespace).Patch(
			ctx,
			svc.Name,
			types.StrategicMergePatchType,
			[]byte(fmt.Sprintf(`{"metadata": {"labels":{"%s": ""}}}`, naming.NodeMaintenanceLabel)),
			metav1.PatchOptions{},
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for Rack 0 to become unready")
		waitCtx2, waitCtx2Cancel := context.WithTimeout(ctx, 1*time.Minute)
		defer waitCtx2Cancel()
		_, err = waitForStatefulSetState(waitCtx2, f.KubeClient().AppsV1(), f.Namespace(), stsName, func(sts *appsv1.StatefulSet) (bool, error) {
			return sts.Status.ObservedGeneration >= sts.Generation &&
				sts.Status.CurrentRevision == sts.Status.UpdateRevision &&
				sts.Status.ReadyReplicas != *sts.Spec.Replicas, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Decommissioning Rack 1's members")
		modified := sc.DeepCopy()
		modified.Spec.Datacenter.Racks[1].Members = 0

		patchData, err := helpers.GenerateMergePatch(sc, modified)
		o.Expect(err).NotTo(o.HaveOccurred())

		sc, err = f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Patch(ctx, sc.Name, types.MergePatchType, patchData, metav1.PatchOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(sc.Spec.Datacenter.Racks[1].Members).To(o.BeEquivalentTo(0))

		framework.By("Waiting for Rack 1's members to decommission")
		waitCtx3, waitCtx3Cancel := context.WithTimeout(ctx, 1*time.Minute)
		defer waitCtx3Cancel()
		_, err = waitForStatefulSetState(waitCtx3, f.KubeClient().AppsV1(), f.Namespace(), naming.StatefulSetNameForRack(sc.Spec.Datacenter.Racks[1], sc), func(sts *appsv1.StatefulSet) (bool, error) {
			return sts.Status.ObservedGeneration >= sts.Generation &&
				sts.Status.CurrentRevision == sts.Status.UpdateRevision &&
				sts.Status.ReadyReplicas == *sts.Spec.Replicas, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())
	})
})
