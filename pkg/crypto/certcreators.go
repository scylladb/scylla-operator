// Copyright (C) 2022 ScyllaDB

package crypto

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"net"
	"time"
)

type CertCreator interface {
	MakeCertificateTemplate(now time.Time, validity time.Duration) *x509.Certificate
	MakeCertificate(signer Signer, validity time.Duration) (*x509.Certificate, *rsa.PrivateKey, error)
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

func (c *X509CertCreator) MakeCertificateTemplate(now time.Time, validity time.Duration) *x509.Certificate {
	return &x509.Certificate{
		Subject:               c.Subject,
		IPAddresses:           c.IPAddresses,
		DNSNames:              c.DNSNames,
		IsCA:                  c.IsCA,
		KeyUsage:              c.KeyUsage,
		ExtKeyUsage:           c.ExtKeyUsage,
		NotBefore:             now.Add(-1 * time.Second),
		NotAfter:              now.Add(validity),
		SignatureAlgorithm:    signatureAlgorithm,
		BasicConstraintsValid: true,
	}
}

func (c *X509CertCreator) MakeCertificate(signer Signer, validity time.Duration) (*x509.Certificate, *rsa.PrivateKey, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, keySize)
	if err != nil {
		return nil, nil, fmt.Errorf("can't generate key: %w", err)
	}

	selfSignedSigner, ok := signer.(*SelfSignedSigner)
	if ok {
		signer = NewSelfSignedSignerWithKey(selfSignedSigner.nowFunc, privateKey)
	}

	template := c.MakeCertificateTemplate(signer.Now(), validity)

	cert, err := signer.SignCertificate(template, &privateKey.PublicKey)
	if err != nil {
		return nil, nil, err
	}

	return cert, privateKey, nil
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
