// Copyright (C) 2023 ScyllaDB

package crypto

import (
	"context"
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

	return &RSAKeyGenerator{
		Generator: *g,
	}, err
}

func (g *RSAKeyGenerator) GetNewKey(ctx context.Context) (*rsa.PrivateKey, error) {
	return g.GetItem(ctx)
}
