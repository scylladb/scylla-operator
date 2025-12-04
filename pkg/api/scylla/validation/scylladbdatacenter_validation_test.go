// Copyright (c) 2024 ScyllaDB.

package validation_test

import (
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/api/scylla/validation"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func TestValidateScyllaDBDatacenter(t *testing.T) {
	t.Parallel()

	newValidScyllaDBDatacenter := func() *scyllav1alpha1.ScyllaDBDatacenter {
		return &scyllav1alpha1.ScyllaDBDatacenter{
			ObjectMeta: metav1.ObjectMeta{
				Name: "basic",
				UID:  "the-uid",
				Labels: map[string]string{
					"default-sc-label": "foo",
				},
				Annotations: map[string]string{
					"default-sc-annotation": "bar",
				},
			},
			Spec: scyllav1alpha1.ScyllaDBDatacenterSpec{
				ClusterName:    "basic",
				DatacenterName: pointer.Ptr("dc"),
				ScyllaDB: scyllav1alpha1.ScyllaDB{
					Image: "scylladb/scylla:latest",
				},
				ScyllaDBManagerAgent: &scyllav1alpha1.ScyllaDBManagerAgent{
					Image: pointer.Ptr("scylladb/scylla-manager-agent:latest"),
				},
				Racks: []scyllav1alpha1.RackSpec{
					{
						Name: "rack",
						RackTemplate: scyllav1alpha1.RackTemplate{
							ScyllaDB: &scyllav1alpha1.ScyllaDBTemplate{
								Storage: &scyllav1alpha1.StorageOptions{
									Capacity: "1Gi",
								},
							},
						},
					},
				},
			},
			Status: scyllav1alpha1.ScyllaDBDatacenterStatus{
				Racks: []scyllav1alpha1.RackStatus{},
			},
		}
	}

	tests := []struct {
		name                string
		datacenter          *scyllav1alpha1.ScyllaDBDatacenter
		expectedErrorList   field.ErrorList
		expectedErrorString string
	}{
		{
			name:                "valid",
			datacenter:          newValidScyllaDBDatacenter(),
			expectedErrorList:   nil,
			expectedErrorString: "",
		},
		{
			name: "invalid ScyllaDB image",
			datacenter: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sdc := newValidScyllaDBDatacenter()
				sdc.Spec.ScyllaDB.Image = "invalid image"
				return sdc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.scyllaDB.image", BadValue: "invalid image", Detail: "unable to parse image: invalid reference format"},
			},
			expectedErrorString: `spec.scyllaDB.image: Invalid value: "invalid image": unable to parse image: invalid reference format`,
		},
		{
			name: "empty ScyllaDB image",
			datacenter: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sdc := newValidScyllaDBDatacenter()
				sdc.Spec.ScyllaDB.Image = ""
				return sdc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeRequired, Field: "spec.scyllaDB.image", BadValue: "", Detail: "must not be empty"},
			},
			expectedErrorString: `spec.scyllaDB.image: Required value: must not be empty`,
		},
		{
			name: "invalid ScyllaDBManagerAgent image",
			datacenter: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sdc := newValidScyllaDBDatacenter()
				sdc.Spec.ScyllaDBManagerAgent.Image = pointer.Ptr("invalid image")
				return sdc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.scyllaDBManagerAgent.image", BadValue: "invalid image", Detail: "unable to parse image: invalid reference format"},
			},
			expectedErrorString: `spec.scyllaDBManagerAgent.image: Invalid value: "invalid image": unable to parse image: invalid reference format`,
		},
		{
			name: "empty ScyllaDBManagerAgent image",
			datacenter: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sdc := newValidScyllaDBDatacenter()
				sdc.Spec.ScyllaDBManagerAgent.Image = pointer.Ptr("")
				return sdc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeRequired, Field: "spec.scyllaDBManagerAgent.image", BadValue: "", Detail: "must not be empty"},
			},
			expectedErrorString: `spec.scyllaDBManagerAgent.image: Required value: must not be empty`,
		},
		{
			name: "two racks with same name",
			datacenter: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sdc := newValidScyllaDBDatacenter()
				sdc.Spec.Racks = append(sdc.Spec.Racks, *sdc.Spec.Racks[0].DeepCopy())
				return sdc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeDuplicate, Field: "spec.racks[1].name", BadValue: "rack"},
			},
			expectedErrorString: `spec.racks[1].name: Duplicate value: "rack"`,
		},
		{
			name: "when CQL ingress is provided, domains must not be empty",
			datacenter: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sdc := newValidScyllaDBDatacenter()
				sdc.Spec.ExposeOptions = &scyllav1alpha1.ExposeOptions{
					CQL: &scyllav1alpha1.CQLExposeOptions{
						Ingress: &scyllav1alpha1.CQLExposeIngressOptions{},
					},
				}

				return sdc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeRequired, Field: "spec.dnsDomains", BadValue: "", Detail: "at least one domain needs to be provided when exposing CQL via ingresses"},
			},
			expectedErrorString: `spec.dnsDomains: Required value: at least one domain needs to be provided when exposing CQL via ingresses`,
		},
		{
			name: "invalid domain",
			datacenter: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sdc := newValidScyllaDBDatacenter()
				sdc.Spec.DNSDomains = []string{"-hello"}

				return sdc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.dnsDomains[0]", BadValue: "-hello", Detail: `a lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character (e.g. 'example.com', regex used for validation is '[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*')`},
			},
			expectedErrorString: `spec.dnsDomains[0]: Invalid value: "-hello": a lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character (e.g. 'example.com', regex used for validation is '[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*')`,
		},
		{
			name: "empty node service type",
			datacenter: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sdc := newValidScyllaDBDatacenter()
				sdc.Spec.ExposeOptions = &scyllav1alpha1.ExposeOptions{
					NodeService: &scyllav1alpha1.NodeServiceTemplate{
						Type: "",
					},
				}

				return sdc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeRequired, Field: "spec.exposeOptions.nodeService.type", BadValue: "", Detail: `supported values: Headless, ClusterIP, LoadBalancer`},
			},
			expectedErrorString: `spec.exposeOptions.nodeService.type: Required value: supported values: Headless, ClusterIP, LoadBalancer`,
		},
		{
			name: "unsupported type of node service",
			datacenter: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sdc := newValidScyllaDBDatacenter()
				sdc.Spec.ExposeOptions = &scyllav1alpha1.ExposeOptions{
					NodeService: &scyllav1alpha1.NodeServiceTemplate{
						Type: "foo",
					},
				}

				return sdc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeNotSupported, Field: "spec.exposeOptions.nodeService.type", BadValue: scyllav1alpha1.NodeServiceType("foo"), Detail: `supported values: "Headless", "ClusterIP", "LoadBalancer"`},
			},
			expectedErrorString: `spec.exposeOptions.nodeService.type: Unsupported value: "foo": supported values: "Headless", "ClusterIP", "LoadBalancer"`,
		},
		{
			name: "invalid load balancer class name in node service template",
			datacenter: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sdc := newValidScyllaDBDatacenter()
				sdc.Spec.ExposeOptions = &scyllav1alpha1.ExposeOptions{
					NodeService: &scyllav1alpha1.NodeServiceTemplate{
						Type:              scyllav1alpha1.NodeServiceTypeClusterIP,
						LoadBalancerClass: pointer.Ptr("-hello"),
					},
				}

				return sdc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.exposeOptions.nodeService.loadBalancerClass", BadValue: "-hello", Detail: `name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]')`},
			},
			expectedErrorString: `spec.exposeOptions.nodeService.loadBalancerClass: Invalid value: "-hello": name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]')`,
		},
		{
			name: "EKS NLB LoadBalancerClass is valid",
			datacenter: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sdc := newValidScyllaDBDatacenter()
				sdc.Spec.ExposeOptions = &scyllav1alpha1.ExposeOptions{
					NodeService: &scyllav1alpha1.NodeServiceTemplate{
						Type:              scyllav1alpha1.NodeServiceTypeLoadBalancer,
						LoadBalancerClass: pointer.Ptr("service.k8s.aws/nlb"),
					},
				}

				return sdc
			}(),
			expectedErrorList:   nil,
			expectedErrorString: "",
		},
		{
			name: "unsupported type of client broadcast address",
			datacenter: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sdc := newValidScyllaDBDatacenter()
				sdc.Spec.ExposeOptions = &scyllav1alpha1.ExposeOptions{
					BroadcastOptions: &scyllav1alpha1.NodeBroadcastOptions{
						Nodes: scyllav1alpha1.BroadcastOptions{
							Type: scyllav1alpha1.BroadcastAddressTypeServiceClusterIP,
						},
						Clients: scyllav1alpha1.BroadcastOptions{
							Type: "foo",
						},
					},
				}

				return sdc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeNotSupported, Field: "spec.exposeOptions.broadcastOptions.clients.type", BadValue: scyllav1alpha1.BroadcastAddressType("foo"), Detail: `supported values: "PodIP", "ServiceClusterIP", "ServiceLoadBalancerIngress"`},
			},
			expectedErrorString: `spec.exposeOptions.broadcastOptions.clients.type: Unsupported value: "foo": supported values: "PodIP", "ServiceClusterIP", "ServiceLoadBalancerIngress"`,
		},
		{
			name: "unsupported type of node broadcast address",
			datacenter: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sdc := newValidScyllaDBDatacenter()
				sdc.Spec.ExposeOptions = &scyllav1alpha1.ExposeOptions{
					BroadcastOptions: &scyllav1alpha1.NodeBroadcastOptions{
						Nodes: scyllav1alpha1.BroadcastOptions{
							Type: "foo",
						},
						Clients: scyllav1alpha1.BroadcastOptions{
							Type: scyllav1alpha1.BroadcastAddressTypeServiceClusterIP,
						},
					},
				}

				return sdc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeNotSupported, Field: "spec.exposeOptions.broadcastOptions.nodes.type", BadValue: scyllav1alpha1.BroadcastAddressType("foo"), Detail: `supported values: "PodIP", "ServiceClusterIP", "ServiceLoadBalancerIngress"`},
			},
			expectedErrorString: `spec.exposeOptions.broadcastOptions.nodes.type: Unsupported value: "foo": supported values: "PodIP", "ServiceClusterIP", "ServiceLoadBalancerIngress"`,
		},
		{
			name: "invalid ClusterIP broadcast type when node service is Headless",
			datacenter: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sdc := newValidScyllaDBDatacenter()
				sdc.Spec.ExposeOptions = &scyllav1alpha1.ExposeOptions{
					NodeService: &scyllav1alpha1.NodeServiceTemplate{
						Type: scyllav1alpha1.NodeServiceTypeHeadless,
					},
					BroadcastOptions: &scyllav1alpha1.NodeBroadcastOptions{
						Clients: scyllav1alpha1.BroadcastOptions{
							Type: scyllav1alpha1.BroadcastAddressTypeServiceClusterIP,
						},
						Nodes: scyllav1alpha1.BroadcastOptions{
							Type: scyllav1alpha1.BroadcastAddressTypeServiceClusterIP,
						},
					},
				}

				return sdc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.exposeOptions.broadcastOptions.clients.type", BadValue: scyllav1alpha1.BroadcastAddressTypeServiceClusterIP, Detail: `can't broadcast address unavailable within the selected node service type, allowed types for chosen broadcast address type are: [ClusterIP LoadBalancer]`},
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.exposeOptions.broadcastOptions.nodes.type", BadValue: scyllav1alpha1.BroadcastAddressTypeServiceClusterIP, Detail: `can't broadcast address unavailable within the selected node service type, allowed types for chosen broadcast address type are: [ClusterIP LoadBalancer]`},
			},
			expectedErrorString: `[spec.exposeOptions.broadcastOptions.clients.type: Invalid value: "ServiceClusterIP": can't broadcast address unavailable within the selected node service type, allowed types for chosen broadcast address type are: [ClusterIP LoadBalancer], spec.exposeOptions.broadcastOptions.nodes.type: Invalid value: "ServiceClusterIP": can't broadcast address unavailable within the selected node service type, allowed types for chosen broadcast address type are: [ClusterIP LoadBalancer]]`,
		},
		{
			name: "invalid LoadBalancerIngressIP broadcast type when node service is Headless",
			datacenter: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sdc := newValidScyllaDBDatacenter()
				sdc.Spec.ExposeOptions = &scyllav1alpha1.ExposeOptions{
					NodeService: &scyllav1alpha1.NodeServiceTemplate{
						Type: scyllav1alpha1.NodeServiceTypeHeadless,
					},
					BroadcastOptions: &scyllav1alpha1.NodeBroadcastOptions{
						Clients: scyllav1alpha1.BroadcastOptions{
							Type: scyllav1alpha1.BroadcastAddressTypeServiceLoadBalancerIngress,
						},
						Nodes: scyllav1alpha1.BroadcastOptions{
							Type: scyllav1alpha1.BroadcastAddressTypeServiceLoadBalancerIngress,
						},
					},
				}

				return sdc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.exposeOptions.broadcastOptions.clients.type", BadValue: scyllav1alpha1.BroadcastAddressTypeServiceLoadBalancerIngress, Detail: `can't broadcast address unavailable within the selected node service type, allowed types for chosen broadcast address type are: [LoadBalancer]`},
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.exposeOptions.broadcastOptions.nodes.type", BadValue: scyllav1alpha1.BroadcastAddressTypeServiceLoadBalancerIngress, Detail: `can't broadcast address unavailable within the selected node service type, allowed types for chosen broadcast address type are: [LoadBalancer]`},
			},
			expectedErrorString: `[spec.exposeOptions.broadcastOptions.clients.type: Invalid value: "ServiceLoadBalancerIngress": can't broadcast address unavailable within the selected node service type, allowed types for chosen broadcast address type are: [LoadBalancer], spec.exposeOptions.broadcastOptions.nodes.type: Invalid value: "ServiceLoadBalancerIngress": can't broadcast address unavailable within the selected node service type, allowed types for chosen broadcast address type are: [LoadBalancer]]`,
		},
		{
			name: "invalid LoadBalancerIngressIP broadcast type when node service is ClusterIP",
			datacenter: func() *scyllav1alpha1.ScyllaDBDatacenter {
				cluster := newValidScyllaDBDatacenter()
				cluster.Spec.ExposeOptions = &scyllav1alpha1.ExposeOptions{
					NodeService: &scyllav1alpha1.NodeServiceTemplate{
						Type: scyllav1alpha1.NodeServiceTypeClusterIP,
					},
					BroadcastOptions: &scyllav1alpha1.NodeBroadcastOptions{
						Clients: scyllav1alpha1.BroadcastOptions{
							Type: scyllav1alpha1.BroadcastAddressTypeServiceLoadBalancerIngress,
						},
						Nodes: scyllav1alpha1.BroadcastOptions{
							Type: scyllav1alpha1.BroadcastAddressTypeServiceLoadBalancerIngress,
						},
					},
				}

				return cluster
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.exposeOptions.broadcastOptions.clients.type", BadValue: scyllav1alpha1.BroadcastAddressTypeServiceLoadBalancerIngress, Detail: `can't broadcast address unavailable within the selected node service type, allowed types for chosen broadcast address type are: [LoadBalancer]`},
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.exposeOptions.broadcastOptions.nodes.type", BadValue: scyllav1alpha1.BroadcastAddressTypeServiceLoadBalancerIngress, Detail: `can't broadcast address unavailable within the selected node service type, allowed types for chosen broadcast address type are: [LoadBalancer]`},
			},
			expectedErrorString: `[spec.exposeOptions.broadcastOptions.clients.type: Invalid value: "ServiceLoadBalancerIngress": can't broadcast address unavailable within the selected node service type, allowed types for chosen broadcast address type are: [LoadBalancer], spec.exposeOptions.broadcastOptions.nodes.type: Invalid value: "ServiceLoadBalancerIngress": can't broadcast address unavailable within the selected node service type, allowed types for chosen broadcast address type are: [LoadBalancer]]`,
		},
		{
			name: "negative minTerminationGracePeriodSeconds",
			datacenter: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sdc := newValidScyllaDBDatacenter()
				sdc.Spec.MinTerminationGracePeriodSeconds = pointer.Ptr(int32(-42))

				return sdc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.minTerminationGracePeriodSeconds", BadValue: int64(-42), Detail: "must be greater than or equal to 0", Origin: "minimum"},
			},
			expectedErrorString: `spec.minTerminationGracePeriodSeconds: Invalid value: -42: must be greater than or equal to 0`,
		},
		{
			name: "negative minReadySeconds",
			datacenter: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sdc := newValidScyllaDBDatacenter()
				sdc.Spec.MinReadySeconds = pointer.Ptr(int32(-42))

				return sdc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.minReadySeconds", BadValue: int64(-42), Detail: "must be greater than or equal to 0", Origin: "minimum"},
			},
			expectedErrorString: `spec.minReadySeconds: Invalid value: -42: must be greater than or equal to 0`,
		},
		{
			name: "minimal alternator cluster passes",
			datacenter: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sdc := newValidScyllaDBDatacenter()
				sdc.Spec.ScyllaDB.AlternatorOptions = &scyllav1alpha1.AlternatorOptions{}
				return sdc
			}(),
			expectedErrorList:   nil,
			expectedErrorString: "",
		},
		{
			name: "alternator cluster with user certificate",
			datacenter: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sdc := newValidScyllaDBDatacenter()
				sdc.Spec.ScyllaDB.AlternatorOptions = &scyllav1alpha1.AlternatorOptions{
					ServingCertificate: &scyllav1alpha1.TLSCertificate{
						Type: scyllav1alpha1.TLSCertificateTypeUserManaged,
						UserManagedOptions: &scyllav1alpha1.UserManagedTLSCertificateOptions{
							SecretName: "my-tls-certificate",
						},
					},
				}
				return sdc
			}(),
			expectedErrorList:   nil,
			expectedErrorString: "",
		},
		{
			name: "alternator cluster with invalid certificate type",
			datacenter: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sdc := newValidScyllaDBDatacenter()
				sdc.Spec.ScyllaDB.AlternatorOptions = &scyllav1alpha1.AlternatorOptions{
					ServingCertificate: &scyllav1alpha1.TLSCertificate{
						Type: "foo",
					},
				}
				return sdc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeNotSupported, Field: "spec.scyllaDB.alternator.servingCertificate.type", BadValue: scyllav1alpha1.TLSCertificateType("foo"), Detail: `supported values: "OperatorManaged", "UserManaged"`},
			},
			expectedErrorString: `spec.scyllaDB.alternator.servingCertificate.type: Unsupported value: "foo": supported values: "OperatorManaged", "UserManaged"`,
		},
		{
			name: "alternator cluster with valid additional domains",
			datacenter: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sdc := newValidScyllaDBDatacenter()
				sdc.Spec.ScyllaDB.AlternatorOptions = &scyllav1alpha1.AlternatorOptions{
					ServingCertificate: &scyllav1alpha1.TLSCertificate{
						Type: scyllav1alpha1.TLSCertificateTypeOperatorManaged,
						OperatorManagedOptions: &scyllav1alpha1.OperatorManagedTLSCertificateOptions{
							AdditionalDNSNames: []string{"scylla-operator.scylladb.com"},
						},
					},
				}
				return sdc
			}(),
			expectedErrorList:   nil,
			expectedErrorString: "",
		},
		{
			name: "alternator cluster with invalid additional domains",
			datacenter: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sdc := newValidScyllaDBDatacenter()
				sdc.Spec.ScyllaDB.AlternatorOptions = &scyllav1alpha1.AlternatorOptions{
					ServingCertificate: &scyllav1alpha1.TLSCertificate{
						Type: scyllav1alpha1.TLSCertificateTypeOperatorManaged,
						OperatorManagedOptions: &scyllav1alpha1.OperatorManagedTLSCertificateOptions{
							AdditionalDNSNames: []string{"[not a domain]"},
						},
					},
				}
				return sdc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.scyllaDB.alternator.servingCertificate.operatorManagedOptions.additionalDNSNames", BadValue: []string{"[not a domain]"}, Detail: `a lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character (e.g. 'example.com', regex used for validation is '[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*')`},
			},
			expectedErrorString: `spec.scyllaDB.alternator.servingCertificate.operatorManagedOptions.additionalDNSNames: Invalid value: ["[not a domain]"]: a lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character (e.g. 'example.com', regex used for validation is '[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*')`,
		},
		{
			name: "alternator cluster with valid additional IP addresses",
			datacenter: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sdc := newValidScyllaDBDatacenter()
				sdc.Spec.ScyllaDB.AlternatorOptions = &scyllav1alpha1.AlternatorOptions{
					ServingCertificate: &scyllav1alpha1.TLSCertificate{
						Type: scyllav1alpha1.TLSCertificateTypeOperatorManaged,
						OperatorManagedOptions: &scyllav1alpha1.OperatorManagedTLSCertificateOptions{
							AdditionalIPAddresses: []string{"127.0.0.1", "::1"},
						},
					},
				}
				return sdc
			}(),
			expectedErrorList:   nil,
			expectedErrorString: "",
		},
		{
			name: "alternator cluster with invalid additional IP addresses",
			datacenter: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sdc := newValidScyllaDBDatacenter()
				sdc.Spec.ScyllaDB.AlternatorOptions = &scyllav1alpha1.AlternatorOptions{
					ServingCertificate: &scyllav1alpha1.TLSCertificate{
						Type: scyllav1alpha1.TLSCertificateTypeOperatorManaged,
						OperatorManagedOptions: &scyllav1alpha1.OperatorManagedTLSCertificateOptions{
							AdditionalIPAddresses: []string{"0.not-an-ip.0.0"},
						},
					},
				}
				return sdc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.scyllaDB.alternator.servingCertificate.operatorManagedOptions.additionalIPAddresses", BadValue: "0.not-an-ip.0.0", Detail: `must be a valid IP address, (e.g. 10.9.8.7 or 2001:db8::ffff)`, Origin: "format=ip-strict"},
			},
			expectedErrorString: `spec.scyllaDB.alternator.servingCertificate.operatorManagedOptions.additionalIPAddresses: Invalid value: "0.not-an-ip.0.0": must be a valid IP address, (e.g. 10.9.8.7 or 2001:db8::ffff)`,
		},
		{
			name: "negative rackTemplate nodes",
			datacenter: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sdc := newValidScyllaDBDatacenter()
				sdc.Spec.RackTemplate = &scyllav1alpha1.RackTemplate{
					Nodes: pointer.Ptr[int32](-42),
				}

				return sdc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.rackTemplate.nodes", BadValue: int64(-42), Detail: "must be greater than or equal to 0", Origin: "minimum"},
			},
			expectedErrorString: `spec.rackTemplate.nodes: Invalid value: -42: must be greater than or equal to 0`,
		},
		{
			name: "invalid topologyLabelSelector in rackTemplate",
			datacenter: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sdc := newValidScyllaDBDatacenter()
				sdc.Spec.RackTemplate = &scyllav1alpha1.RackTemplate{
					TopologyLabelSelector: map[string]string{
						"-123": "*321",
					},
				}

				return sdc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.rackTemplate.topologyLabelSelector", BadValue: "-123", Detail: `name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]')`, Origin: "labelKey"},
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.rackTemplate.topologyLabelSelector", BadValue: "*321", Detail: `a valid label must be an empty string or consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyValue',  or 'my_value',  or '12345', regex used for validation is '(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])?')`},
			},
			expectedErrorString: `[spec.rackTemplate.topologyLabelSelector: Invalid value: "-123": name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]'), spec.rackTemplate.topologyLabelSelector: Invalid value: "*321": a valid label must be an empty string or consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyValue',  or 'my_value',  or '12345', regex used for validation is '(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])?')]`,
		},
		{
			name: "invalid scyllaDB storage capacity in rackTemplate",
			datacenter: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sdc := newValidScyllaDBDatacenter()
				sdc.Spec.RackTemplate = &scyllav1alpha1.RackTemplate{
					ScyllaDB: &scyllav1alpha1.ScyllaDBTemplate{
						Storage: &scyllav1alpha1.StorageOptions{
							Capacity: "hello",
						},
					},
				}

				return sdc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.rackTemplate.scyllaDB.storage.capacity", BadValue: "hello", Detail: `unable to parse capacity: quantities must match the regular expression '^([+-]?[0-9.]+)([eEinumkKMGTP]*[-+]?[0-9]*)$'`},
			},
			expectedErrorString: `spec.rackTemplate.scyllaDB.storage.capacity: Invalid value: "hello": unable to parse capacity: quantities must match the regular expression '^([+-]?[0-9.]+)([eEinumkKMGTP]*[-+]?[0-9]*)$'`,
		},
		{
			name: "invalid negative scyllaDB storage capacity in rackTemplate",
			datacenter: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sdc := newValidScyllaDBDatacenter()
				sdc.Spec.RackTemplate = &scyllav1alpha1.RackTemplate{
					ScyllaDB: &scyllav1alpha1.ScyllaDBTemplate{
						Storage: &scyllav1alpha1.StorageOptions{
							Capacity: "-1Gi",
						},
					},
				}

				return sdc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.rackTemplate.scyllaDB.storage.capacity", BadValue: "-1Gi", Detail: `must be greater than zero`},
			},
			expectedErrorString: `spec.rackTemplate.scyllaDB.storage.capacity: Invalid value: "-1Gi": must be greater than zero`,
		},
		{
			name: "invalid labels in rackTemplate scyllaDB storage metadata",
			datacenter: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sdc := newValidScyllaDBDatacenter()
				sdc.Spec.RackTemplate = &scyllav1alpha1.RackTemplate{
					ScyllaDB: &scyllav1alpha1.ScyllaDBTemplate{
						Storage: &scyllav1alpha1.StorageOptions{
							Capacity: "1Gi",
							Metadata: &scyllav1alpha1.ObjectTemplateMetadata{
								Labels: map[string]string{
									"-123": "*321",
								},
							},
						},
					},
				}

				return sdc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.rackTemplate.scyllaDB.storage.metadata.labels", BadValue: "-123", Detail: `name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]')`, Origin: "labelKey"},
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.rackTemplate.scyllaDB.storage.metadata.labels", BadValue: "*321", Detail: `a valid label must be an empty string or consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyValue',  or 'my_value',  or '12345', regex used for validation is '(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])?')`},
			},
			expectedErrorString: `[spec.rackTemplate.scyllaDB.storage.metadata.labels: Invalid value: "-123": name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]'), spec.rackTemplate.scyllaDB.storage.metadata.labels: Invalid value: "*321": a valid label must be an empty string or consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyValue',  or 'my_value',  or '12345', regex used for validation is '(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])?')]`,
		},
		{
			name: "invalid annotations in rackTemplate scyllaDB storage metadata",
			datacenter: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sdc := newValidScyllaDBDatacenter()
				sdc.Spec.RackTemplate = &scyllav1alpha1.RackTemplate{
					ScyllaDB: &scyllav1alpha1.ScyllaDBTemplate{
						Storage: &scyllav1alpha1.StorageOptions{
							Capacity: "1Gi",
							Metadata: &scyllav1alpha1.ObjectTemplateMetadata{
								Annotations: map[string]string{
									"-123": "*321",
								},
							},
						},
					},
				}

				return sdc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.rackTemplate.scyllaDB.storage.metadata.annotations", BadValue: "-123", Detail: `name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]')`},
			},
			expectedErrorString: `spec.rackTemplate.scyllaDB.storage.metadata.annotations: Invalid value: "-123": name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]')`,
		},
		{
			name: "invalid storageClassName in rackTemplate scyllaDB storage",
			datacenter: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sdc := newValidScyllaDBDatacenter()
				sdc.Spec.RackTemplate = &scyllav1alpha1.RackTemplate{
					ScyllaDB: &scyllav1alpha1.ScyllaDBTemplate{
						Storage: &scyllav1alpha1.StorageOptions{
							Capacity:         "1Gi",
							StorageClassName: pointer.Ptr("-hello"),
						},
					},
				}

				return sdc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.rackTemplate.scyllaDB.storage.storageClassName", BadValue: "-hello", Detail: `a lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character (e.g. 'example.com', regex used for validation is '[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*')`},
			},
			expectedErrorString: `spec.rackTemplate.scyllaDB.storage.storageClassName: Invalid value: "-hello": a lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character (e.g. 'example.com', regex used for validation is '[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*')`,
		},
		{
			name: "invalid customConfigMapRef in scyllaDB template of rackTemplate",
			datacenter: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sdc := newValidScyllaDBDatacenter()
				sdc.Spec.RackTemplate = &scyllav1alpha1.RackTemplate{
					ScyllaDB: &scyllav1alpha1.ScyllaDBTemplate{
						CustomConfigMapRef: pointer.Ptr("-hello"),
					},
				}

				return sdc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.rackTemplate.scyllaDB.customConfigMapRef", BadValue: "-hello", Detail: `a lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character (e.g. 'example.com', regex used for validation is '[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*')`},
			},
			expectedErrorString: `spec.rackTemplate.scyllaDB.customConfigMapRef: Invalid value: "-hello": a lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character (e.g. 'example.com', regex used for validation is '[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*')`,
		},
		{
			name: "invalid customConfigSecretRef of ScyllaDB Manager Agent rackTemplate",
			datacenter: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sdc := newValidScyllaDBDatacenter()
				sdc.Spec.RackTemplate = &scyllav1alpha1.RackTemplate{
					ScyllaDBManagerAgent: &scyllav1alpha1.ScyllaDBManagerAgentTemplate{
						CustomConfigSecretRef: pointer.Ptr("-hello"),
					},
				}

				return sdc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.rackTemplate.scyllaDBManagerAgent.customConfigSecretRef", BadValue: "-hello", Detail: `a lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character (e.g. 'example.com', regex used for validation is '[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*')`},
			},
			expectedErrorString: `spec.rackTemplate.scyllaDBManagerAgent.customConfigSecretRef: Invalid value: "-hello": a lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character (e.g. 'example.com', regex used for validation is '[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*')`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			errList := validation.ValidateScyllaDBDatacenter(test.datacenter)
			if !reflect.DeepEqual(errList, test.expectedErrorList) {
				t.Errorf("expected and actual error lists differ: %s", cmp.Diff(test.expectedErrorList, errList))
			}

			var errStr string
			if agg := errList.ToAggregate(); agg != nil {
				errStr = agg.Error()
			}
			if !reflect.DeepEqual(errStr, test.expectedErrorString) {
				t.Errorf("expected and actual error strings differ: %s", cmp.Diff(test.expectedErrorString, errStr))
			}
		})
	}
}

