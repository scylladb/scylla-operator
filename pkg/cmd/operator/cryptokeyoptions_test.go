package operator

import (
	"testing"
	"time"

	"github.com/scylladb/scylla-operator/pkg/crypto"
	"github.com/spf13/cobra"
)

var defaultTestBufferDelay = 42 * time.Hour

func TestCryptoKeyOptionsValidate(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name        string
		opts        CryptoKeyOptions
		expectedErr string
	}{
		{
			name: "RSA with minimum valid size passes",
			opts: CryptoKeyOptions{
				KeyType:       string(crypto.RSAKeyType),
				KeySize:       rsaKeySizeMin,
				BufferSizeMin: 1,
				BufferSizeMax: 1,
				BufferDelay:   defaultTestBufferDelay,
			},
		},
		{
			name: "RSA with maximum valid size passes",
			opts: CryptoKeyOptions{
				KeyType:       string(crypto.RSAKeyType),
				KeySize:       rsaKeySizeMax,
				BufferSizeMin: 1,
				BufferSizeMax: 1,
				BufferDelay:   defaultTestBufferDelay,
			},
		},
		{
			name: "RSA with too small key size returns error",
			opts: CryptoKeyOptions{
				KeyType:       string(crypto.RSAKeyType),
				KeySize:       rsaKeySizeMin - 1,
				BufferSizeMin: 1,
				BufferSizeMax: 1,
				BufferDelay:   defaultTestBufferDelay,
			},
			expectedErr: "crypto-key-size must not be less than 2048 for RSA keys",
		},
		{
			name: "RSA with too large key size returns error",
			opts: CryptoKeyOptions{
				KeyType:       string(crypto.RSAKeyType),
				KeySize:       rsaKeySizeMax + 1,
				BufferSizeMin: 1,
				BufferSizeMax: 1,
				BufferDelay:   defaultTestBufferDelay,
			},
			expectedErr: "crypto-key-size must not be more than 4096 for RSA keys",
		},
		{
			name: "ECDSA P-256 passes",
			opts: CryptoKeyOptions{
				KeyType:       string(crypto.ECDSAKeyType),
				KeySize:       256,
				BufferSizeMin: 1,
				BufferSizeMax: 1,
				BufferDelay:   defaultTestBufferDelay,
			},
		},
		{
			name: "ECDSA P-384 passes",
			opts: CryptoKeyOptions{
				KeyType:       string(crypto.ECDSAKeyType),
				KeySize:       384,
				BufferSizeMin: 1,
				BufferSizeMax: 1,
				BufferDelay:   defaultTestBufferDelay,
			},
		},
		{
			name: "ECDSA P-521 passes",
			opts: CryptoKeyOptions{
				KeyType:       string(crypto.ECDSAKeyType),
				KeySize:       521,
				BufferSizeMin: 1,
				BufferSizeMax: 1,
				BufferDelay:   defaultTestBufferDelay,
			},
		},
		{
			name: "ECDSA with unsupported key size returns error",
			opts: CryptoKeyOptions{
				KeyType:       string(crypto.ECDSAKeyType),
				KeySize:       1024,
				BufferSizeMin: 1,
				BufferSizeMax: 1,
				BufferDelay:   defaultTestBufferDelay,
			},
			expectedErr: "crypto-key-size must be one of 256, 384, 521, got: 1024",
		},
		{
			name: "unsupported key type returns error",
			opts: CryptoKeyOptions{
				KeyType:       "Ed25519",
				KeySize:       256,
				BufferSizeMin: 1,
				BufferSizeMax: 1,
				BufferDelay:   defaultTestBufferDelay,
			},
			expectedErr: `unsupported crypto-key-type "Ed25519", supported values are: RSA, ECDSA`,
		},
		{
			name: "buffer size min below 1 returns error",
			opts: CryptoKeyOptions{
				KeyType:       string(crypto.RSAKeyType),
				KeySize:       4096,
				BufferSizeMin: 0,
				BufferSizeMax: 1,
				BufferDelay:   defaultTestBufferDelay,
			},
			expectedErr: "crypto-key-buffer-size-min (0) has to be at least 1",
		},
		{
			name: "buffer size max below 1 returns error",
			opts: CryptoKeyOptions{
				KeyType:       string(crypto.RSAKeyType),
				KeySize:       4096,
				BufferSizeMin: 1,
				BufferSizeMax: 0,
				BufferDelay:   defaultTestBufferDelay,
			},
			expectedErr: "[crypto-key-buffer-size-max (0) has to be at least 1, crypto-key-buffer-size-max (0) can't be lower then crypto-key-buffer-size-min (1)]",
		},
		{
			name: "buffer size max lower than min returns error",
			opts: CryptoKeyOptions{
				KeyType:       string(crypto.RSAKeyType),
				KeySize:       4096,
				BufferSizeMin: 10,
				BufferSizeMax: 5,
				BufferDelay:   defaultTestBufferDelay,
			},
			expectedErr: "crypto-key-buffer-size-max (5) can't be lower then crypto-key-buffer-size-min (10)",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := tc.opts.Validate()

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
		})
	}
}

func TestCryptoKeyOptionsComplete(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name         string
		opts         CryptoKeyOptions
		args         []string
		expectedSize int
		expectedMax  int
	}{
		{
			name:         "ECDSA without explicit key size defaults to P-384",
			opts:         NewCryptoKeyOptions(),
			args:         []string{"--crypto-key-type=ECDSA"},
			expectedSize: defaultECDSACurveBitSize,
			expectedMax:  30,
		},
		{
			name:         "ECDSA with explicit key size preserves user value",
			opts:         NewCryptoKeyOptions(),
			args:         []string{"--crypto-key-type=ECDSA", "--crypto-key-size=256"},
			expectedSize: 256,
			expectedMax:  30,
		},
		{
			name:         "RSA key size is not modified",
			opts:         NewCryptoKeyOptions(),
			args:         []string{},
			expectedSize: 4096,
			expectedMax:  30,
		},
		{
			name: "buffer max auto-adjusts to min when not explicitly set and min exceeds max",
			opts: CryptoKeyOptions{
				KeyType:       string(crypto.RSAKeyType),
				KeySize:       4096,
				BufferSizeMin: 20,
				BufferSizeMax: 10,
				BufferDelay:   200 * time.Millisecond,
			},
			args:         []string{},
			expectedSize: 4096,
			expectedMax:  20,
		},
		{
			name: "buffer max is not adjusted when explicitly set even if lower than min",
			opts: CryptoKeyOptions{
				KeyType:       string(crypto.RSAKeyType),
				KeySize:       4096,
				BufferSizeMin: 20,
				BufferSizeMax: 10,
				BufferDelay:   200 * time.Millisecond,
			},
			args:         []string{"--crypto-key-buffer-size-max=10"},
			expectedSize: 4096,
			expectedMax:  10,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cmd := &cobra.Command{Use: "test"}
			tc.opts.AddFlags(cmd)

			err := cmd.ParseFlags(tc.args)
			if err != nil {
				t.Fatalf("unexpected flag parse error: %v", err)
			}

			err = tc.opts.Complete(cmd)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tc.opts.KeySize != tc.expectedSize {
				t.Errorf("expected KeySize %d, got %d", tc.expectedSize, tc.opts.KeySize)
			}

			if tc.opts.BufferSizeMax != tc.expectedMax {
				t.Errorf("expected BufferSizeMax %d, got %d", tc.expectedMax, tc.opts.BufferSizeMax)
			}
		})
	}
}
