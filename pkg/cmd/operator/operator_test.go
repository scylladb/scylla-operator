// Copyright (C) 2024 ScyllaDB

package operator

import (
	"testing"

	"github.com/scylladb/scylla-operator/pkg/crypto"
	"github.com/scylladb/scylla-operator/pkg/genericclioptions"
)

func TestOperatorOptions_Validate_CryptoKeyType(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name        string
		keyType     string
		keySize     int
		expectError bool
	}{
		{
			name:        "valid RSA with 2048 bits",
			keyType:     string(crypto.KeyTypeRSA),
			keySize:     2048,
			expectError: false,
		},
		{
			name:        "valid RSA with 3072 bits",
			keyType:     string(crypto.KeyTypeRSA),
			keySize:     3072,
			expectError: false,
		},
		{
			name:        "valid RSA with 4096 bits",
			keyType:     string(crypto.KeyTypeRSA),
			keySize:     4096,
			expectError: false,
		},
		{
			name:        "invalid RSA with 1024 bits",
			keyType:     string(crypto.KeyTypeRSA),
			keySize:     1024,
			expectError: true,
		},
		{
			name:        "invalid RSA with 2560 bits",
			keyType:     string(crypto.KeyTypeRSA),
			keySize:     2560,
			expectError: true,
		},
		{
			name:        "invalid RSA with 8192 bits",
			keyType:     string(crypto.KeyTypeRSA),
			keySize:     8192,
			expectError: true,
		},
		{
			name:        "valid ECDSA with P-256",
			keyType:     string(crypto.KeyTypeECDSA),
			keySize:     256,
			expectError: false,
		},
		{
			name:        "valid ECDSA with P-384",
			keyType:     string(crypto.KeyTypeECDSA),
			keySize:     384,
			expectError: false,
		},
		{
			name:        "valid ECDSA with P-521",
			keyType:     string(crypto.KeyTypeECDSA),
			keySize:     521,
			expectError: false,
		},
		{
			name:        "invalid ECDSA with 224",
			keyType:     string(crypto.KeyTypeECDSA),
			keySize:     224,
			expectError: true,
		},
		{
			name:        "invalid ECDSA with 512",
			keyType:     string(crypto.KeyTypeECDSA),
			keySize:     512,
			expectError: true,
		},
		{
			name:        "invalid key type",
			keyType:     "INVALID",
			keySize:     2048,
			expectError: true,
		},
		{
			name:        "empty key type",
			keyType:     "",
			keySize:     2048,
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			o := &OperatorOptions{
				ClientConfig:        genericclioptions.ClientConfig{},
				InClusterReflection: genericclioptions.InClusterReflection{},
				LeaderElection:      genericclioptions.LeaderElection{},
				OperatorImage:       "test-image",
				CryptoKeyType:       tc.keyType,
				CryptoKeySize:       tc.keySize,
				CryptoKeyBufferSizeMin: 1,
				CryptoKeyBufferSizeMax: 1,
			}

			err := o.Validate()

			if tc.expectError {
				if err == nil {
					t.Errorf("expected error for keyType=%s keySize=%d, but got none", tc.keyType, tc.keySize)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error for keyType=%s keySize=%d: %v", tc.keyType, tc.keySize, err)
				}
			}
		})
	}
}

func TestOperatorOptions_Validate_CryptoKeyBufferSizes(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name        string
		minSize     int
		maxSize     int
		expectError bool
	}{
		{
			name:        "valid buffer sizes",
			minSize:     10,
			maxSize:     30,
			expectError: false,
		},
		{
			name:        "valid equal buffer sizes",
			minSize:     10,
			maxSize:     10,
			expectError: false,
		},
		{
			name:        "invalid min size zero",
			minSize:     0,
			maxSize:     10,
			expectError: true,
		},
		{
			name:        "invalid min size negative",
			minSize:     -1,
			maxSize:     10,
			expectError: true,
		},
		{
			name:        "invalid max size zero",
			minSize:     10,
			maxSize:     0,
			expectError: true,
		},
		{
			name:        "invalid max less than min",
			minSize:     30,
			maxSize:     10,
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			o := &OperatorOptions{
				ClientConfig:        genericclioptions.ClientConfig{},
				InClusterReflection: genericclioptions.InClusterReflection{},
				LeaderElection:      genericclioptions.LeaderElection{},
				OperatorImage:       "test-image",
				CryptoKeyType:       string(crypto.KeyTypeRSA),
				CryptoKeySize:       4096,
				CryptoKeyBufferSizeMin: tc.minSize,
				CryptoKeyBufferSizeMax: tc.maxSize,
			}

			err := o.Validate()

			if tc.expectError {
				if err == nil {
					t.Errorf("expected error for minSize=%d maxSize=%d, but got none", tc.minSize, tc.maxSize)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error for minSize=%d maxSize=%d: %v", tc.minSize, tc.maxSize, err)
				}
			}
		})
	}
}

