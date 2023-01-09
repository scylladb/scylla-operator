package resourcemerge

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestIsRemovalKey(t *testing.T) {
	tt := []struct {
		name     string
		key      string
		expected bool
	}{
		{
			name:     "regular key",
			key:      "foo",
			expected: false,
		},
		{
			name:     "regular key containing a dash",
			key:      "foo-bar",
			expected: false,
		},
		{
			name:     "removal key",
			key:      "foo-bar-",
			expected: true,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			got := isRemovalKey(tc.key)

			if got != tc.expected {
				t.Errorf("expected %t, got %t", tc.expected, got)
			}
		})
	}
}

func FuzzToRemovalKey(f *testing.F) {
	testCases := []string{
		"foo",
		"foo-",
		"scylla-operator.scylladb.com/foo-bar",
	}
	for _, tc := range testCases {
		f.Add(tc)
	}
	f.Fuzz(func(t *testing.T, k string) {
		t.Logf("key: %q", k)
		rk := toRemovalKey(k)

		if !isRemovalKey(rk) {
			t.Errorf("%q isn't a removal key", rk)
		}
	})
}

func TestCleanRemovalKeys(t *testing.T) {
	tt := []struct {
		name     string
		m        map[string]string
		expected map[string]string
	}{
		{
			name:     "nil map",
			m:        nil,
			expected: nil,
		},
		{
			name:     "empty map",
			m:        map[string]string{},
			expected: map[string]string{},
		},
		{
			name: "removal keys are cleaned",
			m: map[string]string{
				"foo":  "alpha",
				"foo-": "beta",
				"bar-": "gama",
			},
			expected: map[string]string{
				"foo": "alpha",
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			got := cleanRemovalKeys(tc.m)

			if !reflect.DeepEqual(got, tc.expected) {
				t.Errorf("expected and got differs: %s", cmp.Diff(tc.expected, got))
			}
		})
	}
}

func TestSanitizeObject(t *testing.T) {
	tt := []struct {
		name     string
		obj      *metav1.ObjectMeta
		expected *metav1.ObjectMeta
	}{
		{
			name: "object with nils",
			obj: &metav1.ObjectMeta{
				Annotations: nil,
				Labels:      nil,
			},
			expected: &metav1.ObjectMeta{
				Annotations: nil,
				Labels:      nil,
			},
		},
		{
			name: "mixed values",
			obj: &metav1.ObjectMeta{
				Annotations: map[string]string{
					"a-foo":  "a-alpha",
					"a-bar-": "a-beta",
				},
				Labels: map[string]string{
					"l-foo":  "l-alpha",
					"l-bar-": "l-beta",
				},
			},
			expected: &metav1.ObjectMeta{
				Annotations: map[string]string{
					"a-foo": "a-alpha",
				},
				Labels: map[string]string{
					"l-foo": "l-alpha",
				},
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.obj.DeepCopy()
			SanitizeObject(got)

			if !reflect.DeepEqual(got, tc.expected) {
				t.Errorf("expected and got differs: %s", cmp.Diff(tc.expected, got))
			}
		})
	}
}

