package validating

import (
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/apis/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controller/cluster/util"
	"github.com/scylladb/scylla-operator/pkg/test/unit"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"testing"
)

func TestCheckValues(t *testing.T) {

	validCluster := unit.NewSingleRackCluster(3)
	validCluster.Spec.Datacenter.Racks[0].Resources = corev1.ResourceRequirements{
		Limits: map[corev1.ResourceName]resource.Quantity{
			corev1.ResourceCPU:    resource.MustParse("2"),
			corev1.ResourceMemory: resource.MustParse("2Gi"),
		},
	}

	sameName := validCluster.DeepCopy()
	sameName.Spec.Datacenter.Racks = append(sameName.Spec.Datacenter.Racks, sameName.Spec.Datacenter.Racks[0])

	tests := []struct {
		name    string
		obj     *scyllav1alpha1.Cluster
		allowed bool
	}{
		{
			name:    "valid",
			obj:     validCluster,
			allowed: true,
		},
		{
			name:    "two racks with same name",
			obj:     sameName,
			allowed: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			allowed, msg := checkValues(test.obj)
			require.Equalf(t, test.allowed, allowed, "Wrong value returned from checkValues function. Message: '%s'", msg)
		})
	}
}

func TestCheckTransitions(t *testing.T) {

	old := unit.NewSingleRackCluster(3)

	versionChanged := old.DeepCopy()
	versionChanged.Spec.Version = "100.100.100"

	repoChanged := old.DeepCopy()
	repoChanged.Spec.Repository = util.RefFromString("my-private-repo")

	sidecarImageChanged := old.DeepCopy()
	sidecarImageChanged.Spec.SidecarImage = &scyllav1alpha1.ImageSpec{
		Version:    "1.0.0",
		Repository: "my-private-repo",
	}

	dcNameChanged := old.DeepCopy()
	dcNameChanged.Spec.Datacenter.Name = "new-random-name"

	rackPlacementChanged := old.DeepCopy()
	rackPlacementChanged.Spec.Datacenter.Racks[0].Placement = &corev1.Affinity{
		NodeAffinity: &corev1.NodeAffinity{},
	}

	rackStorageChanged := old.DeepCopy()
	rackStorageChanged.Spec.Datacenter.Racks[0].Storage.Capacity = "15Gi"

	rackResourcesChanged := old.DeepCopy()
	rackResourcesChanged.Spec.Datacenter.Racks[0].Resources.Requests = map[corev1.ResourceName]resource.Quantity{
		corev1.ResourceCPU: *resource.NewMilliQuantity(1000, resource.DecimalSI),
	}

	rackDeleted := old.DeepCopy()
	rackDeleted.Spec.Datacenter.Racks = nil

	tests := []struct {
		name    string
		new     *scyllav1alpha1.Cluster
		allowed bool
	}{
		{
			name:    "same as old",
			new:     old,
			allowed: true,
		},
		{
			name:    "version changed",
			new:     versionChanged,
			allowed: false,
		},
		{
			name:    "repo changed",
			new:     repoChanged,
			allowed: false,
		},
		{
			name:    "sidecarImage changed",
			new:     sidecarImageChanged,
			allowed: false,
		},
		{
			name:    "dcName changed",
			new:     dcNameChanged,
			allowed: false,
		},
		{
			name:    "rackPlacement changed",
			new:     rackPlacementChanged,
			allowed: false,
		},
		{
			name:    "rackStorage changed",
			new:     rackStorageChanged,
			allowed: false,
		},
		{
			name:    "rackResources changed",
			new:     rackResourcesChanged,
			allowed: false,
		},
		{
			name:    "rack deleted",
			new:     rackDeleted,
			allowed: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			allowed, msg := checkTransitions(old, test.new)
			require.Equalf(t, test.allowed, allowed, "Wrong value returned from checkTransitions function. Message: '%s'", msg)
		})
	}
}