func TestOperatorOptions_Validate_OperatorImage(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name          string
		operatorImage string
		expectError   bool
	}{
		{
			name:          "valid operator image",
			operatorImage: "scylladb/scylla-operator:latest",
			expectError:   false,
		},
		{
			name:          "empty operator image",
			operatorImage: "",
			expectError:   true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			o := &OperatorOptions{
				ClientConfig:        genericclioptions.ClientConfig{},
				InClusterReflection: genericclioptions.InClusterReflection{},
				LeaderElection:      genericclioptions.LeaderElection{},
				OperatorImage:       tc.operatorImage,
				CryptoKeyType:       string(crypto.KeyTypeRSA),
				CryptoKeySize:       4096,
				CryptoKeyBufferSizeMin: 1,
				CryptoKeyBufferSizeMax: 1,
			}

			err := o.Validate()

			if tc.expectError {
				if err == nil {
					t.Errorf("expected error for operatorImage=%q, but got none", tc.operatorImage)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error for operatorImage=%q: %v", tc.operatorImage, err)
				}
			}
		})
	}
}

func TestOperatorOptions_Validate_CQLSIngressPort(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name        string
		port        int
		expectError bool
	}{
		{
			name:        "valid port 0",
			port:        0,
			expectError: false,
		},
		{
			name:        "valid port 9042",
			port:        9042,
			expectError: false,
		},
		{
			name:        "valid port 65535",
			port:        65535,
			expectError: false,
		},
		{
			name:        "invalid port -1",
			port:        -1,
			expectError: true,
		},
		{
			name:        "invalid port 65536",
			port:        65536,
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			o := &OperatorOptions{
				ClientConfig:        genericclioptions.ClientConfig{},
				InClusterReflection: genericclioptions.InClusterReflection{},
				LeaderElection:      genericclioptions.LeaderElection{},
				OperatorImage:       "test-image",
				CryptoKeyType:       string(crypto.KeyTypeRSA),
				CryptoKeySize:       4096,
				CryptoKeyBufferSizeMin: 1,
				CryptoKeyBufferSizeMax: 1,
				CQLSIngressPort:     tc.port,
			}

			err := o.Validate()

			if tc.expectError {
				if err == nil {
					t.Errorf("expected error for port=%d, but got none", tc.port)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error for port=%d: %v", tc.port, err)
				}
			}
		})
	}
}

func TestNewOperatorOptions_Defaults(t *testing.T) {
	t.Parallel()

	streams := genericclioptions.IOStreams{}
	o := NewOperatorOptions(streams)

	// Verify backward compatibility defaults
	if o.CryptoKeyType != string(crypto.KeyTypeRSA) {
		t.Errorf("expected default CryptoKeyType to be %q, got %q", crypto.KeyTypeRSA, o.CryptoKeyType)
	}

	if o.CryptoKeySize != 4096 {
		t.Errorf("expected default CryptoKeySize to be 4096, got %d", o.CryptoKeySize)
	}

	if o.CryptoKeyBufferSizeMin != 10 {
		t.Errorf("expected default CryptoKeyBufferSizeMin to be 10, got %d", o.CryptoKeyBufferSizeMin)
	}

	if o.CryptoKeyBufferSizeMax != 30 {
		t.Errorf("expected default CryptoKeyBufferSizeMax to be 30, got %d", o.CryptoKeyBufferSizeMax)
	}

	if o.ConcurrentSyncs != 50 {
		t.Errorf("expected default ConcurrentSyncs to be 50, got %d", o.ConcurrentSyncs)
	}
}
