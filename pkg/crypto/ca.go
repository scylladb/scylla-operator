// Copyright (C) 2022 ScyllaDB

package crypto

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"math/big"
	"time"
)

var (
	serialNumberLimit = new(big.Int).Lsh(big.NewInt(1), 128)
)

func makeCATemplateAndKey(subject pkix.Name, lifetime time.Duration, now time.Time) (*x509.Certificate, *rsa.PrivateKey, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, keySize)
	if err != nil {
		return nil, nil, fmt.Errorf("can't generate RSA key: %w", err)
	}

	// We need to give it a random serial number so when a self-signed CA is rotated
	// the new certificate has a different issuer+serial and can be distinguished.
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, nil, fmt.Errorf("can't generate certificate serial number: %w", err)
	}

	return &x509.Certificate{
		SerialNumber:          serialNumber,
		IsCA:                  true,
		Subject:               subject,
		NotBefore:             now.Add(-1 * time.Second),
		NotAfter:              now.Add(lifetime),
		SignatureAlgorithm:    signatureAlgorithm,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}, privateKey, err
}

type CA struct {
	key           *rsa.PrivateKey
	cert          *x509.Certificate
	previousCerts []*x509.Certificate

	nowFunc func() time.Time
}

func NewCAFromCertKey(cert *x509.Certificate, key *rsa.PrivateKey, nowFunc func() time.Time) (*CA, error) {
	if !cert.IsCA {
		return nil, fmt.Errorf("certificate %q is not CA", cert.Subject)
	}

	return &CA{
		key:           key,
		cert:          cert,
		previousCerts: nil,
		nowFunc:       nowFunc,
	}, nil
}

func (ca *CA) Now() time.Time {
	return ca.nowFunc()
}

func (ca *CA) GetCert() *x509.Certificate {
	return ca.cert
}

func (ca *CA) GetCerts() []*x509.Certificate {
	res := make([]*x509.Certificate, 0, 1+len(ca.previousCerts))
	res = append(res, ca.cert)
	if len(ca.previousCerts) > 0 {
		res = append(res, ca.previousCerts...)
	}
	return res
}

func (ca *CA) GetKey() *rsa.PrivateKey {
	return ca.key
}

func (ca *CA) GetCertKey() (*x509.Certificate, *rsa.PrivateKey) {
	return ca.cert, ca.key
}
func (ca *CA) GetCertsKey() ([]*x509.Certificate, *rsa.PrivateKey) {
	return ca.GetCerts(), ca.key
}

func (ca *CA) GetPEMBytes() ([]byte, []byte, error) {
	certBytes, err := EncodeCertificates(ca.GetCerts()...)
	if err != nil {
		return nil, nil, fmt.Errorf("can't encode ca certificates: %w", err)
	}

	keyBytes, err := EncodePrivateKey(ca.key)
	if err != nil {
		return nil, nil, fmt.Errorf("can't encode ca key: %w", err)
	}

	return certBytes, keyBytes, nil
}

func (ca *CA) SignCertificate(template *x509.Certificate, requestKey *rsa.PublicKey) (*x509.Certificate, error) {
	var err error
	template.SerialNumber, err = rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, fmt.Errorf("can't generate certificate serial number: %w", err)
	}

	cert, err := SignCertificate(template, requestKey, ca.cert, ca.key)
	if err != nil {
		return nil, fmt.Errorf("ca %q can't sign certificate for %q: %w", ca.cert.Subject, template.Subject, err)
	}

	return cert, nil
}
