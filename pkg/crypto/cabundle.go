package crypto

import (
	"crypto/x509"
	"time"
)

func MakeCABundle(currentCert *x509.Certificate, previousCerts []*x509.Certificate, now time.Time) []*x509.Certificate {
	// The current certificate should always be present, no matter whether it's expired,
	// so there is always at least one certificate.
	certificates := []*x509.Certificate{currentCert}

	// Previous certs only make it through if they are still valid.
	certificates = append(certificates, FilterOutExpiredCertificates(previousCerts, now)...)

	// Remove duplicates.
	certificates = FilterOutDuplicateCertificates(certificates)

	return certificates
}
