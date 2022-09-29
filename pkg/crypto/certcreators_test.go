package crypto

import (
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"reflect"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/scylladb/scylla-operator/pkg/crypto/testfiles"
	"github.com/scylladb/scylla-operator/pkg/helpers"
)

type certificateWithoutEqual x509.Certificate

func TestX509CertCreator_MakeCertificate(t *testing.T) {
	t.Parallel()

	now := func() time.Time {
		return time.Date(2021, 02, 01, 00, 00, 00, 00, time.UTC)
	}
	ca, err := NewCertificateAuthority(
		helpers.Must(DecodeCertificates(testfiles.AlphaCACertBytes))[0],
		helpers.Must(DecodePrivateKey(testfiles.AlphaCAKeyBytes)),
		now,
	)
	if err != nil {
		t.Fatalf("can't create CA: %v", err)
	}

	tt := []struct {
		name         string
		certCreator  *X509CertCreator
		lifetime     time.Duration
		expectedCert *x509.Certificate
		expectedErr  error
	}{
		{
			name: "basic fields work",
			certCreator: &X509CertCreator{
				Subject: pkix.Name{
					CommonName:   "deprecated-foo",
					Organization: []string{"ScyllaDB"},
				},
				DNSNames:    []string{"foo", "bar"},
				KeyUsage:    x509.KeyUsageDigitalSignature,
				ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
			},
			lifetime: 1 * time.Hour,
			expectedCert: &x509.Certificate{
				Version: 3,
				Subject: pkix.Name{
					Organization: []string{"ScyllaDB"},
					CommonName:   "deprecated-foo",
				},
				Issuer: pkix.Name{
					CommonName: "test.ca-name",
				},
				NotBefore:             time.Date(2021, 01, 31, 23, 59, 59, 00, time.UTC),
				NotAfter:              time.Date(2021, 02, 01, 01, 00, 00, 00, time.UTC),
				DNSNames:              []string{"foo", "bar"},
				IsCA:                  false,
				BasicConstraintsValid: true,
				AuthorityKeyId:        ca.GetCert().SubjectKeyId,
				SignatureAlgorithm:    x509.SHA512WithRSA,
				PublicKeyAlgorithm:    x509.RSA,
				KeyUsage:              x509.KeyUsageDigitalSignature,
				ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
				MaxPathLen:            -1,
			},
			expectedErr: nil,
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cert, key, err := tc.certCreator.MakeCertificate(ca, tc.lifetime)
			if !reflect.DeepEqual(err, tc.expectedErr) {
				t.Errorf("expected and actual error differ: %s", cmp.Diff(tc.expectedErr, err))
			}

			certPool := x509.NewCertPool()
			certPool.AddCert(ca.GetCert())
			_, err = cert.Verify(x509.VerifyOptions{
				CurrentTime: now(),
				Roots:       certPool,
			})
			if err != nil {
				t.Errorf("can't verify cert: %v", err)
			}

			if key == nil {
				t.Error("got nil key")
			}

			cert.Raw = nil
			cert.RawTBSCertificate = nil
			cert.RawIssuer = nil
			cert.RawSubject = nil
			cert.RawSubjectPublicKeyInfo = nil
			cert.Subject.Names = nil
			cert.Issuer.Names = nil
			cert.Extensions = nil

			if tc.expectedCert != nil {
				if cert.Signature == nil {
					t.Errorf("cert doesn't have a signature")
				}
				cert.Signature = nil

				if cert.SerialNumber == nil {
					t.Errorf("cert doesn't have a serialNumber")
				}
				cert.SerialNumber = nil

				if cert.PublicKey == nil {
					t.Errorf("cert doesn't have a publicKey")
				}
				cert.PublicKey = nil
			}

			if !reflect.DeepEqual(cert, tc.expectedCert) {
				// Certificate.Equal is used by cmp.Diff as an unfortunate optimization.
				// We will wrap the type to lose the method.
				t.Errorf(
					"expected and actual certificate differ: '%s'",
					cmp.Diff(
						(*certificateWithoutEqual)(tc.expectedCert),
						(*certificateWithoutEqual)(cert),
						cmp.Comparer(func(lhs, rhs *big.Int) bool {
							if lhs == nil || rhs == nil {
								return lhs == rhs
							}
							return lhs.Cmp(rhs) == 0
						}),
					),
				)
			}
		})
	}
}
