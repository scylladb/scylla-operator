package validation_test

import (
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/api/scylla/validation"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func TestValidateScyllaDBCluster(t *testing.T) {
	t.Parallel()

	newValidScyllaDBCluster := func() *scyllav1alpha1.ScyllaDBCluster {
		return &scyllav1alpha1.ScyllaDBCluster{
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
			Spec: scyllav1alpha1.ScyllaDBClusterSpec{
				ClusterName: pointer.Ptr("basic"),
				ScyllaDB: scyllav1alpha1.ScyllaDB{
					Image: "scylladb/scylla:latest",
				},
				ScyllaDBManagerAgent: &scyllav1alpha1.ScyllaDBManagerAgent{
					Image: pointer.Ptr("scylladb/scylla-manager-agent:latest"),
				},
				DatacenterTemplate: &scyllav1alpha1.ScyllaDBClusterDatacenterTemplate{
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
				Datacenters: []scyllav1alpha1.ScyllaDBClusterDatacenter{
					{
						Name:                        "dc",
						RemoteKubernetesClusterName: "rkc",
						ScyllaDBClusterDatacenterTemplate: scyllav1alpha1.ScyllaDBClusterDatacenterTemplate{
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
					},
				},
			},
		}
	}

	type tableTest struct {
		name                string
		cluster             *scyllav1alpha1.ScyllaDBCluster
		expectedErrorList   field.ErrorList
		expectedErrorString string
	}

	tests := []tableTest{
		{
			name:                "valid",
			cluster:             newValidScyllaDBCluster(),
			expectedErrorList:   field.ErrorList{},
			expectedErrorString: "",
		},
		{
			name: "invalid ScyllaDB image",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.ScyllaDB.Image = "invalid image"
				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.scyllaDB.image", BadValue: "invalid image", Detail: "unable to parse image: invalid reference format"},
			},
			expectedErrorString: `spec.scyllaDB.image: Invalid value: "invalid image": unable to parse image: invalid reference format`,
		},
		{
			name: "empty ScyllaDB image",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.ScyllaDB.Image = ""
				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeRequired, Field: "spec.scyllaDB.image", BadValue: "", Detail: "must not be empty"},
			},
			expectedErrorString: `spec.scyllaDB.image: Required value: must not be empty`,
		},
		{
			name: "invalid ScyllaDBManagerAgent image",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.ScyllaDBManagerAgent.Image = pointer.Ptr("invalid image")
				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.scyllaDBManagerAgent.image", BadValue: "invalid image", Detail: "unable to parse image: invalid reference format"},
			},
			expectedErrorString: `spec.scyllaDBManagerAgent.image: Invalid value: "invalid image": unable to parse image: invalid reference format`,
		},
		{
			name: "empty ScyllaDBManagerAgent image",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.ScyllaDBManagerAgent.Image = pointer.Ptr("")
				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeRequired, Field: "spec.scyllaDBManagerAgent.image", BadValue: "", Detail: "must not be empty"},
			},
			expectedErrorString: `spec.scyllaDBManagerAgent.image: Required value: must not be empty`,
		},
		{
			name: "two datacenters with same name",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.Datacenters = append(sc.Spec.Datacenters, *sc.Spec.Datacenters[0].DeepCopy())
				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeDuplicate, Field: "spec.datacenters[1].name", BadValue: "dc"},
			},
			expectedErrorString: `spec.datacenters[1].name: Duplicate value: "dc"`,
		},
		{
			name: "two racks with same name in datacenter racks",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.Datacenters[0].Racks = append(sc.Spec.Datacenters[0].Racks, *sc.Spec.Datacenters[0].Racks[0].DeepCopy())
				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeDuplicate, Field: "spec.datacenters[0].racks[1].name", BadValue: "rack"},
			},
			expectedErrorString: `spec.datacenters[0].racks[1].name: Duplicate value: "rack"`,
		},
		{
			name: "two racks with same name in datacenter template racks",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.DatacenterTemplate = &scyllav1alpha1.ScyllaDBClusterDatacenterTemplate{
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
				}

				sc.Spec.Datacenters[0].Racks = nil
				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeDuplicate, Field: "spec.datacenterTemplate.racks[1].name", BadValue: "rack"},
			},
			expectedErrorString: `spec.datacenterTemplate.racks[1].name: Duplicate value: "rack"`,
		},
		{
			name: "empty node service type",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.ExposeOptions = &scyllav1alpha1.ScyllaDBClusterExposeOptions{
					NodeService: &scyllav1alpha1.NodeServiceTemplate{
						Type: "",
					},
				}

				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeRequired, Field: "spec.exposeOptions.nodeService.type", BadValue: "", Detail: `supported values: Headless, LoadBalancer`},
			},
			expectedErrorString: `spec.exposeOptions.nodeService.type: Required value: supported values: Headless, LoadBalancer`,
		},
		{
			name: "unsupported type of node service",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.ExposeOptions = &scyllav1alpha1.ScyllaDBClusterExposeOptions{
					NodeService: &scyllav1alpha1.NodeServiceTemplate{
						Type: "foo",
					},
				}

				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeNotSupported, Field: "spec.exposeOptions.nodeService.type", BadValue: scyllav1alpha1.NodeServiceType("foo"), Detail: `supported values: "Headless", "LoadBalancer"`},
			},
			expectedErrorString: `spec.exposeOptions.nodeService.type: Unsupported value: "foo": supported values: "Headless", "LoadBalancer"`,
		},
		{
			name: "invalid load balancer class name in node service template",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.ExposeOptions = &scyllav1alpha1.ScyllaDBClusterExposeOptions{
					NodeService: &scyllav1alpha1.NodeServiceTemplate{
						Type:              scyllav1alpha1.NodeServiceTypeLoadBalancer,
						LoadBalancerClass: pointer.Ptr("-hello"),
					},
				}

				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.exposeOptions.nodeService.loadBalancerClass", BadValue: "-hello", Detail: `name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]')`},
			},
			expectedErrorString: `spec.exposeOptions.nodeService.loadBalancerClass: Invalid value: "-hello": name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]')`,
		},
		{
			name: "EKS NLB LoadBalancerClass is valid",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.ExposeOptions = &scyllav1alpha1.ScyllaDBClusterExposeOptions{
					NodeService: &scyllav1alpha1.NodeServiceTemplate{
						Type:              scyllav1alpha1.NodeServiceTypeLoadBalancer,
						LoadBalancerClass: pointer.Ptr("service.k8s.aws/nlb"),
					},
				}

				return sc
			}(),
			expectedErrorList:   field.ErrorList{},
			expectedErrorString: "",
		},
		{
			name: "unsupported type of client broadcast address",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.ExposeOptions = &scyllav1alpha1.ScyllaDBClusterExposeOptions{
					BroadcastOptions: &scyllav1alpha1.ScyllaDBClusterNodeBroadcastOptions{
						Nodes: scyllav1alpha1.BroadcastOptions{
							Type: scyllav1alpha1.BroadcastAddressTypePodIP,
						},
						Clients: scyllav1alpha1.BroadcastOptions{
							Type: "foo",
						},
					},
				}

				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeNotSupported, Field: "spec.exposeOptions.broadcastOptions.clients.type", BadValue: scyllav1alpha1.BroadcastAddressType("foo"), Detail: `supported values: "PodIP", "ServiceLoadBalancerIngress"`},
			},
			expectedErrorString: `spec.exposeOptions.broadcastOptions.clients.type: Unsupported value: "foo": supported values: "PodIP", "ServiceLoadBalancerIngress"`,
		},
		{
			name: "unsupported type of node broadcast address",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.ExposeOptions = &scyllav1alpha1.ScyllaDBClusterExposeOptions{
					BroadcastOptions: &scyllav1alpha1.ScyllaDBClusterNodeBroadcastOptions{
						Nodes: scyllav1alpha1.BroadcastOptions{
							Type: "foo",
						},
						Clients: scyllav1alpha1.BroadcastOptions{
							Type: scyllav1alpha1.BroadcastAddressTypePodIP,
						},
					},
				}

				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeNotSupported, Field: "spec.exposeOptions.broadcastOptions.nodes.type", BadValue: scyllav1alpha1.BroadcastAddressType("foo"), Detail: `supported values: "PodIP", "ServiceLoadBalancerIngress"`},
			},
			expectedErrorString: `spec.exposeOptions.broadcastOptions.nodes.type: Unsupported value: "foo": supported values: "PodIP", "ServiceLoadBalancerIngress"`,
		},
		{
			name: "invalid LoadBalancerIngressIP broadcast type when node service is Headless",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.ExposeOptions = &scyllav1alpha1.ScyllaDBClusterExposeOptions{
					NodeService: &scyllav1alpha1.NodeServiceTemplate{
						Type: scyllav1alpha1.NodeServiceTypeHeadless,
					},
					BroadcastOptions: &scyllav1alpha1.ScyllaDBClusterNodeBroadcastOptions{
						Clients: scyllav1alpha1.BroadcastOptions{
							Type: scyllav1alpha1.BroadcastAddressTypeServiceLoadBalancerIngress,
						},
						Nodes: scyllav1alpha1.BroadcastOptions{
							Type: scyllav1alpha1.BroadcastAddressTypeServiceLoadBalancerIngress,
						},
					},
				}

				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.exposeOptions.broadcastOptions.clients.type", BadValue: scyllav1alpha1.BroadcastAddressTypeServiceLoadBalancerIngress, Detail: `can't broadcast address unavailable within the selected node service type, allowed types for chosen broadcast address type are: [LoadBalancer]`},
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.exposeOptions.broadcastOptions.nodes.type", BadValue: scyllav1alpha1.BroadcastAddressTypeServiceLoadBalancerIngress, Detail: `can't broadcast address unavailable within the selected node service type, allowed types for chosen broadcast address type are: [LoadBalancer]`},
			},
			expectedErrorString: `[spec.exposeOptions.broadcastOptions.clients.type: Invalid value: "ServiceLoadBalancerIngress": can't broadcast address unavailable within the selected node service type, allowed types for chosen broadcast address type are: [LoadBalancer], spec.exposeOptions.broadcastOptions.nodes.type: Invalid value: "ServiceLoadBalancerIngress": can't broadcast address unavailable within the selected node service type, allowed types for chosen broadcast address type are: [LoadBalancer]]`,
		},
		{
			name: "negative minTerminationGracePeriodSeconds",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.MinTerminationGracePeriodSeconds = pointer.Ptr(int32(-42))

				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.minTerminationGracePeriodSeconds", BadValue: int64(-42), Detail: "must be greater than or equal to 0"},
			},
			expectedErrorString: `spec.minTerminationGracePeriodSeconds: Invalid value: -42: must be greater than or equal to 0`,
		},
		{
			name: "negative minReadySeconds",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.MinReadySeconds = pointer.Ptr(int32(-42))

				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.minReadySeconds", BadValue: int64(-42), Detail: "must be greater than or equal to 0"},
			},
			expectedErrorString: `spec.minReadySeconds: Invalid value: -42: must be greater than or equal to 0`,
		},
		{
			name: "minimal alternator cluster passes",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.ScyllaDB.AlternatorOptions = &scyllav1alpha1.AlternatorOptions{}
				return sc
			}(),
			expectedErrorList:   field.ErrorList{},
			expectedErrorString: "",
		},
		{
			name: "alternator cluster with user certificate",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.ScyllaDB.AlternatorOptions = &scyllav1alpha1.AlternatorOptions{
					ServingCertificate: &scyllav1alpha1.TLSCertificate{
						Type: scyllav1alpha1.TLSCertificateTypeUserManaged,
						UserManagedOptions: &scyllav1alpha1.UserManagedTLSCertificateOptions{
							SecretName: "my-tls-certificate",
						},
					},
				}
				return sc
			}(),
			expectedErrorList:   field.ErrorList{},
			expectedErrorString: "",
		},
		{
			name: "alternator cluster with invalid certificate type",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.ScyllaDB.AlternatorOptions = &scyllav1alpha1.AlternatorOptions{
					ServingCertificate: &scyllav1alpha1.TLSCertificate{
						Type: "foo",
					},
				}
				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeNotSupported, Field: "spec.scyllaDB.alternator.servingCertificate.type", BadValue: scyllav1alpha1.TLSCertificateType("foo"), Detail: `supported values: "OperatorManaged", "UserManaged"`},
			},
			expectedErrorString: `spec.scyllaDB.alternator.servingCertificate.type: Unsupported value: "foo": supported values: "OperatorManaged", "UserManaged"`,
		},
		{
			name: "alternator cluster with valid additional domains",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.ScyllaDB.AlternatorOptions = &scyllav1alpha1.AlternatorOptions{
					ServingCertificate: &scyllav1alpha1.TLSCertificate{
						Type: scyllav1alpha1.TLSCertificateTypeOperatorManaged,
						OperatorManagedOptions: &scyllav1alpha1.OperatorManagedTLSCertificateOptions{
							AdditionalDNSNames: []string{"scylla-operator.scylladb.com"},
						},
					},
				}
				return sc
			}(),
			expectedErrorList:   field.ErrorList{},
			expectedErrorString: "",
		},
		{
			name: "alternator cluster with invalid additional domains",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.ScyllaDB.AlternatorOptions = &scyllav1alpha1.AlternatorOptions{
					ServingCertificate: &scyllav1alpha1.TLSCertificate{
						Type: scyllav1alpha1.TLSCertificateTypeOperatorManaged,
						OperatorManagedOptions: &scyllav1alpha1.OperatorManagedTLSCertificateOptions{
							AdditionalDNSNames: []string{"[not a domain]"},
						},
					},
				}
				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.scyllaDB.alternator.servingCertificate.operatorManagedOptions.additionalDNSNames", BadValue: []string{"[not a domain]"}, Detail: `a lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character (e.g. 'example.com', regex used for validation is '[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*')`},
			},
			expectedErrorString: `spec.scyllaDB.alternator.servingCertificate.operatorManagedOptions.additionalDNSNames: Invalid value: []string{"[not a domain]"}: a lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character (e.g. 'example.com', regex used for validation is '[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*')`,
		},
		{
			name: "alternator cluster with valid additional IP addresses",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.ScyllaDB.AlternatorOptions = &scyllav1alpha1.AlternatorOptions{
					ServingCertificate: &scyllav1alpha1.TLSCertificate{
						Type: scyllav1alpha1.TLSCertificateTypeOperatorManaged,
						OperatorManagedOptions: &scyllav1alpha1.OperatorManagedTLSCertificateOptions{
							AdditionalIPAddresses: []string{"127.0.0.1", "::1"},
						},
					},
				}
				return sc
			}(),
			expectedErrorList:   field.ErrorList{},
			expectedErrorString: "",
		},
		{
			name: "alternator cluster with invalid additional IP addresses",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.ScyllaDB.AlternatorOptions = &scyllav1alpha1.AlternatorOptions{
					ServingCertificate: &scyllav1alpha1.TLSCertificate{
						Type: scyllav1alpha1.TLSCertificateTypeOperatorManaged,
						OperatorManagedOptions: &scyllav1alpha1.OperatorManagedTLSCertificateOptions{
							AdditionalIPAddresses: []string{"0.not-an-ip.0.0"},
						},
					},
				}
				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.scyllaDB.alternator.servingCertificate.operatorManagedOptions.additionalIPAddresses", BadValue: []string{"0.not-an-ip.0.0"}, Detail: `must be a valid IP address, (e.g. 10.9.8.7 or 2001:db8::ffff)`},
			},
			expectedErrorString: `spec.scyllaDB.alternator.servingCertificate.operatorManagedOptions.additionalIPAddresses: Invalid value: []string{"0.not-an-ip.0.0"}: must be a valid IP address, (e.g. 10.9.8.7 or 2001:db8::ffff)`,
		},
		{
			name: "negative rackTemplate nodes in datacenter template",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.DatacenterTemplate.RackTemplate = &scyllav1alpha1.RackTemplate{
					Nodes: pointer.Ptr[int32](-42),
				}

				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenterTemplate.rackTemplate.nodes", BadValue: int64(-42), Detail: "must be greater than or equal to 0"},
			},
			expectedErrorString: `spec.datacenterTemplate.rackTemplate.nodes: Invalid value: -42: must be greater than or equal to 0`,
		},
		{
			name: "negative rack nodes in datacenter template",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.DatacenterTemplate.Racks[0].Nodes = pointer.Ptr[int32](-42)

				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenterTemplate.racks[0].nodes", BadValue: int64(-42), Detail: "must be greater than or equal to 0"},
			},
			expectedErrorString: `spec.datacenterTemplate.racks[0].nodes: Invalid value: -42: must be greater than or equal to 0`,
		},
		{
			name: "invalid topologyLabelSelector in datacenter template",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.DatacenterTemplate.TopologyLabelSelector = map[string]string{
					"-123": "*321",
				}

				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenterTemplate.topologyLabelSelector", BadValue: "-123", Detail: `name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]')`},
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenterTemplate.topologyLabelSelector", BadValue: "*321", Detail: `a valid label must be an empty string or consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyValue',  or 'my_value',  or '12345', regex used for validation is '(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])?')`},
			},
			expectedErrorString: `[spec.datacenterTemplate.topologyLabelSelector: Invalid value: "-123": name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]'), spec.datacenterTemplate.topologyLabelSelector: Invalid value: "*321": a valid label must be an empty string or consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyValue',  or 'my_value',  or '12345', regex used for validation is '(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])?')]`,
		},
		{
			name: "invalid topologyLabelSelector in datacenter template rackTemplate",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.DatacenterTemplate.RackTemplate = &scyllav1alpha1.RackTemplate{
					TopologyLabelSelector: map[string]string{
						"-123": "*321",
					},
				}

				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenterTemplate.rackTemplate.topologyLabelSelector", BadValue: "-123", Detail: `name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]')`},
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenterTemplate.rackTemplate.topologyLabelSelector", BadValue: "*321", Detail: `a valid label must be an empty string or consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyValue',  or 'my_value',  or '12345', regex used for validation is '(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])?')`},
			},
			expectedErrorString: `[spec.datacenterTemplate.rackTemplate.topologyLabelSelector: Invalid value: "-123": name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]'), spec.datacenterTemplate.rackTemplate.topologyLabelSelector: Invalid value: "*321": a valid label must be an empty string or consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyValue',  or 'my_value',  or '12345', regex used for validation is '(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])?')]`,
		},
		{
			name: "invalid topologyLabelSelector in datacenter template rack",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.DatacenterTemplate.Racks[0].TopologyLabelSelector = map[string]string{
					"-123": "*321",
				}

				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenterTemplate.racks[0].topologyLabelSelector", BadValue: "-123", Detail: `name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]')`},
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenterTemplate.racks[0].topologyLabelSelector", BadValue: "*321", Detail: `a valid label must be an empty string or consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyValue',  or 'my_value',  or '12345', regex used for validation is '(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])?')`},
			},
			expectedErrorString: `[spec.datacenterTemplate.racks[0].topologyLabelSelector: Invalid value: "-123": name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]'), spec.datacenterTemplate.racks[0].topologyLabelSelector: Invalid value: "*321": a valid label must be an empty string or consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyValue',  or 'my_value',  or '12345', regex used for validation is '(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])?')]`,
		},
		{
			name: "invalid scyllaDB storage capacity in datacenter template",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.DatacenterTemplate.ScyllaDB = &scyllav1alpha1.ScyllaDBTemplate{
					Storage: &scyllav1alpha1.StorageOptions{
						Capacity: "hello",
					},
				}

				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenterTemplate.scyllaDB.storage.capacity", BadValue: "hello", Detail: `unable to parse capacity: quantities must match the regular expression '^([+-]?[0-9.]+)([eEinumkKMGTP]*[-+]?[0-9]*)$'`},
			},
			expectedErrorString: `spec.datacenterTemplate.scyllaDB.storage.capacity: Invalid value: "hello": unable to parse capacity: quantities must match the regular expression '^([+-]?[0-9.]+)([eEinumkKMGTP]*[-+]?[0-9]*)$'`,
		},
		{
			name: "invalid scyllaDB storage capacity in datacenter template rackTemplate",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.DatacenterTemplate.RackTemplate = &scyllav1alpha1.RackTemplate{
					ScyllaDB: &scyllav1alpha1.ScyllaDBTemplate{
						Storage: &scyllav1alpha1.StorageOptions{
							Capacity: "hello",
						},
					},
				}

				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenterTemplate.rackTemplate.scyllaDB.storage.capacity", BadValue: "hello", Detail: `unable to parse capacity: quantities must match the regular expression '^([+-]?[0-9.]+)([eEinumkKMGTP]*[-+]?[0-9]*)$'`},
			},
			expectedErrorString: `spec.datacenterTemplate.rackTemplate.scyllaDB.storage.capacity: Invalid value: "hello": unable to parse capacity: quantities must match the regular expression '^([+-]?[0-9.]+)([eEinumkKMGTP]*[-+]?[0-9]*)$'`,
		},
		{
			name: "invalid scyllaDB storage capacity in datacenter template rack",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.DatacenterTemplate.Racks[0].ScyllaDB = &scyllav1alpha1.ScyllaDBTemplate{
					Storage: &scyllav1alpha1.StorageOptions{
						Capacity: "hello",
					},
				}

				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenterTemplate.racks[0].scyllaDB.storage.capacity", BadValue: "hello", Detail: `unable to parse capacity: quantities must match the regular expression '^([+-]?[0-9.]+)([eEinumkKMGTP]*[-+]?[0-9]*)$'`},
			},
			expectedErrorString: `spec.datacenterTemplate.racks[0].scyllaDB.storage.capacity: Invalid value: "hello": unable to parse capacity: quantities must match the regular expression '^([+-]?[0-9.]+)([eEinumkKMGTP]*[-+]?[0-9]*)$'`,
		},
		{
			name: "invalid negative scyllaDB storage capacity in datacenter template",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.DatacenterTemplate.ScyllaDB = &scyllav1alpha1.ScyllaDBTemplate{
					Storage: &scyllav1alpha1.StorageOptions{
						Capacity: "-1Gi",
					},
				}

				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenterTemplate.scyllaDB.storage.capacity", BadValue: "-1Gi", Detail: `must be greater than zero`},
			},
			expectedErrorString: `spec.datacenterTemplate.scyllaDB.storage.capacity: Invalid value: "-1Gi": must be greater than zero`,
		},
		{
			name: "invalid negative scyllaDB storage capacity in datacenter template rackTemplate",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.DatacenterTemplate.RackTemplate = &scyllav1alpha1.RackTemplate{
					ScyllaDB: &scyllav1alpha1.ScyllaDBTemplate{
						Storage: &scyllav1alpha1.StorageOptions{
							Capacity: "-1Gi",
						},
					},
				}

				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenterTemplate.rackTemplate.scyllaDB.storage.capacity", BadValue: "-1Gi", Detail: `must be greater than zero`},
			},
			expectedErrorString: `spec.datacenterTemplate.rackTemplate.scyllaDB.storage.capacity: Invalid value: "-1Gi": must be greater than zero`,
		},
		{
			name: "invalid negative scyllaDB storage capacity in datacenter template rack",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.DatacenterTemplate.Racks[0].ScyllaDB = &scyllav1alpha1.ScyllaDBTemplate{
					Storage: &scyllav1alpha1.StorageOptions{
						Capacity: "-1Gi",
					},
				}

				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenterTemplate.racks[0].scyllaDB.storage.capacity", BadValue: "-1Gi", Detail: `must be greater than zero`},
			},
			expectedErrorString: `spec.datacenterTemplate.racks[0].scyllaDB.storage.capacity: Invalid value: "-1Gi": must be greater than zero`,
		},
		{
			name: "invalid labels in datacenter template scyllaDB storage metadata",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.DatacenterTemplate.ScyllaDB = &scyllav1alpha1.ScyllaDBTemplate{
					Storage: &scyllav1alpha1.StorageOptions{
						Capacity: "1Gi",
						Metadata: &scyllav1alpha1.ObjectTemplateMetadata{
							Labels: map[string]string{
								"-123": "*321",
							},
						},
					},
				}

				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenterTemplate.scyllaDB.storage.metadata.labels", BadValue: "-123", Detail: `name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]')`},
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenterTemplate.scyllaDB.storage.metadata.labels", BadValue: "*321", Detail: `a valid label must be an empty string or consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyValue',  or 'my_value',  or '12345', regex used for validation is '(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])?')`},
			},
			expectedErrorString: `[spec.datacenterTemplate.scyllaDB.storage.metadata.labels: Invalid value: "-123": name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]'), spec.datacenterTemplate.scyllaDB.storage.metadata.labels: Invalid value: "*321": a valid label must be an empty string or consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyValue',  or 'my_value',  or '12345', regex used for validation is '(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])?')]`,
		},
		{
			name: "invalid labels in datacenter template rackTemplate scyllaDB storage metadata",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.DatacenterTemplate.RackTemplate = &scyllav1alpha1.RackTemplate{
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

				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenterTemplate.rackTemplate.scyllaDB.storage.metadata.labels", BadValue: "-123", Detail: `name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]')`},
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenterTemplate.rackTemplate.scyllaDB.storage.metadata.labels", BadValue: "*321", Detail: `a valid label must be an empty string or consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyValue',  or 'my_value',  or '12345', regex used for validation is '(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])?')`},
			},
			expectedErrorString: `[spec.datacenterTemplate.rackTemplate.scyllaDB.storage.metadata.labels: Invalid value: "-123": name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]'), spec.datacenterTemplate.rackTemplate.scyllaDB.storage.metadata.labels: Invalid value: "*321": a valid label must be an empty string or consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyValue',  or 'my_value',  or '12345', regex used for validation is '(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])?')]`,
		},
		{
			name: "invalid labels in datacenter template rack scyllaDB storage metadata",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.DatacenterTemplate.Racks[0].ScyllaDB = &scyllav1alpha1.ScyllaDBTemplate{
					Storage: &scyllav1alpha1.StorageOptions{
						Capacity: "1Gi",
						Metadata: &scyllav1alpha1.ObjectTemplateMetadata{
							Labels: map[string]string{
								"-123": "*321",
							},
						},
					},
				}

				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenterTemplate.racks[0].scyllaDB.storage.metadata.labels", BadValue: "-123", Detail: `name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]')`},
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenterTemplate.racks[0].scyllaDB.storage.metadata.labels", BadValue: "*321", Detail: `a valid label must be an empty string or consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyValue',  or 'my_value',  or '12345', regex used for validation is '(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])?')`},
			},
			expectedErrorString: `[spec.datacenterTemplate.racks[0].scyllaDB.storage.metadata.labels: Invalid value: "-123": name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]'), spec.datacenterTemplate.racks[0].scyllaDB.storage.metadata.labels: Invalid value: "*321": a valid label must be an empty string or consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyValue',  or 'my_value',  or '12345', regex used for validation is '(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])?')]`,
		},
		{
			name: "invalid annotations in datacenter template scyllaDB storage metadata",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.DatacenterTemplate.ScyllaDB = &scyllav1alpha1.ScyllaDBTemplate{
					Storage: &scyllav1alpha1.StorageOptions{
						Capacity: "1Gi",
						Metadata: &scyllav1alpha1.ObjectTemplateMetadata{
							Annotations: map[string]string{
								"-123": "*321",
							},
						},
					},
				}

				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenterTemplate.scyllaDB.storage.metadata.annotations", BadValue: "-123", Detail: `name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]')`},
			},
			expectedErrorString: `spec.datacenterTemplate.scyllaDB.storage.metadata.annotations: Invalid value: "-123": name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]')`,
		},
		{
			name: "invalid annotations in datacenter template rackTemplate scyllaDB storage metadata",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.DatacenterTemplate.RackTemplate = &scyllav1alpha1.RackTemplate{
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

				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenterTemplate.rackTemplate.scyllaDB.storage.metadata.annotations", BadValue: "-123", Detail: `name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]')`},
			},
			expectedErrorString: `spec.datacenterTemplate.rackTemplate.scyllaDB.storage.metadata.annotations: Invalid value: "-123": name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]')`,
		},
		{
			name: "invalid annotations in datacenter template rack scyllaDB storage metadata",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.DatacenterTemplate.Racks[0].ScyllaDB = &scyllav1alpha1.ScyllaDBTemplate{
					Storage: &scyllav1alpha1.StorageOptions{
						Capacity: "1Gi",
						Metadata: &scyllav1alpha1.ObjectTemplateMetadata{
							Annotations: map[string]string{
								"-123": "*321",
							},
						},
					},
				}

				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenterTemplate.racks[0].scyllaDB.storage.metadata.annotations", BadValue: "-123", Detail: `name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]')`},
			},
			expectedErrorString: `spec.datacenterTemplate.racks[0].scyllaDB.storage.metadata.annotations: Invalid value: "-123": name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]')`,
		},
		{
			name: "invalid storageClassName in datacenter template scyllaDB storage",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.DatacenterTemplate.ScyllaDB = &scyllav1alpha1.ScyllaDBTemplate{
					Storage: &scyllav1alpha1.StorageOptions{
						Capacity:         "1Gi",
						StorageClassName: pointer.Ptr("-hello"),
					},
				}

				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenterTemplate.scyllaDB.storage.storageClassName", BadValue: "-hello", Detail: `a lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character (e.g. 'example.com', regex used for validation is '[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*')`},
			},
			expectedErrorString: `spec.datacenterTemplate.scyllaDB.storage.storageClassName: Invalid value: "-hello": a lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character (e.g. 'example.com', regex used for validation is '[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*')`,
		},
		{
			name: "invalid storageClassName in datacenter template rackTemplate scyllaDB storage",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.DatacenterTemplate.RackTemplate = &scyllav1alpha1.RackTemplate{
					ScyllaDB: &scyllav1alpha1.ScyllaDBTemplate{
						Storage: &scyllav1alpha1.StorageOptions{
							Capacity:         "1Gi",
							StorageClassName: pointer.Ptr("-hello"),
						},
					},
				}

				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenterTemplate.rackTemplate.scyllaDB.storage.storageClassName", BadValue: "-hello", Detail: `a lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character (e.g. 'example.com', regex used for validation is '[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*')`},
			},
			expectedErrorString: `spec.datacenterTemplate.rackTemplate.scyllaDB.storage.storageClassName: Invalid value: "-hello": a lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character (e.g. 'example.com', regex used for validation is '[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*')`,
		},
		{
			name: "invalid storageClassName in datacenter template rack scyllaDB storage",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.DatacenterTemplate.Racks[0].ScyllaDB = &scyllav1alpha1.ScyllaDBTemplate{
					Storage: &scyllav1alpha1.StorageOptions{
						Capacity:         "1Gi",
						StorageClassName: pointer.Ptr("-hello"),
					},
				}

				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenterTemplate.racks[0].scyllaDB.storage.storageClassName", BadValue: "-hello", Detail: `a lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character (e.g. 'example.com', regex used for validation is '[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*')`},
			},
			expectedErrorString: `spec.datacenterTemplate.racks[0].scyllaDB.storage.storageClassName: Invalid value: "-hello": a lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character (e.g. 'example.com', regex used for validation is '[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*')`,
		},
		{
			name: "invalid customConfigMapRef in scyllaDB template of datacenter template",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.DatacenterTemplate.ScyllaDB = &scyllav1alpha1.ScyllaDBTemplate{
					CustomConfigMapRef: pointer.Ptr("-hello"),
				}

				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenterTemplate.scyllaDB.customConfigMapRef", BadValue: "-hello", Detail: `a lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character (e.g. 'example.com', regex used for validation is '[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*')`},
			},
			expectedErrorString: `spec.datacenterTemplate.scyllaDB.customConfigMapRef: Invalid value: "-hello": a lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character (e.g. 'example.com', regex used for validation is '[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*')`,
		},
		{
			name: "invalid customConfigMapRef in scyllaDB template of datacenter template rackTemplate",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.DatacenterTemplate.RackTemplate = &scyllav1alpha1.RackTemplate{
					ScyllaDB: &scyllav1alpha1.ScyllaDBTemplate{
						CustomConfigMapRef: pointer.Ptr("-hello"),
					},
				}

				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenterTemplate.rackTemplate.scyllaDB.customConfigMapRef", BadValue: "-hello", Detail: `a lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character (e.g. 'example.com', regex used for validation is '[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*')`},
			},
			expectedErrorString: `spec.datacenterTemplate.rackTemplate.scyllaDB.customConfigMapRef: Invalid value: "-hello": a lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character (e.g. 'example.com', regex used for validation is '[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*')`,
		},
		{
			name: "invalid customConfigMapRef in scyllaDB template of datacenter template rack",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.DatacenterTemplate.Racks[0].ScyllaDB = &scyllav1alpha1.ScyllaDBTemplate{
					CustomConfigMapRef: pointer.Ptr("-hello"),
				}

				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenterTemplate.racks[0].scyllaDB.customConfigMapRef", BadValue: "-hello", Detail: `a lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character (e.g. 'example.com', regex used for validation is '[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*')`},
			},
			expectedErrorString: `spec.datacenterTemplate.racks[0].scyllaDB.customConfigMapRef: Invalid value: "-hello": a lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character (e.g. 'example.com', regex used for validation is '[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*')`,
		},
		{
			name: "invalid customConfigSecretRef of ScyllaDB Manager Agent in datacenter template",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.DatacenterTemplate.ScyllaDBManagerAgent = &scyllav1alpha1.ScyllaDBManagerAgentTemplate{
					CustomConfigSecretRef: pointer.Ptr("-hello"),
				}

				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenterTemplate.scyllaDBManagerAgent.customConfigSecretRef", BadValue: "-hello", Detail: `a lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character (e.g. 'example.com', regex used for validation is '[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*')`},
			},
			expectedErrorString: `spec.datacenterTemplate.scyllaDBManagerAgent.customConfigSecretRef: Invalid value: "-hello": a lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character (e.g. 'example.com', regex used for validation is '[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*')`,
		},
		{
			name: "invalid customConfigSecretRef of ScyllaDB Manager Agent in datacenter template rackTemplate",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.DatacenterTemplate.RackTemplate = &scyllav1alpha1.RackTemplate{
					ScyllaDBManagerAgent: &scyllav1alpha1.ScyllaDBManagerAgentTemplate{
						CustomConfigSecretRef: pointer.Ptr("-hello"),
					},
				}

				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenterTemplate.rackTemplate.scyllaDBManagerAgent.customConfigSecretRef", BadValue: "-hello", Detail: `a lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character (e.g. 'example.com', regex used for validation is '[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*')`},
			},
			expectedErrorString: `spec.datacenterTemplate.rackTemplate.scyllaDBManagerAgent.customConfigSecretRef: Invalid value: "-hello": a lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character (e.g. 'example.com', regex used for validation is '[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*')`,
		},
		{
			name: "invalid customConfigSecretRef of ScyllaDB Manager Agent in datacenter template rack",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.DatacenterTemplate.Racks[0].ScyllaDBManagerAgent = &scyllav1alpha1.ScyllaDBManagerAgentTemplate{
					CustomConfigSecretRef: pointer.Ptr("-hello"),
				}

				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenterTemplate.racks[0].scyllaDBManagerAgent.customConfigSecretRef", BadValue: "-hello", Detail: `a lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character (e.g. 'example.com', regex used for validation is '[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*')`},
			},
			expectedErrorString: `spec.datacenterTemplate.racks[0].scyllaDBManagerAgent.customConfigSecretRef: Invalid value: "-hello": a lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character (e.g. 'example.com', regex used for validation is '[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*')`,
		},
		{
			name: "negative rackTemplate nodes",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.Datacenters[0].RackTemplate = &scyllav1alpha1.RackTemplate{
					Nodes: pointer.Ptr[int32](-42),
				}

				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenters[0].rackTemplate.nodes", BadValue: int64(-42), Detail: "must be greater than or equal to 0"},
			},
			expectedErrorString: `spec.datacenters[0].rackTemplate.nodes: Invalid value: -42: must be greater than or equal to 0`,
		},
		{
			name: "negative rack nodes",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.Datacenters[0].Racks[0].Nodes = pointer.Ptr[int32](-42)

				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenters[0].racks[0].nodes", BadValue: int64(-42), Detail: "must be greater than or equal to 0"},
			},
			expectedErrorString: `spec.datacenters[0].racks[0].nodes: Invalid value: -42: must be greater than or equal to 0`,
		},
		{
			name: "invalid topologyLabelSelector in datacenter",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.Datacenters[0].TopologyLabelSelector = map[string]string{
					"-123": "*321",
				}

				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenters[0].topologyLabelSelector", BadValue: "-123", Detail: `name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]')`},
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenters[0].topologyLabelSelector", BadValue: "*321", Detail: `a valid label must be an empty string or consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyValue',  or 'my_value',  or '12345', regex used for validation is '(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])?')`},
			},
			expectedErrorString: `[spec.datacenters[0].topologyLabelSelector: Invalid value: "-123": name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]'), spec.datacenters[0].topologyLabelSelector: Invalid value: "*321": a valid label must be an empty string or consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyValue',  or 'my_value',  or '12345', regex used for validation is '(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])?')]`,
		},
		{
			name: "invalid topologyLabelSelector in rackTemplate",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.Datacenters[0].RackTemplate = &scyllav1alpha1.RackTemplate{
					TopologyLabelSelector: map[string]string{
						"-123": "*321",
					},
				}

				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenters[0].rackTemplate.topologyLabelSelector", BadValue: "-123", Detail: `name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]')`},
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenters[0].rackTemplate.topologyLabelSelector", BadValue: "*321", Detail: `a valid label must be an empty string or consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyValue',  or 'my_value',  or '12345', regex used for validation is '(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])?')`},
			},
			expectedErrorString: `[spec.datacenters[0].rackTemplate.topologyLabelSelector: Invalid value: "-123": name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]'), spec.datacenters[0].rackTemplate.topologyLabelSelector: Invalid value: "*321": a valid label must be an empty string or consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyValue',  or 'my_value',  or '12345', regex used for validation is '(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])?')]`,
		},
		{
			name: "invalid topologyLabelSelector in rack",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.Datacenters[0].Racks[0].TopologyLabelSelector = map[string]string{
					"-123": "*321",
				}

				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenters[0].racks[0].topologyLabelSelector", BadValue: "-123", Detail: `name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]')`},
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenters[0].racks[0].topologyLabelSelector", BadValue: "*321", Detail: `a valid label must be an empty string or consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyValue',  or 'my_value',  or '12345', regex used for validation is '(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])?')`},
			},
			expectedErrorString: `[spec.datacenters[0].racks[0].topologyLabelSelector: Invalid value: "-123": name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]'), spec.datacenters[0].racks[0].topologyLabelSelector: Invalid value: "*321": a valid label must be an empty string or consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyValue',  or 'my_value',  or '12345', regex used for validation is '(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])?')]`,
		},
		{
			name: "invalid scyllaDB storage capacity in datacenter",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.Datacenters[0].ScyllaDB = &scyllav1alpha1.ScyllaDBTemplate{
					Storage: &scyllav1alpha1.StorageOptions{
						Capacity: "hello",
					},
				}

				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenters[0].scyllaDB.storage.capacity", BadValue: "hello", Detail: `unable to parse capacity: quantities must match the regular expression '^([+-]?[0-9.]+)([eEinumkKMGTP]*[-+]?[0-9]*)$'`},
			},
			expectedErrorString: `spec.datacenters[0].scyllaDB.storage.capacity: Invalid value: "hello": unable to parse capacity: quantities must match the regular expression '^([+-]?[0-9.]+)([eEinumkKMGTP]*[-+]?[0-9]*)$'`,
		},
		{
			name: "invalid scyllaDB storage capacity in rackTemplate",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.Datacenters[0].RackTemplate = &scyllav1alpha1.RackTemplate{
					ScyllaDB: &scyllav1alpha1.ScyllaDBTemplate{
						Storage: &scyllav1alpha1.StorageOptions{
							Capacity: "hello",
						},
					},
				}

				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenters[0].rackTemplate.scyllaDB.storage.capacity", BadValue: "hello", Detail: `unable to parse capacity: quantities must match the regular expression '^([+-]?[0-9.]+)([eEinumkKMGTP]*[-+]?[0-9]*)$'`},
			},
			expectedErrorString: `spec.datacenters[0].rackTemplate.scyllaDB.storage.capacity: Invalid value: "hello": unable to parse capacity: quantities must match the regular expression '^([+-]?[0-9.]+)([eEinumkKMGTP]*[-+]?[0-9]*)$'`,
		},
		{
			name: "invalid scyllaDB storage capacity in rack",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.Datacenters[0].Racks[0].ScyllaDB = &scyllav1alpha1.ScyllaDBTemplate{
					Storage: &scyllav1alpha1.StorageOptions{
						Capacity: "hello",
					},
				}

				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenters[0].racks[0].scyllaDB.storage.capacity", BadValue: "hello", Detail: `unable to parse capacity: quantities must match the regular expression '^([+-]?[0-9.]+)([eEinumkKMGTP]*[-+]?[0-9]*)$'`},
			},
			expectedErrorString: `spec.datacenters[0].racks[0].scyllaDB.storage.capacity: Invalid value: "hello": unable to parse capacity: quantities must match the regular expression '^([+-]?[0-9.]+)([eEinumkKMGTP]*[-+]?[0-9]*)$'`,
		},
		{
			name: "invalid negative scyllaDB storage capacity in datacenter",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.Datacenters[0].ScyllaDB = &scyllav1alpha1.ScyllaDBTemplate{
					Storage: &scyllav1alpha1.StorageOptions{
						Capacity: "-1Gi",
					},
				}

				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenters[0].scyllaDB.storage.capacity", BadValue: "-1Gi", Detail: `must be greater than zero`},
			},
			expectedErrorString: `spec.datacenters[0].scyllaDB.storage.capacity: Invalid value: "-1Gi": must be greater than zero`,
		},
		{
			name: "invalid negative scyllaDB storage capacity in rackTemplate",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.Datacenters[0].RackTemplate = &scyllav1alpha1.RackTemplate{
					ScyllaDB: &scyllav1alpha1.ScyllaDBTemplate{
						Storage: &scyllav1alpha1.StorageOptions{
							Capacity: "-1Gi",
						},
					},
				}

				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenters[0].rackTemplate.scyllaDB.storage.capacity", BadValue: "-1Gi", Detail: `must be greater than zero`},
			},
			expectedErrorString: `spec.datacenters[0].rackTemplate.scyllaDB.storage.capacity: Invalid value: "-1Gi": must be greater than zero`,
		},
		{
			name: "invalid negative scyllaDB storage capacity in rack",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.Datacenters[0].Racks[0].ScyllaDB = &scyllav1alpha1.ScyllaDBTemplate{
					Storage: &scyllav1alpha1.StorageOptions{
						Capacity: "-1Gi",
					},
				}

				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenters[0].racks[0].scyllaDB.storage.capacity", BadValue: "-1Gi", Detail: `must be greater than zero`},
			},
			expectedErrorString: `spec.datacenters[0].racks[0].scyllaDB.storage.capacity: Invalid value: "-1Gi": must be greater than zero`,
		},
		{
			name: "invalid labels in datacenter scyllaDB storage metadata",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.Datacenters[0].ScyllaDB = &scyllav1alpha1.ScyllaDBTemplate{
					Storage: &scyllav1alpha1.StorageOptions{
						Capacity: "1Gi",
						Metadata: &scyllav1alpha1.ObjectTemplateMetadata{
							Labels: map[string]string{
								"-123": "*321",
							},
						},
					},
				}

				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenters[0].scyllaDB.storage.metadata.labels", BadValue: "-123", Detail: `name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]')`},
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenters[0].scyllaDB.storage.metadata.labels", BadValue: "*321", Detail: `a valid label must be an empty string or consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyValue',  or 'my_value',  or '12345', regex used for validation is '(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])?')`},
			},
			expectedErrorString: `[spec.datacenters[0].scyllaDB.storage.metadata.labels: Invalid value: "-123": name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]'), spec.datacenters[0].scyllaDB.storage.metadata.labels: Invalid value: "*321": a valid label must be an empty string or consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyValue',  or 'my_value',  or '12345', regex used for validation is '(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])?')]`,
		},
		{
			name: "invalid labels in rackTemplate scyllaDB storage metadata",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.Datacenters[0].RackTemplate = &scyllav1alpha1.RackTemplate{
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

				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenters[0].rackTemplate.scyllaDB.storage.metadata.labels", BadValue: "-123", Detail: `name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]')`},
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenters[0].rackTemplate.scyllaDB.storage.metadata.labels", BadValue: "*321", Detail: `a valid label must be an empty string or consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyValue',  or 'my_value',  or '12345', regex used for validation is '(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])?')`},
			},
			expectedErrorString: `[spec.datacenters[0].rackTemplate.scyllaDB.storage.metadata.labels: Invalid value: "-123": name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]'), spec.datacenters[0].rackTemplate.scyllaDB.storage.metadata.labels: Invalid value: "*321": a valid label must be an empty string or consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyValue',  or 'my_value',  or '12345', regex used for validation is '(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])?')]`,
		},
		{
			name: "invalid labels in rack scyllaDB storage metadata",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.Datacenters[0].Racks[0].ScyllaDB = &scyllav1alpha1.ScyllaDBTemplate{
					Storage: &scyllav1alpha1.StorageOptions{
						Capacity: "1Gi",
						Metadata: &scyllav1alpha1.ObjectTemplateMetadata{
							Labels: map[string]string{
								"-123": "*321",
							},
						},
					},
				}

				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenters[0].racks[0].scyllaDB.storage.metadata.labels", BadValue: "-123", Detail: `name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]')`},
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenters[0].racks[0].scyllaDB.storage.metadata.labels", BadValue: "*321", Detail: `a valid label must be an empty string or consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyValue',  or 'my_value',  or '12345', regex used for validation is '(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])?')`},
			},
			expectedErrorString: `[spec.datacenters[0].racks[0].scyllaDB.storage.metadata.labels: Invalid value: "-123": name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]'), spec.datacenters[0].racks[0].scyllaDB.storage.metadata.labels: Invalid value: "*321": a valid label must be an empty string or consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyValue',  or 'my_value',  or '12345', regex used for validation is '(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])?')]`,
		},
		{
			name: "invalid annotations in datacenter scyllaDB storage metadata",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.Datacenters[0].ScyllaDB = &scyllav1alpha1.ScyllaDBTemplate{
					Storage: &scyllav1alpha1.StorageOptions{
						Capacity: "1Gi",
						Metadata: &scyllav1alpha1.ObjectTemplateMetadata{
							Annotations: map[string]string{
								"-123": "*321",
							},
						},
					},
				}

				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenters[0].scyllaDB.storage.metadata.annotations", BadValue: "-123", Detail: `name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]')`},
			},
			expectedErrorString: `spec.datacenters[0].scyllaDB.storage.metadata.annotations: Invalid value: "-123": name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]')`,
		},
		{
			name: "invalid annotations in rackTemplate scyllaDB storage metadata",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.Datacenters[0].RackTemplate = &scyllav1alpha1.RackTemplate{
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

				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenters[0].rackTemplate.scyllaDB.storage.metadata.annotations", BadValue: "-123", Detail: `name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]')`},
			},
			expectedErrorString: `spec.datacenters[0].rackTemplate.scyllaDB.storage.metadata.annotations: Invalid value: "-123": name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]')`,
		},
		{
			name: "invalid annotations in rack scyllaDB storage metadata",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.Datacenters[0].Racks[0].ScyllaDB = &scyllav1alpha1.ScyllaDBTemplate{
					Storage: &scyllav1alpha1.StorageOptions{
						Capacity: "1Gi",
						Metadata: &scyllav1alpha1.ObjectTemplateMetadata{
							Annotations: map[string]string{
								"-123": "*321",
							},
						},
					},
				}

				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenters[0].racks[0].scyllaDB.storage.metadata.annotations", BadValue: "-123", Detail: `name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]')`},
			},
			expectedErrorString: `spec.datacenters[0].racks[0].scyllaDB.storage.metadata.annotations: Invalid value: "-123": name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]')`,
		},
		{
			name: "invalid storageClassName in datacenter scyllaDB storage",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.Datacenters[0].ScyllaDB = &scyllav1alpha1.ScyllaDBTemplate{
					Storage: &scyllav1alpha1.StorageOptions{
						Capacity:         "1Gi",
						StorageClassName: pointer.Ptr("-hello"),
					},
				}

				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenters[0].scyllaDB.storage.storageClassName", BadValue: "-hello", Detail: `a lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character (e.g. 'example.com', regex used for validation is '[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*')`},
			},
			expectedErrorString: `spec.datacenters[0].scyllaDB.storage.storageClassName: Invalid value: "-hello": a lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character (e.g. 'example.com', regex used for validation is '[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*')`,
		},
		{
			name: "invalid storageClassName in rackTemplate scyllaDB storage",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.Datacenters[0].RackTemplate = &scyllav1alpha1.RackTemplate{
					ScyllaDB: &scyllav1alpha1.ScyllaDBTemplate{
						Storage: &scyllav1alpha1.StorageOptions{
							Capacity:         "1Gi",
							StorageClassName: pointer.Ptr("-hello"),
						},
					},
				}

				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenters[0].rackTemplate.scyllaDB.storage.storageClassName", BadValue: "-hello", Detail: `a lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character (e.g. 'example.com', regex used for validation is '[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*')`},
			},
			expectedErrorString: `spec.datacenters[0].rackTemplate.scyllaDB.storage.storageClassName: Invalid value: "-hello": a lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character (e.g. 'example.com', regex used for validation is '[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*')`,
		},
		{
			name: "invalid storageClassName in rack scyllaDB storage",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.Datacenters[0].Racks[0].ScyllaDB = &scyllav1alpha1.ScyllaDBTemplate{
					Storage: &scyllav1alpha1.StorageOptions{
						Capacity:         "1Gi",
						StorageClassName: pointer.Ptr("-hello"),
					},
				}

				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenters[0].racks[0].scyllaDB.storage.storageClassName", BadValue: "-hello", Detail: `a lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character (e.g. 'example.com', regex used for validation is '[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*')`},
			},
			expectedErrorString: `spec.datacenters[0].racks[0].scyllaDB.storage.storageClassName: Invalid value: "-hello": a lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character (e.g. 'example.com', regex used for validation is '[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*')`,
		},
		{
			name: "invalid customConfigMapRef in scyllaDB template of datacenter",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.Datacenters[0].ScyllaDB = &scyllav1alpha1.ScyllaDBTemplate{
					CustomConfigMapRef: pointer.Ptr("-hello"),
				}

				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenters[0].scyllaDB.customConfigMapRef", BadValue: "-hello", Detail: `a lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character (e.g. 'example.com', regex used for validation is '[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*')`},
			},
			expectedErrorString: `spec.datacenters[0].scyllaDB.customConfigMapRef: Invalid value: "-hello": a lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character (e.g. 'example.com', regex used for validation is '[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*')`,
		},
		{
			name: "invalid customConfigMapRef in scyllaDB template of rackTemplate",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.Datacenters[0].RackTemplate = &scyllav1alpha1.RackTemplate{
					ScyllaDB: &scyllav1alpha1.ScyllaDBTemplate{
						CustomConfigMapRef: pointer.Ptr("-hello"),
					},
				}

				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenters[0].rackTemplate.scyllaDB.customConfigMapRef", BadValue: "-hello", Detail: `a lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character (e.g. 'example.com', regex used for validation is '[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*')`},
			},
			expectedErrorString: `spec.datacenters[0].rackTemplate.scyllaDB.customConfigMapRef: Invalid value: "-hello": a lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character (e.g. 'example.com', regex used for validation is '[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*')`,
		},
		{
			name: "invalid customConfigMapRef in scyllaDB template of rack",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.Datacenters[0].Racks[0].ScyllaDB = &scyllav1alpha1.ScyllaDBTemplate{
					CustomConfigMapRef: pointer.Ptr("-hello"),
				}

				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenters[0].racks[0].scyllaDB.customConfigMapRef", BadValue: "-hello", Detail: `a lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character (e.g. 'example.com', regex used for validation is '[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*')`},
			},
			expectedErrorString: `spec.datacenters[0].racks[0].scyllaDB.customConfigMapRef: Invalid value: "-hello": a lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character (e.g. 'example.com', regex used for validation is '[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*')`,
		},
		{
			name: "invalid customConfigSecretRef of ScyllaDB Manager Agent datacenter",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.Datacenters[0].ScyllaDBManagerAgent = &scyllav1alpha1.ScyllaDBManagerAgentTemplate{
					CustomConfigSecretRef: pointer.Ptr("-hello"),
				}

				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenters[0].scyllaDBManagerAgent.customConfigSecretRef", BadValue: "-hello", Detail: `a lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character (e.g. 'example.com', regex used for validation is '[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*')`},
			},
			expectedErrorString: `spec.datacenters[0].scyllaDBManagerAgent.customConfigSecretRef: Invalid value: "-hello": a lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character (e.g. 'example.com', regex used for validation is '[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*')`,
		},
		{
			name: "invalid customConfigSecretRef of ScyllaDB Manager Agent rackTemplate",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.Datacenters[0].RackTemplate = &scyllav1alpha1.RackTemplate{
					ScyllaDBManagerAgent: &scyllav1alpha1.ScyllaDBManagerAgentTemplate{
						CustomConfigSecretRef: pointer.Ptr("-hello"),
					},
				}

				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenters[0].rackTemplate.scyllaDBManagerAgent.customConfigSecretRef", BadValue: "-hello", Detail: `a lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character (e.g. 'example.com', regex used for validation is '[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*')`},
			},
			expectedErrorString: `spec.datacenters[0].rackTemplate.scyllaDBManagerAgent.customConfigSecretRef: Invalid value: "-hello": a lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character (e.g. 'example.com', regex used for validation is '[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*')`,
		},
		{
			name: "invalid customConfigSecretRef of ScyllaDB Manager Agent rack",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.Datacenters[0].Racks[0].ScyllaDBManagerAgent = &scyllav1alpha1.ScyllaDBManagerAgentTemplate{
					CustomConfigSecretRef: pointer.Ptr("-hello"),
				}

				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenters[0].racks[0].scyllaDBManagerAgent.customConfigSecretRef", BadValue: "-hello", Detail: `a lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character (e.g. 'example.com', regex used for validation is '[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*')`},
			},
			expectedErrorString: `spec.datacenters[0].racks[0].scyllaDBManagerAgent.customConfigSecretRef: Invalid value: "-hello": a lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character (e.g. 'example.com', regex used for validation is '[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*')`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			errList := validation.ValidateScyllaDBCluster(test.cluster)
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

