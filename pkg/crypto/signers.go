// Copyright (C) 2022 ScyllaDB

package crypto

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"fmt"
	"math/big"
	"reflect"
	"time"
)

var (
	serialNumberLimit = new(big.Int).Lsh(big.NewInt(1), 128)
)

type Signer interface {
	Now() time.Time
	GetPublicKey() any
	SignCertificate(template *x509.Certificate, requestKey any) (*x509.Certificate, error)
	VerifyCertificate(cert *x509.Certificate) error
}

type SelfSignedSigner struct {
	privateKey any // *rsa.PrivateKey or *ecdsa.PrivateKey
	nowFunc    func() time.Time
}

var _ Signer = &SelfSignedSigner{}

func NewSelfSignedSigner(nowFunc func() time.Time) *SelfSignedSigner {
	return &SelfSignedSigner{
		nowFunc: nowFunc,
	}
}

func NewSelfSignedSignerWithRSAKey(nowFunc func() time.Time, privateKey *rsa.PrivateKey) *SelfSignedSigner {
	return &SelfSignedSigner{
		privateKey: privateKey,
		nowFunc:    nowFunc,
	}
}

func NewSelfSignedSignerWithECDSAKey(nowFunc func() time.Time, privateKey *ecdsa.PrivateKey) *SelfSignedSigner {
	return &SelfSignedSigner{
		privateKey: privateKey,
		nowFunc:    nowFunc,
	}
}

// NewSelfSignedSignerWithKey is kept for backward compatibility, use NewSelfSignedSignerWithRSAKey instead
func NewSelfSignedSignerWithKey(nowFunc func() time.Time, privateKey *rsa.PrivateKey) *SelfSignedSigner {
	return NewSelfSignedSignerWithRSAKey(nowFunc, privateKey)
}

func (s *SelfSignedSigner) Now() time.Time {
	return s.nowFunc()
}

func (s *SelfSignedSigner) GetPublicKey() any {
	return nil
}

func getPublicKeyFromPrivateKey(privateKey any) any {
	switch key := privateKey.(type) {
	case *rsa.PrivateKey:
		return &key.PublicKey
	case *ecdsa.PrivateKey:
		return &key.PublicKey
	default:
		return nil
	}
}

func (s *SelfSignedSigner) SignCertificate(template *x509.Certificate, requestKey any) (*x509.Certificate, error) {
	// Make sure the self-signed signer was initialized for this publicKey.
	expectedPublicKey := getPublicKeyFromPrivateKey(s.privateKey)
	if !reflect.DeepEqual(requestKey, expectedPublicKey) {
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
	privateKey any // *rsa.PrivateKey or *ecdsa.PrivateKey
	nowFunc    func() time.Time
}

var _ Signer = &CertificateAuthority{}

func NewCertificateAuthorityWithRSAKey(cert *x509.Certificate, key *rsa.PrivateKey, nowFunc func() time.Time) (*CertificateAuthority, error) {
	if cert.IsCA == false {
		return nil, fmt.Errorf("certificate isn't a CA")
	}

	return &CertificateAuthority{
		cert:       cert,
		privateKey: key,
		nowFunc:    nowFunc,
	}, nil
}

func NewCertificateAuthorityWithECDSAKey(cert *x509.Certificate, key *ecdsa.PrivateKey, nowFunc func() time.Time) (*CertificateAuthority, error) {
	if cert.IsCA == false {
		return nil, fmt.Errorf("certificate isn't a CA")
	}

	return &CertificateAuthority{
		cert:       cert,
		privateKey: key,
		nowFunc:    nowFunc,
	}, nil
}

// NewCertificateAuthority is kept for backward compatibility, use NewCertificateAuthorityWithRSAKey instead
func NewCertificateAuthority(cert *x509.Certificate, key *rsa.PrivateKey, nowFunc func() time.Time) (*CertificateAuthority, error) {
	return NewCertificateAuthorityWithRSAKey(cert, key, nowFunc)
}

// NewCertificateAuthorityWithAnyKey creates a CertificateAuthority with either RSA or ECDSA key.
func NewCertificateAuthorityWithAnyKey(cert *x509.Certificate, key any, nowFunc func() time.Time) (*CertificateAuthority, error) {
	switch k := key.(type) {
	case *rsa.PrivateKey:
		return NewCertificateAuthorityWithRSAKey(cert, k, nowFunc)
	case *ecdsa.PrivateKey:
		return NewCertificateAuthorityWithECDSAKey(cert, k, nowFunc)
	default:
		return nil, fmt.Errorf("unsupported key type: %T", key)
	}
}

func (ca *CertificateAuthority) Now() time.Time {
	return ca.nowFunc()
}

func (ca *CertificateAuthority) GetPublicKey() any {
	return getPublicKeyFromPrivateKey(ca.privateKey)
}

func (ca *CertificateAuthority) SignCertificate(template *x509.Certificate, requestKey any) (*x509.Certificate, error) {
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
