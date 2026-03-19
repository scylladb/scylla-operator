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
			expectedErrorList:   nil,
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
				&field.Error{Type: field.ErrorTypeRequired, Field: "spec.exposeOptions.nodeService.type", BadValue: "", Detail: `supported values: Headless, ClusterIP, LoadBalancer`},
			},
			expectedErrorString: `spec.exposeOptions.nodeService.type: Required value: supported values: Headless, ClusterIP, LoadBalancer`,
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
				&field.Error{Type: field.ErrorTypeNotSupported, Field: "spec.exposeOptions.nodeService.type", BadValue: scyllav1alpha1.NodeServiceType("foo"), Detail: `supported values: "Headless", "ClusterIP", "LoadBalancer"`},
			},
			expectedErrorString: `spec.exposeOptions.nodeService.type: Unsupported value: "foo": supported values: "Headless", "ClusterIP", "LoadBalancer"`,
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
			expectedErrorList:   nil,
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
				&field.Error{Type: field.ErrorTypeNotSupported, Field: "spec.exposeOptions.broadcastOptions.clients.type", BadValue: scyllav1alpha1.BroadcastAddressType("foo"), Detail: `supported values: "PodIP", "ServiceClusterIP", "ServiceLoadBalancerIngress"`},
			},
			expectedErrorString: `spec.exposeOptions.broadcastOptions.clients.type: Unsupported value: "foo": supported values: "PodIP", "ServiceClusterIP", "ServiceLoadBalancerIngress"`,
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
				&field.Error{Type: field.ErrorTypeNotSupported, Field: "spec.exposeOptions.broadcastOptions.nodes.type", BadValue: scyllav1alpha1.BroadcastAddressType("foo"), Detail: `supported values: "PodIP", "ServiceClusterIP", "ServiceLoadBalancerIngress"`},
			},
			expectedErrorString: `spec.exposeOptions.broadcastOptions.nodes.type: Unsupported value: "foo": supported values: "PodIP", "ServiceClusterIP", "ServiceLoadBalancerIngress"`,
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
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.minTerminationGracePeriodSeconds", BadValue: int64(-42), Detail: "must be greater than or equal to 0", Origin: "minimum"},
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
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.minReadySeconds", BadValue: int64(-42), Detail: "must be greater than or equal to 0", Origin: "minimum"},
			},
			expectedErrorString: `spec.minReadySeconds: Invalid value: -42: must be greater than or equal to 0`,
		},
		{
			name: "invalid readiness gate condition type",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.ReadinessGates = []corev1.PodReadinessGate{
					{
						ConditionType: "-foo",
					},
				}

				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.readinessGates[0].conditionType", BadValue: corev1.PodConditionType("-foo"), Detail: `name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]')`},
			},
			expectedErrorString: `spec.readinessGates[0].conditionType: Invalid value: "-foo": name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]')`,
		},
		{
			name: "minimal alternator cluster passes",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.ScyllaDB.AlternatorOptions = &scyllav1alpha1.AlternatorOptions{}
				return sc
			}(),
			expectedErrorList:   nil,
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
			expectedErrorList:   nil,
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
			expectedErrorList:   nil,
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
			expectedErrorString: `spec.scyllaDB.alternator.servingCertificate.operatorManagedOptions.additionalDNSNames: Invalid value: ["[not a domain]"]: a lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character (e.g. 'example.com', regex used for validation is '[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*')`,
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
			expectedErrorList:   nil,
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
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.scyllaDB.alternator.servingCertificate.operatorManagedOptions.additionalIPAddresses", BadValue: "0.not-an-ip.0.0", Detail: `must be a valid IP address, (e.g. 10.9.8.7 or 2001:db8::ffff)`, Origin: "format=ip-strict"},
			},
			expectedErrorString: `spec.scyllaDB.alternator.servingCertificate.operatorManagedOptions.additionalIPAddresses: Invalid value: "0.not-an-ip.0.0": must be a valid IP address, (e.g. 10.9.8.7 or 2001:db8::ffff)`,
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
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenterTemplate.rackTemplate.nodes", BadValue: int64(-42), Detail: "must be greater than or equal to 0", Origin: "minimum"},
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
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenterTemplate.racks[0].nodes", BadValue: int64(-42), Detail: "must be greater than or equal to 0", Origin: "minimum"},
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
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenterTemplate.topologyLabelSelector", BadValue: "-123", Detail: `name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]')`, Origin: "format=k8s-label-key"},
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenterTemplate.topologyLabelSelector", BadValue: "*321", Detail: `a valid label must be an empty string or consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyValue',  or 'my_value',  or '12345', regex used for validation is '(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])?')`, Origin: "format=k8s-label-value"},
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
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenterTemplate.rackTemplate.topologyLabelSelector", BadValue: "-123", Detail: `name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]')`, Origin: "format=k8s-label-key"},
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenterTemplate.rackTemplate.topologyLabelSelector", BadValue: "*321", Detail: `a valid label must be an empty string or consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyValue',  or 'my_value',  or '12345', regex used for validation is '(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])?')`, Origin: "format=k8s-label-value"},
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
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenterTemplate.racks[0].topologyLabelSelector", BadValue: "-123", Detail: `name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]')`, Origin: "format=k8s-label-key"},
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenterTemplate.racks[0].topologyLabelSelector", BadValue: "*321", Detail: `a valid label must be an empty string or consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyValue',  or 'my_value',  or '12345', regex used for validation is '(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])?')`, Origin: "format=k8s-label-value"},
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
			name: "invalid labels in spec metadata",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.Metadata = &scyllav1alpha1.ObjectTemplateMetadata{
					Labels: map[string]string{
						"-123": "*321",
					},
				}

				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.metadata.labels", BadValue: "-123", Detail: `name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]')`, Origin: "format=k8s-label-key"},
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.metadata.labels", BadValue: "*321", Detail: `a valid label must be an empty string or consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyValue',  or 'my_value',  or '12345', regex used for validation is '(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])?')`, Origin: "format=k8s-label-value"},
			},
			expectedErrorString: `[spec.metadata.labels: Invalid value: "-123": name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]'), spec.metadata.labels: Invalid value: "*321": a valid label must be an empty string or consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyValue',  or 'my_value',  or '12345', regex used for validation is '(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])?')]`,
		},
		{
			name: "invalid labels in datacenter template metadata",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.DatacenterTemplate.Metadata = &scyllav1alpha1.ObjectTemplateMetadata{
					Labels: map[string]string{
						"-123": "*321",
					},
				}

				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenterTemplate.metadata.labels", BadValue: "-123", Detail: `name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]')`, Origin: "format=k8s-label-key"},
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenterTemplate.metadata.labels", BadValue: "*321", Detail: `a valid label must be an empty string or consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyValue',  or 'my_value',  or '12345', regex used for validation is '(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])?')`, Origin: "format=k8s-label-value"},
			},
			expectedErrorString: `[spec.datacenterTemplate.metadata.labels: Invalid value: "-123": name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]'), spec.datacenterTemplate.metadata.labels: Invalid value: "*321": a valid label must be an empty string or consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyValue',  or 'my_value',  or '12345', regex used for validation is '(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])?')]`,
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
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenterTemplate.scyllaDB.storage.metadata.labels", BadValue: "-123", Detail: `name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]')`, Origin: "format=k8s-label-key"},
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenterTemplate.scyllaDB.storage.metadata.labels", BadValue: "*321", Detail: `a valid label must be an empty string or consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyValue',  or 'my_value',  or '12345', regex used for validation is '(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])?')`, Origin: "format=k8s-label-value"},
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
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenterTemplate.rackTemplate.scyllaDB.storage.metadata.labels", BadValue: "-123", Detail: `name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]')`, Origin: "format=k8s-label-key"},
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenterTemplate.rackTemplate.scyllaDB.storage.metadata.labels", BadValue: "*321", Detail: `a valid label must be an empty string or consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyValue',  or 'my_value',  or '12345', regex used for validation is '(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])?')`, Origin: "format=k8s-label-value"},
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
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenterTemplate.racks[0].scyllaDB.storage.metadata.labels", BadValue: "-123", Detail: `name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]')`, Origin: "format=k8s-label-key"},
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenterTemplate.racks[0].scyllaDB.storage.metadata.labels", BadValue: "*321", Detail: `a valid label must be an empty string or consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyValue',  or 'my_value',  or '12345', regex used for validation is '(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])?')`, Origin: "format=k8s-label-value"},
			},
			expectedErrorString: `[spec.datacenterTemplate.racks[0].scyllaDB.storage.metadata.labels: Invalid value: "-123": name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]'), spec.datacenterTemplate.racks[0].scyllaDB.storage.metadata.labels: Invalid value: "*321": a valid label must be an empty string or consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyValue',  or 'my_value',  or '12345', regex used for validation is '(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])?')]`,
		},
		{
			name: "invalid annotations in spec metadata",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.Metadata = &scyllav1alpha1.ObjectTemplateMetadata{
					Annotations: map[string]string{
						"-123": "*321",
					},
				}

				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.metadata.annotations", BadValue: "-123", Detail: `name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]')`, Origin: "format=k8s-label-key"},
			},
			expectedErrorString: `spec.metadata.annotations: Invalid value: "-123": name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]')`,
		},
		{
			name: "invalid annotations in spec datacenter template metadata",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.DatacenterTemplate.Metadata = &scyllav1alpha1.ObjectTemplateMetadata{
					Annotations: map[string]string{
						"-123": "*321",
					},
				}

				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenterTemplate.metadata.annotations", BadValue: "-123", Detail: `name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]')`, Origin: "format=k8s-label-key"},
			},
			expectedErrorString: `spec.datacenterTemplate.metadata.annotations: Invalid value: "-123": name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]')`,
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
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenterTemplate.scyllaDB.storage.metadata.annotations", BadValue: "-123", Detail: `name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]')`, Origin: "format=k8s-label-key"},
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
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenterTemplate.rackTemplate.scyllaDB.storage.metadata.annotations", BadValue: "-123", Detail: `name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]')`, Origin: "format=k8s-label-key"},
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
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenterTemplate.racks[0].scyllaDB.storage.metadata.annotations", BadValue: "-123", Detail: `name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]')`, Origin: "format=k8s-label-key"},
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
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenters[0].rackTemplate.nodes", BadValue: int64(-42), Detail: "must be greater than or equal to 0", Origin: "minimum"},
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
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenters[0].racks[0].nodes", BadValue: int64(-42), Detail: "must be greater than or equal to 0", Origin: "minimum"},
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
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenters[0].topologyLabelSelector", BadValue: "-123", Detail: `name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]')`, Origin: "format=k8s-label-key"},
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenters[0].topologyLabelSelector", BadValue: "*321", Detail: `a valid label must be an empty string or consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyValue',  or 'my_value',  or '12345', regex used for validation is '(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])?')`, Origin: "format=k8s-label-value"},
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
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenters[0].rackTemplate.topologyLabelSelector", BadValue: "-123", Detail: `name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]')`, Origin: "format=k8s-label-key"},
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenters[0].rackTemplate.topologyLabelSelector", BadValue: "*321", Detail: `a valid label must be an empty string or consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyValue',  or 'my_value',  or '12345', regex used for validation is '(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])?')`, Origin: "format=k8s-label-value"},
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
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenters[0].racks[0].topologyLabelSelector", BadValue: "-123", Detail: `name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]')`, Origin: "format=k8s-label-key"},
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenters[0].racks[0].topologyLabelSelector", BadValue: "*321", Detail: `a valid label must be an empty string or consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyValue',  or 'my_value',  or '12345', regex used for validation is '(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])?')`, Origin: "format=k8s-label-value"},
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
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenters[0].scyllaDB.storage.metadata.labels", BadValue: "-123", Detail: `name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]')`, Origin: "format=k8s-label-key"},
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenters[0].scyllaDB.storage.metadata.labels", BadValue: "*321", Detail: `a valid label must be an empty string or consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyValue',  or 'my_value',  or '12345', regex used for validation is '(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])?')`, Origin: "format=k8s-label-value"},
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
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenters[0].rackTemplate.scyllaDB.storage.metadata.labels", BadValue: "-123", Detail: `name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]')`, Origin: "format=k8s-label-key"},
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenters[0].rackTemplate.scyllaDB.storage.metadata.labels", BadValue: "*321", Detail: `a valid label must be an empty string or consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyValue',  or 'my_value',  or '12345', regex used for validation is '(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])?')`, Origin: "format=k8s-label-value"},
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
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenters[0].racks[0].scyllaDB.storage.metadata.labels", BadValue: "-123", Detail: `name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]')`, Origin: "format=k8s-label-key"},
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenters[0].racks[0].scyllaDB.storage.metadata.labels", BadValue: "*321", Detail: `a valid label must be an empty string or consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyValue',  or 'my_value',  or '12345', regex used for validation is '(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])?')`, Origin: "format=k8s-label-value"},
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
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenters[0].scyllaDB.storage.metadata.annotations", BadValue: "-123", Detail: `name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]')`, Origin: "format=k8s-label-key"},
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
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenters[0].rackTemplate.scyllaDB.storage.metadata.annotations", BadValue: "-123", Detail: `name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]')`, Origin: "format=k8s-label-key"},
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
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenters[0].racks[0].scyllaDB.storage.metadata.annotations", BadValue: "-123", Detail: `name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]')`, Origin: "format=k8s-label-key"},
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
		// PLACEMENT
		{
			name: "invalid node selector requirement in node affinity having invalid operator in datacenter template placement",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.DatacenterTemplate.Placement = &scyllav1alpha1.Placement{
					NodeAffinity: &corev1.NodeAffinity{
						RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
							NodeSelectorTerms: []corev1.NodeSelectorTerm{{
								MatchExpressions: []corev1.NodeSelectorRequirement{{
									Key: "key1",
								}},
							}},
						},
					},
				}

				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenterTemplate.placement.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[0].matchExpressions[0].operator", BadValue: corev1.NodeSelectorOperator(""), Detail: `not a valid selector operator`},
			},
			expectedErrorString: `spec.datacenterTemplate.placement.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[0].matchExpressions[0].operator: Invalid value: "": not a valid selector operator`,
		},
		{
			name: "invalid node selector requirement in node affinity having invalid key in datacenter template placement",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.DatacenterTemplate.Placement = &scyllav1alpha1.Placement{
					NodeAffinity: &corev1.NodeAffinity{
						RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
							NodeSelectorTerms: []corev1.NodeSelectorTerm{{
								MatchExpressions: []corev1.NodeSelectorRequirement{{
									Key:      "invalid key ___@#",
									Operator: corev1.NodeSelectorOpExists,
								}},
							}},
						},
					},
				}

				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenterTemplate.placement.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[0].matchExpressions[0].key", BadValue: "invalid key ___@#", Detail: `name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]')`, Origin: "format=k8s-label-key"},
			},
			expectedErrorString: `spec.datacenterTemplate.placement.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[0].matchExpressions[0].key: Invalid value: "invalid key ___@#": name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]')`,
		},
		{
			name: "invalid node field selector in node affinity having too many values in datacenter template placement",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.DatacenterTemplate.Placement = &scyllav1alpha1.Placement{
					NodeAffinity: &corev1.NodeAffinity{
						RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
							NodeSelectorTerms: []corev1.NodeSelectorTerm{{
								MatchFields: []corev1.NodeSelectorRequirement{{
									Key:      "metadata.name",
									Operator: corev1.NodeSelectorOpIn,
									Values:   []string{"host1", "host2"},
								}},
							}},
						},
					},
				}

				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeRequired, Field: "spec.datacenterTemplate.placement.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[0].matchFields[0].values", BadValue: "", Detail: "must be only one value when `operator` is 'In' or 'NotIn' for node field selector"},
			},
			expectedErrorString: "spec.datacenterTemplate.placement.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[0].matchFields[0].values: Required value: must be only one value when `operator` is 'In' or 'NotIn' for node field selector",
		},
		{
			name: "invalid node field selector in node affinity having invalid operator in datacenter template placement",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.DatacenterTemplate.Placement = &scyllav1alpha1.Placement{
					NodeAffinity: &corev1.NodeAffinity{
						RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
							NodeSelectorTerms: []corev1.NodeSelectorTerm{{
								MatchFields: []corev1.NodeSelectorRequirement{{
									Key:      "metadata.name",
									Operator: corev1.NodeSelectorOpExists,
								}},
							}},
						},
					},
				}

				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenterTemplate.placement.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[0].matchFields[0].operator", BadValue: corev1.NodeSelectorOpExists, Detail: "not a valid selector operator"},
			},
			expectedErrorString: `spec.datacenterTemplate.placement.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[0].matchFields[0].operator: Invalid value: "Exists": not a valid selector operator`,
		},
		{
			name: "invalid node field selector in node affinity having invalid key in datacenter template placement",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.DatacenterTemplate.Placement = &scyllav1alpha1.Placement{
					NodeAffinity: &corev1.NodeAffinity{
						RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
							NodeSelectorTerms: []corev1.NodeSelectorTerm{{
								MatchFields: []corev1.NodeSelectorRequirement{{
									Key:      "metadata.namespace",
									Operator: corev1.NodeSelectorOpIn,
									Values:   []string{"ns1"},
								}},
							}},
						},
					},
				}

				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenterTemplate.placement.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[0].matchFields[0].key", BadValue: "metadata.namespace", Detail: "not a valid field selector key"},
			},
			expectedErrorString: `spec.datacenterTemplate.placement.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[0].matchFields[0].key: Invalid value: "metadata.namespace": not a valid field selector key`,
		},
		{
			name: "invalid preferredSchedulingTerm in node affinity having weight out of range in datacenter template placement",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.DatacenterTemplate.Placement = &scyllav1alpha1.Placement{
					NodeAffinity: &corev1.NodeAffinity{
						PreferredDuringSchedulingIgnoredDuringExecution: []corev1.PreferredSchedulingTerm{{
							Weight: 199,
							Preference: corev1.NodeSelectorTerm{
								MatchExpressions: []corev1.NodeSelectorRequirement{{
									Key:      "foo",
									Operator: corev1.NodeSelectorOpIn,
									Values:   []string{"bar"},
								}},
							},
						}},
					},
				}

				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenterTemplate.placement.nodeAffinity.preferredDuringSchedulingIgnoredDuringExecution[0].weight", BadValue: int32(199), Detail: "must be in the range 1-100"},
			},
			expectedErrorString: `spec.datacenterTemplate.placement.nodeAffinity.preferredDuringSchedulingIgnoredDuringExecution[0].weight: Invalid value: 199: must be in the range 1-100`,
		},
		{
			name: "invalid requiredDuringSchedulingIgnoredDuringExecution node selector in node affinity having empty nodeSelectorTerms in datacenter template placement",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.DatacenterTemplate.Placement = &scyllav1alpha1.Placement{
					NodeAffinity: &corev1.NodeAffinity{
						RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
							NodeSelectorTerms: []corev1.NodeSelectorTerm{},
						},
					},
				}

				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeRequired, Field: "spec.datacenterTemplate.placement.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms", BadValue: "", Detail: "must have at least one node selector term"},
			},
			expectedErrorString: `spec.datacenterTemplate.placement.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms: Required value: must have at least one node selector term`,
		},
		{
			name: "invalid requiredDuringSchedulingIgnoredDuringExecution node selector in node affinity having empty nodeSelectorTerms in datacenter template placement",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.DatacenterTemplate.Placement = &scyllav1alpha1.Placement{
					NodeAffinity: &corev1.NodeAffinity{
						RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
							NodeSelectorTerms: []corev1.NodeSelectorTerm{},
						},
					},
				}

				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeRequired, Field: "spec.datacenterTemplate.placement.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms", BadValue: "", Detail: "must have at least one node selector term"},
			},
			expectedErrorString: `spec.datacenterTemplate.placement.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms: Required value: must have at least one node selector term`,
		},
		{
			name: "invalid weight in preferredDuringSchedulingIgnoredDuringExecution in pod affinity having weight out of range in datacenter template placement",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.DatacenterTemplate.Placement = &scyllav1alpha1.Placement{
					PodAffinity: &corev1.PodAffinity{
						PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{{
							Weight: 109,
							PodAffinityTerm: corev1.PodAffinityTerm{
								LabelSelector: &metav1.LabelSelector{
									MatchExpressions: []metav1.LabelSelectorRequirement{{
										Key:      "key2",
										Operator: metav1.LabelSelectorOpNotIn,
										Values:   []string{"value1", "value2"},
									}},
								},
								Namespaces:  []string{"ns"},
								TopologyKey: "region",
							},
						}},
					},
				}

				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenterTemplate.placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[0].weight", BadValue: int32(109), Detail: "must be in the range 1-100"},
			},
			expectedErrorString: `spec.datacenterTemplate.placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[0].weight: Invalid value: 109: must be in the range 1-100`,
		},
		{
			name: "invalid labelSelector in preferredDuringSchedulingIgnoredDuringExecution in podAntiAffinity having values with Exists operator in datacenter template placement",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.DatacenterTemplate.Placement = &scyllav1alpha1.Placement{
					PodAntiAffinity: &corev1.PodAntiAffinity{
						PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{{
							Weight: 10,
							PodAffinityTerm: corev1.PodAffinityTerm{
								LabelSelector: &metav1.LabelSelector{
									MatchExpressions: []metav1.LabelSelectorRequirement{{
										Key:      "key2",
										Operator: metav1.LabelSelectorOpExists,
										Values:   []string{"value1", "value2"},
									}},
								},
								Namespaces:  []string{"ns"},
								TopologyKey: "region",
							},
						}},
					},
				}
				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeForbidden, Field: "spec.datacenterTemplate.placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[0].podAffinityTerm.labelSelector.matchExpressions[0].values", BadValue: "", Detail: "may not be specified when `operator` is 'Exists' or 'DoesNotExist'"},
			},
			expectedErrorString: "spec.datacenterTemplate.placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[0].podAffinityTerm.labelSelector.matchExpressions[0].values: Forbidden: may not be specified when `operator` is 'Exists' or 'DoesNotExist'",
		},
		{
			name: "invalid namespaceSelector in preferredDuringSchedulingIgnoredDuringExecution in podAntiAffinity having missing values with In operator in datacenter template placement",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.DatacenterTemplate.Placement = &scyllav1alpha1.Placement{
					PodAntiAffinity: &corev1.PodAntiAffinity{
						PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{{
							Weight: 10,
							PodAffinityTerm: corev1.PodAffinityTerm{
								NamespaceSelector: &metav1.LabelSelector{
									MatchExpressions: []metav1.LabelSelectorRequirement{{
										Key:      "key2",
										Operator: metav1.LabelSelectorOpIn,
									}},
								},
								Namespaces:  []string{"ns"},
								TopologyKey: "region",
							},
						}},
					},
				}
				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeRequired, Field: "spec.datacenterTemplate.placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[0].podAffinityTerm.namespaceSelector.matchExpressions[0].values", BadValue: "", Detail: "must be specified when `operator` is 'In' or 'NotIn'"},
			},
			expectedErrorString: "spec.datacenterTemplate.placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[0].podAffinityTerm.namespaceSelector.matchExpressions[0].values: Required value: must be specified when `operator` is 'In' or 'NotIn'",
		},
		{
			name: "invalid namespaceSelector in preferredDuringSchedulingIgnoredDuringExecution in podAntiAffinity having values with Exists operator in datacenter template placement",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.DatacenterTemplate.Placement = &scyllav1alpha1.Placement{
					PodAntiAffinity: &corev1.PodAntiAffinity{
						PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{{
							Weight: 10,
							PodAffinityTerm: corev1.PodAffinityTerm{
								NamespaceSelector: &metav1.LabelSelector{
									MatchExpressions: []metav1.LabelSelectorRequirement{{
										Key:      "key2",
										Operator: metav1.LabelSelectorOpExists,
										Values:   []string{"value1", "value2"},
									}},
								},
								Namespaces:  []string{"ns"},
								TopologyKey: "region",
							},
						}},
					},
				}
				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeForbidden, Field: "spec.datacenterTemplate.placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[0].podAffinityTerm.namespaceSelector.matchExpressions[0].values", BadValue: "", Detail: "may not be specified when `operator` is 'Exists' or 'DoesNotExist'"},
			},
			expectedErrorString: "spec.datacenterTemplate.placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[0].podAffinityTerm.namespaceSelector.matchExpressions[0].values: Forbidden: may not be specified when `operator` is 'Exists' or 'DoesNotExist'",
		},
		{
			name: "invalid namespace in preferredDuringSchedulingIgnoredDuringExecution in podAffinity having invalid namespace in datacenter template placement",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.DatacenterTemplate.Placement = &scyllav1alpha1.Placement{
					PodAffinity: &corev1.PodAffinity{
						PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{{
							Weight: 10,
							PodAffinityTerm: corev1.PodAffinityTerm{
								LabelSelector: &metav1.LabelSelector{
									MatchExpressions: []metav1.LabelSelectorRequirement{{
										Key:      "key2",
										Operator: metav1.LabelSelectorOpExists,
									}},
								},
								Namespaces:  []string{"INVALID_NAMESPACE"},
								TopologyKey: "region",
							},
						}},
					},
				}
				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenterTemplate.placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[0].podAffinityTerm.namespace", BadValue: "INVALID_NAMESPACE", Detail: `a lowercase RFC 1123 label must consist of lower case alphanumeric characters or '-', and must start and end with an alphanumeric character (e.g. 'my-name',  or '123-abc', regex used for validation is '[a-z0-9]([-a-z0-9]*[a-z0-9])?')`},
			},
			expectedErrorString: `spec.datacenterTemplate.placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[0].podAffinityTerm.namespace: Invalid value: "INVALID_NAMESPACE": a lowercase RFC 1123 label must consist of lower case alphanumeric characters or '-', and must start and end with an alphanumeric character (e.g. 'my-name',  or '123-abc', regex used for validation is '[a-z0-9]([-a-z0-9]*[a-z0-9])?')`,
		},
		{
			name: "invalid hard pod affinity having empty topologyKey in datacenter template placement",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.DatacenterTemplate.Placement = &scyllav1alpha1.Placement{
					PodAffinity: &corev1.PodAffinity{
						RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{{
							LabelSelector: &metav1.LabelSelector{
								MatchExpressions: []metav1.LabelSelectorRequirement{{
									Key:      "key2",
									Operator: metav1.LabelSelectorOpIn,
									Values:   []string{"value1", "value2"},
								}},
							},
							Namespaces: []string{"ns"},
						}},
					},
				}
				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeRequired, Field: "spec.datacenterTemplate.placement.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[0].topologyKey", BadValue: "", Detail: "can not be empty"},
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenterTemplate.placement.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[0].topologyKey", BadValue: "", Detail: "name part must be non-empty", Origin: "format=k8s-label-key"},
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenterTemplate.placement.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[0].topologyKey", BadValue: "", Detail: "name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]')", Origin: "format=k8s-label-key"},
			},
			expectedErrorString: `[spec.datacenterTemplate.placement.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[0].topologyKey: Required value: can not be empty, spec.datacenterTemplate.placement.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[0].topologyKey: Invalid value: "": name part must be non-empty, spec.datacenterTemplate.placement.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[0].topologyKey: Invalid value: "": name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]')]`,
		},
		{
			name: "invalid hard pod anti-affinity having empty topologyKey in datacenter template placement",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.DatacenterTemplate.Placement = &scyllav1alpha1.Placement{
					PodAntiAffinity: &corev1.PodAntiAffinity{
						RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{{
							LabelSelector: &metav1.LabelSelector{
								MatchExpressions: []metav1.LabelSelectorRequirement{{
									Key:      "key2",
									Operator: metav1.LabelSelectorOpIn,
									Values:   []string{"value1", "value2"},
								}},
							},
							Namespaces: []string{"ns"},
						}},
					},
				}
				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeRequired, Field: "spec.datacenterTemplate.placement.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[0].topologyKey", BadValue: "", Detail: "can not be empty"},
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenterTemplate.placement.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[0].topologyKey", BadValue: "", Detail: "name part must be non-empty", Origin: "format=k8s-label-key"},
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenterTemplate.placement.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[0].topologyKey", BadValue: "", Detail: "name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]')", Origin: "format=k8s-label-key"},
			},
			expectedErrorString: `[spec.datacenterTemplate.placement.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[0].topologyKey: Required value: can not be empty, spec.datacenterTemplate.placement.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[0].topologyKey: Invalid value: "": name part must be non-empty, spec.datacenterTemplate.placement.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[0].topologyKey: Invalid value: "": name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]')]`,
		},
		{
			name: "invalid soft pod affinity having empty topologyKey in datacenter template placement",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.DatacenterTemplate.Placement = &scyllav1alpha1.Placement{
					PodAffinity: &corev1.PodAffinity{
						PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{{
							Weight: 10,
							PodAffinityTerm: corev1.PodAffinityTerm{
								LabelSelector: &metav1.LabelSelector{
									MatchExpressions: []metav1.LabelSelectorRequirement{{
										Key:      "key2",
										Operator: metav1.LabelSelectorOpNotIn,
										Values:   []string{"value1", "value2"},
									}},
								},
								Namespaces: []string{"ns"},
							},
						}},
					},
				}
				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeRequired, Field: "spec.datacenterTemplate.placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[0].podAffinityTerm.topologyKey", BadValue: "", Detail: "can not be empty"},
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenterTemplate.placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[0].podAffinityTerm.topologyKey", BadValue: "", Detail: "name part must be non-empty", Origin: "format=k8s-label-key"},
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenterTemplate.placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[0].podAffinityTerm.topologyKey", BadValue: "", Detail: "name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]')", Origin: "format=k8s-label-key"},
			},
			expectedErrorString: `[spec.datacenterTemplate.placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[0].podAffinityTerm.topologyKey: Required value: can not be empty, spec.datacenterTemplate.placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[0].podAffinityTerm.topologyKey: Invalid value: "": name part must be non-empty, spec.datacenterTemplate.placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[0].podAffinityTerm.topologyKey: Invalid value: "": name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]')]`,
		},
		{
			name: "invalid soft pod anti-affinity having empty topologyKey in datacenter template placement",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.DatacenterTemplate.Placement = &scyllav1alpha1.Placement{
					PodAntiAffinity: &corev1.PodAntiAffinity{
						PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{{
							Weight: 10,
							PodAffinityTerm: corev1.PodAffinityTerm{
								LabelSelector: &metav1.LabelSelector{
									MatchExpressions: []metav1.LabelSelectorRequirement{{
										Key:      "key2",
										Operator: metav1.LabelSelectorOpNotIn,
										Values:   []string{"value1", "value2"},
									}},
								},
								Namespaces: []string{"ns"},
							},
						}},
					},
				}
				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeRequired, Field: "spec.datacenterTemplate.placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[0].podAffinityTerm.topologyKey", BadValue: "", Detail: "can not be empty"},
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenterTemplate.placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[0].podAffinityTerm.topologyKey", BadValue: "", Detail: "name part must be non-empty", Origin: "format=k8s-label-key"},
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenterTemplate.placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[0].podAffinityTerm.topologyKey", BadValue: "", Detail: "name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]')", Origin: "format=k8s-label-key"},
			},
			expectedErrorString: `[spec.datacenterTemplate.placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[0].podAffinityTerm.topologyKey: Required value: can not be empty, spec.datacenterTemplate.placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[0].podAffinityTerm.topologyKey: Invalid value: "": name part must be non-empty, spec.datacenterTemplate.placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[0].podAffinityTerm.topologyKey: Invalid value: "": name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]')]`,
		},
		{
			name: "invalid soft pod affinity having incorrectly defined key in MatchLabelKeys in datacenter template placement",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.DatacenterTemplate.Placement = &scyllav1alpha1.Placement{
					PodAffinity: &corev1.PodAffinity{
						PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{{
							Weight: 10,
							PodAffinityTerm: corev1.PodAffinityTerm{
								LabelSelector: &metav1.LabelSelector{
									MatchExpressions: []metav1.LabelSelectorRequirement{{
										Key:      "key",
										Operator: metav1.LabelSelectorOpNotIn,
										Values:   []string{"value1", "value2"},
									}},
								},
								TopologyKey:    "k8s.io/zone",
								MatchLabelKeys: []string{"/simple"},
							},
						}},
					},
				}
				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenterTemplate.placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[0].podAffinityTerm.matchLabelKeys[0]", BadValue: "/simple", Detail: "prefix part must be non-empty", Origin: "format=k8s-label-key"},
			},
			expectedErrorString: `spec.datacenterTemplate.placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[0].podAffinityTerm.matchLabelKeys[0]: Invalid value: "/simple": prefix part must be non-empty`,
		},
		{
			name: "invalid hard pod affinity having incorrectly defined key in MatchLabelKeys in datacenter template placement",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.DatacenterTemplate.Placement = &scyllav1alpha1.Placement{
					PodAffinity: &corev1.PodAffinity{
						RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{{
							LabelSelector: &metav1.LabelSelector{
								MatchExpressions: []metav1.LabelSelectorRequirement{{
									Key:      "key",
									Operator: metav1.LabelSelectorOpNotIn,
									Values:   []string{"value1", "value2"},
								}},
							},
							TopologyKey:    "k8s.io/zone",
							MatchLabelKeys: []string{"/simple"},
						}},
					},
				}
				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenterTemplate.placement.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[0].matchLabelKeys[0]", BadValue: "/simple", Detail: "prefix part must be non-empty", Origin: "format=k8s-label-key"},
			},
			expectedErrorString: `spec.datacenterTemplate.placement.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[0].matchLabelKeys[0]: Invalid value: "/simple": prefix part must be non-empty`,
		},
		{
			name: "invalid soft pod anti-affinity having incorrectly defined key in MatchLabelKeys in datacenter template placement",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.DatacenterTemplate.Placement = &scyllav1alpha1.Placement{
					PodAntiAffinity: &corev1.PodAntiAffinity{
						PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{{
							Weight: 10,
							PodAffinityTerm: corev1.PodAffinityTerm{
								LabelSelector: &metav1.LabelSelector{
									MatchExpressions: []metav1.LabelSelectorRequirement{{
										Key:      "key",
										Operator: metav1.LabelSelectorOpNotIn,
										Values:   []string{"value1", "value2"},
									}},
								},
								TopologyKey:    "k8s.io/zone",
								MatchLabelKeys: []string{"/simple"},
							},
						}},
					},
				}
				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenterTemplate.placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[0].podAffinityTerm.matchLabelKeys[0]", BadValue: "/simple", Detail: "prefix part must be non-empty", Origin: "format=k8s-label-key"},
			},
			expectedErrorString: `spec.datacenterTemplate.placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[0].podAffinityTerm.matchLabelKeys[0]: Invalid value: "/simple": prefix part must be non-empty`,
		},
		{
			name: "invalid hard pod anti-affinity having incorrectly defined key in MatchLabelKeys in datacenter template placement",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.DatacenterTemplate.Placement = &scyllav1alpha1.Placement{
					PodAntiAffinity: &corev1.PodAntiAffinity{
						RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{{
							LabelSelector: &metav1.LabelSelector{
								MatchExpressions: []metav1.LabelSelectorRequirement{{
									Key:      "key",
									Operator: metav1.LabelSelectorOpNotIn,
									Values:   []string{"value1", "value2"},
								}},
							},
							TopologyKey:    "k8s.io/zone",
							MatchLabelKeys: []string{"/simple"},
						}},
					},
				}
				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenterTemplate.placement.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[0].matchLabelKeys[0]", BadValue: "/simple", Detail: "prefix part must be non-empty", Origin: "format=k8s-label-key"},
			},
			expectedErrorString: `spec.datacenterTemplate.placement.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[0].matchLabelKeys[0]: Invalid value: "/simple": prefix part must be non-empty`,
		},
		{
			name: "invalid soft pod affinity having incorrectly defined key in MismatchLabelKeys in datacenter template placement",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.DatacenterTemplate.Placement = &scyllav1alpha1.Placement{
					PodAffinity: &corev1.PodAffinity{
						PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{{
							Weight: 10,
							PodAffinityTerm: corev1.PodAffinityTerm{
								LabelSelector: &metav1.LabelSelector{
									MatchExpressions: []metav1.LabelSelectorRequirement{{
										Key:      "key",
										Operator: metav1.LabelSelectorOpNotIn,
										Values:   []string{"value1", "value2"},
									}},
								},
								TopologyKey:       "k8s.io/zone",
								MismatchLabelKeys: []string{"/simple"},
							},
						}},
					},
				}
				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenterTemplate.placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[0].podAffinityTerm.mismatchLabelKeys[0]", BadValue: "/simple", Detail: "prefix part must be non-empty", Origin: "format=k8s-label-key"},
			},
			expectedErrorString: `spec.datacenterTemplate.placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[0].podAffinityTerm.mismatchLabelKeys[0]: Invalid value: "/simple": prefix part must be non-empty`,
		},
		{
			name: "invalid hard pod affinity having incorrectly defined key in MismatchLabelKeys in datacenter template placement",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.DatacenterTemplate.Placement = &scyllav1alpha1.Placement{
					PodAffinity: &corev1.PodAffinity{
						RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{{
							LabelSelector: &metav1.LabelSelector{
								MatchExpressions: []metav1.LabelSelectorRequirement{{
									Key:      "key",
									Operator: metav1.LabelSelectorOpNotIn,
									Values:   []string{"value1", "value2"},
								}},
							},
							TopologyKey:       "k8s.io/zone",
							MismatchLabelKeys: []string{"/simple"},
						}},
					},
				}
				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenterTemplate.placement.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[0].mismatchLabelKeys[0]", BadValue: "/simple", Detail: "prefix part must be non-empty", Origin: "format=k8s-label-key"},
			},
			expectedErrorString: `spec.datacenterTemplate.placement.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[0].mismatchLabelKeys[0]: Invalid value: "/simple": prefix part must be non-empty`,
		},
		{
			name: "invalid soft pod anti-affinity having incorrectly defined key in MismatchLabelKeys in datacenter template placement",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.DatacenterTemplate.Placement = &scyllav1alpha1.Placement{
					PodAntiAffinity: &corev1.PodAntiAffinity{
						PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{{
							Weight: 10,
							PodAffinityTerm: corev1.PodAffinityTerm{
								LabelSelector: &metav1.LabelSelector{
									MatchExpressions: []metav1.LabelSelectorRequirement{{
										Key:      "key",
										Operator: metav1.LabelSelectorOpNotIn,
										Values:   []string{"value1", "value2"},
									}},
								},
								TopologyKey:       "k8s.io/zone",
								MismatchLabelKeys: []string{"/simple"},
							},
						}},
					},
				}
				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenterTemplate.placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[0].podAffinityTerm.mismatchLabelKeys[0]", BadValue: "/simple", Detail: "prefix part must be non-empty", Origin: "format=k8s-label-key"},
			},
			expectedErrorString: `spec.datacenterTemplate.placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[0].podAffinityTerm.mismatchLabelKeys[0]: Invalid value: "/simple": prefix part must be non-empty`,
		},
		{
			name: "invalid hard pod anti-affinity having incorrectly defined key in MismatchLabelKeys in datacenter template placement",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.DatacenterTemplate.Placement = &scyllav1alpha1.Placement{
					PodAntiAffinity: &corev1.PodAntiAffinity{
						RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{{
							LabelSelector: &metav1.LabelSelector{
								MatchExpressions: []metav1.LabelSelectorRequirement{{
									Key:      "key",
									Operator: metav1.LabelSelectorOpNotIn,
									Values:   []string{"value1", "value2"},
								}},
							},
							TopologyKey:       "k8s.io/zone",
							MismatchLabelKeys: []string{"/simple"},
						}},
					},
				}
				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenterTemplate.placement.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[0].mismatchLabelKeys[0]", BadValue: "/simple", Detail: "prefix part must be non-empty", Origin: "format=k8s-label-key"},
			},
			expectedErrorString: `spec.datacenterTemplate.placement.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[0].mismatchLabelKeys[0]: Invalid value: "/simple": prefix part must be non-empty`,
		},
		{
			name: "invalid soft pod affinity having key in both MatchLabelKeys and labelSelector in datacenter template placement",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.DatacenterTemplate.Placement = &scyllav1alpha1.Placement{
					PodAffinity: &corev1.PodAffinity{
						PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{{
							Weight: 10,
							PodAffinityTerm: corev1.PodAffinityTerm{
								LabelSelector: &metav1.LabelSelector{
									MatchExpressions: []metav1.LabelSelectorRequirement{
										{
											Key:      "key",
											Operator: metav1.LabelSelectorOpIn,
											Values:   []string{"value1"},
										},
										{
											Key:      "key",
											Operator: metav1.LabelSelectorOpNotIn,
											Values:   []string{"value2"},
										},
									},
								},
								TopologyKey:    "k8s.io/zone",
								MatchLabelKeys: []string{"key"},
							},
						}},
					},
				}
				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenterTemplate.placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[0].podAffinityTerm[0]", BadValue: "key", Detail: "exists in both matchLabelKeys and labelSelector"},
			},
			expectedErrorString: `spec.datacenterTemplate.placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[0].podAffinityTerm[0]: Invalid value: "key": exists in both matchLabelKeys and labelSelector`,
		},
		{
			name: "invalid hard pod affinity having key in both MatchLabelKeys and labelSelector in datacenter template placement",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.DatacenterTemplate.Placement = &scyllav1alpha1.Placement{
					PodAffinity: &corev1.PodAffinity{
						RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{{
							LabelSelector: &metav1.LabelSelector{
								MatchExpressions: []metav1.LabelSelectorRequirement{
									{
										Key:      "key",
										Operator: metav1.LabelSelectorOpIn,
										Values:   []string{"value1"},
									},
									{
										Key:      "key",
										Operator: metav1.LabelSelectorOpNotIn,
										Values:   []string{"value2"},
									},
								},
							},
							TopologyKey:    "k8s.io/zone",
							MatchLabelKeys: []string{"key"},
						}},
					},
				}
				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenterTemplate.placement.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[0][0]", BadValue: "key", Detail: "exists in both matchLabelKeys and labelSelector"},
			},
			expectedErrorString: `spec.datacenterTemplate.placement.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[0][0]: Invalid value: "key": exists in both matchLabelKeys and labelSelector`,
		},
		{
			name: "invalid soft pod anti-affinity having key in both MatchLabelKeys and labelSelector in datacenter template placement",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.DatacenterTemplate.Placement = &scyllav1alpha1.Placement{
					PodAntiAffinity: &corev1.PodAntiAffinity{
						PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{{
							Weight: 10,
							PodAffinityTerm: corev1.PodAffinityTerm{
								LabelSelector: &metav1.LabelSelector{
									MatchExpressions: []metav1.LabelSelectorRequirement{
										{
											Key:      "key",
											Operator: metav1.LabelSelectorOpIn,
											Values:   []string{"value1"},
										},
										{
											Key:      "key",
											Operator: metav1.LabelSelectorOpNotIn,
											Values:   []string{"value2"},
										},
									},
								},
								TopologyKey:    "k8s.io/zone",
								MatchLabelKeys: []string{"key"},
							},
						}},
					},
				}
				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenterTemplate.placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[0].podAffinityTerm[0]", BadValue: "key", Detail: "exists in both matchLabelKeys and labelSelector"},
			},
			expectedErrorString: `spec.datacenterTemplate.placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[0].podAffinityTerm[0]: Invalid value: "key": exists in both matchLabelKeys and labelSelector`,
		},
		{
			name: "invalid hard pod anti-affinity having key in both MatchLabelKeys and labelSelector in datacenter template placement",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.DatacenterTemplate.Placement = &scyllav1alpha1.Placement{
					PodAntiAffinity: &corev1.PodAntiAffinity{
						RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{{
							LabelSelector: &metav1.LabelSelector{
								MatchExpressions: []metav1.LabelSelectorRequirement{
									{
										Key:      "key",
										Operator: metav1.LabelSelectorOpIn,
										Values:   []string{"value1"},
									},
									{
										Key:      "key",
										Operator: metav1.LabelSelectorOpNotIn,
										Values:   []string{"value2"},
									},
								},
							},
							TopologyKey:    "k8s.io/zone",
							MatchLabelKeys: []string{"key"},
						}},
					},
				}
				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenterTemplate.placement.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[0][0]", BadValue: "key", Detail: "exists in both matchLabelKeys and labelSelector"},
			},
			expectedErrorString: `spec.datacenterTemplate.placement.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[0][0]: Invalid value: "key": exists in both matchLabelKeys and labelSelector`,
		},
		{
			name: "invalid soft pod affinity having key in both MatchLabelKeys and MismatchLabelKeys in datacenter template placement",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.DatacenterTemplate.Placement = &scyllav1alpha1.Placement{
					PodAffinity: &corev1.PodAffinity{
						PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{{
							Weight: 10,
							PodAffinityTerm: corev1.PodAffinityTerm{
								LabelSelector: &metav1.LabelSelector{
									MatchExpressions: []metav1.LabelSelectorRequirement{{
										Key:      "key",
										Operator: metav1.LabelSelectorOpNotIn,
										Values:   []string{"value1", "value2"},
									}},
								},
								TopologyKey:       "k8s.io/zone",
								MatchLabelKeys:    []string{"samekey"},
								MismatchLabelKeys: []string{"samekey"},
							},
						}},
					},
				}
				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenterTemplate.placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[0].podAffinityTerm.matchLabelKeys[0]", BadValue: "samekey", Detail: "exists in both matchLabelKeys and mismatchLabelKeys"},
			},
			expectedErrorString: `spec.datacenterTemplate.placement.podAffinity.preferredDuringSchedulingIgnoredDuringExecution[0].podAffinityTerm.matchLabelKeys[0]: Invalid value: "samekey": exists in both matchLabelKeys and mismatchLabelKeys`,
		},
		{
			name: "invalid hard pod affinity having key in both MatchLabelKeys and MismatchLabelKeys in datacenter template placement",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.DatacenterTemplate.Placement = &scyllav1alpha1.Placement{
					PodAffinity: &corev1.PodAffinity{
						RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{{
							LabelSelector: &metav1.LabelSelector{
								MatchExpressions: []metav1.LabelSelectorRequirement{{
									Key:      "key",
									Operator: metav1.LabelSelectorOpNotIn,
									Values:   []string{"value1", "value2"},
								}},
							},
							TopologyKey:       "k8s.io/zone",
							MatchLabelKeys:    []string{"samekey"},
							MismatchLabelKeys: []string{"samekey"},
						}},
					},
				}
				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenterTemplate.placement.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[0].matchLabelKeys[0]", BadValue: "samekey", Detail: "exists in both matchLabelKeys and mismatchLabelKeys"},
			},
			expectedErrorString: `spec.datacenterTemplate.placement.podAffinity.requiredDuringSchedulingIgnoredDuringExecution[0].matchLabelKeys[0]: Invalid value: "samekey": exists in both matchLabelKeys and mismatchLabelKeys`,
		},
		{
			name: "invalid soft pod anti-affinity having key in both MatchLabelKeys and MismatchLabelKeys in datacenter template placement",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.DatacenterTemplate.Placement = &scyllav1alpha1.Placement{
					PodAntiAffinity: &corev1.PodAntiAffinity{
						PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{{
							Weight: 10,
							PodAffinityTerm: corev1.PodAffinityTerm{
								LabelSelector: &metav1.LabelSelector{
									MatchExpressions: []metav1.LabelSelectorRequirement{{
										Key:      "key",
										Operator: metav1.LabelSelectorOpNotIn,
										Values:   []string{"value1", "value2"},
									}},
								},
								TopologyKey:       "k8s.io/zone",
								MatchLabelKeys:    []string{"samekey"},
								MismatchLabelKeys: []string{"samekey"},
							},
						}},
					},
				}
				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenterTemplate.placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[0].podAffinityTerm.matchLabelKeys[0]", BadValue: "samekey", Detail: "exists in both matchLabelKeys and mismatchLabelKeys"},
			},
			expectedErrorString: `spec.datacenterTemplate.placement.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution[0].podAffinityTerm.matchLabelKeys[0]: Invalid value: "samekey": exists in both matchLabelKeys and mismatchLabelKeys`,
		},
		{
			name: "invalid hard pod anti-affinity having key in both MatchLabelKeys and MismatchLabelKeys in datacenter template placement",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.DatacenterTemplate.Placement = &scyllav1alpha1.Placement{
					PodAntiAffinity: &corev1.PodAntiAffinity{
						RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{{
							LabelSelector: &metav1.LabelSelector{
								MatchExpressions: []metav1.LabelSelectorRequirement{{
									Key:      "key",
									Operator: metav1.LabelSelectorOpNotIn,
									Values:   []string{"value1", "value2"},
								}},
							},
							TopologyKey:       "k8s.io/zone",
							MatchLabelKeys:    []string{"samekey"},
							MismatchLabelKeys: []string{"samekey"},
						}},
					},
				}
				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenterTemplate.placement.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[0].matchLabelKeys[0]", BadValue: "samekey", Detail: "exists in both matchLabelKeys and mismatchLabelKeys"},
			},
			expectedErrorString: `spec.datacenterTemplate.placement.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution[0].matchLabelKeys[0]: Invalid value: "samekey": exists in both matchLabelKeys and mismatchLabelKeys`,
		},
		{
			name: "invalid toleration having invalid toleration key in datacenter template placement",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.DatacenterTemplate.Placement = &scyllav1alpha1.Placement{
					Tolerations: []corev1.Toleration{
						{
							Key:      "nospecialchars^=@",
							Operator: "Equal",
							Value:    "bar",
							Effect:   "NoSchedule",
						},
					},
				}
				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenterTemplate.placement.tolerations[0].key", BadValue: "nospecialchars^=@", Detail: "name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]')", Origin: "format=k8s-label-key"},
			},
			expectedErrorString: `spec.datacenterTemplate.placement.tolerations[0].key: Invalid value: "nospecialchars^=@": name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]')`,
		},
		{
			name: "invalid toleration having invalid value in datacenter template placement",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.DatacenterTemplate.Placement = &scyllav1alpha1.Placement{
					Tolerations: []corev1.Toleration{
						{
							Key:      "foo",
							Operator: "Exists",
							Value:    "bar",
							Effect:   "NoSchedule",
						},
					},
				}
				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenterTemplate.placement.tolerations[0].operator", BadValue: corev1.Toleration{Key: "foo", Operator: "Exists", Value: "bar", Effect: "NoSchedule"}, Detail: "value must be empty when `operator` is 'Exists'"},
			},
			expectedErrorString: "spec.datacenterTemplate.placement.tolerations[0].operator: Invalid value: {\"key\":\"foo\",\"operator\":\"Exists\",\"value\":\"bar\",\"effect\":\"NoSchedule\"}: value must be empty when `operator` is 'Exists'",
		},
		{
			name: "invalid toleration having invalid operator when key is empty in datacenter template placement",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.DatacenterTemplate.Placement = &scyllav1alpha1.Placement{
					Tolerations: []corev1.Toleration{
						{
							Operator: "Equal",
							Value:    "bar",
							Effect:   "NoSchedule",
						},
					},
				}
				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenterTemplate.placement.tolerations[0].operator", BadValue: corev1.TolerationOperator("Equal"), Detail: "operator must be Exists when `key` is empty, which means \"match all values and all keys\""},
			},
			expectedErrorString: "spec.datacenterTemplate.placement.tolerations[0].operator: Invalid value: \"Equal\": operator must be Exists when `key` is empty, which means \"match all values and all keys\"",
		},
		{
			name: "invalid toleration having invalid effect when TolerationsSeconds is set in datacenter template placement",
			cluster: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.DatacenterTemplate.Placement = &scyllav1alpha1.Placement{
					Tolerations: []corev1.Toleration{
						{
							Key:               "node.kubernetes.io/not-ready",
							Operator:          "Exists",
							Effect:            corev1.TaintEffectNoSchedule,
							TolerationSeconds: &[]int64{20}[0],
						},
					},
				}
				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.datacenterTemplate.placement.tolerations[0].effect", BadValue: corev1.TaintEffectNoSchedule, Detail: "effect must be 'NoExecute' when `tolerationSeconds` is set"},
			},
			expectedErrorString: "spec.datacenterTemplate.placement.tolerations[0].effect: Invalid value: \"NoSchedule\": effect must be 'NoExecute' when `tolerationSeconds` is set",
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
			expectedErrorList:   nil,
			expectedErrorString: "",
		},
		{
			name: "cluster name changed",
			old:  newValidScyllaDBCluster(),
			new: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.ClusterName = pointer.Ptr("foo")
				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.clusterName", BadValue: pointer.Ptr("foo"), Detail: `field is immutable`},
			},
			expectedErrorString: `spec.clusterName: Invalid value: "foo": field is immutable`,
		},
		{
			name: "empty rack removed",
			old: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.Datacenters[0].Racks[0].Nodes = pointer.Ptr[int32](0)
				sc.Status = scyllav1alpha1.ScyllaDBClusterStatus{
					ObservedGeneration: pointer.Ptr(sc.Generation),
					Datacenters: []scyllav1alpha1.ScyllaDBClusterDatacenterStatus{
						{
							Name: sc.Spec.Datacenters[0].Name,
							Racks: []scyllav1alpha1.ScyllaDBClusterRackStatus{
								{
									Name:  sc.Spec.Datacenters[0].Racks[0].Name,
									Nodes: pointer.Ptr[int32](0),
									Stale: pointer.Ptr(false),
								},
							},
						},
					},
				}
				return sc
			}(),
			new: func() *scyllav1alpha1.ScyllaDBCluster {
				sc := newValidScyllaDBCluster()
				sc.Spec.Datacenters[0].Racks = nil
				return sc
			}(),
			expectedErrorList:   nil,
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
				&field.Error{Type: field.ErrorTypeForbidden, Field: "spec.datacenters[0].racks[0]", BadValue: "", Detail: `rack "rack" can't be removed because its nodes are being scaled down`},
			},
			expectedErrorString: `spec.datacenters[0].racks[0]: Forbidden: rack "rack" can't be removed because its nodes are being scaled down`,
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
				&field.Error{Type: field.ErrorTypeForbidden, Field: "spec.datacenters[0].racks[0]", BadValue: "", Detail: `rack "rack-0" can't be removed because its nodes are being scaled down`},
				&field.Error{Type: field.ErrorTypeForbidden, Field: "spec.datacenters[0].racks[1]", BadValue: "", Detail: `rack "rack-1" can't be removed because its nodes are being scaled down`},
				&field.Error{Type: field.ErrorTypeForbidden, Field: "spec.datacenters[0].racks[2]", BadValue: "", Detail: `rack "rack-2" can't be removed because its nodes are being scaled down`},
			},
			expectedErrorString: `[spec.datacenters[0].racks[0]: Forbidden: rack "rack-0" can't be removed because its nodes are being scaled down, spec.datacenters[0].racks[1]: Forbidden: rack "rack-1" can't be removed because its nodes are being scaled down, spec.datacenters[0].racks[2]: Forbidden: rack "rack-2" can't be removed because its nodes are being scaled down]`,
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
			expectedErrorString: `spec.datacenterTemplate.scyllaDB.storage: Invalid value: null: field is immutable`,
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
			expectedErrorString: `spec.datacenterTemplate.rackTemplate.scyllaDB.storage: Invalid value: null: field is immutable`,
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
			expectedErrorString: `spec.datacenterTemplate.racks[0].scyllaDB.storage: Invalid value: null: field is immutable`,
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
			expectedErrorString: `spec.datacenters[0].scyllaDB.storage: Invalid value: null: field is immutable`,
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
			expectedErrorString: `spec.datacenters[0].rackTemplate.scyllaDB.storage: Invalid value: null: field is immutable`,
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
			expectedErrorString: `spec.datacenters[0].racks[0].scyllaDB.storage: Invalid value: null: field is immutable`,
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
			expectedErrorString: `spec.exposeOptions.nodeService.type: Invalid value: null: field is immutable`,
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
