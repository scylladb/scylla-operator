# Support for ScyllaDB Alternator

## Summary

This proposal aims to introduce proper support for enabling Alternator for ScyllaClusters 
and deal with the legacy API fields used to partially enable it only in an insecure manner
which also prevents us from serving on any new port within the ScyllaDB pod in the future 
without breaking existing deployments.

## Motivation

Alternator is an important service provided by ScyllaDB for DynamoDB compatible API
that we need to support in the operator.
We have historical and limited option to enable it but only for unencrypted traffic.
There are no e2e tests for the existing functionality.
Also, the user specified (variable) port means that when we need to add a new port to the sidecar,
we can break existing deployments. Same goes for the Service port.

### Goals

- Expose Alternator service using a load balanced endpoint (Service)
- Expose Alternator over secure and static port
- Expose Alternator on insecure port only if explicitly requested
- Expose Alternator with Authorization on by default
- Ensure e2e coverage for Alternator
- Expose Alternator together with CQL (not exclusively)
- Deprecate `scyllaclusters.scylla.scylladb.com/v1.spec.alternator.port`
- Enable transition period and be backwards compatible with existing clusters

### Non-Goals

- Exposing Alternator on every node individually and relying on client side load balancing
- Supporting host networking tweaks
- Disabling CQL when Alternator is enabled
- Enabling Alternator on insecure port only

## Proposal