func TestMergeMapInPlaceWithoutRemovalKeys(t *testing.T) {
	tt := []struct {
		name           string
		required       map[string]string
		existing       map[string]string
		expected       map[string]string
		expectPanicErr error
	}{
		{
			name:     "nil required map panics",
			required: nil,
			existing: map[string]string{
				"bar": "beta",
			},
			expected:       nil,
			expectPanicErr: fmt.Errorf("assignment to entry in nil map"),
		},
		{
			name: "no existing map",
			required: map[string]string{
				"foo": "alpha",
			},
			existing: nil,
			expected: map[string]string{
				"foo": "alpha",
			},
		},
		{
			name: "removal key is removed",
			required: map[string]string{
				"foo":  "alpha",
				"bar-": "",
			},
			existing: map[string]string{
				"foo": "alpha",
				"bar": "beta",
			},
			expected: map[string]string{
				"foo": "alpha",
			},
		},
		{
			name: "removal keys are not copied",
			required: map[string]string{
				"foo":  "alpha",
				"bar-": "",
			},
			existing: map[string]string{
				"foo":    "alpha",
				"bar-":   "beta",
				"extra-": "gama",
			},
			expected: map[string]string{
				"foo": "alpha",
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				r := recover()
				if r == nil && tc.expectPanicErr != nil {
					t.Errorf("expected panic error %q, got none", tc.expectPanicErr.Error())
				}
				if r != nil && tc.expectPanicErr.Error() != r.(error).Error() {
					t.Errorf("expected panic error %q, got %q", tc.expectPanicErr.Error(), r.(error).Error())

				}
			}()

			var got map[string]string
			if tc.required != nil {
				got = make(map[string]string, len(tc.required))
				for k, v := range tc.required {
					got[k] = v
				}
			}
			MergeMapInPlaceWithoutRemovalKeys(got, tc.existing)

			if !reflect.DeepEqual(got, tc.expected) {
				t.Errorf("expected and got differs: %s", cmp.Diff(tc.expected, got))
			}
		})
	}
}

func TestMergeMetadataInPlace(t *testing.T) {
	tt := []struct {
		name     string
		required *metav1.ObjectMeta
		existing *metav1.ObjectMeta
		expected *metav1.ObjectMeta
	}{
		{
			name: "nil maps",
			required: &metav1.ObjectMeta{
				Annotations: nil,
				Labels:      nil,
			},
			existing: &metav1.ObjectMeta{
				Annotations: nil,
				Labels:      nil,
			},
			expected: &metav1.ObjectMeta{
				Annotations: nil,
				Labels:      nil,
			},
		},
		{
			name: "removal keys are removed",
			required: &metav1.ObjectMeta{
				Annotations: map[string]string{
					"a-1":  "foo",
					"a-2-": "",
				},
				Labels: map[string]string{
					"l-1":  "bar",
					"l-2-": "",
				},
			},
			existing: &metav1.ObjectMeta{
				Annotations: map[string]string{
					"a-1": "foo",
					"a-2": "old",
				},
				Labels: map[string]string{
					"l-1": "bar",
					"l-2": "old",
				},
			},
			expected: &metav1.ObjectMeta{
				Annotations: map[string]string{
					"a-1": "foo",
				},
				Labels: map[string]string{
					"l-1": "bar",
				},
			},
		},
		{
			name: "new fields are added",
			required: &metav1.ObjectMeta{
				Annotations: map[string]string{
					"a-1": "foo",
				},
				Labels: map[string]string{
					"l-1": "bar",
				},
			},
			existing: &metav1.ObjectMeta{
				Annotations: nil,
				Labels:      nil,
			},
			expected: &metav1.ObjectMeta{
				Annotations: map[string]string{
					"a-1": "foo",
				},
				Labels: map[string]string{
					"l-1": "bar",
				},
			},
		},
		{
			name: "unmanaged keys are kept",
			required: &metav1.ObjectMeta{
				Annotations: map[string]string{
					"a-1": "foo",
				},
				Labels: map[string]string{
					"l-1": "bar",
				},
			},
			existing: &metav1.ObjectMeta{
				Annotations: map[string]string{
					"user-annotation": "ua",
				},
				Labels: map[string]string{
					"user-label": "ul",
				},
			},
			expected: &metav1.ObjectMeta{
				Annotations: map[string]string{
					"a-1":             "foo",
					"user-annotation": "ua",
				},
				Labels: map[string]string{
					"l-1":        "bar",
					"user-label": "ul",
				},
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.required.DeepCopy()
			MergeMetadataInPlace(got, tc.existing)

			if !reflect.DeepEqual(got, tc.expected) {
				t.Errorf("expected and got differs: %s", cmp.Diff(tc.expected, got))
			}
		})
	}
}