func TestValidateScyllaDBDatacenterUpdate(t *testing.T) {
	newValidScyllaDBDatacenter := func() *scyllav1alpha1.ScyllaDBDatacenter {
		return &scyllav1alpha1.ScyllaDBDatacenter{
			ObjectMeta: metav1.ObjectMeta{
				Name: "basic",
				UID:  "the-uid",
				Labels: map[string]string{
					"default-sc-label": "foo",
				},
				Annotations: map[string]string{
					"default-sc-annotation": "bar",
				},
				Generation: 1,
			},
			Spec: scyllav1alpha1.ScyllaDBDatacenterSpec{
				ClusterName:    "basic",
				DatacenterName: pointer.Ptr("dc"),
				ScyllaDB: scyllav1alpha1.ScyllaDB{
					Image: "scylladb/scylla:latest",
				},
				ScyllaDBManagerAgent: &scyllav1alpha1.ScyllaDBManagerAgent{
					Image: pointer.Ptr("scylladb/scylla-manager-agent:latest"),
				},
				Racks: []scyllav1alpha1.RackSpec{
					{
						Name: "rack",
						RackTemplate: scyllav1alpha1.RackTemplate{
							ScyllaDB: &scyllav1alpha1.ScyllaDBTemplate{
								Storage: &scyllav1alpha1.StorageOptions{
									Capacity: "1Gi",
								},
							},
						},
					},
				},
			},
			Status: scyllav1alpha1.ScyllaDBDatacenterStatus{
				ObservedGeneration: pointer.Ptr[int64](1),
				Racks:              []scyllav1alpha1.RackStatus{},
			},
		}
	}

	tests := []struct {
		name                string
		old                 *scyllav1alpha1.ScyllaDBDatacenter
		new                 *scyllav1alpha1.ScyllaDBDatacenter
		expectedErrorList   field.ErrorList
		expectedErrorString string
	}{
		{
			name:                "same as old",
			old:                 newValidScyllaDBDatacenter(),
			new:                 newValidScyllaDBDatacenter(),
			expectedErrorList:   nil,
			expectedErrorString: "",
		},
		{
			name: "clusterName changed",
			old:  newValidScyllaDBDatacenter(),
			new: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sdc := newValidScyllaDBDatacenter()
				sdc.Spec.ClusterName = "foo"
				return sdc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.clusterName", BadValue: "foo", Detail: "field is immutable"},
			},
			expectedErrorString: `spec.clusterName: Invalid value: "foo": field is immutable`,
		},
		{
			name: "rackStorage changed",
			old:  newValidScyllaDBDatacenter(),
			new: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sdc := newValidScyllaDBDatacenter()
				sdc.Spec.Racks[0].RackTemplate.ScyllaDB.Storage.Capacity = "123Gi"
				return sdc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeForbidden, Field: "spec.racks[0].scyllaDB.storage", BadValue: "", Detail: "changes in storage are currently not supported"},
			},
			expectedErrorString: "spec.racks[0].scyllaDB.storage: Forbidden: changes in storage are currently not supported",
		},
		{
			name: "empty rack removed",
			old: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sdc := newValidScyllaDBDatacenter()
				sdc.Spec.Racks[0].Nodes = pointer.Ptr[int32](0)
				return sdc
			}(),
			new: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sdc := newValidScyllaDBDatacenter()
				sdc.Spec.Racks = []scyllav1alpha1.RackSpec{}
				return sdc
			}(),
			expectedErrorList:   nil,
			expectedErrorString: "",
		},
		{
			name: "empty rack with members under decommission",
			old: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sdc := newValidScyllaDBDatacenter()
				sdc.Status.Racks = []scyllav1alpha1.RackStatus{
					{
						Name:  sdc.Spec.Racks[0].Name,
						Nodes: pointer.Ptr[int32](3),
					},
				}
				return sdc
			}(),
			new: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sdc := newValidScyllaDBDatacenter()
				sdc.Spec.Racks = []scyllav1alpha1.RackSpec{}
				return sdc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeForbidden, Field: "spec.racks[0]", BadValue: "", Detail: `rack "rack" can't be removed because the members are being scaled down`},
			},
			expectedErrorString: `spec.racks[0]: Forbidden: rack "rack" can't be removed because the members are being scaled down`,
		},
		{
			name: "empty rack with stale status",
			old: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sdc := newValidScyllaDBDatacenter()
				sdc.Status.Racks = []scyllav1alpha1.RackStatus{
					{
						Name:  sdc.Spec.Racks[0].Name,
						Nodes: pointer.Ptr[int32](0),
						Stale: pointer.Ptr(true),
					},
				}
				return sdc
			}(),
			new: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sdc := newValidScyllaDBDatacenter()
				sdc.Spec.Racks = []scyllav1alpha1.RackSpec{}
				return sdc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInternal, Field: "spec.racks[0]", Detail: `rack "rack" can't be removed because its status, that's used to determine members count, is not yet up to date with the generation of this resource; please retry later`},
			},
			expectedErrorString: `spec.racks[0]: Internal error: rack "rack" can't be removed because its status, that's used to determine members count, is not yet up to date with the generation of this resource; please retry later`,
		},
		{
			name: "empty rack with not reconciled generation",
			old: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sdc := newValidScyllaDBDatacenter()
				sdc.Generation = 2
				sdc.Status.ObservedGeneration = pointer.Ptr[int64](1)
				sdc.Status.Racks = []scyllav1alpha1.RackStatus{
					{
						Name:  sdc.Spec.Racks[0].Name,
						Nodes: pointer.Ptr[int32](0),
					},
				}
				return sdc
			}(),
			new: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sdc := newValidScyllaDBDatacenter()
				sdc.Spec.Racks = []scyllav1alpha1.RackSpec{}
				return sdc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInternal, Field: "spec.racks[0]", Detail: `rack "rack" can't be removed because its status, that's used to determine members count, is not yet up to date with the generation of this resource; please retry later`},
			},
			expectedErrorString: `spec.racks[0]: Internal error: rack "rack" can't be removed because its status, that's used to determine members count, is not yet up to date with the generation of this resource; please retry later`,
		},
		{
			name: "non-empty racks deleted",
			old: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sdc := newValidScyllaDBDatacenter()
				sdc.Spec.Racks = []scyllav1alpha1.RackSpec{
					func() scyllav1alpha1.RackSpec {
						rackSpec := *sdc.Spec.Racks[0].DeepCopy()
						rackSpec.Name = "rack-0"
						rackSpec.Nodes = pointer.Ptr[int32](3)
						return rackSpec
					}(),
					func() scyllav1alpha1.RackSpec {
						rackSpec := *sdc.Spec.Racks[0].DeepCopy()
						rackSpec.Name = "rack-1"
						rackSpec.Nodes = pointer.Ptr[int32](2)
						return rackSpec
					}(),
					func() scyllav1alpha1.RackSpec {
						rackSpec := *sdc.Spec.Racks[0].DeepCopy()
						rackSpec.Name = "rack-2"
						rackSpec.Nodes = pointer.Ptr[int32](1)
						return rackSpec
					}(),
				}
				sdc.Status.Racks = []scyllav1alpha1.RackStatus{
					{
						Name:  sdc.Spec.Racks[0].Name,
						Nodes: pointer.Ptr[int32](3),
						Stale: pointer.Ptr(false),
					},
					{
						Name:  sdc.Spec.Racks[1].Name,
						Nodes: pointer.Ptr[int32](2),
						Stale: pointer.Ptr(false),
					},
					{
						Name:  sdc.Spec.Racks[2].Name,
						Nodes: pointer.Ptr[int32](1),
						Stale: pointer.Ptr(false),
					},
				}
				return sdc
			}(),
			new: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sdc := newValidScyllaDBDatacenter()
				sdc.Spec.Racks = []scyllav1alpha1.RackSpec{}
				return sdc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeForbidden, Field: "spec.racks[0]", BadValue: "", Detail: `rack "rack-0" can't be removed because it still has members that have to be scaled down to zero first`},
				&field.Error{Type: field.ErrorTypeForbidden, Field: "spec.racks[1]", BadValue: "", Detail: `rack "rack-1" can't be removed because it still has members that have to be scaled down to zero first`},
				&field.Error{Type: field.ErrorTypeForbidden, Field: "spec.racks[2]", BadValue: "", Detail: `rack "rack-2" can't be removed because it still has members that have to be scaled down to zero first`},
			},
			expectedErrorString: `[spec.racks[0]: Forbidden: rack "rack-0" can't be removed because it still has members that have to be scaled down to zero first, spec.racks[1]: Forbidden: rack "rack-1" can't be removed because it still has members that have to be scaled down to zero first, spec.racks[2]: Forbidden: rack "rack-2" can't be removed because it still has members that have to be scaled down to zero first]`,
		},
		{
			name: "node service type cannot be unset",
			old: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sdc := newValidScyllaDBDatacenter()
				sdc.Spec.ExposeOptions = &scyllav1alpha1.ExposeOptions{
					NodeService: &scyllav1alpha1.NodeServiceTemplate{
						Type: scyllav1alpha1.NodeServiceTypeClusterIP,
					},
				}
				return sdc
			}(),
			new: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sdc := newValidScyllaDBDatacenter()
				sdc.Spec.ExposeOptions = nil
				return sdc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.exposeOptions.nodeService.type", BadValue: (*scyllav1alpha1.NodeServiceType)(nil), Detail: `field is immutable`},
			},
			expectedErrorString: `spec.exposeOptions.nodeService.type: Invalid value: null: field is immutable`,
		},
		{
			name: "node service type cannot be changed",
			old: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sdc := newValidScyllaDBDatacenter()
				sdc.Spec.ExposeOptions = &scyllav1alpha1.ExposeOptions{
					NodeService: &scyllav1alpha1.NodeServiceTemplate{
						Type: scyllav1alpha1.NodeServiceTypeHeadless,
					},
				}
				return sdc
			}(),
			new: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sdc := newValidScyllaDBDatacenter()
				sdc.Spec.ExposeOptions = &scyllav1alpha1.ExposeOptions{
					NodeService: &scyllav1alpha1.NodeServiceTemplate{
						Type: scyllav1alpha1.NodeServiceTypeClusterIP,
					},
				}
				return sdc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.exposeOptions.nodeService.type", BadValue: pointer.Ptr(scyllav1alpha1.NodeServiceTypeClusterIP), Detail: `field is immutable`},
			},
			expectedErrorString: `spec.exposeOptions.nodeService.type: Invalid value: "ClusterIP": field is immutable`,
		},
		{
			name: "clients broadcast address type cannot be changed",
			old: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sdc := newValidScyllaDBDatacenter()
				sdc.Spec.ExposeOptions = &scyllav1alpha1.ExposeOptions{
					BroadcastOptions: &scyllav1alpha1.NodeBroadcastOptions{
						Clients: scyllav1alpha1.BroadcastOptions{
							Type: scyllav1alpha1.BroadcastAddressTypeServiceClusterIP,
						},
						Nodes: scyllav1alpha1.BroadcastOptions{
							Type: scyllav1alpha1.BroadcastAddressTypeServiceClusterIP,
						},
					},
				}
				return sdc
			}(),
			new: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sdc := newValidScyllaDBDatacenter()
				sdc.Spec.ExposeOptions = &scyllav1alpha1.ExposeOptions{
					BroadcastOptions: &scyllav1alpha1.NodeBroadcastOptions{
						Clients: scyllav1alpha1.BroadcastOptions{
							Type: scyllav1alpha1.BroadcastAddressTypePodIP,
						},
						Nodes: scyllav1alpha1.BroadcastOptions{
							Type: scyllav1alpha1.BroadcastAddressTypeServiceClusterIP,
						},
					},
				}
				return sdc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.exposeOptions.broadcastOptions.clients.type", BadValue: pointer.Ptr(scyllav1alpha1.BroadcastAddressTypePodIP), Detail: `field is immutable`},
			},
			expectedErrorString: `spec.exposeOptions.broadcastOptions.clients.type: Invalid value: "PodIP": field is immutable`,
		},
		{
			name: "nodes broadcast address type cannot be changed",
			old: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sdc := newValidScyllaDBDatacenter()
				sdc.Spec.ExposeOptions = &scyllav1alpha1.ExposeOptions{
					BroadcastOptions: &scyllav1alpha1.NodeBroadcastOptions{
						Clients: scyllav1alpha1.BroadcastOptions{
							Type: scyllav1alpha1.BroadcastAddressTypeServiceClusterIP,
						},
						Nodes: scyllav1alpha1.BroadcastOptions{
							Type: scyllav1alpha1.BroadcastAddressTypeServiceClusterIP,
						},
					},
				}
				return sdc
			}(),
			new: func() *scyllav1alpha1.ScyllaDBDatacenter {
				sdc := newValidScyllaDBDatacenter()
				sdc.Spec.ExposeOptions = &scyllav1alpha1.ExposeOptions{
					BroadcastOptions: &scyllav1alpha1.NodeBroadcastOptions{
						Clients: scyllav1alpha1.BroadcastOptions{
							Type: scyllav1alpha1.BroadcastAddressTypeServiceClusterIP,
						},
						Nodes: scyllav1alpha1.BroadcastOptions{
							Type: scyllav1alpha1.BroadcastAddressTypePodIP,
						},
					},
				}
				return sdc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.exposeOptions.broadcastOptions.nodes.type", BadValue: pointer.Ptr(scyllav1alpha1.BroadcastAddressTypePodIP), Detail: `field is immutable`},
			},
			expectedErrorString: `spec.exposeOptions.broadcastOptions.nodes.type: Invalid value: "PodIP": field is immutable`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			errList := validation.ValidateScyllaDBDatacenterUpdate(test.new, test.old)
			if !reflect.DeepEqual(errList, test.expectedErrorList) {
				t.Errorf("expected and actual error lists differ: %s", cmp.Diff(test.expectedErrorList, errList))
			}

			errStr := ""
			if agg := errList.ToAggregate(); agg != nil {
				errStr = agg.Error()
			}
			if !reflect.DeepEqual(errStr, test.expectedErrorString) {
				t.Errorf("expected and actual error strings differ: %s", cmp.Diff(test.expectedErrorString, errStr))
			}
		})
	}
}

