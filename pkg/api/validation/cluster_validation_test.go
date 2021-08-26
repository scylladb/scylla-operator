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
)

func TestValidateScyllaCluster(t *testing.T) {
	validCluster := unit.NewSingleRackCluster(3)
	validCluster.Spec.Datacenter.Racks[0].Resources = corev1.ResourceRequirements{
		Limits: map[corev1.ResourceName]resource.Quantity{
			corev1.ResourceCPU:    resource.MustParse("2"),
			corev1.ResourceMemory: resource.MustParse("2Gi"),
		},
	}

	sameName := validCluster.DeepCopy()
	sameName.Spec.Datacenter.Racks = append(sameName.Spec.Datacenter.Racks, sameName.Spec.Datacenter.Racks[0])

	invalidIntensity := validCluster.DeepCopy()
	invalidIntensity.Spec.Repairs = append(invalidIntensity.Spec.Repairs, v1.RepairTaskSpec{
		Intensity: "100Mib",
	})

	nonUniqueManagerTaskNames := validCluster.DeepCopy()
	nonUniqueManagerTaskNames.Spec.Backups = append(nonUniqueManagerTaskNames.Spec.Backups, v1.BackupTaskSpec{
		SchedulerTaskSpec: v1.SchedulerTaskSpec{
			Name: "task-name",
		},
	})
	nonUniqueManagerTaskNames.Spec.Repairs = append(nonUniqueManagerTaskNames.Spec.Repairs, v1.RepairTaskSpec{
		SchedulerTaskSpec: v1.SchedulerTaskSpec{
			Name: "task-name",
		},
	})

	tests := []struct {
		name                string
		obj                 *v1.ScyllaCluster
		expectedErrorList   field.ErrorList
		expectedErrorString string
	}{
		{
			name:                "valid",
			obj:                 validCluster,
			expectedErrorList:   field.ErrorList{},
			expectedErrorString: "",
		},
		{
			name: "two racks with same name",
			obj:  sameName,
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeDuplicate, Field: "spec.datacenter.racks[1].name", BadValue: "test-rack"},
			},
			expectedErrorString: `spec.datacenter.racks[1].name: Duplicate value: "test-rack"`,
		},
		{
			name: "invalid intensity in repair task spec",
			obj:  invalidIntensity,
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.repairs[0].intensity", BadValue: "100Mib", Detail: "invalid intensity, it must be a float value"},
			},
			expectedErrorString: `spec.repairs[0].intensity: Invalid value: "100Mib": invalid intensity, it must be a float value`,
		},
		{
			name: "invalid intensity in repair task spec && non-unique names in manager tasks spec",
			obj:  nonUniqueManagerTaskNames,
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInvalid, Field: "spec.repairs[0].intensity", BadValue: "", Detail: "invalid intensity, it must be a float value"},
				&field.Error{Type: field.ErrorTypeDuplicate, Field: "spec.backups[0].name", BadValue: "task-name"},
			},
			expectedErrorString: `[spec.repairs[0].intensity: Invalid value: "": invalid intensity, it must be a float value, spec.backups[0].name: Duplicate value: "task-name"]`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			errList := validation.ValidateScyllaCluster(test.obj)
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
			name: "rack deleted",
			old:  unit.NewSingleRackCluster(3),
			new:  rackDeleted(unit.NewSingleRackCluster(3)),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeForbidden, Field: "spec.datacenter.racks", BadValue: "", Detail: "racks [test-rack] not found, you cannot remove racks from the spec"},
			},
			expectedErrorString: "spec.datacenter.racks: Forbidden: racks [test-rack] not found, you cannot remove racks from the spec",
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

func rackDeleted(c *v1.ScyllaCluster) *v1.ScyllaCluster {
	c.Spec.Datacenter.Racks = nil
	return c
}

func storageChanged(c *v1.ScyllaCluster) *v1.ScyllaCluster {
	c.Spec.Datacenter.Racks[0].Storage.Capacity = "15Gi"
	return c
}
