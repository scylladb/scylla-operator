// Copyright (C) 2017 ScyllaDB

package parallel

import (
	"fmt"
	"reflect"
	"sort"
	"testing"

	apimachineryutilerrors "k8s.io/apimachinery/pkg/util/errors"
)

func TestForEach(t *testing.T) {
	anError := func(i int) error {
		return fmt.Errorf("test error #%d", i)
	}

	tt := []struct {
		name        string
		length      int
		f           func(i int) error
		expectedErr error
	}{
		{
			name:   "single nil",
			length: 1,
			f: func(i int) error {
				switch i {
				case 0:
					return nil
				default:
					panic("out of range")
				}
			},
			expectedErr: nil,
		},
		{
			name:   "two nils",
			length: 2,
			f: func(i int) error {
				switch i {
				case 0, 1:
					return nil
				default:
					panic("out of range")
				}
			},
			expectedErr: nil,
		},
		{
			name:   "single error",
			length: 1,
			f: func(i int) error {
				switch i {
				case 0:
					return anError(i)
				default:
					panic("out of range")
				}
			},
			expectedErr: apimachineryutilerrors.NewAggregate([]error{anError(0)}),
		},
		{
			name:   "two errors",
			length: 2,
			f: func(i int) error {
				switch i {
				case 0, 1:
					return anError(i)
				default:
					panic("out of range")
				}
			},
			expectedErr: apimachineryutilerrors.NewAggregate([]error{anError(0), anError(1)}),
		},
		{
			name:   "mixed",
			length: 5,
			f: func(i int) error {
				switch i {
				case 0, 2, 4:
					return nil
				case 1, 3:
					return anError(i)
				default:
					panic("out of range")
				}
			},
			expectedErr: apimachineryutilerrors.NewAggregate([]error{nil, anError(1), nil, anError(3), nil}),
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			gotErr := ForEach(tc.length, tc.f)

			// Sort the errors to avoid random ordering from parallelism.
			if gotErr != nil {
				errs := gotErr.(apimachineryutilerrors.Aggregate).Errors()
				sort.Slice(errs, func(i, j int) bool {
					return errs[i].Error() < errs[j].Error()
				})
			}

			if !reflect.DeepEqual(gotErr, tc.expectedErr) {
				t.Errorf("expected %v, got %v", tc.expectedErr, gotErr)
			}
		})

	}
}
