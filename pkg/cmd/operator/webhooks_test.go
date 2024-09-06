// Copyright (c) 2021 ScyllaDB

package operator

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"os"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/scylladb/scylla-operator/pkg/genericclioptions"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apiserver/pkg/server/dynamiccertificates"
)

const (
	pollInterval = 1 * time.Second
	pollTimeout  = 3 * time.Second
)

func TestMain(m *testing.M) {
	dynamiccertificates.FileRefreshDuration = 1 * time.Second
	os.Exit(m.Run())
}

func TestWebhookOptionsRun(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tt := []struct {
		Name           string
		WebhookOptions *WebhookOptions
		ExpectedError  error
	}{
		{
			Name: "valid webhook options",
			WebhookOptions: func() *WebhookOptions {
				wo := NewWebhookOptions(genericclioptions.IOStreams{}, DefaultValidators)
				wo.Port = 65535
				wo.InsecureGenerateLocalhostCerts = true

				return wo
			}(),
			ExpectedError: nil,
		},
		{
			Name: "invalid port",
			WebhookOptions: func() *WebhookOptions {
				wo := NewWebhookOptions(genericclioptions.IOStreams{}, DefaultValidators)
				wo.Port = 65536
				wo.InsecureGenerateLocalhostCerts = true

				return wo
			}(),
			ExpectedError: fmt.Errorf("can't create listener: %w", &net.OpError{
				Op:  "listen",
				Net: "tcp",
				Err: &net.AddrError{
					Err:  "invalid port",
					Addr: "65536",
				},
			}),
		},
	}

	for _, tc := range tt {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			err := tc.WebhookOptions.Complete()
			if err != nil {
				t.Fatalf("can't complete WebhookOptions: %v", err)
			}

			err = tc.WebhookOptions.run(ctx, genericclioptions.IOStreams{
				Out:    os.Stdout,
				ErrOut: os.Stderr,
			})
			if !reflect.DeepEqual(tc.ExpectedError, err) {
				t.Errorf("expected and actual errors differ: %s", cmp.Diff(tc.ExpectedError, err))
			}
		})
	}
}

func TestWebhookOptionsRunWithReload(t *testing.T) {
	var wg sync.WaitGroup
	defer wg.Wait()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dir := t.TempDir()
	tlsCertFile := dir + "/tls.crt"
	tlsKeyFile := dir + "/tls.key"

	cc, err := newCertificateCreator()
	if err != nil {
		t.Fatalf("can't create certificateCreator: %v", err)
	}

	initialCertSerialNumber := big.NewInt(1)
	encodedInitialCert, err := cc.generateEncodedCertificate(initialCertSerialNumber)
	if err != nil {
		t.Fatalf("can't create new encoded certificate: %v", err)
	}

	err = createFileWithContent(tlsCertFile, encodedInitialCert.cert)
	if err != nil {
		t.Fatalf("can't create a file: %v", err)
	}

	err = createFileWithContent(tlsKeyFile, encodedInitialCert.privateKey)
	if err != nil {
		t.Fatalf("can't create a file: %v", err)
	}

	wo := NewWebhookOptions(genericclioptions.IOStreams{}, DefaultValidators)
	wo.TLSCertFile = tlsCertFile
	wo.TLSKeyFile = tlsKeyFile
	wo.Port = 0
	wo.InsecureGenerateLocalhostCerts = false

	err = wo.Complete()
	if err != nil {
		t.Fatalf("can't complete WebhookOptions: %v", err)
	}

	wg.Add(1)
	go func() {
		defer wg.Done()

		err := wo.run(ctx, genericclioptions.IOStreams{
			Out:    os.Stdout,
			ErrOut: os.Stderr,
		})
		if err != nil {
			t.Errorf("can't run webhook server: %v", err)
		}
	}()

	certpool := x509.NewCertPool()
	certpool.AppendCertsFromPEM(cc.encodedCA)
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs: certpool,
			},
		},
	}

	// Wait for the listen address to be established.
	<-wo.resolvedListenAddrCh
	addr := fmt.Sprintf("https://%s/readyz", wo.resolvedListenAddr)

	hasExpectedCertificate := func(expectedCertificateSerialNumber *big.Int) func() (bool, error) {
		return func() (bool, error) {
			resp, err := client.Get(addr)
			if err != nil {
				t.Logf("can't make a GET request: %v", err)
				return false, nil
			}

			if !reflect.DeepEqual(expectedCertificateSerialNumber, resp.TLS.PeerCertificates[0].SerialNumber) {
				t.Logf("serial numbers differ: expected %s, actual %s", expectedCertificateSerialNumber, resp.TLS.PeerCertificates[0].SerialNumber)
				return false, nil
			}

			return true, nil
		}
	}

	err = wait.PollImmediate(pollInterval, pollTimeout, hasExpectedCertificate(initialCertSerialNumber))
	if err != nil {
		t.Fatalf("can't observe the initial certificate: %v", err)
	}

	updatedCertSerialNumber := big.NewInt(2)
	encodedUpdatedCert, err := cc.generateEncodedCertificate(updatedCertSerialNumber)
	if err != nil {
		t.Fatalf("can't create new encoded certificate: %v", err)
	}

	err = createFileWithContent(wo.TLSCertFile, encodedUpdatedCert.cert)
	if err != nil {
		t.Fatalf("can't create a file: %v", err)
	}

	err = createFileWithContent(wo.TLSKeyFile, encodedUpdatedCert.privateKey)
	if err != nil {
		t.Fatalf("can't create a file: %v", err)
	}

	err = wait.PollImmediate(pollInterval, pollTimeout, hasExpectedCertificate(updatedCertSerialNumber))
	if err != nil {
		t.Errorf("certificate wasn't updated: %v", err)
	}
}

