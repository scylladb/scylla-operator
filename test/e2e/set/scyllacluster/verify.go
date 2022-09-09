package scyllacluster

import (
	"context"
	"sort"
	"strings"

	o "github.com/onsi/gomega"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	appsv1 "k8s.io/api/apps/v1"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/utils/pointer"
)

func verifyPersistentVolumeClaims(ctx context.Context, coreClient corev1client.CoreV1Interface, sc *scyllav1.ScyllaCluster) {
	pvcList, err := coreClient.PersistentVolumeClaims(sc.Namespace).List(ctx, metav1.ListOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	framework.Infof("Found %d pvc(s) in namespace %q", len(pvcList.Items), sc.Namespace)

	pvcNamePrefix := naming.PVCNamePrefixForScyllaCluster(sc.Name)

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
			stsName := naming.StatefulSetNameForRack(rack, sc)
			expectedPvcNames = append(expectedPvcNames, naming.PVCNameForStatefulSet(stsName, ord))
		}
	}

	sort.Strings(scPVCNames)
	sort.Strings(expectedPvcNames)
	o.Expect(scPVCNames).To(o.BeEquivalentTo(expectedPvcNames))
}

func verifyStatefulset(sts *appsv1.StatefulSet) {
	o.Expect(sts.DeletionTimestamp).To(o.BeNil())
	o.Expect(sts.Status.ObservedGeneration).To(o.Equal(sts.Generation))
	o.Expect(sts.Spec.Replicas).NotTo(o.BeNil())
	o.Expect(sts.Status.ReadyReplicas).To(o.Equal(*sts.Spec.Replicas))
	o.Expect(sts.Status.CurrentRevision).To(o.Equal(sts.Status.UpdateRevision))
}

func verifyPodDisruptionBudget(sc *scyllav1.ScyllaCluster, pdb *policyv1.PodDisruptionBudget) {
	o.Expect(pdb.ObjectMeta.OwnerReferences).To(o.BeEquivalentTo(
		[]metav1.OwnerReference{
			{
				APIVersion:         "scylla.scylladb.com/v1",
				Kind:               "ScyllaCluster",
				Name:               sc.Name,
				UID:                sc.UID,
				BlockOwnerDeletion: pointer.Bool(true),
				Controller:         pointer.Bool(true),
			},
		}),
	)
	o.Expect(pdb.Spec.MaxUnavailable.IntValue()).To(o.Equal(1))
	o.Expect(pdb.Spec.Selector).To(o.Equal(metav1.SetAsLabelSelector(naming.ClusterLabels(sc))))
}

func verifyScyllaCluster(ctx context.Context, kubeClient kubernetes.Interface, sc *scyllav1.ScyllaCluster, di *DataInserter) {
	framework.By("Verifying the ScyllaCluster")

	o.Expect(sc.Status.ObservedGeneration).NotTo(o.BeNil())
	o.Expect(sc.Status.Racks).To(o.HaveLen(len(sc.Spec.Datacenter.Racks)))

	statefulsets, err := utils.GetStatefulSetsForScyllaCluster(ctx, kubeClient.AppsV1(), sc)
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(statefulsets).To(o.HaveLen(len(sc.Spec.Datacenter.Racks)))

	memberCount := 0
	for _, r := range sc.Spec.Datacenter.Racks {
		memberCount += int(r.Members)

		s := statefulsets[r.Name]

		verifyStatefulset(s)

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

	pdb, err := kubeClient.PolicyV1().PodDisruptionBudgets(sc.Namespace).Get(ctx, naming.PodDisruptionBudgetName(sc), metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	verifyPodDisruptionBudget(sc, pdb)

	verifyPersistentVolumeClaims(ctx, kubeClient.CoreV1(), sc)

	// TODO: Use scylla client to check at least "UN"
	scyllaClient, hosts, err := utils.GetScyllaClient(ctx, kubeClient.CoreV1(), sc)
	o.Expect(err).NotTo(o.HaveOccurred())
	defer scyllaClient.Close()

	o.Expect(hosts).To(o.HaveLen(memberCount))

	status, err := scyllaClient.Status(ctx, "")
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(status.DownHosts()).To(o.BeEmpty())
	o.Expect(status.LiveHosts()).To(o.HaveLen(memberCount))

	if di != nil {
		read, err := di.Read()
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Verifying data consistency")
		o.Expect(read).To(o.Equal(di.GetExpected()))
	}
}
