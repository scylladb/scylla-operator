package helpers

import (
	"fmt"
	"reflect"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

func TestIsStatefulSetRolledOut(t *testing.T) {
	tt := []struct {
		name        string
		sts         *appsv1.StatefulSet
		expected    bool
		expectedErr error
	}{
		{
			name: "sts with OnDelete strategy will fail",
			sts: &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Generation: 42,
				},
				Spec: appsv1.StatefulSetSpec{
					Replicas: pointer.Int32Ptr(3),
					UpdateStrategy: appsv1.StatefulSetUpdateStrategy{
						Type: appsv1.OnDeleteStatefulSetStrategyType,
						RollingUpdate: &appsv1.RollingUpdateStatefulSetStrategy{
							Partition: nil,
						},
					},
				},
			},
			expected:    false,
			expectedErr: fmt.Errorf("can't determine rollout status for OnDelete strategy type"),
		},
		{
			name: "sts status is stale",
			sts: &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Generation: 42,
				},
				Spec: appsv1.StatefulSetSpec{
					Replicas: pointer.Int32Ptr(3),
					UpdateStrategy: appsv1.StatefulSetUpdateStrategy{
						Type: appsv1.RollingUpdateStatefulSetStrategyType,
						RollingUpdate: &appsv1.RollingUpdateStatefulSetStrategy{
							Partition: nil,
						},
					},
				},
				Status: appsv1.StatefulSetStatus{
					ObservedGeneration: 21,
					Replicas:           3,
					ReadyReplicas:      3,
					CurrentReplicas:    3,
					CurrentRevision:    "foo",
					UpdatedReplicas:    3,
					UpdateRevision:     "foo",
				},
			},
			expected:    false,
			expectedErr: nil,
		},
		{
			name: "sts in progress",
			sts: &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Generation: 42,
				},
				Spec: appsv1.StatefulSetSpec{
					Replicas: pointer.Int32Ptr(3),
					UpdateStrategy: appsv1.StatefulSetUpdateStrategy{
						Type: appsv1.RollingUpdateStatefulSetStrategyType,
						RollingUpdate: &appsv1.RollingUpdateStatefulSetStrategy{
							Partition: nil,
						},
					},
				},
				Status: appsv1.StatefulSetStatus{
					ObservedGeneration: 42,
					Replicas:           3,
					ReadyReplicas:      3,
					CurrentReplicas:    2,
					CurrentRevision:    "foo",
					UpdatedReplicas:    1,
					UpdateRevision:     "bar",
				},
			},
			expected:    false,
			expectedErr: nil,
		},
		{
			name: "partitioned sts in progress",
			sts: &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Generation: 42,
				},
				Spec: appsv1.StatefulSetSpec{
					Replicas: pointer.Int32Ptr(3),
					UpdateStrategy: appsv1.StatefulSetUpdateStrategy{
						Type: appsv1.RollingUpdateStatefulSetStrategyType,
						RollingUpdate: &appsv1.RollingUpdateStatefulSetStrategy{
							Partition: pointer.Int32Ptr(1),
						},
					},
				},
				Status: appsv1.StatefulSetStatus{
					ObservedGeneration: 42,
					Replicas:           3,
					ReadyReplicas:      3,
					CurrentReplicas:    2,
					CurrentRevision:    "foo",
					UpdatedReplicas:    1,
					UpdateRevision:     "bar",
				},
			},
			expected:    false,
			expectedErr: nil,
		},
		{
			name: "sts is fully rolled out",
			sts: &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Generation: 42,
				},
				Spec: appsv1.StatefulSetSpec{
					Replicas: pointer.Int32Ptr(3),
					UpdateStrategy: appsv1.StatefulSetUpdateStrategy{
						Type: appsv1.RollingUpdateStatefulSetStrategyType,
						RollingUpdate: &appsv1.RollingUpdateStatefulSetStrategy{
							Partition: nil,
						},
					},
				},
				Status: appsv1.StatefulSetStatus{
					ObservedGeneration: 42,
					Replicas:           3,
					ReadyReplicas:      3,
					CurrentReplicas:    3,
					CurrentRevision:    "bar",
					UpdatedReplicas:    3,
					UpdateRevision:     "bar",
				},
			},
			expected:    true,
			expectedErr: nil,
		},
		{
			name: "partitioned sts is rolled out",
			sts: &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Generation: 42,
				},
				Spec: appsv1.StatefulSetSpec{
					Replicas: pointer.Int32Ptr(3),
					UpdateStrategy: appsv1.StatefulSetUpdateStrategy{
						Type: appsv1.RollingUpdateStatefulSetStrategyType,
						RollingUpdate: &appsv1.RollingUpdateStatefulSetStrategy{
							Partition: pointer.Int32Ptr(1),
						},
					},
				},
				Status: appsv1.StatefulSetStatus{
					ObservedGeneration: 42,
					Replicas:           3,
					ReadyReplicas:      3,
					CurrentReplicas:    1,
					CurrentRevision:    "foo",
					UpdatedReplicas:    2,
					UpdateRevision:     "bar",
				},
			},
			expected:    true,
			expectedErr: nil,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			got, gotErr := IsStatefulSetRolledOut(tc.sts)

			if !reflect.DeepEqual(gotErr, tc.expectedErr) {
				t.Fatalf("expected error %v, got %v", tc.expectedErr, gotErr)
			}

			if got != tc.expected {
				t.Errorf("expected %T, got %T", tc.expected, got)
			}
		})
	}
}
