// Copyright (c) 2022 ScyllaDB.

package v1

import (
	"context"
	"crypto"
	"crypto/x509"
	"fmt"
	"sort"
	"strings"

	o "github.com/onsi/gomega"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	scyllaclient "github.com/scylladb/scylla-operator/pkg/client/scylla/clientset/versioned"
	ocrypto "github.com/scylladb/scylla-operator/pkg/crypto"
	"github.com/scylladb/scylla-operator/pkg/features"
	"github.com/scylladb/scylla-operator/pkg/naming"
	cqlclientv1alpha1 "github.com/scylladb/scylla-operator/pkg/scylla/api/cqlclient/v1alpha1"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/scheme"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	v1utils "github.com/scylladb/scylla-operator/test/e2e/utils/v1"
	v1alpha1utils "github.com/scylladb/scylla-operator/test/e2e/utils/v1alpha1"
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

func verifyPersistentVolumeClaims(ctx context.Context, coreClient corev1client.CoreV1Interface, sd *scyllav1alpha1.ScyllaDatacenter) {
	pvcList, err := coreClient.PersistentVolumeClaims(sd.Namespace).List(ctx, metav1.ListOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	framework.Infof("Found %d pvc(s) in namespace %q", len(pvcList.Items), sd.Namespace)

	pvcNamePrefix := fmt.Sprintf("%s-%s-", naming.PVCTemplateName, sd.Name)

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
	framework.Infof("Found %d pvc(s) for ScyllaCluster %q", len(scPVCNames), naming.ObjRef(sd))

	var expectedPvcNames []string
	for _, rack := range sd.Spec.Racks {
		for ord := int32(0); ord < *rack.Nodes; ord++ {
			stsName := fmt.Sprintf("%s-%s-%s", sd.Name, sd.Spec.DatacenterName, rack.Name)
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

func verifyPodDisruptionBudget(sd *scyllav1alpha1.ScyllaDatacenter, pdb *policyv1.PodDisruptionBudget) {
	o.Expect(pdb.ObjectMeta.OwnerReferences).To(o.BeEquivalentTo(
		[]metav1.OwnerReference{
			{
				APIVersion:         "scylla.scylladb.com/v1alpha1",
				Kind:               "ScyllaDatacenter",
				Name:               sd.Name,
				UID:                sd.UID,
				BlockOwnerDeletion: pointer.Bool(true),
				Controller:         pointer.Bool(true),
			},
		}),
	)
	clusterSelector := naming.ScyllaLabels()
	clusterSelector[naming.ClusterNameLabel] = sd.Name
	o.Expect(pdb.Spec.MaxUnavailable.IntValue()).To(o.Equal(1))
	o.Expect(pdb.Spec.Selector).To(o.Equal(metav1.SetAsLabelSelector(clusterSelector)))
}

func verifyScyllaCluster(ctx context.Context, kubeClient kubernetes.Interface, scyllaClient scyllaclient.Interface, sc *scyllav1.ScyllaCluster) {
	framework.By("Verifying the ScyllaCluster")

	o.Expect(sc.Status.ObservedGeneration).NotTo(o.BeNil())
	o.Expect(sc.Status.Racks).To(o.HaveLen(len(sc.Spec.Datacenter.Racks)))

	sd, err := scyllaClient.ScyllaV1alpha1().ScyllaDatacenters(sc.Namespace).Get(ctx, sc.Name, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(sd.ObjectMeta.OwnerReferences).To(o.BeEquivalentTo(
		[]metav1.OwnerReference{
			{
				APIVersion:         "scylla.scylladb.com/v2alpha1",
				Kind:               "ScyllaCluster",
				Name:               sc.Name,
				UID:                sc.UID,
				BlockOwnerDeletion: pointer.Bool(true),
				Controller:         pointer.Bool(true),
			},
		}),
	)

	verifyScyllaDatacenter(ctx, kubeClient, sd)

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

	if sc.Status.Upgrade != nil {
		o.Expect(sc.Status.Upgrade.FromVersion).To(o.Equal(sc.Status.Upgrade.ToVersion))
	}
}

func verifyScyllaDatacenter(ctx context.Context, kubeClient kubernetes.Interface, sd *scyllav1alpha1.ScyllaDatacenter) {
	framework.By("Verifying the ScyllaDatacenter")

	o.Expect(sd.Status.ObservedGeneration).NotTo(o.BeNil())
	o.Expect(sd.Status.Racks).To(o.HaveLen(len(sd.Spec.Racks)))

	statefulsets, err := v1alpha1utils.GetStatefulSetsForScyllaDatacenter(ctx, kubeClient.AppsV1(), sd)
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(statefulsets).To(o.HaveLen(len(sd.Spec.Racks)))

	memberCount := 0
	for _, r := range sd.Spec.Racks {
		memberCount += int(*r.Nodes)

		s := statefulsets[r.Name]

		verifyStatefulset(s)

		o.Expect(sd.Status.Racks[r.Name].Stale).NotTo(o.BeNil())
		o.Expect(*sd.Status.Racks[r.Name].Stale).To(o.BeFalse())

		o.Expect(sd.Status.Racks[r.Name].ReadyNodes).NotTo(o.BeNil())
		o.Expect(*sd.Status.Racks[r.Name].ReadyNodes).To(o.Equal(*r.Nodes))
		o.Expect(*sd.Status.Racks[r.Name].ReadyNodes).To(o.Equal(s.Status.ReadyReplicas))

		o.Expect(sd.Status.Racks[r.Name].UpdatedNodes).NotTo(o.BeNil())
		o.Expect(*sd.Status.Racks[r.Name].UpdatedNodes).To(o.Equal(*r.Nodes))
		o.Expect(*sd.Status.Racks[r.Name].UpdatedNodes).To(o.Equal(s.Status.UpdatedReplicas))
	}

	if sd.Status.Upgrade != nil {
		o.Expect(sd.Status.Upgrade.FromVersion).To(o.Equal(sd.Status.Upgrade.ToVersion))
	}

	pdb, err := kubeClient.PolicyV1().PodDisruptionBudgets(sd.Namespace).Get(ctx, naming.PodDisruptionBudgetName(sd), metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	verifyPodDisruptionBudget(sd, pdb)

	verifyPersistentVolumeClaims(ctx, kubeClient.CoreV1(), sd)
}

func getScyllaHostsAndWaitForFullQuorum(ctx context.Context, client corev1client.CoreV1Interface, sc *scyllav1.ScyllaCluster) []string {
	framework.By("Waiting for the ScyllaCluster to reach consistency ALL")
	hosts, err := v1utils.GetScyllaHostsAndWaitForFullQuorum(ctx, client, sc)
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(hosts).To(o.HaveLen(int(v1utils.GetMemberCount(sc))))

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

func verifyAndParseCABundle(cm *corev1.ConfigMap) ([]*x509.Certificate, []byte) {
	o.Expect(cm.Data).To(o.HaveKey("ca-bundle.crt"))

	bundleBytes := cm.Data["ca-bundle.crt"]
	o.Expect(bundleBytes).NotTo(o.BeEmpty())

	certs, err := ocrypto.DecodeCertificates([]byte(bundleBytes))
	o.Expect(err).NotTo(o.HaveOccurred())

	return certs, []byte(bundleBytes)
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
