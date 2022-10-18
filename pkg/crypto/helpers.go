// Copyright (C) 2022 ScyllaDB

package crypto

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"
	"time"

	"k8s.io/klog/v2"
)

func SignCertificate(template *x509.Certificate, requestKey *rsa.PublicKey, issuer *x509.Certificate, issuerKey *rsa.PrivateKey) (*x509.Certificate, error) {
	if len(template.Subject.CommonName) == 0 && len(template.IPAddresses) == 0 && len(strings.Join(template.DNSNames, "")) == 0 {
		return nil, fmt.Errorf("certificate requires either CommonName, IPAddresses or DNSNames to be set")
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, template, issuer, requestKey, issuerKey)
	if err != nil {
		return nil, fmt.Errorf("can't create certificate: %w", err)
	}

	certs, err := x509.ParseCertificates(derBytes)
	if err != nil {
		return nil, fmt.Errorf("can't parse der encoded certificate: %w", err)
	}
	if len(certs) != 1 {
		return nil, fmt.Errorf("expected to parse 1 certificate from der bytes but %d were present", len(certs))
	}

	return certs[0], nil
}

func EncodeCertificates(certificates ...*x509.Certificate) ([]byte, error) {
	buffer := bytes.Buffer{}
	for _, certificate := range certificates {
		err := pem.Encode(&buffer, &pem.Block{
			Type:  "CERTIFICATE",
			Bytes: certificate.Raw,
		})
		if err != nil {
			return nil, fmt.Errorf("can't pem encode certificate: %w", err)
		}
	}
	return buffer.Bytes(), nil
}

func EncodePrivateKey(key *rsa.PrivateKey) ([]byte, error) {
	buffer := bytes.Buffer{}
	err := pem.Encode(&buffer, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})
	if err != nil {
		return nil, fmt.Errorf("can't pem encode rsa private key: %w", err)
	}

	return buffer.Bytes(), nil
}

func DecodeCertificates(certBytes []byte) ([]*x509.Certificate, error) {
	var certificates []*x509.Certificate
	remainingBytes := certBytes
	for len(remainingBytes) > 0 {
		var block *pem.Block
		block, remainingBytes = pem.Decode(remainingBytes)
		if block == nil {
			break
		}

		certificate, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("can't parse certificate from block type %q: %w", block.Type, err)
		}

		certificates = append(certificates, certificate)
	}

	return certificates, nil
}

func DecodePrivateKey(keyBytes []byte) (*rsa.PrivateKey, error) {
	block, remainingBytes := pem.Decode(keyBytes)
	if block == nil {
		return nil, fmt.Errorf("no private key block found")
	}

	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("can't parse private key from block type %q: %w", block.Type, err)
	}

	var unexpectedBlockTypes []string
	for len(remainingBytes) != 0 {
		block, remainingBytes = pem.Decode(keyBytes)
		if block == nil {
			break
		}

		unexpectedBlockTypes = append(unexpectedBlockTypes, block.Type)
	}

	if len(unexpectedBlockTypes) > 0 {
		klog.ErrorS(
			errors.New("encountered unexpected blocks"),
			"Private key was followed by unexpected blocks",
			"UnexpectedBlockTypes", unexpectedBlockTypes,
		)
	}

	return privateKey, nil
}

func GetTLSCertificatesFromBytes(certBytes, keyBytes []byte) ([]*x509.Certificate, *rsa.PrivateKey, error) {
	certificates, err := DecodeCertificates(certBytes)
	if err != nil {
		return nil, nil, err
	}

	privateKey, err := DecodePrivateKey(keyBytes)
	if err != nil {
		return nil, nil, err
	}

	return certificates, privateKey, nil
}

func FilterOutExpiredCertificates(certs []*x509.Certificate, now time.Time) []*x509.Certificate {
	var res []*x509.Certificate

	for _, cert := range certs {
		if cert.NotAfter.After(now) {
			res = append(res, cert)
		}
	}

	return res
}

func HasCertificate(certs []*x509.Certificate, cert *x509.Certificate) bool {
	for _, c := range certs {
		if cert.Equal(c) {
			return true
		}
	}

	return false
}
func FilterOutDuplicateCertificates(certs []*x509.Certificate) []*x509.Certificate {
	var res []*x509.Certificate

	for _, cert := range certs {
		if HasCertificate(res, cert) {
			continue
		}

		res = append(res, cert)
	}

	return res
}