func TestValidateScyllaDBClusterUpdate(t *testing.T) {
	newValidScyllaDBCluster := func() *scyllav1alpha1.ScyllaDBCluster {
		return &scyllav1alpha1.ScyllaDBCluster{
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
			Spec: scyllav1alpha1.ScyllaDBClusterSpec{
				ClusterName: pointer.Ptr("basic"),
				ScyllaDB: scyllav1alpha1.ScyllaDB{
					Image: "scylladb/scylla:latest",
				},
				ScyllaDBManagerAgent: &scyllav1alpha1.ScyllaDBManagerAgent{
					Image: pointer.Ptr("scylladb/scylla-manager-agent:latest"),
				},
				Datacenters: []scyllav1alpha1.ScyllaDBClusterDatacenter{
					{
						Name:                        "dc",
						RemoteKubernetesClusterName: "rkc",
						ScyllaDBClusterDatacenterTemplate: scyllav1alpha1.ScyllaDBClusterDatacenterTemplate{
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
					},
				},
			},
		}
	}

	tests := []struct {
		name                string
		old                 *scyllav1alpha1.ScyllaDBCluster
		new                 *scyllav1alpha1.ScyllaDBCluster
		expectedErrorList   field.ErrorList
		expectedErrorString string
	}{
		{
			name:                "same as old",
			old:                 newValidScyllaDBCluster(),
			new:                 newValidScyllaDBCluster(),
			expectedErrorList:   field.ErrorList{},
			expectedErrorString: "",
		},
		{
			name: "empty rack removed",
			old: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.Datacenters[0].Racks[0].Nodes = pointer.Ptr[int32](0)
				return sc
			}(),
			new: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.Datacenters[0].Racks = nil
				return sc
			}(),
			expectedErrorList:   field.ErrorList{},
			expectedErrorString: "",
		},
		{
			name: "empty rack with members under decommission",
			old: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Status = scyllav1alpha1.ScyllaDBClusterStatus{
					Datacenters: []scyllav1alpha1.ScyllaDBClusterDatacenterStatus{
						{
							Name: sc.Spec.Datacenters[0].Name,
							Racks: []scyllav1alpha1.ScyllaDBClusterRackStatus{
								{
									Name:  sc.Spec.Datacenters[0].Racks[0].Name,
									Nodes: pointer.Ptr[int32](3),
								},
							},
						},
					},
				}

				return sc
			}(),
			new: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.Datacenters[0].Racks = []scyllav1alpha1.RackSpec{}
				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeForbidden, Field: "spec.datacenters[0].racks[0]", BadValue: "", Detail: `rack "rack" can't be removed because the nodes are being scaled down`},
			},
			expectedErrorString: `spec.datacenters[0].racks[0]: Forbidden: rack "rack" can't be removed because the nodes are being scaled down`,
		},
		{
			name: "empty rack with stale status",
			old: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Status = scyllav1alpha1.ScyllaDBClusterStatus{
					Datacenters: []scyllav1alpha1.ScyllaDBClusterDatacenterStatus{
						{
							Name: sc.Spec.Datacenters[0].Name,
							Racks: []scyllav1alpha1.ScyllaDBClusterRackStatus{
								{
									Name:  sc.Spec.Datacenters[0].Racks[0].Name,
									Nodes: pointer.Ptr[int32](0),
									Stale: pointer.Ptr(true),
								},
							},
						},
					},
				}
				return sc
			}(),
			new: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.Datacenters[0].Racks = []scyllav1alpha1.RackSpec{}
				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInternal, Field: "spec.datacenters[0].racks[0]", Detail: `rack "rack" can't be removed because its status, that's used to determine node count, is not yet up to date with the generation of this resource; please retry later`},
			},
			expectedErrorString: `spec.datacenters[0].racks[0]: Internal error: rack "rack" can't be removed because its status, that's used to determine node count, is not yet up to date with the generation of this resource; please retry later`,
		},
		{
			name: "empty rack with not reconciled generation",
			old: func() *scyllav1alpha1.ScyllaDBCluster {
				sdc := newValidScyllaDBCluster()
				sdc.Generation = 2
				sdc.Status.ObservedGeneration = pointer.Ptr[int64](1)
				sdc.Status = scyllav1alpha1.ScyllaDBClusterStatus{
					Datacenters: []scyllav1alpha1.ScyllaDBClusterDatacenterStatus{
						{
							Name: sdc.Spec.Datacenters[0].Name,
							Racks: []scyllav1alpha1.ScyllaDBClusterRackStatus{
								{
									Name:  sdc.Spec.Datacenters[0].Racks[0].Name,
									Nodes: pointer.Ptr[int32](0),
								},
							},
						},
					},
				}
				return sdc
			}(),
			new: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.Datacenters[0].Racks = []scyllav1alpha1.RackSpec{}
				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInternal, Field: "spec.datacenters[0].racks[0]", Detail: `rack "rack" can't be removed because its status, that's used to determine node count, is not yet up to date with the generation of this resource; please retry later`},
			},
			expectedErrorString: `spec.datacenters[0].racks[0]: Internal error: rack "rack" can't be removed because its status, that's used to determine node count, is not yet up to date with the generation of this resource; please retry later`,
		},
		{
			name: "non-empty racks deleted",
			old: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.Datacenters[0].Racks = []scyllav1alpha1.RackSpec{
					func() scyllav1alpha1.RackSpec {
						rackSpec := *sc.Spec.Datacenters[0].Racks[0].DeepCopy()
						rackSpec.Name = "rack-0"
						rackSpec.Nodes = pointer.Ptr[int32](3)
						return rackSpec
					}(),
					func() scyllav1alpha1.RackSpec {
						rackSpec := *sc.Spec.Datacenters[0].Racks[0].DeepCopy()
						rackSpec.Name = "rack-1"
						rackSpec.Nodes = pointer.Ptr[int32](2)
						return rackSpec
					}(),
					func() scyllav1alpha1.RackSpec {
						rackSpec := *sc.Spec.Datacenters[0].Racks[0].DeepCopy()
						rackSpec.Name = "rack-2"
						rackSpec.Nodes = pointer.Ptr[int32](1)
						return rackSpec
					}(),
				}
				sc.Status = scyllav1alpha1.ScyllaDBClusterStatus{
					Datacenters: []scyllav1alpha1.ScyllaDBClusterDatacenterStatus{
						{
							Name: sc.Spec.Datacenters[0].Name,
							Racks: []scyllav1alpha1.ScyllaDBClusterRackStatus{
								{
									Name:  sc.Spec.Datacenters[0].Racks[0].Name,
									Nodes: pointer.Ptr[int32](3),
									Stale: pointer.Ptr(false),
								},
								{
									Name:  sc.Spec.Datacenters[0].Racks[1].Name,
									Nodes: pointer.Ptr[int32](2),
									Stale: pointer.Ptr(false),
								},
								{
									Name:  sc.Spec.Datacenters[0].Racks[2].Name,
									Nodes: pointer.Ptr[int32](1),
									Stale: pointer.Ptr(false),
								},
							},
							Nodes: pointer.Ptr[int32](6),
						},
					},
				}
				return sc
			}(),
			new: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.Datacenters[0].Racks = []scyllav1alpha1.RackSpec{}
				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeForbidden, Field: "spec.datacenters[0].racks[0]", BadValue: "", Detail: `rack "rack-0" can't be removed because the nodes are being scaled down`},
				&field.Error{Type: field.ErrorTypeForbidden, Field: "spec.datacenters[0].racks[1]", BadValue: "", Detail: `rack "rack-1" can't be removed because the nodes are being scaled down`},
				&field.Error{Type: field.ErrorTypeForbidden, Field: "spec.datacenters[0].racks[2]", BadValue: "", Detail: `rack "rack-2" can't be removed because the nodes are being scaled down`},
			},
			expectedErrorString: `[spec.datacenters[0].racks[0]: Forbidden: rack "rack-0" can't be removed because the nodes are being scaled down, spec.datacenters[0].racks[1]: Forbidden: rack "rack-1" can't be removed because the nodes are being scaled down, spec.datacenters[0].racks[2]: Forbidden: rack "rack-2" can't be removed because the nodes are being scaled down]`,
		},
		{
			name: "storage specified in datacenter template is immutable",
			old: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.DatacenterTemplate = &scyllav1alpha1.ScyllaDBClusterDatacenterTemplate{
					ScyllaDB: &scyllav1alpha1.ScyllaDBTemplate{
						Storage: &scyllav1alpha1.StorageOptions{
							Capacity: "1Gi",
						},
					},
				}
				return sc
			}(),
			new: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.DatacenterTemplate = &scyllav1alpha1.ScyllaDBClusterDatacenterTemplate{
					ScyllaDB: &scyllav1alpha1.ScyllaDBTemplate{
						Storage: nil,
					},
				}
				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenterTemplate.scyllaDB.storage", BadValue: (*scyllav1alpha1.StorageOptions)(nil), Detail: `field is immutable`},
			},
			expectedErrorString: `spec.datacenterTemplate.scyllaDB.storage: Invalid value: "null": field is immutable`,
		},
		{
			name: "storage specified in datacenter template rack template is immutable",
			old: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.DatacenterTemplate = &scyllav1alpha1.ScyllaDBClusterDatacenterTemplate{
					RackTemplate: &scyllav1alpha1.RackTemplate{
						ScyllaDB: &scyllav1alpha1.ScyllaDBTemplate{
							Storage: &scyllav1alpha1.StorageOptions{
								Capacity: "1Gi",
							},
						},
					},
				}
				return sc
			}(),
			new: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.DatacenterTemplate = &scyllav1alpha1.ScyllaDBClusterDatacenterTemplate{
					RackTemplate: &scyllav1alpha1.RackTemplate{
						ScyllaDB: &scyllav1alpha1.ScyllaDBTemplate{
							Storage: nil,
						},
					},
				}
				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenterTemplate.rackTemplate.scyllaDB.storage", BadValue: (*scyllav1alpha1.StorageOptions)(nil), Detail: `field is immutable`},
			},
			expectedErrorString: `spec.datacenterTemplate.rackTemplate.scyllaDB.storage: Invalid value: "null": field is immutable`,
		},
		{
			name: "storage specified in datacenter template rack is immutable",
			old: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.DatacenterTemplate = &scyllav1alpha1.ScyllaDBClusterDatacenterTemplate{
					Racks: []scyllav1alpha1.RackSpec{
						{
							Name: "foo",
							RackTemplate: scyllav1alpha1.RackTemplate{
								ScyllaDB: &scyllav1alpha1.ScyllaDBTemplate{
									Storage: &scyllav1alpha1.StorageOptions{
										Capacity: "1Gi",
									},
								},
							},
						},
					},
				}
				return sc
			}(),
			new: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.DatacenterTemplate = &scyllav1alpha1.ScyllaDBClusterDatacenterTemplate{
					Racks: []scyllav1alpha1.RackSpec{
						{
							Name: "foo",
							RackTemplate: scyllav1alpha1.RackTemplate{
								ScyllaDB: &scyllav1alpha1.ScyllaDBTemplate{
									Storage: nil,
								},
							},
						},
					},
				}
				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenterTemplate.racks[0].scyllaDB.storage", BadValue: (*scyllav1alpha1.StorageOptions)(nil), Detail: `field is immutable`},
			},
			expectedErrorString: `spec.datacenterTemplate.racks[0].scyllaDB.storage: Invalid value: "null": field is immutable`,
		},
		{
			name: "storage specified in datacenter is immutable",
			old: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.Datacenters[0].ScyllaDB = &scyllav1alpha1.ScyllaDBTemplate{
					Storage: &scyllav1alpha1.StorageOptions{
						Capacity: "1Gi",
					},
				}
				return sc
			}(),
			new: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.Datacenters[0].ScyllaDB = &scyllav1alpha1.ScyllaDBTemplate{
					Storage: nil,
				}
				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenters[0].scyllaDB.storage", BadValue: (*scyllav1alpha1.StorageOptions)(nil), Detail: `field is immutable`},
			},
			expectedErrorString: `spec.datacenters[0].scyllaDB.storage: Invalid value: "null": field is immutable`,
		},
		{
			name: "storage specified in datacenter rack template is immutable",
			old: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.Datacenters[0].RackTemplate = &scyllav1alpha1.RackTemplate{
					ScyllaDB: &scyllav1alpha1.ScyllaDBTemplate{
						Storage: &scyllav1alpha1.StorageOptions{
							Capacity: "1Gi",
						},
					},
				}

				return sc
			}(),
			new: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.Datacenters[0].RackTemplate = &scyllav1alpha1.RackTemplate{
					ScyllaDB: &scyllav1alpha1.ScyllaDBTemplate{
						Storage: nil,
					},
				}
				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenters[0].rackTemplate.scyllaDB.storage", BadValue: (*scyllav1alpha1.StorageOptions)(nil), Detail: `field is immutable`},
			},
			expectedErrorString: `spec.datacenters[0].rackTemplate.scyllaDB.storage: Invalid value: "null": field is immutable`,
		},
		{
			name: "storage specified in datacenter rack is immutable",
			old: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.Datacenters[0].Racks[0].ScyllaDB = &scyllav1alpha1.ScyllaDBTemplate{
					Storage: &scyllav1alpha1.StorageOptions{
						Capacity: "1Gi",
					},
				}

				return sc
			}(),
			new: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.Datacenters[0].Racks[0].ScyllaDB = &scyllav1alpha1.ScyllaDBTemplate{
					Storage: nil,
				}
				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenters[0].racks[0].scyllaDB.storage", BadValue: (*scyllav1alpha1.StorageOptions)(nil), Detail: `field is immutable`},
			},
			expectedErrorString: `spec.datacenters[0].racks[0].scyllaDB.storage: Invalid value: "null": field is immutable`,
		},
		{
			name: "node service type cannot be unset",
			old: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.ExposeOptions = &scyllav1alpha1.ScyllaDBClusterExposeOptions{
					NodeService: &scyllav1alpha1.NodeServiceTemplate{
						Type: scyllav1alpha1.NodeServiceTypeHeadless,
					},
				}
				return sc
			}(),
			new: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.ExposeOptions = nil
				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.exposeOptions.nodeService.type", BadValue: (*scyllav1alpha1.NodeServiceType)(nil), Detail: `field is immutable`},
			},
			expectedErrorString: `spec.exposeOptions.nodeService.type: Invalid value: "null": field is immutable`,
		},
		{
			name: "node service type cannot be changed",
			old: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.ExposeOptions = &scyllav1alpha1.ScyllaDBClusterExposeOptions{
					NodeService: &scyllav1alpha1.NodeServiceTemplate{
						Type: scyllav1alpha1.NodeServiceTypeHeadless,
					},
				}
				return sc
			}(),
			new: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.ExposeOptions = &scyllav1alpha1.ScyllaDBClusterExposeOptions{
					NodeService: &scyllav1alpha1.NodeServiceTemplate{
						Type: scyllav1alpha1.NodeServiceTypeLoadBalancer,
					},
				}
				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.exposeOptions.nodeService.type", BadValue: pointer.Ptr(scyllav1alpha1.NodeServiceTypeLoadBalancer), Detail: `field is immutable`},
			},
			expectedErrorString: `spec.exposeOptions.nodeService.type: Invalid value: "LoadBalancer": field is immutable`,
		},
		{
			name: "clients broadcast address type cannot be changed",
			old: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.ExposeOptions = &scyllav1alpha1.ScyllaDBClusterExposeOptions{
					NodeService: &scyllav1alpha1.NodeServiceTemplate{
						Type: scyllav1alpha1.NodeServiceTypeLoadBalancer,
					},
					BroadcastOptions: &scyllav1alpha1.ScyllaDBClusterNodeBroadcastOptions{
						Clients: scyllav1alpha1.BroadcastOptions{
							Type: scyllav1alpha1.BroadcastAddressTypePodIP,
						},
						Nodes: scyllav1alpha1.BroadcastOptions{
							Type: scyllav1alpha1.BroadcastAddressTypePodIP,
						},
					},
				}
				return sc
			}(),
			new: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.ExposeOptions = &scyllav1alpha1.ScyllaDBClusterExposeOptions{
					NodeService: &scyllav1alpha1.NodeServiceTemplate{
						Type: scyllav1alpha1.NodeServiceTypeLoadBalancer,
					},
					BroadcastOptions: &scyllav1alpha1.ScyllaDBClusterNodeBroadcastOptions{
						Clients: scyllav1alpha1.BroadcastOptions{
							Type: scyllav1alpha1.BroadcastAddressTypeServiceLoadBalancerIngress,
						},
						Nodes: scyllav1alpha1.BroadcastOptions{
							Type: scyllav1alpha1.BroadcastAddressTypePodIP,
						},
					},
				}
				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.exposeOptions.broadcastOptions.clients.type", BadValue: pointer.Ptr(scyllav1alpha1.BroadcastAddressTypeServiceLoadBalancerIngress), Detail: `field is immutable`},
			},
			expectedErrorString: `spec.exposeOptions.broadcastOptions.clients.type: Invalid value: "ServiceLoadBalancerIngress": field is immutable`,
		},
		{
			name: "nodes broadcast address type cannot be changed",
			old: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.ExposeOptions = &scyllav1alpha1.ScyllaDBClusterExposeOptions{
					NodeService: &scyllav1alpha1.NodeServiceTemplate{
						Type: scyllav1alpha1.NodeServiceTypeLoadBalancer,
					},
					BroadcastOptions: &scyllav1alpha1.ScyllaDBClusterNodeBroadcastOptions{
						Clients: scyllav1alpha1.BroadcastOptions{
							Type: scyllav1alpha1.BroadcastAddressTypePodIP,
						},
						Nodes: scyllav1alpha1.BroadcastOptions{
							Type: scyllav1alpha1.BroadcastAddressTypePodIP,
						},
					},
				}
				return sc
			}(),
			new: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.ExposeOptions = &scyllav1alpha1.ScyllaDBClusterExposeOptions{
					NodeService: &scyllav1alpha1.NodeServiceTemplate{
						Type: scyllav1alpha1.NodeServiceTypeLoadBalancer,
					},
					BroadcastOptions: &scyllav1alpha1.ScyllaDBClusterNodeBroadcastOptions{
						Clients: scyllav1alpha1.BroadcastOptions{
							Type: scyllav1alpha1.BroadcastAddressTypePodIP,
						},
						Nodes: scyllav1alpha1.BroadcastOptions{
							Type: scyllav1alpha1.BroadcastAddressTypeServiceLoadBalancerIngress,
						},
					},
				}
				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.exposeOptions.broadcastOptions.nodes.type", BadValue: pointer.Ptr(scyllav1alpha1.BroadcastAddressTypeServiceLoadBalancerIngress), Detail: `field is immutable`},
			},
			expectedErrorString: `spec.exposeOptions.broadcastOptions.nodes.type: Invalid value: "ServiceLoadBalancerIngress": field is immutable`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			errList := validation.ValidateScyllaDBClusterUpdate(test.new, test.old)
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
