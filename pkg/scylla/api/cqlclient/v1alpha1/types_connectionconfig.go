package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Datacenter struct {
	// CertificateAuthorityPath is the path to a cert file for the certificate authority.
	// +optional
	CertificateAuthorityPath string `json:"certificateAuthorityPath,omitempty"`

	// CertificateAuthorityData contains PEM-encoded certificate authority certificates. Overrides CertificateAuthorityPath.
	// +optional
	CertificateAuthorityData []byte `json:"certificateAuthorityData,omitempty"`

	// Server is the address of the Scylla cluster. All requests are sent to this address.
	// Example: https://hostname:port
	Server string `json:"server"`

	// nodeDomain is the domain suffix that is concatenated with the host_id of the node driver wants to connect to.
	// The concatenated string is then set as TLS server name for this TLS connection. This value is only used
	// for setting the ServerName in TLS ClientHello, never resolved or used as a server address.
	// Example: host_id.<nodeDomain>
	// Client uses this nodeDomain as bare TLS server name when reaching out to the load balanced (discovery) endpoint.
	NodeDomain string `json:"nodeDomain"`

	// InsecureSkipTLSVerify skips the validity check for the server's certificate.
	// This will make your connections insecure.
	// +optional
	InsecureSkipTLSVerify bool `json:"insecureSkipTlsVerify,omitempty"`

	// ProxyURL is the URL to the proxy to be used for all requests made by this
	// client. URLs with "http", "https", and "socks5" schemes are supported. If
	// this configuration is not provided or the empty string, the client
	// attempts to construct a proxy configuration from http_proxy and
	// https_proxy environment variables. If these environment variables are not
	// set, the client does not attempt to proxy requests.
	// +optional
	ProxyURL string `json:"proxyUrl,omitempty"`
}

type AuthInfo struct {
	// ClientCertificatePath is the path to a client cert file for TLS.
	// +optional
	ClientCertificatePath string `json:"clientCertificatePath,omitempty"`

	// ClientCertificateData contains PEM-encoded data from a client cert file for TLS. Overrides ClientCertificatePath.
	// +optional
	ClientCertificateData []byte `json:"clientCertificateData,omitempty"`

	// ClientKeyPath is the path to a client key file for TLS.
	// +optional
	ClientKeyPath string `json:"clientKeyPath,omitempty"`

	// ClientKeyData contains PEM-encoded data from a client key file for TLS. Overrides ClientKeyPath.
	// +optional
	ClientKeyData []byte `json:"clientKeyData,omitempty"`

	// Username is the username for basic authentication to the Scylla cluster.
	// +optional
	Username string `json:"username,omitempty"`

	// Password is the password for basic authentication to the Scylla cluster.
	// +optional	`
	Password string `json:"password,omitempty"`
}

type Context struct {
	// DatacenterName is the name of the datacenter for this context.
	DatacenterName string `json:"datacenterName"`

	// AuthInfoName is the name of the authInfo for this context.
	AuthInfoName string `json:"authInfoName"`
}

type CQLConsistencyString string

const (
	CQLDefaultAnyConsistency         CQLConsistencyString = "ANY"
	CQLDefaultOneConsistency         CQLConsistencyString = "ONE"
	CQLDefaultTwoConsistency         CQLConsistencyString = "TWO"
	CQLDefaultThreeConsistency       CQLConsistencyString = "THREE"
	CQLDefaultQuorumConsistency      CQLConsistencyString = "QUORUM"
	CQLDefaultAllConsistency         CQLConsistencyString = "ALL"
	CQLDefaultLocalQuorumConsistency CQLConsistencyString = "LOCAL_QUORUM"
	CQLDefaultEachQuorumConsistency  CQLConsistencyString = "EACH_QUORUM"
	CQLDefaultSerialConsistency      CQLConsistencyString = "SERIAL"
	CQLDefaultLocalSerialConsistency CQLConsistencyString = "LOCAL_SERIAL"
	CQLDefaultLocalOneConsistency    CQLConsistencyString = "LOCAL_ONE"
)

type CQLParameters struct {
	// DefaultConsistency is the default consistency level used for user queries.
	// +optional
	DefaultConsistency CQLConsistencyString `json:"defaultConsistency,omitempty"`

	// DefaultSerialConsistency is the default consistency level for the serial part of user queries.
	// +optional
	DefaultSerialConsistency CQLConsistencyString `json:"defaultSerialConsistency,omitempty"`
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CQLConnectionConfig holds the information needed to build CQL connections to remote cluster using SNI proxy.
type CQLConnectionConfig struct {
	metav1.TypeMeta `json:",inline"`

	// Datacenters is a map of referencable names to datacenter configs.
	Datacenters map[string]*Datacenter `json:"datacenters"`

	// AuthInfos is a map of referencable names to authentication configs.
	AuthInfos map[string]*AuthInfo `json:"authInfos"`

	// Contexts is a map of referencable names to context configs.
	Contexts map[string]*Context `json:"contexts"`

	// CurrentContext is the name of the context in use.
	CurrentContext string `json:"currentContext"`

	// Parameters is a struct containing common driver configuration parameters.
	// +optional
	Parameters *CQLParameters `json:"parameters,omitempty"`
}
