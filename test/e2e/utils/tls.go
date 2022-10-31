package utils

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
)

func GetServerTLSCertificates(address string, tlsConfig *tls.Config) ([]*x509.Certificate, error) {
	connection, err := tls.Dial("tcp", address, tlsConfig)
	if err != nil {
		return nil, fmt.Errorf("can't dial server on %q: %w", address, err)
	}
	defer connection.Close()

	return connection.ConnectionState().PeerCertificates, nil
}
