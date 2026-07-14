// Copyright (C) 2026 ScyllaDB

package scyllacluster

import (
	"context"
	"fmt"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	scyllav1client "github.com/scylladb/scylla-operator/pkg/client/scylla/clientset/versioned/typed/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	scyllaclusterverification "github.com/scylladb/scylla-operator/test/e2e/utils/verification/scyllacluster"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	appsv1client "k8s.io/client-go/kubernetes/typed/apps/v1"
)

const outageAnnotationKey = "e2e.scylla-operator.scylladb.com/outage-test"

func setOperatorDeploymentReplicasAndWaitForRollout(ctx context.Context, deploymentsClient appsv1client.DeploymentInterface, replicas int32) {
	framework.By("Setting %s deployment replicas to %d", operatorDeploymentName, replicas)
	_, err := deploymentsClient.Patch(
		ctx,
		operatorDeploymentName,
		types.MergePatchType,
		[]byte(fmt.Sprintf(`{"spec":{"replicas":%d}}`, replicas)),
		metav1.PatchOptions{},
	)
	o.Expect(err).NotTo(o.HaveOccurred())

	framework.By("Waiting for %s deployment to roll out", operatorDeploymentName)
	_, err = controllerhelpers.WaitForDeploymentState(ctx, deploymentsClient, operatorDeploymentName, controllerhelpers.WaitForStateOptions{}, controllerhelpers.IsDeploymentRolledOut)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func updateClusterPodAnnotations(ctx context.Context, scyllaClient scyllav1client.ScyllaClusterInterface, sc *scyllav1.ScyllaCluster, key, value string) *scyllav1.ScyllaCluster {
	sc, err := scyllaClient.Get(ctx, sc.Name, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	scCopy := sc.DeepCopy()
	if scCopy.Spec.PodMetadata == nil {
		scCopy.Spec.PodMetadata = &scyllav1.ObjectTemplateMetadata{}
	}
	if scCopy.Spec.PodMetadata.Annotations == nil {
		scCopy.Spec.PodMetadata.Annotations = map[string]string{}
	}
	scCopy.Spec.PodMetadata.Annotations[key] = value

	patch, err := controllerhelpers.GenerateMergePatch(sc, scCopy)
	o.Expect(err).NotTo(o.HaveOccurred())

	sc, err = scyllaClient.Patch(ctx, sc.Name, types.MergePatchType, patch, metav1.PatchOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(sc.Spec.PodMetadata).NotTo(o.BeNil())
	o.Expect(sc.Spec.PodMetadata.Annotations).To(o.HaveKeyWithValue(key, value))

	return sc
}

var _ = g.Describe("ScyllaCluster", framework.SuiteSerial, func() {
	var f *framework.Framework

	g.BeforeEach(func(ctx context.Context) {
		f = framework.NewFramework(ctx, "scyllacluster")
	})

	g.It("should reconcile spec changes after operator outage", func(ctx g.SpecContext) {
		sc := f.GetDefaultScyllaCluster()
		o.Expect(sc.Spec.Datacenter.Racks).To(o.HaveLen(1))
		sc.Spec.Datacenter.Racks[0].Members = 1

		framework.By("Creating a ScyllaCluster")
		var err error
		sc, err = f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Create(ctx, sc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaCluster to roll out (RV=%s)", sc.ResourceVersion)
		initialRolloutCtx, initialRolloutCtxCancel := utils.ContextForRollout(ctx, sc)
		defer initialRolloutCtxCancel()
		sc, err = controllerhelpers.WaitForScyllaClusterState(initialRolloutCtx, f.ScyllaClient().ScyllaV1().ScyllaClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		scyllaclusterverification.Verify(ctx, f.KubeClient(), f.ScyllaClient(), sc)
		scyllaclusterverification.WaitForFullQuorum(ctx, f.KubeClient().CoreV1(), sc)

		framework.By("Saving the original operator replica count")
		operatorDeploy, err := f.KubeAdminClient().AppsV1().Deployments(operatorNamespace).Get(ctx, operatorDeploymentName, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		originalOperatorReplicas := *operatorDeploy.Spec.Replicas
		o.Expect(originalOperatorReplicas).To(o.BeNumerically(">", 0))

		operatorDeployments := f.KubeAdminClient().AppsV1().Deployments(operatorNamespace)

		g.DeferCleanup(func(ctx context.Context) {
			setOperatorDeploymentReplicasAndWaitForRollout(ctx, operatorDeployments, originalOperatorReplicas)
		})

		setOperatorDeploymentReplicasAndWaitForRollout(ctx, operatorDeployments, 0)

		framework.By("Updating ScyllaCluster spec.podMetadata.annotations while operator is down")
		outageAnnotationValue := sc.ResourceVersion
		sc = updateClusterPodAnnotations(ctx, f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()), sc, outageAnnotationKey, outageAnnotationValue)

		framework.By("Restoring the original operator replica count")
		setOperatorDeploymentReplicasAndWaitForRollout(ctx, operatorDeployments, originalOperatorReplicas)

		framework.By("Waiting for the ScyllaCluster to reconcile the spec change")
		reconcileCtx, reconcileCtxCancel := utils.ContextForRollout(ctx, sc)
		defer reconcileCtxCancel()
		sc, err = controllerhelpers.WaitForScyllaClusterState(reconcileCtx, f.ScyllaClient().ScyllaV1().ScyllaClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Verifying the annotation was propagated to ScyllaCluster pods")
		podSelector := labels.Set(naming.ScyllaDBNodePodsSelectorLabelsForScyllaCluster(sc)).AsSelector()
		pods, err := f.KubeClient().CoreV1().Pods(f.Namespace()).List(ctx, metav1.ListOptions{
			LabelSelector: podSelector.String(),
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(pods.Items).NotTo(o.BeEmpty())

		for _, pod := range pods.Items {
			o.Expect(pod.Annotations).To(o.HaveKeyWithValue(outageAnnotationKey, outageAnnotationValue),
				"pod %s/%s should have the annotation set during operator outage", pod.Namespace, pod.Name)
		}
	})
})
