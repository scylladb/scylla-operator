package scyllacluster

import (
	"context"
	"fmt"
	"sort"
	"strings"

	o "github.com/onsi/gomega"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	scyllaclient "github.com/scylladb/scylla-operator/pkg/client/scylla/clientset/versioned"
	"github.com/scylladb/scylla-operator/pkg/features"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	appsv1 "k8s.io/api/apps/v1"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	"k8s.io/client-go/kubernetes"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
)

func verifyPersistentVolumeClaims(ctx context.Context, coreClient corev1client.CoreV1Interface, sc *scyllav1.ScyllaCluster) {
	pvcList, err := coreClient.PersistentVolumeClaims(sc.Namespace).List(ctx, metav1.ListOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	framework.Infof("Found %d pvc(s) in namespace %q", len(pvcList.Items), sc.Namespace)

	pvcNamePrefix := naming.PVCNamePrefixForScyllaCluster(sc)

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
	framework.Infof("Found %d pvc(s) for ScyllaCluster %q", len(scPVCNames), naming.ObjRef(sc))

	var expectedPvcNames []string
	for _, rack := range sc.Spec.Datacenter.Racks {
		for ord := int32(0); ord < rack.Members; ord++ {
			stsName := naming.StatefulSetNameForRackForScyllaCluster(rack, sc)
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
	podMap, err := utils.GetPodsForStatefulSet(ctx, client, sts)
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

func verifyPodDisruptionBudget(sc *scyllav1.ScyllaCluster, pdb *policyv1.PodDisruptionBudget, sdc *scyllav1alpha1.ScyllaDBDatacenter) {
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
	o.Expect(pdb.Spec.Selector.MatchLabels).To(o.Equal(naming.ClusterLabelsForScyllaCluster(sc)))
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

func VerifyWithOptions(ctx context.Context, kubeClient kubernetes.Interface, scyllaClient scyllaclient.Interface, sc *scyllav1.ScyllaCluster, options VerifyOptions) {
	framework.By("Verifying the ScyllaCluster")

	sc = sc.DeepCopy()

	o.Expect(sc.CreationTimestamp).NotTo(o.BeNil())
	o.Expect(sc.Status.ObservedGeneration).NotTo(o.BeNil())
	o.Expect(*sc.Status.ObservedGeneration).To(o.BeNumerically(">=", sc.Generation))
	o.Expect(sc.Status.Racks).To(o.HaveLen(len(sc.Spec.Datacenter.Racks)))

	for i := range sc.Status.Conditions {
		c := &sc.Status.Conditions[i]
		o.Expect(c.LastTransitionTime).NotTo(o.BeNil())
		o.Expect(c.LastTransitionTime.Time.Before(sc.CreationTimestamp.Time)).NotTo(o.BeTrue())

		// To be able to compare the statuses we need to remove the random timestamp.
		c.LastTransitionTime = metav1.Time{}
	}
	o.Expect(sc.Status.Conditions).To(o.ConsistOf(func() []interface{} {
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
			{
				condType: "ScyllaDBDatacenterControllerProgressing",
				status:   metav1.ConditionFalse,
			},
			{
				condType: "ScyllaDBDatacenterControllerDegraded",
				status:   metav1.ConditionFalse,
			},
			{
				condType: "ScyllaDBManagerTaskControllerProgressing",
				status:   metav1.ConditionFalse,
			},
			{
				condType: "ScyllaDBManagerTaskControllerDegraded",
				status:   metav1.ConditionFalse,
			},
		}

		if utilfeature.DefaultMutableFeatureGate.Enabled(features.AutomaticTLSCertificates) || sc.Spec.Alternator != nil {
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
				ObservedGeneration: sc.Generation,
			})
		}

		return expectedConditions
	}()...))

	sdc, err := scyllaClient.ScyllaV1alpha1().ScyllaDBDatacenters(sc.Namespace).Get(ctx, sc.Name, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(sdc.ObjectMeta.OwnerReferences).To(o.BeEquivalentTo(
		[]metav1.OwnerReference{
			{
				APIVersion:         "scylla.scylladb.com/v1",
				Kind:               "ScyllaCluster",
				Name:               sc.Name,
				UID:                sc.UID,
				BlockOwnerDeletion: pointer.Ptr(true),
				Controller:         pointer.Ptr(true),
			},
		}),
	)
	statefulsets, err := utils.GetStatefulSetsForScyllaCluster(ctx, kubeClient.AppsV1(), sc)
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(statefulsets).To(o.HaveLen(len(sc.Spec.Datacenter.Racks)))

	memberCount := 0
	for _, r := range sc.Spec.Datacenter.Racks {
		memberCount += int(r.Members)

		s := statefulsets[r.Name]

		verifyStatefulset(ctx, kubeClient.CoreV1(), s, sdc, options.VerifyStatefulSetOptions)

		o.Expect(sc.Status.Racks[r.Name].Stale).NotTo(o.BeNil())
		o.Expect(*sc.Status.Racks[r.Name].Stale).To(o.BeFalse())
		o.Expect(sc.Status.Racks[r.Name].ReadyMembers).To(o.Equal(r.Members))
		o.Expect(sc.Status.Racks[r.Name].ReadyMembers).To(o.Equal(s.Status.ReadyReplicas))
		o.Expect(sc.Status.Racks[r.Name].UpdatedMembers).NotTo(o.BeNil())
		o.Expect(*sc.Status.Racks[r.Name].UpdatedMembers).To(o.Equal(s.Status.UpdatedReplicas))
	}

	if sc.Status.Upgrade != nil {
		o.Expect(sc.Status.Upgrade.FromVersion).To(o.Equal(sc.Status.Upgrade.ToVersion))
	}

	pdb, err := kubeClient.PolicyV1().PodDisruptionBudgets(sc.Namespace).Get(ctx, naming.PodDisruptionBudgetNameForScyllaCluster(sc), metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	verifyPodDisruptionBudget(sc, pdb, sdc)

	verifyPersistentVolumeClaims(ctx, kubeClient.CoreV1(), sc)

	clusterClient, hosts, err := utils.GetScyllaClient(ctx, kubeClient.CoreV1(), sc)
	o.Expect(err).NotTo(o.HaveOccurred())
	defer clusterClient.Close()

	o.Expect(hosts).To(o.HaveLen(memberCount))
}

func Verify(ctx context.Context, kubeClient kubernetes.Interface, scyllaClient scyllaclient.Interface, sc *scyllav1.ScyllaCluster) {
	VerifyWithOptions(
		ctx,
		kubeClient,
		scyllaClient,
		sc,
		VerifyOptions{
			VerifyStatefulSetOptions: VerifyStatefulSetOptions{
				PodRestartCountAssertion: func(a o.Assertion, containerName, podName string) {
					a.To(o.BeZero(), fmt.Sprintf("container %q in pod %q should not be restarted", containerName, podName))
				},
			},
		},
	)
}
