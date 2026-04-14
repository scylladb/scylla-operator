package crypto

import (
	"context"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"reflect"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/scylladb/scylla-operator/pkg/crypto/testfiles"
	"github.com/scylladb/scylla-operator/pkg/helpers"
	testcrypto "github.com/scylladb/scylla-operator/pkg/test/crypto"
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

	selfSignedSigner := NewSelfSignedSigner(now)

	tt := []struct {
		name            string
		certCreator     *X509CertCreator
		keyGeneratorCfg KeyGeneratorConfig
		signer          Signer
		lifetime        time.Duration
		verifyCert      func(t *testing.T, cert *x509.Certificate)
		expectedCert    *x509.Certificate
		expectedErr     error
	}{
		{
			name: "RSA basic fields work",
			certCreator: &X509CertCreator{
				Subject: pkix.Name{
					CommonName:   "deprecated-foo",
					Organization: []string{"ScyllaDB"},
				},
				DNSNames:    []string{"foo", "bar"},
				KeyUsage:    x509.KeyUsageDigitalSignature,
				ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
			},
			keyGeneratorCfg: KeyGeneratorConfig{Type: RSAKeyType, KeySize: 4096, BufferSizeMin: 1, BufferSizeMax: 1, BufferDelay: 42 * time.Hour},
			signer:          ca,
			lifetime:        1 * time.Hour,
			verifyCert: func(t *testing.T, cert *x509.Certificate) {
				certPool := x509.NewCertPool()
				certPool.AddCert(ca.GetCert())
				_, err := cert.Verify(x509.VerifyOptions{
					CurrentTime: now(),
					Roots:       certPool,
				})
				if err != nil {
					t.Errorf("can't verify cert: %v", err)
				}
			},
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
				SignatureAlgorithm:    x509.SHA512WithRSA,
				PublicKeyAlgorithm:    x509.RSA,
				KeyUsage:              x509.KeyUsageDigitalSignature,
				ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
				MaxPathLen:            -1,
			},
			expectedErr: nil,
		},
		{
			name: "ECDSA self-signed CA with P-256",
			certCreator: (&CACertCreatorConfig{
				Subject: pkix.Name{
					CommonName: "test-ecdsa-ca",
				},
			}).ToCreator(),
			keyGeneratorCfg: KeyGeneratorConfig{Type: ECDSAKeyType, KeySize: 256, BufferSizeMin: 1, BufferSizeMax: 1, BufferDelay: 42 * time.Hour},
			signer:          selfSignedSigner,
			lifetime:        24 * time.Hour,
			verifyCert: func(t *testing.T, cert *x509.Certificate) {
				err := cert.CheckSignatureFrom(cert)
				if err != nil {
					t.Errorf("CA certificate signature verification failed: %v", err)
				}
			},
			expectedCert: &x509.Certificate{
				Version: 3,
				Subject: pkix.Name{
					CommonName: "test-ecdsa-ca",
				},
				Issuer: pkix.Name{
					CommonName: "test-ecdsa-ca",
				},
				NotBefore:             time.Date(2021, 01, 31, 23, 59, 59, 00, time.UTC),
				NotAfter:              time.Date(2021, 02, 02, 00, 00, 00, 00, time.UTC),
				IsCA:                  true,
				BasicConstraintsValid: true,
				SignatureAlgorithm:    x509.ECDSAWithSHA256,
				PublicKeyAlgorithm:    x509.ECDSA,
				KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
				MaxPathLen:            -1,
				MaxPathLenZero:        false,
			},
			expectedErr: nil,
		},
		{
			name: "ECDSA serving cert with P-384",
			certCreator: (&ServingCertCreatorConfig{
				Subject: pkix.Name{
					CommonName: "test-ecdsa-serving",
				},
				DNSNames: []string{"localhost"},
			}).ToCreator(),
			keyGeneratorCfg: KeyGeneratorConfig{Type: ECDSAKeyType, KeySize: 384, BufferSizeMin: 1, BufferSizeMax: 1, BufferDelay: 42 * time.Hour},
			signer:          selfSignedSigner,
			lifetime:        1 * time.Hour,
			verifyCert: func(t *testing.T, cert *x509.Certificate) {
				// Non-CA self-signed certs cannot verify themselves via CheckSignatureFrom
				// because x509 rejects non-CA issuers. Verify the signature directly instead.
				err := cert.CheckSignature(cert.SignatureAlgorithm, cert.RawTBSCertificate, cert.Signature)
				if err != nil {
					t.Errorf("certificate signature verification failed: %v", err)
				}
			},
			expectedCert: &x509.Certificate{
				Version: 3,
				Subject: pkix.Name{
					CommonName: "test-ecdsa-serving",
				},
				Issuer: pkix.Name{
					CommonName: "test-ecdsa-serving",
				},
				NotBefore:             time.Date(2021, 01, 31, 23, 59, 59, 00, time.UTC),
				NotAfter:              time.Date(2021, 02, 01, 01, 00, 00, 00, time.UTC),
				DNSNames:              []string{"localhost"},
				IsCA:                  false,
				BasicConstraintsValid: true,
				SignatureAlgorithm:    x509.ECDSAWithSHA384,
				PublicKeyAlgorithm:    x509.ECDSA,
				KeyUsage:              x509.KeyUsageDigitalSignature,
				MaxPathLen:            -1,
			},
			expectedErr: nil,
		},
		{
			name: "ECDSA client cert with P-521",
			certCreator: (&ClientCertCreatorConfig{
				Subject: pkix.Name{
					CommonName: "test-ecdsa-client",
				},
				DNSNames: []string{"client.local"},
			}).ToCreator(),
			keyGeneratorCfg: KeyGeneratorConfig{Type: ECDSAKeyType, KeySize: 521, BufferSizeMin: 1, BufferSizeMax: 1, BufferDelay: 42 * time.Hour},
			signer:          selfSignedSigner,
			lifetime:        2 * time.Hour,
			verifyCert: func(t *testing.T, cert *x509.Certificate) {
				// Non-CA self-signed certs cannot verify themselves via CheckSignatureFrom
				// because x509 rejects non-CA issuers. Verify the signature directly instead.
				err := cert.CheckSignature(cert.SignatureAlgorithm, cert.RawTBSCertificate, cert.Signature)
				if err != nil {
					t.Errorf("certificate signature verification failed: %v", err)
				}
			},
			expectedCert: &x509.Certificate{
				Version: 3,
				Subject: pkix.Name{
					CommonName: "test-ecdsa-client",
				},
				Issuer: pkix.Name{
					CommonName: "test-ecdsa-client",
				},
				NotBefore:             time.Date(2021, 01, 31, 23, 59, 59, 00, time.UTC),
				NotAfter:              time.Date(2021, 02, 01, 02, 00, 00, 00, time.UTC),
				DNSNames:              []string{"client.local"},
				IsCA:                  false,
				BasicConstraintsValid: true,
				SignatureAlgorithm:    x509.ECDSAWithSHA512,
				PublicKeyAlgorithm:    x509.ECDSA,
				KeyUsage:              x509.KeyUsageDigitalSignature,
				ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
				MaxPathLen:            -1,
			},
			expectedErr: nil,
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			kg, err := NewKeyGenerator(tc.keyGeneratorCfg)
			if err != nil {
				t.Fatalf("can't create key generator: %v", err)
			}
			testcrypto.StartKeyGenerator(t, kg)

			ctx, ctxCancel := context.WithCancel(context.Background())
			defer ctxCancel()

			cert, key, err := tc.certCreator.MakeCertificate(ctx, kg, tc.signer, tc.lifetime)
			if !reflect.DeepEqual(err, tc.expectedErr) {
				t.Errorf("expected and actual error differ: %s", cmp.Diff(tc.expectedErr, err))
			}

			if err != nil {
				return
			}

			tc.verifyCert(t, cert)

			if key == nil {
				t.Error("got nil key")
			}

			// Strip non-deterministic/raw fields for comparison.
			cert.Raw = nil
			cert.RawTBSCertificate = nil
			cert.RawIssuer = nil
			cert.RawSubject = nil
			cert.RawSubjectPublicKeyInfo = nil
			cert.Subject.Names = nil
			cert.Issuer.Names = nil
			cert.Extensions = nil
			cert.SubjectKeyId = nil
			cert.AuthorityKeyId = nil

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

func TestAdjustKeyUsageForKeyType(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name     string
		keyUsage x509.KeyUsage
		keyType  KeyType
		expected x509.KeyUsage
	}{
		{
			name:     "RSA keeps KeyEncipherment",
			keyUsage: x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
			keyType:  RSAKeyType,
			expected: x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		},
		{
			name:     "ECDSA strips KeyEncipherment",
			keyUsage: x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
			keyType:  ECDSAKeyType,
			expected: x509.KeyUsageDigitalSignature,
		},
		{
			name:     "ECDSA preserves other usages",
			keyUsage: x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
			keyType:  ECDSAKeyType,
			expected: x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		},
		{
			name:     "ECDSA no-op when KeyEncipherment not set",
			keyUsage: x509.KeyUsageDigitalSignature,
			keyType:  ECDSAKeyType,
			expected: x509.KeyUsageDigitalSignature,
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result := AdjustKeyUsageForKeyType(tc.keyUsage, tc.keyType)
			if result != tc.expected {
				t.Errorf("expected %v, got %v", tc.expected, result)
			}
		})
	}
}
