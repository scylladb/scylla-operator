package operator

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/scylladb/scylla-operator/pkg/crypto"
	"github.com/spf13/cobra"
)

const defaultTestBufferDelay = 42 * time.Hour

func TestCryptoKeyOptionsValidate(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name        string
		args        []string
		expectedErr string
	}{
		{
			name: "RSA with minimum valid size passes",
			args: []string{"--crypto-key-type=RSA", "--crypto-rsa-key-size=2048"},
		},
		{
			name: "RSA with maximum valid size passes",
			args: []string{"--crypto-key-type=RSA", "--crypto-rsa-key-size=4096"},
		},
		{
			name:        "RSA with too small key size returns error",
			args:        []string{"--crypto-key-type=RSA", "--crypto-rsa-key-size=2047"},
			expectedErr: "crypto-rsa-key-size must not be less than 2048",
		},
		{
			name:        "RSA with too large key size returns error",
			args:        []string{"--crypto-key-type=RSA", "--crypto-rsa-key-size=4097"},
			expectedErr: "crypto-rsa-key-size must not be more than 4096",
		},
		{
			name: "ECDSA P-256 passes",
			args: []string{"--crypto-key-type=ECDSA", "--crypto-ecdsa-key-size=256"},
		},
		{
			name: "ECDSA P-384 passes",
			args: []string{"--crypto-key-type=ECDSA", "--crypto-ecdsa-key-size=384"},
		},
		{
			name: "ECDSA P-521 passes",
			args: []string{"--crypto-key-type=ECDSA", "--crypto-ecdsa-key-size=521"},
		},
		{
			name:        "ECDSA with unsupported key size returns error",
			args:        []string{"--crypto-key-type=ECDSA", "--crypto-ecdsa-key-size=1024"},
			expectedErr: "crypto-ecdsa-key-size must be one of [256 384 521], got: 1024",
		},
		{
			name:        "unsupported key type returns error",
			args:        []string{"--crypto-key-type=Ed25519"},
			expectedErr: `unsupported crypto-key-type "Ed25519", supported values are: RSA, ECDSA`,
		},
		{
			name:        "buffer size min below 1 returns error",
			args:        []string{"--crypto-key-buffer-size-min=0", "--crypto-key-buffer-size-max=1"},
			expectedErr: "crypto-key-buffer-size-min (0) has to be at least 1",
		},
		{
			name:        "buffer size max below 1 returns error",
			args:        []string{"--crypto-key-buffer-size-min=1", "--crypto-key-buffer-size-max=0"},
			expectedErr: "crypto-key-buffer-size-max (0) can't be lower then crypto-key-buffer-size-min (1)",
		},
		{
			name:        "buffer size max lower than min returns error",
			args:        []string{"--crypto-key-buffer-size-min=10", "--crypto-key-buffer-size-max=5"},
			expectedErr: "crypto-key-buffer-size-max (5) can't be lower then crypto-key-buffer-size-min (10)",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cmd := &cobra.Command{Use: "test"}
			opts := DefaultCryptoKeyOptions()
			opts.AddFlags(cmd)

			if err := cmd.ParseFlags(tc.args); err != nil {
				t.Fatalf("unexpected flag parse error: %v", err)
			}

			err := opts.Validate()

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
		name        string
		opts        CryptoKeyOptions
		args        []string
		expectedMax int
	}{
		{
			name: "buffer max auto-adjusts to min when not explicitly set and min exceeds max",
			opts: CryptoKeyOptions{
				KeyType:       string(crypto.RSAKeyType),
				RSAKeySize:    defaultRSAKeySize,
				ECDSAKeySize:  defaultECDSACurveBitSize,
				BufferSizeMin: 20,
				BufferSizeMax: 10,
				BufferDelay:   defaultTestBufferDelay,
			},
			args:        []string{},
			expectedMax: 20,
		},
		{
			name: "buffer max is not adjusted when explicitly set even if lower than min",
			opts: CryptoKeyOptions{
				KeyType:       string(crypto.RSAKeyType),
				RSAKeySize:    defaultRSAKeySize,
				ECDSAKeySize:  defaultECDSACurveBitSize,
				BufferSizeMin: 20,
				BufferSizeMax: 10,
				BufferDelay:   defaultTestBufferDelay,
			},
			args:        []string{"--crypto-key-buffer-size-max=10"},
			expectedMax: 10,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cmd := &cobra.Command{Use: "test"}
			tc.opts.AddFlags(cmd)

			if err := cmd.ParseFlags(tc.args); err != nil {
				t.Fatalf("unexpected flag parse error: %v", err)
			}

			if err := tc.opts.Complete(cmd); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tc.opts.BufferSizeMax != tc.expectedMax {
				t.Errorf("expected BufferSizeMax %d, got %d", tc.expectedMax, tc.opts.BufferSizeMax)
			}
		})
	}
}

func TestCryptoKeyOptionsCryptoKeySizeDeprecation(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name                string
		args                []string
		expectedRSASize     int
		expectedECDSASize   int
		expectedDeprecation string
	}{
		{
			name:                "--crypto-key-size sets RSAKeySize and emits a deprecation notice",
			args:                []string{"--crypto-key-size=2048"},
			expectedRSASize:     2048,
			expectedECDSASize:   defaultECDSACurveBitSize,
			expectedDeprecation: "Flag --crypto-key-size has been deprecated, use --crypto-rsa-key-size or --crypto-ecdsa-key-size instead",
		},
		{
			name:                "--crypto-key-size does not affect ECDSAKeySize even with --crypto-key-type=ECDSA",
			args:                []string{"--crypto-key-type=ECDSA", "--crypto-key-size=2048"},
			expectedRSASize:     2048,
			expectedECDSASize:   defaultECDSACurveBitSize,
			expectedDeprecation: "Flag --crypto-key-size has been deprecated, use --crypto-rsa-key-size or --crypto-ecdsa-key-size instead",
		},
		{
			name:              "without --crypto-key-size no deprecation notice is emitted",
			args:              []string{"--crypto-rsa-key-size=2048"},
			expectedRSASize:   2048,
			expectedECDSASize: defaultECDSACurveBitSize,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			opts := DefaultCryptoKeyOptions()
			cmd := &cobra.Command{Use: "test"}
			opts.AddFlags(cmd)

			stdout := &bytes.Buffer{}
			cmd.SetOut(stdout)

			if err := cmd.ParseFlags(tc.args); err != nil {
				t.Fatalf("unexpected flag parse error: %v", err)
			}

			if opts.RSAKeySize != tc.expectedRSASize {
				t.Errorf("expected RSAKeySize %d, got %d", tc.expectedRSASize, opts.RSAKeySize)
			}

			if opts.ECDSAKeySize != tc.expectedECDSASize {
				t.Errorf("expected ECDSAKeySize %d, got %d", tc.expectedECDSASize, opts.ECDSAKeySize)
			}

			gotDeprecation := strings.TrimSpace(stdout.String())
			if gotDeprecation != tc.expectedDeprecation {
				t.Errorf("expected deprecation notice %q, got %q", tc.expectedDeprecation, gotDeprecation)
			}
		})
	}
}
