package crypto

import (
	"testing"
	"time"
)

func TestNewKeyGenerator(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name            string
		cfg             KeyGeneratorConfig
		expectedKeyType KeyType
		expectedErr     string
	}{
		{
			name: "RSA returns RSA key generator",
			cfg: KeyGeneratorConfig{
				Type:          RSAKeyType,
				KeySize:       2048,
				BufferSizeMin: 1,
				BufferSizeMax: 1,
				BufferDelay:   42 * time.Hour,
			},
			expectedKeyType: RSAKeyType,
		},
		{
			name: "ECDSA P-256 returns ECDSA key generator",
			cfg: KeyGeneratorConfig{
				Type:          ECDSAKeyType,
				KeySize:       256,
				BufferSizeMin: 1,
				BufferSizeMax: 1,
				BufferDelay:   42 * time.Hour,
			},
			expectedKeyType: ECDSAKeyType,
		},
		{
			name: "ECDSA P-384 returns ECDSA key generator",
			cfg: KeyGeneratorConfig{
				Type:          ECDSAKeyType,
				KeySize:       384,
				BufferSizeMin: 1,
				BufferSizeMax: 1,
				BufferDelay:   42 * time.Hour,
			},
			expectedKeyType: ECDSAKeyType,
		},
		{
			name: "ECDSA P-521 returns ECDSA key generator",
			cfg: KeyGeneratorConfig{
				Type:          ECDSAKeyType,
				KeySize:       521,
				BufferSizeMin: 1,
				BufferSizeMax: 1,
				BufferDelay:   42 * time.Hour,
			},
			expectedKeyType: ECDSAKeyType,
		},
		{
			name: "unsupported key type returns error",
			cfg: KeyGeneratorConfig{
				Type:          "Ed25519",
				KeySize:       256,
				BufferSizeMin: 1,
				BufferSizeMax: 1,
				BufferDelay:   42 * time.Hour,
			},
			expectedErr: `unsupported key type "Ed25519", supported values are: RSA, ECDSA`,
		},
		{
			name: "ECDSA with unsupported key size returns error",
			cfg: KeyGeneratorConfig{
				Type:          ECDSAKeyType,
				KeySize:       1024,
				BufferSizeMin: 1,
				BufferSizeMax: 1,
				BufferDelay:   42 * time.Hour,
			},
			expectedErr: "can't determine ECDSA curve: unsupported ECDSA key size 1024, supported values are: 256, 384, 521",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			kg, err := NewKeyGenerator(tc.cfg)

			if tc.expectedErr != "" {
				if err == nil {
					t.Fatalf("expected error %q, got nil", tc.expectedErr)
				}
				if err.Error() != tc.expectedErr {
					t.Errorf("expected error %q, got %q", tc.expectedErr, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if kg.GetKeyType() != tc.expectedKeyType {
				t.Errorf("expected key type %v, got %v", tc.expectedKeyType, kg.GetKeyType())
			}
		})
	}
}
