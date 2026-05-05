// Copyright (C) 2022 ScyllaDB

package crypto

import (
	"context"
	"crypto"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"net"
	"time"
)

type CertCreator interface {
	MakeCertificateTemplate(now time.Time, validity time.Duration, keyType KeyType) *x509.Certificate
	MakeCertificate(ctx context.Context, keyGetter KeyGenerator, signer Signer, validity time.Duration) (*x509.Certificate, crypto.Signer, error)
}

type X509CertCreator struct {
	Subject     pkix.Name
	IPAddresses []net.IP
	DNSNames    []string
	KeyUsage    x509.KeyUsage
	ExtKeyUsage []x509.ExtKeyUsage
	IsCA        bool
}

var _ CertCreator = &X509CertCreator{}

func (c *X509CertCreator) MakeCertificateTemplate(now time.Time, validity time.Duration, keyType KeyType) *x509.Certificate {
	return &x509.Certificate{
		Subject:               c.Subject,
		IPAddresses:           c.IPAddresses,
		DNSNames:              c.DNSNames,
		IsCA:                  c.IsCA,
		KeyUsage:              AdjustKeyUsageForKeyType(c.KeyUsage, keyType),
		ExtKeyUsage:           c.ExtKeyUsage,
		NotBefore:             now.Add(-1 * time.Second),
		NotAfter:              now.Add(validity),
		BasicConstraintsValid: true,
	}
}

func (c *X509CertCreator) MakeCertificate(ctx context.Context, keyGetter KeyGenerator, signer Signer, validity time.Duration) (*x509.Certificate, crypto.Signer, error) {
	privateKey, err := keyGetter.GetNewKey(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("can't get generated key: %w", err)
	}

	selfSignedSigner, ok := signer.(*SelfSignedSigner)
	if ok {
		signer = NewSelfSignedSignerWithKey(selfSignedSigner.nowFunc, privateKey)
	}

	template := c.MakeCertificateTemplate(signer.Now(), validity, keyGetter.GetKeyType())

	sigAlg, err := SignatureAlgorithmForKey(privateKey)
	if err != nil {
		return nil, nil, fmt.Errorf("can't determine signature algorithm: %w", err)
	}
	template.SignatureAlgorithm = sigAlg
	cert, err := signer.SignCertificate(template, privateKey.Public())
	if err != nil {
		return nil, nil, err
	}

	return cert, privateKey, nil
}

// AdjustKeyUsageForKeyType removes KeyUsageKeyEncipherment for non-RSA key types.
// KeyUsageKeyEncipherment is required for RSA key exchange (TLS_RSA_* cipher suites
// in TLS 1.2), where the client encrypts the pre-master secret with the server's RSA
// public key. ScyllaDB's default TLS priority string (SECURE128) keeps those suites
// active (https://github.com/scylladb/scylladb/blob/8e7ba7efe25cdb18923f0e350b022bef88c62194/db/config.cc#L1755),
// so RSA certificates must retain this bit.
// For ECDSA certificates the bit must be stripped: TLS cipher suites encode both the
// key exchange and authentication algorithms, so TLS_RSA_* suites (which require RSA
// key encipherment) can never be negotiated when the server certificate is ECDSA -
// the TLS stack filters them out before the handshake. The bit would therefore be
// meaningless even if present, and its inclusion produces a non-compliant certificate.
func AdjustKeyUsageForKeyType(keyUsage x509.KeyUsage, keyType KeyType) x509.KeyUsage {
	switch keyType {
	case RSAKeyType:
		return keyUsage
	default:
		return keyUsage &^ x509.KeyUsageKeyEncipherment
	}
}

type CACertCreatorConfig struct {
	Subject pkix.Name
}

func (c *CACertCreatorConfig) ToCreator() *X509CertCreator {
	return &X509CertCreator{
		Subject:  c.Subject,
		IsCA:     true,
		KeyUsage: x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
	}
}

type ClientCertCreatorConfig struct {
	Subject  pkix.Name
	DNSNames []string
}

func (c *ClientCertCreatorConfig) ToCreator() *X509CertCreator {
	return &X509CertCreator{
		Subject:     c.Subject,
		DNSNames:    c.DNSNames,
		KeyUsage:    x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
}

type ServingCertCreatorConfig struct {
	Subject     pkix.Name
	IPAddresses []net.IP
	DNSNames    []string
}

func (c *ServingCertCreatorConfig) ToCreator() *X509CertCreator {
	return &X509CertCreator{
		Subject:     c.Subject,
		IPAddresses: c.IPAddresses,
		DNSNames:    c.DNSNames,
		KeyUsage:    x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
	}
}
