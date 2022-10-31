package scyllacluster

import (
	"context"
	"crypto"
	"crypto/x509"
	"sort"
	"strings"

	o "github.com/onsi/gomega"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	ocrypto "github.com/scylladb/scylla-operator/pkg/crypto"
	"github.com/scylladb/scylla-operator/pkg/features"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
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

func verifyScyllaCluster(ctx context.Context, kubeClient kubernetes.Interface, sc *scyllav1.ScyllaCluster) {
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
		}

		if utilfeature.DefaultMutableFeatureGate.Enabled(features.AutomaticTLSCertificates) {
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

	scyllaClient, hosts, err := utils.GetScyllaClient(ctx, kubeClient.CoreV1(), sc)
	o.Expect(err).NotTo(o.HaveOccurred())
	defer scyllaClient.Close()

	o.Expect(hosts).To(o.HaveLen(memberCount))
}

func getScyllaHostsAndWaitForFullQuorum(ctx context.Context, client corev1client.CoreV1Interface, sc *scyllav1.ScyllaCluster) []string {
	framework.By("Waiting for the ScyllaCluster to reach consistency ALL")
	hosts, err := utils.GetScyllaHostsAndWaitForFullQuorum(ctx, client, sc)
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(hosts).To(o.HaveLen(int(utils.GetMemberCount(sc))))

	return hosts
}

func verifyCQLData(ctx context.Context, di *utils.DataInserter) {
	framework.By("Verifying the data")
	data, err := di.Read()
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(data).To(o.Equal(di.GetExpected()))
}

func insertAndVerifyCQLData(ctx context.Context, hosts []string) *utils.DataInserter {
	framework.By("Inserting data")
	di, err := utils.NewDataInserter(hosts)
	o.Expect(err).NotTo(o.HaveOccurred())

	err = di.Insert()
	o.Expect(err).NotTo(o.HaveOccurred())

	verifyCQLData(ctx, di)

	return di
}

type verifyTLSCertOptions struct {
	isCA     *bool
	keyUsage *x509.KeyUsage
}

func verifyAndParseTLSCert(secret *corev1.Secret, options verifyTLSCertOptions) ([]*x509.Certificate, []byte, crypto.PrivateKey, []byte) {
	o.Expect(secret.Type).To(o.Equal(corev1.SecretType("kubernetes.io/tls")))
	o.Expect(secret.Data).To(o.HaveKey("tls.crt"))
	o.Expect(secret.Data).To(o.HaveKey("tls.key"))

	certsBytes := secret.Data["tls.crt"]
	keyBytes := secret.Data["tls.key"]
	o.Expect(certsBytes).NotTo(o.BeEmpty())
	o.Expect(keyBytes).NotTo(o.BeEmpty())

	certs, key, err := ocrypto.GetTLSCertificatesFromBytes(certsBytes, keyBytes)
	o.Expect(err).NotTo(o.HaveOccurred())

	o.Expect(certs).NotTo(o.BeEmpty())
	o.Expect(certs[0].IsCA).To(o.Equal(*options.isCA))
	o.Expect(certs[0].KeyUsage).To(o.Equal(*options.keyUsage))

	o.Expect(key.Validate()).To(o.Succeed())

	return certs, certsBytes, key, keyBytes
}

func verifyAndParseCABundle(cm *corev1.ConfigMap) []*x509.Certificate {
	o.Expect(cm.Data).To(o.HaveKey("ca-bundle.crt"))

	bundleBytes := cm.Data["ca-bundle.crt"]
	o.Expect(bundleBytes).NotTo(o.BeEmpty())

	certs, err := ocrypto.DecodeCertificates([]byte(bundleBytes))
	o.Expect(err).NotTo(o.HaveOccurred())

	return certs
}
