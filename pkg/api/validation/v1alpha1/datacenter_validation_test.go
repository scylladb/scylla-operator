// Copyright (c) 2022 ScyllaDB.

package v1alpha1_test

import (
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	validationv1alpha1 "github.com/scylladb/scylla-operator/pkg/api/validation/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/test/unit"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/pointer"
)

func TestValidateScyllaDatacenter(t *testing.T) {
	tests := []struct {
		name                string
		obj                 *scyllav1alpha1.ScyllaDatacenter
		expectedErrorList   field.ErrorList
		expectedErrorString string
	}{
		{
			name: "valid",
			obj: func() *scyllav1alpha1.ScyllaDatacenter {
				cluster := unit.NewSingleRackDatacenter(3)
				return cluster
			}(),
			expectedErrorList:   field.ErrorList{},
			expectedErrorString: "",
		},
		{
			name: "two racks with same name",
			obj: func() *scyllav1alpha1.ScyllaDatacenter {
				cluster := unit.NewSingleRackDatacenter(3)
				cluster.Spec.Racks = append(cluster.Spec.Racks, *cluster.Spec.Racks[0].DeepCopy())
				return cluster
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeDuplicate, Field: "spec.racks[1].name", BadValue: "test-rack"},
			},
			expectedErrorString: `spec.racks[1].name: Duplicate value: "test-rack"`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			errList := validationv1alpha1.ValidateScyllaDatacenter(test.obj)
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

func TestValidateScyllaDatacenterUpdate(t *testing.T) {
	tests := []struct {
		name                string
		old                 *scyllav1alpha1.ScyllaDatacenter
		new                 *scyllav1alpha1.ScyllaDatacenter
		expectedErrorList   field.ErrorList
		expectedErrorString string
	}{
		{
			name:                "same as old",
			old:                 unit.NewSingleRackDatacenter(3),
			new:                 unit.NewSingleRackDatacenter(3),
			expectedErrorList:   field.ErrorList{},
			expectedErrorString: "",
		},
		{
			name:                "major version changed",
			old:                 unit.NewSingleRackDatacenter(3),
			new:                 unit.NewDetailedSingleRackDatacenter("test-cluster", "test-ns", "repo:3.3.1", "test-dc", "test-rack", 3),
			expectedErrorList:   field.ErrorList{},
			expectedErrorString: "",
		},
		{
			name:                "minor version changed",
			old:                 unit.NewSingleRackDatacenter(3),
			new:                 unit.NewDetailedSingleRackDatacenter("test-cluster", "test-ns", "repo:2.4.2", "test-dc", "test-rack", 3),
			expectedErrorList:   field.ErrorList{},
			expectedErrorString: "",
		},
		{
			name:                "patch version changed",
			old:                 unit.NewSingleRackDatacenter(3),
			new:                 unit.NewDetailedSingleRackDatacenter("test-cluster", "test-ns", "repo:2.3.2", "test-dc", "test-rack", 3),
			expectedErrorList:   field.ErrorList{},
			expectedErrorString: "",
		},
		{
			name:                "repo changed",
			old:                 unit.NewSingleRackDatacenter(3),
			new:                 unit.NewDetailedSingleRackDatacenter("test-cluster", "test-ns", "new-repo:2.3.2", "test-dc", "test-rack", 3),
			expectedErrorList:   field.ErrorList{},
			expectedErrorString: "",
		},
		{
			name: "dcName changed",
			old:  unit.NewSingleRackDatacenter(3),
			new:  unit.NewDetailedSingleRackDatacenter("test-cluster", "test-ns", "repo:2.3.1", "new-dc", "test-rack", 3),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeForbidden, Field: "spec.datacenterName", BadValue: "", Detail: "change of datacenter name is currently not supported"},
			},
			expectedErrorString: "spec.datacenterName: Forbidden: change of datacenter name is currently not supported",
		},
		{
			name:                "rackPlacement changed",
			old:                 unit.NewSingleRackDatacenter(3),
			new:                 placementChanged(unit.NewSingleRackDatacenter(3)),
			expectedErrorList:   field.ErrorList{},
			expectedErrorString: "",
		},
		{
			name: "rackStorage changed",
			old:  unit.NewSingleRackDatacenter(3),
			new:  storageChanged(unit.NewSingleRackDatacenter(3)),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeForbidden, Field: "spec.racks[0].scylla.storage", BadValue: "", Detail: "changes in storage are currently not supported"},
			},
			expectedErrorString: "spec.racks[0].scylla.storage: Forbidden: changes in storage are currently not supported",
		},
		{
			name:                "rackResources changed",
			old:                 unit.NewSingleRackDatacenter(3),
			new:                 resourceChanged(unit.NewSingleRackDatacenter(3)),
			expectedErrorList:   field.ErrorList{},
			expectedErrorString: "",
		},
		{
			name:                "empty rack removed",
			old:                 unit.NewSingleRackDatacenter(0),
			new:                 racksDeleted(unit.NewSingleRackDatacenter(0)),
			expectedErrorList:   field.ErrorList{},
			expectedErrorString: "",
		},
		{
			name: "empty rack with members under decommission",
			old:  withStatus(unit.NewSingleRackDatacenter(0), scyllav1alpha1.ScyllaDatacenterStatus{Racks: map[string]scyllav1alpha1.RackStatus{"test-rack": {Nodes: pointer.Int32(3)}}}),
			new:  racksDeleted(unit.NewSingleRackDatacenter(0)),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeForbidden, Field: "spec.racks[0]", BadValue: "", Detail: `rack "test-rack" can't be removed because the members are being scaled down`},
			},
			expectedErrorString: `spec.racks[0]: Forbidden: rack "test-rack" can't be removed because the members are being scaled down`,
		},
		{
			name: "empty rack with stale status",
			old:  withStatus(unit.NewSingleRackDatacenter(0), scyllav1alpha1.ScyllaDatacenterStatus{Racks: map[string]scyllav1alpha1.RackStatus{"test-rack": {Stale: pointer.Bool(true), Nodes: pointer.Int32(0)}}}),
			new:  racksDeleted(unit.NewSingleRackDatacenter(0)),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInternal, Field: "spec.racks[0]", Detail: `rack "test-rack" can't be removed because its status, that's used to determine members count, is not yet up to date with the generation of this resource; please retry later`},
			},
			expectedErrorString: `spec.racks[0]: Internal error: rack "test-rack" can't be removed because its status, that's used to determine members count, is not yet up to date with the generation of this resource; please retry later`,
		},
		{
			name: "empty rack with not reconciled generation",
			old:  withStatus(withGeneration(unit.NewSingleRackDatacenter(0), 123), scyllav1alpha1.ScyllaDatacenterStatus{ObservedGeneration: pointer.Int64(321), Racks: map[string]scyllav1alpha1.RackStatus{"test-rack": {Nodes: pointer.Int32(0)}}}),
			new:  racksDeleted(unit.NewSingleRackDatacenter(0)),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeInternal, Field: "spec.racks[0]", Detail: `rack "test-rack" can't be removed because its status, that's used to determine members count, is not yet up to date with the generation of this resource; please retry later`},
			},
			expectedErrorString: `spec.racks[0]: Internal error: rack "test-rack" can't be removed because its status, that's used to determine members count, is not yet up to date with the generation of this resource; please retry later`,
		},
		{
			name: "non-empty racks deleted",
			old:  unit.NewMultiRackDatacenter(3, 2, 1, 0),
			new:  racksDeleted(unit.NewSingleRackDatacenter(3)),
			expectedErrorList: field.ErrorList{
				&field.Error{Type: field.ErrorTypeForbidden, Field: "spec.racks[0]", BadValue: "", Detail: `rack "rack-0" can't be removed because it still has members that have to be scaled down to zero first`},
				&field.Error{Type: field.ErrorTypeForbidden, Field: "spec.racks[1]", BadValue: "", Detail: `rack "rack-1" can't be removed because it still has members that have to be scaled down to zero first`},
				&field.Error{Type: field.ErrorTypeForbidden, Field: "spec.racks[2]", BadValue: "", Detail: `rack "rack-2" can't be removed because it still has members that have to be scaled down to zero first`},
			},
			expectedErrorString: `[spec.racks[0]: Forbidden: rack "rack-0" can't be removed because it still has members that have to be scaled down to zero first, spec.racks[1]: Forbidden: rack "rack-1" can't be removed because it still has members that have to be scaled down to zero first, spec.racks[2]: Forbidden: rack "rack-2" can't be removed because it still has members that have to be scaled down to zero first]`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			errList := validationv1alpha1.ValidateScyllaDatacenterUpdate(test.new, test.old)
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

func withGeneration(sc *scyllav1alpha1.ScyllaDatacenter, generation int64) *scyllav1alpha1.ScyllaDatacenter {
	sc.Generation = generation
	return sc
}

func withStatus(sc *scyllav1alpha1.ScyllaDatacenter, status scyllav1alpha1.ScyllaDatacenterStatus) *scyllav1alpha1.ScyllaDatacenter {
	sc.Status = status
	return sc
}

func placementChanged(c *scyllav1alpha1.ScyllaDatacenter) *scyllav1alpha1.ScyllaDatacenter {
	c.Spec.Racks[0].Placement = &scyllav1alpha1.Placement{}
	return c
}

func resourceChanged(c *scyllav1alpha1.ScyllaDatacenter) *scyllav1alpha1.ScyllaDatacenter {
	c.Spec.Racks[0].Scylla.Resources = &corev1.ResourceRequirements{
		Requests: map[corev1.ResourceName]resource.Quantity{
			corev1.ResourceCPU: *resource.NewMilliQuantity(1111, resource.DecimalSI),
		},
	}
	return c
}

func racksDeleted(c *scyllav1alpha1.ScyllaDatacenter) *scyllav1alpha1.ScyllaDatacenter {
	c.Spec.Racks = nil
	return c
}

func storageChanged(c *scyllav1alpha1.ScyllaDatacenter) *scyllav1alpha1.ScyllaDatacenter {
	c.Spec.Racks[0].Scylla.Storage.Resources.Requests[corev1.ResourceStorage] = resource.MustParse("15Gi")
	return c
}
