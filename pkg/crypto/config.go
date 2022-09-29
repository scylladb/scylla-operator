package crypto

import (
	"crypto/x509"
)

var (
	keySize            = 4096
	signatureAlgorithm = x509.SHA512WithRSA
)
