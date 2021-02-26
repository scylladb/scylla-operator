// Copyright (C) 2021 ScyllaDB

package resourceapply_test

import (
	"context"
	"testing"

	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/util/resourceapply"
	"k8s.io/api/policy/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func ClientGoScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		panic(err)
	}
	return scheme
}

func TestApplyPodDisruptionBudge2(t *testing.T) {
	ctx := context.Background()

	one := intstr.FromInt(1)
	two := intstr.FromInt(2)

	pdbOneMaxUnavailable := &v1beta1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: make(map[string]string),
			Name:        "pdb-name",
			Namespace:   "pdb-namespace",
		},
		Spec: v1beta1.PodDisruptionBudgetSpec{
			MaxUnavailable: &one,
			Selector:       metav1.SetAsLabelSelector(make(map[string]string)),
		},
	}

	pdbOneMaxUnavailableWithHash := pdbOneMaxUnavailable.DeepCopy()

	pdbOneMaxUnavailableWithHash.Annotations[naming.ManagedHash] = resourceapply.MustHashObjects(pdbOneMaxUnavailableWithHash)

	pdbTwoMaxUnavailable := pdbOneMaxUnavailable.DeepCopy()
	pdbTwoMaxUnavailable.Spec.MaxUnavailable = &two

	tests := []struct {
		Name         string
		Objects      []runtime.Object
		InputPDB     *v1beta1.PodDisruptionBudget
		ExpectedHash string
		ExpectEvent  bool
	}{
		{
			Name:         "object is created when it doesn't exists",
			Objects:      []runtime.Object{},
			InputPDB:     pdbOneMaxUnavailable.DeepCopy(),
			ExpectedHash: resourceapply.MustHashObjects(pdbOneMaxUnavailable.DeepCopy()),
			ExpectEvent:  true,
		},
		{
			Name:         "object isn't updated when it has the same hash",
			Objects:      []runtime.Object{pdbOneMaxUnavailableWithHash.DeepCopy()},
			InputPDB:     pdbOneMaxUnavailable.DeepCopy(),
			ExpectedHash: pdbOneMaxUnavailableWithHash.Annotations[naming.ManagedHash],
			ExpectEvent:  false,
		},
		{
			Name:         "object is updated when it is different than existing one",
			Objects:      []runtime.Object{pdbOneMaxUnavailableWithHash.DeepCopy()},
			InputPDB:     pdbTwoMaxUnavailable.DeepCopy(),
			ExpectedHash: resourceapply.MustHashObjects(pdbTwoMaxUnavailable.DeepCopy()),
			ExpectEvent:  true,
		},
	}

	for i := range tests {
		test := tests[i]
		t.Run(test.Name, func(t *testing.T) {
			t.Parallel()

			client := fake.NewClientBuilder().
				WithScheme(ClientGoScheme()).
				WithRuntimeObjects(test.Objects...).
				Build()
			recorder := record.NewFakeRecorder(10)

			if err := resourceapply.ApplyPodDisruptionBudget(ctx, recorder, client, test.InputPDB); err != nil {
				t.Errorf("failed to apply pdb, got err %v", err)
			}

			pdbKey := naming.NamespacedName(test.InputPDB.Name, test.InputPDB.Namespace)
			result := &v1beta1.PodDisruptionBudget{}
			if err := client.Get(ctx, pdbKey, result); err != nil {
				t.Errorf("failed to get pdb, got err %v", err)
			}

			if result.Annotations[naming.ManagedHash] != test.ExpectedHash {
				t.Errorf("object hash is different then expected %s", result.Annotations[naming.ManagedHash])
			}

			if test.ExpectEvent {
				select {
				case _ = <-recorder.Events:
				default:
					t.Errorf("expected event but it wasn't emitted")
				}
			} else {
				if len(recorder.Events) != 0 {
					t.Errorf("expected no event but it was emitted, %s", <-recorder.Events)
				}
			}
		})
	}
}
