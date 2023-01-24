package scyllacluster

import (
	"context"
	"sort"
	"strings"

	o "github.com/onsi/gomega"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/features"
	"github.com/scylladb/scylla-operator/pkg/naming"
	cqlclientv1alpha1 "github.com/scylladb/scylla-operator/pkg/scylla/api/cqlclient/v1alpha1"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/scheme"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
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
	err := di.AwaitSchemaAgreement(ctx)
	o.Expect(err).NotTo(o.HaveOccurred())

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

type verifyCQLConnectionConfigsOptions struct {
	domains               []string
	datacenters           []string
	ServingCAData         []byte
	ClientCertificateData []byte
	ClientKeyData         []byte
}

func verifyAndParseCQLConnectionConfigs(secret *corev1.Secret, options verifyCQLConnectionConfigsOptions) map[string]*cqlclientv1alpha1.CQLConnectionConfig {
	o.Expect(secret.Type).To(o.Equal(corev1.SecretType("Opaque")))
	o.Expect(secret.Data).To(o.HaveLen(len(options.domains)))

	connectionConfigs := make(map[string]*cqlclientv1alpha1.CQLConnectionConfig, len(secret.Data))
	for _, domain := range options.domains {
		o.Expect(secret.Data).To(o.HaveKey(domain))
		obj, err := runtime.Decode(
			scheme.Codecs.DecoderToVersion(scheme.Codecs.UniversalDeserializer(), cqlclientv1alpha1.GroupVersion),
			secret.Data[domain],
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		cfg := obj.(*cqlclientv1alpha1.CQLConnectionConfig)

		o.Expect(cfg.Datacenters).NotTo(o.BeEmpty())
		o.Expect(cfg.Datacenters).To(o.HaveLen(len(options.datacenters)))
		for _, dcName := range options.datacenters {
			o.Expect(cfg.Datacenters).To(o.HaveKey(dcName))
			dc := cfg.Datacenters[dcName]
			o.Expect(dc.Server).To(o.Equal("cql." + domain))
			o.Expect(dc.NodeDomain).To(o.Equal("cql." + domain))
			o.Expect(dc.InsecureSkipTLSVerify).To(o.BeFalse())
			o.Expect(dc.CertificateAuthorityData).To(o.Equal(options.ServingCAData))
			o.Expect(dc.CertificateAuthorityPath).To(o.BeEmpty())
			o.Expect(dc.ProxyURL).To(o.BeEmpty())
		}

		o.Expect(cfg.AuthInfos).To(o.HaveLen(1))
		o.Expect(cfg.AuthInfos).To(o.HaveKey("admin"))
		admAuthInfo := cfg.AuthInfos["admin"]
		o.Expect(admAuthInfo.Username).To(o.Equal("cassandra"))
		o.Expect(admAuthInfo.Password).To(o.Equal("cassandra"))
		o.Expect(admAuthInfo.ClientCertificateData).To(o.Equal(options.ClientCertificateData))
		o.Expect(admAuthInfo.ClientCertificatePath).To(o.BeEmpty())
		o.Expect(admAuthInfo.ClientKeyData).To(o.Equal(options.ClientKeyData))
		o.Expect(admAuthInfo.ClientKeyPath).To(o.BeEmpty())

		o.Expect(cfg.Contexts).To(o.HaveLen(1))
		o.Expect(cfg.Contexts).To(o.HaveKey("default"))
		defaultContext := cfg.Contexts["default"]
		o.Expect(defaultContext.DatacenterName).To(o.Equal(options.datacenters[0]))
		o.Expect(defaultContext.AuthInfoName).To(o.Equal("admin"))

		o.Expect(cfg.CurrentContext).To(o.Equal("default"))

		o.Expect(cfg.Parameters).NotTo(o.BeNil())
		o.Expect(cfg.Parameters.DefaultConsistency).To(o.BeEquivalentTo("QUORUM"))
		o.Expect(cfg.Parameters.DefaultSerialConsistency).To(o.BeEquivalentTo("SERIAL"))

		connectionConfigs[domain] = cfg
	}

	return connectionConfigs
}
