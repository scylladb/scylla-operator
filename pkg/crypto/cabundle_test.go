package crypto

import (
	"crypto/x509"
	"reflect"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	hashutil "github.com/scylladb/scylla-operator/pkg/util/hash"
)

func TestMakeCABundle(t *testing.T) {
	now := time.Date(2021, 02, 01, 10, 00, 00, 00, time.UTC)

	tt := []struct {
		name          string
		currentCert   *x509.Certificate
		previousCerts []*x509.Certificate
		expectedCerts []*x509.Certificate
	}{
		{
			name: "valid cert with no previous certs projects into the bundle",
			currentCert: &x509.Certificate{
				NotBefore: now.Add(-1 * time.Hour),
				NotAfter:  now.Add(1 * time.Hour),
			},
			previousCerts: nil,
			expectedCerts: []*x509.Certificate{
				{
					NotBefore: now.Add(-1 * time.Hour),
					NotAfter:  now.Add(1 * time.Hour),
				},
			},
		},
		{
			name: "valid cert and valid previous certs projects into the bundle",
			currentCert: &x509.Certificate{
				NotBefore: now.Add(-1 * time.Hour),
				NotAfter:  now.Add(3 * time.Hour),
			},
			previousCerts: []*x509.Certificate{
				{
					NotBefore: now.Add(-3 * time.Hour),
					NotAfter:  now.Add(1 * time.Hour),
				},
				{
					NotBefore: now.Add(-5 * time.Hour),
					NotAfter:  now.Add(-3 * time.Hour),
				},
			},
			expectedCerts: []*x509.Certificate{
				{
					NotBefore: now.Add(-1 * time.Hour),
					NotAfter:  now.Add(3 * time.Hour),
				},
				{
					NotBefore: now.Add(-3 * time.Hour),
					NotAfter:  now.Add(1 * time.Hour),
				},
			},
		},
		{
			name: "expired cert with no previous certs projects into the bundle",
			currentCert: &x509.Certificate{
				NotBefore: now.Add(-3 * time.Hour),
				NotAfter:  now.Add(-1 * time.Hour),
			},
			previousCerts: nil,
			expectedCerts: []*x509.Certificate{
				{
					NotBefore: now.Add(-3 * time.Hour),
					NotAfter:  now.Add(-1 * time.Hour),
				},
			},
		},
		{
			name: "expired cert with valid previous certs projects into the bundle",
			currentCert: &x509.Certificate{
				NotBefore: now.Add(-3 * time.Hour),
				NotAfter:  now.Add(-1 * time.Hour),
			},
			previousCerts: []*x509.Certificate{
				{
					NotBefore: now.Add(-1 * time.Hour),
					NotAfter:  now.Add(1 * time.Hour),
				},
			},
			expectedCerts: []*x509.Certificate{
				{
					NotBefore: now.Add(-3 * time.Hour),
					NotAfter:  now.Add(-1 * time.Hour),
				},
				{
					NotBefore: now.Add(-1 * time.Hour),
					NotAfter:  now.Add(1 * time.Hour),
				},
			},
		},
		{
			name: "expired cert projects into the bundle but previous expired certs get filtered out",
			currentCert: &x509.Certificate{
				NotBefore: now.Add(-3 * time.Hour),
				NotAfter:  now.Add(-1 * time.Hour),
			},
			previousCerts: []*x509.Certificate{
				{
					NotBefore: now.Add(-1 * time.Hour),
					NotAfter:  now.Add(1 * time.Hour),
				},
				{
					NotBefore: now.Add(-5 * time.Hour),
					NotAfter:  now.Add(-3 * time.Hour),
				},
			},
			expectedCerts: []*x509.Certificate{
				{
					NotBefore: now.Add(-3 * time.Hour),
					NotAfter:  now.Add(-1 * time.Hour),
				},
				{
					NotBefore: now.Add(-1 * time.Hour),
					NotAfter:  now.Add(1 * time.Hour),
				},
			},
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			allCerts := make([]*x509.Certificate, 0, 1+2*len(tc.expectedCerts))
			allCerts = append(allCerts, tc.currentCert)
			allCerts = append(allCerts, tc.previousCerts...)
			for _, c := range allCerts {
				// We need to fill this for the Equal() to work when filtering certs. The content can be arbitrary
				// as long as it is unique for a distinct templates.
				templateHash, err := hashutil.HashObjects(struct {
					NotBefore, NotAfter time.Time
				}{
					NotBefore: c.NotBefore,
					NotAfter:  c.NotAfter,
				})
				if err != nil {
					t.Fatal(err)
				}

				c.Raw = []byte(templateHash)
			}

			got := MakeCABundle(tc.currentCert, tc.previousCerts, now)

			allCerts = append(allCerts, got...)
			for _, c := range allCerts {
				c.Raw = nil
			}

			if !reflect.DeepEqual(got, tc.expectedCerts) {
				t.Errorf("expected and got certs differ: %s", cmp.Diff(tc.expectedCerts, got))
			}
		})
	}
}
