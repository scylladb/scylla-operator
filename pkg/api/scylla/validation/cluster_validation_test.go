package validation_test

import (
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/api/scylla/validation"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	"github.com/scylladb/scylla-operator/pkg/test/unit"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func TestValidateScyllaCluster(t *testing.T) {
	t.Parallel()

	validCluster := unit.NewSingleRackCluster(3)
	validCluster.Spec.Datacenter.Racks[0].Resources = corev1.ResourceRequirements{
		Limits: map[corev1.ResourceName]resource.Quantity{
			corev1.ResourceCPU:    resource.MustParse("2"),
			corev1.ResourceMemory: resource.MustParse("2Gi"),
		},
	}

	tests := []struct {
		name                string
		cluster             *scyllav1.ScyllaCluster
		expectedErrorList   field.ErrorList
		expectedErrorString string
	}{
		{
			name:                "valid",
			cluster:             validCluster.DeepCopy(),
			expectedErrorList:   nil,
			expectedErrorString: "",
		},
		{
			name: "two racks with same name",
			cluster: func() *scyllav1.ScyllaCluster {
				cluster := validCluster.DeepCopy()
				cluster.Spec.Datacenter.Racks = append(cluster.Spec.Datacenter.Racks, *cluster.Spec.Datacenter.Racks[0].DeepCopy())
				return cluster
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeDuplicate, Field: "spec.datacenter.racks[1].name", BadValue: "test-rack"},
			},
			expectedErrorString: `spec.datacenter.racks[1].name: Duplicate value: "test-rack"`,
		},
		{
			name: "invalid intensity in repair task spec",
			cluster: func() *scyllav1.ScyllaCluster {
				cluster := validCluster.DeepCopy()
				cluster.Spec.Repairs = append(cluster.Spec.Repairs, scyllav1.RepairTaskSpec{
					Intensity:           "100Mib",
					SmallTableThreshold: "1GiB",
				})
				return cluster
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.repairs[0].intensity", BadValue: "100Mib", Detail: "must be a float"},
			},
			expectedErrorString: `spec.repairs[0].intensity: Invalid value: "100Mib": must be a float`,
		},
		{
			name: "non-unique names across manager tasks of different types",
			cluster: func() *scyllav1.ScyllaCluster {
				cluster := validCluster.DeepCopy()
				cluster.Spec.Repairs = []scyllav1.RepairTaskSpec{
					{
						TaskSpec: scyllav1.TaskSpec{
							Name: "task-name",
						},
						Intensity: "1",
					},
				}
				cluster.Spec.Backups = []scyllav1.BackupTaskSpec{
					{
						TaskSpec: scyllav1.TaskSpec{
							Name: "task-name",
						},
					},
				}
				return cluster
			}(),
			expectedErrorList:   nil,
			expectedErrorString: "",
		},
		{
			name: "non-unique names in manager repair tasks spec",
			cluster: func() *scyllav1.ScyllaCluster {
				cluster := validCluster.DeepCopy()
				cluster.Spec.Repairs = []scyllav1.RepairTaskSpec{
					{
						TaskSpec: scyllav1.TaskSpec{
							Name: "task-name",
						},
						Intensity: "1",
					},
					{
						TaskSpec: scyllav1.TaskSpec{
							Name: "task-name",
						},
						Intensity: "1",
					},
				}
				return cluster
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeDuplicate,
					Field:    "spec.repairs[1].name",
					BadValue: "task-name",
				},
			},
			expectedErrorString: `spec.repairs[1].name: Duplicate value: "task-name"`,
		},
		{
			name: "non-unique names in manager backup tasks spec",
			cluster: func() *scyllav1.ScyllaCluster {
				cluster := validCluster.DeepCopy()
				cluster.Spec.Backups = []scyllav1.BackupTaskSpec{
					{
						TaskSpec: scyllav1.TaskSpec{
							Name: "task-name",
						},
					},
					{
						TaskSpec: scyllav1.TaskSpec{
							Name: "task-name",
						},
					},
				}
				return cluster
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeDuplicate,
					Field:    "spec.backups[1].name",
					BadValue: "task-name",
				},
			},
			expectedErrorString: `spec.backups[1].name: Duplicate value: "task-name"`,
		},
		{
			name: "invalid cron in manager repair task spec",
			cluster: func() *scyllav1.ScyllaCluster {
				cluster := validCluster.DeepCopy()
				cluster.Spec.Repairs = append(cluster.Spec.Repairs, scyllav1.RepairTaskSpec{
					TaskSpec: scyllav1.TaskSpec{
						Name: "task-name",
						SchedulerTaskSpec: scyllav1.SchedulerTaskSpec{
							Cron: pointer.Ptr("invalid"),
						},
					},
					Intensity: "1",
				})
				return cluster
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeInvalid,
					Field:    "spec.repairs[0].cron",
					BadValue: pointer.Ptr("invalid"),
					Detail:   "expected 5 to 6 fields, found 1: [invalid]",
				},
			},
			expectedErrorString: `spec.repairs[0].cron: Invalid value: "invalid": expected 5 to 6 fields, found 1: [invalid]`,
		},
		{
			name: "unallowed TZ in cron in manager repair task spec",
			cluster: func() *scyllav1.ScyllaCluster {
				cluster := validCluster.DeepCopy()
				cluster.Spec.Repairs = append(cluster.Spec.Repairs, scyllav1.RepairTaskSpec{
					TaskSpec: scyllav1.TaskSpec{
						Name: "task-name",
						SchedulerTaskSpec: scyllav1.SchedulerTaskSpec{
							Cron: pointer.Ptr("TZ=Europe/Warsaw 0 23 * * SAT"),
						},
					},
					Intensity: "1",
				})
				return cluster
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeInvalid,
					Field:    "spec.repairs[0].cron",
					BadValue: pointer.Ptr("TZ=Europe/Warsaw 0 23 * * SAT"),
					Detail:   "can't use TZ or CRON_TZ in cron, use timezone instead",
				},
			},
			expectedErrorString: `spec.repairs[0].cron: Invalid value: "TZ=Europe/Warsaw 0 23 * * SAT": can't use TZ or CRON_TZ in cron, use timezone instead`,
		},
		{
			name: "unallowed TZ in cron in manager backup task spec",
			cluster: func() *scyllav1.ScyllaCluster {
				cluster := validCluster.DeepCopy()
				cluster.Spec.Backups = append(cluster.Spec.Backups, scyllav1.BackupTaskSpec{
					TaskSpec: scyllav1.TaskSpec{
						Name: "task-name",
						SchedulerTaskSpec: scyllav1.SchedulerTaskSpec{
							Cron: pointer.Ptr("TZ=Europe/Warsaw 0 23 * * SAT"),
						},
					},
				})
				return cluster
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeInvalid,
					Field:    "spec.backups[0].cron",
					BadValue: pointer.Ptr("TZ=Europe/Warsaw 0 23 * * SAT"),
					Detail:   "can't use TZ or CRON_TZ in cron, use timezone instead",
				},
			},
			expectedErrorString: `spec.backups[0].cron: Invalid value: "TZ=Europe/Warsaw 0 23 * * SAT": can't use TZ or CRON_TZ in cron, use timezone instead`,
		},
		{
			name: "unallowed CRON_TZ in cron in manager repair task spec",
			cluster: func() *scyllav1.ScyllaCluster {
				cluster := validCluster.DeepCopy()
				cluster.Spec.Repairs = append(cluster.Spec.Repairs, scyllav1.RepairTaskSpec{
					TaskSpec: scyllav1.TaskSpec{
						Name: "task-name",
						SchedulerTaskSpec: scyllav1.SchedulerTaskSpec{
							Cron: pointer.Ptr("CRON_TZ=Europe/Warsaw 0 23 * * SAT"),
						},
					},
					Intensity: "1",
				})
				return cluster
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeInvalid,
					Field:    "spec.repairs[0].cron",
					BadValue: pointer.Ptr("CRON_TZ=Europe/Warsaw 0 23 * * SAT"),
					Detail:   "can't use TZ or CRON_TZ in cron, use timezone instead",
				},
			},
			expectedErrorString: `spec.repairs[0].cron: Invalid value: "CRON_TZ=Europe/Warsaw 0 23 * * SAT": can't use TZ or CRON_TZ in cron, use timezone instead`,
		},
		{
			name: "unallowed CRON_TZ in cron in manager backup task spec",
			cluster: func() *scyllav1.ScyllaCluster {
				cluster := validCluster.DeepCopy()
				cluster.Spec.Backups = append(cluster.Spec.Backups, scyllav1.BackupTaskSpec{
					TaskSpec: scyllav1.TaskSpec{
						Name: "task-name",
						SchedulerTaskSpec: scyllav1.SchedulerTaskSpec{
							Cron: pointer.Ptr("CRON_TZ=Europe/Warsaw 0 23 * * SAT"),
						},
					},
				})
				return cluster
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeInvalid,
					Field:    "spec.backups[0].cron",
					BadValue: pointer.Ptr("CRON_TZ=Europe/Warsaw 0 23 * * SAT"),
					Detail:   "can't use TZ or CRON_TZ in cron, use timezone instead",
				},
			},
			expectedErrorString: `spec.backups[0].cron: Invalid value: "CRON_TZ=Europe/Warsaw 0 23 * * SAT": can't use TZ or CRON_TZ in cron, use timezone instead`,
		},
		{
			name: "invalid timezone in manager repair task spec",
			cluster: func() *scyllav1.ScyllaCluster {
				cluster := validCluster.DeepCopy()
				cluster.Spec.Repairs = append(cluster.Spec.Repairs, scyllav1.RepairTaskSpec{
					TaskSpec: scyllav1.TaskSpec{
						Name: "task-name",
						SchedulerTaskSpec: scyllav1.SchedulerTaskSpec{
							Cron:     pointer.Ptr("0 23 * * SAT"),
							Timezone: pointer.Ptr("invalid"),
						},
					},
					Intensity: "1",
				})
				return cluster
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeInvalid,
					Field:    "spec.repairs[0].timezone",
					BadValue: pointer.Ptr("invalid"),
					Detail:   "unknown time zone invalid",
				},
			},
			expectedErrorString: `spec.repairs[0].timezone: Invalid value: "invalid": unknown time zone invalid`,
		},
		{
			name: "invalid timezone in manager backup task spec",
			cluster: func() *scyllav1.ScyllaCluster {
				cluster := validCluster.DeepCopy()
				cluster.Spec.Backups = append(cluster.Spec.Backups, scyllav1.BackupTaskSpec{
					TaskSpec: scyllav1.TaskSpec{
						Name: "task-name",
						SchedulerTaskSpec: scyllav1.SchedulerTaskSpec{
							Cron:     pointer.Ptr("0 23 * * SAT"),
							Timezone: pointer.Ptr("invalid"),
						},
					},
				})
				return cluster
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeInvalid,
					Field:    "spec.backups[0].timezone",
					BadValue: pointer.Ptr("invalid"),
					Detail:   "unknown time zone invalid",
				},
			},
			expectedErrorString: `spec.backups[0].timezone: Invalid value: "invalid": unknown time zone invalid`,
		},
		{
			name: "cron and timezone in manager repair task spec",
			cluster: func() *scyllav1.ScyllaCluster {
				cluster := validCluster.DeepCopy()
				cluster.Spec.Repairs = append(cluster.Spec.Repairs, scyllav1.RepairTaskSpec{
					TaskSpec: scyllav1.TaskSpec{
						Name: "task-name",
						SchedulerTaskSpec: scyllav1.SchedulerTaskSpec{
							Cron:     pointer.Ptr("0 23 * * SAT"),
							Timezone: pointer.Ptr("Europe/Warsaw"),
						},
					},
					Intensity: "1",
				})
				return cluster
			}(),
			expectedErrorList:   nil,
			expectedErrorString: "",
		},
		{
			name: "cron and timezone in manager backup task spec",
			cluster: func() *scyllav1.ScyllaCluster {
				cluster := validCluster.DeepCopy()
				cluster.Spec.Backups = append(cluster.Spec.Backups, scyllav1.BackupTaskSpec{
					TaskSpec: scyllav1.TaskSpec{
						Name: "task-name",
						SchedulerTaskSpec: scyllav1.SchedulerTaskSpec{
							Cron:     pointer.Ptr("0 23 * * SAT"),
							Timezone: pointer.Ptr("Europe/Warsaw"),
						},
					},
				})
				return cluster
			}(),
			expectedErrorList:   nil,
			expectedErrorString: "",
		},
		{
			name: "nil cron and invalid interval in manager repair task spec",
			cluster: func() *scyllav1.ScyllaCluster {
				cluster := validCluster.DeepCopy()
				cluster.Spec.Repairs = append(cluster.Spec.Repairs, scyllav1.RepairTaskSpec{
					TaskSpec: scyllav1.TaskSpec{
						Name: "task-name",
						SchedulerTaskSpec: scyllav1.SchedulerTaskSpec{
							Interval: pointer.Ptr("invalid"),
							Cron:     nil,
						},
					},
					Intensity: "1",
				})
				return cluster
			}(),
			expectedErrorList:   nil,
			expectedErrorString: "",
		},
		{
			name: "nil cron and invalid interval in manager backup task spec",
			cluster: func() *scyllav1.ScyllaCluster {
				cluster := validCluster.DeepCopy()
				cluster.Spec.Backups = append(cluster.Spec.Backups, scyllav1.BackupTaskSpec{
					TaskSpec: scyllav1.TaskSpec{
						Name: "task-name",
						SchedulerTaskSpec: scyllav1.SchedulerTaskSpec{
							Interval: pointer.Ptr("invalid"),
							Cron:     nil,
						},
					},
				})
				return cluster
			}(),
			expectedErrorList:   nil,
			expectedErrorString: "",
		},
		{
			name: "nil cron and non-zero interval in manager repair task spec",
			cluster: func() *scyllav1.ScyllaCluster {
				cluster := validCluster.DeepCopy()
				cluster.Spec.Repairs = append(cluster.Spec.Repairs, scyllav1.RepairTaskSpec{
					TaskSpec: scyllav1.TaskSpec{
						Name: "task-name",
						SchedulerTaskSpec: scyllav1.SchedulerTaskSpec{
							Interval: pointer.Ptr("7d"),
							Cron:     nil,
						},
					},
					Intensity: "1",
				})
				return cluster
			}(),
			expectedErrorList:   nil,
			expectedErrorString: "",
		},
		{
			name: "nil cron and non-zero interval in manager backup task spec",
			cluster: func() *scyllav1.ScyllaCluster {
				cluster := validCluster.DeepCopy()
				cluster.Spec.Backups = append(cluster.Spec.Backups, scyllav1.BackupTaskSpec{
					TaskSpec: scyllav1.TaskSpec{
						Name: "task-name",
						SchedulerTaskSpec: scyllav1.SchedulerTaskSpec{
							Interval: pointer.Ptr("7d"),
							Cron:     nil,
						},
					},
				})
				return cluster
			}(),
			expectedErrorList:   nil,
			expectedErrorString: "",
		},
		{
			name: "cron and invalid interval in manager repair task spec",
			cluster: func() *scyllav1.ScyllaCluster {
				cluster := validCluster.DeepCopy()
				cluster.Spec.Repairs = append(cluster.Spec.Repairs, scyllav1.RepairTaskSpec{
					TaskSpec: scyllav1.TaskSpec{
						Name: "task-name",
						SchedulerTaskSpec: scyllav1.SchedulerTaskSpec{
							Interval: pointer.Ptr("invalid"),
							Cron:     pointer.Ptr("0 23 * * SAT"),
						},
					},
					Intensity: "1",
				})
				return cluster
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeInvalid,
					Field:    "spec.repairs[0].interval",
					BadValue: pointer.Ptr("invalid"),
					Detail:   "valid units are d, h, m, s",
				},
			},
			expectedErrorString: `spec.repairs[0].interval: Invalid value: "invalid": valid units are d, h, m, s`,
		},
		{
			name: "cron and invalid interval in manager backup task spec",
			cluster: func() *scyllav1.ScyllaCluster {
				cluster := validCluster.DeepCopy()
				cluster.Spec.Backups = append(cluster.Spec.Backups, scyllav1.BackupTaskSpec{
					TaskSpec: scyllav1.TaskSpec{
						Name: "task-name",
						SchedulerTaskSpec: scyllav1.SchedulerTaskSpec{
							Interval: pointer.Ptr("invalid"),
							Cron:     pointer.Ptr("0 23 * * SAT"),
						},
					},
				})
				return cluster
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeInvalid,
					Field:    "spec.backups[0].interval",
					BadValue: pointer.Ptr("invalid"),
					Detail:   "valid units are d, h, m, s",
				},
			},
			expectedErrorString: `spec.backups[0].interval: Invalid value: "invalid": valid units are d, h, m, s`,
		},
		{
			name: "cron and zero interval in manager repair task spec",
			cluster: func() *scyllav1.ScyllaCluster {
				cluster := validCluster.DeepCopy()
				cluster.Spec.Repairs = append(cluster.Spec.Repairs, []scyllav1.RepairTaskSpec{
					{
						TaskSpec: scyllav1.TaskSpec{
							Name: "task-name",
							SchedulerTaskSpec: scyllav1.SchedulerTaskSpec{
								Interval: pointer.Ptr("0"),
								Cron:     pointer.Ptr("0 23 * * SAT"),
							},
						},
						Intensity: "1",
					},
					{
						TaskSpec: scyllav1.TaskSpec{
							Name: "other-task-name",
							SchedulerTaskSpec: scyllav1.SchedulerTaskSpec{
								Interval: pointer.Ptr("0d"),
								Cron:     pointer.Ptr("0 23 * * SAT"),
							},
						},
						Intensity: "1",
					},
				}...)
				return cluster
			}(),
			expectedErrorList:   nil,
			expectedErrorString: "",
		},
		{
			name: "cron and zero interval in manager backup task spec",
			cluster: func() *scyllav1.ScyllaCluster {
				cluster := validCluster.DeepCopy()
				cluster.Spec.Backups = append(cluster.Spec.Backups, []scyllav1.BackupTaskSpec{
					{
						TaskSpec: scyllav1.TaskSpec{
							Name: "task-name",
							SchedulerTaskSpec: scyllav1.SchedulerTaskSpec{
								Interval: pointer.Ptr("0"),
								Cron:     pointer.Ptr("0 23 * * SAT"),
							},
						},
					},
					{
						TaskSpec: scyllav1.TaskSpec{
							Name: "other-task-name",
							SchedulerTaskSpec: scyllav1.SchedulerTaskSpec{
								Interval: pointer.Ptr("0d"),
								Cron:     pointer.Ptr("0 23 * * SAT"),
							},
						},
					},
				}...)
				return cluster
			}(),
			expectedErrorList:   nil,
			expectedErrorString: "",
		},
		{
			name: "cron and non-zero interval in manager repair task spec",
			cluster: func() *scyllav1.ScyllaCluster {
				cluster := validCluster.DeepCopy()
				cluster.Spec.Repairs = append(cluster.Spec.Repairs, scyllav1.RepairTaskSpec{
					TaskSpec: scyllav1.TaskSpec{
						Name: "task-name",
						SchedulerTaskSpec: scyllav1.SchedulerTaskSpec{
							Interval: pointer.Ptr("7d"),
							Cron:     pointer.Ptr("0 23 * * SAT"),
						},
					},
					Intensity: "1",
				})
				return cluster
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeForbidden,
					Field:    "spec.repairs[0].interval",
					BadValue: "",
					Detail:   "can't be non-zero when cron is specified",
				},
			},
			expectedErrorString: `spec.repairs[0].interval: Forbidden: can't be non-zero when cron is specified`,
		},
		{
			name: "cron and non-zero interval in manager backup task spec",
			cluster: func() *scyllav1.ScyllaCluster {
				cluster := validCluster.DeepCopy()
				cluster.Spec.Backups = append(cluster.Spec.Backups, scyllav1.BackupTaskSpec{
					TaskSpec: scyllav1.TaskSpec{
						Name: "task-name",
						SchedulerTaskSpec: scyllav1.SchedulerTaskSpec{
							Interval: pointer.Ptr("7d"),
							Cron:     pointer.Ptr("0 23 * * SAT"),
						},
					},
				})
				return cluster
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeForbidden,
					Field:    "spec.backups[0].interval",
					BadValue: "",
					Detail:   "can't be non-zero when cron is specified",
				},
			},
			expectedErrorString: `spec.backups[0].interval: Forbidden: can't be non-zero when cron is specified`,
		},
		{
			name: "timezone and nil cron in manager repair task spec",
			cluster: func() *scyllav1.ScyllaCluster {
				cluster := validCluster.DeepCopy()
				cluster.Spec.Repairs = append(cluster.Spec.Repairs, scyllav1.RepairTaskSpec{
					TaskSpec: scyllav1.TaskSpec{
						Name: "task-name",
						SchedulerTaskSpec: scyllav1.SchedulerTaskSpec{
							Cron:     nil,
							Timezone: pointer.Ptr("UTC"),
						},
					},
					Intensity: "1",
				})
				return cluster
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeForbidden,
					Field:    "spec.repairs[0].timezone",
					BadValue: "",
					Detail:   "can't be set when cron is not specified",
				},
			},
			expectedErrorString: `spec.repairs[0].timezone: Forbidden: can't be set when cron is not specified`,
		},
		{
			name: "timezone and nil cron in manager backup task spec",
			cluster: func() *scyllav1.ScyllaCluster {
				cluster := validCluster.DeepCopy()
				cluster.Spec.Backups = append(cluster.Spec.Backups, scyllav1.BackupTaskSpec{
					TaskSpec: scyllav1.TaskSpec{
						Name: "task-name",
						SchedulerTaskSpec: scyllav1.SchedulerTaskSpec{
							Cron:     nil,
							Timezone: pointer.Ptr("UTC"),
						},
					},
				})
				return cluster
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeForbidden,
					Field:    "spec.backups[0].timezone",
					BadValue: "",
					Detail:   "can't be set when cron is not specified",
				},
			},
			expectedErrorString: `spec.backups[0].timezone: Forbidden: can't be set when cron is not specified`,
		},
		{
			name: "when CQL ingress is provided, domains must not be empty",
			cluster: func() *scyllav1.ScyllaCluster {
				cluster := validCluster.DeepCopy()
				cluster.Spec.ExposeOptions = &scyllav1.ExposeOptions{
					CQL: &scyllav1.CQLExposeOptions{
						Ingress: &scyllav1.IngressOptions{},
					},
				}

				return cluster
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeRequired, Field: "spec.dnsDomains", BadValue: "", Detail: "at least one domain needs to be provided when exposing CQL via ingresses"},
			},
			expectedErrorString: `spec.dnsDomains: Required value: at least one domain needs to be provided when exposing CQL via ingresses`,
		},
		{
			name: "invalid domain",
			cluster: func() *scyllav1.ScyllaCluster {
				cluster := validCluster.DeepCopy()
				cluster.Spec.DNSDomains = []string{"-hello"}

				return cluster
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.dnsDomains[0]", BadValue: "-hello", Detail: `a lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character (e.g. 'example.com', regex used for validation is '[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*')`},
			},
			expectedErrorString: `spec.dnsDomains[0]: Invalid value: "-hello": a lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character (e.g. 'example.com', regex used for validation is '[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*')`,
		},
		{
			name: "unsupported type of node service",
			cluster: func() *scyllav1.ScyllaCluster {
				cluster := validCluster.DeepCopy()
				cluster.Spec.Version = "5.2.0"
				cluster.Spec.ExposeOptions = &scyllav1.ExposeOptions{
					NodeService: &scyllav1.NodeServiceTemplate{
						Type: "foo",
					},
				}

				return cluster
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeNotSupported, Field: "spec.exposeOptions.nodeService.type", BadValue: scyllav1.NodeServiceType("foo"), Detail: `supported values: "Headless", "ClusterIP", "LoadBalancer"`},
			},
			expectedErrorString: `spec.exposeOptions.nodeService.type: Unsupported value: "foo": supported values: "Headless", "ClusterIP", "LoadBalancer"`,
		},
		{
			name: "invalid load balancer class name in node service template",
			cluster: func() *scyllav1.ScyllaCluster {
				cluster := validCluster.DeepCopy()
				cluster.Spec.ExposeOptions = &scyllav1.ExposeOptions{
					NodeService: &scyllav1.NodeServiceTemplate{
						Type:              scyllav1.NodeServiceTypeClusterIP,
						LoadBalancerClass: pointer.Ptr("-hello"),
					},
				}

				return cluster
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.exposeOptions.nodeService.loadBalancerClass", BadValue: "-hello", Detail: `name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]')`},
			},
			expectedErrorString: `spec.exposeOptions.nodeService.loadBalancerClass: Invalid value: "-hello": name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]')`,
		},
		{
			name: "EKS NLB LoadBalancerClass is valid",
			cluster: func() *scyllav1.ScyllaCluster {
				cluster := validCluster.DeepCopy()
				cluster.Spec.ExposeOptions = &scyllav1.ExposeOptions{
					NodeService: &scyllav1.NodeServiceTemplate{
						Type:              scyllav1.NodeServiceTypeLoadBalancer,
						LoadBalancerClass: pointer.Ptr("service.k8s.aws/nlb"),
					},
				}

				return cluster
			}(),
			expectedErrorList:   nil,
			expectedErrorString: "",
		},
		{
			name: "unsupported type of client broadcast address",
			cluster: func() *scyllav1.ScyllaCluster {
				cluster := validCluster.DeepCopy()
				cluster.Spec.Version = "5.2.0"
				cluster.Spec.ExposeOptions = &scyllav1.ExposeOptions{
					BroadcastOptions: &scyllav1.NodeBroadcastOptions{
						Nodes: scyllav1.BroadcastOptions{
							Type: scyllav1.BroadcastAddressTypeServiceClusterIP,
						},
						Clients: scyllav1.BroadcastOptions{
							Type: "foo",
						},
					},
				}

				return cluster
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeNotSupported, Field: "spec.exposeOptions.broadcastOptions.clients.type", BadValue: scyllav1.BroadcastAddressType("foo"), Detail: `supported values: "PodIP", "ServiceClusterIP", "ServiceLoadBalancerIngress"`},
			},
			expectedErrorString: `spec.exposeOptions.broadcastOptions.clients.type: Unsupported value: "foo": supported values: "PodIP", "ServiceClusterIP", "ServiceLoadBalancerIngress"`,
		},
		{
			name: "unsupported type of node broadcast address",
			cluster: func() *scyllav1.ScyllaCluster {
				cluster := validCluster.DeepCopy()
				cluster.Spec.Version = "5.2.0"
				cluster.Spec.ExposeOptions = &scyllav1.ExposeOptions{
					BroadcastOptions: &scyllav1.NodeBroadcastOptions{
						Nodes: scyllav1.BroadcastOptions{
							Type: "foo",
						},
						Clients: scyllav1.BroadcastOptions{
							Type: scyllav1.BroadcastAddressTypeServiceClusterIP,
						},
					},
				}

				return cluster
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeNotSupported, Field: "spec.exposeOptions.broadcastOptions.nodes.type", BadValue: scyllav1.BroadcastAddressType("foo"), Detail: `supported values: "PodIP", "ServiceClusterIP", "ServiceLoadBalancerIngress"`},
			},
			expectedErrorString: `spec.exposeOptions.broadcastOptions.nodes.type: Unsupported value: "foo": supported values: "PodIP", "ServiceClusterIP", "ServiceLoadBalancerIngress"`,
		},
		{
			name: "invalid ClusterIP broadcast type when node service is Headless",
			cluster: func() *scyllav1.ScyllaCluster {
				cluster := validCluster.DeepCopy()
				cluster.Spec.Version = "5.2.0"
				cluster.Spec.ExposeOptions = &scyllav1.ExposeOptions{
					NodeService: &scyllav1.NodeServiceTemplate{
						Type: scyllav1.NodeServiceTypeHeadless,
					},
					BroadcastOptions: &scyllav1.NodeBroadcastOptions{
						Clients: scyllav1.BroadcastOptions{
							Type: scyllav1.BroadcastAddressTypeServiceClusterIP,
						},
						Nodes: scyllav1.BroadcastOptions{
							Type: scyllav1.BroadcastAddressTypeServiceClusterIP,
						},
					},
				}

				return cluster
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.exposeOptions.broadcastOptions.clients.type", BadValue: scyllav1.BroadcastAddressTypeServiceClusterIP, Detail: `can't broadcast address unavailable within the selected node service type, allowed types for chosen broadcast address type are: [ClusterIP LoadBalancer]`},
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.exposeOptions.broadcastOptions.nodes.type", BadValue: scyllav1.BroadcastAddressTypeServiceClusterIP, Detail: `can't broadcast address unavailable within the selected node service type, allowed types for chosen broadcast address type are: [ClusterIP LoadBalancer]`},
			},
			expectedErrorString: `[spec.exposeOptions.broadcastOptions.clients.type: Invalid value: "ServiceClusterIP": can't broadcast address unavailable within the selected node service type, allowed types for chosen broadcast address type are: [ClusterIP LoadBalancer], spec.exposeOptions.broadcastOptions.nodes.type: Invalid value: "ServiceClusterIP": can't broadcast address unavailable within the selected node service type, allowed types for chosen broadcast address type are: [ClusterIP LoadBalancer]]`,
		},
		{
			name: "invalid LoadBalancerIngressIP broadcast type when node service is Headless",
			cluster: func() *scyllav1.ScyllaCluster {
				cluster := validCluster.DeepCopy()
				cluster.Spec.Version = "5.2.0"
				cluster.Spec.ExposeOptions = &scyllav1.ExposeOptions{
					NodeService: &scyllav1.NodeServiceTemplate{
						Type: scyllav1.NodeServiceTypeHeadless,
					},
					BroadcastOptions: &scyllav1.NodeBroadcastOptions{
						Clients: scyllav1.BroadcastOptions{
							Type: scyllav1.BroadcastAddressTypeServiceLoadBalancerIngress,
						},
						Nodes: scyllav1.BroadcastOptions{
							Type: scyllav1.BroadcastAddressTypeServiceLoadBalancerIngress,
						},
					},
				}

				return cluster
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.exposeOptions.broadcastOptions.clients.type", BadValue: scyllav1.BroadcastAddressTypeServiceLoadBalancerIngress, Detail: `can't broadcast address unavailable within the selected node service type, allowed types for chosen broadcast address type are: [LoadBalancer]`},
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.exposeOptions.broadcastOptions.nodes.type", BadValue: scyllav1.BroadcastAddressTypeServiceLoadBalancerIngress, Detail: `can't broadcast address unavailable within the selected node service type, allowed types for chosen broadcast address type are: [LoadBalancer]`},
			},
			expectedErrorString: `[spec.exposeOptions.broadcastOptions.clients.type: Invalid value: "ServiceLoadBalancerIngress": can't broadcast address unavailable within the selected node service type, allowed types for chosen broadcast address type are: [LoadBalancer], spec.exposeOptions.broadcastOptions.nodes.type: Invalid value: "ServiceLoadBalancerIngress": can't broadcast address unavailable within the selected node service type, allowed types for chosen broadcast address type are: [LoadBalancer]]`,
		},
		{
			name: "invalid LoadBalancerIngressIP broadcast type when node service is ClusterIP",
			cluster: func() *scyllav1.ScyllaCluster {
				cluster := validCluster.DeepCopy()
				cluster.Spec.Version = "5.2.0"
				cluster.Spec.ExposeOptions = &scyllav1.ExposeOptions{
					NodeService: &scyllav1.NodeServiceTemplate{
						Type: scyllav1.NodeServiceTypeClusterIP,
					},
					BroadcastOptions: &scyllav1.NodeBroadcastOptions{
						Clients: scyllav1.BroadcastOptions{
							Type: scyllav1.BroadcastAddressTypeServiceLoadBalancerIngress,
						},
						Nodes: scyllav1.BroadcastOptions{
							Type: scyllav1.BroadcastAddressTypeServiceLoadBalancerIngress,
						},
					},
				}

				return cluster
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.exposeOptions.broadcastOptions.clients.type", BadValue: scyllav1.BroadcastAddressTypeServiceLoadBalancerIngress, Detail: `can't broadcast address unavailable within the selected node service type, allowed types for chosen broadcast address type are: [LoadBalancer]`},
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.exposeOptions.broadcastOptions.nodes.type", BadValue: scyllav1.BroadcastAddressTypeServiceLoadBalancerIngress, Detail: `can't broadcast address unavailable within the selected node service type, allowed types for chosen broadcast address type are: [LoadBalancer]`},
			},
			expectedErrorString: `[spec.exposeOptions.broadcastOptions.clients.type: Invalid value: "ServiceLoadBalancerIngress": can't broadcast address unavailable within the selected node service type, allowed types for chosen broadcast address type are: [LoadBalancer], spec.exposeOptions.broadcastOptions.nodes.type: Invalid value: "ServiceLoadBalancerIngress": can't broadcast address unavailable within the selected node service type, allowed types for chosen broadcast address type are: [LoadBalancer]]`,
		},
		{
			name: "negative minTerminationGracePeriodSeconds",
			cluster: func() *scyllav1.ScyllaCluster {
				cluster := validCluster.DeepCopy()
				cluster.Spec.MinTerminationGracePeriodSeconds = pointer.Ptr(int32(-42))

				return cluster
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.minTerminationGracePeriodSeconds", BadValue: int32(-42), Detail: "must be non-negative integer"},
			},
			expectedErrorString: `spec.minTerminationGracePeriodSeconds: Invalid value: -42: must be non-negative integer`,
		},
		{
			name: "negative minReadySeconds",
			cluster: func() *scyllav1.ScyllaCluster {
				cluster := validCluster.DeepCopy()
				cluster.Spec.MinReadySeconds = pointer.Ptr(int32(-42))

				return cluster
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.minReadySeconds", BadValue: int32(-42), Detail: "must be non-negative integer"},
			},
			expectedErrorString: `spec.minReadySeconds: Invalid value: -42: must be non-negative integer`,
		},
		{
			name: "minimal alternator cluster passes",
			cluster: func() *scyllav1.ScyllaCluster {
				cluster := validCluster.DeepCopy()
				cluster.Spec.Alternator = &scyllav1.AlternatorSpec{}
				return cluster
			}(),
			expectedErrorList:   nil,
			expectedErrorString: "",
		},
		{
			name: "alternator cluster with user certificate",
			cluster: func() *scyllav1.ScyllaCluster {
				cluster := validCluster.DeepCopy()
				cluster.Spec.Alternator = &scyllav1.AlternatorSpec{
					ServingCertificate: &scyllav1.TLSCertificate{
						Type: scyllav1.TLSCertificateTypeUserManaged,
						UserManagedOptions: &scyllav1.UserManagedTLSCertificateOptions{
							SecretName: "my-tls-certificate",
						},
					},
				}
				return cluster
			}(),
			expectedErrorList:   nil,
			expectedErrorString: "",
		},
		{
			name: "alternator cluster with invalid certificate type",
			cluster: func() *scyllav1.ScyllaCluster {
				cluster := validCluster.DeepCopy()
				cluster.Spec.Alternator = &scyllav1.AlternatorSpec{
					ServingCertificate: &scyllav1.TLSCertificate{
						Type: "foo",
					},
				}
				return cluster
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeNotSupported, Field: "spec.alternator.servingCertificate.type", BadValue: scyllav1.TLSCertificateType("foo"), Detail: `supported values: "OperatorManaged", "UserManaged"`},
			},
			expectedErrorString: `spec.alternator.servingCertificate.type: Unsupported value: "foo": supported values: "OperatorManaged", "UserManaged"`,
		},
		{
			name: "alternator cluster with valid additional domains",
			cluster: func() *scyllav1.ScyllaCluster {
				cluster := validCluster.DeepCopy()
				cluster.Spec.Alternator = &scyllav1.AlternatorSpec{
					ServingCertificate: &scyllav1.TLSCertificate{
						Type: scyllav1.TLSCertificateTypeOperatorManaged,
						OperatorManagedOptions: &scyllav1.OperatorManagedTLSCertificateOptions{
							AdditionalDNSNames: []string{"scylla-operator.scylladb.com"},
						},
					},
				}
				return cluster
			}(),
			expectedErrorList:   nil,
			expectedErrorString: "",
		},
		{
			name: "alternator cluster with invalid additional domains",
			cluster: func() *scyllav1.ScyllaCluster {
				cluster := validCluster.DeepCopy()
				cluster.Spec.Alternator = &scyllav1.AlternatorSpec{
					ServingCertificate: &scyllav1.TLSCertificate{
						Type: scyllav1.TLSCertificateTypeOperatorManaged,
						OperatorManagedOptions: &scyllav1.OperatorManagedTLSCertificateOptions{
							AdditionalDNSNames: []string{"[not a domain]"},
						},
					},
				}
				return cluster
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.alternator.servingCertificate.operatorManagedOptions.additionalDNSNames", BadValue: []string{"[not a domain]"}, Detail: `a lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character (e.g. 'example.com', regex used for validation is '[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*')`},
			},
			expectedErrorString: `spec.alternator.servingCertificate.operatorManagedOptions.additionalDNSNames: Invalid value: ["[not a domain]"]: a lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character (e.g. 'example.com', regex used for validation is '[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*')`,
		},
		{
			name: "alternator cluster with valid additional IP addresses",
			cluster: func() *scyllav1.ScyllaCluster {
				cluster := validCluster.DeepCopy()
				cluster.Spec.Alternator = &scyllav1.AlternatorSpec{
					ServingCertificate: &scyllav1.TLSCertificate{
						Type: scyllav1.TLSCertificateTypeOperatorManaged,
						OperatorManagedOptions: &scyllav1.OperatorManagedTLSCertificateOptions{
							AdditionalIPAddresses: []string{"127.0.0.1", "::1"},
						},
					},
				}
				return cluster
			}(),
			expectedErrorList:   nil,
			expectedErrorString: "",
		},
		{
			name: "alternator cluster with invalid additional IP addresses",
			cluster: func() *scyllav1.ScyllaCluster {
				cluster := validCluster.DeepCopy()
				cluster.Spec.Alternator = &scyllav1.AlternatorSpec{
					ServingCertificate: &scyllav1.TLSCertificate{
						Type: scyllav1.TLSCertificateTypeOperatorManaged,
						OperatorManagedOptions: &scyllav1.OperatorManagedTLSCertificateOptions{
							AdditionalIPAddresses: []string{"0.not-an-ip.0.0"},
						},
					},
				}
				return cluster
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.alternator.servingCertificate.operatorManagedOptions.additionalIPAddresses", BadValue: "0.not-an-ip.0.0", Detail: `must be a valid IP address, (e.g. 10.9.8.7 or 2001:db8::ffff)`, Origin: "format=ip-strict"},
			},
			expectedErrorString: `spec.alternator.servingCertificate.operatorManagedOptions.additionalIPAddresses: Invalid value: "0.not-an-ip.0.0": must be a valid IP address, (e.g. 10.9.8.7 or 2001:db8::ffff)`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			errList := validation.ValidateScyllaCluster(test.cluster)
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

func TestValidateScyllaClusterUpdate(t *testing.T) {
	tests := []struct {
		name                string
		old                 *scyllav1.ScyllaCluster
		new                 *scyllav1.ScyllaCluster
		expectedErrorList   field.ErrorList
		expectedErrorString string
	}{
		{
			name:                "same as old",
			old:                 unit.NewSingleRackCluster(3),
			new:                 unit.NewSingleRackCluster(3),
			expectedErrorList:   nil,
			expectedErrorString: "",
		},
		{
			name:                "major version changed",
			old:                 unit.NewSingleRackCluster(3),
			new:                 unit.NewDetailedSingleRackCluster("test-cluster", "test-ns", "repo", "3.3.1", "test-dc", "test-rack", 3),
			expectedErrorList:   nil,
			expectedErrorString: "",
		},
		{
			name:                "minor version changed",
			old:                 unit.NewSingleRackCluster(3),
			new:                 unit.NewDetailedSingleRackCluster("test-cluster", "test-ns", "repo", "2.4.2", "test-dc", "test-rack", 3),
			expectedErrorList:   nil,
			expectedErrorString: "",
		},
		{
			name:                "patch version changed",
			old:                 unit.NewSingleRackCluster(3),
			new:                 unit.NewDetailedSingleRackCluster("test-cluster", "test-ns", "repo", "2.3.2", "test-dc", "test-rack", 3),
			expectedErrorList:   nil,
			expectedErrorString: "",
		},
		{
			name:                "repo changed",
			old:                 unit.NewSingleRackCluster(3),
			new:                 unit.NewDetailedSingleRackCluster("test-cluster", "test-ns", "new-repo", "2.3.2", "test-dc", "test-rack", 3),
			expectedErrorList:   nil,
			expectedErrorString: "",
		},
		{
			name: "dcName changed",
			old:  unit.NewSingleRackCluster(3),
			new:  unit.NewDetailedSingleRackCluster("test-cluster", "test-ns", "repo", "2.3.1", "new-dc", "test-rack", 3),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeForbidden, Field: "spec.datacenter.name", BadValue: "", Detail: "change of datacenter name is currently not supported"},
			},
			expectedErrorString: "spec.datacenter.name: Forbidden: change of datacenter name is currently not supported",
		},
		{
			name:                "rackPlacement changed",
			old:                 unit.NewSingleRackCluster(3),
			new:                 placementChanged(unit.NewSingleRackCluster(3)),
			expectedErrorList:   nil,
			expectedErrorString: "",
		},
		{
			name: "rackStorage changed",
			old:  unit.NewSingleRackCluster(3),
			new:  storageChanged(unit.NewSingleRackCluster(3)),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeForbidden, Field: "spec.datacenter.racks[0].storage", BadValue: "", Detail: "changes in storage are currently not supported"},
			},
			expectedErrorString: "spec.datacenter.racks[0].storage: Forbidden: changes in storage are currently not supported",
		},
		{
			name:                "rackResources changed",
			old:                 unit.NewSingleRackCluster(3),
			new:                 resourceChanged(unit.NewSingleRackCluster(3)),
			expectedErrorList:   nil,
			expectedErrorString: "",
		},
		{
			name:                "empty rack removed",
			old:                 unit.NewSingleRackCluster(0),
			new:                 racksDeleted(unit.NewSingleRackCluster(0)),
			expectedErrorList:   nil,
			expectedErrorString: "",
		},
		{
			name: "empty rack with members under decommission",
			old:  withStatus(unit.NewSingleRackCluster(0), scyllav1.ScyllaClusterStatus{Racks: map[string]scyllav1.RackStatus{"test-rack": {Members: 3}}}),
			new:  racksDeleted(unit.NewSingleRackCluster(0)),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeForbidden, Field: "spec.datacenter.racks[0]", BadValue: "", Detail: `rack "test-rack" can't be removed because the members are being scaled down`},
			},
			expectedErrorString: `spec.datacenter.racks[0]: Forbidden: rack "test-rack" can't be removed because the members are being scaled down`,
		},
		{
			name: "empty rack with stale status",
			old:  withStatus(unit.NewSingleRackCluster(0), scyllav1.ScyllaClusterStatus{Racks: map[string]scyllav1.RackStatus{"test-rack": {Stale: pointer.Ptr(true), Members: 0}}}),
			new:  racksDeleted(unit.NewSingleRackCluster(0)),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInternal, Field: "spec.datacenter.racks[0]", Detail: `rack "test-rack" can't be removed because its status, that's used to determine members count, is not yet up to date with the generation of this resource; please retry later`},
			},
			expectedErrorString: `spec.datacenter.racks[0]: Internal error: rack "test-rack" can't be removed because its status, that's used to determine members count, is not yet up to date with the generation of this resource; please retry later`,
		},
		{
			name: "empty rack with not reconciled generation",
			old:  withStatus(withGeneration(unit.NewSingleRackCluster(0), 123), scyllav1.ScyllaClusterStatus{ObservedGeneration: pointer.Ptr(int64(321)), Racks: map[string]scyllav1.RackStatus{"test-rack": {Members: 0}}}),
			new:  racksDeleted(unit.NewSingleRackCluster(0)),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInternal, Field: "spec.datacenter.racks[0]", Detail: `rack "test-rack" can't be removed because its status, that's used to determine members count, is not yet up to date with the generation of this resource; please retry later`},
			},
			expectedErrorString: `spec.datacenter.racks[0]: Internal error: rack "test-rack" can't be removed because its status, that's used to determine members count, is not yet up to date with the generation of this resource; please retry later`,
		},
		{
			name: "non-empty racks deleted",
			old:  unit.NewMultiRackCluster(3, 2, 1, 0),
			new:  racksDeleted(unit.NewSingleRackCluster(3)),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeForbidden, Field: "spec.datacenter.racks[0]", BadValue: "", Detail: `rack "rack-0" can't be removed because it still has members that have to be scaled down to zero first`},
				&field.Error{Type: field.ErrorTypeForbidden, Field: "spec.datacenter.racks[1]", BadValue: "", Detail: `rack "rack-1" can't be removed because it still has members that have to be scaled down to zero first`},
				&field.Error{Type: field.ErrorTypeForbidden, Field: "spec.datacenter.racks[2]", BadValue: "", Detail: `rack "rack-2" can't be removed because it still has members that have to be scaled down to zero first`},
			},
			expectedErrorString: `[spec.datacenter.racks[0]: Forbidden: rack "rack-0" can't be removed because it still has members that have to be scaled down to zero first, spec.datacenter.racks[1]: Forbidden: rack "rack-1" can't be removed because it still has members that have to be scaled down to zero first, spec.datacenter.racks[2]: Forbidden: rack "rack-2" can't be removed because it still has members that have to be scaled down to zero first]`,
		},
		{
			name: "node service type cannot be unset",
			old: func() *scyllav1.ScyllaCluster {
				sc := unit.NewSingleRackCluster(3)
				sc.Spec.ExposeOptions = &scyllav1.ExposeOptions{
					NodeService: &scyllav1.NodeServiceTemplate{
						Type: scyllav1.NodeServiceTypeClusterIP,
					},
				}
				return sc
			}(),
			new: func() *scyllav1.ScyllaCluster {
				sc := unit.NewSingleRackCluster(3)
				sc.Spec.ExposeOptions = nil
				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.exposeOptions.nodeService.type", BadValue: (*scyllav1.NodeServiceType)(nil), Detail: `field is immutable`},
			},
			expectedErrorString: `spec.exposeOptions.nodeService.type: Invalid value: null: field is immutable`,
		},
		{
			name: "node service type cannot be changed",
			old: func() *scyllav1.ScyllaCluster {
				sc := unit.NewSingleRackCluster(3)
				sc.Spec.ExposeOptions = &scyllav1.ExposeOptions{
					NodeService: &scyllav1.NodeServiceTemplate{
						Type: scyllav1.NodeServiceTypeHeadless,
					},
				}
				return sc
			}(),
			new: func() *scyllav1.ScyllaCluster {
				sc := unit.NewSingleRackCluster(3)
				sc.Spec.ExposeOptions = &scyllav1.ExposeOptions{
					NodeService: &scyllav1.NodeServiceTemplate{
						Type: scyllav1.NodeServiceTypeClusterIP,
					},
				}
				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.exposeOptions.nodeService.type", BadValue: pointer.Ptr(scyllav1.NodeServiceTypeClusterIP), Detail: `field is immutable`},
			},
			expectedErrorString: `spec.exposeOptions.nodeService.type: Invalid value: "ClusterIP": field is immutable`,
		},
		{
			name: "clients broadcast address type cannot be changed",
			old: func() *scyllav1.ScyllaCluster {
				sc := unit.NewSingleRackCluster(3)
				sc.Spec.Version = "5.2.0"
				sc.Spec.ExposeOptions = &scyllav1.ExposeOptions{
					BroadcastOptions: &scyllav1.NodeBroadcastOptions{
						Clients: scyllav1.BroadcastOptions{
							Type: scyllav1.BroadcastAddressTypeServiceClusterIP,
						},
						Nodes: scyllav1.BroadcastOptions{
							Type: scyllav1.BroadcastAddressTypeServiceClusterIP,
						},
					},
				}
				return sc
			}(),
			new: func() *scyllav1.ScyllaCluster {
				sc := unit.NewSingleRackCluster(3)
				sc.Spec.Version = "5.2.0"
				sc.Spec.ExposeOptions = &scyllav1.ExposeOptions{
					BroadcastOptions: &scyllav1.NodeBroadcastOptions{
						Clients: scyllav1.BroadcastOptions{
							Type: scyllav1.BroadcastAddressTypePodIP,
						},
						Nodes: scyllav1.BroadcastOptions{
							Type: scyllav1.BroadcastAddressTypeServiceClusterIP,
						},
					},
				}
				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.exposeOptions.broadcastOptions.clients.type", BadValue: pointer.Ptr(scyllav1.BroadcastAddressTypePodIP), Detail: `field is immutable`},
			},
			expectedErrorString: `spec.exposeOptions.broadcastOptions.clients.type: Invalid value: "PodIP": field is immutable`,
		},
		{
			name: "nodes broadcast address type cannot be changed",
			old: func() *scyllav1.ScyllaCluster {
				sc := unit.NewSingleRackCluster(3)
				sc.Spec.Version = "5.2.0"
				sc.Spec.ExposeOptions = &scyllav1.ExposeOptions{
					BroadcastOptions: &scyllav1.NodeBroadcastOptions{
						Nodes: scyllav1.BroadcastOptions{
							Type: scyllav1.BroadcastAddressTypeServiceClusterIP,
						},
						Clients: scyllav1.BroadcastOptions{
							Type: scyllav1.BroadcastAddressTypeServiceClusterIP,
						},
					},
				}
				return sc
			}(),
			new: func() *scyllav1.ScyllaCluster {
				sc := unit.NewSingleRackCluster(3)
				sc.Spec.Version = "5.2.0"
				sc.Spec.ExposeOptions = &scyllav1.ExposeOptions{
					BroadcastOptions: &scyllav1.NodeBroadcastOptions{
						Nodes: scyllav1.BroadcastOptions{
							Type: scyllav1.BroadcastAddressTypePodIP,
						},
						Clients: scyllav1.BroadcastOptions{
							Type: scyllav1.BroadcastAddressTypeServiceClusterIP,
						},
					},
				}
				return sc
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.exposeOptions.broadcastOptions.nodes.type", BadValue: pointer.Ptr(scyllav1.BroadcastAddressTypePodIP), Detail: `field is immutable`},
			},
			expectedErrorString: `spec.exposeOptions.broadcastOptions.nodes.type: Invalid value: "PodIP": field is immutable`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			errList := validation.ValidateScyllaClusterUpdate(test.new, test.old)
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

func withGeneration(sc *scyllav1.ScyllaCluster, generation int64) *scyllav1.ScyllaCluster {
	sc.Generation = generation
	return sc
}

func withStatus(sc *scyllav1.ScyllaCluster, status scyllav1.ScyllaClusterStatus) *scyllav1.ScyllaCluster {
	sc.Status = status
	return sc
}

func placementChanged(c *scyllav1.ScyllaCluster) *scyllav1.ScyllaCluster {
	c.Spec.Datacenter.Racks[0].Placement = &scyllav1.PlacementSpec{}
	return c
}

func resourceChanged(c *scyllav1.ScyllaCluster) *scyllav1.ScyllaCluster {
	c.Spec.Datacenter.Racks[0].Resources.Requests = map[corev1.ResourceName]resource.Quantity{
		corev1.ResourceCPU: *resource.NewMilliQuantity(1000, resource.DecimalSI),
	}
	return c
}

func racksDeleted(c *scyllav1.ScyllaCluster) *scyllav1.ScyllaCluster {
	c.Spec.Datacenter.Racks = nil
	return c
}

func storageChanged(c *scyllav1.ScyllaCluster) *scyllav1.ScyllaCluster {
	c.Spec.Datacenter.Racks[0].Storage.Capacity = "15Gi"
	return c
}

func TestGetWarningsOnScyllaClusterCreate(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name             string
		cluster          *scyllav1.ScyllaCluster
		expectedWarnings []string
	}{
		{
			name:             "no warnings",
			cluster:          unit.NewSingleRackCluster(3),
			expectedWarnings: nil,
		},
		{
			name: "sysctls deprecation warning",
			cluster: func() *scyllav1.ScyllaCluster {
				sc := unit.NewSingleRackCluster(3)
				sc.Spec.Sysctls = []string{
					"fs.aio-max-nr=30000000",
				}
				return sc
			}(),
			expectedWarnings: []string{
				"spec.sysctls: deprecated; use NodeConfig's .spec.sysctls instead",
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			warnings := validation.GetWarningsOnScyllaClusterCreate(tc.cluster)
			if !reflect.DeepEqual(tc.expectedWarnings, warnings) {
				t.Errorf("expected and actual warnings differ: %s", cmp.Diff(tc.expectedWarnings, warnings))
			}
		})
	}
}

func TestGetWarningsOnScyllaClusterUpdate(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name             string
		oldSC            *scyllav1.ScyllaCluster
		newSC            *scyllav1.ScyllaCluster
		expectedWarnings []string
	}{
		{
			name:             "no warnings",
			oldSC:            unit.NewSingleRackCluster(3),
			newSC:            unit.NewSingleRackCluster(3),
			expectedWarnings: nil,
		},
		{
			name:  "sysctls deprecation warning",
			oldSC: unit.NewSingleRackCluster(3),
			newSC: func() *scyllav1.ScyllaCluster {
				sc := unit.NewSingleRackCluster(3)
				sc.Spec.Sysctls = []string{
					"fs.aio-max-nr=30000000",
				}
				return sc
			}(),
			expectedWarnings: []string{
				"spec.sysctls: deprecated; use NodeConfig's .spec.sysctls instead",
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			warnings := validation.GetWarningsOnScyllaClusterUpdate(tc.newSC, tc.oldSC)
			if !reflect.DeepEqual(tc.expectedWarnings, warnings) {
				t.Errorf("expected and actual warnings differ: %s", cmp.Diff(tc.expectedWarnings, warnings))
			}
		})
	}
}
