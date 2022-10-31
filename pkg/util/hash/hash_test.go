package hash

import (
	"reflect"
	"testing"
)

func TestHashObjects(t *testing.T) {
	tt := []struct {
		name          string
		sets          [][]interface{}
		expectedHash  string
		expectedError error
	}{
		{
			name: "object's map order doesn't matter",
			sets: [][]interface{}{
				{
					map[string]string{
						"key_1": "val_1",
						"key_2": "val_2",
					},
				},
				{
					map[string]string{
						"key_2": "val_2",
						"key_1": "val_1",
					},
				},
			},
			expectedHash:  "9jhCVR1KXrirK3/lDCjEA1x5bXnHBpGIU1z7hgkLd1JAUDJnfOyIqw22zvXMZvvI62Ngr0xFSvyw+B9ifT4Lfg==",
			expectedError: nil,
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			for _, objs := range tc.sets {
				got, err := HashObjects(objs...)

				if !reflect.DeepEqual(err, tc.expectedError) {
					t.Errorf("expected error %v, got %v", tc.expectedError, err)
				}

				if got != tc.expectedHash {
					t.Errorf("expected hash %q, got %q", tc.expectedHash, got)
				}
			}
		})
	}
}
