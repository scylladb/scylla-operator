// Copyright (C) 2024 ScyllaDB

package crypto

import (
	"context"
	"crypto/elliptic"
	"crypto/x509"
	"testing"
	"time"
)

func TestECDSAKeyGenerator(t *testing.T) {
	t.Parallel()

	testCases := []struct {
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

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gen, err := NewECDSAKeyGenerator(1, 1, tc.curve, 1*time.Second)
			if err != nil {
				t.Fatalf("failed to create ECDSA key generator: %v", err)
			}
			defer gen.Close()

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			done := make(chan struct{})
			go func() {
				gen.Run(ctx)
				close(done)
			}()

			key, err := gen.GetNewKey(ctx)
			if err != nil {
				t.Fatalf("failed to get new ECDSA key: %v", err)
			}

			if key == nil {
				t.Fatal("got nil ECDSA key")
			}

			if key.Curve != tc.curve {
				t.Errorf("expected curve %v, got %v", tc.curve, key.Curve)
			}

			// Verify the key is valid
			if !key.IsOnCurve(key.X, key.Y) {
				t.Error("generated key is not on the curve")
			}

			cancel()
			<-done
		})
	}
}

func TestRSAKeyGenerator(t *testing.T) {
	t.Parallel()

	gen, err := NewRSAKeyGenerator(1, 1, 2048, 1*time.Second)
	if err != nil {
		t.Fatalf("failed to create RSA key generator: %v", err)
	}
	defer gen.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		gen.Run(ctx)
		close(done)
	}()

	key, err := gen.GetNewKey(ctx)
	if err != nil {
		t.Fatalf("failed to get new RSA key: %v", err)
	}

	if key == nil {
		t.Fatal("got nil RSA key")
	}

	if key.N.BitLen() != 2048 {
		t.Errorf("expected key size 2048, got %d", key.N.BitLen())
	}

	// Verify the key is valid
	if err := key.Validate(); err != nil {
		t.Errorf("generated RSA key is invalid: %v", err)
	}

	cancel()
	<-done
}

func TestEncodeDecodeECDSAKey(t *testing.T) {
	t.Parallel()

	curves := []elliptic.Curve{
		elliptic.P256(),
		elliptic.P384(),
		elliptic.P521(),
	}

	for _, curve := range curves {
		t.Run(curve.Params().Name, func(t *testing.T) {
			t.Parallel()

			gen, err := NewECDSAKeyGenerator(1, 1, curve, 1*time.Second)
			if err != nil {
				t.Fatalf("failed to create ECDSA key generator: %v", err)
			}
			defer gen.Close()

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			done := make(chan struct{})
			go func() {
				gen.Run(ctx)
				close(done)
			}()

			key, err := gen.GetNewKey(ctx)
			if err != nil {
				t.Fatalf("failed to get new ECDSA key: %v", err)
			}

			// Encode the key
			encoded, err := EncodePrivateKeyECDSA(key)
			if err != nil {
				t.Fatalf("failed to encode ECDSA key: %v", err)
			}

			// Decode the key
			decoded, err := DecodePrivateKeyECDSA(encoded)
			if err != nil {
				t.Fatalf("failed to decode ECDSA key: %v", err)
			}

			// Verify the decoded key matches the original
			if !decoded.Equal(key) {
				t.Error("decoded key does not match original")
			}

			cancel()
			<-done
		})
	}
}

func TestEncodeDecodeRSAKey(t *testing.T) {
	t.Parallel()

	gen, err := NewRSAKeyGenerator(1, 1, 2048, 1*time.Second)
	if err != nil {
		t.Fatalf("failed to create RSA key generator: %v", err)
	}
	defer gen.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		gen.Run(ctx)
		close(done)
	}()

	key, err := gen.GetNewKey(ctx)
	if err != nil {
		t.Fatalf("failed to get new RSA key: %v", err)
	}

	// Encode the key
	encoded, err := EncodePrivateKeyRSA(key)
	if err != nil {
		t.Fatalf("failed to encode RSA key: %v", err)
	}

	// Decode the key
	decoded, err := DecodePrivateKeyRSA(encoded)
	if err != nil {
		t.Fatalf("failed to decode RSA key: %v", err)
	}

	// Verify the decoded key matches the original
	if !decoded.Equal(key) {
		t.Error("decoded key does not match original")
	}

	cancel()
	<-done
}

