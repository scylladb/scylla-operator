package crypto

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net"
	"reflect"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func Test_extractTemplateFields(t *testing.T) {
	// Make sure the time is fine-grained to test that nanoseconds get truncated.
	now := time.Date(2022, 12, 22, 23, 59, 59, 512345678, time.UTC)

	// Fill in as much info as possible to verify hashing.
	fullTemplate := &x509.Certificate{
		Subject: pkix.Name{
			CommonName:         "Cert Name",
			Country:            []string{"CZ"},
			Organization:       []string{"ScyllaDB"},
			OrganizationalUnit: []string{"Operator"},
		},
		BasicConstraintsValid: true,
		MaxPathLen:            -1,
		IsCA:                  true,
		IPAddresses:           []net.IP{net.IPv4(127, 0, 0, 1)},
		DNSNames:              []string{"localhost"},
		NotBefore:             now.Add(-1 * time.Second),
		NotAfter:              now.Add(24 * time.Hour),
		SerialNumber:          big.NewInt(42),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}

	extractedTemplate := ExtractDesiredFieldsFromTemplate(fullTemplate)

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}

	signer := NewSelfSignedSignerWithKey(time.Now, privateKey)
	cert, err := signer.SignCertificate(fullTemplate, &privateKey.PublicKey)
	if err != nil {
		t.Fatal(err)
	}

	extractedCert := ExtractDesiredFieldsFromTemplate(cert)
	if !reflect.DeepEqual(extractedCert, extractedTemplate) {
		t.Errorf("cert and template hashes differ: %s", cmp.Diff(extractedTemplate, extractedCert))
	}
}
