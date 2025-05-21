// Copyright (C) 2025 ScyllaDB

package scylladbdatacenter

import (
	"context"
	"fmt"
	"sort"
	"strings"

	o "github.com/onsi/gomega"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	scyllaclient "github.com/scylladb/scylla-operator/pkg/client/scylla/clientset/versioned"
	"github.com/scylladb/scylla-operator/pkg/features"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	utilsv1alpha1 "github.com/scylladb/scylla-operator/test/e2e/utils/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	"k8s.io/client-go/kubernetes"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
)

func verifyPersistentVolumeClaims(ctx context.Context, coreClient corev1client.CoreV1Interface, sdc *scyllav1alpha1.ScyllaDBDatacenter) {
	pvcList, err := coreClient.PersistentVolumeClaims(sdc.Namespace).List(ctx, metav1.ListOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	framework.Infof("Found %d pvc(s) in namespace %q", len(pvcList.Items), sdc.Namespace)

	pvcNamePrefix := naming.PVCNamePrefix(sdc)

	var scPVCNames []string
	for _, pvc := range pvcList.Items {
		if pvc.DeletionTimestamp != nil {
			framework.Infof("pvc %s is being deleted", naming.ObjRef(&pvc))
			continue
		}

		if !strings.HasPrefix(pvc.Name, pvcNamePrefix) {
			framework.Infof("pvc %s doesn't match the prefix %q", naming.ObjRef(&pvc), pvcNamePrefix)
			continue
		}

		scPVCNames = append(scPVCNames, pvc.Name)
	}
	framework.Infof("Found %d pvc(s) for ScyllaDBDatacenter %q", len(scPVCNames), naming.ObjRef(sdc))

	rackTemplateNodeCount := int32(0)
	if sdc.Spec.RackTemplate != nil && sdc.Spec.RackTemplate.Nodes != nil {
		rackTemplateNodeCount = *sdc.Spec.RackTemplate.Nodes
	}

	var expectedPvcNames []string
	for _, rack := range sdc.Spec.Racks {
		nodeCount := rackTemplateNodeCount
		if rack.Nodes != nil {
			nodeCount = *rack.Nodes
		}

		for ord := int32(0); ord < nodeCount; ord++ {
			stsName := naming.StatefulSetNameForRack(rack, sdc)
			expectedPvcNames = append(expectedPvcNames, naming.PVCNameForStatefulSet(stsName, ord))
		}
	}

	sort.Strings(scPVCNames)
	sort.Strings(expectedPvcNames)
	o.Expect(scPVCNames).To(o.BeEquivalentTo(expectedPvcNames))
}

type VerifyStatefulSetOptions struct {
	PodRestartCountAssertion func(a o.Assertion, containerName, podName string)
}

func verifyStatefulset(ctx context.Context, client corev1client.CoreV1Interface, sts *appsv1.StatefulSet, sdc *scyllav1alpha1.ScyllaDBDatacenter, options VerifyStatefulSetOptions) {
	o.Expect(sts.ObjectMeta.OwnerReferences).To(o.BeEquivalentTo(
		[]metav1.OwnerReference{
			{
				APIVersion:         "scylla.scylladb.com/v1alpha1",
				Kind:               "ScyllaDBDatacenter",
				Name:               sdc.Name,
				UID:                sdc.UID,
				BlockOwnerDeletion: pointer.Ptr(true),
				Controller:         pointer.Ptr(true),
			},
		}),
	)
	o.Expect(sts.DeletionTimestamp).To(o.BeNil())
	o.Expect(sts.Status.ObservedGeneration).To(o.Equal(sts.Generation))
	o.Expect(sts.Spec.Replicas).NotTo(o.BeNil())
	o.Expect(sts.Status.Replicas).To(o.Equal(*sts.Spec.Replicas))
	o.Expect(sts.Status.ReadyReplicas).To(o.Equal(*sts.Spec.Replicas))
	o.Expect(sts.Status.CurrentRevision).To(o.Equal(sts.Status.UpdateRevision))

	// Verify pod invariants.
	podMap, err := utilsv1alpha1.GetPodsForStatefulSet(ctx, client, sts)
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(podMap).To(o.HaveLen(int(sts.Status.Replicas)))

	// No container in the Pod should be restarted.
	for _, pod := range podMap {
		o.Expect(pod.Status.ContainerStatuses).NotTo(o.BeEmpty())
		for _, cs := range pod.Status.ContainerStatuses {
			options.PodRestartCountAssertion(o.ExpectWithOffset(1, cs.RestartCount), cs.Name, pod.Name)
		}
	}
}

func verifyPodDisruptionBudget(pdb *policyv1.PodDisruptionBudget, sdc *scyllav1alpha1.ScyllaDBDatacenter) {
	o.Expect(pdb.ObjectMeta.OwnerReferences).To(o.BeEquivalentTo(
		[]metav1.OwnerReference{
			{
				APIVersion:         "scylla.scylladb.com/v1alpha1",
				Kind:               "ScyllaDBDatacenter",
				Name:               sdc.Name,
				UID:                sdc.UID,
				BlockOwnerDeletion: pointer.Ptr(true),
				Controller:         pointer.Ptr(true),
			},
		}),
	)
	o.Expect(pdb.Spec.MaxUnavailable.IntValue()).To(o.Equal(1))
	o.Expect(pdb.Spec.Selector).ToNot(o.BeNil())
	o.Expect(pdb.Spec.Selector.MatchLabels).To(o.Equal(naming.ClusterLabels(sdc)))
	o.Expect(pdb.Spec.Selector.MatchExpressions).To(o.Equal([]metav1.LabelSelectorRequirement{
		{
			Key:      "batch.kubernetes.io/job-name",
			Operator: metav1.LabelSelectorOpDoesNotExist,
		},
	}))
}

type VerifyOptions struct {
	VerifyStatefulSetOptions
}

func VerifyWithOptions(ctx context.Context, kubeClient kubernetes.Interface, scyllaClient scyllaclient.Interface, sdc *scyllav1alpha1.ScyllaDBDatacenter, options VerifyOptions) {
	framework.By("Verifying ScyllaDBDatacenter")

	sdc = sdc.DeepCopy()

	o.Expect(sdc.CreationTimestamp).NotTo(o.BeNil())
	o.Expect(sdc.Status.ObservedGeneration).NotTo(o.BeNil())
	o.Expect(*sdc.Status.ObservedGeneration).To(o.BeNumerically(">=", sdc.Generation))
	o.Expect(sdc.Status.Racks).To(o.HaveLen(len(sdc.Spec.Racks)))

	for i := range sdc.Status.Conditions {
		c := &sdc.Status.Conditions[i]
		o.Expect(c.LastTransitionTime).NotTo(o.BeNil())
		o.Expect(c.LastTransitionTime.Time.Before(sdc.CreationTimestamp.Time)).NotTo(o.BeTrue())

		// To be able to compare the statuses we need to remove the random timestamp.
		c.LastTransitionTime = metav1.Time{}
	}
	o.Expect(sdc.Status.Conditions).To(o.ConsistOf(func() []interface{} {
		type condValue struct {
			condType string
			status   metav1.ConditionStatus
		}
		condList := []condValue{
			// Aggregated conditions
			{
				condType: "Available",
				status:   metav1.ConditionTrue,
			},
			{
				condType: "Progressing",
				status:   metav1.ConditionFalse,
			},
			{
				condType: "Degraded",
				status:   metav1.ConditionFalse,
			},

			// Controller conditions
			{
				condType: "ServiceAccountControllerProgressing",
				status:   metav1.ConditionFalse,
			},
			{
				condType: "ServiceAccountControllerDegraded",
				status:   metav1.ConditionFalse,
			},
			{
				condType: "RoleBindingControllerProgressing",
				status:   metav1.ConditionFalse,
			},
			{
				condType: "RoleBindingControllerDegraded",
				status:   metav1.ConditionFalse,
			},
			{
				condType: "AgentTokenControllerProgressing",
				status:   metav1.ConditionFalse,
			},
			{
				condType: "AgentTokenControllerDegraded",
				status:   metav1.ConditionFalse,
			},
			{
				condType: "StatefulSetControllerAvailable",
				status:   metav1.ConditionTrue,
			},
			{
				condType: "StatefulSetControllerProgressing",
				status:   metav1.ConditionFalse,
			},
			{
				condType: "StatefulSetControllerDegraded",
				status:   metav1.ConditionFalse,
			},
			{
				condType: "ServiceControllerProgressing",
				status:   metav1.ConditionFalse,
			},
			{
				condType: "ServiceControllerDegraded",
				status:   metav1.ConditionFalse,
			},
			{
				condType: "PDBControllerProgressing",
				status:   metav1.ConditionFalse,
			},
			{
				condType: "PDBControllerDegraded",
				status:   metav1.ConditionFalse,
			},
			{
				condType: "IngressControllerProgressing",
				status:   metav1.ConditionFalse,
			},
			{
				condType: "IngressControllerDegraded",
				status:   metav1.ConditionFalse,
			},
			{
				condType: "JobControllerProgressing",
				status:   metav1.ConditionFalse,
			},
			{
				condType: "JobControllerDegraded",
				status:   metav1.ConditionFalse,
			},
			{
				condType: "ConfigControllerProgressing",
				status:   metav1.ConditionFalse,
			},
			{
				condType: "ConfigControllerDegraded",
				status:   metav1.ConditionFalse,
			},
		}

		if utilfeature.DefaultMutableFeatureGate.Enabled(features.AutomaticTLSCertificates) || sdc.Spec.ScyllaDB.AlternatorOptions != nil {
			condList = append(condList,
				condValue{
					condType: "CertControllerProgressing",
					status:   metav1.ConditionFalse,
				},
				condValue{
					condType: "CertControllerDegraded",
					status:   metav1.ConditionFalse,
				},
			)
		}

		expectedConditions := make([]interface{}, 0, len(condList))
		for _, item := range condList {
			expectedConditions = append(expectedConditions, metav1.Condition{
				Type:               item.condType,
				Status:             item.status,
				Reason:             "AsExpected",
				Message:            "",
				ObservedGeneration: sdc.Generation,
			})
		}

		return expectedConditions
	}()...))

	statefulsets, err := utilsv1alpha1.GetStatefulSetsForScyllaDBDatacenter(ctx, kubeClient.AppsV1(), sdc)
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(statefulsets).To(o.HaveLen(len(sdc.Spec.Racks)))

	rackStatusMap := map[string]scyllav1alpha1.RackStatus{}
	for _, rackStatus := range sdc.Status.Racks {
		rackStatusMap[rackStatus.Name] = rackStatus
	}

	rackTemplateNodeCount := int32(0)
	if sdc.Spec.RackTemplate != nil && sdc.Spec.RackTemplate.Nodes != nil {
		rackTemplateNodeCount = *sdc.Spec.RackTemplate.Nodes
	}

	nodeCount := int32(0)
	for _, r := range sdc.Spec.Racks {
		rackNodeCount := rackTemplateNodeCount
		if r.Nodes != nil {
			rackNodeCount = *r.Nodes
		}
		nodeCount += rackNodeCount

		s := statefulsets[r.Name]

		verifyStatefulset(ctx, kubeClient.CoreV1(), s, sdc, options.VerifyStatefulSetOptions)

		rackStatus, ok := rackStatusMap[r.Name]
		o.Expect(ok).To(o.BeTrue(), fmt.Sprintf("rack %s is expected in status", r.Name))
		o.Expect(rackStatus.Stale).NotTo(o.BeNil())
		o.Expect(*rackStatus.Stale).To(o.BeFalse())
		o.Expect(rackStatus.Nodes).NotTo(o.BeNil())
		o.Expect(*rackStatus.Nodes).To(o.Equal(rackNodeCount))
		o.Expect(rackStatus.ReadyNodes).NotTo(o.BeNil())
		o.Expect(*rackStatus.ReadyNodes).To(o.Equal(rackNodeCount))
		o.Expect(*rackStatus.ReadyNodes).To(o.Equal(s.Status.ReadyReplicas))
		o.Expect(rackStatus.AvailableNodes).NotTo(o.BeNil())
		o.Expect(*rackStatus.AvailableNodes).To(o.Equal(rackNodeCount))
		o.Expect(rackStatus.UpdatedNodes).NotTo(o.BeNil())
		o.Expect(*rackStatus.UpdatedNodes).To(o.Equal(rackNodeCount))
		o.Expect(*rackStatus.UpdatedNodes).To(o.Equal(s.Status.UpdatedReplicas))
	}

	pdb, err := kubeClient.PolicyV1().PodDisruptionBudgets(sdc.Namespace).Get(ctx, naming.PodDisruptionBudgetName(sdc), metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	verifyPodDisruptionBudget(pdb, sdc)

	verifyPersistentVolumeClaims(ctx, kubeClient.CoreV1(), sdc)

	clusterClient, hosts, err := utilsv1alpha1.GetScyllaClient(ctx, kubeClient.CoreV1(), sdc)
	o.Expect(err).NotTo(o.HaveOccurred())
	defer clusterClient.Close()

	o.Expect(hosts).To(o.HaveLen(int(nodeCount)))
}

func Verify(ctx context.Context, kubeClient kubernetes.Interface, scyllaClient scyllaclient.Interface, sdc *scyllav1alpha1.ScyllaDBDatacenter) {
	VerifyWithOptions(
		ctx,
		kubeClient,
		scyllaClient,
		sdc,
		VerifyOptions{
			VerifyStatefulSetOptions: VerifyStatefulSetOptions{
				PodRestartCountAssertion: func(a o.Assertion, containerName, podName string) {
					a.To(o.BeZero(), fmt.Sprintf("container %q in pod %q should not be restarted", containerName, podName))
				},
			},
		},
	)
}
