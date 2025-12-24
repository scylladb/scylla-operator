// Copyright (C) 2023 ScyllaDB

package crypto

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"time"

	"github.com/scylladb/scylla-operator/pkg/itemgenerator"
)

type RSAKeyGetter interface {
	GetNewKey(ctx context.Context) (*rsa.PrivateKey, error)
}

type RSAKeyGenerator struct {
	itemgenerator.Generator[rsa.PrivateKey]
}

var _ RSAKeyGetter = &RSAKeyGenerator{}

func NewRSAKeyGenerator(min, max, keySize int, delay time.Duration) (*RSAKeyGenerator, error) {
	g, err := itemgenerator.NewGenerator[rsa.PrivateKey]("RSAKeyGenerator", min, max, delay, func() (*rsa.PrivateKey, error) {
		privateKey, err := rsa.GenerateKey(rand.Reader, keySize)
		if err != nil {
			return nil, fmt.Errorf("can't generate key: %w", err)
		}

		return privateKey, nil
	})
	if err != nil {
		return nil, err
	}

	return &RSAKeyGenerator{
		Generator: *g,
	}, nil
}

func (g *RSAKeyGenerator) GetNewKey(ctx context.Context) (*rsa.PrivateKey, error) {
	return g.GetItem(ctx)
}

type ECDSAKeyGetter interface {
	GetNewKey(ctx context.Context) (*ecdsa.PrivateKey, error)
}

type ECDSAKeyGenerator struct {
	itemgenerator.Generator[ecdsa.PrivateKey]
}

var _ ECDSAKeyGetter = &ECDSAKeyGenerator{}

func NewECDSAKeyGenerator(min, max int, curve elliptic.Curve, delay time.Duration) (*ECDSAKeyGenerator, error) {
	g, err := itemgenerator.NewGenerator[ecdsa.PrivateKey]("ECDSAKeyGenerator", min, max, delay, func() (*ecdsa.PrivateKey, error) {
		privateKey, err := ecdsa.GenerateKey(curve, rand.Reader)
		if err != nil {
			return nil, fmt.Errorf("can't generate key: %w", err)
		}
	if err != nil {
		return nil, err
	}

		return privateKey, nil
	})

	return &ECDSAKeyGenerator{
		Generator: *g,
	}, nil
}

func (g *ECDSAKeyGenerator) GetNewKey(ctx context.Context) (*ecdsa.PrivateKey, error) {
	return g.GetItem(ctx)
}