type certificateCreator struct {
	caCert       *x509.Certificate
	caPrivateKey *rsa.PrivateKey
	encodedCA    []byte
}

type encodedCertificate struct {
	cert       []byte
	privateKey []byte
}

func newCertificateCreator() (*certificateCreator, error) {
	now := time.Now()
	caCert := &x509.Certificate{
		SerialNumber: big.NewInt(0),
		NotBefore:    now,
		NotAfter:     now.Add(1 * time.Hour),

		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,

		DNSNames:    []string{"localhost"},
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::")},
	}

	caPrivateKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, fmt.Errorf("can't generate key: %w", err)
	}

	caBytes, err := x509.CreateCertificate(rand.Reader, caCert, caCert, caPrivateKey.Public(), caPrivateKey)
	if err != nil {
		return nil, fmt.Errorf("can't create a certificate: %w", err)
	}

	encodedCA, err := pemEncode(caBytes, "CERTIFICATE")
	if err != nil {
		return nil, fmt.Errorf("can't encode a certificate: %w", err)
	}

	return &certificateCreator{
		caCert:       caCert,
		caPrivateKey: caPrivateKey,
		encodedCA:    encodedCA,
	}, nil
}

func (cc *certificateCreator) generateEncodedCertificate(serialNumber *big.Int) (*encodedCertificate, error) {
	now := time.Now()
	cert := &x509.Certificate{
		SerialNumber: serialNumber,
		NotBefore:    now,
		NotAfter:     now.Add(1 * time.Hour),

		KeyUsage:    x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},

		DNSNames:    []string{"localhost"},
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::")},
	}

	privateKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, fmt.Errorf("can't generate key: %w", err)
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, cert, cc.caCert, privateKey.Public(), cc.caPrivateKey)
	if err != nil {
		return nil, fmt.Errorf("can't create a certificate: %w", err)
	}

	certEncoded, err := pemEncode(certBytes, "CERTIFICATE")
	if err != nil {
		return nil, fmt.Errorf("can't encode a certificate: %w", err)
	}

	privateKeyEncoded, err := pemEncode(x509.MarshalPKCS1PrivateKey(privateKey), "RSA PRIVATE KEY")
	if err != nil {
		return nil, fmt.Errorf("can't encode a private key: %w", err)
	}

	return &encodedCertificate{
		cert:       certEncoded,
		privateKey: privateKeyEncoded,
	}, nil
}

func pemEncode(cert []byte, certType string) ([]byte, error) {
	buff := &bytes.Buffer{}
	err := pem.Encode(buff, &pem.Block{
		Type:  certType,
		Bytes: cert,
	})
	if err != nil {
		return nil, fmt.Errorf("can't PEM encode: %w", err)
	}

	return buff.Bytes(), nil
}

func createFileWithContent(path string, data []byte) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("can't create file: %w", err)
	}
	defer f.Close()

	_, err = f.Write(data)
	if err != nil {
		return fmt.Errorf("can't write to file: %w", err)
	}

	return nil
}
