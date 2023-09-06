package controllerhelpers

import (
	"testing"

	"github.com/scylladb/scylla-operator/pkg/pointer"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestGenerateMergePatch(t *testing.T) {
	tt := []struct {
		name     string
		original runtime.Object
		modifyFn func(runtime.Object)
		expected []byte
	}{
		{
			name: "pod replica change",
			original: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo",
					Name:      "bar",
				},
				Spec: corev1.PodSpec{
					ActiveDeadlineSeconds: pointer.Ptr(int64(30)),
					NodeName:              "node",
				},
			},
			modifyFn: func(obj runtime.Object) {
				pod := obj.(*corev1.Pod)
				pod.Spec.ActiveDeadlineSeconds = pointer.Ptr(int64(10))
			},
			expected: []byte(`{"spec":{"activeDeadlineSeconds":10}}`),
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			modified := tc.original.DeepCopyObject()
			tc.modifyFn(modified)
			got, err := GenerateMergePatch(tc.original, modified)
			if err != nil {
				t.Error(err)
			}

			if !apiequality.Semantic.DeepEqual(got, tc.expected) {
				t.Errorf("expected %s, got %s", tc.expected, got)
			}
		})
	}
}
