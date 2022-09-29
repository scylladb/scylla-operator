package crypto

import (
	"crypto/x509"
	"encoding/asn1"
	"encoding/json"
	"net"
	"net/url"

	"github.com/scylladb/scylla-operator/pkg/helpers"
)

type pkixName struct {
	Country, Organization, OrganizationalUnit []string
	Locality, Province                        []string
	StreetAddress, PostalCode                 []string
	CommonName                                string
}

// DesiredCertTemplate holds desired fields from a certificate that are not dependent on time.
type DesiredCertTemplate struct {
	Subject               pkixName
	KeyUsage              x509.KeyUsage
	ExtKeyUsage           []x509.ExtKeyUsage
	BasicConstraintsValid bool
	IsCA                  bool
	MaxPathLen            int
	MaxPathLenZero        bool
	// RFC 5280, 4.2.2.1 (Authority Information Access)
	OCSPServer            []string
	IssuingCertificateURL []string
	// Subject Alternate Name values.
	DNSNames       []string
	EmailAddresses []string
	IPAddresses    []net.IP
	URIs           []*url.URL
	// Name constraints
	PermittedDNSDomainsCritical bool // if true then the name constraints are marked critical.
	PermittedDNSDomains         []string
	ExcludedDNSDomains          []string
	PermittedIPRanges           []*net.IPNet
	ExcludedIPRanges            []*net.IPNet
	PermittedEmailAddresses     []string
	ExcludedEmailAddresses      []string
	PermittedURIDomains         []string
	ExcludedURIDomains          []string
	// CRL Distribution Points
	CRLDistributionPoints []string
	PolicyIdentifiers     []asn1.ObjectIdentifier
}

func (t *DesiredCertTemplate) ToJson() ([]byte, error) {
	return json.Marshal(t)
}

func (t *DesiredCertTemplate) StringOrDie() string {
	return string(helpers.Must(t.ToJson()))
}

func ExtractDesiredFieldsFromTemplate(template *x509.Certificate) *DesiredCertTemplate {
	return &DesiredCertTemplate{
		Subject: pkixName{
			Country:            template.Subject.Country,
			Organization:       template.Subject.Organization,
			OrganizationalUnit: template.Subject.OrganizationalUnit,
			Locality:           template.Subject.Locality,
			Province:           template.Subject.Province,
			StreetAddress:      template.Subject.StreetAddress,
			PostalCode:         template.Subject.PostalCode,
			CommonName:         template.Subject.CommonName,
		},
		KeyUsage:                    template.KeyUsage,
		ExtKeyUsage:                 template.ExtKeyUsage,
		BasicConstraintsValid:       template.BasicConstraintsValid,
		IsCA:                        template.IsCA,
		OCSPServer:                  template.OCSPServer,
		IssuingCertificateURL:       template.IssuingCertificateURL,
		DNSNames:                    template.DNSNames,
		EmailAddresses:              template.EmailAddresses,
		IPAddresses:                 helpers.NormalizeIPs(template.IPAddresses),
		URIs:                        template.URIs,
		PermittedDNSDomainsCritical: template.PermittedDNSDomainsCritical,
		PermittedDNSDomains:         template.PermittedDNSDomains,
		ExcludedDNSDomains:          template.ExcludedDNSDomains,
		PermittedIPRanges:           template.PermittedIPRanges,
		ExcludedIPRanges:            template.ExcludedIPRanges,
		PermittedEmailAddresses:     template.PermittedEmailAddresses,
		ExcludedEmailAddresses:      template.ExcludedEmailAddresses,
		PermittedURIDomains:         template.PermittedURIDomains,
		ExcludedURIDomains:          template.ExcludedURIDomains,
		CRLDistributionPoints:       template.CRLDistributionPoints,
		PolicyIdentifiers:           template.PolicyIdentifiers,
	}
}