I propose to expose Alternator using the existing identity Service (`<sc-name>-client`) on port `8043` (https).
Alternator API won't be exposed by default, only on an explicit request.
When Alternator API is enabled, the insecure port `8000` will be disabled by default, unless explicitly opted-in.
Disabling the insecure port won't have effect on the old setup with manually specified port.
Any Alternator settings won't have effect on how / if CQL is exposed.
This is primarily meant to make sure
the Scylla Operator or the administrator can still execute maintenance task using CQL, like in
[#1358](https://github.com/scylladb/scylla-operator/issues/1358).
Also, the Alternator [authorization](https://enterprise.docs.scylladb.com/stable/alternator/compatibility.html#authorization)
is managed through CQL by updating `system_auth.roles` table.

Alternator will be setup with Authorization enabled. Users can opt out, but it should be secure by default.

In the same release `scyllaclusters.scylla.scylladb.com/v1.spec.alternator.port` will be deprecated, so
we may ignore the value in a future release. In this release, and with the new changes, it is intended for it to retain the existing behaviour for backward compatibility.

### User Stories

#### Automatic certificate
As a user I want to enable Alternator API, and have serving certificate and key automatically generated.
To configure the clients to trust the certificates I'll use the provided serving CA from a ConfigMap.

#### Custom certificate
As a user I want to enable Alternator API and provide my own serving certificate and either ensure it is signed 
by a CA that clients implicitly trust or provide them with an appropriate CA on my own.

#### Ingress
As a user I want to expose Alternator API using an Ingress.
This allows me to expose it externally without allocating a dedicated IP.

#### Load Balancer Service
As a user I want to expose Alternator API on Service with a load balancer.
This allows me to direct traffic directly through a platform LB on a dedicated IP.

### Risks and Mitigations

Not known.

## Design Details

### API changes for ScyllaClusters.scylla.scylladb.com/v1

The following API changes show how we'd approximately extend `scyllaclusters.scylla.scylladb.com/v1.spec.alternator` specification. Only new fields are shown for existing structs.

```golang
package v1

type TLSCertificateType string

const (
	TLSCertificateTypeOperatorManaged   TLSCertificateType = "OperatorManaged"
	TLSCertificateTypeUserManaged       TLSCertificateType = "UserManaged"
)

type UserManagedTLSCertificateOptions struct {
	// secretName references a kubernetes.io/tls type secret containing the TLS cert and key.
	// +kubebuilder:validation:Required
	SecretName string `json:"secretName"`
}

type OperatorManagedTLSCertificateOptions struct {
	// additionalDNSNames represents external DNS names that the certificates should be signed for.
	// +optional
	AdditionalDNSNames []string `json:"additionalDNSNames,omitempty"`
	
	// additionalIPAddresses represents external IP addresses that the certificates should be signed for.
	// +optional
	AdditionalIPAddresses []string `json:"additionalIPAddresses,omitempty"`
}

type TLSCertificate struct {
	// type determines the source of this certificate.
	// +kubebuilder:validation:Enum="OperatorManaged";"UserManaged"
	Type TLSCertificateType `json:"type"`
	
	// userManagedCertificateOptions specify option for certificates manged by users.
	// +optional
	UserManagedOptions *UserManagedTLSCertificateOptions `json:"userManagedOptions,omitempty"`
	
	// OperatorManagedCertificateOptions specify option for certificates manged by the operator.
	// +optional
	OperatorManagedOptions *OperatorManagedTLSCertificateOptions `json:"operatorManagedOptions,omitempty"`
}

// AlternatorSpec hold options related to Alternator API.
// +kubebuilder:validation:XValidation:rule="( has(self.port) && self.port != 0) ? self.insecureEnableHTTP != false : true",message="insecureEnableHTTP can't be false when using (insecure) port."
type AlternatorSpec struct {
	// insecureEnableHTTP enables serving Alternator traffic also on insecure HTTP port.
	InsecureEnableHTTP *bool `json:"insecureEnableHTTP,omitempty"`
	
	// insecureDisableAuthorization disables Alternator authorization.
	// If not specified, the authorization is enabled.
	// For backwards compatibility the authorization is disabled when this field is not specified
	// and a manual port is used.
	// +optional
	InsecureDisableAuthorization *bool `json:"insecureDisableAuthorization,omitempty"`
	
	// servingCertificate references a TLS certificate for serving secure traffic.
	// +kubebuilder:default:="{type:"OperatorManaged"}"
	// +optional
	ServingCertificate *TLSCertificate `json:"servingCertificate,omitempty"`
}
```

### Certificates

Given we aim to always enable HTTPS we need to provision default serving certificate. Operator will create signer Secret `<sc-name>-local-alternator-serving-ca`, save the CA bundle into ConfigMap `<sc-name>-local-alternator-serving-ca` and create serving Secret `<sc-name>-local-alternator-serving-certs`.
Serving certs will be valid for 30 days and rotated from 20 day mark.
Because users can create an alternative Service for the alternator (e.g. LB type), we'll allow specifying additional DNS names and IPs to include in the managed certificates. 

Optionally, users can decide they want to provide a serving certificate on their own, which will disable the operator provided certificates.
This is useful in case users want the service to have a certificate signed by a CA in their root of trust or for similar customizations.

Alternator on HTTPS will be enabled with `--alternator-https-port`.
Alternator certificates will be controlled with `--alternator-encryption-options keyfile="..."` and `--alternator-encryption-options certificate="..."`

### Behaviour details

When `spec.alternator != nil`, the controller will enable Alternator API over HTTPS and expose it on the identity service on port `8043`.
If `spec.alternator.insecureEnableHTTP == true` then the controller will also enable Alternator API over HTTP on port `8000`.
If `spec.alternator.port > 0` it implies `insecureEnableHTTP` to be enabled and will change the port `8000` to the specified value.

For `spec.alternator.port > 0` we will keep the existing behaviour of exposing it also on the member Services to maintain backwards compatibility.

By default, Alternator will have Authorization enabled. Because this wasn't the case previously (when enabled with manual port), we'll not require authorization when `spec.alternator.port > 0 && spec.alternator.insecureDisableAuthorization == nil`. Users can still opt-in by setting `spec.alternator.insecureDisableAuthorization` explicitly to `false`.

### Test Plan

There will be a simple unit test to make sure that correct alternator ports are exposed when enabled and vice versa.

There will also be an e2e test covering the full flow and making sure the TLS layer works as well, including changing the certificate type / source.
Ideally this should include reading from Alternator API during a rolling upgrade and making sure all requests succeed. As it's the default, e2e test will use authorization.

### Upgrade / Downgrade Strategy

No specific action for upgrades / downgrades is needed.

### Version Skew Strategy

For existing ScyllaClusters specifying `alternator.port` will imply `insecureEnableHTTP = true` and `spec.alternator.insecureDisableAuthorization = true` so the clusters stay backwards compatible.
When rolled back this will behave the same way.
Any other new functionality like HTTPS will be "disabled" on rollback and the new certificates may stay unmanaged in the API until the operator is upgraded again.

## Implementation History

- 2023-09-19: Initial enhancement proposal
- 2024-02-16: Extended with using Authorization by default

## Drawbacks

Not known.

## Alternatives

### Extending the problematic user specified ports to cover HTTPS

This seems like a wrong path for us, given this causes conflicts and breaks existing deployments in future versions by design. In the end the path isn't technically that different, it's mostly about API, isolation and architectural split, so it wouldn't have significant impact on the timeline.