func TestSignatureAlgorithmSelection(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name              string
		generateKey       func() (any, error)
		expectedAlgorithm x509.SignatureAlgorithm
	}{
		{
			name: "RSA key",
			generateKey: func() (any, error) {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()

				gen, err := NewRSAKeyGenerator(1, 1, 2048, 1*time.Second)
				if err != nil {
					return nil, err
				}
				defer gen.Close()

				done := make(chan struct{})
				go func() {
					gen.Run(ctx)
					close(done)
				}()

				key, err := gen.GetNewKey(ctx)
				if err != nil {
					return nil, err
				}

				cancel()
				<-done
				return key, nil
			},
			expectedAlgorithm: x509.SHA512WithRSA,
		},
		{
			name: "ECDSA key",
			generateKey: func() (any, error) {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()

				gen, err := NewECDSAKeyGenerator(1, 1, elliptic.P384(), 1*time.Second)
				if err != nil {
					return nil, err
				}
				defer gen.Close()

				done := make(chan struct{})
				go func() {
					gen.Run(ctx)
					close(done)
				}()

				key, err := gen.GetNewKey(ctx)
				if err != nil {
					return nil, err
				}

				cancel()
				<-done
				return key, nil
			},
			expectedAlgorithm: x509.ECDSAWithSHA384,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			key, err := tc.generateKey()
			if err != nil {
				t.Fatalf("failed to generate key: %v", err)
			}

			algorithm := getSignatureAlgorithm(key)
			if algorithm != tc.expectedAlgorithm {
				t.Errorf("expected signature algorithm %v, got %v", tc.expectedAlgorithm, algorithm)
			}
		})
	}
}

func TestKeyTypeCompatibility(t *testing.T) {
	t.Parallel()

	// Test that RSA keys work with RSAKeyGetter interface
	t.Run("RSA compatibility", func(t *testing.T) {
		t.Parallel()

		var getter RSAKeyGetter
		gen, err := NewRSAKeyGenerator(1, 1, 2048, 1*time.Second)
		if err != nil {
			t.Fatalf("failed to create RSA key generator: %v", err)
		}
		defer gen.Close()

		getter = gen
		if getter == nil {
			t.Fatal("RSAKeyGenerator does not implement RSAKeyGetter")
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		done := make(chan struct{})
		go func() {
			gen.Run(ctx)
			close(done)
		}()

		key, err := getter.GetNewKey(ctx)
		if err != nil {
			t.Fatalf("failed to get key: %v", err)
		}

		if key == nil {
			t.Fatal("got nil key")
		}

		cancel()
		<-done
	})

	// Test that ECDSA keys work with ECDSAKeyGetter interface
	t.Run("ECDSA compatibility", func(t *testing.T) {
		t.Parallel()

		var getter ECDSAKeyGetter
		gen, err := NewECDSAKeyGenerator(1, 1, elliptic.P384(), 1*time.Second)
		if err != nil {
			t.Fatalf("failed to create ECDSA key generator: %v", err)
		}
		defer gen.Close()

		getter = gen
		if getter == nil {
			t.Fatal("ECDSAKeyGenerator does not implement ECDSAKeyGetter")
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		done := make(chan struct{})
		go func() {
			gen.Run(ctx)
			close(done)
		}()

		key, err := getter.GetNewKey(ctx)
		if err != nil {
			t.Fatalf("failed to get key: %v", err)
		}

		if key == nil {
			t.Fatal("got nil key")
		}

		cancel()
		<-done
	})
}

func BenchmarkRSAKeyGeneration(b *testing.B) {
	gen, err := NewRSAKeyGenerator(10, 20, 2048, 100*time.Millisecond)
	if err != nil {
		b.Fatalf("failed to create RSA key generator: %v", err)
	}
	defer gen.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		gen.Run(ctx)
		close(done)
	}()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := gen.GetNewKey(ctx)
		if err != nil {
			b.Fatalf("failed to get key: %v", err)
		}
	}

	cancel()
	<-done
}

func BenchmarkECDSAKeyGeneration(b *testing.B) {
	gen, err := NewECDSAKeyGenerator(10, 20, elliptic.P384(), 100*time.Millisecond)
	if err != nil {
		b.Fatalf("failed to create ECDSA key generator: %v", err)
	}
	defer gen.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		gen.Run(ctx)
		close(done)
	}()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := gen.GetNewKey(ctx)
		if err != nil {
			b.Fatalf("failed to get key: %v", err)
		}
	}

	cancel()
	<-done
}

func TestRSAKeyGenerator_MinimumKeySize(t *testing.T) {
t.Parallel()

testCases := []struct {
name        string
keySize     int
expectError bool
}{
{
name:        "below minimum (1024)",
keySize:     1024,
expectError: true,
},
{
name:        "at minimum (2048)",
keySize:     2048,
expectError: false,
},
{
name:        "above minimum (4096)",
keySize:     4096,
expectError: false,
},
}

for _, tc := range testCases {
t.Run(tc.name, func(t *testing.T) {
t.Parallel()

gen, err := NewRSAKeyGenerator(1, 1, tc.keySize, 1*time.Second)

if tc.expectError {
if err == nil {
t.Errorf("expected error for key size %d, but got none", tc.keySize)
}
if gen != nil {
t.Errorf("expected nil generator for invalid key size, got %v", gen)
}
} else {
if err != nil {
t.Errorf("unexpected error for valid key size %d: %v", tc.keySize, err)
}
if gen != nil {
gen.Close()
}
}
})
}
}
