package validating

import (
	"testing"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/apis/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/test/unit"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
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
	tests := []struct {
		name    string
		old     *scyllav1alpha1.Cluster
		new     *scyllav1alpha1.Cluster
		allowed bool
	}{
		{
			name:    "same as old",
			old:     unit.NewSingleRackCluster(3),
			new:     unit.NewSingleRackCluster(3),
			allowed: true,
		},

		{
			name:    "major version changed",
			old:     unit.NewSingleRackCluster(3),
			new:     unit.NewDetailedSingleRackCluster("test-cluster", "test-ns", "repo", "3.3.1", "test-dc", "test-rack", 3),
			allowed: false,
		},
		{
			name:    "minor version changed",
			old:     unit.NewSingleRackCluster(3),
			new:     unit.NewDetailedSingleRackCluster("test-cluster", "test-ns", "repo", "2.4.2", "test-dc", "test-rack", 3),
			allowed: true,
		},
		{
			name:    "patch version changed",
			old:     unit.NewSingleRackCluster(3),
			new:     unit.NewDetailedSingleRackCluster("test-cluster", "test-ns", "repo", "2.3.2", "test-dc", "test-rack", 3),
			allowed: true,
		},
		{
			name:    "repo changed",
			old:     unit.NewSingleRackCluster(3),
			new:     unit.NewDetailedSingleRackCluster("test-cluster", "test-ns", "new-repo", "2.3.2", "test-dc", "test-rack", 3),
			allowed: false,
		},
		{
			name:    "sidecarImage changed",
			old:     unit.NewSingleRackCluster(3),
			new:     sidecarImageChanged(unit.NewSingleRackCluster(3)),
			allowed: false,
		},
		{
			name:    "dcName changed",
			old:     unit.NewSingleRackCluster(3),
			new:     unit.NewDetailedSingleRackCluster("test-cluster", "test-ns", "repo", "2.3.1", "new-dc", "test-rack", 3),
			allowed: false,
		},
		{
			name:    "rackPlacement changed",
			old:     unit.NewSingleRackCluster(3),
			new:     placementChanged(unit.NewSingleRackCluster(3)),
			allowed: false,
		},
		{
			name:    "rackStorage changed",
			old:     unit.NewSingleRackCluster(3),
			new:     storageChanged(unit.NewSingleRackCluster(3)),
			allowed: false,
		},
		{
			name:    "rackResources changed",
			old:     unit.NewSingleRackCluster(3),
			new:     resourceChanged(unit.NewSingleRackCluster(3)),
			allowed: false,
		},
		{
			name:    "rack deleted",
			old:     unit.NewSingleRackCluster(3),
			new:     rackDeleted(unit.NewSingleRackCluster(3)),
			allowed: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			allowed, msg := checkTransitions(test.old, test.new)
			require.Equalf(t, test.allowed, allowed, "Wrong value returned from checkTransitions function. Message: '%s'", msg)
		})
	}
}

func placementChanged(c *scyllav1alpha1.Cluster) *scyllav1alpha1.Cluster {
	c.Spec.Datacenter.Racks[0].Placement = &scyllav1alpha1.PlacementSpec{}
	return c
}

func resourceChanged(c *scyllav1alpha1.Cluster) *scyllav1alpha1.Cluster {
	c.Spec.Datacenter.Racks[0].Resources.Requests = map[corev1.ResourceName]resource.Quantity{
		corev1.ResourceCPU: *resource.NewMilliQuantity(1000, resource.DecimalSI),
	}
	return c
}

func rackDeleted(c *scyllav1alpha1.Cluster) *scyllav1alpha1.Cluster {
	c.Spec.Datacenter.Racks = nil
	return c
}

func sidecarImageChanged(c *scyllav1alpha1.Cluster) *scyllav1alpha1.Cluster {
	c.Spec.SidecarImage = &scyllav1alpha1.ImageSpec{
		Version:    "1.0.0",
		Repository: "my-private-repo",
	}
	return c
}

func storageChanged(c *scyllav1alpha1.Cluster) *scyllav1alpha1.Cluster {
	c.Spec.Datacenter.Racks[0].Storage.Capacity = "15Gi"
	return c
}
