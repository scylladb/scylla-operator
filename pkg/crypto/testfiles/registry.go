// Package testfiles contains test certificate fixtures manually generated using the operator's crypto functions.
package testfiles

import (
	_ "embed"
)

var (
	//go:embed "alpha-ca.crt"
	AlphaCACertBytes []byte

	//go:embed "alpha-ca.key"
	AlphaCAKeyBytes []byte

	//go:embed "alpha-serving.crt"
	AlphaServingCertBytes []byte

	//go:embed "alpha-serving.key"
	AlphaServingKeyBytes []byte

	//go:embed "alpha-ecdsa-ca.crt"
	AlphaECDSACACertBytes []byte

	//go:embed "alpha-ecdsa-ca.key"
	AlphaECDSACAKeyBytes []byte

	//go:embed "alpha-ecdsa-serving.crt"
	AlphaECDSAServingCertBytes []byte

	//go:embed "alpha-ecdsa-serving.key"
	AlphaECDSAServingKeyBytes []byte
)
