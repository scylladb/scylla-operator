// Copyright (c) 2023 ScyllaDB

package naming

import (
	"fmt"
	"reflect"
	"testing"
)

func Test_ImageToVersion(t *testing.T) {
	t.Parallel()

	tcs := []struct {
		name            string
		image           string
		expectedVersion string
		expectedError   error
	}{
		{
			name:            "tagged reference with a domain",
			image:           "docker.io/scylladb/scylla:5.1.15",
			expectedVersion: "5.1.15",
			expectedError:   nil,
		},
		{
			name:            "tagged reference without a domain",
			image:           "scylladb/scylla:5.1.15",
			expectedVersion: "5.1.15",
			expectedError:   nil,
		},
		{
			name:            "tagged reference with port in domain",
			image:           "someproxy:1234/proxy/scylladb/scylla:5.1.15",
			expectedVersion: "5.1.15",
			expectedError:   nil,
		},
		{
			name:            "tagged and digested reference",
			image:           "docker.io/scylladb/scylla:5.1.15@sha256:12a95529d94498d8f56fc4596f8ac570e961618fab7ab5cf3e3df781c2b8fd33",
			expectedVersion: "5.1.15",
			expectedError:   nil,
		},
		{
			name:          "digested reference",
			image:         "docker.io/scylladb/scylla@sha256:12a95529d94498d8f56fc4596f8ac570e961618fab7ab5cf3e3df781c2b8fd33",
			expectedError: fmt.Errorf("invalid, non-tagged image reference of type reference.canonicalReference: docker.io/scylladb/scylla@sha256:12a95529d94498d8f56fc4596f8ac570e961618fab7ab5cf3e3df781c2b8fd33"),
		},
	}

	for i := range tcs {
		tc := tcs[i]

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			version, err := ImageToVersion(tc.image)

			if !reflect.DeepEqual(err, tc.expectedError) {
				t.Fatalf("expected error: %#v, got: %#v", tc.expectedError, err)
			}
			if !reflect.DeepEqual(version, tc.expectedVersion) {
				t.Fatalf("expected version: %s, got: %s", tc.expectedVersion, version)
			}
		})
	}
}
