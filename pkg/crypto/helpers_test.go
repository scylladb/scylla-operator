package crypto

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"strings"
	"testing"
)

func TestDecodePrivateKey(t *testing.T) {
	t.Parallel()

	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("can't generate RSA key: %v", err)
	}
	rsaPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(rsaKey),
	})

	ecKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("can't generate ECDSA key: %v", err)
	}
	ecDER, err := x509.MarshalECPrivateKey(ecKey)
	if err != nil {
		t.Fatalf("can't marshal ECDSA key: %v", err)
	}
	ecPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: ecDER,
	})

	tt := []struct {
		name        string
		input       []byte
		expectedErr string
		checkKey    func(t *testing.T, key interface{})
	}{
		{
			name:  "valid RSA key",
			input: rsaPEM,
			checkKey: func(t *testing.T, key interface{}) {
				t.Helper()
				k, ok := key.(*rsa.PrivateKey)
				if !ok {
					t.Fatalf("expected *rsa.PrivateKey, got %T", key)
				}
				if !rsaKey.Equal(k) {
					t.Error("decoded RSA key does not match original")
				}
			},
		},
		{
			name:  "valid EC key",
			input: ecPEM,
			checkKey: func(t *testing.T, key interface{}) {
				t.Helper()
				k, ok := key.(*ecdsa.PrivateKey)
				if !ok {
					t.Fatalf("expected *ecdsa.PrivateKey, got %T", key)
				}
				if !ecKey.Equal(k) {
					t.Error("decoded ECDSA key does not match original")
				}
			},
		},
		{
			name:        "unsupported PEM block type",
			input:       pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: []byte("bogus")}),
			expectedErr: `unsupported PEM block type "PRIVATE KEY"`,
		},
		{
			name:        "no PEM block",
			input:       []byte("not a pem"),
			expectedErr: "no private key block found",
		},
		{
			name:        "empty input",
			input:       []byte{},
			expectedErr: "no private key block found",
		},
		{
			name:        "nil input",
			input:       nil,
			expectedErr: "no private key block found",
		},
		{
			name:        "corrupted RSA key bytes",
			input:       pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: []byte("corrupted")}),
			expectedErr: `can't parse RSA private key from block type "RSA PRIVATE KEY"`,
		},
		{
			name:        "corrupted EC key bytes",
			input:       pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: []byte("corrupted")}),
			expectedErr: `can't parse EC private key from block type "EC PRIVATE KEY"`,
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			key, err := DecodePrivateKey(tc.input)
			if len(tc.expectedErr) > 0 {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tc.expectedErr)
				}
				if got := err.Error(); !strings.Contains(got, tc.expectedErr) {
					t.Fatalf("expected error containing %q, got %q", tc.expectedErr, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.checkKey != nil {
				tc.checkKey(t, key)
			}
		})
	}
}
