package collect

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestOmitManagedFieldsPrinter_PrintObj(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name           string
		resourceInfo   *ResourceInfo
		object         runtime.Object
		expectedError  error
		expectedString string
	}{
		{
			name: "prints object without managed fields",
			resourceInfo: &ResourceInfo{
				Scope: meta.RESTScopeRoot,
				Resource: schema.GroupVersionResource{
					Group:    "",
					Version:  "",
					Resource: "",
				},
			},
			object: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-namespace",
					ManagedFields: []metav1.ManagedFieldsEntry{
						{
							Manager: "foo",
						},
					},
				},
			},
			expectedString: strings.TrimPrefix(`
metadata:
  creationTimestamp: null
  name: my-namespace
spec: {}
status: {}
`, "\n"),
		},
	}
	for _, tc := range tt {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			printer := OmitManagedFieldsPrinter{
				Delegate: &YAMLPrinter{},
			}
			var buffer bytes.Buffer
			err := printer.PrintObj(tc.resourceInfo, tc.object, &buffer)

			if !reflect.DeepEqual(err, tc.expectedError) {
				t.Errorf("expected error %v, got %v", tc.expectedError, err)
			}

			s := buffer.String()
			if !reflect.DeepEqual(s, tc.expectedString) {
				t.Errorf("expected and got contents differ: %s", cmp.Diff(tc.expectedString, s))
			}
		})
	}
}
