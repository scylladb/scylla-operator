package scyllacluster

import (
	"context"

	o "github.com/onsi/gomega"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	cqlclientv1alpha1 "github.com/scylladb/scylla-operator/pkg/scylla/api/cqlclient/v1alpha1"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/scheme"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
)

func WaitForFullQuorum(ctx context.Context, client corev1client.CoreV1Interface, sc *scyllav1.ScyllaCluster) {
	dcClientMap := make(map[string]corev1client.CoreV1Interface, 1)
	dcClientMap[sc.Spec.Datacenter.Name] = client
	WaitForFullMultiDCQuorum(ctx, dcClientMap, []*scyllav1.ScyllaCluster{sc})
}

func WaitForFullMultiDCQuorum(ctx context.Context, dcClientMap map[string]corev1client.CoreV1Interface, scs []*scyllav1.ScyllaCluster) {
	framework.By("Waiting for the ScyllaCluster(s) to reach consistency ALL")
	err := utils.WaitForFullMultiDCQuorum(ctx, dcClientMap, scs)
	o.Expect(err).NotTo(o.HaveOccurred())
}

type VerifyCQLConnectionConfigsOptions struct {
	Domains               []string
	Datacenters           []string
	ServingCAData         []byte
	ClientCertificateData []byte
	ClientKeyData         []byte
}

func VerifyAndParseCQLConnectionConfigs(secret *corev1.Secret, options VerifyCQLConnectionConfigsOptions) map[string]*cqlclientv1alpha1.CQLConnectionConfig {
	o.Expect(secret.Type).To(o.Equal(corev1.SecretType("Opaque")))
	o.Expect(secret.Data).To(o.HaveLen(len(options.Domains)))

	connectionConfigs := make(map[string]*cqlclientv1alpha1.CQLConnectionConfig, len(secret.Data))
	for _, domain := range options.Domains {
		o.Expect(secret.Data).To(o.HaveKey(domain))
		obj, err := runtime.Decode(
			scheme.Codecs.DecoderToVersion(scheme.Codecs.UniversalDeserializer(), cqlclientv1alpha1.GroupVersion),
			secret.Data[domain],
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		cfg := obj.(*cqlclientv1alpha1.CQLConnectionConfig)

		o.Expect(cfg.Datacenters).NotTo(o.BeEmpty())
		o.Expect(cfg.Datacenters).To(o.HaveLen(len(options.Datacenters)))
		for _, dcName := range options.Datacenters {
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
		o.Expect(defaultContext.DatacenterName).To(o.Equal(options.Datacenters[0]))
		o.Expect(defaultContext.AuthInfoName).To(o.Equal("admin"))

		o.Expect(cfg.CurrentContext).To(o.Equal("default"))

		o.Expect(cfg.Parameters).NotTo(o.BeNil())
		o.Expect(cfg.Parameters.DefaultConsistency).To(o.BeEquivalentTo("QUORUM"))
		o.Expect(cfg.Parameters.DefaultSerialConsistency).To(o.BeEquivalentTo("SERIAL"))

		connectionConfigs[domain] = cfg
	}

	return connectionConfigs
}
