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
)
