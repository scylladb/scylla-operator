package nodeconfig

import (
	"context"
	"fmt"

	o "github.com/onsi/gomega"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
)

func verifyDaemonSet(ds *appsv1.DaemonSet) {
	o.Expect(ds.DeletionTimestamp).To(o.BeNil())
	o.Expect(ds.Status.ObservedGeneration).To(o.Equal(ds.Generation))
	o.Expect(ds.Status.UpdatedNumberScheduled).To(o.Equal(ds.Status.DesiredNumberScheduled))
	o.Expect(ds.Status.NumberAvailable).To(o.Equal(ds.Status.DesiredNumberScheduled))
}

func verifyNodeConfig(ctx context.Context, kubeClient kubernetes.Interface, nc *scyllav1alpha1.NodeConfig) {
	framework.By("Verifying NodeConfig %q", naming.ObjRef(nc))

	o.Expect(nc.Status.ObservedGeneration).NotTo(o.BeNil())

	daemonSets, err := utils.GetDaemonSetsForNodeConfig(ctx, kubeClient.AppsV1(), nc)
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(daemonSets).To(o.HaveLen(1))

	ds := daemonSets[0]
	verifyDaemonSet(ds)

	for i := range nc.Status.Conditions {
		c := &nc.Status.Conditions[i]
		o.Expect(c.LastTransitionTime).NotTo(o.BeNil())
		o.Expect(c.LastTransitionTime.Time.Before(nc.CreationTimestamp.Time)).NotTo(o.BeTrue())

		// To be able to compare the statuses we need to remove the random timestamp.
		c.LastTransitionTime = metav1.Time{}
	}

	o.Expect(nc.Status.Conditions).To(o.ConsistOf(func() []interface{} {
		var expectedConditions []interface{}

		dsPods, err := kubeClient.CoreV1().Pods(ds.Namespace).List(ctx, metav1.ListOptions{LabelSelector: labels.SelectorFromSet(ds.Spec.Selector.MatchLabels).String()})
		o.Expect(err).NotTo(o.HaveOccurred())

		dsNodeNames := make([]string, 0, len(dsPods.Items))
		for _, dsPod := range dsPods.Items {
			dsNodeNames = append(dsNodeNames, dsPod.Spec.NodeName)
		}

		type condValue struct {
			condType string
			status   corev1.ConditionStatus
		}
		for _, nodeName := range dsNodeNames {
			condList := []condValue{
				// Node aggregated conditions
				{
					condType: fmt.Sprintf("Node%sAvailable", nodeName),
					status:   corev1.ConditionTrue,
				},
				{
					condType: fmt.Sprintf("Node%sProgressing", nodeName),
					status:   corev1.ConditionFalse,
				},
				{
					condType: fmt.Sprintf("Node%sDegraded", nodeName),
					status:   corev1.ConditionFalse,
				},
				// Controller conditions
				{
					condType: fmt.Sprintf("FilesystemControllerNode%sProgressing", nodeName),
					status:   corev1.ConditionFalse,
				},
				{
					condType: fmt.Sprintf("FilesystemControllerNode%sDegraded", nodeName),
					status:   corev1.ConditionFalse,
				},
				{
					condType: fmt.Sprintf("MountControllerNode%sProgressing", nodeName),
					status:   corev1.ConditionFalse,
				},
				{
					condType: fmt.Sprintf("MountControllerNode%sDegraded", nodeName),
					status:   corev1.ConditionFalse,
				},
				{
					condType: fmt.Sprintf("RaidControllerNode%sProgressing", nodeName),
					status:   corev1.ConditionFalse,
				},
				{
					condType: fmt.Sprintf("RaidControllerNode%sDegraded", nodeName),
					status:   corev1.ConditionFalse,
				},
				{
					condType: fmt.Sprintf("LoopDeviceControllerNode%sProgressing", nodeName),
					status:   corev1.ConditionFalse,
				},
				{
					condType: fmt.Sprintf("LoopDeviceControllerNode%sDegraded", nodeName),
					status:   corev1.ConditionFalse,
				},
			}

			for _, item := range condList {
				expectedConditions = append(expectedConditions, scyllav1alpha1.NodeConfigCondition{
					Type:               scyllav1alpha1.NodeConfigConditionType(item.condType),
					Status:             item.status,
					Reason:             "AsExpected",
					Message:            "",
					ObservedGeneration: nc.Generation,
				})
			}

			expectedConditions = append(expectedConditions, scyllav1alpha1.NodeConfigCondition{
				Type:               scyllav1alpha1.NodeConfigReconciledConditionType,
				Status:             corev1.ConditionTrue,
				Reason:             "FullyReconciledAndUp",
				Message:            "All operands are reconciled and available.",
				ObservedGeneration: nc.Generation,
			})
		}

		return expectedConditions
	}()...))
}
