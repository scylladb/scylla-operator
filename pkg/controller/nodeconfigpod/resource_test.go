package nodeconfigpod

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestMakeConfigMap(t *testing.T) {
	tt := []struct {
		name     string
		pod      *corev1.Pod
		data     map[string]string
		expected *corev1.ConfigMap
	}{
		{
			name: "basic",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo",
					Name:      "bar",
					UID:       "42",
				},
			},
			data: map[string]string{
				"key": "value",
			},
			expected: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo",
					Name:      "nodeconfig-podinfo-42",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "v1",
							Kind:               "Pod",
							Name:               "bar",
							UID:                "42",
							BlockOwnerDeletion: pointer.Ptr(true),
							Controller:         pointer.Ptr(true),
						},
					},
					Labels: map[string]string{
						"scylla-operator.scylladb.com/owner-uid":       "42",
						"scylla-operator.scylladb.com/config-map-type": "NodeConfigData",
					},
				},
				Data: map[string]string{
					"key": "value",
				},
			},
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			got := makeConfigMap(tc.pod, tc.data)

			if !apiequality.Semantic.DeepEqual(tc.expected, got) {
				t.Errorf("expected and gotten ConfigMaps differ: \n%s", cmp.Diff(tc.expected, got))
			}
		})
	}
}