func TestValidateScyllaArgsIPFamily(t *testing.T) {
	t.Parallel()

	ipv4 := corev1.IPv4Protocol
	ipv6 := corev1.IPv6Protocol

	tt := []struct {
		name                string
		ipFamily            *corev1.IPFamily
		scyllaArgs          []string
		expectedErrorList   field.ErrorList
		expectedErrorString string
	}{
		{
			name:                "no IP family specified with no args is valid",
			ipFamily:            nil,
			scyllaArgs:          []string{},
			expectedErrorList:   nil,
			expectedErrorString: "",
		},
		{
			name:                "IPv4 with no address args is valid",
			ipFamily:            &ipv4,
			scyllaArgs:          []string{"--some-other-flag=value"},
			expectedErrorList:   nil,
			expectedErrorString: "",
		},
		{
			name:                "IPv6 with no address args is valid",
			ipFamily:            &ipv6,
			scyllaArgs:          []string{"--some-other-flag=value"},
			expectedErrorList:   nil,
			expectedErrorString: "",
		},
		{
			name:     "IPv4 with IPv4 rpc-address is valid",
			ipFamily: &ipv4,
			scyllaArgs: []string{
				"--rpc-address=10.0.0.1",
			},
			expectedErrorList:   nil,
			expectedErrorString: "",
		},
		{
			name:     "IPv4 with IPv4 listen-address is valid",
			ipFamily: &ipv4,
			scyllaArgs: []string{
				"--listen-address=10.0.0.2",
			},
			expectedErrorList:   nil,
			expectedErrorString: "",
		},
		{
			name:     "IPv6 with IPv6 rpc-address is valid",
			ipFamily: &ipv6,
			scyllaArgs: []string{
				"--rpc-address=2001:db8::1",
			},
			expectedErrorList:   nil,
			expectedErrorString: "",
		},
		{
			name:     "IPv6 with IPv6 listen-address is valid",
			ipFamily: &ipv6,
			scyllaArgs: []string{
				"--listen-address=2001:db8::2",
			},
			expectedErrorList:   nil,
			expectedErrorString: "",
		},
		{
			name:     "IPv4 with wildcard 0.0.0.0 is valid",
			ipFamily: &ipv4,
			scyllaArgs: []string{
				"--rpc-address=0.0.0.0",
				"--listen-address=0.0.0.0",
			},
			expectedErrorList:   nil,
			expectedErrorString: "",
		},
		{
			name:     "IPv6 with wildcard :: is valid",
			ipFamily: &ipv6,
			scyllaArgs: []string{
				"--rpc-address=::",
				"--listen-address=::",
			},
			expectedErrorList:   nil,
			expectedErrorString: "",
		},
		{
			name:     "default IPv4 with wildcard 0.0.0.0 is valid",
			ipFamily: nil,
			scyllaArgs: []string{
				"--rpc-address=0.0.0.0",
			},
			expectedErrorList:   nil,
			expectedErrorString: "",
		},
		{
			name:     "IPv4 with IPv6 rpc-address is invalid",
			ipFamily: &ipv4,
			scyllaArgs: []string{
				"--rpc-address=2001:db8::1",
			},
			expectedErrorList: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeInvalid,
					Field:    "test",
					BadValue: "--rpc-address=2001:db8::1",
					Detail:   "--rpc-address '2001:db8::1' IP family (IPv6) must match spec.ipFamily (IPv4)",
				},
			},
			expectedErrorString: `test: Invalid value: "--rpc-address=2001:db8::1": --rpc-address '2001:db8::1' IP family (IPv6) must match spec.ipFamily (IPv4)`,
		},
		{
			name:     "IPv4 with IPv6 listen-address is invalid",
			ipFamily: &ipv4,
			scyllaArgs: []string{
				"--listen-address=2001:db8::2",
			},
			expectedErrorList: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeInvalid,
					Field:    "test",
					BadValue: "--listen-address=2001:db8::2",
					Detail:   "--listen-address '2001:db8::2' IP family (IPv6) must match spec.ipFamily (IPv4)",
				},
			},
			expectedErrorString: `test: Invalid value: "--listen-address=2001:db8::2": --listen-address '2001:db8::2' IP family (IPv6) must match spec.ipFamily (IPv4)`,
		},
		{
			name:     "IPv6 with IPv4 rpc-address is invalid",
			ipFamily: &ipv6,
			scyllaArgs: []string{
				"--rpc-address=10.0.0.1",
			},
			expectedErrorList: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeInvalid,
					Field:    "test",
					BadValue: "--rpc-address=10.0.0.1",
					Detail:   "--rpc-address '10.0.0.1' IP family (IPv4) must match spec.ipFamily (IPv6)",
				},
			},
			expectedErrorString: `test: Invalid value: "--rpc-address=10.0.0.1": --rpc-address '10.0.0.1' IP family (IPv4) must match spec.ipFamily (IPv6)`,
		},
		{
			name:     "IPv6 with IPv4 listen-address is invalid",
			ipFamily: &ipv6,
			scyllaArgs: []string{
				"--listen-address=10.0.0.2",
			},
			expectedErrorList: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeInvalid,
					Field:    "test",
					BadValue: "--listen-address=10.0.0.2",
					Detail:   "--listen-address '10.0.0.2' IP family (IPv4) must match spec.ipFamily (IPv6)",
				},
			},
			expectedErrorString: `test: Invalid value: "--listen-address=10.0.0.2": --listen-address '10.0.0.2' IP family (IPv4) must match spec.ipFamily (IPv6)`,
		},
		{
			name:     "IPv4 with multiple mismatched addresses returns multiple errors",
			ipFamily: &ipv4,
			scyllaArgs: []string{
				"--rpc-address=2001:db8::1",
				"--listen-address=2001:db8::2",
			},
			expectedErrorList: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeInvalid,
					Field:    "test",
					BadValue: "--rpc-address=2001:db8::1 --listen-address=2001:db8::2",
					Detail:   "--rpc-address '2001:db8::1' IP family (IPv6) must match spec.ipFamily (IPv4)",
				},
				&field.Error{
					Type:     field.ErrorTypeInvalid,
					Field:    "test",
					BadValue: "--rpc-address=2001:db8::1 --listen-address=2001:db8::2",
					Detail:   "--listen-address '2001:db8::2' IP family (IPv6) must match spec.ipFamily (IPv4)",
				},
			},
			expectedErrorString: `[test: Invalid value: "--rpc-address=2001:db8::1 --listen-address=2001:db8::2": --rpc-address '2001:db8::1' IP family (IPv6) must match spec.ipFamily (IPv4), test: Invalid value: "--rpc-address=2001:db8::1 --listen-address=2001:db8::2": --listen-address '2001:db8::2' IP family (IPv6) must match spec.ipFamily (IPv4)]`,
		},
		{
			name:     "quoted IPv6 rpc-address with IPv4 is invalid",
			ipFamily: &ipv4,
			scyllaArgs: []string{
				`--rpc-address="2001:db8::1"`,
			},
			expectedErrorList: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeInvalid,
					Field:    "test",
					BadValue: `--rpc-address="2001:db8::1"`,
					Detail:   "--rpc-address '2001:db8::1' IP family (IPv6) must match spec.ipFamily (IPv4)",
				},
			},
			expectedErrorString: `test: Invalid value: "--rpc-address=\"2001:db8::1\"": --rpc-address '2001:db8::1' IP family (IPv6) must match spec.ipFamily (IPv4)`,
		},
		{
			name:     "space-separated IPv4 rpc-address with IPv6 is invalid",
			ipFamily: &ipv6,
			scyllaArgs: []string{
				"--rpc-address 10.0.0.1",
			},
			expectedErrorList: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeInvalid,
					Field:    "test",
					BadValue: "--rpc-address 10.0.0.1",
					Detail:   "--rpc-address '10.0.0.1' IP family (IPv4) must match spec.ipFamily (IPv6)",
				},
			},
			expectedErrorString: `test: Invalid value: "--rpc-address 10.0.0.1": --rpc-address '10.0.0.1' IP family (IPv4) must match spec.ipFamily (IPv6)`,
		},
		{
			name:     "default IPv4 with IPv6 rpc-address is invalid",
			ipFamily: nil,
			scyllaArgs: []string{
				"--rpc-address=2001:db8::1",
			},
			expectedErrorList: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeInvalid,
					Field:    "test",
					BadValue: "--rpc-address=2001:db8::1",
					Detail:   "--rpc-address '2001:db8::1' IP family (IPv6) must match spec.ipFamily (IPv4)",
				},
			},
			expectedErrorString: `test: Invalid value: "--rpc-address=2001:db8::1": --rpc-address '2001:db8::1' IP family (IPv6) must match spec.ipFamily (IPv4)`,
		},
		{
			name:     "mixed valid and invalid addresses",
			ipFamily: &ipv4,
			scyllaArgs: []string{
				"--rpc-address=10.0.0.1",
				"--listen-address=2001:db8::2",
				"--some-other-flag=value",
			},
			expectedErrorList: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeInvalid,
					Field:    "test",
					BadValue: "--rpc-address=10.0.0.1 --listen-address=2001:db8::2 --some-other-flag=value",
					Detail:   "--listen-address '2001:db8::2' IP family (IPv6) must match spec.ipFamily (IPv4)",
				},
			},
			expectedErrorString: `test: Invalid value: "--rpc-address=10.0.0.1 --listen-address=2001:db8::2 --some-other-flag=value": --listen-address '2001:db8::2' IP family (IPv6) must match spec.ipFamily (IPv4)`,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			errList := validation.ValidateScyllaArgsIPFamily(tc.ipFamily, tc.scyllaArgs, field.NewPath("test"))
			if !reflect.DeepEqual(errList, tc.expectedErrorList) {
				t.Errorf("expected and actual error lists differ: %s", cmp.Diff(tc.expectedErrorList, errList))
			}

			errStr := ""
			if agg := errList.ToAggregate(); agg != nil {
				errStr = agg.Error()
			}
			if !reflect.DeepEqual(errStr, tc.expectedErrorString) {
				t.Errorf("expected and actual error strings differ: %s", cmp.Diff(tc.expectedErrorString, errStr))
			}
		})
	}
}
