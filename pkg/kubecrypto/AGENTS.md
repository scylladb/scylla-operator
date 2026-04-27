# pkg/kubecrypto

Kubernetes-aware TLS certificate lifecycle manager. Stores certificates and CA bundles in `corev1.Secret` and `corev1.ConfigMap` objects, with automatic creation and rotation. Builds on the lower-level crypto primitives in `pkg/crypto`.

## Package relationship

```
pkg/crypto          — pure crypto: Signer, CertCreator, RSAKeyGetter, PEM encode/decode
pkg/kubecrypto      — k8s layer: maps crypto primitives to Secrets/ConfigMaps, manages lifecycle
```

Use `pkg/kubecrypto` from controllers. Import `pkg/crypto` only when you need to construct `CertCreator` configs or `Signer` types.

## Files

| File | Purpose |
|---|---|
| `certmanager.go` | `CertificateManager`, config types (`CAConfig`, `CertificateConfig`, `CABundleConfig`), `NewCertificateManager` |
| `certs.go` | `makeCertificate`, `extractExistingSecret`, `needsRefresh`, `MakeSelfSignedCA` |
| `tlssecret.go` | `TLSSecret` — wrapper over `corev1.Secret` with cert/key cache and CA bundle creation |
| `signingtlssecret.go` | `SigningTLSSecret` — `TLSSecret` that can act as a CA (`AsCertificateAuthority`) |

## Key types

```go
// pkg/crypto (lower level — used when building config)
type Signer interface { Now() time.Time; GetSubjectKeyID() []byte; SignCertificate(...); VerifyCertificate(...) }
// Implementations: ocrypto.NewSelfSignedSigner(nowFunc), ocrypto.NewCertificateAuthority(cert, key, nowFunc)

type CertCreator interface { MakeCertificateTemplate(now, validity) *x509.Certificate; MakeCertificate(...) }
// Built via config helpers:
ocrypto.CACertCreatorConfig{...}.ToCreator()
ocrypto.ServingCertCreatorConfig{Subject, IPAddresses, DNSNames, ...}.ToCreator()
ocrypto.ClientCertCreatorConfig{...}.ToCreator()

type RSAKeyGetter interface { ... }
// Implementation: ocrypto.NewRSAKeyGenerator(min, max, keySize, delay)

// pkg/kubecrypto (high level)
type TLSSecret        // wraps *corev1.Secret, caches parsed certs/key
type SigningTLSSecret // embeds TLSSecret; .AsCertificateAuthority() → *ocrypto.CertificateAuthority
type CertificateManager
```

## Constructor

```go
cm := kubecrypto.NewCertificateManager(
    keyGetter,        // ocrypto.RSAKeyGetter
    kubeClient.CoreV1(),  // corev1client.SecretsGetter
    secretLister,         // corev1listers.SecretLister
    kubeClient.CoreV1(),  // corev1client.ConfigMapsGetter
    configMapLister,      // corev1listers.ConfigMapLister
    recorder,             // record.EventRecorder
)
```

## Controller integration recipe

```go
// 1. Build config objects
caConfig := &kubecrypto.CAConfig{
    MetaConfig: kubecrypto.MetaConfig{Name: "my-ca", Labels: labels},
    Validity:   10 * 365 * 24 * time.Hour,
    Refresh:    7 * 24 * time.Hour,
}
caBundleConfig := &kubecrypto.CABundleConfig{
    MetaConfig: kubecrypto.MetaConfig{Name: "my-ca-bundle", Labels: labels},
}
certConfigs := []*kubecrypto.CertificateConfig{
    {
        MetaConfig:  kubecrypto.MetaConfig{Name: "my-serving-cert", Labels: labels},
        Validity:    90 * 24 * time.Hour,
        Refresh:     7 * 24 * time.Hour,
        CertCreator: ocrypto.ServingCertCreatorConfig{
            Subject:    pkix.Name{CommonName: "my-service"},
            DNSNames:   []string{"my-service.ns.svc"},
        }.ToCreator(),
    },
}

// 2. Prefetch existing secrets/configmaps using listers (nil if not found)
existingSecrets := map[string]*corev1.Secret{
    caConfig.Name:       getOrNil(secretLister.Secrets(ns).Get(caConfig.Name)),
    certConfigs[0].Name: getOrNil(secretLister.Secrets(ns).Get(certConfigs[0].Name)),
}
existingConfigMaps := map[string]*corev1.ConfigMap{
    caBundleConfig.Name: getOrNil(configMapLister.ConfigMaps(ns).Get(caBundleConfig.Name)),
}

// 3. Call ManageCertificates in every reconcile iteration
err := cm.ManageCertificates(
    ctx,
    time.Now,     // nowFunc
    controllerMeta,    // metav1.ObjectMeta of the owning object (sets ownerRefs)
    controllerGVK,     // schema.GroupVersionKind of the owning object
    caConfig,
    caBundleConfig,
    certConfigs,
    existingSecrets,
    existingConfigMaps,
)
```

`ManageCertificates` is **idempotent** — call it on every reconcile. It uses `resourceapply.ApplySecret` / `ApplyConfigMap` internally; do not apply these Secrets/ConfigMaps yourself.

## Rotation logic (`needsRefresh`)

A certificate is refreshed when **any** of the following is true:
1. Certificate is expired (`now > NotAfter`)
2. Reached 80% of its lifetime
3. `NotBefore + refresh interval` has passed
4. Desired template fields differ from existing certificate
5. Issuer's `SubjectKeyId` changed (CA was rotated)

## CA bundle behavior

`MakeCABundle` combines the current CA cert with any non-expired previous CA certs. The bundle ConfigMap uses key `ca-bundle.crt`.

## Annotations set automatically

`makeCertificate` writes these annotations before applying the Secret (do not set them manually):
- `certificates.internal.scylladb.com/refresh-reason`
- `certificates.internal.scylladb.com/count`
- `certificates.internal.scylladb.com/not-before`
- `certificates.internal.scylladb.com/not-after`
- `certificates.internal.scylladb.com/is-ca`
- `certificates.internal.scylladb.com/issuer`
- `certificates.internal.scylladb.com/key-size-bits`

## Informer wiring

Your controller must watch the Secrets and ConfigMaps managed by `CertificateManager` to react to out-of-band modifications. Because `ManageCertificates` sets `ownerReferences` on created resources, use the standard claim/release pattern in `pkg/controllerhelpers` to enqueue the owner when these resources change.

## Testing

Pass a deterministic `nowFunc` (e.g., `func() time.Time { return fixedTime }`) to control rotation timing in unit tests. Use an `RSAKeyGenerator` stub if you need deterministic key generation.
