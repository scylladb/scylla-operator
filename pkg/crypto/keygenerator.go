// Copyright (C) 2023 ScyllaDB

package crypto

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"time"

	"github.com/scylladb/scylla-operator/pkg/itemgenerator"
)

// KeyType identifies the algorithm family of a cryptographic key.
type KeyType string

const (
	RSAKeyType   KeyType = "RSA"
	ECDSAKeyType KeyType = "ECDSA"
)

type KeyGenerator interface {
	GetNewKey(ctx context.Context) (crypto.Signer, error)
	GetKeyType() KeyType
}

// RunnableKeyGenerator extends KeyGenerator with lifecycle methods for running the
// background key pre-generation loop and closing its buffers.
type RunnableKeyGenerator interface {
	KeyGenerator
	Run(ctx context.Context)
	Close()
}

// KeyGeneratorConfig holds the parameters for constructing a key generator.
type KeyGeneratorConfig struct {
	// Type is the key algorithm, either RSAKeyType or ECDSAKeyType.
	Type KeyType
	// KeySize is the RSA key size in bits, or the ECDSA curve bit size.
	KeySize       int
	BufferSizeMin int
	BufferSizeMax int
	BufferDelay   time.Duration
}

// NewKeyGenerator creates a RunnableKeyGenerator for the given configuration.
func NewKeyGenerator(cfg KeyGeneratorConfig) (RunnableKeyGenerator, error) {
	switch cfg.Type {
	case RSAKeyType:
		return NewRSAKeyGenerator(cfg.BufferSizeMin, cfg.BufferSizeMax, cfg.KeySize, cfg.BufferDelay)

	case ECDSAKeyType:
		curve, err := ecdsaCurveForKeySize(cfg.KeySize)
		if err != nil {
			return nil, fmt.Errorf("can't determine ECDSA curve: %w", err)
		}
		return NewECDSAKeyGenerator(cfg.BufferSizeMin, cfg.BufferSizeMax, curve, cfg.BufferDelay)

	default:
		return nil, fmt.Errorf("unsupported key type %q, supported values are: %s, %s", cfg.Type, RSAKeyType, ECDSAKeyType)
	}
}

func ecdsaCurveForKeySize(keySize int) (elliptic.Curve, error) {
	switch keySize {
	case 256:
		return elliptic.P256(), nil
	case 384:
		return elliptic.P384(), nil
	case 521:
		return elliptic.P521(), nil
	default:
		return nil, fmt.Errorf("unsupported ECDSA key size %d, supported values are: 256, 384, 521", keySize)
	}
}

type RSAKeyGenerator struct {
	itemgenerator.Generator[rsa.PrivateKey]
}

var _ KeyGenerator = &RSAKeyGenerator{}

func NewRSAKeyGenerator(min, max, keySize int, delay time.Duration) (*RSAKeyGenerator, error) {
	g, err := itemgenerator.NewGenerator[rsa.PrivateKey]("RSAKeyGenerator", min, max, delay, func() (*rsa.PrivateKey, error) {
		privateKey, err := rsa.GenerateKey(rand.Reader, keySize)
		if err != nil {
			return nil, fmt.Errorf("can't generate key: %w", err)
		}

		return privateKey, nil
	})
	if err != nil {
		return nil, fmt.Errorf("can't create RSAKeyGenerator: %w", err)
	}

	return &RSAKeyGenerator{
		Generator: *g,
	}, err
}

func (g *RSAKeyGenerator) GetNewKey(ctx context.Context) (crypto.Signer, error) {
	return g.GetItem(ctx)
}

func (g *RSAKeyGenerator) GetKeyType() KeyType {
	return RSAKeyType
}

type ECDSAKeyGenerator struct {
	itemgenerator.Generator[ecdsa.PrivateKey]
}

var _ KeyGenerator = &ECDSAKeyGenerator{}

func NewECDSAKeyGenerator(min, max int, curve elliptic.Curve, delay time.Duration) (*ECDSAKeyGenerator, error) {
	g, err := itemgenerator.NewGenerator[ecdsa.PrivateKey]("ECDSAKeyGenerator", min, max, delay, func() (*ecdsa.PrivateKey, error) {
		privateKey, err := ecdsa.GenerateKey(curve, rand.Reader)
		if err != nil {
			return nil, fmt.Errorf("can't generate key: %w", err)
		}

		return privateKey, nil
	})
	if err != nil {
		return nil, fmt.Errorf("can't create ECDSAKeyGenerator: %w", err)
	}

	return &ECDSAKeyGenerator{
		Generator: *g,
	}, err
}

func (g *ECDSAKeyGenerator) GetNewKey(ctx context.Context) (crypto.Signer, error) {
	return g.GetItem(ctx)
}

func (g *ECDSAKeyGenerator) GetKeyType() KeyType {
	return ECDSAKeyType
}
