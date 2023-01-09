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

type CAConfig struct {
	MetaConfig
	Validity time.Duration
	Refresh  time.Duration
}

type CABundleConfig struct {
	MetaConfig
}

type CertificateConfig struct {
	MetaConfig
	Validity    time.Duration
	Refresh     time.Duration
	CertCreator ocrypto.CertCreator
}

type CertificateManager struct {
	secretsClient   corev1client.SecretsGetter
	secretLister    corev1listers.SecretLister
	configMapClient corev1client.ConfigMapsGetter
	configMapLister corev1listers.ConfigMapLister
	eventRecorder   record.EventRecorder
}

func NewCertificateManager(secretsClient corev1client.SecretsGetter,
	secretLister corev1listers.SecretLister,
	configMapClient corev1client.ConfigMapsGetter,
	configMapLister corev1listers.ConfigMapLister,
	eventRecorder record.EventRecorder,
) *CertificateManager {
	return &CertificateManager{
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
	caTLSSecret, err := MakeSelfSignedCA(caConfig.Name, caCertCreatorConfig.ToCreator(), nowFunc, caConfig.Validity, caConfig.Refresh, controller, controllerGVK, existingSecrets[caConfig.Name])
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
		tlsSecret, err := caTLSSecret.MakeCertificate(cc.Name, cc.CertCreator, controller, controllerGVK, existingSecrets[cc.Name], cc.Validity, cc.Refresh)
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
