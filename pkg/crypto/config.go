package crypto

import (
	"crypto/x509"
)

// KeyType represents the type of private key to generate
type KeyType string

const (
	// KeyTypeRSA represents RSA key type
	KeyTypeRSA KeyType = "RSA"
	// KeyTypeECDSA represents ECDSA key type
	KeyTypeECDSA KeyType = "ECDSA"
)

var (
	signatureAlgorithm = x509.SHA512WithRSA
)
