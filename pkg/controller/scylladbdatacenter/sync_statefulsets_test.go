package scylladbdatacenter

import (
	"testing"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	testNamespace     = "default"
	testScyllaDBImage = "scylladb/scylla:latest"
)

func Test_makeRequiredStatefulSets(t *testing.T) {
	t.Parallel()

	sdc := newScyllaDBDatacenter()
	sdc.Name = "basic"

	t.Run("waits for the node exporter image", func(t *testing.T) {
		t.Parallel()

		sdcc := &Controller{}
		required, conditions, err := sdcc.makeRequiredStatefulSets(sdc, &scyllav1alpha1.ScyllaOperatorConfig{}, nil, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if required != nil || len(conditions) != 1 || conditions[0].Reason != reasonWaitingForScyllaDBNodeExporterImage {
			t.Errorf("expected only a %q condition, got %v, %v", reasonWaitingForScyllaDBNodeExporterImage, required, conditions)
		}
	})

	t.Run("waits for the managed config", func(t *testing.T) {
		t.Parallel()

		soc := &scyllav1alpha1.ScyllaOperatorConfig{}
		soc.Status.ScyllaDBNodeExporterImage = new("node-exporter:latest")

		sdcc := &Controller{}
		required, conditions, err := sdcc.makeRequiredStatefulSets(sdc, soc, nil, map[string]*corev1.ConfigMap{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if required != nil || len(conditions) != 1 || conditions[0].Reason != reasonWaitingForManagedConfig {
			t.Errorf("expected only a %q condition, got %v, %v", reasonWaitingForManagedConfig, required, conditions)
		}
	})
}

func newScyllaDBDatacenter() *scyllav1alpha1.ScyllaDBDatacenter {
	return &scyllav1alpha1.ScyllaDBDatacenter{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNamespace,
		},
		Spec: scyllav1alpha1.ScyllaDBDatacenterSpec{
			ScyllaDB: scyllav1alpha1.ScyllaDB{
				Image: testScyllaDBImage,
			},
		},
	}
}

func newStatefulSet(name string) *appsv1.StatefulSet {
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      name,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: scyllav1alpha1.GroupVersion.String(),
					Kind:       "ScyllaDBDatacenter",
					Name:       "basic",
					UID:        "owner",
					Controller: new(true),
				},
			},
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: new(int32(1)),
		},
	}
}
