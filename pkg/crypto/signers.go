// Copyright (C) 2022 ScyllaDB

package crypto

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"fmt"
	"reflect"
	"time"
)

type Signer interface {
	Now() time.Time
	GetPublicKey() *rsa.PublicKey
	SignCertificate(template *x509.Certificate, requestKey *rsa.PublicKey) (*x509.Certificate, error)
	VerifyCertificate(cert *x509.Certificate) error
}

type SelfSignedSigner struct {
	privateKey *rsa.PrivateKey
	nowFunc    func() time.Time
}

var _ Signer = &SelfSignedSigner{}

func NewSelfSignedSigner(nowFunc func() time.Time) *SelfSignedSigner {
	return &SelfSignedSigner{
		nowFunc: nowFunc,
	}
}
func NewSelfSignedSignerWithKey(nowFunc func() time.Time, privateKey *rsa.PrivateKey) *SelfSignedSigner {
	return &SelfSignedSigner{
		privateKey: privateKey,
		nowFunc:    nowFunc,
	}
}

func (s *SelfSignedSigner) Now() time.Time {
	return s.nowFunc()
}

func (s *SelfSignedSigner) GetPublicKey() *rsa.PublicKey {
	return nil
}

func (s *SelfSignedSigner) SignCertificate(template *x509.Certificate, requestKey *rsa.PublicKey) (*x509.Certificate, error) {
	// Make sure the self-signed signer was initialized for this publicKey.
	if !reflect.DeepEqual(requestKey, s.privateKey.Public()) {
		return nil, fmt.Errorf("self-signed signer: public key mismatch")
	}

	var err error
	template.SerialNumber, err = rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, fmt.Errorf("can't generate certificate serial number: %w", err)
	}

	cert, err := SignCertificate(template, requestKey, template, s.privateKey)
	if err != nil {
		return nil, fmt.Errorf("can't self-sign certificate for %q: %w", template.Subject, err)
	}

	return cert, nil
}

func (s *SelfSignedSigner) VerifyCertificate(cert *x509.Certificate) error {
	return cert.CheckSignatureFrom(cert)
}

type CertificateAuthority struct {
	cert       *x509.Certificate
	privateKey *rsa.PrivateKey
	nowFunc    func() time.Time
}

var _ Signer = &CertificateAuthority{}

func NewCertificateAuthority(cert *x509.Certificate, key *rsa.PrivateKey, nowFunc func() time.Time) (*CertificateAuthority, error) {
	if cert.IsCA == false {
		return nil, fmt.Errorf("certificate isn't a CA")
	}

	return &CertificateAuthority{
		cert:       cert,
		privateKey: key,
		nowFunc:    nowFunc,
	}, nil
}

func (ca *CertificateAuthority) Now() time.Time {
	return ca.nowFunc()
}

func (ca *CertificateAuthority) GetPublicKey() *rsa.PublicKey {
	return &ca.privateKey.PublicKey
}

func (ca *CertificateAuthority) SignCertificate(template *x509.Certificate, requestKey *rsa.PublicKey) (*x509.Certificate, error) {
	var err error
	template.SerialNumber, err = rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, fmt.Errorf("can't generate certificate serial number: %w", err)
	}

	cert, err := SignCertificate(template, requestKey, ca.cert, ca.privateKey)
	if err != nil {
		return nil, fmt.Errorf("can't sign certificate for %q: %w", template.Subject, err)
	}

	return cert, nil
}

func (ca *CertificateAuthority) VerifyCertificate(cert *x509.Certificate) error {
	return cert.CheckSignatureFrom(ca.cert)
}

func (ca *CertificateAuthority) GetCert() *x509.Certificate {
	return ca.cert
}
