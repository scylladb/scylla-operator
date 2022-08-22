package validation_test

import (
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	v1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/api/validation"
	"github.com/scylladb/scylla-operator/pkg/test/unit"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/pointer"
)

func TestValidateScyllaCluster(t *testing.T) {
	validCluster := unit.NewSingleRackCluster(3)
	validCluster.Spec.Datacenter.Racks[0].Resources = corev1.ResourceRequirements{
		Limits: map[corev1.ResourceName]resource.Quantity{
			corev1.ResourceCPU:    resource.MustParse("2"),
			corev1.ResourceMemory: resource.MustParse("2Gi"),
		},
	}

	tests := []struct {
		name                string
		cluster             *v1.ScyllaCluster
		expectedErrorList   field.ErrorList
		expectedErrorString string
	}{
		{
			name:                "valid",
			cluster:             validCluster.DeepCopy(),
			expectedErrorList:   field.ErrorList{},
			expectedErrorString: "",
		},
		{
			name: "two racks with same name",
			cluster: func() *v1.ScyllaCluster {
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
			cluster: func() *v1.ScyllaCluster {
				cluster := validCluster.DeepCopy()
				cluster.Spec.Repairs = append(cluster.Spec.Repairs, v1.RepairTaskSpec{
					Intensity: "100Mib",
				})
				return cluster
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.repairs[0].intensity", BadValue: "100Mib", Detail: "invalid intensity, it must be a float value"},
			},
			expectedErrorString: `spec.repairs[0].intensity: Invalid value: "100Mib": invalid intensity, it must be a float value`,
		},
		{
			name: "invalid intensity in repair task spec && non-unique names in manager tasks spec",
			cluster: func() *v1.ScyllaCluster {
				cluster := validCluster.DeepCopy()
				cluster.Spec.Backups = append(cluster.Spec.Backups, v1.BackupTaskSpec{
					SchedulerTaskSpec: v1.SchedulerTaskSpec{
						Name: "task-name",
					},
				})
				cluster.Spec.Repairs = append(cluster.Spec.Repairs, v1.RepairTaskSpec{
					SchedulerTaskSpec: v1.SchedulerTaskSpec{
						Name: "task-name",
					},
				})
				return cluster
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.repairs[0].intensity", BadValue: "", Detail: "invalid intensity, it must be a float value"},
				&field.Error{Type: field.ErrorTypeDuplicate, Field: "spec.backups[0].name", BadValue: "task-name"},
			},
			expectedErrorString: `[spec.repairs[0].intensity: Invalid value: "": invalid intensity, it must be a float value, spec.backups[0].name: Duplicate value: "task-name"]`,
		},
		{
			name: "when CQL ingress is provided, domains must not be empty",
			cluster: func() *v1.ScyllaCluster {
				cluster := validCluster.DeepCopy()
				cluster.Spec.ExposeOptions = &v1.ExposeOptions{
					CQL: &v1.CQLExposeOptions{
						Ingress: &v1.IngressOptions{},
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
			cluster: func() *v1.ScyllaCluster {
				cluster := validCluster.DeepCopy()
				cluster.Spec.DNSDomains = []string{"-hello"}

				return cluster
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.dnsDomains[0]", BadValue: "-hello", Detail: `a lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character (e.g. 'example.com', regex used for validation is '[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*')`},
			},
			expectedErrorString: `spec.dnsDomains[0]: Invalid value: "-hello": a lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character (e.g. 'example.com', regex used for validation is '[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*')`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			errList := validation.ValidateScyllaCluster(test.cluster)
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

func TestValidateScyllaClusterUpdate(t *testing.T) {
	tests := []struct {
		name                string
		old                 *v1.ScyllaCluster
		new                 *v1.ScyllaCluster
		expectedErrorList   field.ErrorList
		expectedErrorString string
	}{
		{
			name:                "same as old",
			old:                 unit.NewSingleRackCluster(3),
			new:                 unit.NewSingleRackCluster(3),
			expectedErrorList:   field.ErrorList{},
			expectedErrorString: "",
		},
		{
			name:                "major version changed",
			old:                 unit.NewSingleRackCluster(3),
			new:                 unit.NewDetailedSingleRackCluster("test-cluster", "test-ns", "repo", "3.3.1", "test-dc", "test-rack", 3),
			expectedErrorList:   field.ErrorList{},
			expectedErrorString: "",
		},
		{
			name:                "minor version changed",
			old:                 unit.NewSingleRackCluster(3),
			new:                 unit.NewDetailedSingleRackCluster("test-cluster", "test-ns", "repo", "2.4.2", "test-dc", "test-rack", 3),
			expectedErrorList:   field.ErrorList{},
			expectedErrorString: "",
		},
		{
			name:                "patch version changed",
			old:                 unit.NewSingleRackCluster(3),
			new:                 unit.NewDetailedSingleRackCluster("test-cluster", "test-ns", "repo", "2.3.2", "test-dc", "test-rack", 3),
			expectedErrorList:   field.ErrorList{},
			expectedErrorString: "",
		},
		{
			name:                "repo changed",
			old:                 unit.NewSingleRackCluster(3),
			new:                 unit.NewDetailedSingleRackCluster("test-cluster", "test-ns", "new-repo", "2.3.2", "test-dc", "test-rack", 3),
			expectedErrorList:   field.ErrorList{},
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
			expectedErrorList:   field.ErrorList{},
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
			expectedErrorList:   field.ErrorList{},
			expectedErrorString: "",
		},
		{
			name:                "empty rack removed",
			old:                 unit.NewSingleRackCluster(0),
			new:                 racksDeleted(unit.NewSingleRackCluster(0)),
			expectedErrorList:   field.ErrorList{},
			expectedErrorString: "",
		},
		{
			name: "empty rack with members under decommission",
			old:  withStatus(unit.NewSingleRackCluster(0), v1.ScyllaClusterStatus{Racks: map[string]v1.RackStatus{"test-rack": {Members: 3}}}),
			new:  racksDeleted(unit.NewSingleRackCluster(0)),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeForbidden, Field: "spec.datacenter.racks[0]", BadValue: "", Detail: `rack "test-rack" can't be removed because the members are being scaled down`},
			},
			expectedErrorString: `spec.datacenter.racks[0]: Forbidden: rack "test-rack" can't be removed because the members are being scaled down`,
		},
		{
			name: "empty rack with stale status",
			old:  withStatus(unit.NewSingleRackCluster(0), v1.ScyllaClusterStatus{Racks: map[string]v1.RackStatus{"test-rack": {Stale: pointer.Bool(true), Members: 0}}}),
			new:  racksDeleted(unit.NewSingleRackCluster(0)),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInternal, Field: "spec.datacenter.racks[0]", Detail: `rack "test-rack" can't be removed because its status, that's used to determine members count, is not yet up to date with the generation of this resource; please retry later`},
			},
			expectedErrorString: `spec.datacenter.racks[0]: Internal error: rack "test-rack" can't be removed because its status, that's used to determine members count, is not yet up to date with the generation of this resource; please retry later`,
		},
		{
			name: "empty rack with not reconciled generation",
			old:  withStatus(withGeneration(unit.NewSingleRackCluster(0), 123), v1.ScyllaClusterStatus{ObservedGeneration: pointer.Int64(321), Racks: map[string]v1.RackStatus{"test-rack": {Members: 0}}}),
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

func withGeneration(sc *v1.ScyllaCluster, generation int64) *v1.ScyllaCluster {
	sc.Generation = generation
	return sc
}

func withStatus(sc *v1.ScyllaCluster, status v1.ScyllaClusterStatus) *v1.ScyllaCluster {
	sc.Status = status
	return sc
}

func placementChanged(c *v1.ScyllaCluster) *v1.ScyllaCluster {
	c.Spec.Datacenter.Racks[0].Placement = &v1.PlacementSpec{}
	return c
}

func resourceChanged(c *v1.ScyllaCluster) *v1.ScyllaCluster {
	c.Spec.Datacenter.Racks[0].Resources.Requests = map[corev1.ResourceName]resource.Quantity{
		corev1.ResourceCPU: *resource.NewMilliQuantity(1000, resource.DecimalSI),
	}
	return c
}

func racksDeleted(c *v1.ScyllaCluster) *v1.ScyllaCluster {
	c.Spec.Datacenter.Racks = nil
	return c
}

func storageChanged(c *v1.ScyllaCluster) *v1.ScyllaCluster {
	c.Spec.Datacenter.Racks[0].Storage.Capacity = "15Gi"
	return c
}
