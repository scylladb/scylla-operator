package scylladbdatacenter

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
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

func Test_runStatefulSetSyncSteps(t *testing.T) {
	t.Parallel()

	cond := func(reason string) metav1.Condition {
		return metav1.Condition{Type: statefulSetControllerProgressingCondition, Status: metav1.ConditionTrue, Reason: reason}
	}

	// step returns a step that records its run and returns the given result.
	type recorder struct{ ran []string }
	step := func(r *recorder, name string, res stepResult, err error) statefulSetSyncStep {
		return statefulSetSyncStep{
			name: name,
			run: func(context.Context, *statefulSetSyncContext) (stepResult, error) {
				r.ran = append(r.ran, name)
				return res, err
			},
		}
	}

	tt := []struct {
		name                 string
		steps                func(*recorder) []statefulSetSyncStep
		expectedRan          []string
		expectedReasons      []string
		expectedRequeueAfter time.Duration
		expectedErrorString  string
	}{
		{
			name: "runs all steps when none blocks",
			steps: func(r *recorder) []statefulSetSyncStep {
				return []statefulSetSyncStep{
					step(r, "a", proceed(), nil),
					step(r, "b", blockWith(), nil),
					step(r, "c", proceed(), nil),
				}
			},
			expectedRan:     []string{"a", "b", "c"},
			expectedReasons: nil,
		},
		{
			name: "stops at the first step returning a condition",
			steps: func(r *recorder) []statefulSetSyncStep {
				return []statefulSetSyncStep{
					step(r, "a", proceed(), nil),
					step(r, "b", blockWith(cond("B1"), cond("B2")), nil),
					step(r, "c", blockWith(cond("C")), nil),
				}
			},
			expectedRan:     []string{"a", "b"},
			expectedReasons: []string{"B1", "B2"},
		},
		{
			name: "stops at the first step asking for a requeue",
			steps: func(r *recorder) []statefulSetSyncStep {
				return []statefulSetSyncStep{
					step(r, "a", requeueIn(5*time.Second), nil),
					step(r, "b", proceed(), nil),
				}
			},
			expectedRan:          []string{"a"},
			expectedReasons:      nil,
			expectedRequeueAfter: 5 * time.Second,
		},
		{
			name: "stops at the first failing step and keeps the conditions produced so far",
			steps: func(r *recorder) []statefulSetSyncStep {
				return []statefulSetSyncStep{
					step(r, "a", proceed(), nil),
					step(r, "b", blockWith(cond("B")), errors.New("boom")),
					step(r, "c", proceed(), nil),
				}
			},
			expectedRan:         []string{"a", "b"},
			expectedReasons:     []string{"B"},
			expectedErrorString: "boom",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := &recorder{}
			res, err := runStatefulSetSyncSteps(context.Background(), &statefulSetSyncContext{sdc: newScyllaDBDatacenter()}, tc.steps(r))

			gotErrorString := ""
			if err != nil {
				gotErrorString = err.Error()
			}
			if gotErrorString != tc.expectedErrorString {
				t.Fatalf("expected error %q, got %q", tc.expectedErrorString, gotErrorString)
			}

			if diff := cmp.Diff(tc.expectedRan, r.ran); diff != "" {
				t.Errorf("steps run differ (-want +got):\n%s", diff)
			}

			var reasons []string
			for _, c := range res.progressingConditions {
				reasons = append(reasons, c.Reason)
			}
			if diff := cmp.Diff(tc.expectedReasons, reasons); diff != "" {
				t.Errorf("condition reasons differ (-want +got):\n%s", diff)
			}

			if res.requeueAfter != tc.expectedRequeueAfter {
				t.Errorf("expected requeue after %v, got %v", tc.expectedRequeueAfter, res.requeueAfter)
			}
		})
	}
}
