package crypto

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"crypto/x509"
	"fmt"
)

func SupportedECDSACurveBitSizes() []int {
	return []int{256, 384, 521}
}

func SignatureAlgorithmForKey(key crypto.Signer) (x509.SignatureAlgorithm, error) {
	switch k := key.(type) {
	case *rsa.PrivateKey:
		return x509.SHA512WithRSA, nil
	case *ecdsa.PrivateKey:
		switch k.Curve {
		case elliptic.P256():
			return x509.ECDSAWithSHA256, nil
		case elliptic.P384():
			return x509.ECDSAWithSHA384, nil
		case elliptic.P521():
			return x509.ECDSAWithSHA512, nil
		default:
			return x509.UnknownSignatureAlgorithm, fmt.Errorf("unsupported ECDSA curve: %v", k.Curve.Params().Name)
		}
	default:
		return x509.UnknownSignatureAlgorithm, fmt.Errorf("unsupported key type %T", key)
	}
}
