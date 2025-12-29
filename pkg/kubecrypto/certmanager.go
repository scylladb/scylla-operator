package kubecrypto

import (
	"context"
	"crypto/x509/pkix"
	"fmt"
	"time"

	ocrypto "github.com/scylladb/scylla-operator/pkg/crypto"
	"github.com/scylladb/scylla-operator/pkg/helpers"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/resourceapply"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/record"
)

type MetaConfig struct {
	Name        string
	Labels      map[string]string
	Annotations map[string]string
}

func (c *MetaConfig) GetObjectMeta() *metav1.ObjectMeta {
	return (&metav1.ObjectMeta{
		Name:        c.Name,
		Labels:      c.Labels,
		Annotations: c.Annotations,
	}).DeepCopy()
}

type CAConfig struct {
	MetaConfig
	Validity time.Duration
	Refresh  time.Duration
}

func (c *CAConfig) GetMetaSecret() *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: *c.GetObjectMeta(),
	}
}

type CABundleConfig struct {
	MetaConfig
}

func (c *CABundleConfig) GetMetaConfigMap() *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: *c.GetObjectMeta(),
	}
}

type CertificateConfig struct {
	MetaConfig
	Validity    time.Duration
	Refresh     time.Duration
	CertCreator ocrypto.CertCreator
}

func (c *CertificateConfig) GetMetaSecret() *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: *c.GetObjectMeta(),
	}
}

type CertChainConfig struct {
	CAConfig       *CAConfig
	CABundleConfig *CABundleConfig
	CertConfigs    []*CertificateConfig
}

func (c *CertChainConfig) GetMetaSecrets() []*corev1.Secret {
	secrets := make([]*corev1.Secret, 0, len(c.CertConfigs)+1)
	secrets = append(secrets, c.CAConfig.GetMetaSecret())

	for _, cc := range c.CertConfigs {
		secrets = append(secrets, cc.GetMetaSecret())
	}

	return secrets
}

func (c *CertChainConfig) GetMetaConfigMaps() []*corev1.ConfigMap {
	return []*corev1.ConfigMap{
		c.CABundleConfig.GetMetaConfigMap(),
	}
}

type CertChainConfigs []*CertChainConfig

func (configs CertChainConfigs) GetMetaSecrets() []*corev1.Secret {
	secrets := make([]*corev1.Secret, 0, len(configs)*2)

	for _, c := range configs {
		secrets = append(secrets, c.GetMetaSecrets()...)
	}

	return secrets
}

func (configs CertChainConfigs) GetMetaConfigMaps() []*corev1.ConfigMap {
	configMaps := make([]*corev1.ConfigMap, 0, len(configs)*2)

	for _, c := range configs {
		configMaps = append(configMaps, c.GetMetaConfigMaps()...)
	}

	return configMaps
}

type CertificateManager struct {
	keyGetter       ocrypto.KeyGetter
	secretsClient   corev1client.SecretsGetter
	secretLister    corev1listers.SecretLister
	configMapClient corev1client.ConfigMapsGetter
	configMapLister corev1listers.ConfigMapLister
	eventRecorder   record.EventRecorder
}

func NewCertificateManager(
	keyGetter ocrypto.KeyGetter,
	secretsClient corev1client.SecretsGetter,
	secretLister corev1listers.SecretLister,
	configMapClient corev1client.ConfigMapsGetter,
	configMapLister corev1listers.ConfigMapLister,
	eventRecorder record.EventRecorder,
) *CertificateManager {
	return &CertificateManager{
		keyGetter:       keyGetter,
		secretsClient:   secretsClient,
		secretLister:    secretLister,
		configMapClient: configMapClient,
		configMapLister: configMapLister,
		eventRecorder:   eventRecorder,
	}
}

