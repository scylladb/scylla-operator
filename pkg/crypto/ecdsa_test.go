package crypto

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/x509"
	"testing"
	"time"

	testcrypto "github.com/scylladb/scylla-operator/pkg/test/crypto"
)

func TestECDSAKeyGenerator(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name  string
		curve elliptic.Curve
	}{
		{
			name:  "P-256",
			curve: elliptic.P256(),
		},
		{
			name:  "P-384",
			curve: elliptic.P384(),
		},
		{
			name:  "P-521",
			curve: elliptic.P521(),
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gen, err := NewECDSAKeyGenerator(1, 1, tc.curve, 42*time.Hour)
			if err != nil {
				t.Fatalf("can't create ECDSA key generator: %v", err)
			}
			testcrypto.StartKeyGenerator(t, gen)

			key, err := gen.GetNewKey(t.Context())
			if err != nil {
				t.Fatalf("can't get ECDSA key: %v", err)
			}

			ecKey, ok := key.(*ecdsa.PrivateKey)
			if !ok {
				t.Fatalf("expected *ecdsa.PrivateKey, got %T", key)
			}

			if ecKey.Curve != tc.curve {
				t.Errorf("expected curve %v, got %v", tc.curve.Params().Name, ecKey.Curve.Params().Name)
			}
		})
	}
}

func TestEncodeDecodePrivateKey_ECDSA(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name  string
		curve elliptic.Curve
	}{
		{
			name:  "P-256",
			curve: elliptic.P256(),
		},
		{
			name:  "P-384",
			curve: elliptic.P384(),
		},
		{
			name:  "P-521",
			curve: elliptic.P521(),
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gen, err := NewECDSAKeyGenerator(1, 1, tc.curve, 42*time.Hour)
			if err != nil {
				t.Fatalf("can't create ECDSA key generator: %v", err)
			}
			testcrypto.StartKeyGenerator(t, gen)

			key, err := gen.GetNewKey(t.Context())
			if err != nil {
				t.Fatalf("can't get ECDSA key: %v", err)
			}

			encoded, err := EncodePrivateKey(key)
			if err != nil {
				t.Fatalf("can't encode ECDSA key: %v", err)
			}

			decoded, err := DecodePrivateKey(encoded)
			if err != nil {
				t.Fatalf("can't decode ECDSA key: %v", err)
			}

			decodedEC, ok := decoded.(*ecdsa.PrivateKey)
			if !ok {
				t.Fatalf("expected *ecdsa.PrivateKey, got %T", decoded)
			}

			originalEC := key.(*ecdsa.PrivateKey)
			if !originalEC.Equal(decodedEC) {
				t.Errorf("decoded ECDSA key does not match original")
			}
		})
	}
}

func TestSignatureAlgorithmForKey_ECDSA(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name     string
		curve    elliptic.Curve
		expected x509.SignatureAlgorithm
	}{
		{
			name:     "P-256 returns ECDSAWithSHA256",
			curve:    elliptic.P256(),
			expected: x509.ECDSAWithSHA256,
		},
		{
			name:     "P-384 returns ECDSAWithSHA384",
			curve:    elliptic.P384(),
			expected: x509.ECDSAWithSHA384,
		},
		{
			name:     "P-521 returns ECDSAWithSHA512",
			curve:    elliptic.P521(),
			expected: x509.ECDSAWithSHA512,
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gen, err := NewECDSAKeyGenerator(1, 1, tc.curve, 42*time.Hour)
			if err != nil {
				t.Fatalf("can't create ECDSA key generator: %v", err)
			}
			testcrypto.StartKeyGenerator(t, gen)

			key, err := gen.GetNewKey(t.Context())
			if err != nil {
				t.Fatalf("can't get ECDSA key: %v", err)
			}

			alg, err := SignatureAlgorithmForKey(key)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if alg != tc.expected {
				t.Errorf("expected %v, got %v", tc.expected, alg)
			}
		})
	}
}