// ManageCertificates creates and manages the lifetime of a certificate chain. All certificates are automatically
// recreated when their desired config changes. Certificates are automatically refreshed when they reach their refresh
// interval, or 80% of their lifetime, whichever comes sooner.
func (cm *CertificateManager) ManageCertificates(ctx context.Context, nowFunc func() time.Time, controller *metav1.ObjectMeta, controllerGVK schema.GroupVersionKind, caConfig *CAConfig, caBundleConfig *CABundleConfig, certConfigs []*CertificateConfig, existingSecrets map[string]*corev1.Secret, existingConfigMaps map[string]*corev1.ConfigMap) error {
	caCertCreatorConfig := &ocrypto.CACertCreatorConfig{
		Subject: pkix.Name{
			CommonName: caConfig.Name,
		},
	}
	caTLSSecret, err := MakeSelfSignedCA(ctx, caConfig.Name, caCertCreatorConfig.ToCreator(), cm.keyGetter, nowFunc, caConfig.Validity, caConfig.Refresh, controller, controllerGVK, existingSecrets[caConfig.Name])
	if err != nil {
		return fmt.Errorf("can't make selfsigned CA %q: %w", caConfig.Name, err)
	}

	caSecret := caTLSSecret.GetSecret()
	caSecret.Annotations = helpers.MergeMaps(caSecret.Annotations, caConfig.Annotations)
	caSecret.Labels = helpers.MergeMaps(caSecret.Labels, caConfig.Labels)

	updatedCASecret, caSecretChanged, err := resourceapply.ApplySecret(ctx, cm.secretsClient, cm.secretLister, cm.eventRecorder, caSecret, resourceapply.ApplyOptions{})
	if err != nil {
		return fmt.Errorf("can't apply secret %q: %w", naming.ObjRef(caSecret), err)
	}
	if caSecretChanged {
		caTLSSecret.Refresh(updatedCASecret)
	}

	caBundleCM, err := caTLSSecret.MakeCABundle(caBundleConfig.Name, controller, controllerGVK, existingConfigMaps[caBundleConfig.Name], nowFunc())
	if err != nil {
		return fmt.Errorf("can't make ca bundle ConfigMap %q: %w", caBundleConfig.Name, err)
	}

	caBundleCM.Annotations = helpers.MergeMaps(caBundleCM.Annotations, caBundleConfig.Annotations)
	caBundleCM.Labels = helpers.MergeMaps(caBundleCM.Labels, caBundleConfig.Labels)

	_, _, err = resourceapply.ApplyConfigMap(ctx, cm.configMapClient, cm.configMapLister, cm.eventRecorder, caBundleCM, resourceapply.ApplyOptions{})
	if err != nil {
		return fmt.Errorf("can't apply ConfigMap %q: %w", naming.ObjRef(caBundleCM), err)
	}

	for _, cc := range certConfigs {
		tlsSecret, err := caTLSSecret.MakeCertificate(ctx, cc.Name, cc.CertCreator, cm.keyGetter, controller, controllerGVK, existingSecrets[cc.Name], cc.Validity, cc.Refresh)
		if err != nil {
			return fmt.Errorf("can't make certificate %q: %w", cc.Name, err)
		}

		secret := tlsSecret.GetSecret()
		secret.Annotations = helpers.MergeMaps(secret.Annotations, cc.Annotations)
		secret.Labels = helpers.MergeMaps(secret.Labels, cc.Labels)

		_, _, err = resourceapply.ApplySecret(ctx, cm.secretsClient, cm.secretLister, cm.eventRecorder, secret, resourceapply.ApplyOptions{})
		if err != nil {
			return fmt.Errorf("can't apply secret %q: %w", naming.ObjRef(secret), err)
		}
	}

	return nil
}

func (cm *CertificateManager) ManageCertificateChain(ctx context.Context, nowFunc func() time.Time, controller *metav1.ObjectMeta, controllerGVK schema.GroupVersionKind, certChainConfig *CertChainConfig, existingSecrets map[string]*corev1.Secret, existingConfigMaps map[string]*corev1.ConfigMap) error {
	return cm.ManageCertificates(ctx, nowFunc, controller, controllerGVK, certChainConfig.CAConfig, certChainConfig.CABundleConfig, certChainConfig.CertConfigs, existingSecrets, existingConfigMaps)
}
